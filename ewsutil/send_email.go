package ewsutil

import (
	"strings"

	"github.com/vihapr/go-ews"
)

// SendEmail helper method to send Message.
// attachments is variadic so existing callers (without attachments) compile unchanged.
func SendEmail(c ews.Client, to []string, subject, body string, attachments ...ews.FileAttachment) error {
	trimmedBody := strings.TrimSpace(body)
	var bodyType string
	if strings.HasPrefix(trimmedBody, "<") {
		bodyType = "HTML"
	} else {
		bodyType = "Text"
	}

	m := ews.Message{
		ItemClass: "IPM.Note",
		Subject:   subject,
		Body: ews.Body{
			BodyType: bodyType,
			Body:     []byte(body),
		},
		Sender: ews.OneMailbox{
			Mailbox: ews.Mailbox{
				EmailAddress: c.GetUsername(),
			},
		},
	}
	mb := make([]ews.Mailbox, len(to))
	for i, addr := range to {
		mb[i].EmailAddress = addr
	}
	m.ToRecipients.Mailbox = append(m.ToRecipients.Mailbox, mb...)

	if len(attachments) > 0 {
		m.Attachments = &ews.Attachments{Attachments: attachments}
	}

	return ews.CreateMessageItem(c, m)
}
