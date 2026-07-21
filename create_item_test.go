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

// Test_marshal_CalendarItem_with_MeetingTimeZone проверяет, что заполненный
// MeetingTimeZone сериализуется как самозакрывающийся тег с атрибутом Name и
// располагается в схемной позиции CalendarItemType — между UID и Start.
func Test_marshal_CalendarItem_with_MeetingTimeZone(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-09-01T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-09-03T00:00:00+03:00")

	citem := &CalendarItem{
		Subject:                    "Absence",
		ReminderIsSet:              false,
		ReminderMinutesBeforeStart: 0,
		UID:                        "almanac-abc@x5.ru",
		MeetingTimeZone:            &MeetingTimeZone{Name: "Russian Standard Time"},
		Start:                      start,
		End:                        end,
		IsAllDayEvent:              true,
		LegacyFreeBusyStatus:       "OOF",
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Открывающий тег с атрибутом Name. Go-маршаллер для struct-only-attrs
	// выдаёт `<t:MeetingTimeZone Name="..."></t:MeetingTimeZone>`, что
	// XML-эквивалентно самозакрывающемуся `<.../>` — Exchange принимает оба.
	assert.Contains(t, out, `<t:MeetingTimeZone Name="Russian Standard Time"`)

	// Схемная позиция: после <t:UID> и до <t:Start>.
	uidIdx := indexOf(t, out, "<t:UID>")
	mtzIdx := indexOf(t, out, "<t:MeetingTimeZone")
	startIdx := indexOf(t, out, "<t:Start>")
	if uidIdx >= mtzIdx || mtzIdx >= startIdx {
		t.Fatalf("MeetingTimeZone must be between UID and Start; got UID=%d MTZ=%d Start=%d",
			uidIdx, mtzIdx, startIdx)
	}
}

// Test_marshal_CalendarItem_with_StartEndTimeZone проверяет современную пару
// StartTimeZone/EndTimeZone (Exchange 2010+). Оба элемента должны идти подряд,
// в позиции между UID и Start.
func Test_marshal_CalendarItem_with_StartEndTimeZone(t *testing.T) {
	start, _ := time.Parse(time.RFC3339, "2026-09-01T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-09-03T00:00:00+03:00")

	citem := &CalendarItem{
		Subject:                    "Absence",
		ReminderIsSet:              false,
		ReminderMinutesBeforeStart: 0,
		UID:                        "almanac-abc@x5.ru",
		StartTimeZone:              &TimeZoneDefinition{Id: "Russian Standard Time", Name: "(UTC+03:00) Moscow, St. Petersburg"},
		EndTimeZone:                &TimeZoneDefinition{Id: "Russian Standard Time", Name: "(UTC+03:00) Moscow, St. Petersburg"},
		Start:                      start,
		End:                        end,
		IsAllDayEvent:              true,
		LegacyFreeBusyStatus:       "OOF",
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Оба элемента с обоими атрибутами. См. комментарий выше про формат
	// open+close vs self-closing — проверяем только открывающий тег.
	assert.Contains(t, out, `<t:StartTimeZone Id="Russian Standard Time" Name="(UTC+03:00) Moscow, St. Petersburg"`)
	assert.Contains(t, out, `<t:EndTimeZone Id="Russian Standard Time" Name="(UTC+03:00) Moscow, St. Petersburg"`)

	// Схемная позиция: после <t:UID> и до <t:Start>, и StartTimeZone перед EndTimeZone.
	uidIdx := indexOf(t, out, "<t:UID>")
	stzIdx := indexOf(t, out, "<t:StartTimeZone")
	etzIdx := indexOf(t, out, "<t:EndTimeZone")
	startIdx := indexOf(t, out, "<t:Start>")
	if uidIdx >= stzIdx || stzIdx >= etzIdx || etzIdx >= startIdx {
		t.Fatalf("StartTimeZone/EndTimeZone must be between UID and Start in order; "+
			"got UID=%d STZ=%d ETZ=%d Start=%d", uidIdx, stzIdx, etzIdx, startIdx)
	}
}

// Test_marshal_CalendarItem_without_TimeZoneFields_unchanged страхует от
// утечки пустых <t:MeetingTimeZone/> или <t:StartTimeZone/> в простых сценариях
// — это критично для обратной совместимости с существующими потребителями
// библиотеки (например, notificator на старых релизах).
func Test_marshal_CalendarItem_without_TimeZoneFields_unchanged(t *testing.T) {
	citem := &CalendarItem{
		Subject:              "no tz",
		ReminderIsSet:        false,
		Start:                time.Time{},
		End:                  time.Time{},
		IsAllDayEvent:        false,
		LegacyFreeBusyStatus: "Busy",
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)
	assert.NotContains(t, out, "<t:MeetingTimeZone")
	assert.NotContains(t, out, "<t:StartTimeZone")
	assert.NotContains(t, out, "<t:EndTimeZone")
}
