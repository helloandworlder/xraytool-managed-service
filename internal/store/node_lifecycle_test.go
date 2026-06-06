package store

import (
	"fmt"
	"testing"
	"time"

	"xraytool/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupNodeLifecycleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatalf("automigrate failed: %v", err)
	}
	return db
}

func TestGetNodeLifecycleDefaultsToActive(t *testing.T) {
	db := setupNodeLifecycleTestDB(t)
	st := New(db)

	state, err := st.GetNodeLifecycle()
	if err != nil {
		t.Fatalf("GetNodeLifecycle failed: %v", err)
	}
	if state.Status != "ACTIVE" {
		t.Fatalf("status = %q, want ACTIVE", state.Status)
	}
	if state.Message != "" {
		t.Fatalf("message = %q, want empty", state.Message)
	}
}

func TestSetNodeLifecyclePersistsState(t *testing.T) {
	db := setupNodeLifecycleTestDB(t)
	st := New(db)

	state, err := st.SetNodeLifecycle("DRAINING", "preparing retirement")
	if err != nil {
		t.Fatalf("SetNodeLifecycle failed: %v", err)
	}
	if state.Status != "DRAINING" {
		t.Fatalf("status = %q, want DRAINING", state.Status)
	}
	if state.Message != "preparing retirement" {
		t.Fatalf("message = %q, want preparing retirement", state.Message)
	}
	if state.UpdatedAt == "" {
		t.Fatalf("UpdatedAt should not be empty")
	}

	loaded, err := st.GetNodeLifecycle()
	if err != nil {
		t.Fatalf("GetNodeLifecycle after set failed: %v", err)
	}
	if loaded.Status != "DRAINING" {
		t.Fatalf("loaded status = %q, want DRAINING", loaded.Status)
	}
	if loaded.Message != "preparing retirement" {
		t.Fatalf("loaded message = %q, want preparing retirement", loaded.Message)
	}
}

func TestSetNodeLifecycleRejectsInvalidStatus(t *testing.T) {
	db := setupNodeLifecycleTestDB(t)
	st := New(db)

	if _, err := st.SetNodeLifecycle("BROKEN", ""); err == nil {
		t.Fatalf("expected invalid lifecycle status to fail")
	}
}
