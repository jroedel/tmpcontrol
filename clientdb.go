package tmpcontrol

import (
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	_ "modernc.org/sqlite"
	"strconv"
	"strings"
	"time"
)

type ClientDb interface {
	PersistTmpLog(tmplog TmpLog) error
	io.Closer
}

type SqliteClientDb struct {
	db                         *sql.DB
	isClosed                   bool
	currentExecutionIdentifier string //random string to identify the current execution of the script
	logger                     Logger
}

func NewSqliteDbFromFilename(filename string, logger Logger) (SqliteClientDb, error) {
	//This variable must include any connection params so it's ready each time we invoke the database
	filename = filename + "?_pragma=busy_timeout(1000)&_pragma=journal_mode(WAL)"
	logger.Printf("We're opening/creating a database at \"%s\"", filename)
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		logger.Printf("Failed to open database at \"%s\": %s", filename, err)
		return SqliteClientDb{}, err
	}

	// get SQLite version
	result := db.QueryRow("select sqlite_version()")
	var version string
	err = result.Scan(&version)
	if err != nil {
		logger.Printf("Failed to query version: %s", err)
		return SqliteClientDb{}, err
	}
	logger.Printf("sqlite version: %s", version)

	//create the table(s)
	sqlCmds := []string{
		//Id is an autoincrement field and shouldn't be specified when inserting rows
		//Timestamp must be stored as RFC3339
		`CREATE TABLE IF NOT EXISTS tmplog (
	          Id INTEGER PRIMARY KEY,
    		  ExecutionIdentifier TEXT NOT NULL,
	          ControllerName TEXT NOT NULL,
	          Timestamp INTEGER NOT NULL,
	          TemperatureInF TEXT NULL,
	          DesiredTemperatureInF TEXT NULL,
	          IsHeatingNotCooling INTEGER NOT NULL,
	          TurningOnNotOff INTEGER NOT NULL,
	          HostsPipeSeparated TEXT NULL,
	          HasBeenSentToServer INTEGER NOT NULL
	       );`,
	}
	for _, v := range sqlCmds {
		_, err = db.Exec(v)
		if err != nil {
			logger.Printf("Failed to construct database. Couldn't execute command \"%s\": %s", v, err)
			return SqliteClientDb{}, err
		}
	}

	return SqliteClientDb{db: db, logger: logger, currentExecutionIdentifier: generateRandomExecutionIdentifier()}, nil
}

func (dbo SqliteClientDb) Close() error {
	return dbo.db.Close()
}

func generateRandomExecutionIdentifier() string {
	return randString(8)
}

func (dbo SqliteClientDb) PersistTmpLog(tmplog TmpLog) error {
	statement, _ := dbo.db.Prepare("INSERT INTO tmplog (ExecutionIdentifier, ControllerName, Timestamp, TemperatureInF, DesiredTemperatureInF, IsHeatingNotCooling, TurningOnNotOff, HostsPipeSeparated, HasBeenSentToServer) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	_, err := statement.Exec(dbo.currentExecutionIdentifier, tmplog.ControllerName, tmplog.Timestamp.Unix(), tmplog.TemperatureInF, tmplog.DesiredTemperatureInF, tmplog.IsHeatingNotCooling, tmplog.TurningOnNotOff, tmplog.HostsPipeSeparated, false)
	if err != nil {
		return err
	}
	return nil
}

func (dbo SqliteClientDb) FetchTmpLogsNotYetSentToServer() ([]TmpLog, error) {
	rows, _ := dbo.db.Query("SELECT Id, ExecutionIdentifier, ControllerName, Timestamp, TemperatureInF, DesiredTemperatureInF, IsHeatingNotCooling, TurningOnNotOff, HostsPipeSeparated FROM tmplog WHERE HasBeenSentToServer = 0")
	//30 is just a guess of how many rows we're getting
	tmpLogs := make([]TmpLog, 0, 30)
	var tempTmpLog TmpLog
	var tempTimestampStr int64
	for rows.Next() {
		rows.Scan(&tempTmpLog.DbAutoId, &tempTmpLog.ExecutionIdentifier, &tempTmpLog.ControllerName, &tempTimestampStr, &tempTmpLog.TemperatureInF, &tempTmpLog.DesiredTemperatureInF, &tempTmpLog.IsHeatingNotCooling, &tempTmpLog.TurningOnNotOff, &tempTmpLog.HostsPipeSeparated)
		tempTimestamp := time.Unix(tempTimestampStr, 0)
		tempTmpLog.Timestamp = tempTimestamp
		tmpLogs = append(tmpLogs, tempTmpLog)
	}
	return tmpLogs, nil
}

func (dbo SqliteClientDb) GetAverageRecentTemperature(controllerName string, d time.Duration) (float32, error) {
	timestampRef := time.Now().Add(-d).Unix()
	row := dbo.db.QueryRow("SELECT AVG(TemperatureInF) FROM tmplog WHERE ExecutionIdentifier = ? AND ControllerName = ? AND Timestamp >= ?", dbo.currentExecutionIdentifier, controllerName, timestampRef)
	var avgTemp float32
	err := row.Scan(&avgTemp)
	if err != nil {
		return 0, err
	}
	return avgTemp, nil
}

// MarkTmpLogsAsSentToServer TODO implement a limit to how many can be marked complete at at time
func (dbo SqliteClientDb) MarkTmpLogsAsSentToServer(ids []int) error {
	idListBuilder := strings.Builder{}

	writeLeadingComma := false
	for _, id := range ids {
		if writeLeadingComma {
			idListBuilder.WriteRune(',')
		}
		idListBuilder.WriteString(strconv.Itoa(id))
		writeLeadingComma = true
	}

	idList := idListBuilder.String()
	fmt.Printf("Here's the list of Ids we'll mark as sent: %#v\n", idList)
	cmd := "UPDATE tmplog SET HasBeenSentToServer = TRUE WHERE Id IN (" + idList + ")"
	_, err := dbo.db.Exec(cmd)
	return err
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go
// https://stackoverflow.com/a/31832326
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}
