package tmpcontrol_test

import (
	"encoding/json"
	"github.com/jroedel/tmpcontrol"
	"log"
	"os"
	"path"
	"testing"
)

func TestServerDB(t *testing.T) {
	filePath := path.Join(os.TempDir(), "tempserverdb")
	os.Remove(filePath) //start fresh
	defer os.Remove(filePath)
	logger := log.New(os.Stdout, "[tmpserverdb_test] ", 0)
	dbo, err := tmpcontrol.NewSqliteServerDbFromFilename(filePath, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer dbo.Close()

	clientId := "test-client"
	_, ok, err := dbo.GetConfig(clientId)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("We expected an empty config")
	}

	testConfigStr := `
{
  "controllers": [
    {
      "name":"test-config",
      "thermometerPath": "../../temperature.txt",
      "controlType": "cool",
      "switchHosts": ["192.168.2.161"],
      "disableFreezeProtection": false,
      "temperatureSchedule": {
        "2024-06-28T00:00:00Z": 33
      }
    }
  ]
}`
	testConfig := tmpcontrol.ControllersConfig{}
	err = json.Unmarshal([]byte(testConfigStr), &testConfig)
	if err != nil {
		t.Fatal(err)
	}
	err = dbo.CreateOrUpdateConfig(clientId, testConfig)
	if err != nil {
		t.Fatal(err)
	}

	returnedConfig, ok, err := dbo.GetConfig(clientId)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("We expected the config we just stored")
	}
	if !tmpcontrol.AreConfigsEqual(testConfig, returnedConfig) {
		t.Fatal("We expected the config to be the same")
	}

	config2 := returnedConfig
	config2.Controllers[0].Name = "this is a new name to test config dbo update"
	err = dbo.CreateOrUpdateConfig(clientId, config2)
	if err != nil {
		t.Fatal(err)
	}
	returnedConfig2, ok, err := dbo.GetConfig(clientId)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("We expected the config we just stored")
	}
	if !tmpcontrol.AreConfigsEqual(config2, returnedConfig2) {
		t.Fatal("We expected the config to be the same")
	}
}
