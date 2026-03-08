package persistence

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_Sqlite(t *testing.T) {
	db, err := Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = Migrate(db.GormDB(), "sqlite3")
	require.NoError(t, err)
}

func TestConversationSummaryView_Drivers(t *testing.T) {
	sqlite := conversationSummaryView("sqlite")
	require.Contains(t, sqlite, "datetime")
	pg := conversationSummaryView("postgres")
	require.Contains(t, pg, "::text")
	mysqlV := conversationSummaryView("mysql")
	require.Contains(t, mysqlV, "DATE_FORMAT")
	defaultV := conversationSummaryView("unknown")
	require.Contains(t, defaultV, "datetime")
}
