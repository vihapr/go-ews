package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
)

// MaxDeleteItemsPerRequest — лимит числа элементов в одном EWS DeleteItem.
// Exchange документирует потолок 50 (общий лимит на batch-операцию).
const MaxDeleteItemsPerRequest = 50

// DeleteItemResult — результат одного элемента batch-Delete.
//
// ItemID — эхо входного id (Exchange не возвращает новых id для Delete).
// При неудаче Error описывает причину (например, ErrorItemNotFound для
// уже удалённого события).
type DeleteItemResult struct {
	Index  int
	ItemID string
	Error  error
}

// ItemIdsList — plural-версия ItemIds для batch-операций (Delete, Get).
//
// Существующий ItemIds (singular) остаётся для DeleteCalendarItem(c, id)
// без изменений; см. spec §2.4.
type ItemIdsList struct {
	XMLName struct{} `xml:"m:ItemIds"`
	ItemId  []ItemId `xml:"t:ItemId"`
}

// deleteItemBatchRequest — запрос DeleteItem с несколькими ItemId.
// Отдельная структура от DeleteItem (singular), чтобы не ломать
// существующий DeleteCalendarItem(c, id) и его сериализацию.
type deleteItemBatchRequest struct {
	XMLName                  struct{}    `xml:"m:DeleteItem"`
	DeleteType               string      `xml:"DeleteType,attr"`
	SendMeetingCancellations string      `xml:"SendMeetingCancellations,attr"`
	ItemIds                  ItemIdsList `xml:"m:ItemIds"`
}

// DeleteCalendarItems удаляет несколько событий в одном SOAP-запросе.
//
// Возвращает по одному DeleteItemResult на каждый входной id в том же
// порядке. Частичные ошибки (один ResponseClass="Error" в батче, например
// ErrorItemNotFound для уже удалённого) не заваливают весь вызов —
// такие элементы возвращаются с Error != nil, остальные с пустым Error.
//
// Top-level error непусто при ошибке маршалинга, HTTP ≠ 200, пустом
// конверте, превышении лимита или отмене контекста — в этих случаях
// []DeleteItemResult — nil. См. CreateCalendarItems для подробностей.
//
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/deleteitem-operation
func DeleteCalendarItems(ctx context.Context, c Client, ids []string) ([]DeleteItemResult, error) {
	if len(ids) == 0 {
		return []DeleteItemResult{}, nil
	}
	if len(ids) > MaxDeleteItemsPerRequest {
		return nil, fmt.Errorf("go-ews: DeleteCalendarItems: %d ids exceeds limit %d", len(ids), MaxDeleteItemsPerRequest)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	itemIds := make([]ItemId, 0, len(ids))
	for _, id := range ids {
		itemIds = append(itemIds, ItemId{Id: id})
	}

	req := &deleteItemBatchRequest{
		DeleteType:               "HardDelete",
		SendMeetingCancellations: "SendOnlyToAll",
		ItemIds:                  ItemIdsList{ItemId: itemIds},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, err
	}

	bb, err := c.SendAndReceiveContext(ctx, xmlBytes)
	if err != nil {
		return nil, err
	}

	var soapResp deleteItemBatchResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return nil, err
	}

	msgs := soapResp.Body.DeleteItemResponse.ResponseMessages.DeleteItemResponseMessage
	if len(msgs) == 0 {
		return nil, errors.New("go-ews: DeleteCalendarItems: empty response (no DeleteItemResponseMessage)")
	}

	results := make([]DeleteItemResult, len(ids))
	for i, id := range ids {
		if i >= len(msgs) {
			results[i] = DeleteItemResult{
				Index:  i,
				ItemID: id,
				Error:  fmt.Errorf("go-ews: DeleteCalendarItems: missing response message at index %d", i),
			}
			continue
		}
		msg := msgs[i]
		if msg.ResponseClass == ResponseClassError {
			results[i] = DeleteItemResult{
				Index:  i,
				ItemID: id,
				Error:  fmt.Errorf("go-ews: DeleteCalendarItems[%d]: %s (%s)", i, msg.MessageText, msg.ResponseCode),
			}
			continue
		}
		results[i] = DeleteItemResult{
			Index:  i,
			ItemID: id,
		}
	}
	return results, nil
}

// deleteItemBatchResponseBodyEnvelop — отдельный парсер для batch-response.
// Существующий deleteItemResponseBodyEnvelop (singular) не трогаем.
type deleteItemBatchResponseBodyEnvelop struct {
	XMLName struct{}                    `xml:"Envelope"`
	Body    deleteItemBatchResponseBody `xml:"Body"`
}

type deleteItemBatchResponseBody struct {
	DeleteItemResponse deleteItemBatchResponse `xml:"DeleteItemResponse"`
}

type deleteItemBatchResponse struct {
	ResponseMessages deleteItemBatchResponseMessages `xml:"ResponseMessages"`
}

type deleteItemBatchResponseMessages struct {
	DeleteItemResponseMessage []Response `xml:"DeleteItemResponseMessage"`
}
