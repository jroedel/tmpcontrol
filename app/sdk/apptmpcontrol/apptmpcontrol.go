package apptmpcontrol

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/jroedel/tmpcontrol/business/busclient/busadminnotifier"
	"github.com/jroedel/tmpcontrol/business/busclient/busclienttempdata"
	"github.com/jroedel/tmpcontrol/business/busclient/busconfiggopher"
	"github.com/jroedel/tmpcontrol/foundation/clienttoserverapi"
	"github.com/jroedel/tmpcontrol/foundation/ctlkasaplug"
	"github.com/jroedel/tmpcontrol/foundation/ds18b20therm"
)

type App struct {
	//required
	cg     *busconfiggopher.ConfigGopher
	th     *busclienttempdata.TempHandler
	kasa   *ctlkasaplug.KasaController
	logger *log.Logger

	//optional
	notify *busadminnotifier.AdminNotifier

	//internal
	currentConfig busconfiggopher.ControllersConfig
}

func New(cg *busconfiggopher.ConfigGopher, th *busclienttempdata.TempHandler, kasa *ctlkasaplug.KasaController, logger *log.Logger, notify *busadminnotifier.AdminNotifier) (*App, error) {
	if cg == nil {
		return nil, fmt.Errorf("app construct: ConfigGopher is required")
	}
	if th == nil {
		return nil, fmt.Errorf("app construct: TempHandler is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("app construct: Logger is required")
	}
	return &App{cg: cg, th: th, kasa: kasa, logger: logger, notify: notify}, nil
}

func (app *App) Start(ctx context.Context) error {
	app.logger.Println("Fetching initial config")

	config, source, err := app.cg.FetchConfig()
	if err != nil {
		return fmt.Errorf("%s", err)
	}

	// NEED MUTEX HERE
	app.currentConfig = config

	app.logger.Printf("Successfully fetched initial config from %s; we'll continue to poll every 60 seconds\n%+v\n", source, config)
	app.notify.NotifyAdmin("we got some config and we're starting up", clienttoserverapi.InfoNotification)

	const configFetchInterval = 60 * time.Second

	go func() {
		timer := time.NewTimer(configFetchInterval)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				config, _, err := app.cg.FetchConfig()
				if err != nil {
					app.logger.Printf("fetching config: %s", err)
				}

				// NEED RWMUTEX HERE AND ON ANY READ
				app.currentConfig = config

			case <-ctx.Done():
				app.logger.Println("Context canceled")
				return
			}
		}
	}()

	go func() {
		// // TODO notify admin if it's been more than a few minutes without being able to get new config
		// const intervalNotifyServerForConfigFetch = 15 * time.Minute
		// err = app.cg.NotifyAdminIfWeHaventReceivedConfigInInterval(ctx, intervalNotifyServerForConfigFetch)
		// if err != nil {
		// 	return fmt.Errorf("NotifyAdminIfWeHaventReceivedConfigInInterval: %s", err)
		// }
		for {
			<-ctx.Done()
			app.logger.Println("Context canceled")
		}
	}()

	app.logger.Printf("Beginning control loop for %d controller(s)\n", len(config.Controllers))

	app.Run(ctx)

	return nil
}

// Run will start the process of doing the actual work.
func (app *App) Run(ctx context.Context) {
	const loopInterval = 15 * time.Second
	timer := time.NewTicker(loopInterval)

	for {
		timer.Reset(loopInterval)
		loopStart := time.Now()

		select {
		case <-timer.C:

			//make sure the config doesn't get updated while we're in the middle of our work
			// NEED MUTEX READ HERE
			config := app.currentConfig

			returnChan := make(chan temperatureControlReturn, len(config.Controllers))

			for i := range config.Controllers {

				//TODO set a timeout of 12 seconds
				//TODO how can we notify the server when a new temperature rule has been applied for the first time
				go func() {
					tcr := app.temperatureControl(ctx, app.currentConfig.Controllers[i])
					if ctx.Err() != nil {
						returnChan <- tcr
					}
				}()
			}

			//the idea behind this 2nd loop is to wait for each of the goroutines spun up to finish and report back
			for range len(config.Controllers) {
				var returnValue temperatureControlReturn

				// How do we make it so if one loop fails, the other can keep on ticking?
				select {
				case returnValue = <-returnChan:

				case <-ctx.Done():
					app.logger.Println("Requested to cancel work")
					return
				}

				if returnValue.err != nil {
					app.logger.Printf("[%s] Error in temperatureControl loop: %s\n", returnValue.controllerConfig.Name, returnValue.err.Error())
					continue
				}

				err := app.th.HandleNewTempData(returnValue.temperature)
				if err != nil {
					app.logger.Printf("[%s] Error persisting log to sqlite dbo: %s", returnValue.controllerConfig.Name, err)
					app.notify.NotifyAdmin("We couldn't save a TmpLog to the sqlite dbo", clienttoserverapi.ProblemNotification)
				}
			}

			app.logger.Printf("This iteration of the control loop took %s\n", time.Since(loopStart).String())

		case <-ctx.Done():
			return
		}
	}
}

// @TODO Maybe these should all be configurable
const (
	intervalNotifyServerForSwitchHostComm = 5 * time.Minute
	intervalNotifyServerForTempRead       = 1 * time.Minute
)

type temperatureControlReturn struct {
	controllerConfig                   busconfiggopher.Controller
	successfulTemperatureReadTimestamp time.Time
	//keys are the hostname and values are whether they succeeded or not
	successfulHostControlTimestamp map[string]time.Time
	noSchedulesAreActive           bool
	temperature                    busclienttempdata.Temperature
	err                            error
}

