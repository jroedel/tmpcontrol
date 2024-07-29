// Package notifyserver provides client functionality to send notification messages. May also be used by the server code to decode input
package notifyserver

import (
	"bytes"
	"encoding/json"
)

type NotifyApiMessage struct {
	ClientId string  `json:"clientId"`
	Message  string  `json:"message"`
	Urgency  Urgency `json:"urgency"`
}

// should this be pointer/value???
// should this be exported or not???
type Client struct {
	serverBaseUrl string
}

func NewClient(serverBaseUrl string) *Client {
	return &Client{serverBaseUrl: serverBaseUrl}
}

func (s *Client) Notify(msg NotifyApiMessage) error {
	//TODO
	return nil
}

type Urgency int

const (
	InfoNotification Urgency = iota + 1
	ProblemNotification
	SeriousNotification
)

func (s Urgency) String() string {
	switch s {
	case InfoNotification:
		return "info"
	case ProblemNotification:
		return "problem"
	case SeriousNotification:
		return "serious"
	default:
		return ""
	}
}

func (s Urgency) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`""`)
	buffer.WriteString(s.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func (s *Urgency) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	switch j {
	case "info":
		*s = InfoNotification
	case "problem":
		*s = ProblemNotification
	case "serious":
		*s = SeriousNotification
	default:
		*s = InfoNotification //we won't make a big deal about it
	}
	return nil
}
