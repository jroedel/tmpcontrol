// Package busconfiggopher fetches and validates controller config
package busconfiggopher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"github.com/jroedel/tmpcontrol/business/busclient/busadminnotifier"
	"github.com/jroedel/tmpcontrol/foundation/clienttoserverapi"
	"os"
	"slices"
	"time"
)

type ConfigGopher struct {
	cln *clienttoserverapi.Client

	//if both parameters are specified, the local path will be a fallback,
	//The contents of the local config file will be overridden with server
	//data when we get it successfully
	localConfigPath string

	//completely optional
	notify *busadminnotifier.AdminNotifier
}

func New(cln *clienttoserverapi.Client, localConfigPath string, notify *busadminnotifier.AdminNotifier) (*ConfigGopher, error) {
	if cln == nil && localConfigPath == "" {
		return nil, fmt.Errorf("ConfigGopher: we require either a client or localConfigPath")
	}
	return &ConfigGopher{
		cln:             cln,
		localConfigPath: localConfigPath,
		notify:          notify,
	}, nil
}

func (cg *ConfigGopher) GetSourceKind() ConfigSource {
	if cg.cln != nil {
		return ConfigSourceServer
	} else if cg.localConfigPath != "" {
		return ConfigSourceLocalFile
	}
	panic("ConfigGopher: GetSourceKind: this code path should be impossible")
}

// TODO REREAD THIS WHOLE FUNCTION FOR LOGIC ERRORS
func (cg *ConfigGopher) FetchConfig() (ControllersConfig, ConfigSource, error) {
	//TODO notify user/server if there are no configured switchHosts
	if cg.cln != nil {
		config, err := cg.fetchConfigFromServer()
		if err != nil {
			err = fmt.Errorf("fetch config: %v", err)
		} else {
			err = ValidateConfig(config)
			if err != nil {
				err = fmt.Errorf("validate config from server: %w", err)
			} else {
				return config, ConfigSourceServer, nil
			}
		}

		//TODO if we get an error and have a fallback path, read it
		return config, ConfigSourceServer, err
	}

	//fetch from file
	config, err := cg.fetchConfigFromFile()
	err = ValidateConfig(config)
	if err != nil {
		err = fmt.Errorf("validate config from local file: %w", err)
		return ControllersConfig{}, ConfigSourceLocalFile, err
	} else {
		return config, ConfigSourceServer, nil
	}
}

func (cg *ConfigGopher) fetchConfigFromServer() (ControllersConfig, error) {
	result, err := cg.cln.GetConfig()
	if err != nil {
		return ControllersConfig{}, fmt.Errorf("fetchConfigFromServer: %v", err)
	}
	return convertServerResponseToOurConfigModel(result)
}

func convertServerResponseToOurConfigModel(msg clienttoserverapi.ConfigApiMessage) (ControllersConfig, error) {
	//TODO
	return ControllersConfig{}, nil
}

func (cg *ConfigGopher) fetchConfigFromFile() (ControllersConfig, error) {
	file, err := os.Open(cg.localConfigPath)
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

func (cg *ConfigGopher) NotifyAdminIfWeHaventReceivedConfigInInterval(ctx context.Context, interval time.Duration) error {
	//TODO
	return nil
}

func ValidateConfig(config ControllersConfig) error {
	//TODO
	return nil
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
