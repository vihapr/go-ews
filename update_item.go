package ews

import (
	"encoding/xml"
	"errors"
	"fmt"
	"time"
)

type UpdateItem struct {
	XMLName                               struct{}    `xml:"m:UpdateItem"`
	MessageDisposition                    string      `xml:"MessageDisposition,attr"`
	SendMeetingInvitationsOrCancellations string      `xml:"SendMeetingInvitationsOrCancellations,attr"`
	ConflictResolution                    string      `xml:"ConflictResolution,attr"`
	ItemChanges                           ItemChanges `xml:"m:ItemChanges"`
}

// ItemChanges — обёртка вокруг списка ItemChange.
//
// Когда поле parent'а имеет тег xml:"m:ItemChanges", а сам элемент слайса
// ItemChange несёт собственный XMLName с t:ItemChange, Go-кодер игнорирует
// тег слайса и рендерит каждый <t:ItemChange> напрямую под <m:UpdateItem>,
// пропуская обязательный элемент <m:ItemChanges>. Exchange в таком случае
// отвергает запрос с HTTP 500 (ErrorInvalidArgument, "The request is invalid.").
// Явная обёртка с собственным XMLName заставляет Go выдать корректную структуру
// <m:ItemChanges><t:ItemChange>...</t:ItemChange></m:ItemChanges>.
type ItemChanges struct {
	XMLName struct{}     `xml:"m:ItemChanges"`
	Items   []ItemChange `xml:"t:ItemChange"`
}

type ItemChange struct {
	XMLName struct{} `xml:"t:ItemChange"`
	ItemId  ItemId   `xml:"t:ItemId"`
	Updates Updates  `xml:"t:Updates"`
}

type Updates struct {
	SetItem []SetItemField `xml:"t:SetItemField"`
}

type SetItemField struct {
	FieldURI     FieldURI           `xml:"t:FieldURI"`
	CalendarItem updateCalendarItem `xml:"t:CalendarItem"`
}

// updateCalendarItem содержит только поля, передаваемые внутри SetItemField при
// UpdateItem. Указатели нужны, чтобы omitempty корректно исключал невыбранные
// поля: time.Time — это структура, и без указателя она сериализуется даже с
// нулевым значением (0001-01-01T00:00:00Z), из-за чего Exchange отвергает
// запрос с "The request is invalid."
type updateCalendarItem struct {
	Start *time.Time `xml:"t:Start,omitempty"`
	End   *time.Time `xml:"t:End,omitempty"`
}

type updateItemResponseBodyEnvelop struct {
	XMLName struct{}               `xml:"Envelope"`
	Body    updateItemResponseBody `xml:"Body"`
}

type updateItemResponseBody struct {
	UpdateItemResponse UpdateItemResponse `xml:"UpdateItemResponse"`
}

type UpdateItemResponse struct {
	ResponseMessages UpdateResponseMessages `xml:"ResponseMessages"`
}

type UpdateResponseMessages struct {
	UpdateItemResponseMessage UpdateItemResponseMessage `xml:"UpdateItemResponseMessage"`
}

type UpdateItemResponseMessage struct {
	ResponseClass ResponseClass    `xml:"ResponseClass,attr"`
	MessageText   string          `xml:"MessageText"`
	ResponseCode  string          `xml:"ResponseCode"`
	Items         ResponseItems   `xml:"Items"`
}

// UpdateCalendarItemTime updates only the start and end time of a calendar item
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/updateitem-operation
func UpdateCalendarItemTime(c Client, id string, start, end time.Time) (string, error) {
	// Exchange requires a current ChangeKey on every UpdateItem request. The
	// caller only knows the item ID, so we resolve the ChangeKey via GetItem
	// first. This avoids stale-key errors and the generic
	// "The request is invalid." response Exchange returns when ChangeKey is
	// missing entirely.
	changeKey, err := GetCalendarItemChangeKey(c, id)
	if err != nil {
		return "", fmt.Errorf("resolve change key: %w", err)
	}

	item := &UpdateItem{
		MessageDisposition:                    "SaveOnly",
		SendMeetingInvitationsOrCancellations: "SendToAllAndSaveCopy",
		ConflictResolution:                    "AlwaysOverwrite",
	}

	// Create SetItemField for Start time
	startField := SetItemField{
		FieldURI:     FieldURI{FieldURI: "calendar:Start"},
		CalendarItem: updateCalendarItem{Start: &start},
	}

	// Create SetItemField for End time
	endField := SetItemField{
		FieldURI:     FieldURI{FieldURI: "calendar:End"},
		CalendarItem: updateCalendarItem{End: &end},
	}

	change := ItemChange{
		ItemId: ItemId{Id: id, ChangeKey: changeKey},
		Updates: Updates{
			SetItem: []SetItemField{startField, endField},
		},
	}

	item.ItemChanges = ItemChanges{Items: []ItemChange{change}}

	xmlBytes, err := xml.MarshalIndent(item, "", "  ")
	if err != nil {
		return "", err
	}

	bb, err := c.SendAndReceive(xmlBytes)
	if err != nil {
		return "", err
	}

	if err := checkUpdateItemResponseForErrors(bb); err != nil {
		return "", err
	}

	updatedId, err := getUpdatedItemId(bb)
	if err != nil {
		return "", err
	}

	return updatedId, nil
}

func checkUpdateItemResponseForErrors(bb []byte) error {
	var soapResp updateItemResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return err
	}

	resp := soapResp.Body.UpdateItemResponse.ResponseMessages.UpdateItemResponseMessage
	if resp.ResponseClass == ResponseClassError {
		return errors.New(resp.MessageText)
	}
	return nil
}

func getUpdatedItemId(bb []byte) (string, error) {
	var soapResp updateItemResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return "", err
	}

	resp := soapResp.Body.UpdateItemResponse.ResponseMessages.UpdateItemResponseMessage
	return resp.Items.CalendarItem.ItemId.Id, nil
}
