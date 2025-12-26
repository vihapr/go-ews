package ewsutil

import (
	"github.com/vihapr/go-ews"
)

func DeleteCalendarEvent(
	c ews.Client, id string,
) (string, error) {
	return deleteEvent(c, id)
}

func deleteEvent(
	c ews.Client, id string,
) (string, error) {

	return ews.DeleteCalendarItem(c, id)
}
