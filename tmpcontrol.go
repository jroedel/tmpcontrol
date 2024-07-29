package tmpcontrol

import (
	"errors"
	"fmt"
	"github.com/jroedel/tmpcontrol/business/busconfiggopher"
	"os/exec"
	"strings"
	"time"
)

type Logger interface {
	Printf(string, ...interface{})
}

type TmpLog struct {
	ControllerName        string
	Timestamp             time.Time
	TemperatureInF        float32
	DesiredTemperatureInF float32
	IsHeatingNotCooling   bool
	TurningOnNotOff       bool
	HostsPipeSeparated    string

	//these should be left blank unless we get this from the local dbo
	DbAutoId            int
	ExecutionIdentifier string
}

const minValidFahrenheitTemperature = -30
const maxValidFahrenheitTemperature = 215

type TemperatureReader interface {
	// ReadTemperatureInF Returns the temperature in Fahrenheit
	ReadTemperatureInF(connectionString string) (float32, error)
}

type ControlLooper struct {
	Cg                   *busconfiggopher.ConfigGopher
	HeatOrCoolController HeatOrCoolController
	TemperatureReader    TemperatureReader
	dbFileName           string
	Logger               Logger
}

func NewControlLooper(cg *busconfiggopher.ConfigGopher, HeatOrCoolController HeatOrCoolController, logger Logger) *ControlLooper {
	//TODO maybe we can find the kasa path, test it, and suggest the user how to get it if they don't have it. Of course, it's not necessary if they want to supply a controlFunc
	tmpReader := NewDS18B20Reader(logger)
	cl := ControlLooper{
		Cg:                   cg,
		HeatOrCoolController: HeatOrCoolController,
		TemperatureReader:    tmpReader,
		dbFileName:           "tmplog.dbo",
		Logger:               logger,
	}
	return &cl
}

