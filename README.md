# go-ews — клиент Exchange Web Services для Go

Библиотека для работы с Microsoft Exchange Web Services (EWS) из Go. Реализует
SOAP-клиент, типы запросов/ответов и набор готовых операций над почтовым
ящиком, календарём и персонами.

Модуль: `github.com/vihapr/go-ews`

Минимальная версия Go: **1.21**.

---

## Содержание

- [Установка](#установка)
- [Создание клиента](#создание-клиента)
- [Аутентификация](#аутентификация)
- [Поддержка context.Context](#поддержка-contextcontext)
- [Календарь — одиночные операции](#календарь--одиночные-операции)
- [Календарь — пакетные операции (batch)](#календарь--пакетные-операции-batch)
- [iCalUid и дедупликация событий](#icaluid-и-дедупликация-событий)
- [Контракт ошибок batch-методов](#контракт-ошибок-batch-методов)
- [Лимиты EWS](#лимиты-ews)
- [Email](#email)
- [Доступность и помещения](#доступность-и-помещения)
- [Персоны и фотографии](#персоны-и-фотографии)
- [Утилиты ewsutil](#утилиты-ewsutil)
- [Поддерживаемые операции](#поддерживаемые-операции)
- [Ссылки](#ссылки)

---

## Установка

```bash
go get github.com/vihapr/go-ews
```

Импорт:

```go
import (
    "github.com/vihapr/go-ews"
    "github.com/vihapr/go-ews/ewsutil"
)
```

---

## Создание клиента

```go
c := ews.NewClient(
    "https://outlook.office365.com/EWS/Exchange.asmx",
    "email@exchangedomain",
    "password",
    &ews.Config{
        Dump:    false, // логировать запросы/ответы в stdout
        NTLM:    false, // использовать NTLM-аутентификацию
        SkipTLS: false, // пропустить проверку TLS-сертификата
    },
)
```

`Config` — опциональная структура, можно передать `nil` для значений по
умолчанию (Basic-аутентификация, без дампа).

### Методы клиента

`ews.Client` — интерфейс с тремя методами:

| Метод                                              | Назначение                                               |
| -------------------------------------------------- | -------------------------------------------------------- |
| `SendAndReceive(body []byte) ([]byte, error)`      | Синхронный SOAP-вызов без контекста (context.Background) |
| `SendAndReceiveContext(ctx, body) ([]byte, error)` | SOAP-вызов с поддержкой отмены/таймаута через ctx        |
| `GetEWSAddr() string`                              | URL EWS-эндпоинта                                        |
| `GetUsername() string`                             | Имя аутентифицированного пользователя                    |

`SendAndReceive` теперь делегирует в `SendAndReceiveContext(context.Background(), ...)`.
HTTP ≠ 200 оборачивается в `*ews.HTTPError{Status, StatusCode}` или `*ews.SoapError{Fault}`.

---

## Аутентификация

| Тип   | Флаг          | Когда использовать                             |
| ----- | ------------- | ---------------------------------------------- |
| Basic | `NTLM: false` | Office 365, Exchange Online, cloud-инсталляции |
| NTLM  | `NTLM: true`  | On-premises Exchange в AD-домене               |

Для on-premises Exchange с NTLM имя пользователя обычно передаётся как
`DOMAIN\username`, например `MYCOMPANY\ivan.ivanov`.

---

## Поддержка context.Context

Начиная с версии с batch-операциями интерфейс `Client` расширён методом
`SendAndReceiveContext(ctx, body)`. Это позволяет:

- отменять долгий HTTP-вызов через `ctx.Done()`;
- ставить таймаут на транспорт (`context.WithTimeout`);
- пробрасывать дедлайн из вышележащего обработчика.

Все batch-методы библиотеки принимают `context.Context` первым аргументом.
Одиночные (`CreateCalendarItem`, `DeleteCalendarItem`, `UpdateCalendarItemTime`,
`GetCalendarItemChangeKey`) оставлены без контекста для обратной совместимости
и внутри используют `context.Background()`.

При заранее отменённом контексте библиотека возвращает `ctx.Err()` без
выполнения HTTP-запроса.

---

## Календарь — одиночные операции

### Создать событие

```go
item := ews.CalendarItem{
    Subject:                    "Planning Meeting",
    Body:                       ews.Body{BodyType: "Text", Body: []byte("Agenda")},
    ReminderIsSet:              true,
    ReminderMinutesBeforeStart: 15,
    UID:                        "11223344-5566-7788-99aa-bbccddeeff00", // опц. — см. раздел про iCalUid
    Start:                      start,
    End:                        end,
    IsAllDayEvent:              false,
    LegacyFreeBusyStatus:       ews.BusyTypeBusy,
    Location:                   "Conference Room 721",
    RequiredAttendees:          []ews.Attendees{{Attendee: attendees}},
    OptionalAttendees:          []ews.Attendees{{Attendee: optional}},
    Resources:                  []ews.Attendees{{Attendee: rooms}},
    Attachments:                &ews.Attachments{Attachments: files}, // опц.
}
id, err := ews.CreateCalendarItem(c, item)
```

### Удалить событие

```go
id, err := ews.DeleteCalendarItem(c, "AAMkAGY...")
```

### Обновить время события

```go
updatedID, err := ews.UpdateCalendarItemTime(c, "AAMkAGY...", start, end)
```

Внутри резолвит текущий `ChangeKey` через `GetCalendarItemChangeKey` и
формирует `UpdateItem` с полями `calendar:Start` и `calendar:End`.

### Получить ChangeKey

```go
changeKey, err := ews.GetCalendarItemChangeKey(c, "AAMkAGY...")
```

Реализован как тонкая обёртка над `GetCalendarItemsChangeKeys` (batch-версия)
для одного id.

---

## Календарь — пакетные операции (batch)

Позволяют до 50 операций в одном SOAP-запросе. Снижают число HTTP-вызовов к
Exchange и подходят для массовой синхронизации событий (sync absence-фонов,
массовые напоминания и т.п.).

Все batch-методы принимают `context.Context` первым аргументом.

### Создать несколько событий

```go
items := []ews.CalendarItem{
    {Subject: "Event A", UID: "uid-a", Start: s1, End: e1, LegacyFreeBusyStatus: "Busy"},
    {Subject: "Event B", UID: "uid-b", Start: s2, End: e2, LegacyFreeBusyStatus: "OOF"},
}
results, err := ews.CreateCalendarItems(ctx, c, items)
if err != nil {
    // ошибка всего запроса: HTTP ≠ 200, ошибка маршалинга, превышение лимита и т.п.
    return
}
for _, r := range results {
    if r.Error != nil {
        // частичная ошибка — конкретный элемент не создан, остальные в порядке
        log.Printf("item[%d] failed: %v", r.Index, r.Error)
        continue
    }
    log.Printf("item[%d] created: id=%s changeKey=%s", r.Index, r.ItemID, r.ChangeKey)
}
```

Тип результата:

```go
type CreateItemResult struct {
    Index     int      // позиция во входном слайсе
    ItemID    string   // пустая строка при ошибке
    ChangeKey string
    Error     error    // nil при успехе
}
```

### Удалить несколько событий

```go
results, err := ews.DeleteCalendarItems(ctx, c, []string{"AAMkAAA=", "AAMkBBB="})
```

```go
type DeleteItemResult struct {
    Index  int
    ItemID string // эхо входного id
    Error  error
}
```

### Обновить время нескольких событий

```go
items := []ews.UpdateCalendarItemInput{
    {ItemID: "AAMkAAA=", ChangeKey: "ck-aaa", Start: s1, End: e1},
    {ItemID: "AAMkBBB=", ChangeKey: "",        Start: s2, End: e2}, // ChangeKey будет зарезолвлен
}
results, err := ews.UpdateCalendarItemsTime(ctx, c, items)
```

```go
type UpdateCalendarItemInput struct {
    ItemID    string
    ChangeKey string // если пусто — резолвится одним batch-запросом GetItem
    Start     time.Time
    End       time.Time
}

type UpdateItemResult struct {
    Index     int
    ItemID    string // новый id/ChangeKey после апдейта
    ChangeKey string
    Error     error
}
```

Если у одного или нескольких элементов `ChangeKey` пустой, библиотека делает
**один** дополнительный batch-запрос `GetCalendarItemsChangeKeys` для резолва.
Если хотя бы один id не зарезолвился — весь батч отменяется (Exchange всё
равно отвергнет апдейт без валидного ChangeKey на каждый элемент).

### Резолв ChangeKey для набора событий

```go
keys, failed, err := ews.GetCalendarItemsChangeKeys(ctx, c, []string{"AAMkAAA=", "AAMkBBB="})
// keys     = map[itemID]changeKey — успешно найденные
// failed   = []string             — id, не найденные на сервере (ResponseClass=Error или пустой ChangeKey)
```

---

## iCalUid и дедупликация событий

Поле `UID` в структуре `CalendarItem` соответствует `iCalUid` события в
календаре Exchange. Используется для серверной и клиентской дедупликации:

- Exchange сохраняет переданный `UID` как `iCalUid` события.
- Повторный `CreateItem` с тем же `UID` воспринимается Outlook/mobile-клиентами
  как апдейт того же события, дубль не плодится.
- При ретраях (повторных отправках) идемпотентность достигается именно через `UID`.

```go
item := ews.CalendarItem{
    Subject: "Absence",
    UID:     "11223344-5566-7788-99aa-bbccddeeff00", // iCalUid
    Start:   start,
    End:     end,
}
```

Поле имеет тег `xml:"t:UID,omitempty"`, поэтому вызовы без `UID` сериализуются
байт-в-байт как раньше (обратная совместимость).

Позиция `<t:UID>` в XML соответствует EWS-схеме `CalendarItemType` — перед
`<t:Start>`, рядом с другими strongly-typed свойствами.

---

## Контракт ошибок batch-методов

Все batch-методы (`CreateCalendarItems`, `DeleteCalendarItems`,
`UpdateCalendarItemsTime`, `GetCalendarItemsChangeKeys`) следуют одному
контракту:

**Top-level `error`** (второе возвращаемое значение) непусто только при:

- ошибке маршалинга запроса или анмаршалинга ответа;
- HTTP ≠ 200 (включая 429 и 5xx — проброс `*ews.HTTPError` / `*ews.SoapError`,
  вышележащий код сам делает ретраи);
- превышении лимита `Max*ItemsPerRequest`;
- пустом конверте ответа (ни одного `*ResponseMessage`);
- невозможности зарезолвить `ChangeKey` (только для `UpdateCalendarItemsTime`);
- отмене контекста (`ctx.Err()` пробрасывается как есть).

В этих случаях слайс результатов — `nil`.

**Частичные ошибки** (`ResponseClass="Error"` на отдельных элементах батча)
не заваливают весь вызов. Соответствующая позиция в слайсе результатов
содержит `Error != nil`, остальные элементы — штатные.

Проброс `*ews.HTTPError` делает батч-методы совместимыми с throttling /
retry / circuit-breaker декораторами вышележащего сервиса — retry-логика
реагирует на стандартный тип ошибки.

---

## Лимиты EWS

Exchange ограничивает число операций в одном batch-запросе. Библиотека
объявляет константы и принудительно проверяет входные слайсы:

| Константа                  | Значение | Применима к                  |
| -------------------------- | -------- | ---------------------------- |
| `MaxCreateItemsPerRequest` | 50       | `CreateCalendarItems`        |
| `MaxDeleteItemsPerRequest` | 50       | `DeleteCalendarItems`        |
| `MaxUpdateItemsPerRequest` | 50       | `UpdateCalendarItemsTime`    |
| `MaxGetItemsPerRequest`    | 50       | `GetCalendarItemsChangeKeys` |

При превышении возвращается явная ошибка — библиотека не режет входной
слайс молча. Вышележащий код (например, almanac `ThrottledClient`) сам
формирует батчи нужного размера.

---

## Email

```go
// без вложений
err := ewsutil.SendEmail(c,
    []string{"alice@example.com", "bob@example.com"},
    "Тема письма",
    "Тело письма plain text",
)

// с вложениями
err := ewsutil.SendEmail(c,
    []string{"alice@example.com"},
    "Тема письма",
    "<p>HTML тело</p>",
    ews.FileAttachment{ContentId: "logo@nodemailer.com", Name: "logo.png", Content: base64Content, ContentType: "image/png"},
)
```

---

## Доступность и помещения

### GetUserAvailability

Запрос занятости пользователей в диапазоне времени. Возвращает free/busy и
опционально детали событий (тема, место).

```go
req := &ews.GetUserAvailabilityRequest{
    TimeZone:         ews.TimeZone{Bias: -180, /* ... */},
    MailboxDataArray: ews.MailboxDataArray{MailboxData: mb},
    FreeBusyViewOptions: ews.FreeBusyViewOptions{
        TimeWindow:      ews.TimeWindow{StartTime: from, EndTime: to},
        RequestedView:   ews.RequestedViewDetailed, // или FreeBusy, FreeBusyMerged, DetailedMerged
    },
}
resp, err := ews.GetUserAvailability(c, req)
```

### GetRoomLists

Список распределительных групп помещений. Полезен как лёгкий health-check.

```go
resp, err := ews.GetRoomLists(c)
```

---

## Персоны и фотографии

| Метод                                  | Описание                                |
| -------------------------------------- | --------------------------------------- |
| `ews.FindPeople(c, q)`                 | Поиск персон по строке                  |
| `ews.GetPersona(c, personaID)`         | Получить детальную информацию о персоне |
| `ewsutil.GetUserPhoto(c, email)`       | Бинарное содержимое фотографии          |
| `ewsutil.GetUserPhotoBase64(c, email)` | Base64-представление фотографии         |
| `ewsutil.GetUserPhotoURL(c, email)`    | Прямой URL для скачивания фотографии    |

---

## Утилиты `ewsutil`

Тонкие обёртки над операциями `ews` для типовых сценариев:

| Функция                                      | Эквивалент                                   |
| -------------------------------------------- | -------------------------------------------- |
| `ewsutil.CreateEvent(...)`                   | `ews.CreateCalendarItem` (text body)         |
| `ewsutil.CreateHTMLEvent(...)`               | `ews.CreateCalendarItem` (HTML body + files) |
| `ewsutil.DeleteCalendarEvent(c, id)`         | `ews.DeleteCalendarItem`                     |
| `ewsutil.UpdateEventTime(c, id, s, e)`       | `ews.UpdateCalendarItemTime`                 |
| `ewsutil.ListUsersEvents(c, users, from, d)` | `ews.GetUserAvailability`                    |
| `ewsutil.SendEmail(c, to, subject, body, atts)` | `ews.CreateMessageItem`                      |
| `ewsutil.FindPeople(c, q)`                   | `ews.FindPeople`                             |
| `ewsutil.GetPersona(c, id)`                  | `ews.GetPersona`                             |
| `ewsutil.GetUserPhoto*`                      | см. таблицу выше                             |

---

## Поддерживаемые операции

| Категория         | Операция              | Поддержка                           |
| ----------------- | --------------------- | ----------------------------------- |
| Почта и календарь | `CreateItem`          | Email, Calendar                     |
|                   | `DeleteItem`          | Calendar (single + batch)           |
|                   | `UpdateItem`          | Calendar Start/End (single + batch) |
|                   | `GetItem`             | IdOnly / ChangeKey (single + batch) |
| Доступность       | `GetUserAvailability` | FreeBusy, Detailed                  |
|                   | `GetRoomLists`        | Health-check, список помещений      |
| Персон            | `FindPeople`          | Поиск                               |
|                   | `GetPersona`          | Детали персоны                      |
| Фотографии        | `GetUserPhoto`        | Binary, Base64, URL                 |

> Не все поля EWS-схем отображены — покрываются поля, необходимые для
> текущих потребителей (`notificator`, `almanac`).

---

## Ссылки

- [EWS operations in Exchange](https://docs.microsoft.com/en-us/exchange/client-developer/web-service-reference/ews-operations-in-exchange)
- [CreateItem operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/createitem-operation)
- [DeleteItem operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/deleteitem-operation)
- [UpdateItem operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/updateitem-operation)
- [GetItem operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/getitem-operation)
- [CalendarItemType (UID, schema)](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/calendaritem)
- [GetUserAvailability operation](https://learn.microsoft.com/en-us/exchange/client-developer/web-service-reference/getuseravailability-operation)
