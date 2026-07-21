package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
)

// MaxCreateItemsPerRequest — лимит числа элементов в одном EWS CreateItem.
// Exchange документирует потолок 50; больше — ErrorTooManyObjectsOpened.
const MaxCreateItemsPerRequest = 50

// CreateItemResult — результат одного элемента batch-Create.
//
// Index соответствует позиции входного CalendarItem в слайсе, переданном
// в CreateCalendarItems. При успешном создании ItemID/ChangeKey заполнены,
// Error == nil. При неудаче ItemID/ChangeKey пустые, Error описывает причину.
type CreateItemResult struct {
	Index     int
	ItemID    string
	ChangeKey string
	Error     error
}

// CreateCalendarItems создаёт несколько событий в одном SOAP-запросе.
//
// Возвращает по одному CreateItemResult на каждый входной CalendarItem
// в том же порядке. Частичные ошибки (один ResponseClass="Error" в батче)
// не заваливают весь вызов: такие элементы возвращаются с Error != nil,
// остальные — со штатными ItemID/ChangeKey.
//
// Второе возвращаемое значение (top-level error) непусто только при:
//   - ошибке маршалинга запроса или анмаршалинга ответа;
//   - HTTP ≠ 200 (проброс *ews.HTTPError / *ews.SoapError — almanac делает
//     ретраи);
//   - превышении лимита MaxCreateItemsPerRequest;
//   - пустом конверте ответа (ни одного CreateItemResponseMessage);
//   - отмене контекста.
//
// В этих случаях []CreateItemResult — nil.
//
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/createitem-operation
func CreateCalendarItems(ctx context.Context, c Client, items []CalendarItem) ([]CreateItemResult, error) {
	if len(items) == 0 {
		return []CreateItemResult{}, nil
	}
	if len(items) > MaxCreateItemsPerRequest {
		return nil, fmt.Errorf("go-ews: CreateCalendarItems: %d items exceeds limit %d", len(items), MaxCreateItemsPerRequest)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	req := &CreateItem{
		SendMeetingInvitations: "SendToAllAndSaveCopy",
		SavedItemFolderId:      SavedItemFolderId{DistinguishedFolderId{Id: "calendar"}},
	}
	req.Items.CalendarItem = append(req.Items.CalendarItem, items...)

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, err
	}

	bb, err := c.SendAndReceiveContext(ctx, xmlBytes)
	if err != nil {
		return nil, err
	}

	var soapResp createItemBatchResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return nil, err
	}

	msgs := soapResp.Body.CreateItemResponse.ResponseMessages.CreateItemResponseMessage
	if len(msgs) == 0 {
		return nil, errors.New("go-ews: CreateCalendarItems: empty response (no CreateItemResponseMessage)")
	}

	results := make([]CreateItemResult, len(items))
	for i := range items {
		if i >= len(msgs) {
			results[i] = CreateItemResult{
				Index: i,
				Error: fmt.Errorf("go-ews: CreateCalendarItems: missing response message at index %d", i),
			}
			continue
		}
		msg := msgs[i]
		if msg.ResponseClass == ResponseClassError {
			results[i] = CreateItemResult{
				Index: i,
				Error: fmt.Errorf("go-ews: CreateCalendarItems[%d]: %s (%s)", i, msg.MessageText, msg.ResponseCode),
			}
			continue
		}
		if msg.ResponseClass == ResponseClassWarning {
			// Warning: item создан, но сервер сообщает о проблеме (например,
			// пропуск一部分 аттачментов). Возвращаем ItemID, ошибку не выставляем.
		}
		results[i] = CreateItemResult{
			Index:     i,
			ItemID:    msg.Items.CalendarItem.ItemId.Id,
			ChangeKey: msg.Items.CalendarItem.ItemId.ChangeKey,
		}
	}
	return results, nil
}

// createItemBatchResponseBodyEnvelop — отдельный парсер для batch-response.
// Существующий createItemResponseBodyEnvelop (singular) не трогаем — он
// используется функциями CreateMessageItem / CreateCalendarItem.
type createItemBatchResponseBodyEnvelop struct {
	XMLName struct{}                    `xml:"Envelope"`
	Body    createItemBatchResponseBody `xml:"Body"`
}

type createItemBatchResponseBody struct {
	CreateItemResponse createItemBatchResponse `xml:"CreateItemResponse"`
}

type createItemBatchResponse struct {
	ResponseMessages createItemBatchResponseMessages `xml:"ResponseMessages"`
}

type createItemBatchResponseMessages struct {
	CreateItemResponseMessage []Response `xml:"CreateItemResponseMessage"`
}
