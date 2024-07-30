package sms

import "errors"

type Sms struct {
	apiKey string
}

func New(apiKey string) (*Sms, error) {
	sms := Sms{apiKey: apiKey}
	if !sms.healthy() {
		return nil, errors.New("sms: can't connect")
	}
	return &sms, nil
}

func (s *Sms) Send(number string, message string) error {
	return nil
	//url := "https://czsrqykgpsseofmwnbja5t46f40tojzd.lambda-url.us-east-2.on.aws/"
	//key := os.Getenv("SMS_NOTIFY_KEY")
	//if key == "" {
	//	errMessage := "SMS_NOTIFY_KEY environment variable not set, unable to notify admin via SMS"
	//	fmt.Println(errMessage)
	//	return 0, errors.New(errMessage)
	//}
	//number := os.Getenv("SMS_NOTIFY_NUMBER")
	//if number == "" {
	//	errMessage := "SMS_NOTIFY_NUMBER environment variable not set, unable to notify admin via SMS"
	//	fmt.Println(errMessage)
	//	return 0, errors.New(errMessage)
	//}
	//payload := smsPayload{
	//	Key:     key,
	//	To:      number,
	//	Message: "[tmpcontrol]" + string(p),
	//}
	//body, _ := json.Marshal(payload)
	//req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	//if err != nil {
	//	return 0, err
	//}
	//req.Header.Set("Content-Type", "application/json")
	//client := &http.Client{}
	//resp, err := client.Do(req)
	//if err != nil {
	//	return 0, nil
	//}
	//defer resp.Body.Close()
	//return len(p), nil
}

func (s *Sms) healthy() bool {
	//TODO
	return true
}

type smsPayload struct {
	Key     string `json:"key"`
	To      string `json:"to"`
	Message string `json:"message"`
}
