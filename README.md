## EWS Exchange Web Service

Exchange Web Service client for golang

### usage:

```go
package main

import (
	"fmt"
	"github.com/vihapr/go-ews"
	"github.com/vihapr/go-ews/ewsutil"
	"log"
)

func main() {

	c := ews.NewClient(
		"https://outlook.office365.com/EWS/Exchange.asmx",
		"email@exchangedomain",
		"password",
		&ews.Config{Dump: true, NTLM: false},
	)

	err := ewsutil.SendEmail(c,
		[]string{"mhewedy@gmail.com", "someone@else.com"},
		"An email subject",
		"The email body, as plain text",
	)

	if err != nil {
		log.Fatal("err>: ", err.Error())
	}

	fmt.Println("--- success ---")
}

```

> Note: if you are using an on-premises Exchange server (or even if you manage your servers at the cloud), you need to pass the username as `AD_DOMAINNAME\username` instead, for examle `MYCOMANY\mhewedy`.

### Supported Feature matrix:

| Category                         | Operation            | Supported\*           |
| -------------------------------- | -------------------- | --------------------- |
| eDiscovery operations            |                      |                       |
| Exchange mailbox data operations |                      |                       |
|                                  | CreateItem operation | ✔️ (Email & Calendar) |
|                                  | GetUserPhoto         | ✔️                    |
| Availability operations          |                      |                       |
|                                  | GetUserAvailability  | ✔️                    |
|                                  | GetRoomLists         | ✔️                    |
| Bulk transfer operations         |                      |                       |
| Delegate management operations   |                      |                       |
| Inbox rules operations           |                      |                       |
| Mail app management operations   |                      |                       |
| Mail tips operation              |                      |                       |
| Message tracking operations      |                      |                       |
| Notification operations          |                      |                       |
| Persona operations               |                      |                       |
|                                  | FindPeople           | ✔️                    |
|                                  | GetPersona           | ✔️                    |
| Retention policy operation       |                      |                       |
| Service configuration operation  |                      |                       |
| Sharing operations               |                      |                       |
| Synchronization operations       |                      |                       |
| Time zone operation              |                      |                       |
| Unified Messaging operations     |                      |                       |
| Unified Contact Store operations |                      |                       |
| User configuration operations    |                      |                       |

- Not always 100% of fields are mapped.

### Extras

Besides the operations supported above, few new operations under the namespace `ewsutil` has been introduced:

- `ewsutil.SendEmail`
- `ewsutil.CreateEvent`
- `ewsutil.ListUsersEvents`
- `ewsutil.FindPeople`
- `ewsutil.GetUserPhoto`
- `ewsutil.GetUserPhotoBase64`
- `ewsutil.GetUserPhotoURL`
- `ewsutil.GetPersona`

NTLM is supported as well as Basic authentication

#### Reference:

https://docs.microsoft.com/en-us/exchange/client-developer/web-service-reference/ews-operations-in-exchange
