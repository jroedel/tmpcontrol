package clientsqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// migrate will be executed every time the New function is
// called. For this reason they must be crafted in a way that they
// don't create duplicate data.
//
//go:embed sql/clientmigrate.sql
var migrate string

const queryTimeout = 2 * time.Second

type ClientSqlite struct {
	db     *sql.DB
	closed bool
}

func New(filePath string) (*ClientSqlite, error) {
	const connectionParams = "?_pragma=busy_timeout(1000)&_pragma=journal_mode(WAL)"

	dataSourceName := fmt.Sprintf("%s%s", filePath, connectionParams)
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open connection: %q: %w", dataSourceName, err)
	}

	cln := ClientSqlite{
		db:     db,
		closed: false,
	}

	if _, err := cln.db.Exec(migrate); err != nil {
		return nil, fmt.Errorf("exec migration: %w", err)
	}

	return &cln, nil
}

func (sw *ClientSqlite) Close() error {
	return sw.db.Close()
}

func (sw *ClientSqlite) Create(query string, args ...any) error {
	statement, err := sw.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}

	if _, err = statement.Exec(args...); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

func (sw *ClientSqlite) Query(query string, params []any, fields ...any) ([][]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	rows, err := sw.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	var slice [][]any
	var count int
	for rows.Next() {
		var copyFields []any
		copy(copyFields, fields)

		if err := rows.Scan(copyFields...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		slice = append(slice, []any{})
		slice[count] = copyFields

		count++
	}

	return slice, nil
}

func (sw *ClientSqlite) QueryRow(query string, params []any, fields ...any) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	row := sw.db.QueryRowContext(ctx, query, params...)
	return row.Scan(fields...)
}

// ExecuteQuery no result is returned
func (sw *ClientSqlite) ExecuteQuery(query string, params []any) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	_, err := sw.db.ExecContext(ctx, query, params...)
	return fmt.Errorf("execute query %q: %w", query, err)
}
