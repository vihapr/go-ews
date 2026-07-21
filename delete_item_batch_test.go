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

func Test_marshal_DeleteItem_batch(t *testing.T) {
	req := &deleteItemBatchRequest{
		DeleteType:               "HardDelete",
		SendMeetingCancellations: "SendOnlyToAll",
		ItemIds: ItemIdsList{ItemId: []ItemId{
			{Id: "AAA="},
			{Id: "BBB="},
			{Id: "CCC="},
		}},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	assert.Contains(t, out, `DeleteType="HardDelete"`)
	assert.Contains(t, out, `SendMeetingCancellations="SendOnlyToAll"`)

	// Все три ItemId присутствуют под одним <m:ItemIds>.
	assert.Contains(t, out, `<m:ItemIds>`)
	assert.Equal(t, 3, strings.Count(out, `<t:ItemId `))
	assert.Contains(t, out, `Id="AAA="`)
	assert.Contains(t, out, `Id="BBB="`)
	assert.Contains(t, out, `Id="CCC="`)
}

// --- unmarshal tests ---

const deleteItemBatchSuccessResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:DeleteItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages">
      <m:ResponseMessages>
        <m:DeleteItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
        </m:DeleteItemResponseMessage>
        <m:DeleteItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
        </m:DeleteItemResponseMessage>
      </m:ResponseMessages>
    </m:DeleteItemResponse>
  </s:Body>
</s:Envelope>`

const deleteItemBatchPartialResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:DeleteItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages">
      <m:ResponseMessages>
        <m:DeleteItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
        </m:DeleteItemResponseMessage>
        <m:DeleteItemResponseMessage ResponseClass="Error">
          <m:MessageText>The item wasn't found.</m:MessageText>
          <m:ResponseCode>ErrorItemNotFound</m:ResponseCode>
        </m:DeleteItemResponseMessage>
      </m:ResponseMessages>
    </m:DeleteItemResponse>
  </s:Body>
</s:Envelope>`

func Test_unmarshal_DeleteItemResponse_batch_success(t *testing.T) {
	var soapResp deleteItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(deleteItemBatchSuccessResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.DeleteItemResponse.ResponseMessages.DeleteItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, ResponseClassSuccess, msgs[0].ResponseClass)
	assert.Equal(t, ResponseClassSuccess, msgs[1].ResponseClass)
}

func Test_unmarshal_DeleteItemResponse_batch_partial(t *testing.T) {
	var soapResp deleteItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(deleteItemBatchPartialResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.DeleteItemResponse.ResponseMessages.DeleteItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, ResponseClassSuccess, msgs[0].ResponseClass)
	assert.Equal(t, ResponseClassError, msgs[1].ResponseClass)
	assert.Equal(t, "ErrorItemNotFound", msgs[1].ResponseCode)
}

// --- end-to-end tests ---

func Test_DeleteCalendarItems_success(t *testing.T) {
	fake := &fakeClient{response: []byte(deleteItemBatchSuccessResponse)}

	results, err := DeleteCalendarItems(context.Background(), fake, []string{"AAA=", "BBB="})
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "AAA=", results[0].ItemID)
	assert.Equal(t, "BBB=", results[1].ItemID)
	assert.NoError(t, results[0].Error)
	assert.NoError(t, results[1].Error)
}

func Test_DeleteCalendarItems_partial(t *testing.T) {
	fake := &fakeClient{response: []byte(deleteItemBatchPartialResponse)}

	results, err := DeleteCalendarItems(context.Background(), fake, []string{"AAA=", "BBB="})
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	assert.NoError(t, results[0].Error)
	assert.Equal(t, "AAA=", results[0].ItemID)

	assert.Error(t, results[1].Error)
	assert.Equal(t, "BBB=", results[1].ItemID) // эхо входного id
	assert.Contains(t, results[1].Error.Error(), "ErrorItemNotFound")
}

func Test_DeleteCalendarItems_limit_exceeded(t *testing.T) {
	ids := make([]string, MaxDeleteItemsPerRequest+1)
	_, err := DeleteCalendarItems(context.Background(), &fakeClient{}, ids)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func Test_DeleteCalendarItems_empty_input(t *testing.T) {
	results, err := DeleteCalendarItems(context.Background(), &fakeClient{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func Test_DeleteCalendarItems_http_error(t *testing.T) {
	fake := &fakeClient{err: &HTTPError{Status: "500 Internal Server Error", StatusCode: 500}}
	results, err := DeleteCalendarItems(context.Background(), fake, []string{"AAA="})
	assert.Nil(t, results)
	assert.Error(t, err)

	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, 500, httpErr.StatusCode)
}

func Test_DeleteCalendarItems_cancelled_context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := DeleteCalendarItems(ctx, &fakeClient{}, []string{"AAA="})
	assert.True(t, errors.Is(err, context.Canceled))
}
