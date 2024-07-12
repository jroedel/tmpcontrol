package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/jroedel/tmpcontrol"
	"io"
	"log"
	"net/http"
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
	adminNotifier := SmsNotifier{}
	cg := tmpcontrol.ConfigGopher{ServerRoot: configServerRootUrl, ClientId: clientIdentifier, LocalConfigPath: localConfigPath, ConfigFetchInterval: time.Duration(configFetchIntervalInSeconds) * time.Second, NotifyOutput: adminNotifier}
	cl := tmpcontrol.NewControlLooper(&cg, kasaController, logger)
	cl.StartControlLoop()
}

type SmsNotifier struct {
	io.Writer
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

// notifyAdminViaSms send and pray, this function will receive no logging
func (SmsNotifier) Write(p []byte) (int, error) {
	url := "https://czsrqykgpsseofmwnbja5t46f40tojzd.lambda-url.us-east-2.on.aws/"
	key := os.Getenv("ADMIN_NOTIFY_KEY")
	if key == "" {
		errMessage := "ADMIN_NOTIFY_KEY environment variable not set, unable to notify admin via SMS"
		fmt.Println(errMessage)
		return 0, errors.New(errMessage)
	}
	number := os.Getenv("ADMIN_NOTIFY_NUMBER")
	if number == "" {
		errMessage := "ADMIN_NOTIFY_NUMBER environment variable not set, unable to notify admin via SMS"
		fmt.Println(errMessage)
		return 0, errors.New(errMessage)
	}
	payload := smsPayload{
		Key:     key,
		To:      number,
		Message: "[tmpcontrol]" + string(p),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)
	//respBody, err := io.ReadAll(resp.Body)
	//if err != nil {
	//	return
	//}
	//fmt.Printf("%#v\n", string(respBody))
	return len(p), nil
}

type smsPayload struct {
	Key     string `json:"key"`
	To      string `json:"to"`
	Message string `json:"message"`
}
