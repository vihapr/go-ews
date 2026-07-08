package ews

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_marshal_GetItem(t *testing.T) {
	req := &GetItem{
		ItemShape: ItemShape{BaseShape: BaseShapeIdOnly},
		ItemIds:   GetItemIds{ItemId: ItemId{Id: "AAA=", ChangeKey: "BBB"}},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	assert.NoError(t, err)

	assert.Equal(t, `<m:GetItem>
  <m:ItemShape>
    <t:BaseShape>IdOnly</t:BaseShape>
  </m:ItemShape>
  <m:ItemIds>
    <t:ItemId Id="AAA=" ChangeKey="BBB"></t:ItemId>
  </m:ItemIds>
</m:GetItem>`, string(xmlBytes))
}

func Test_unmarshal_GetItemResponse_extracts_ChangeKey(t *testing.T) {
	resp := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
                       xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
      <m:ResponseMessages>
        <m:GetItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="BBB"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:GetItemResponseMessage>
      </m:ResponseMessages>
    </m:GetItemResponse>
  </s:Body>
</s:Envelope>`

	var soapResp getItemResponseBodyEnvelop
	err := xml.Unmarshal([]byte(resp), &soapResp)
	assert.NoError(t, err)

	msg := soapResp.Body.GetItemResponse.ResponseMessages.GetItemResponseMessage
	assert.Equal(t, ResponseClassSuccess, msg.ResponseClass)
	assert.Equal(t, "AAA=", msg.Items.CalendarItem.ItemId.Id)
	assert.Equal(t, "BBB", msg.Items.CalendarItem.ItemId.ChangeKey)
}

func Test_unmarshal_GetItemResponse_error(t *testing.T) {
	resp := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages">
      <m:ResponseMessages>
        <m:GetItemResponseMessage ResponseClass="Error">
          <m:MessageText>Item not found.</m:MessageText>
          <m:ResponseCode>ErrorItemNotFound</m:ResponseCode>
        </m:GetItemResponseMessage>
      </m:ResponseMessages>
    </m:GetItemResponse>
  </s:Body>
</s:Envelope>`

	var soapResp getItemResponseBodyEnvelop
	err := xml.Unmarshal([]byte(resp), &soapResp)
	assert.NoError(t, err)

	msg := soapResp.Body.GetItemResponse.ResponseMessages.GetItemResponseMessage
	assert.Equal(t, ResponseClassError, msg.ResponseClass)
	assert.Equal(t, "Item not found.", msg.MessageText)
}
