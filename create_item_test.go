package ews

import (
	"encoding/xml"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
	"time"
)

func Test_marshal_CalendarItem(t *testing.T) {

	attendee := make([]Attendee, 0)
	attendee = append(attendee,
		Attendee{Mailbox: Mailbox{EmailAddress: "User1@example.com"}},
		Attendee{Mailbox: Mailbox{EmailAddress: "User2@example.com"}},
	)
	attendees := make([]Attendees, 0)
	attendees = append(attendees, Attendees{Attendee: attendee})

	start, _ := time.Parse(time.RFC3339, "2006-11-02T14:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2006-11-02T15:00:00Z")

	citem := &CalendarItem{
		Subject: "Planning Meeting",
		Body: Body{
			BodyType: "Text",
			Body:     []byte("Plan the agenda for next week's meeting."),
		},
		ReminderIsSet:              true,
		ReminderMinutesBeforeStart: 60,
		Start:                      start,
		End:                        end,
		IsAllDayEvent:              false,
		LegacyFreeBusyStatus:       "Busy",
		Location:                   "Conference Room 721",
		RequiredAttendees:          attendees,
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	assert.Equal(t, `<CalendarItem>
  <t:Subject>Planning Meeting</t:Subject>
  <t:Body BodyType="Text">Plan the agenda for next week&#39;s meeting.</t:Body>
  <t:ReminderIsSet>true</t:ReminderIsSet>
  <t:ReminderMinutesBeforeStart>60</t:ReminderMinutesBeforeStart>
  <t:Start>2006-11-02T14:00:00Z</t:Start>
  <t:End>2006-11-02T15:00:00Z</t:End>
  <t:IsAllDayEvent>false</t:IsAllDayEvent>
  <t:LegacyFreeBusyStatus>Busy</t:LegacyFreeBusyStatus>
  <t:Location>Conference Room 721</t:Location>
  <t:RequiredAttendees>
    <t:Attendee>
      <t:Mailbox>
        <t:EmailAddress>User1@example.com</t:EmailAddress>
      </t:Mailbox>
    </t:Attendee>
    <t:Attendee>
      <t:Mailbox>
        <t:EmailAddress>User2@example.com</t:EmailAddress>
      </t:Mailbox>
    </t:Attendee>
  </t:RequiredAttendees>
</CalendarItem>`, string(xmlBytes))
}

// Test_marshal_CalendarItem_with_UID проверяет, что заполненный UID
// сериализуется как <t:UID> в позиции, предписанной EWS-схемой
// CalendarItemType (перед <t:Start>).
func Test_marshal_CalendarItem_with_UID(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-02-19T10:00:00Z")
	end, _ := time.Parse(time.RFC3339, "2026-02-19T11:00:00Z")

	citem := &CalendarItem{
		Subject:                    "Absence",
		ReminderIsSet:              false,
		ReminderMinutesBeforeStart: 0,
		UID:                        "11223344-5566-7788-99aa-bbccddeeff00",
		Start:                      start,
		End:                        end,
		IsAllDayEvent:              false,
		LegacyFreeBusyStatus:       "OOF",
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// UID должен присутствовать ровно один раз, в позиции между
	// ReminderMinutesBeforeStart и Start (EWS CalendarItemType schema).
	assert.Contains(t, out, "<t:UID>11223344-5566-7788-99aa-bbccddeeff00</t:UID>")

	// Схема-корректная позиция: UID перед Start.
	startIdx := indexOf(t, out, "<t:Start>")
	uidIdx := indexOf(t, out, "<t:UID>")
	if uidIdx >= startIdx {
		t.Fatalf("UID must precede Start in CalendarItem XML schema; got UID at %d, Start at %d", uidIdx, startIdx)
	}
}

// Test_marshal_CalendarItem_without_UID_unchanged — существующий снапшот
// Test_marshal_CalendarItem уже покрывает случай без UID (тег omitempty
// исключает поле). Здесь дополнительно страхуем от случайной утечки
// пустого <t:UID></t:UID> в простом сценарии.
func Test_marshal_CalendarItem_without_UID_unchanged(t *testing.T) {
	citem := &CalendarItem{
		Subject:              "no uid",
		ReminderIsSet:        false,
		Start:                time.Time{},
		End:                  time.Time{},
		IsAllDayEvent:        false,
		LegacyFreeBusyStatus: "Busy",
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	assert.NoError(t, err)

	assert.NotContains(t, string(xmlBytes), "<t:UID>")
}

func indexOf(t *testing.T, s, sub string) int {
	t.Helper()
	idx := -1
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return idx
}

