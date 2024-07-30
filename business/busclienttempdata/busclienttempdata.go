// Package busclienttempdata handle temperature data produced by the running of the client
// this includes:
// 1. storing data locally upon request
// 2. periodically send temperature data to the server
// 3. periodically send temperature averages to brewfatherapi
package busclienttempdata

import (
	"errors"
	"github.com/jroedel/tmpcontrol/foundation/brewfatherapi"
	"github.com/jroedel/tmpcontrol/foundation/clientsqlite"
	"github.com/jroedel/tmpcontrol/foundation/clienttoserverapi"
	"time"
)

type TempHandler struct {
	//required
	db *clientsqlite.ClientSqlite

	//optional
	cln   *clienttoserverapi.Client
	bfapi *brewfatherapi.Client
}

func NewTempHandler(db *clientsqlite.ClientSqlite, cln *clienttoserverapi.Client, bfapi *brewfatherapi.Client) (*TempHandler, error) {
	if db == nil {
		return nil, errors.New("db is required")
	}
	return &TempHandler{db: db, cln: cln, bfapi: bfapi}, nil
}

func (th *TempHandler) HandleNewTempData(data TemperatureData) error {
	//TODO
	return nil
}

func (th *TempHandler) EnabledSendingTempDataToServer() bool {
	return th.cln != nil
}
func (th *TempHandler) StartSendingTempDataToServer(interval time.Duration) (cancel chan<- interface{}, err error) {
	if th.cln == nil {
		return nil, errors.New("client not initialized")
	}
	cancel = make(chan interface{})
	go func() {
		//loop every `interval`; lookup unsent temperature data from the db and
		//send it to the server
	}()
	return cancel, nil
}

func (th *TempHandler) EnabledSendingTempAvgToBrewfatherEvery15Minutes() bool {
	return th.bfapi != nil
}

func (th *TempHandler) StartSendingTempAvgToBrewfatherEvery15Minutes() (cancel chan<- interface{}, err error) {
	if th.bfapi == nil {
		return nil, errors.New("client not initialized")
	}
	cancel = make(chan interface{})
	go func() {
		//loop every 15 min; avg recent temperature data from the db and
		//send it to brewfather
	}()
	return cancel, nil
}

type TemperatureData struct {
	//TODO
}
