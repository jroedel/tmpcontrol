package busclienttempdata

import "time"

type Temperature struct {
	ControllerName            string
	Timestamp                 time.Time
	TemperatureInF            float32
	DesiredTemperatureInF     float32
	IsHeatingNotCooling       bool
	TurningOnNotOff           bool
	SuccessfulHostsControlled []string
	DbAutoId                  int
	ExecutionIdentifier       string
}
