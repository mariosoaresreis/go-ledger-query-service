package repository

import (
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func newMockDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}

	db := sqlx.NewDb(sqlDB, "sqlmock")
	cleanup := func() {
		_ = db.Close()
	}

	return db, mock, cleanup
}

func noRowsError() error {
	return sql.ErrNoRows
}
