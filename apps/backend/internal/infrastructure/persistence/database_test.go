package persistence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen_Sqlite(t *testing.T) {
	db, err := Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	assert.NotNil(t, db.GormDB())
	assert.NotNil(t, db.DB())
	assert.Equal(t, "sqlite3", db.Driver())
}

func TestOpen_SqliteAlias(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	assert.Equal(t, "sqlite", db.Driver())
}

func TestOpen_UnsupportedDriver(t *testing.T) {
	_, err := Open("oracle", "conn")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := Open("sqlite3", "file:/nonexistent/path/db.sqlite?mode=ro")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gorm.Open")
}