var ErrTemperatureRead = errors.New("there was a problem reading the current temperature")
var ErrAtLeastOneHostControlFailed = fmt.Errorf("at least one host control failed")

// we'll print directly from this function, prefixing the name of the controller
func (app *App) temperatureControl(ctx context.Context, controllerConfig busconfiggopher.Controller) temperatureControlReturn {
	desiredTemperature, ok := getCurrentDesiredTemperature(controllerConfig)
	if !ok {
		app.logger.Printf("[%s]: No temperature schedules have come to pass. We should wait around for a little\n", controllerConfig.Name)
		return temperatureControlReturn{
			controllerConfig:               controllerConfig,
			successfulHostControlTimestamp: make(map[string]time.Time),
			noSchedulesAreActive:           true,
		}
	}

	currentTemperature, err := ds18b20therm.ReadTemperatureInF(controllerConfig.ThermometerPath)
	if err != nil {
		app.logger.Printf("[%s]: We had a problem getting current temperature from %#v. Turning off controls just in case. We'll wait a second and try again: %s\n", controllerConfig.Name, controllerConfig.ThermometerPath, err)
		err = errors.Join(ErrTemperatureRead, err)
	}

	app.logger.Printf("[%s]: The latest temperature is %.2f and desired temperature is %.2f\n", controllerConfig.Name, currentTemperature, desiredTemperature)

	newState := ctlkasaplug.ControlOff

	if err == nil {
		switch controllerConfig.ControlType {
		case "cool":
			// If the current temperature is greater than the desired temperature,
			// then turn on the cooling elements
			if currentTemperature > desiredTemperature {
				newState = ctlkasaplug.ControlOn
			}

		default:
			// If the current temperature is less than the desired temperature,
			// then turn on the heating elements
			if currentTemperature < desiredTemperature {
				newState = ctlkasaplug.ControlOn
			}
		}
	}

	if !controllerConfig.DisableFreezeProtection && currentTemperature < 33 && newState != ctlkasaplug.ControlOff {
		newState = ctlkasaplug.ControlOff
		app.logger.Printf("[%s]: FREEZE PROTECTION We're turning off all hosts since the temperature is %.2f\n", controllerConfig.Name, currentTemperature)
	}

	//communicate with the control hosts
	successfulHostControlTimestamp := make(map[string]bool)

	for _, host := range controllerConfig.SwitchHosts {
		app.logger.Printf("[%s] Turning %s %s\n", controllerConfig.Name, newState, host)

		if ctx.Err() != nil {
			return temperatureControlReturn{
				err: ctx.Err(),
			}
		}

		err := func() error {
			const kasaTimeout = 4 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), kasaTimeout)
			defer cancel()

			return app.kasa.ControlDevice(ctx, host, newState)
		}()

		if ctx.Err() != nil {
			return temperatureControlReturn{
				err: ctx.Err(),
			}
		}

		switch {
		case err != nil:
			var exitError *exec.ExitError
			if errors.As(err, &exitError) { // is our error because it timed out?
				app.logger.Printf("[%s] Our call to controlDevice timed out after 4 seconds\n", controllerConfig.Name)
			} else {
				app.logger.Printf("[%s] Our call to controlDevice returned an error: %s\n", controllerConfig.Name, err)
			}

		default:
			successfulHostControlTimestamp[host] = true
		}

		//TODO can we schedule a failsafe on the device in case we crash next iteration?
	}

	if ctx.Err() != nil {
		return temperatureControlReturn{
			err: ctx.Err(),
		}
	}

	if len(successfulHostControlTimestamp) != len(controllerConfig.SwitchHosts) {
		err = errors.Join(err, ErrAtLeastOneHostControlFailed)
	}

	//if newState == ctlkasaplug.ControlOff {
	if errors.Is(err, ErrTemperatureRead) {
		return temperatureControlReturn{
			// YOU NEED TO FIGURE THIS OUT!
			controllerConfig:               controllerConfig,
			successfulHostControlTimestamp: make(map[string]time.Time),
			noSchedulesAreActive:           true,
		}
	}

	successfulHosts := make([]string, 0, len(successfulHostControlTimestamp))
	for k := range successfulHostControlTimestamp {
		successfulHosts = append(successfulHosts, k)
	}

	temperature := busclienttempdata.Temperature{
		ControllerName:            controllerConfig.Name,
		Timestamp:                 time.Now(),
		TemperatureInF:            currentTemperature,
		DesiredTemperatureInF:     desiredTemperature,
		IsHeatingNotCooling:       controllerConfig.ControlType != "cool",
		TurningOnNotOff:           newState == ctlkasaplug.ControlOn,
		SuccessfulHostsControlled: successfulHosts,
	}

	return temperatureControlReturn{
		// YOU NEED TO FIGURE THIS OUT!
		controllerConfig:               controllerConfig,
		successfulHostControlTimestamp: make(map[string]time.Time),
		noSchedulesAreActive:           true,
		temperature:                    temperature,
	}
}

func getCurrentDesiredTemperature(controller busconfiggopher.Controller) (float32, bool) {
	//we look for the newest entry before the current time
	now := time.Now()
	var mostRecentSchedule time.Time
	for k := range controller.TemperatureSchedule {
		if k.Before(now) && k.After(mostRecentSchedule) {
			mostRecentSchedule = k
		}
	}
	if mostRecentSchedule.IsZero() {
		return 0, false
	}
	return controller.TemperatureSchedule[mostRecentSchedule], true
}
