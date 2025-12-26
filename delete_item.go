package ews

import (
	"encoding/xml"
	"errors"
)

type DeleteItem struct {
	XMLName                  struct{} `xml:"m:DeleteItem"`
	DeleteType               string   `xml:"DeleteType,attr"`
	SendMeetingCancellations string   `xml:"SendMeetingCancellations,attr"`
	ItemIds                  ItemIds  `xml:"m:ItemIds"`
}

type ItemIds struct {
	ItemId DeleteItemId `xml:"t:ItemId"`
}

type DeleteItemId struct {
	Id string `xml:"Id,attr"`
}

type deleteItemResponseBodyEnvelop struct {
	XMLName struct{}               `xml:"Envelope"`
	Body    deleteItemResponseBody `xml:"Body"`
}

type deleteItemResponseBody struct {
	DeleteItemResponse DeleteItemResponse `xml:"DeleteItemResponse"`
}

type DeleteItemResponse struct {
	ResponseMessages DeleteResponseMessages `xml:"ResponseMessages"`
}

type DeleteResponseMessages struct {
	DeleteItemResponseMessage DeleteItemResponseMessage `xml:"DeleteItemResponseMessage"`
}

type DeleteItemResponseMessage struct {
	DeleteResponseClass ResponseClass `xml:"ResponseClass,attr"`
	MessageText         string        `xml:"MessageText"`
	ResponseCode        string        `xml:"ResponseCode"`
}

// DeleteItem operation
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/deleteitem-operation
func DeleteCalendarItem(c Client, id string) (string, error) {

	item := &DeleteItem{
		DeleteType:               "HardDelete",
		SendMeetingCancellations: "SendOnlyToAll",
	}
	item.ItemIds.ItemId.Id = id

	xmlBytes, err := xml.MarshalIndent(item, "", "  ")
	if err != nil {
		return "", err
	}

	bb, err := c.SendAndReceive(xmlBytes)
	if err != nil {
		return "", err
	}

	if err := checkDeleteItemResponseForErrors(bb); err != nil {
		return "", err
	}

	return id, nil
}

func checkDeleteItemResponseForErrors(bb []byte) error {
	var soapResp deleteItemResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return err
	}

	resp := soapResp.Body.DeleteItemResponse.ResponseMessages.DeleteItemResponseMessage
	if resp.DeleteResponseClass == ResponseClassError {
		return errors.New(resp.MessageText)
	}
	return nil
}
