package ews

import "encoding/xml"

type GetRoomListsRequest struct {
	XMLName struct{} `xml:"m:GetRoomLists"`
}

type GetRoomListsResponse struct {
	Response
	RoomLists RoomLists `xml:"RoomLists"`
}

type RoomLists struct {
	Address []EmailAddress `xml:"Address"`
}

type ItemId struct {
	Id        string `xml:"Id,attr"`
	ChangeKey string `xml:"ChangeKey,attr,omitempty"`
}

type getRoomListsResponseEnvelop struct {
	XMLName struct{}                 `xml:"Envelope"`
	Body    getRoomListsResponseBody `xml:"Body"`
}
type getRoomListsResponseBody struct {
	GetRoomListsResponse GetRoomListsResponse `xml:"GetRoomListsResponse"`
}

func GetRoomLists(c Client) (*GetRoomListsResponse, error) {

	xmlBytes, err := xml.MarshalIndent(&GetRoomListsRequest{}, "", "  ")
	if err != nil {
		return nil, err
	}

	bb, err := c.SendAndReceive(xmlBytes)
	if err != nil {
		return nil, err
	}

	var soapResp getRoomListsResponseEnvelop
	err = xml.Unmarshal(bb, &soapResp)
	if err != nil {
		return nil, err
	}

	return &soapResp.Body.GetRoomListsResponse, nil
}
