package tmpcontrol

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

type ConfigGopher struct {
	LocalConfigPath string
	//ServerRoot includes the protocol scheme, hostname and port. Trailing '/' is optional
	ServerRoot string
	//ClientId the client identifier to let the server know who we are
	ClientId            string
	ConfigFetchInterval time.Duration
	//if a Writer is defined, server notifications will be written additionally to this Writer
	NotifyOutput io.Writer
}

type ServerNotificationUrgency int

const (
	InfoNotification ServerNotificationUrgency = iota + 1
	ProblemNotification
	SeriousNotification
)

func (cg *ConfigGopher) SendConfig(config ControllersConfig) error {
	//TODO POST
	return nil
}

// NotifyServer
// Send the server a message
// If the server can't be contacted, queue the message in a text file
// maybe we can restructure all the logging code to use structured messages (with error levels). Above a certain error level could be automatically reported
func (cg *ConfigGopher) NotifyServer(message string, urgency ServerNotificationUrgency) {
	fmt.Println("NOTIFYING SERVER: ", message)
	if cg.NotifyOutput != nil {
		_, _ = cg.NotifyOutput.Write([]byte(message))
	}
}

func (cg *ConfigGopher) GetSourceKind() (ConfigSource, bool) {
	if err := cg.HasError(); err != nil {
		return 0, false
	}
	if cg.ServerRoot != "" {
		return ConfigSourceServer, true
	} else if cg.LocalConfigPath != "" {
		return ConfigSourceLocalFile, true
	}
	return 0, false
}

func (cg *ConfigGopher) FetchConfig() (ControllersConfig, error) {
	if err := cg.HasError(); err != nil {
		return ControllersConfig{}, err
	}

	//TODO notify user/server if there are no configured switchHosts
	if cg.ServerRoot != "" {
		config, err := cg.fetchConfigFromServer()
		if err == nil {
			config.Source = ConfigSourceServer
		}
		return config, err
	} else if cg.LocalConfigPath != "" {
		//fetch from file
		config, err := cg.fetchConfigFromFile()
		if err == nil {
			config.Source = ConfigSourceLocalFile
		}
		return config, err
	}

	//TODO validate configuration. for example, no duplicate Controller names, also "heat"/"cool"
	return ControllersConfig{}, fmt.Errorf("please specify a configuration file path or control server url")
}

func (cg *ConfigGopher) HasError() error {
	//we need a clientIdentifier if a server url has been specified by user
	if cg.ServerRoot != "" {
		//handle a blank clientIdentifier
		if cg.ClientId == "" {
			return fmt.Errorf("ConfigGopher: we received a blank ClientId")
		}
		//validate clientIdentifier
		if !ClientIdentifiersRegex.MatchString(cg.ClientId) {
			return fmt.Errorf("ConfigGopher: ClientId must match the regular expression: %s", ClientIdentifiersRegex.String())
		}
	} else if cg.LocalConfigPath == "" {
		return fmt.Errorf("ConfigGopher: we require either a ServerRoot or LocalConfigPath")
	}
	return nil
}

func (cg *ConfigGopher) fetchConfigFromServer() (ControllersConfig, error) {
	err := cg.HasError()
	if err != nil {
		return ControllersConfig{}, err
	}
	url := cg.getServerRequestUrl()

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ControllersConfig{}, err
	}
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return ControllersConfig{}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(response.Body)
	if response.StatusCode != http.StatusOK {
		return ControllersConfig{}, fmt.Errorf("server responded with %d", response.StatusCode)
	}
	decoder := json.NewDecoder(response.Body)
	var config ControllersConfig
	err = decoder.Decode(&config)
	if err != nil {
		return ControllersConfig{}, err
	}
	return config, nil
}

func (cg *ConfigGopher) getServerRequestUrl() string {
	//TODO what should happen if there's no server root??

	url := cg.ServerRoot
	if !strings.HasSuffix(url, "/") {
		url = url + "/"
	}
	url = url + "configuration/" + cg.ClientId
	return url
}

func (cg *ConfigGopher) fetchConfigFromFile() (ControllersConfig, error) {
	file, err := os.Open(cg.LocalConfigPath)
	if err != nil {
		return ControllersConfig{}, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)
	rd := bufio.NewReader(file)

	var config ControllersConfig
	dec := json.NewDecoder(rd)
	if err := dec.Decode(&config); err != nil {
		return ControllersConfig{}, err
	}

	//make sure there aren't two controllers reading from the same thermometer
	thermometers := make([]string, 0, len(config.Controllers))
	for _, controller := range config.Controllers {
		if slices.Contains(thermometers, controller.ThermometerPath) {
			//problem

		}
		thermometers = append(thermometers, controller.ThermometerPath)
	}
	return config, nil
}

// AreConfigsEqual Based on https://stackoverflow.com/questions/48253423/unique-hash-from-struct
func AreConfigsEqual(a ControllersConfig, b ControllersConfig) bool {
	return compareConfigHashes(hashConfig(a), hashConfig(b))
}

func compareConfigHashes(a, b []byte) bool {
	a = append(a, b...)
	c := 0
	for _, x := range a {
		c ^= int(x)
	}
	return c == 0
}

func hashConfig(c ControllersConfig) []byte {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(c)
	if err != nil {
		return nil
	}
	return b.Bytes()
}
