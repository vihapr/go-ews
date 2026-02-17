package ewsutil

import (
	"time"

	"github.com/vihapr/go-ews"
)

// UpdateEventTime updates only the start and end time of an existing calendar event
// It does not modify subject, body, attachments, location or recipients
// Automatically sends update invitations to all attendees
func UpdateEventTime(
	c ews.Client, id string, start time.Time, end time.Time,
) (string, error) {
	return ews.UpdateCalendarItemTime(c, id, start, end)
}
