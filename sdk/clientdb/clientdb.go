package clientdb

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed sql/migrate.sql
var migrate string

type ClientDB struct {
	db     *sql.DB
	closed bool
}

func New(dataSourceName string) (*ClientDB, error) {
	const connectionParams = "?_pragma=busy_timeout(1000)&_pragma=journal_mode(WAL)"

	dataSourceName = fmt.Sprintf("%s%s", dataSourceName, connectionParams)
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("open connection: %q: %w", dataSourceName, err)
	}

	cln := ClientDB{
		db:     db,
		closed: false,
	}

	return &cln, nil
}

func (cln *ClientDB) Close() error {
	return cln.db.Close()
}

func (cln *ClientDB) Migrate(dataSourceName string) error {
	if _, err := cln.db.Exec(migrate); err != nil {
		return fmt.Errorf("exec migration: %w", err)
	}

	return nil
}

func (cln *ClientDB) Create(query string, args ...any) error {
	statement, err := cln.db.Prepare(query)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}

	if _, err = statement.Exec(args...); err != nil {
		return fmt.Errorf("exec: %w", err)
	}

	return nil
}

func (cln *ClientDB) Query(query string, fields ...any) ([][]any, error) {
	rows, err := cln.db.Query(query)
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
