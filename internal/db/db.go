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
	if err := migrateOrderItemUsernameIndex(database); err != nil {
		return nil, err
	}
	return database, nil
}

func migrateOrderItemUsernameIndex(database *gorm.DB) error {
	migrator := database.Migrator()
	for _, name := range []string{"idx_order_items_auth", "idx_order_items_username"} {
		if migrator.HasIndex(&model.OrderItem{}, name) {
			if err := migrator.DropIndex(&model.OrderItem{}, name); err != nil {
				return err
			}
		}
	}
	return database.Exec("CREATE INDEX IF NOT EXISTS idx_order_items_auth ON order_items(username)").Error
}
