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
		CalendarItem: CalendarItem{Start: start},
	}
	endField := SetItemField{
		FieldURI:     FieldURI{FieldURI: "calendar:End"},
		CalendarItem: CalendarItem{End: end},
	}

	item := &UpdateItem{
		MessageDisposition:     "SaveOnly",
		SendMeetingInvitations: "SendToAllAndSaveCopy",
		ConflictResolution:     "AlwaysOverwrite",
		ItemChanges: []ItemChange{{
			ItemId: ItemId{Id: "AAA=", ChangeKey: "BBB"},
			Updates: Updates{
				SetItem: []SetItemField{startField, endField},
			},
		}},
	}

	xmlBytes, err := xml.MarshalIndent(item, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Critical: ItemId must carry both Id and ChangeKey attributes.
	assert.Contains(t, out, `<t:ItemId Id="AAA=" ChangeKey="BBB"`)
	assert.Contains(t, out, `FieldURI="calendar:Start"`)
	assert.Contains(t, out, `FieldURI="calendar:End"`)
	assert.Contains(t, out, "2026-08-11T00:00:00+03:00")
	assert.Contains(t, out, "2026-08-11T23:59:59+03:00")

	// Negative guard: empty ChangeKey attribute must never appear.
	assert.NotContains(t, out, `ChangeKey=""`)
	// Negative guard: bare ItemId without ChangeKey must never appear.
	assert.False(t, strings.Contains(out, `<t:ItemId Id="AAA=">`))
}
