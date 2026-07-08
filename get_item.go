package ews

import (
	"encoding/xml"
	"errors"
)

// GetItem represents a SOAP request for the GetItem operation.
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/getitem-operation
type GetItem struct {
	XMLName   struct{}   `xml:"m:GetItem"`
	ItemShape ItemShape  `xml:"m:ItemShape"`
	ItemIds   GetItemIds `xml:"m:ItemIds"`
}

// ItemShape describes the set of properties to return for items in GetItem/FindItem responses.
type ItemShape struct {
	BaseShape BaseShape `xml:"t:BaseShape"`
}

// GetItemIds wraps the list of ItemId elements for a GetItem request.
type GetItemIds struct {
	ItemId ItemId `xml:"t:ItemId"`
}

type getItemResponseBodyEnvelop struct {
	XMLName struct{}            `xml:"Envelope"`
	Body    getItemResponseBody `xml:"Body"`
}

type getItemResponseBody struct {
	GetItemResponse GetItemResponse `xml:"GetItemResponse"`
}

// GetItemResponse is the top-level body of a GetItem SOAP response.
type GetItemResponse struct {
	ResponseMessages GetItemResponseMessages `xml:"ResponseMessages"`
}

// GetItemResponseMessages wraps the per-item response messages.
type GetItemResponseMessages struct {
	GetItemResponseMessage GetItemResponseMessage `xml:"GetItemResponseMessage"`
}

// GetItemResponseMessage carries the result for a single requested item.
type GetItemResponseMessage struct {
	ResponseClass ResponseClass `xml:"ResponseClass,attr"`
	MessageText   string        `xml:"MessageText"`
	ResponseCode  string        `xml:"ResponseCode"`
	Items         ResponseItems `xml:"Items"`
}

// GetCalendarItemChangeKey resolves the current ChangeKey of a calendar item
// by its ID via the GetItem operation. Exchange requires a valid ChangeKey on
// UpdateItem requests; use this helper to obtain one when only the item ID is
// known.
//
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/getitem-operation
func GetCalendarItemChangeKey(c Client, id string) (string, error) {
	req := &GetItem{
		ItemShape: ItemShape{BaseShape: BaseShapeIdOnly},
		ItemIds:   GetItemIds{ItemId: ItemId{Id: id}},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	if err != nil {
		return "", err
	}

	bb, err := c.SendAndReceive(xmlBytes)
	if err != nil {
		return "", err
	}

	var soapResp getItemResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return "", err
	}

	msg := soapResp.Body.GetItemResponse.ResponseMessages.GetItemResponseMessage
	if msg.ResponseClass == ResponseClassError {
		return "", errors.New(msg.MessageText)
	}

	changeKey := msg.Items.CalendarItem.ItemId.ChangeKey
	if changeKey == "" {
		return "", errors.New("exchange returned empty ChangeKey for item " + id)
	}
	return changeKey, nil
}
