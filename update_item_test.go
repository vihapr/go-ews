package ews

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test_marshal_UpdateItem_with_ChangeKey guards against regression of the bug
// where UpdateItem was sent without a ChangeKey attribute, which caused
// Exchange to reject the request with the generic "The request is invalid."
// message. The ChangeKey is now populated by UpdateCalendarItemTime via
// GetCalendarItemChangeKey before marshaling.
func Test_marshal_UpdateItem_with_ChangeKey(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")

	startField := SetItemField{
		FieldURI:     FieldURI{FieldURI: "calendar:Start"},
		CalendarItem: updateCalendarItem{Start: &start},
	}
	endField := SetItemField{
		FieldURI:     FieldURI{FieldURI: "calendar:End"},
		CalendarItem: updateCalendarItem{End: &end},
	}

	item := &UpdateItem{
		MessageDisposition:                    "SaveOnly",
		SendMeetingInvitationsOrCancellations: "SendToAllAndSaveCopy",
		ConflictResolution:                    "AlwaysOverwrite",
		ItemChanges: ItemChanges{Items: []ItemChange{{
			ItemId: ItemId{Id: "AAA=", ChangeKey: "BBB"},
			Updates: Updates{
				SetItem: []SetItemField{startField, endField},
			},
		}}},
	}

	xmlBytes, err := xml.MarshalIndent(item, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Critical: ItemId must carry both Id and ChangeKey attributes.
	assert.Contains(t, out, `<t:ItemId Id="AAA=" ChangeKey="BBB"`)
	// Critical: <t:ItemChange> elements MUST live inside an <m:ItemChanges>
	// wrapper. Go's encoding/xml silently drops the wrapper tag when the slice
	// element has its own XMLName, leaving bare <t:ItemChange> children
	// directly under <m:UpdateItem>. Exchange rejects that malformed request
	// with HTTP 500 (ErrorInvalidArgument, "The request is invalid.").
	assert.Contains(t, out, `<m:ItemChanges>`)
	assert.Contains(t, out, `</m:ItemChanges>`)
	assert.Contains(t, out, `<t:ItemChange>`)
	assert.Contains(t, out, `</t:ItemChange>`)
	// Critical: UpdateItem must use SendMeetingInvitationsOrCancellations
	// (the CreateItem attribute SendMeetingInvitations is schema-invalid here
	// and Exchange rejects it with "The request is invalid.").
	assert.Contains(t, out, `SendMeetingInvitationsOrCancellations="SendToAllAndSaveCopy"`)
	assert.NotContains(t, out, `SendMeetingInvitations=`)
	assert.Contains(t, out, `FieldURI="calendar:Start"`)
	assert.Contains(t, out, `FieldURI="calendar:End"`)
	assert.Contains(t, out, "2026-08-11T00:00:00+03:00")
	assert.Contains(t, out, "2026-08-11T23:59:59+03:00")

	// Negative guard: empty ChangeKey attribute must never appear.
	assert.NotContains(t, out, `ChangeKey=""`)
	// Negative guard: bare ItemId without ChangeKey must never appear.
	assert.False(t, strings.Contains(out, `<t:ItemId Id="AAA=">`))

	// Negative guard: zero-value fields must not leak into SetItemField —
	// only the field referenced by FieldURI should be present.
	assert.NotContains(t, out, "0001-01-01")
	assert.NotContains(t, out, "<t:Subject>")
	assert.NotContains(t, out, "<t:Body")
	assert.NotContains(t, out, "<t:ReminderIsSet>")
	assert.NotContains(t, out, "<t:Location>")
}
