package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- marshal tests ---

// Test_marshal_UpdateItem_batch_with_explicit_ChangeKeys проверяет, что
// несколько ItemChange попадают в один <m:ItemChanges> и каждый имеет
// свой ChangeKey (без дополнительных GetItem).
func Test_marshal_UpdateItem_batch_with_explicit_ChangeKeys(t *testing.T) {
	start1, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end1, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")
	start2, _ := time.Parse(time.RFC3339, "2026-08-12T00:00:00+03:00")
	end2, _ := time.Parse(time.RFC3339, "2026-08-12T23:59:59+03:00")

	changes := []ItemChange{
		{
			ItemId: ItemId{Id: "AAA=", ChangeKey: "ck-aaa"},
			Updates: Updates{SetItem: []SetItemField{
				{FieldURI: FieldURI{FieldURI: "calendar:Start"}, CalendarItem: updateCalendarItem{Start: &start1}},
				{FieldURI: FieldURI{FieldURI: "calendar:End"}, CalendarItem: updateCalendarItem{End: &end1}},
			}},
		},
		{
			ItemId: ItemId{Id: "BBB=", ChangeKey: "ck-bbb"},
			Updates: Updates{SetItem: []SetItemField{
				{FieldURI: FieldURI{FieldURI: "calendar:Start"}, CalendarItem: updateCalendarItem{Start: &start2}},
				{FieldURI: FieldURI{FieldURI: "calendar:End"}, CalendarItem: updateCalendarItem{End: &end2}},
			}},
		},
	}

	item := &UpdateItem{
		MessageDisposition:                    "SaveOnly",
		SendMeetingInvitationsOrCancellations: "SendToAllAndSaveCopy",
		ConflictResolution:                    "AlwaysOverwrite",
		ItemChanges:                           ItemChanges{Items: changes},
	}

	xmlBytes, err := xml.MarshalIndent(item, "", "  ")
	assert.NoError(t, err)

	out := string(xmlBytes)

	// Wrapper обязателен (см. комментарий в update_item.go).
	assert.Contains(t, out, "<m:ItemChanges>")
	assert.Contains(t, out, "</m:ItemChanges>")
	assert.Equal(t, 2, strings.Count(out, "<t:ItemChange>"))

	// Каждый ItemId имеет свой ChangeKey.
	assert.Contains(t, out, `<t:ItemId Id="AAA=" ChangeKey="ck-aaa"`)
	assert.Contains(t, out, `<t:ItemId Id="BBB=" ChangeKey="ck-bbb"`)
	assert.NotContains(t, out, `ChangeKey=""`)

	// Различные значения времени для каждого элемента.
	assert.Contains(t, out, "2026-08-11T00:00:00+03:00")
	assert.Contains(t, out, "2026-08-12T00:00:00+03:00")
}

// --- unmarshal tests ---

const updateItemBatchSuccessResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:UpdateItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages"
                          xmlns:t="http://schemas.microsoft.com/exchange/services/2006/types">
      <m:ResponseMessages>
        <m:UpdateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="ck-aaa-v2"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:UpdateItemResponseMessage>
        <m:UpdateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="BBB=" ChangeKey="ck-bbb-v2"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:UpdateItemResponseMessage>
      </m:ResponseMessages>
    </m:UpdateItemResponse>
  </s:Body>
</s:Envelope>`

const updateItemBatchPartialResponse = `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <m:UpdateItemResponse xmlns:m="http://schemas.microsoft.com/exchange/services/2006/messages">
      <m:ResponseMessages>
        <m:UpdateItemResponseMessage ResponseClass="Success">
          <m:ResponseCode>NoError</m:ResponseCode>
          <m:Items>
            <t:CalendarItem>
              <t:ItemId Id="AAA=" ChangeKey="ck-aaa-v2"></t:ItemId>
            </t:CalendarItem>
          </m:Items>
        </m:UpdateItemResponseMessage>
        <m:UpdateItemResponseMessage ResponseClass="Error">
          <m:MessageText>One or more properties cannot be updated.</m:MessageText>
          <m:ResponseCode>ErrorInvalidPropertySet</m:ResponseCode>
        </m:UpdateItemResponseMessage>
      </m:ResponseMessages>
    </m:UpdateItemResponse>
  </s:Body>
