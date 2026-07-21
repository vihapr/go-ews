package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- marshal tests ---

func Test_marshal_GetItem_batch(t *testing.T) {
	req := &getItemBatchRequest{
		ItemShape: ItemShape{BaseShape: BaseShapeIdOnly},
		ItemIds: ItemIdsList{ItemId: []ItemId{
			{Id: "AAA="},
			{Id: "BBB="},
		}},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	assert.Contains(t, out, "<m:ItemShape>")
	assert.Contains(t, out, "<t:BaseShape>IdOnly</t:BaseShape>")
	assert.Contains(t, out, "<m:ItemIds>")
	assert.Equal(t, 2, strings.Count(out, "<t:ItemId "))
	assert.Contains(t, out, `Id="AAA="`)
	assert.Contains(t, out, `Id="BBB="`)
}

// --- unmarshal tests ---

const getItemBatchSuccessResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
                       xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
      <m:ResponseMessages>
        <m:GetItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="ck-aaa"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:GetItemResponseMessage>
        <m:GetItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="BBB=" ChangeKey="ck-bbb"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:GetItemResponseMessage>
      </m:ResponseMessages>
    </m:GetItemResponse>
  </s:Body>
</s:Envelope>`

const getItemBatchPartialResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages">
      <m:ResponseMessages>
        <m:GetItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="ck-aaa"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:GetItemResponseMessage>
        <m:GetItemResponseMessage ResponseClass="Error">
          <m:MessageText>Item not found.</m:MessageText>
          <m:ResponseCode>ErrorItemNotFound</m:ResponseCode>
        </m:GetItemResponseMessage>
      </m:ResponseMessages>
    </m:GetItemResponse>
  </s:Body>
</s:Envelope>`

func Test_unmarshal_GetItemResponse_batch_success(t *testing.T) {
	var soapResp getItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(getItemBatchSuccessResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.GetItemResponse.ResponseMessages.GetItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, "ck-aaa", msgs[0].Items.CalendarItem.ItemId.ChangeKey)
	assert.Equal(t, "ck-bbb", msgs[1].Items.CalendarItem.ItemId.ChangeKey)
}

func Test_unmarshal_GetItemResponse_batch_partial(t *testing.T) {
	var soapResp getItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(getItemBatchPartialResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.GetItemResponse.ResponseMessages.GetItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, ResponseClassSuccess, msgs[0].ResponseClass)
	assert.Equal(t, "ck-aaa", msgs[0].Items.CalendarItem.ItemId.ChangeKey)
	assert.Equal(t, ResponseClassError, msgs[1].ResponseClass)
}

// --- end-to-end tests ---

func Test_GetCalendarItemsChangeKeys_success(t *testing.T) {
	fake := &fakeClient{response: []byte(getItemBatchSuccessResponse)}

	keys, failed, err := GetCalendarItemsChangeKeys(context.Background(), fake, []string{"AAA=", "BBB="})
	assert.NoError(t, err)
	assert.Empty(t, failed)
	assert.Equal(t, map[string]string{"AAA=": "ck-aaa", "BBB=": "ck-bbb"}, keys)
}

func Test_GetCalendarItemsChangeKeys_partial(t *testing.T) {
	fake := &fakeClient{response: []byte(getItemBatchPartialResponse)}

	keys, failed, err := GetCalendarItemsChangeKeys(context.Background(), fake, []string{"AAA=", "BBB="})
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"AAA=": "ck-aaa"}, keys)
	assert.Equal(t, []string{"BBB="}, failed)
}

func Test_GetCalendarItemsChangeKeys_limit_exceeded(t *testing.T) {
	ids := make([]string, MaxGetItemsPerRequest+1)
	_, _, err := GetCalendarItemsChangeKeys(context.Background(), &fakeClient{}, ids)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func Test_GetCalendarItemsChangeKeys_empty_input(t *testing.T) {
	keys, failed, err := GetCalendarItemsChangeKeys(context.Background(), &fakeClient{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, keys)
	assert.Empty(t, failed)
}

func Test_GetCalendarItemsChangeKeys_cancelled_context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := GetCalendarItemsChangeKeys(ctx, &fakeClient{}, []string{"AAA="})
	assert.True(t, errors.Is(err, context.Canceled))
}

// Test_GetCalendarItemChangeKey_legacy_wraps_batch — проверяет, что
// существующий сингл-API остался тонкой обёрткой над batch-функцией.
func Test_GetCalendarItemChangeKey_legacy_wraps_batch(t *testing.T) {
	// Singular-ответ по-прежнему парсится через batch-декодер, т.к.
	// []Response с одним элементом эквивалентен singular-структуре.
	fake := &fakeClient{response: []byte(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:GetItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages">
      <m:ResponseMessages>
        <m:GetItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="ck-aaa"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:GetItemResponseMessage>
      </m:ResponseMessages>
    </m:GetItemResponse>
  </s:Body>
</s:Envelope>`)}

	ck, err := GetCalendarItemChangeKey(fake, "AAA=")
	assert.NoError(t, err)
	assert.Equal(t, "ck-aaa", ck)
}
