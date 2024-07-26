package main

import (
	"flag"
	"log"
	"os"

	"github.com/open-policy-agent/opa/server"
)

var (
	serverAddress string
)

const defaultServerAddress = "localhost:8080"

func init() {
	flag.StringVar(&serverAddress, "server-address", "", "server address, can also be set via environment variable TEMPSERVER_ADDR, default :80")
}

// TODO implement notifications to server admin
func main() {
	flag.Parse()

	// Create logger
	logger := log.New(os.Stdout, "[tmpserver] ", 0)

	s, err := server.New("tmpserver.db", logger)
	if err != nil {
		logger.Fatal(err)
	}

	// run server
	addr := serverAddress //command line arg gets first priority
	if addr == "" {
		addr = os.Getenv("TEMPSERVER_ADDR")
		if addr == "" {
			addr = defaultServerAddress
		} else {
			logger.Printf("We got a server address from env TEMPSERVER_ADDR: %#v", addr)
		}
	} else {
		logger.Printf("We got a server address from command line: %#v", addr)
	}
	s.Address = addr
	s.ListenAndServe()
}
