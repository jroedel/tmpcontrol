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
	"math/rand"
	"time"
)

type TempHandler struct {
	//required
	db *clientsqlite.ClientSqlite

	//optional
	cln   *clienttoserverapi.Client
	bfapi *brewfatherapi.Client

	//internal
	executionID string
}

func New(db *clientsqlite.ClientSqlite, cln *clienttoserverapi.Client, bfapi *brewfatherapi.Client) (*TempHandler, error) {
	if db == nil {
		return nil, errors.New("db is required")
	}

	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const lenExecutionID = 8

	b := make([]byte, lenExecutionID)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}

	return &TempHandler{db: db, cln: cln, bfapi: bfapi, executionID: string(b)}, nil
}

func (th *TempHandler) HandleNewTempData(data Temperature) error {
	return th.create(data)
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

func (th *TempHandler) StartSendingTempAvgToBrewfatherEvery15Minutes() (cancel chan<- interface{}, reterr error) {
	if th.bfapi == nil {
		return nil, errors.New("client not initialized")
	}
	cnl := make(chan interface{})
	cancel = cnl
	go func() {
		//loop every 15 min; avg recent temperature data from the db and
		//send it to brewfather
		tick := time.NewTicker(15 * time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				th.sendAvgToBrewfather()
			case <-cnl:
				return
			}
		}
	}()
	return cancel, nil
}

func (th *TempHandler) sendAvgToBrewfather() {

}
