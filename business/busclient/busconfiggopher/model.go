package busconfiggopher

import (
	"fmt"
	"time"
)

type ControllersConfig struct {
	Controllers []Controller `json:"controllers"`
}

type Controller struct {
	Name                    string                `json:"name"`
	ThermometerPath         string                `json:"thermometerPath"`
	ControlType             string                `json:"controlType"`
	SwitchHosts             []string              `json:"switchHosts"`
	TemperatureSchedule     map[time.Time]float32 `json:"temperatureSchedule"`
	DisableFreezeProtection bool                  `json:"disableFreezeProtection"`
}

type ConfigSource int

const (
	ConfigSourceLocalFile ConfigSource = iota + 1
	ConfigSourceServer
)

func (c ConfigSource) String() string {
	switch c {
	case ConfigSourceLocalFile:
		return "local file"
	case ConfigSourceServer:
		return "server"
	default:
		panic(fmt.Sprintf("Unknown config source: %#v", c))
	}
	return ""
}
