package db

import (
	"fmt"
	"os"
	"path/filepath"

	"xraytool/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	if err := database.AutoMigrate(
		&model.Admin{},
		&model.Customer{},
		&model.HostIP{},
		&model.XrayNode{},
		&model.SocksOutbound{},
		&model.Order{},
		&model.OrderItem{},
		&model.XrayResource{},
		&model.Setting{},
		&model.TaskLog{},
	); err != nil {
		return nil, err
	}
	return database, nil
}
