package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

type DedicatedEntryService struct {
	db *gorm.DB
}

type DedicatedEntryInput struct {
	Name            string   `json:"name"`
	Domain          string   `json:"domain"`
	MixedPort       int      `json:"mixed_port"`
	VmessPort       int      `json:"vmess_port"`
	VlessPort       int      `json:"vless_port"`
	ShadowsocksPort int      `json:"shadowsocks_port"`
	Priority        int      `json:"priority"`
	Features        []string `json:"features"`
	Enabled         *bool    `json:"enabled"`
	Notes           string   `json:"notes"`
}

func NewDedicatedEntryService(db *gorm.DB) *DedicatedEntryService {
	return &DedicatedEntryService{db: db}
}

func (s *DedicatedEntryService) List() ([]model.DedicatedEntry, error) {
	rows := []model.DedicatedEntry{}
	if err := s.db.Order("enabled desc, priority asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DedicatedEntryService) Create(in DedicatedEntryInput) (*model.DedicatedEntry, error) {
	row, err := normalizeDedicatedEntryInput(in)
	if err != nil {
		return nil, err
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *DedicatedEntryService) Update(id uint, in DedicatedEntryInput) (*model.DedicatedEntry, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	base := model.DedicatedEntry{}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	row, err := normalizeDedicatedEntryInput(in)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"name":             row.Name,
		"domain":           row.Domain,
		"mixed_port":       row.MixedPort,
		"vmess_port":       row.VmessPort,
		"vless_port":       row.VlessPort,
		"shadowsocks_port": row.ShadowsocksPort,
		"priority":         row.Priority,
		"features":         row.Features,
		"enabled":          row.Enabled,
		"notes":            row.Notes,
		"updated_at":       time.Now(),
	}
	if err := s.db.Model(&model.DedicatedEntry{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	return &base, nil
}

func (s *DedicatedEntryService) Toggle(id uint, enabled bool) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.db.Model(&model.DedicatedEntry{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

func (s *DedicatedEntryService) Delete(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	var count int64
	if err := s.db.Model(&model.Order{}).
		Where("dedicated_entry_id = ? and status = ? and expires_at > ?", id, model.OrderStatusActive, time.Now()).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("entry is used by %d active orders", count)
	}
	return s.db.Delete(&model.DedicatedEntry{}, id).Error
}

func normalizeDedicatedEntryInput(in DedicatedEntryInput) (model.DedicatedEntry, error) {
	features, err := normalizeDedicatedFeatures(in.Features)
	if err != nil {
		return model.DedicatedEntry{}, err
	}
	domain := strings.TrimSpace(in.Domain)
	if domain == "" {
		return model.DedicatedEntry{}, errors.New("domain is required")
	}
	if in.Priority <= 0 {
		in.Priority = 100
	}
	featureSet := map[string]struct{}{}
	for _, feature := range features {
		featureSet[feature] = struct{}{}
	}
	if _, ok := featureSet[model.DedicatedFeatureMixed]; ok && (in.MixedPort <= 0 || in.MixedPort > 65535) {
		return model.DedicatedEntry{}, errors.New("mixed_port is required for mixed")
	}
	if _, ok := featureSet[model.DedicatedFeatureVmess]; ok && (in.VmessPort <= 0 || in.VmessPort > 65535) {
		return model.DedicatedEntry{}, errors.New("vmess_port is required for vmess")
	}
	if _, ok := featureSet[model.DedicatedFeatureVless]; ok && (in.VlessPort <= 0 || in.VlessPort > 65535) {
		return model.DedicatedEntry{}, errors.New("vless_port is required for vless")
	}
	if _, ok := featureSet[model.DedicatedFeatureShadowsocks]; ok && (in.ShadowsocksPort <= 0 || in.ShadowsocksPort > 65535) {
		return model.DedicatedEntry{}, errors.New("shadowsocks_port is required for shadowsocks")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return model.DedicatedEntry{
		Name:            strings.TrimSpace(in.Name),
		Domain:          domain,
		MixedPort:       in.MixedPort,
		VmessPort:       in.VmessPort,
		VlessPort:       in.VlessPort,
		ShadowsocksPort: in.ShadowsocksPort,
		Priority:        in.Priority,
		Features:        joinDedicatedFeatures(features),
		Enabled:         enabled,
		Notes:           strings.TrimSpace(in.Notes),
	}, nil
}
