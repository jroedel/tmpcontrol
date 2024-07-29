package brewfatherapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

type Client struct {
	apiLogId string
}

func New(brewfatherApiLogId string) (Client, error) {
	if brewfatherApiLogId == "" {
		return Client{}, errors.New("brewfatherapi: logId is empty")
	}
	const logIdRegex = `[a-z0-9-]{4,18}`
	var re = regexp.MustCompile(logIdRegex)
	if !re.MatchString(brewfatherApiLogId) {
		return Client{}, errors.New("brewfatherapi: logId is not valid")
	}
	return Client{
		apiLogId: brewfatherApiLogId,
	}, nil
}

func (c Client) SendTemperatureReading(temp TempReading) error {
	if !temp.valid() {
		return errors.New("invalid temperature reading")
	}
	const endpointUrlPattern = "https://log.brewfather.net/stream?id=%s"
	url := fmt.Sprintf(endpointUrlPattern, c.apiLogId)

	payload, err := json.Marshal(temp)
	if err != nil {
		return err
	}

	const apiCallTimeout = time.Second * 5
	ctx, cancel := context.WithTimeout(context.Background(), apiCallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	const bodyLengthLimit = 1_000_000
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, bodyLengthLimit))
		return fmt.Errorf("brewfatherapi: bad status code: %d (%s)", resp.StatusCode, string(body))
	}
	return nil
}

type TempReading struct {
	TempType   TempType `json:"temp_type"`
	DeviceName string
	Temp       float32
	TempUnit   TempUnit
}

func (tr TempReading) valid() bool {
	return tr.TempType.String() != "" && tr.TempUnit.String() != "" && tr.DeviceName != ""
}

func (tr TempReading) MarshalJSON() ([]byte, error) {
	if !tr.valid() {
		return nil, errors.New("invalid temperature reading")
	}
	f := func(tr TempReading) (any, error) {
		switch tr.TempType {
		case RoomTemp:
			return struct {
				Temp     string `json:"ext_temp"`
				TempUnit string `json:"temp_unit"`
				Name     string `json:"name"`
			}{
				Temp:     fmt.Sprintf("%.2f", tr.Temp),
				TempUnit: tr.TempUnit.String(),
				Name:     tr.DeviceName,
			}, nil
		case FridgeTemp:
			return struct {
				Temp     string `json:"aux_temp"`
				TempUnit string `json:"temp_unit"`
				Name     string `json:"name"`
			}{
				Temp:     fmt.Sprintf("%.2f", tr.Temp),
				TempUnit: tr.TempUnit.String(),
				Name:     tr.DeviceName,
			}, nil
		case FermentationTemp:
			return struct {
				Temp     string `json:"temp"`
				TempUnit string `json:"temp_unit"`
				Name     string `json:"name"`
			}{
				Temp:     fmt.Sprintf("%.2f", tr.Temp),
				TempUnit: tr.TempUnit.String(),
				Name:     tr.DeviceName,
			}, nil
		default:
			return nil, errors.New("invalid temperature reading")
		}
	}
	structure, err := f(tr)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(structure)
	if err != nil {
		return nil, fmt.Errorf("brewfather: request json: %w", err)
	}
	return result, nil
}

type TempUnit int

const (
	Celsius TempUnit = iota + 1
	Fahrenheit
)

func (tu TempUnit) String() string {
	switch tu {
	case Celsius:
		return "C"
	case Fahrenheit:
		return "F"
	}
	return ""
}

type TempType int

const (
	FermentationTemp TempType = iota + 1
	RoomTemp
	FridgeTemp
)

func (t TempType) String() string {
	switch t {
	case FermentationTemp:
		return "temp"
	case RoomTemp:
		return "ext_temp"
	case FridgeTemp:
		return "aux_temp"
	}
	return ""
}
