package tmpcontrol

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

/**
DB schema

Config
================
clientId TEXT
Config TEXT
UpdatedAt INTEGER

Notifications
=====================
NotificationId PRIMARY KEY
PostedAt INTEGER
clientId TEXT
Message TEXT
Severity INTEGER

*/

const maxConfigBytes = 100000
const maxSqlExecutionTime = 2 * time.Second

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
	          clientId TEXT PRIMARY KEY,
    		  ConfigJson TEXT NOT NULL,
	          UpdatedAt INTEGER NOT NULL
	       );`,
		`CREATE TABLE IF NOT EXISTS notifications (
	          NotificationId TEXT PRIMARY KEY,
	          ReportedAt INTEGER NOT NULL,
	          clientId TEXT NOT NULL,
	          Message TEXT NOT NULL,
	          Severity INTEGER NOT NULL,
	          HasUserBeenNotified INTEGER
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

func (dbo SqliteServerDb) ListNotifications(clientId string) ([]Notification, error) {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()
	rows, err := dbo.db.QueryContext(ctx, "SELECT NotificationId, ReportedAt, Message, Severity, HasUserBeenNotified FROM notifications WHERE clientId = ? LIMIT 50", clientId)
	if err != nil {
		return nil, err
	}
	var notifications []Notification
	for rows.Next() {
		var notification Notification
		notification.ClientId = clientId
		var tmpTime int64
		err := rows.Scan(&notification.NotificationId, &tmpTime, &notification.Message,
			&notification.Severity, &notification.HasUserBeenNotified,
		)
		if err != nil {
			//TODO do we have to sacrifice everything?
			return nil, err
		}
		//TODO test this
		notification.ReportedAt = time.Unix(tmpTime, 0)
		notifications = append(notifications, notification)
	}
	return notifications, nil
}

func (dbo SqliteServerDb) PutNotification(clientId string, note Notification) error {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()

	result, err := dbo.db.ExecContext(ctx, "INSERT INTO notifications (ReportedAt, clientId, Message, Severity, HasUserBeenNotified) VALUES ($1, $2, $3, $4, $5)", note.ReportedAt.Unix(), note.ClientId, note.Message, note.Severity, note.HasUserBeenNotified)
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

func (dbo SqliteServerDb) GetConfig(clientId string) (ControllersConfig, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), maxSqlExecutionTime)
	defer cancel()
	row := dbo.db.QueryRowContext(ctx, "SELECT ConfigJson FROM tmpconfig WHERE clientId = ?", clientId)
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
	row := dbo.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM tmpconfig WHERE clientId = $1)", clientId)
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
	result, err := dbo.db.ExecContext(ctx, "UPDATE tmpconfig SET ConfigJson = $1 WHERE clientId = $2", config, clientId)
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
	result, err := dbo.db.ExecContext(ctx, "INSERT INTO tmpconfig (clientId, ConfigJson, UpdatedAt) VALUES ($1, $2, $3)", clientId, config, now.Unix())
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