func (cl *ControlLooper) StartControlLoop() {
	var lastConfigFetched time.Time
	cl.Logger.Printf("%s Fetching initial config\n", stdTimestamp())
	config, source, err := cl.Cg.FetchConfig()
	if err != nil {
		//if the config couldn't be fetched the first time, the application will exit; later on, config reads will be tolerated
		panic(fmt.Sprintf("%s", err.Error()))
	}
	lastConfigFetched = time.Now()
	//isConfigFetchFailing used to track when to notify the server of issues
	isConfigFetchFailing := false
	cl.Logger.Printf("%s Successfully fetched initial config from %s; we'll continue to poll every %d seconds\n%+v\n", stdTimestamp(), source, cl.Cg.ConfigFetchInterval, config)
	cl.Cg.NotifyServer(fmt.Sprintf("%s: we got some config and we're starting up", cl.Cg.ClientId), InfoNotification)
	cl.Logger.Printf("%s Beginning control loop for %d controller(s)\n", stdTimestamp(), len(config.Controllers))

	//enumerate hosts to track if too much time has passed and failingState
	//TODO what happens when a host is removed from the config entirely?
	successfulHostControlTimestamp := make(map[string]time.Time)
	failingHostStates := make(map[string]bool)
	for _, controller := range config.Controllers {
		for _, host := range controller.SwitchHosts {
			successfulHostControlTimestamp[host] = time.Time{}
			failingHostStates[host] = false
		}
	}

	successfulTempReadByControllerName := make(map[string]time.Time) //controller name maps to last timestamp successful
	failingTempReadStates := make(map[string]bool)                   //controller name maps to bool whether it's currently in a failing state

	db, err := NewSqliteDbFromFilename(cl.dbFileName, cl.Logger)
	if err != nil {
		cl.Logger.Printf("Error creating sqlite dbo: %s\n", err)
		cl.Cg.NotifyServer(fmt.Sprintf("Error creating sqlite dbo: %s\n", err), SeriousNotification)
	}
	defer db.Close()

	start := time.Now()
	oneMinuteAfterStart := start.Add(time.Minute) //start to worry if we haven't heard from hosts
	returnChan := make(chan temperatureControlReturn)
	timer := time.Tick(time.Second * 15)
	// Loop forever
	for range timer {
		loopStart := time.Now()
		//if it's been more than the configured interval between fetches, we'll check for new config (note: we start checking every 15 secs)
		if lastConfigFetched.Add(cl.Cg.ConfigFetchInterval).Before(time.Now()) {
			newConfig, source, err := cl.Cg.FetchConfig()
			if err != nil {
				//TODO Factor out this function call
				configSource, ok := cl.Cg.GetSourceKind()
				if ok {
					cl.Logger.Printf("%s We failed to get new config from %s: %#v", stdTimestamp(), configSource, err)
				} else {
					//TODO Notify the server if the error was malformed config, they may have to update it!
					//TODO Notify the server if it's been too long since receiving an up-to-date config
					cl.Logger.Printf("%s We failed to get new config: %#v", stdTimestamp(), err)
				}
				//note: we never modified `config` so things should continue working with the previous config

				//see if we need to notify the server of issues
				if !isConfigFetchFailing && lastConfigFetched.Add(intervalNotifyServerForConfigFetch).Before(time.Now()) {
					isConfigFetchFailing = true
					cl.Cg.NotifyServer(fmt.Sprintf("%s: we haven't received config in %s", cl.Cg.ClientId, intervalNotifyServerForConfigFetch.String()), ProblemNotification)
				}
			} else {
				cl.Logger.Printf("%s We successfully fetched config from %s\n", stdTimestamp(), source)
				if !busconfiggopher.AreConfigsEqual(config, newConfig) {
					cl.Logger.Printf("NEW config!")
					cl.Logger.Printf("%+v\n", newConfig)
					cl.Cg.NotifyServer("We just got some updated config", InfoNotification)
				}
				config = newConfig
				timeElapsedSinceLastConfigFetched := time.Now().Sub(lastConfigFetched)
				lastConfigFetched = time.Now()

				if isConfigFetchFailing { //we just recovered from the config fetch failing
					isConfigFetchFailing = false
					cl.Cg.NotifyServer(fmt.Sprintf("%s: we have recovered from config retrieval issues after %s", cl.Cg.ClientId, timeElapsedSinceLastConfigFetched.String()), ProblemNotification)
				}
			}
		}

		//temp read errors won't be reported for sleeping controllers
		sleepingControllers := make(map[string]bool) //controller name maps to bool whether their sleeping or not
		sleepingHosts := make(map[string]bool)       //host maps to bool if they belong to no awake controller

		for i := range config.Controllers {
			//TODO set a timeout of 12 seconds
			//TODO how can we notify the server when a new temperature rule has been applied for the first time
			go cl.temperatureControl(returnChan, &config.Controllers[i])
		}

		//the idea behind this 2nd loop is to wait for each of the goroutines spun up to finish and report back
		for range config.Controllers {
			//How do we make it so if one loop fails, the other can keep on ticking?
			returnValue := <-returnChan

			//debug code TODO remove
			//(*cl.Logger).Printf("[%s] just returned to main; err: %t, controller sleeping %t\n", returnValue.controllerConfig.Name, err != nil, returnValue.noSchedulesAreActive)

			if returnValue.err != nil {
				cl.Logger.Printf("%s [%s] Error in temperatureControl loop: %s\n", stdTimestamp(), returnValue.controllerConfig.Name, returnValue.err.Error())
			}

			//log 'em if you got 'em
			if (TmpLog{}) != returnValue.tmplog {
				err := db.PersistTmpLog(returnValue.tmplog)
				if err != nil {
					cl.Logger.Printf("%s [%s] Error persisting log to sqlite dbo: %s", stdTimestamp(), returnValue.controllerConfig.Name, err)
					cl.Cg.NotifyServer("We couldn't save a TmpLog to the sqlite dbo", ProblemNotification)
				}
			}

			//analyze which controllers and hosts are asleep
			if returnValue.noSchedulesAreActive {
				sleepingControllers[returnValue.controllerConfig.Name] = true
				for _, host := range returnValue.controllerConfig.SwitchHosts {
					hostSleeping, ok := sleepingHosts[host]
					if !ok || hostSleeping { //be careful not to switch a value from false to true, because as long as the host is awake for one controller, it's generally awake
						sleepingHosts[host] = true
					}
				}
			} else { //controller isn't sleeping
				for _, host := range returnValue.controllerConfig.SwitchHosts {
					sleepingHosts[host] = false
				}
			}

			if !returnValue.successfulTemperatureReadTimestamp.IsZero() {
				successfulTempReadByControllerName[returnValue.controllerConfig.Name] = returnValue.successfulTemperatureReadTimestamp
			}
			successfulHostControlTimestamp = updateSuccessfulHostTimestamps(successfulHostControlTimestamp, returnValue.successfulHostControlTimestamp)
		}
		//debug code: TODO remove
		//(*cl.Logger).Printf("Here are the sleeping controllers: {")
		//for controller := range sleepingControllers {
		//	(*cl.Logger).Printf("%s=%t,", controller, sleepingControllers[controller])
		//}
		//(*cl.Logger).Printf("}\nHere are the sleeping hosts: {")
		//for host := range sleepingHosts {
		//	(*cl.Logger).Printf("%s=%t,", host, sleepingHosts[host])
		//}
		//(*cl.Logger).Printf("}\n")

		nowRef := time.Now()

		//check up on temperature read health for each controller. Only notify the server if we are just entering into (or recovering from) the failing state
		for i := range config.Controllers {
			//see if
			//no need to worry if the controller is asleep
			isControllerAsleep, ok := sleepingControllers[config.Controllers[i].Name]
			if ok && isControllerAsleep {
				previouslyFailing, ok := failingTempReadStates[config.Controllers[i].Name]
				if ok && previouslyFailing {
					failingTempReadStates[config.Controllers[i].Name] = false
					cl.Cg.NotifyServer(fmt.Sprintf("We previously informed that the thermometer for controller %s couldn't be read. Now that controller is sleeping, so we'll ignore the problem for now", config.Controllers[i].Name), SeriousNotification)
				}
				continue
			}

			lastSuccessfulTempRead, ok := successfulTempReadByControllerName[config.Controllers[i].Name]
			if !ok || lastSuccessfulTempRead.IsZero() || lastSuccessfulTempRead.Add(intervalNotifyServerForTempRead).Before(nowRef) {
				//we're failing to read this controller's thermometer

				//do we need to notify the server?
				previouslyFailing, ok := failingTempReadStates[config.Controllers[i].Name]
				if (ok && !previouslyFailing) || !ok {
					failingTempReadStates[config.Controllers[i].Name] = true
					cl.Cg.NotifyServer(fmt.Sprintf("We haven't had contact with the thermometer for controller %s for %s", config.Controllers[i].Name, intervalNotifyServerForTempRead.String()), SeriousNotification)
				}
			} else {
				//we are successfully reading this controller's thermometer. Maybe we need to notify server we have recovered from a failing state
				previouslyFailing, ok := failingTempReadStates[config.Controllers[i].Name]
				if ok && previouslyFailing {
					failingTempReadStates[config.Controllers[i].Name] = false
					cl.Cg.NotifyServer(fmt.Sprintf("We recovered contact with the thermometer for controller %s", config.Controllers[i].Name), SeriousNotification)
				}
			}
		}

		//check up on host communication health to see if we should notify
		for host := range successfulHostControlTimestamp {
			//no need to worry if host is asleep
			isHostAsleep, ok := sleepingHosts[host]
			if ok && isHostAsleep {
				previouslyFailing, ok := failingHostStates[host]
				if ok && previouslyFailing {
					failingHostStates[host] = false
					cl.Cg.NotifyServer(fmt.Sprintf("We previously informed that the host %s couldn't be contacted. That switch-host is no longer associated with an active controller. We'll ignore the problem for now", host), ProblemNotification)
				}
				continue
			}

			//one minute after start is a special case since we won't have any successful timestamps before the first time
			if successfulHostControlTimestamp[host].IsZero() && nowRef.After(oneMinuteAfterStart) {
				if !failingHostStates[host] {
					cl.Logger.Printf("%#v\n", successfulHostControlTimestamp)
					failingHostStates[host] = true
					cl.Cg.NotifyServer(fmt.Sprintf("We started the control loop over a minute ago and we still haven't heard from host %s", host), ProblemNotification)
				}
			} else if !successfulHostControlTimestamp[host].IsZero() && successfulHostControlTimestamp[host].Add(intervalNotifyServerForSwitchHostComm).Before(nowRef) {
				if !failingHostStates[host] {
					cl.Logger.Printf("%#v", successfulHostControlTimestamp)
					failingHostStates[host] = true
					cl.Cg.NotifyServer(fmt.Sprintf("We haven't had contact with switch-host %s since %s", host, successfulHostControlTimestamp[host].Format("2006-01-02 15:04:05")), ProblemNotification)
				}
			} else if failingHostStates[host] { //it seems this host has recovered
				cl.Logger.Printf("%#v", successfulHostControlTimestamp)
				failingHostStates[host] = false
				//we maintain the problem notification urgency to make sure the admin was notified and through the same means
				cl.Cg.NotifyServer(fmt.Sprintf("We recovered contact with switch-host %s at %s", host, successfulHostControlTimestamp[host].Format("2006-01-02 15:04:05")), ProblemNotification)
			}
		}
		cl.Logger.Printf("This iteration of the control loop took %s\n", time.Since(loopStart).String())
	}
}

