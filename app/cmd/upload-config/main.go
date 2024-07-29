package main

import (
	"flag"
	"github.com/jroedel/tmpcontrol/business/busconfiggopher"
	"os"
)

var (
	configPath string
	serverRoot string
	clientId   string
)

func init() {
	flag.StringVar(&configPath, "config", "", "path to config file")
	flag.StringVar(&serverRoot, "server", "", "path to server root")
	flag.StringVar(&clientId, "client-id", "", "client id to upload config for")
}

func main() {
	flag.Parse()
	if serverRoot == "" || configPath == "" || clientId == "" {
		flag.Usage()
		os.Exit(1)
	}
	cg := busconfiggopher.ConfigGopher{ServerRoot: serverRoot, ClientId: clientId}

}
