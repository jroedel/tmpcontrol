// Package notifyserver provides client functionality to send notification messages. May also be used by the server code to decode input
package clienttoserverapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type NotifyApiMessage struct {
	ClientId string  `json:"clientId"`
	Message  string  `json:"message"`
	Urgency  Urgency `json:"urgency"`
}

type Client struct {
	serverBaseUrl string
	clientId      string
}

func NewClient(serverBaseUrl string, clientId string) *Client {
	return &Client{serverBaseUrl: serverBaseUrl, clientId: clientId}
}

const stdTimeout = time.Second * 5

func (s *Client) GetConfig() (ConfigApiMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stdTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", s.serverBaseUrl+"/configuration", nil)
	if err != nil {
		return ConfigApiMessage{}, fmt.Errorf("create config get request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ConfigApiMessage{}, fmt.Errorf("get config: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ConfigApiMessage{}, fmt.Errorf("read config: %w", err)
	}
	var config ConfigApiMessage
	err = json.Unmarshal(body, &config)
	if err != nil {
		return ConfigApiMessage{}, fmt.Errorf("parse config: %w", err)
	}
	return config, nil
}

func (s *Client) SendTemperature(data TemperatureApiMessage) error {
	//TODO
	return nil
}

func (s *Client) Notify(msg NotifyApiMessage) error {
	//TODO
	return nil
}

type ConfigApiMessage struct {
	//TODO
}

type TemperatureApiMessage struct {
	//TODO
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
