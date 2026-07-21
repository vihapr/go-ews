package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
)

// MaxGetItemsPerRequest — лимит числа элементов в одном EWS GetItem.
// Exchange документирует потолок 50 (общий лимит на batch-операцию).
const MaxGetItemsPerRequest = 50

// getItemBatchRequest — запрос GetItem с несколькими ItemId.
// Отдельная структура от GetItem (singular), чтобы не ломать
// GetCalendarItemChangeKey и его тест Test_marshal_GetItem.
type getItemBatchRequest struct {
	XMLName   struct{}    `xml:"m:GetItem"`
	ItemShape ItemShape   `xml:"m:ItemShape"`
	ItemIds   ItemIdsList `xml:"m:ItemIds"`
}

// GetCalendarItemsChangeKeys резолвит ChangeKey для набора item id одним
// GetItem-запросом.
//
// Возвращает:
//   - map[itemID]changeKey для успешно найденных элементов;
//   - слайс id, которые не удалось найти (ResponseClass=Error или пустой
//     ChangeKey в ответе);
//   - top-level error при ошибке маршалинга, HTTP ≠ 200, пустом конверте,
//     превышении лимита или отмене контекста.
//
// В случае top-level error остальные возвращаемые значения — nil/пустые.
//
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/getitem-operation
func GetCalendarItemsChangeKeys(ctx context.Context, c Client, ids []string) (map[string]string, []string, error) {
	if len(ids) == 0 {
		return map[string]string{}, []string{}, nil
	}
	if len(ids) > MaxGetItemsPerRequest {
		return nil, nil, fmt.Errorf("go-ews: GetCalendarItemsChangeKeys: %d ids exceeds limit %d", len(ids), MaxGetItemsPerRequest)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	itemIds := make([]ItemId, 0, len(ids))
	for _, id := range ids {
		itemIds = append(itemIds, ItemId{Id: id})
	}

	req := &getItemBatchRequest{
		ItemShape: ItemShape{BaseShape: BaseShapeIdOnly},
		ItemIds:   ItemIdsList{ItemId: itemIds},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	bb, err := c.SendAndReceiveContext(ctx, xmlBytes)
	if err != nil {
		return nil, nil, err
	}

	var soapResp getItemBatchResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return nil, nil, err
	}

	msgs := soapResp.Body.GetItemResponse.ResponseMessages.GetItemResponseMessage
	if len(msgs) == 0 {
		return nil, nil, errors.New("go-ews: GetCalendarItemsChangeKeys: empty response (no GetItemResponseMessage)")
	}

	keys := make(map[string]string, len(ids))
	failed := make([]string, 0)
	for i, id := range ids {
		if i >= len(msgs) {
			failed = append(failed, id)
			continue
		}
		msg := msgs[i]
		if msg.ResponseClass == ResponseClassError {
			failed = append(failed, id)
			continue
		}
		changeKey := msg.Items.CalendarItem.ItemId.ChangeKey
		if changeKey == "" {
			failed = append(failed, id)
			continue
		}
		keys[id] = changeKey
	}
	return keys, failed, nil
}

// getItemBatchResponseBodyEnvelop — отдельный парсер для batch-response.
// Существующий getItemResponseBodyEnvelop (singular) не трогаем.
type getItemBatchResponseBodyEnvelop struct {
	XMLName struct{}                  `xml:"Envelope"`
	Body    getItemBatchResponseBody  `xml:"Body"`
}

type getItemBatchResponseBody struct {
	GetItemResponse getItemBatchResponse `xml:"GetItemResponse"`
}

type getItemBatchResponse struct {
	ResponseMessages getItemBatchResponseMessages `xml:"ResponseMessages"`
}

type getItemBatchResponseMessages struct {
	GetItemResponseMessage []Response `xml:"GetItemResponseMessage"`
}
