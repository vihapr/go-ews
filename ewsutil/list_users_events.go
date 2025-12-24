package ewsutil

import (
	"time"

	"github.com/vihapr/go-ews"
)

type EventUser struct {
	Email        string
	AttendeeType ews.AttendeeType
}

type Event struct {
	Start    time.Time
	End      time.Time
	BusyType ews.BusyType
}

func ListUsersEvents(
	c ews.Client, eventUsers []EventUser, from time.Time, duration time.Duration,
) (map[EventUser][]Event, error) {

	req := buildGetUserAvailabilityRequest(eventUsers, from, duration)

	resp, err := ews.GetUserAvailability(c, req)
	if err != nil {
		return nil, err
	}

	events, err := traverseGetUserAvailabilityResponse(eventUsers, resp)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func buildGetUserAvailabilityRequest(
	eventUsers []EventUser, from time.Time, duration time.Duration,
) *ews.GetUserAvailabilityRequest {

	mb := make([]ews.MailboxData, 0)
	for _, mm := range eventUsers {
		mb = append(mb, ews.MailboxData{
			Email: ews.Email{
				Name:        "",
				Address:     mm.Email,
				RoutingType: "SMTP",
			},
			AttendeeType:     mm.AttendeeType,
			ExcludeConflicts: false,
		})
	}
	_, offset := time.Now().Zone()
	req := &ews.GetUserAvailabilityRequest{
		//https://github.com/MicrosoftDocs/office-developer-exchange-docs/issues/61
		TimeZone: ews.TimeZone{
			Bias: -offset / 60,
			StandardTime: ews.TimeZoneTime{ // I don't have much clue about the values here
				Bias:      0,
				Time:      "02:00:00",
				DayOrder:  5,
				Month:     10,
				DayOfWeek: "Sunday",
			},
			DaylightTime: ews.TimeZoneTime{ // I don't have much clue about the values here
				Bias:      0,
				Time:      "02:00:00",
				DayOrder:  1,
				Month:     4,
				DayOfWeek: "Sunday",
			},
		},
		MailboxDataArray: ews.MailboxDataArray{MailboxData: mb},
		FreeBusyViewOptions: ews.FreeBusyViewOptions{
			TimeWindow: ews.TimeWindow{
				StartTime: from,
				EndTime:   from.Add(duration),
			},
			RequestedView: ews.RequestedViewFreeBusy,
		},
	}
	return req
}

func traverseGetUserAvailabilityResponse(
	eventUsers []EventUser, resp *ews.GetUserAvailabilityResponse,
) (map[EventUser][]Event, error) {

	m := make(map[EventUser][]Event)
	for i, rr := range resp.FreeBusyResponseArray.FreeBusyResponse {

		ce := make([]Event, 0)
		for _, cc := range rr.FreeBusyView.CalendarEventArray.CalendarEvent {

			start, err := cc.StartTime.ToTime()
			if err != nil {
				return nil, err
			}

			end, err := cc.EndTime.ToTime()
			if err != nil {
				return nil, err
			}

			ce = append(ce, Event{
				Start:    start,
				End:      end,
				BusyType: cc.BusyType,
			})
		}
		m[eventUsers[i]] = ce
	}
	return m, nil
}
