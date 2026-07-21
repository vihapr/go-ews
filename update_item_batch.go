package ews

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"time"
)

// MaxUpdateItemsPerRequest — лимит числа элементов в одном EWS UpdateItem.
// Exchange документирует потолок 50 (общий лимит на batch-операцию).
const MaxUpdateItemsPerRequest = 50

// UpdateCalendarItemInput — вход одной операции batch-Update.
//
// Если ChangeKey пуст, библиотека резолвит его одним batch-запросом GetItem
// (GetCalendarItemsChangeKeys) для ВСЕХ элементов батча с пустым ChangeKey.
// Если ChangeKey заполнен вызывающей стороной —额外的 GetItem не делается.
type UpdateCalendarItemInput struct {
	ItemID    string
	ChangeKey string
	Start     time.Time
	End       time.Time
}

// UpdateItemResult — результат одного элемента batch-Update.
//
// ItemID/ChangeKey — обновлённые значения после апдейта (Exchange может
// менять ChangeKey при каждом успешном UpdateItem). При неудаче Error != nil,
// остальные поля пустые.
type UpdateItemResult struct {
	Index     int
	ItemID    string
	ChangeKey string
	Error     error
}

// UpdateCalendarItemsTime обновляет время набора событий в одном SOAP-запросе.
//
// Возвращает по одному UpdateItemResult на каждый входной элемент в том же
// порядке. Частичные ошибки (ResponseClass="Error" на конкретном элементе)
// возвращаются как Error в соответствующей позиции слайса.
//
// ChangeKey handling:
//   - если входной ChangeKey заполнен — используется как есть;
//   - если пуст — библиотека резолвит ChangeKey одним batch-запросом GetItem
//     для всех элементов с пустым ChangeKey. Если хотя бы один id не
//     зарезолвился — весь батч отменяется (Exchange всё равно отвергнет
//     апдейт без валидного ChangeKey).
//
// Top-level error непусто при ошибке маршалинга, HTTP ≠ 200, превышении
// лимита, невозможности зарезолвить ChangeKey, пустом конверте или отмене
// контекста. В этих случаях []UpdateItemResult — nil.
//
// https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/updateitem-operation
func UpdateCalendarItemsTime(ctx context.Context, c Client, items []UpdateCalendarItemInput) ([]UpdateItemResult, error) {
	if len(items) == 0 {
		return []UpdateItemResult{}, nil
	}
	if len(items) > MaxUpdateItemsPerRequest {
		return nil, fmt.Errorf("go-ews: UpdateCalendarItemsTime: %d items exceeds limit %d", len(items), MaxUpdateItemsPerRequest)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Резолвим ChangeKey для элементов, где он не передан, одним batch-запросом.
	missingIndices := make([]int, 0)
	missingIDs := make([]string, 0)
	for i, item := range items {
		if item.ChangeKey == "" {
			missingIndices = append(missingIndices, i)
			missingIDs = append(missingIDs, item.ItemID)
		}
	}
	if len(missingIDs) > 0 {
		keys, failed, err := GetCalendarItemsChangeKeys(ctx, c, missingIDs)
		if err != nil {
			return nil, fmt.Errorf("go-ews: UpdateCalendarItemsTime: resolve change keys: %w", err)
		}
		if len(failed) > 0 {
			return nil, fmt.Errorf("go-ews: UpdateCalendarItemsTime: failed to resolve ChangeKey for %d item(s): %v", len(failed), failed)
		}
		for idx, id := range missingIDs {
			items[missingIndices[idx]].ChangeKey = keys[id]
		}
	}

	// Структура UpdateItem уже поддерживает слайс ItemChange, формируем батч.
	// ВАЖНО: под go 1.21 range-переменная переиспользуется между итерациями,
	// поэтому item := items[i] внутри тела цикла обязательно — иначе
	// &item.Start будет указывать на одну и ту же memory location для всех
	// итераций, и весь батч уйдёт с одинаковыми time.Time значениями.
	changes := make([]ItemChange, 0, len(items))
	for i := range items {
		item := items[i]
		startField := SetItemField{
			FieldURI:     FieldURI{FieldURI: "calendar:Start"},
			CalendarItem: updateCalendarItem{Start: &item.Start},
		}
		endField := SetItemField{
			FieldURI:     FieldURI{FieldURI: "calendar:End"},
			CalendarItem: updateCalendarItem{End: &item.End},
		}
		changes = append(changes, ItemChange{
			ItemId: ItemId{Id: item.ItemID, ChangeKey: item.ChangeKey},
			Updates: Updates{
				SetItem: []SetItemField{startField, endField},
			},
		})
	}

	req := &UpdateItem{
		MessageDisposition:                    "SaveOnly",
		SendMeetingInvitationsOrCancellations: "SendToAllAndSaveCopy",
		ConflictResolution:                    "AlwaysOverwrite",
		ItemChanges:                           ItemChanges{Items: changes},
	}

	xmlBytes, err := xml.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, err
	}

	bb, err := c.SendAndReceiveContext(ctx, xmlBytes)
	if err != nil {
		return nil, err
	}

	var soapResp updateItemBatchResponseBodyEnvelop
	if err := xml.Unmarshal(bb, &soapResp); err != nil {
		return nil, err
	}

	msgs := soapResp.Body.UpdateItemResponse.ResponseMessages.UpdateItemResponseMessage
	if len(msgs) == 0 {
		return nil, errors.New("go-ews: UpdateCalendarItemsTime: empty response (no UpdateItemResponseMessage)")
	}

	results := make([]UpdateItemResult, len(items))
	for i := range items {
		if i >= len(msgs) {
			results[i] = UpdateItemResult{
				Index: i,
				Error: fmt.Errorf("go-ews: UpdateCalendarItemsTime: missing response message at index %d", i),
			}
			continue
		}
		msg := msgs[i]
		if msg.ResponseClass == ResponseClassError {
			results[i] = UpdateItemResult{
				Index: i,
				Error: fmt.Errorf("go-ews: UpdateCalendarItemsTime[%d]: %s (%s)", i, msg.MessageText, msg.ResponseCode),
			}
			continue
		}
		results[i] = UpdateItemResult{
			Index:     i,
			ItemID:    msg.Items.CalendarItem.ItemId.Id,
			ChangeKey: msg.Items.CalendarItem.ItemId.ChangeKey,
		}
	}
	return results, nil
}

// updateItemBatchResponseBodyEnvelop — отдельный парсер для batch-response.
// Существующий updateItemResponseBodyEnvelop (singular) не трогаем.
type updateItemBatchResponseBodyEnvelop struct {
	XMLName struct{}                    `xml:"Envelope"`
	Body    updateItemBatchResponseBody `xml:"Body"`
}

type updateItemBatchResponseBody struct {
	UpdateItemResponse updateItemBatchResponse `xml:"UpdateItemResponse"`
}

type updateItemBatchResponse struct {
	ResponseMessages updateItemBatchResponseMessages `xml:"ResponseMessages"`
}

type updateItemBatchResponseMessages struct {
	UpdateItemResponseMessage []Response `xml:"UpdateItemResponseMessage"`
}