func updateSuccessfulHostTimestamps(hsMaster map[string]time.Time, hsUpdatesToPerform map[string]time.Time) map[string]time.Time {
	for key := range hsUpdatesToPerform {
		if !hsUpdatesToPerform[key].IsZero() {
			hsMaster[key] = hsUpdatesToPerform[key]
		}
	}
	return hsMaster
}

// @TODO Maybe these should all be configurable with the ControlLooper
const (
	intervalNotifyServerForSwitchHostComm = 5 * time.Minute
	intervalNotifyServerForConfigFetch    = 15 * time.Minute
	intervalNotifyServerForTempRead       = 1 * time.Minute
)

type temperatureControlReturn struct {
	controllerConfig                   *Controller
	successfulTemperatureReadTimestamp time.Time
	//keys are the hostname and values are whether they succeeded or not
	successfulHostControlTimestamp map[string]time.Time
	noSchedulesAreActive           bool
	tmplog                         TmpLog
	err                            error
}

var TemperatureReadError = errors.New("there was a problem reading the current temperature")
var AtLeastOneHostControlFailed = fmt.Errorf("at least one host control failed")

// we'll print directly from this function, prefixing the name of the controller
func (cl *ControlLooper) temperatureControl(retChan chan<- temperatureControlReturn, controllerConfig *Controller) {
	//prepare our channel response
	ret := temperatureControlReturn{
		controllerConfig:               controllerConfig,
		successfulHostControlTimestamp: make(map[string]time.Time),
	}

	desiredTemperature, ok := controllerConfig.GetCurrentDesiredTemperature()
	if !ok {
		ret.noSchedulesAreActive = true
		cl.Logger.Printf("%s [%s]: No temperature schedules have come to pass. We should wait around for a little\n", stdTimestamp(), controllerConfig.Name)
		retChan <- ret
		return
	}
	// Get the current temperature
	weCouldntReadTempPleaseTurnOffControls := false
	currentTemperature, err := cl.TemperatureReader.ReadTemperatureInF(controllerConfig.ThermometerPath)
	if err != nil {
		cl.Logger.Printf("%s [%s]: We had a problem getting current temperature from %#v. Turning off controls just in case. We'll wait a second and try again: %s\n", stdTimestamp(), controllerConfig.Name, controllerConfig.ThermometerPath, err)
		ret.err = errors.Join(TemperatureReadError, err)
		weCouldntReadTempPleaseTurnOffControls = true
	} else {
		ret.successfulTemperatureReadTimestamp = time.Now()
		cl.Logger.Printf("%s [%s]: The latest temperature is %.2f and desired temperature is %.2f\n", stdTimestamp(), controllerConfig.Name, currentTemperature, desiredTemperature)
	}

	//should we turn controls on or off?
	var newState Control
	if weCouldntReadTempPleaseTurnOffControls {
		newState = ControlOff
	} else {
		if controllerConfig.ControlType == "cool" { //our device(s) are coolers
			// If the current temperature is greater than the desired temperature, then turn on the cooling elements
			if currentTemperature > desiredTemperature {
				newState = ControlOn
			} else {
				newState = ControlOff
			}
		} else { //our device(s) are heaters
			// If the current temperature is less than the desired temperature, then turn on the heating elements
			if currentTemperature < desiredTemperature {
				newState = ControlOn
			} else {
				newState = ControlOff
			}
		}
	}
	if !controllerConfig.DisableFreezeProtection && currentTemperature < 33 && newState != ControlOff {
		newState = ControlOff
		cl.Logger.Printf("%s [%s]: FREEZE PROTECTION We're turning off all hosts since the temperature is %.2f\n", stdTimestamp(), controllerConfig.Name, currentTemperature)
	}

	//communicate with the control hosts
	var allHostsSuccessful = true
	successfulHosts := make([]string, 0, len(controllerConfig.SwitchHosts))
	for _, host := range controllerConfig.SwitchHosts {
		cl.Logger.Printf("%s [%s] Turning %s %s\n", stdTimestamp(), controllerConfig.Name, newState, host)
		if cl.HeatOrCoolController == nil {
			panic("HeatOrCoolController is nil")
		}
		err := cl.HeatOrCoolController.ControlDevice(host, newState)
		if err != nil {
			//note: we don't want to send this error to the channel because it will be confusing if there are more hosts. Err is set later
			allHostsSuccessful = false
			ret.successfulHostControlTimestamp[host] = time.Time{}
			var exitError *exec.ExitError
			if errors.As(err, &exitError) { // is our error because it timed out?
				cl.Logger.Printf("%s [%s] Our call to controlDevice timed out after 3 seconds\n", stdTimestamp(), controllerConfig.Name)
			} else {
				cl.Logger.Printf("%s [%s] Our call to controlDevice returned an error: %s\n", stdTimestamp(), controllerConfig.Name, err.Error())
			}
		} else {
			ret.successfulHostControlTimestamp[host] = time.Now()
			successfulHosts = append(successfulHosts, host)
		}

		//TODO can we schedule a failsafe on the device in case we crash next iteration?
	}
	if !allHostsSuccessful {
		if ret.err != nil {
			ret.err = errors.Join(ret.err, AtLeastOneHostControlFailed)
		} else {
			ret.err = AtLeastOneHostControlFailed
		}
	}

	if weCouldntReadTempPleaseTurnOffControls { //don't write to csv if we had issues getting the temperature
		retChan <- ret
		return
	}

	//pass on a pre-formatted log object so our caller can save it
	ret.tmplog = TmpLog{
		ControllerName:        controllerConfig.Name,
		Timestamp:             time.Now(),
		TemperatureInF:        currentTemperature,
		DesiredTemperatureInF: desiredTemperature,
		IsHeatingNotCooling:   controllerConfig.ControlType != "cool",
		TurningOnNotOff:       newState == ControlOn,
		HostsPipeSeparated:    strings.Join(successfulHosts, "|"),
	}

	retChan <- ret
}

func (controller *Controller) GetCurrentDesiredTemperature() (float32, bool) {
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
