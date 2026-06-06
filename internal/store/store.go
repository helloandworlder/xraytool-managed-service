package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

const (
	NodeLifecycleStatusActive         = "ACTIVE"
	NodeLifecycleStatusDraining       = "DRAINING"
	NodeLifecycleStatusRetired        = "RETIRED"
	NodeLifecycleStatusDecommissioned = "DECOMMISSIONED"
	nodeLifecycleStatusKey            = "node_lifecycle_status"
	nodeLifecycleMessageKey           = "node_lifecycle_message"
	nodeLifecycleUpdatedAtKey         = "node_lifecycle_updated_at"
)

type NodeLifecycleState struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) EnsureDefaultSettings(defaultPort int, barkBase string, extraDefaults map[string]string) error {
	defaults := map[string]string{
		"default_inbound_port":     strconv.Itoa(defaultPort),
		"default_inbound_listen":   "0.0.0.0",
		"bark_enabled":             "false",
		"bark_base_url":            barkBase,
		"bark_device_key":          "",
		"bark_group":               "xraytool",
		"xray_api_server":          "127.0.0.1:10085",
		"dedicated_vless_security": "tls",
		"dedicated_vless_sni":      "",
		"dedicated_vless_type":     "tcp",
		"dedicated_vless_path":     "",
		"dedicated_vless_host":     "",
		"residential_name_prefix":  "家宽-Socks5",
		nodeLifecycleStatusKey:     NodeLifecycleStatusActive,
		nodeLifecycleMessageKey:    "",
		nodeLifecycleUpdatedAtKey:  "",
	}
	for k, v := range extraDefaults {
		defaults[k] = v
	}
	now := time.Now()
	for k, v := range defaults {
		entry := model.Setting{Key: k, Value: v, UpdatedAt: now}
		if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetSettings() (map[string]string, error) {
	var rows []model.Setting
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Key] = row.Value
	}
	return out, nil
}

func (s *Store) SetSettings(values map[string]string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		for k, v := range values {
			row := model.Setting{Key: k, Value: v, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) GetSetting(key string) (string, error) {
	var row model.Setting
	if err := s.db.First(&row, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return row.Value, nil
}

func (s *Store) EnsureAdmin(username, passwordHash string) error {
	var count int64
	if err := s.db.Model(&model.Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	admin := model.Admin{Username: username, PasswordHash: passwordHash}
	return s.db.Create(&admin).Error
}

func (s *Store) ResetAdminPassword(username, passwordHash string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var admin model.Admin
		if err := tx.First(&admin, "username = ?", username).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				admin = model.Admin{Username: username, PasswordHash: passwordHash}
				return tx.Create(&admin).Error
			}
			return err
		}
		admin.PasswordHash = passwordHash
		return tx.Save(&admin).Error
	})
}

func (s *Store) AddTaskLog(level, msg, detail string) {
	_ = s.db.Create(&model.TaskLog{Level: level, Message: msg, Detail: detail}).Error
}

func (s *Store) GetNodeLifecycle() (NodeLifecycleState, error) {
	values, err := s.GetSettings()
	if err != nil {
		return NodeLifecycleState{}, err
	}
	status, err := normalizeNodeLifecycleStatus(values[nodeLifecycleStatusKey])
	if err != nil {
		status = NodeLifecycleStatusActive
	}
	return NodeLifecycleState{
		Status:    status,
		Message:   strings.TrimSpace(values[nodeLifecycleMessageKey]),
		UpdatedAt: strings.TrimSpace(values[nodeLifecycleUpdatedAtKey]),
	}, nil
}

func (s *Store) SetNodeLifecycle(status string, message string) (NodeLifecycleState, error) {
	normalized, err := normalizeNodeLifecycleStatus(status)
	if err != nil {
		return NodeLifecycleState{}, err
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.SetSettings(map[string]string{
		nodeLifecycleStatusKey:    normalized,
		nodeLifecycleMessageKey:   strings.TrimSpace(message),
		nodeLifecycleUpdatedAtKey: updatedAt,
	}); err != nil {
		return NodeLifecycleState{}, err
	}
	return NodeLifecycleState{
		Status:    normalized,
		Message:   strings.TrimSpace(message),
		UpdatedAt: updatedAt,
	}, nil
}

func normalizeNodeLifecycleStatus(status string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(status))
	switch normalized {
	case "", NodeLifecycleStatusActive:
		return NodeLifecycleStatusActive, nil
	case NodeLifecycleStatusDraining, NodeLifecycleStatusRetired, NodeLifecycleStatusDecommissioned:
		return normalized, nil
	default:
		return "", fmt.Errorf("invalid node lifecycle status %q", status)
	}
}
