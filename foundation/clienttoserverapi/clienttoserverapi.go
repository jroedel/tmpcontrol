// Package notifyserver provides client functionality to send notification messages. May also be used by the server code to decode input
package clienttoserverapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

var ClientIdRegex = regexp.MustCompile(`^[-a-zA-Z0-9]{3,50}$`)

type Client struct {
	serverBaseUrl string
	clientId      string
}

func New(serverBaseUrl string, clientId string) (*Client, error) {
	cln := Client{serverBaseUrl: serverBaseUrl, clientId: clientId}
	if !cln.Healthy() {
		return nil, fmt.Errorf("could not connect to server")
	}
	return &cln, nil
}

const stdTimeout = time.Second * 5

func (cln *Client) Healthy() bool {
	ctx, cancel := context.WithTimeout(context.Background(), stdTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", cln.serverBaseUrl+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return true
}

func (cln *Client) GetConfig() (ConfigApiMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stdTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", cln.serverBaseUrl+"/configuration", nil)
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

func (cln *Client) SendTemperature(data TemperatureApiMessage) error {
	//TODO
	return nil
}

// NewNotifyApiMessage prefills the client id
func (cln *Client) NewNotifyApiMessage() NotifyApiMessage {
	return NotifyApiMessage{ClientId: cln.clientId}
}

func (cln *Client) Notify(msg NotifyApiMessage) error {
	//TODO
	return nil
}
