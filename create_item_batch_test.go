package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- marshal tests ---

// Test_marshal_CreateItem_batch проверяет, что несколько CalendarItem
// попадают в один <m:Items> как повторяющиеся <t:CalendarItem>.
func Test_marshal_CreateItem_batch(t *testing.T) {
	start1, _ := time.Parse(time.RFC3339, "2026-02-19T10:00:00Z")
	end1, _ := time.Parse(time.RFC3339, "2026-02-19T11:00:00Z")
	start2, _ := time.Parse(time.RFC3339, "2026-02-20T14:00:00+03:00")
	end2, _ := time.Parse(time.RFC3339, "2026-02-20T15:00:00+03:00")

	items := []CalendarItem{
		{
			Subject:              "Event A",
			UID:                  "uid-a",
			Start:                start1,
			End:                  end1,
			LegacyFreeBusyStatus: "Busy",
		},
		{
			Subject:              "Event B",
			UID:                  "uid-b",
			Start:                start2,
			End:                  end2,
			LegacyFreeBusyStatus: "OOF",
		},
	}

	req := &CreateItem{
		SendMeetingInvitations: "SendToAllAndSaveCopy",
		SavedItemFolderId:      SavedItemFolderId{DistinguishedFolderId{Id: "calendar"}},
	}
	req.Items.CalendarItem = append(req.Items.CalendarItem, items...)

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Оба события присутствуют.
	assert.Contains(t, out, "<t:Subject>Event A</t:Subject>")
	assert.Contains(t, out, "<t:Subject>Event B</t:Subject>")
	assert.Contains(t, out, "<t:UID>uid-a</t:UID>")
	assert.Contains(t, out, "<t:UID>uid-b</t:UID>")

	// Существенно: ровно два <t:CalendarItem> внутри одного <m:Items>.
	assert.Equal(t, 2, strings.Count(out, "<t:CalendarItem>"))

	// Атрибуты CreateItem сохранены.
	assert.Contains(t, out, `SendMeetingInvitations="SendToAllAndSaveCopy"`)
	assert.Contains(t, out, "<t:DistinguishedFolderId Id=\"calendar\">")
}

// --- unmarshal tests ---

const createItemBatchSuccessResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:CreateItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
                          xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
      <m:ResponseMessages>
        <m:CreateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="aaa"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:CreateItemResponseMessage>
        <m:CreateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="BBB=" ChangeKey="bbb"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:CreateItemResponseMessage>
      </m:ResponseMessages>
    </m:CreateItemResponse>
  </s:Body>
</s:Envelope>`

const createItemBatchPartialResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:CreateItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
                          xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
      <m:ResponseMessages>
        <m:CreateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="aaa"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:CreateItemResponseMessage>
        <m:CreateItemResponseMessage ResponseClass="Error">
          <m:MessageText>At least one recipient isn't valid.</m:MessageText>
          <m:ResponseCode>ErrorInvalidRecipients</m:ResponseCode>
        </m:CreateItemResponseMessage>
        <m:CreateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="CCC=" ChangeKey="ccc"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:CreateItemResponseMessage>
      </m:ResponseMessages>
    </m:CreateItemResponse>
  </s:Body>
</s:Envelope>`

func Test_unmarshal_CreateItemResponse_batch_success(t *testing.T) {
	var soapResp createItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(createItemBatchSuccessResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.CreateItemResponse.ResponseMessages.CreateItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, ResponseClassSuccess, msgs[0].ResponseClass)
	assert.Equal(t, "AAA=", msgs[0].Items.CalendarItem.ItemId.Id)
	assert.Equal(t, "aaa", msgs[0].Items.CalendarItem.ItemId.ChangeKey)
	assert.Equal(t, ResponseClassSuccess, msgs[1].ResponseClass)
	assert.Equal(t, "BBB=", msgs[1].Items.CalendarItem.ItemId.Id)
	assert.Equal(t, "bbb", msgs[1].Items.CalendarItem.ItemId.ChangeKey)
}

