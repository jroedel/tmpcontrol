package ds18b20therm

func ReadTemperatureInF(temperaturePath string) (float32, error) {
	return 0.0, nil
}

func EnumerateThermometerPaths() ([]string, error) {
	return []string{}, nil
}

/*
import (
"errors"
"fmt"
"os"
"strconv"
"strings"
"time"
)

type DS18B20Reader struct {
	logger Logger
}

func NewDS18B20Reader(logger Logger) *DS18B20Reader {
	return &DS18B20Reader{
		logger: logger,
	}
}

func (t DS18B20Reader) ReadTemperatureInF(temperaturePath string) (float32, error) {
	var temperatureBytes []byte
	for counter := 1; counter <= 3; counter++ {
		// Read the temperature from the file.
		var err error
		temperatureBytes, err = os.ReadFile(temperaturePath)
		if err != nil {
			return 0, err
		}

		temperatureString := string(temperatureBytes)
		if "" != temperatureString {
			break
		} else {
			t.logger.Printf("Temperature read attempt %d resulted in an empty string", counter)
			// Sleep for 0.2 seconds. //TODO Maybe there's a better way to do this?
			time.Sleep(time.Millisecond * 200)
		}
	}

	temperatureFahrenheit, err := processTemperatureFileBytes(temperatureBytes)
	if err != nil {
		return 0, err
	}
	return temperatureFahrenheit, nil
}

func processTemperatureFileBytes(temperatureBytes []byte) (float32, error) {
	temperatureString := string(temperatureBytes)
	if "" == temperatureString {
		return 0, errors.New("we received an empty string from the temperature file")
	}

	// Trim the line break from the end of the string.
	temperatureString = strings.TrimSuffix(temperatureString, "\n")

	temperature32, err := strconv.ParseFloat(temperatureString, 32)
	if err != nil {
		return 0, err
	}

	temperature := float32(temperature32 / 1000)

	// Convert the temperature from Celsius to Fahrenheit.
	temperatureFahrenheit := temperature*9/5 + 32

	// Check if the temperature is valid and reasonable.
	if temperatureFahrenheit < minValidFahrenheitTemperature || temperatureFahrenheit > maxValidFahrenheitTemperature {
		return 0, fmt.Errorf("invalid temperature: %#v", temperatureFahrenheit)
	}

	return temperatureFahrenheit, nil
}

// ThermometerDevicesRootPath where to look for DS18B20 devices
const ThermometerDevicesRootPath = "/sys/bus/w1/devices/"

// EnumerateThermometerPaths Assuming we're on a Raspberry Pi, check if we can find any DS18B20 devices running
func (t DS18B20Reader) EnumerateThermometerPaths() []string {
	var temperaturePaths []string

	entries, err := os.ReadDir(ThermometerDevicesRootPath)
	if err != nil {
		return temperaturePaths
	}
	for _, entry := range entries {
		//we should probably filter out non-directories, but the IsDir() function wasn't working for some reason
		//check if there's a readable temperature file inside
		//TODO optimize with concurrency
		filePath := ThermometerDevicesRootPath + entry.Name() + "/temperature"
		_, err := os.Open(filePath)
		if err != nil {
			continue //temperature file doesn't exist
		}
		//we have a real file, so continue to verify it's a valid temperature
		temperatureBytes, err := os.ReadFile(filePath)
		if err != nil {
			continue //we couldn't parse the content as a temperature
		}

		_, err = processTemperatureFileBytes(temperatureBytes)
		if err != nil {
			continue
		}

		//it seems we found a real temperature device path
		temperaturePaths = append(temperaturePaths, filePath)
	}
	return temperaturePaths
}
*/
