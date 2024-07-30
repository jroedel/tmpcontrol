package clienttoserverapi

import (
	"bytes"
	"encoding/json"
)

type ConfigApiMessage struct {
	//TODO
}

type TemperatureApiMessage struct {
	//TODO
}

type NotifyApiMessage struct {
	ClientId             string  `json:"clientId"`
	Message              string  `json:"message"`
	Urgency              Urgency `json:"urgency"`
	HasAdminBeenNotified bool    `json:"hasAdminBeenNotified"`
}

type Urgency int

const (
	InfoNotification Urgency = iota + 1
	ProblemNotification
	SeriousNotification
)

func (u Urgency) String() string {
	switch u {
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

func (u Urgency) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`""`)
	buffer.WriteString(u.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func (u *Urgency) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	switch j {
	case "info":
		*u = InfoNotification
	case "problem":
		*u = ProblemNotification
	case "serious":
		*u = SeriousNotification
	default:
		*u = InfoNotification //we won't make a big deal about it
	}
	return nil
}
