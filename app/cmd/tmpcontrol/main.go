package main

import (
	"flag"
	"fmt"
	"github.com/jroedel/tmpcontrol"
	"log"
	"os"
	"strings"
	"time"
)

var (
	configServerRootUrl          string
	kasaPath                     string
	clientIdentifier             string
	localConfigPath              string
	configFetchIntervalInSeconds int
)

func init() {
	flag.StringVar(&kasaPath, "kasa-path", "", "The path to the kasa executable") //no default value, because we need to know if the user submitted it or not
	flag.StringVar(&configServerRootUrl, "config-server-root-url", "", "The root url of the control server")
	flag.StringVar(&localConfigPath, "local-config-path", "", "The path to a local configuration file")
	flag.StringVar(&clientIdentifier, "client-identifier", "", "The string to identify ourselves to the server")
	flag.IntVar(&configFetchIntervalInSeconds, "config-fetch-interval", 60, "The number of seconds between polling the config file or server")
}

/*
@TODO Notify admin within 2 degrees of boiling or freezing

build for raspberry pi using `env GOOS=linux GOARCH=arm GOARM=6 go build`
*/
func main() {
	flag.Parse()
	if err := validateParams(); err != nil {
		log.Fatal(err)
	}
	logger := tmpcontrol.Logger(log.New(os.Stdout, "[tmpcontrol] ", 0))
	//db, err := tmpcontrol.NewSqliteDbFromFilename("tmps.db", logger)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer db.Close()
	//err = db.PersistTmpLog(tmpcontrol.TmpLog{
	//	ControllerName:        "test",
	//	Timestamp:             time.Now(),
	//	TemperatureInF:        44.2,
	//	DesiredTemperatureInF: 33,
	//	IsHeatingNotCooling:   false,
	//	TurningOnNotOff:       true,
	//	HostsPipeSeparated:    "",
	//})
	//if err != nil {
	//	fmt.Printf("Error persisting tmp log: %s\n", err)
	//}
	//
	//tmplogs, err := db.FetchTmpLogsNotYetSentToServer()
	//if err != nil {
	//	fmt.Printf("Error fetching tmp logs: %s\n", err)
	//}
	//fmt.Printf("Fetched %d tmp logs:\n%+v", len(tmplogs), tmplogs)
	//
	//idsToMarkSentToServer := make([]int, 0, len(tmplogs))
	//for _, obj := range tmplogs {
	//	idsToMarkSentToServer = append(idsToMarkSentToServer, obj.DbAutoId)
	//}
	//fmt.Printf("Ids to mark sent to server: %#v\n", idsToMarkSentToServer)
	//db.MarkTmpLogsAsSentToServer(idsToMarkSentToServer)
	//return
	tempReader := tmpcontrol.NewDS18B20Reader(logger)
	fmt.Printf("Assuming we're on a Raspberry Pi, we'll check %#v for connected thermometers\n", tmpcontrol.ThermometerDevicesRootPath)
	thermometerPaths := tempReader.EnumerateThermometerPaths()
	if len(thermometerPaths) == 0 {
		fmt.Println("We didn't find any :-(")
	} else {
		fmt.Printf("We found these:\n%s\n", strings.Join(thermometerPaths, "\n"))
	}

	kasaController := tmpcontrol.HeatOrCoolController(tmpcontrol.NewKasaHeatOrCoolController(kasaPath))
	adminNotifier := tmpcontrol.SmsNotifier{}
	cg := tmpcontrol.ConfigGopher{ServerRoot: configServerRootUrl, ClientId: clientIdentifier, LocalConfigPath: localConfigPath, ConfigFetchInterval: time.Duration(configFetchIntervalInSeconds) * time.Second, NotifyOutput: adminNotifier}
	cl := tmpcontrol.NewControlLooper(&cg, kasaController, logger)
	cl.StartControlLoop()
}

// validate user input
func validateParams() error {
	//we need a clientIdentifier if a server url has been specified by user
	if configServerRootUrl != "" {
		//handle a blank clientIdentifier
		if clientIdentifier == "" {
			return fmt.Errorf("please specify the client identifier `tmpcontrol -client-identifier our-name`")
		}
		//validate clientIdentifier
		if !tmpcontrol.ClientIdentifiersRegex.MatchString(clientIdentifier) {
			return fmt.Errorf("the client identifier must match the regular expression: %s", tmpcontrol.ClientIdentifiersRegex.String())
		}
	}

	//set kasa path
	if kasaPath == "" {
		kasaPath = os.Getenv("KASA_PATH")
		if kasaPath == "" {
			kasaPath = "kasa"
		}
	}

	//TODO check if we can execute kasa
	return nil
}
