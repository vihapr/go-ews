package ews

// MeetingTimeZone задаёт часовой пояс, в котором интерпретируются Start/End
// календарного события (Exchange 2007 SP1+).
//
// Без этого элемента (и без пары StartTimeZone/EndTimeZone) EWS «прилипает» к
// полуночи в часовом поясе сервисного аккаунта (часто UTC). Для all-day
// событий это ломает отображение в Outlook: пользователь в MSK видит «03:00»
// вместо честного полного дня.
//
// Поле Name принимает Microsoft timezone names (например "Russian Standard
// Time"); в этом случае Bias/StandardTime/DaylightTime можно не задавать —
// Exchange резолвит их по имени.
//
// Deprecated с Exchange 2010 в пользу StartTimeZone/EndTimeZone, но
// продолжает работать на всех актуальных версиях Exchange.
type MeetingTimeZone struct {
	Name string `xml:"Name,attr,omitempty"`
}

// TimeZoneDefinition — современное (Exchange 2010+) описание часового пояса
// для StartTimeZone/EndTimeZone. Атрибут Id принимает Microsoft timezone
// names (например "Russian Standard Time"). Name — человекочитаемая подпись,
// Exchange её не интерпретирует.
type TimeZoneDefinition struct {
	Id   string `xml:"Id,attr,omitempty"`
	Name string `xml:"Name,attr,omitempty"`
}
