package main

import (
	"fmt"
	"github.com/jroedel/tmpcontrol"
	"log"
	"os"
	"strings"
)

func main() {
	logger := tmpcontrol.Logger(log.New(os.Stdout, "[tmpcontrol] ", 0))
	tempReader := tmpcontrol.NewDS18B20Reader(logger)
	fmt.Printf("Assuming we're on a Raspberry Pi, we'll check %#v for connected thermometers\n", tmpcontrol.ThermometerDevicesRootPath)
	thermometerPaths := tempReader.EnumerateThermometerPaths()
	if len(thermometerPaths) == 0 {
		fmt.Println("We didn't find any :-(")
	} else {
		fmt.Printf("We found these:\n%s\n", strings.Join(thermometerPaths, "\n"))
	}
}
