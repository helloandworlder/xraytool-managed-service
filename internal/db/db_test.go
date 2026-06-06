package db

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenConfiguresSQLiteForConcurrentAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "xraytool.db")

	database, err := Open(path)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	defer sqlDB.Close()

	if got := sqlDB.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections = %d, want 1", got)
	}

	var journalMode string
	if err := database.Raw("PRAGMA journal_mode;").Row().Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := database.Raw("PRAGMA busy_timeout;").Row().Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != sqliteBusyTimeoutMs {
		t.Fatalf("busy_timeout = %d, want %d", busyTimeout, sqliteBusyTimeoutMs)
	}

	var foreignKeys int
	if err := database.Raw("PRAGMA foreign_keys;").Row().Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
}
