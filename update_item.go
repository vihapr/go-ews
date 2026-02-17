package ews

import (
	"encoding/xml"
	"errors"
	"time"
)

type UpdateItem struct {
	XMLName                struct{}   `xml:"m:UpdateItem"`
	MessageDisposition     string     `xml:"MessageDisposition,attr"`
	SendMeetingInvitations string     `xml:"SendMeetingInvitations,attr"`
	ConflictResolution     string     `xml:"ConflictResolution,attr"`
	ItemChanges            []ItemChange `xml:"m:ItemChanges"`
}

type ItemChange struct {
	ItemId     ItemId  `xml:"t:ItemId"`
	Updates    Updates `xml:"t:Updates"`
}

type Updates struct {
	SetItem []SetItemField `xml:"t:SetItemField"`
}

type SetItemField struct {
	FieldURI     FieldURI     `xml:"t:FieldURI"`
	CalendarItem CalendarItem `xml:"t:CalendarItem"`
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

	item := &UpdateItem{
		MessageDisposition:     "SaveOnly",
		SendMeetingInvitations: "SendToAllAndSaveCopy",
		ConflictResolution:     "AlwaysOverwrite",
	}

	// Create SetItemField for Start time
	startField := SetItemField{
		FieldURI: FieldURI{FieldURI: "calendar:Start"},
		CalendarItem: CalendarItem{
			Start: start,
		},
	}

	// Create SetItemField for End time
	endField := SetItemField{
		FieldURI: FieldURI{FieldURI: "calendar:End"},
		CalendarItem: CalendarItem{
			End: end,
		},
	}

	change := ItemChange{
		ItemId: ItemId{Id: id},
		Updates: Updates{
			SetItem: []SetItemField{startField, endField},
		},
	}

	item.ItemChanges = []ItemChange{change}

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
