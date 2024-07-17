package tmpcontrol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

type SmsNotifier struct{}

// notifyAdminViaSms send and pray, this function will receive no logging
func (SmsNotifier) Write(p []byte) (int, error) {
	url := "https://czsrqykgpsseofmwnbja5t46f40tojzd.lambda-url.us-east-2.on.aws/"
	key := os.Getenv("ADMIN_NOTIFY_KEY")
	if key == "" {
		errMessage := "ADMIN_NOTIFY_KEY environment variable not set, unable to notify admin via SMS"
		fmt.Println(errMessage)
		return 0, errors.New(errMessage)
	}
	number := os.Getenv("ADMIN_NOTIFY_NUMBER")
	if number == "" {
		errMessage := "ADMIN_NOTIFY_NUMBER environment variable not set, unable to notify admin via SMS"
		fmt.Println(errMessage)
		return 0, errors.New(errMessage)
	}
	payload := smsPayload{
		Key:     key,
		To:      number,
		Message: "[tmpcontrol]" + string(p),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	return len(p), nil
}

type smsPayload struct {
	Key     string `json:"key"`
	To      string `json:"to"`
	Message string `json:"message"`
}
