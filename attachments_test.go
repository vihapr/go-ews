package ews

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_marshal_CalendarItem_with_Attachments verifies that base64 attachment
// content is serialized verbatim inside <t:Content>, with no XML escaping,
// internal whitespace, or wrapping. EWS expects FileAttachment.Content to be
// the base64-encoded binary, so any modification would corrupt the payload.
func Test_marshal_CalendarItem_with_Attachments(t *testing.T) {
	citem := &CalendarItem{
		Subject: "Test",
		Body:    Body{BodyType: "HTML", Body: []byte("<p>hi</p>")},
		Attachments: &Attachments{
			Attachments: []FileAttachment{
				{
					Name:        "logo.png",
					ContentId:   "logo",
					ContentType: "image/png",
					Content:     "iVBORw0KGgoAAAANSUhEUg==",
				},
				{
					Name:        "empty.txt",
					ContentType: "text/plain",
					Content:     "",
				},
			},
		},
	}

	xmlBytes, err := xml.MarshalIndent(citem, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Base64 content must appear inside <t:Content> verbatim: no escaping,
	// no internal whitespace, no CDATA wrapping. Standard base64 alphabet
	// (A-Z, a-z, 0-9, +, /, =) is XML-safe so encoding/xml leaves it alone.
	assert.Contains(t, out, "<t:Content>iVBORw0KGgoAAAANSUhEUg==</t:Content>")
	assert.Contains(t, out, "<t:ContentId>logo</t:ContentId>")
	assert.Contains(t, out, "<t:Name>logo.png</t:Name>")
	assert.Contains(t, out, "<t:ContentType>image/png</t:ContentType>")

	// omitempty on Content: attachment with empty Content must not emit a
	// <t:Content> element for that FileAttachment.
	assert.Contains(t, out, "<t:Name>empty.txt</t:Name>")
	emptyIdx := strings.Index(out, "<t:Name>empty.txt</t:Name>")
	endIdx := emptyIdx + len("<t:Name>empty.txt</t:Name>")
	closeIdx := strings.Index(out[endIdx:], "</t:FileAttachment>")
	between := out[endIdx : endIdx+closeIdx]
	assert.NotContains(t, between, "<t:Content>",
		"empty Content must be omitted, but found <t:Content> within: %q", between)
}