</s:Envelope>`

func Test_unmarshal_UpdateItemResponse_batch_success(t *testing.T) {
	var soapResp updateItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(updateItemBatchSuccessResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.UpdateItemResponse.ResponseMessages.UpdateItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, "ck-aaa-v2", msgs[0].Items.CalendarItem.ItemId.ChangeKey)
	assert.Equal(t, "ck-bbb-v2", msgs[1].Items.CalendarItem.ItemId.ChangeKey)
}

func Test_unmarshal_UpdateItemResponse_batch_partial(t *testing.T) {
	var soapResp updateItemBatchResponseBodyEnvelop
	err := xml.Unmarshal([]byte(updateItemBatchPartialResponse), &soapResp)
	assert.NoError(t, err)

	msgs := soapResp.Body.UpdateItemResponse.ResponseMessages.UpdateItemResponseMessage
	assert.Len(t, msgs, 2)
	assert.Equal(t, ResponseClassSuccess, msgs[0].ResponseClass)
	assert.Equal(t, ResponseClassError, msgs[1].ResponseClass)
	assert.Equal(t, "ErrorInvalidPropertySet", msgs[1].ResponseCode)
}

// --- end-to-end tests ---

func Test_UpdateCalendarItemsTime_with_explicit_ChangeKey(t *testing.T) {
	fake := &fakeClient{response: []byte(updateItemBatchSuccessResponse)}

	start1, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end1, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")
	start2, _ := time.Parse(time.RFC3339, "2026-08-12T00:00:00+03:00")
	end2, _ := time.Parse(time.RFC3339, "2026-08-12T23:59:59+03:00")

	items := []UpdateCalendarItemInput{
		{ItemID: "AAA=", ChangeKey: "ck-aaa", Start: start1, End: end1},
		{ItemID: "BBB=", ChangeKey: "ck-bbb", Start: start2, End: end2},
	}

	results, err := UpdateCalendarItemsTime(context.Background(), fake, items)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	assert.Equal(t, "AAA=", results[0].ItemID)
	assert.Equal(t, "ck-aaa-v2", results[0].ChangeKey)
	assert.NoError(t, results[0].Error)

	assert.Equal(t, "BBB=", results[1].ItemID)
	assert.Equal(t, "ck-bbb-v2", results[1].ChangeKey)
	assert.NoError(t, results[1].Error)

	// Существенно: при заданных ChangeKey дополнительных GetItem-запросов
	// быть не должно — только один HTTP-запрос UpdateItem.
	assert.Equal(t, 1, fake.callCount())
}

func Test_UpdateCalendarItemsTime_resolves_missing_ChangeKey(t *testing.T) {
	// Сценарий: один элемент с пустым ChangeKey — библиотека должна
	// сначала сделать GetItem, потом UpdateItem.
	sequences := [][]byte{
		[]byte(getItemBatchSuccessResponse), // ответ на GetItem
		[]byte(updateItemBatchSuccessResponse), // ответ на UpdateItem
	}
	fake := &sequenceClient{responses: sequences}

	start, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")

	items := []UpdateCalendarItemInput{
		{ItemID: "AAA=", ChangeKey: "", Start: start, End: end},
		{ItemID: "BBB=", ChangeKey: "ck-bbb", Start: start, End: end},
	}

	results, err := UpdateCalendarItemsTime(context.Background(), fake, items)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	// Был ровно один GetItem + один UpdateItem.
	assert.Equal(t, 2, fake.calls)

	// В UpdateItem-запросе резолвнутый ChangeKey для AAA присутствует.
	assert.Contains(t, string(fake.requests[1]), `<t:ItemId Id="AAA=" ChangeKey="ck-aaa"`)
}

func Test_UpdateCalendarItemsTime_partial(t *testing.T) {
	fake := &fakeClient{response: []byte(updateItemBatchPartialResponse)}

	start, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")

	items := []UpdateCalendarItemInput{
		{ItemID: "AAA=", ChangeKey: "ck-aaa", Start: start, End: end},
		{ItemID: "BBB=", ChangeKey: "ck-bbb", Start: start, End: end},
	}

	results, err := UpdateCalendarItemsTime(context.Background(), fake, items)
	assert.NoError(t, err)
	assert.Len(t, results, 2)

	assert.NoError(t, results[0].Error)
	assert.Equal(t, "ck-aaa-v2", results[0].ChangeKey)

	assert.Error(t, results[1].Error)
	assert.Contains(t, results[1].Error.Error(), "ErrorInvalidPropertySet")
}

func Test_UpdateCalendarItemsTime_failed_to_resolve_ChangeKey(t *testing.T) {
	// GetItem вернул ErrorItemNotFound — весь батч должен быть отменён.
	fake := &fakeClient{response: []byte(getItemBatchPartialResponse)}

	start, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")

	items := []UpdateCalendarItemInput{
		{ItemID: "AAA=", ChangeKey: "", Start: start, End: end},
		{ItemID: "BBB=", ChangeKey: "", Start: start, End: end},
	}

	results, err := UpdateCalendarItemsTime(context.Background(), fake, items)
	assert.Nil(t, results)
	assert.Error(t, err)
	// Батч отменён при неудачном резолве ChangeKey; проверяем по
	// характерной фразе из любой из двух ошибок в этой ветке.
	msg := err.Error()
	assert.True(t,
		strings.Contains(msg, "resolve change keys") ||
			strings.Contains(msg, "failed to resolve ChangeKey"),
		"expected ChangeKey resolution failure, got: %s", msg)
}

func Test_UpdateCalendarItemsTime_limit_exceeded(t *testing.T) {
	items := make([]UpdateCalendarItemInput, MaxUpdateItemsPerRequest+1)
	_, err := UpdateCalendarItemsTime(context.Background(), &fakeClient{}, items)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds limit")
}

func Test_UpdateCalendarItemsTime_empty_input(t *testing.T) {
	results, err := UpdateCalendarItemsTime(context.Background(), &fakeClient{}, nil)
	assert.NoError(t, err)
	assert.Empty(t, results)
}

func Test_UpdateCalendarItemsTime_http_error(t *testing.T) {
	fake := &fakeClient{err: &HTTPError{Status: "429 Too Many Requests", StatusCode: 429}}
	start, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")
	items := []UpdateCalendarItemInput{
		{ItemID: "AAA=", ChangeKey: "ck-aaa", Start: start, End: end},
	}
	results, err := UpdateCalendarItemsTime(context.Background(), fake, items)
	assert.Nil(t, results)
	var httpErr *HTTPError
	assert.True(t, errors.As(err, &httpErr))
	assert.Equal(t, 429, httpErr.StatusCode)
}

func Test_UpdateCalendarItemsTime_cancelled_context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start, _ := time.Parse(time.RFC3339, "2026-08-11T00:00:00+03:00")
	end, _ := time.Parse(time.RFC3339, "2026-08-11T23:59:59+03:00")
	items := []UpdateCalendarItemInput{
		{ItemID: "AAA=", ChangeKey: "ck-aaa", Start: start, End: end},
	}
	_, err := UpdateCalendarItemsTime(ctx, &fakeClient{}, items)
	assert.True(t, errors.Is(err, context.Canceled))
}

// --- helpers: count + sequence ---

func (f *fakeClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.request == nil {
		return 0
	}
	return 1
}

// sequenceClient возвращает заготовленные ответы по очереди. Запоминает
// тела запросов для последующих assert'ов в тестах.
type sequenceClient struct {
	responses [][]byte
	requests  [][]byte
	calls     int
	err       error
}

func (s *sequenceClient) SendAndReceive(body []byte) ([]byte, error) {
	return s.SendAndReceiveContext(context.Background(), body)
}

func (s *sequenceClient) SendAndReceiveContext(_ context.Context, body []byte) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.calls >= len(s.responses) {
		return nil, errors.New("sequenceClient: out of canned responses")
	}
	s.requests = append(s.requests, body)
	resp := s.responses[s.calls]
	s.calls++
	return resp, nil
}

func (s *sequenceClient) GetEWSAddr() string  { return "https://fake/ews" }
func (s *sequenceClient) GetUsername() string { return "fake" }

var _ Client = (*sequenceClient)(nil)