func Test_unmarshal_CreateItemResponse_batch_partial(t *testing.T) {
	var soapResp createItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(createItemBatchPartialResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.CreateItemResponse.ResponseMessages.CreateItemResponseMessage
	assert.Len(t, msgs, 3)
	assert.Equal(t, ResponseClassSuccess, msgs[0].ResponseClass)
	assert.Equal(t, "AAA=", msgs[0].Items.CalendarItem.ItemId.Id)

	assert.Equal(t, ResponseClassError, msgs[1].ResponseClass)
	assert.Equal(t, "ErrorInvalidRecipients", msgs[1].ResponseCode)
	assert.Equal(t, "At least one recipient isn't valid.", msgs[1].MessageText)

	assert.Equal(t, ResponseClassSuccess, msgs[2].ResponseClass)
	assert.Equal(t, "CCC=", msgs[2].Items.CalendarItem.ItemId.Id)
}

// --- end-to-end tests via fake client ---

func Test_CreateCalendarItems_success(t *testing.T) {
	fake := &fakeClient{response: []byte(createItemBatchSuccessResponse)}

	items := []CalendarItem{
		{Subject: "A", UID: "uid-a"},
		{Subject: "B", UID: "uid-b"},
	}

	results, err := CreateCalendarItems(context.Background(), fake, items)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	assert.Equal(t, 0, results[0].Index)
	assert.Equal(t, "AAA=", results[0].ItemID)
	assert.Equal(t, "aaa", results[0].ChangeKey)
	assert.NoError(t, results[0].Error)

	assert.Equal(t, 1, results[1].Index)
	assert.Equal(t, "BBB=", results[1].ItemID)
	assert.Equal(t, "bbb", results[1].ChangeKey)
	assert.NoError(t, results[1].Error)
}

func Test_CreateCalendarItems_partial(t *testing.T) {
	fake := &fakeClient{response: []byte(createItemBatchPartialResponse)}

	items := []CalendarItem{
		{Subject: "A"},
		{Subject: "B"},
		{Subject: "C"},
	}
	results, err := CreateCalendarItems(context.Background(), fake, items)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	assert.NoError(t, results[0].Error)
	assert.Equal(t, "AAA=", results[0].ItemID)

	assert.Error(t, results[1].Error)
	assert.Empty(t, results[1].ItemID)
	assert.Contains(t, results[1].Error.Error(), "ErrorInvalidRecipients")

	assert.NoError(t, results[2].Error)
	assert.Equal(t, "CCC=", results[2].ItemID)
}

func Test_CreateCalendarItems_limit_exceeded(t *testing.T) {
	items := make([]CalendarItem, MaxCreateItemsPerRequest+1)
	_, err := CreateCalendarItems(context.Background(), &fakeClient{}, items)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func Test_CreateCalendarItems_empty_input(t *testing.T) {
	results, err := CreateCalendarItems(context.Background(), &fakeClient{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func Test_CreateCalendarItems_http_error(t *testing.T) {
	fake := &fakeClient{err: &HTTPError{Status: "429 Too Many Requests", StatusCode: 429}}
	items := []CalendarItem{{Subject: "A"}}
	results, err := CreateCalendarItems(context.Background(), fake, items)
	assert.Nil(t, results)
	assert.Error(t, err)

	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, 429, httpErr.StatusCode)
}

func Test_CreateCalendarItems_cancelled_context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	items := []CalendarItem{{Subject: "A"}}
	_, err := CreateCalendarItems(ctx, &fakeClient{}, items)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

// --- fake client ---

type fakeClient struct {
	request  []byte
	response []byte
	err      error
	mu       sync.Mutex
}

func (f *fakeClient) SendAndReceive(body []byte) ([]byte, error) {
	return f.SendAndReceiveContext(context.Background(), body)
}

func (f *fakeClient) SendAndReceiveContext(_ context.Context, body []byte) ([]byte, error) {
	f.mu.Lock()
	f.request = body
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.response, nil
}

func (f *fakeClient) GetEWSAddr() string  { return "https://fake/ews" }
func (f *fakeClient) GetUsername() string { return "fake" }

// --- guard: ensure fakeClient satisfies Client after interface extension ---
var _ Client = (*fakeClient)(nil)
var _ Client = (*client)(nil)

// dummy reference to keep net/http import meaningful if extended later
var _ = http.StatusOK
