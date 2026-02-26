package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

type DedicatedIngressService struct {
	db *gorm.DB
}

type DedicatedInboundInput struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	ListenPort int    `json:"listen_port"`
	Priority   int    `json:"priority"`
	Enabled    *bool  `json:"enabled"`
	Notes      string `json:"notes"`
}

type DedicatedIngressInput struct {
	DedicatedInboundID uint   `json:"dedicated_inbound_id"`
	Name               string `json:"name"`
	Domain             string `json:"domain"`
	IngressPort        int    `json:"ingress_port"`
	CountryCode        string `json:"country_code"`
	Region             string `json:"region"`
	Priority           int    `json:"priority"`
	Enabled            *bool  `json:"enabled"`
	Notes              string `json:"notes"`
}

func NewDedicatedIngressService(db *gorm.DB) *DedicatedIngressService {
	return &DedicatedIngressService{db: db}
}

func (s *DedicatedIngressService) ListInbounds() ([]model.DedicatedInbound, error) {
	rows := []model.DedicatedInbound{}
	if err := s.db.Preload("Ingresses", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("enabled desc, priority asc, id asc")
	}).Order("enabled desc, priority asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DedicatedIngressService) ListIngresses() ([]model.DedicatedIngress, error) {
	rows := []model.DedicatedIngress{}
	if err := s.db.Preload("DedicatedInbound").Order("enabled desc, priority asc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *DedicatedIngressService) CreateInbound(in DedicatedInboundInput) (*model.DedicatedInbound, error) {
	row, err := normalizeDedicatedInboundInput(in)
	if err != nil {
		return nil, err
	}
	var count int64
	if err := s.db.Model(&model.DedicatedInbound{}).Where("protocol = ? and listen_port = ?", row.Protocol, row.ListenPort).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("protocol + listen_port already exists")
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *DedicatedIngressService) UpdateInbound(id uint, in DedicatedInboundInput) (*model.DedicatedInbound, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	base := model.DedicatedInbound{}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	row, err := normalizeDedicatedInboundInput(in)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(base.Protocol), strings.TrimSpace(row.Protocol)) || base.ListenPort != row.ListenPort {
		var activeCount int64
		if err := s.db.Model(&model.Order{}).Where("dedicated_inbound_id = ? and status = ? and expires_at > ?", id, model.OrderStatusActive, time.Now()).Count(&activeCount).Error; err != nil {
			return nil, err
		}
		if activeCount > 0 {
			return nil, fmt.Errorf("inbound is used by %d active orders, cannot change protocol/listen_port", activeCount)
		}
	}
	var duplicateCount int64
	if err := s.db.Model(&model.DedicatedInbound{}).Where("protocol = ? and listen_port = ? and id <> ?", row.Protocol, row.ListenPort, id).Count(&duplicateCount).Error; err != nil {
		return nil, err
	}
	if duplicateCount > 0 {
		return nil, errors.New("protocol + listen_port already exists")
	}
	updates := map[string]interface{}{
		"name":        row.Name,
		"protocol":    row.Protocol,
		"listen_port": row.ListenPort,
		"priority":    row.Priority,
		"enabled":     row.Enabled,
		"notes":       row.Notes,
		"updated_at":  time.Now(),
	}
	if err := s.db.Model(&model.DedicatedInbound{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	return &base, nil
}

func (s *DedicatedIngressService) DeleteInbound(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	var ingressCount int64
	if err := s.db.Model(&model.DedicatedIngress{}).Where("dedicated_inbound_id = ?", id).Count(&ingressCount).Error; err != nil {
		return err
	}
	if ingressCount > 0 {
		return fmt.Errorf("inbound has %d ingresses", ingressCount)
	}
	var orderCount int64
	if err := s.db.Model(&model.Order{}).Where("dedicated_inbound_id = ? and status = ? and expires_at > ?", id, model.OrderStatusActive, time.Now()).Count(&orderCount).Error; err != nil {
		return err
	}
	if orderCount > 0 {
		return fmt.Errorf("inbound is used by %d active orders", orderCount)
	}
	return s.db.Delete(&model.DedicatedInbound{}, id).Error
}

func (s *DedicatedIngressService) ToggleInbound(id uint, enabled bool) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.db.Model(&model.DedicatedInbound{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

func (s *DedicatedIngressService) CreateIngress(in DedicatedIngressInput) (*model.DedicatedIngress, error) {
	row, err := s.normalizeDedicatedIngressInput(in)
	if err != nil {
		return nil, err
	}
	var duplicateCount int64
	if err := s.db.Model(&model.DedicatedIngress{}).
		Where("dedicated_inbound_id = ? and domain = ? and ingress_port = ?", row.DedicatedInboundID, row.Domain, row.IngressPort).
		Count(&duplicateCount).Error; err != nil {
		return nil, err
	}
	if duplicateCount > 0 {
		return nil, errors.New("ingress already exists for inbound + domain + port")
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("DedicatedInbound").First(&row, row.ID).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *DedicatedIngressService) UpdateIngress(id uint, in DedicatedIngressInput) (*model.DedicatedIngress, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	base := model.DedicatedIngress{}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	if in.DedicatedInboundID == 0 {
		in.DedicatedInboundID = base.DedicatedInboundID
	}
	row, err := s.normalizeDedicatedIngressInput(in)
	if err != nil {
		return nil, err
	}
	if base.DedicatedInboundID != row.DedicatedInboundID {
		var activeCount int64
		if err := s.db.Model(&model.Order{}).Where("dedicated_ingress_id = ? and status = ? and expires_at > ?", id, model.OrderStatusActive, time.Now()).Count(&activeCount).Error; err != nil {
			return nil, err
		}
		if activeCount > 0 {
			return nil, fmt.Errorf("ingress is used by %d active orders, cannot change bound inbound", activeCount)
		}
	}
	var duplicateCount int64
	if err := s.db.Model(&model.DedicatedIngress{}).
		Where("dedicated_inbound_id = ? and domain = ? and ingress_port = ? and id <> ?", row.DedicatedInboundID, row.Domain, row.IngressPort, id).
		Count(&duplicateCount).Error; err != nil {
		return nil, err
	}
	if duplicateCount > 0 {
		return nil, errors.New("ingress already exists for inbound + domain + port")
	}
	updates := map[string]interface{}{
		"dedicated_inbound_id": row.DedicatedInboundID,
		"name":                 row.Name,
		"domain":               row.Domain,
		"ingress_port":         row.IngressPort,
		"country_code":         row.CountryCode,
		"region":               row.Region,
		"priority":             row.Priority,
		"enabled":              row.Enabled,
		"notes":                row.Notes,
		"updated_at":           time.Now(),
	}
	if err := s.db.Model(&model.DedicatedIngress{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.Preload("DedicatedInbound").First(&base, id).Error; err != nil {
		return nil, err
	}
	return &base, nil
}

func (s *DedicatedIngressService) DeleteIngress(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	var count int64
	if err := s.db.Model(&model.Order{}).Where("dedicated_ingress_id = ? and status = ? and expires_at > ?", id, model.OrderStatusActive, time.Now()).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("ingress is used by %d active orders", count)
	}
	return s.db.Delete(&model.DedicatedIngress{}, id).Error
}

func (s *DedicatedIngressService) ToggleIngress(id uint, enabled bool) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.db.Model(&model.DedicatedIngress{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

func normalizeDedicatedInboundInput(in DedicatedInboundInput) (model.DedicatedInbound, error) {
	protocol, err := normalizeDedicatedProtocol(in.Protocol)
	if err != nil {
		return model.DedicatedInbound{}, err
	}
	if in.ListenPort <= 0 || in.ListenPort > 65535 {
		return model.DedicatedInbound{}, errors.New("listen_port invalid")
	}
	if in.Priority <= 0 {
		in.Priority = 100
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = strings.ToUpper(protocol) + "-" + fmt.Sprintf("%d", in.ListenPort)
	}
	return model.DedicatedInbound{
		Name:       name,
		Protocol:   protocol,
		ListenPort: in.ListenPort,
		Priority:   in.Priority,
		Enabled:    enabled,
		Notes:      strings.TrimSpace(in.Notes),
	}, nil
}

func (s *DedicatedIngressService) normalizeDedicatedIngressInput(in DedicatedIngressInput) (model.DedicatedIngress, error) {
	if in.DedicatedInboundID == 0 {
		return model.DedicatedIngress{}, errors.New("dedicated_inbound_id is required")
	}
	inbound := model.DedicatedInbound{}
	if err := s.db.Where("id = ?", in.DedicatedInboundID).First(&inbound).Error; err != nil {
		return model.DedicatedIngress{}, fmt.Errorf("dedicated inbound invalid: %w", err)
	}
	domain := strings.TrimSpace(in.Domain)
	if domain == "" {
		return model.DedicatedIngress{}, errors.New("domain is required")
	}
	if in.IngressPort <= 0 || in.IngressPort > 65535 {
		return model.DedicatedIngress{}, errors.New("ingress_port invalid")
	}
	if in.Priority <= 0 {
		in.Priority = 100
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = fmt.Sprintf("%s-%d", strings.ToUpper(inbound.Protocol), in.IngressPort)
	}
	return model.DedicatedIngress{
		DedicatedInboundID: in.DedicatedInboundID,
		Name:               name,
		Domain:             domain,
		IngressPort:        in.IngressPort,
		CountryCode:        strings.ToLower(strings.TrimSpace(in.CountryCode)),
		Region:             strings.TrimSpace(in.Region),
		Priority:           in.Priority,
		Enabled:            enabled,
		Notes:              strings.TrimSpace(in.Notes),
	}, nil
}
