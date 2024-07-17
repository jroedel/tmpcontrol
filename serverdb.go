package tmpcontrol

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

/**
DB schema

Really the main thing is saving the configuration

Config
================
ClientId TEXT
Config TEXT
UpdatedAt INTEGER

ClientCheckin
================
ClientId TEXT

*/

const maxConfigBytes = 100000
const maxSqlExecutionTime = 2 * time.Second

type ServerDb interface {
	//Configs: the main purpose of this server, to receive and server client config
	CreateOrUpdateConfig(clientId string, config ControllersConfig) error
	GetConfig(clientId string) (ControllersConfig, bool, error)

	//Check-ins: meant to detect offline clients
	//ClientIdCheckIn(clientId string) error
	//GetLastClientIdCheckIn(clientId string) (time.Time, error)
	io.Closer
}

type SqliteServerDb struct {
	db       *sql.DB
	isClosed bool
	logger   Logger
}

func NewSqliteServerDbFromFilename(filename string, logger Logger) (SqliteServerDb, error) {
	//This variable must include any connection params so it's ready each time we invoke the database
	filename = filename + "?_pragma=busy_timeout(1000)&_pragma=journal_mode(WAL)"
	logger.Printf("We're opening/creating a database at \"%s\"", filename)
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		logger.Printf("Failed to open database at \"%s\": %s", filename, err)
		return SqliteServerDb{}, err
	}

	// get SQLite version
	result := db.QueryRow("select sqlite_version()")
	var version string
	err = result.Scan(&version)
	if err != nil {
		logger.Printf("Failed to query version: %s", err)
		return SqliteServerDb{}, err
	}
	logger.Printf("sqlite version: %s", version)

	//create the table(s)
	sqlCmds := []string{
		//Id is an autoincrement field and shouldn't be specified when inserting rows
		//Timestamp must be stored as RFC3339
		`CREATE TABLE IF NOT EXISTS tmpconfig (
	          ClientId TEXT PRIMARY KEY,
    		  ConfigJson TEXT NOT NULL,
	          UpdatedAt INTEGER NOT NULL
	       );`,
	}
	for _, v := range sqlCmds {
		_, err = db.Exec(v)
		if err != nil {
			logger.Printf("Failed to construct database. Couldn't execute command \"%s\": %s", v, err)
			return SqliteServerDb{}, err
		}
	}

	return SqliteServerDb{db: db, logger: logger}, nil
}

func (dbo SqliteServerDb) Close() error {
	return dbo.db.Close()
}

func (dbo SqliteServerDb) GetConfig(clientId string) (ControllersConfig, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()
	row := dbo.db.QueryRowContext(ctx, "SELECT ConfigJson FROM tmpconfig WHERE ClientId = ?", clientId)
	var configBytes []byte
	err := row.Scan(&configBytes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ControllersConfig{}, false, nil
		}
		return ControllersConfig{}, false, err
	}
	config := ControllersConfig{}
	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return ControllersConfig{}, false, err
	}
	return config, true, nil
}

// CreateOrUpdateConfig TODO test me
func (dbo SqliteServerDb) CreateOrUpdateConfig(clientId string, config ControllersConfig) error {
	exists, err := dbo.existsClientIdConfig(clientId)
	if err != nil {
		return fmt.Errorf("error checking if client id exists: %s", err)
	}
	configBytes, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshalling config: %s", err)
	}
	if len(configBytes) > maxConfigBytes {
		return fmt.Errorf("config is too large: %d > %d", len(configBytes), maxConfigBytes)
	}
	if exists {
		err = dbo.updateConfig(clientId, configBytes)
		if err != nil {
			return fmt.Errorf("error updating config %s: %s", clientId, err)
		}
	} else {
		err = dbo.createConfig(clientId, configBytes)
		if err != nil {
			return fmt.Errorf("error creating config %s: %s", clientId, err)
		}
	}
	return nil
}

func (dbo SqliteServerDb) existsClientIdConfig(clientId string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()
	row := dbo.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM tmpconfig WHERE ClientId = $1)", clientId)
	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		} else {
			return false, err
		}
	}
	return exists, nil
}

func (dbo SqliteServerDb) updateConfig(clientId string, config []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()
	result, err := dbo.db.ExecContext(ctx, "UPDATE tmpconfig SET ConfigJson = $1 WHERE ClientId = $2", config, clientId)
	if err != nil {
		return err
	}
	rowsCount, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsCount == 0 {
		return fmt.Errorf("no rows affected")
	} else if rowsCount > 1 {
		return fmt.Errorf("too many rows affected")
	}
	return nil
}

func (dbo SqliteServerDb) createConfig(clientId string, config []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()

	now := time.Now()
	result, err := dbo.db.ExecContext(ctx, "INSERT INTO tmpconfig (ClientId, ConfigJson, UpdatedAt) VALUES ($1, $2, $3)", clientId, config, now.Unix())
	if err != nil {
		return err
	}
	rowsCount, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsCount == 0 {
		return fmt.Errorf("no rows affected")
	} else if rowsCount > 1 {
		return fmt.Errorf("too many rows affected")
	}
	return nil
}
