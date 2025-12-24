package ewsutil

import (
	"encoding/base64"
	"fmt"

	"github.com/vihapr/go-ews"
)

// GetUserPhoto
// https://docs.microsoft.com/en-us/exchange/client-developer/web-service-reference/getuserphoto-operation
func GetUserPhotoBase64(c ews.Client, email string) (string, error) {

	resp, err := ews.GetUserPhoto(c, &ews.GetUserPhotoRequest{
		Email:         email,
		SizeRequested: "HR48x48",
	})

	if err != nil {
		return "", err
	}

	return resp.PictureData, nil
}

func GetUserPhoto(c ews.Client, email string) ([]byte, error) {
	s, err := GetUserPhotoBase64(c, email)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(s)
}

func GetUserPhotoURL(c ews.Client, email string) string {
	return fmt.Sprintf("%s/s/GetUserPhoto?email=%s&size=HR48x48", c.GetEWSAddr(), email)
}
