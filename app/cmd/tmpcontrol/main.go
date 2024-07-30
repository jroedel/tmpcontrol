package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jroedel/tmpcontrol/app/sdk/apptmpcontrol"
	"github.com/jroedel/tmpcontrol/business/busclient/busadminnotifier"
	"github.com/jroedel/tmpcontrol/business/busclient/busclienttempdata"
	"github.com/jroedel/tmpcontrol/business/busclient/busconfiggopher"
	"github.com/jroedel/tmpcontrol/foundation/brewfatherapi"
	"github.com/jroedel/tmpcontrol/foundation/clientsqlite"
	"github.com/jroedel/tmpcontrol/foundation/clienttoserverapi"
	"github.com/jroedel/tmpcontrol/foundation/ctlkasaplug"
	"github.com/jroedel/tmpcontrol/foundation/ds18b20therm"
	"github.com/jroedel/tmpcontrol/foundation/sms"
	"log"
	"os"
	"strings"
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
	logger := log.New(os.Stdout, "[tmpcontrol] ", 0)

	fmt.Println("Assuming we're on a Raspberry Pi, we'll check for connected thermometers")
	thermometerPaths, _ := ds18b20therm.EnumerateThermometerPaths()
	if len(thermometerPaths) == 0 {
		fmt.Println("We didn't find any :-(")
	} else {
		fmt.Printf("We found these:\n%s\n", strings.Join(thermometerPaths, "\n"))
	}

	smsKey := os.Getenv("SMS_NOTIFY_KEY")
	smsNumber := os.Getenv("SMS_NOTIFY_NUMBER")
	var smsApi *sms.Sms
	if smsKey != "" && smsNumber != "" {
		var err error
		smsApi, err = sms.New(smsKey)
		if err != nil {
			logger.Printf("Failed to create SMS client: %v", err)
		}
	} else {
		smsKey = ""
		smsNumber = ""
	}

	var cln *clienttoserverapi.Client
	if configServerRootUrl != "" && clientIdentifier != "" {
		var err error
		cln, err = clienttoserverapi.New(configServerRootUrl, clientIdentifier)
		if err != nil {
			logger.Printf("Failed to create config API client: %v", err)
		}
	}

	var notify *busadminnotifier.AdminNotifier
	if smsApi != nil || cln != nil {
		var err error
		notify, err = busadminnotifier.New(smsApi, smsNumber, cln)
		if err != nil {
			logger.Printf("Failed to create admin notifier: %v", err)
		}
	}

	const dbPath = "tmpclient.db"
	db, err := clientsqlite.New(dbPath)
	if err != nil {
		if notify != nil {
			notify.NotifyAdmin(fmt.Sprintf("Error creating sqlite dbo: %s\n", err), clienttoserverapi.SeriousNotification)
		}
		logger.Fatalf("Error creating sqlite dbo: %s\n", err)
	}
	defer db.Close()

	bfLogId := os.Getenv("BREWFATHER_LOG_ID")
	var bfapi *brewfatherapi.Client
	if bfLogId != "" {
		bfapi, err = brewfatherapi.New(bfLogId)
	}

	th, err := busclienttempdata.New(db, cln, bfapi)
	if err != nil {
		logger.Fatalf("create temp handler: %v", err)
	}

	cg, err := busconfiggopher.New(cln, localConfigPath, notify)
	if err != nil {
		logger.Fatalf("create config gopher: %v", err)
	}

	kasa, err := ctlkasaplug.New(kasaPath)
	if err != nil {
		logger.Fatalf("create kasa: %v", err)
	}

	app, err := apptmpcontrol.New(cg, th, kasa, logger, notify)
	if err != nil {
		logger.Fatalf("Error creating app: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = app.Start(ctx)
	if err != nil {
		logger.Fatalf("Error starting app: %v", err)
	}

	select {
	case <-ctx.Done():
		fmt.Println("Is this even possible?")
	}
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
		if !clienttoserverapi.ClientIdRegex.MatchString(clientIdentifier) {
			return fmt.Errorf("the client identifier must match the regular expression: %s", clienttoserverapi.ClientIdRegex.String())
		}
	}

	//set kasa path
	if kasaPath == "" {
		kasaPath = os.Getenv("KASA_PATH")
		if kasaPath == "" {
			kasaPath = "kasa"
		}
	}
	return nil
}
