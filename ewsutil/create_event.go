package ewsutil

import (
	"time"

	"github.com/vihapr/go-ews"
)

func CreateHTMLEvent(
	c ews.Client, to, optional []string, subject, body, location string, from time.Time, duration time.Duration,
) error {
	return createEvent(c, to, optional, subject, body, location, "HTML", from, duration)
}

// CreateEvent helper method to send Message
func CreateEvent(
	c ews.Client, to, optional []string, subject, body, location string, from time.Time, duration time.Duration,
) error {
	return createEvent(c, to, optional, subject, body, location, "Text", from, duration)
}

func createEvent(
	c ews.Client, to, optional []string, subject, body, location, bodyType string, from time.Time, duration time.Duration,
) error {

	requiredAttendees := make([]ews.Attendee, len(to))
	for i, tt := range to {
		requiredAttendees[i] = ews.Attendee{Mailbox: ews.Mailbox{EmailAddress: tt}}
	}

	optionalAttendees := make([]ews.Attendee, len(optional))
	for i, tt := range optional {
		optionalAttendees[i] = ews.Attendee{Mailbox: ews.Mailbox{EmailAddress: tt}}
	}

	room := make([]ews.Attendee, 1)
	room[0] = ews.Attendee{Mailbox: ews.Mailbox{EmailAddress: location}}

	m := ews.CalendarItem{
		Subject: subject,
		Body: ews.Body{
			BodyType: bodyType,
			Body:     []byte(body),
		},
		ReminderIsSet:              true,
		ReminderMinutesBeforeStart: 15,
		Start:                      from,
		End:                        from.Add(duration),
		IsAllDayEvent:              false,
		LegacyFreeBusyStatus:       ews.BusyTypeBusy,
		Location:                   location,
		RequiredAttendees:          []ews.Attendees{{Attendee: requiredAttendees}},
		OptionalAttendees:          []ews.Attendees{{Attendee: optionalAttendees}},
		Resources:                  []ews.Attendees{{Attendee: room}},
	}

	return ews.CreateCalendarItem(c, m)
}
