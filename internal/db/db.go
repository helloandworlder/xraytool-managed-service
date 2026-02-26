package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		&model.DedicatedEntry{},
		&model.DedicatedInbound{},
		&model.DedicatedIngress{},
		&model.Order{},
		&model.OrderItem{},
		&model.DedicatedEgress{},
		&model.XrayResource{},
		&model.Setting{},
		&model.TaskLog{},
	); err != nil {
		return nil, err
	}
	if err := migrateOrderItemUsernameIndex(database); err != nil {
		return nil, err
	}
	if err := migrateOrderNoIndex(database); err != nil {
		return nil, err
	}
	if err := migrateOrderNo(database); err != nil {
		return nil, err
	}
	if err := migrateDedicatedEntryToInboundIngress(database); err != nil {
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

func migrateOrderNo(database *gorm.DB) error {
	type orderNoRow struct {
		ID        uint
		OrderNo   string
		CreatedAt time.Time
	}
	rows := []orderNoRow{}
	if err := database.Model(&model.Order{}).Select("id", "order_no", "created_at").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.OrderNo) != "" {
			continue
		}
		ts := row.CreatedAt
		if ts.IsZero() {
			ts = time.Now()
		}
		orderNo := fmt.Sprintf("OD%s%06d", ts.Format("060102"), row.ID)
		if err := database.Model(&model.Order{}).Where("id = ?", row.ID).Update("order_no", orderNo).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateOrderNoIndex(database *gorm.DB) error {
	if err := database.Exec("DROP INDEX IF EXISTS idx_orders_order_no").Error; err != nil {
		return err
	}
	return database.Exec("CREATE INDEX IF NOT EXISTS idx_orders_order_no ON orders(order_no)").Error
}

func migrateDedicatedEntryToInboundIngress(database *gorm.DB) error {
	return database.Transaction(func(tx *gorm.DB) error {
		entries := []model.DedicatedEntry{}
		if err := tx.Find(&entries).Error; err != nil {
			return err
		}
		inboundByKey := map[string]model.DedicatedInbound{}
		ingressByKey := map[string]model.DedicatedIngress{}

		ensureInbound := func(name string, protocol string, listenPort int, priority int, enabled bool, notes string) (model.DedicatedInbound, error) {
			key := fmt.Sprintf("%s:%d", protocol, listenPort)
			if row, ok := inboundByKey[key]; ok {
				return row, nil
			}
			row := model.DedicatedInbound{}
			err := tx.Where("protocol = ? and listen_port = ?", protocol, listenPort).First(&row).Error
			if err == nil {
				inboundByKey[key] = row
				return row, nil
			}
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return model.DedicatedInbound{}, err
			}
			row = model.DedicatedInbound{
				Name:       name,
				Protocol:   protocol,
				ListenPort: listenPort,
				Priority:   priority,
				Enabled:    enabled,
				Notes:      notes,
			}
			if err := tx.Create(&row).Error; err != nil {
				return model.DedicatedInbound{}, err
			}
			inboundByKey[key] = row
			return row, nil
		}

		ensureIngress := func(inboundID uint, name string, domain string, ingressPort int, priority int, enabled bool, notes string) (model.DedicatedIngress, error) {
			key := fmt.Sprintf("%d:%s:%d", inboundID, domain, ingressPort)
			if row, ok := ingressByKey[key]; ok {
				return row, nil
			}
			row := model.DedicatedIngress{}
			err := tx.Where("dedicated_inbound_id = ? and domain = ? and ingress_port = ?", inboundID, domain, ingressPort).First(&row).Error
			if err == nil {
				ingressByKey[key] = row
				return row, nil
			}
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return model.DedicatedIngress{}, err
			}
			row = model.DedicatedIngress{
				DedicatedInboundID: inboundID,
				Name:               name,
				Domain:             domain,
				IngressPort:        ingressPort,
				Priority:           priority,
				Enabled:            enabled,
				Notes:              notes,
			}
			if err := tx.Create(&row).Error; err != nil {
				return model.DedicatedIngress{}, err
			}
			ingressByKey[key] = row
			return row, nil
		}

		for _, entry := range entries {
			features := map[string]bool{}
			for _, part := range []string{"mixed", "vmess", "vless", "shadowsocks"} {
				features[part] = false
			}
			for _, part := range splitFeatureList(entry.Features) {
				features[part] = true
			}
			if features["mixed"] && entry.MixedPort > 0 {
				inbound, err := ensureInbound(chooseName(entry.Name, "Mixed"), "mixed", entry.MixedPort, entry.Priority, entry.Enabled, entry.Notes)
				if err != nil {
					return err
				}
				if _, err := ensureIngress(inbound.ID, chooseName(entry.Name, "Mixed入口"), entry.Domain, entry.MixedPort, entry.Priority, entry.Enabled, entry.Notes); err != nil {
					return err
				}
			}
			if features["vmess"] && entry.VmessPort > 0 {
				inbound, err := ensureInbound(chooseName(entry.Name, "Vmess"), "vmess", entry.VmessPort, entry.Priority, entry.Enabled, entry.Notes)
				if err != nil {
					return err
				}
				if _, err := ensureIngress(inbound.ID, chooseName(entry.Name, "Vmess入口"), entry.Domain, entry.VmessPort, entry.Priority, entry.Enabled, entry.Notes); err != nil {
					return err
				}
			}
			if features["vless"] && entry.VlessPort > 0 {
				inbound, err := ensureInbound(chooseName(entry.Name, "Vless"), "vless", entry.VlessPort, entry.Priority, entry.Enabled, entry.Notes)
				if err != nil {
					return err
				}
				if _, err := ensureIngress(inbound.ID, chooseName(entry.Name, "Vless入口"), entry.Domain, entry.VlessPort, entry.Priority, entry.Enabled, entry.Notes); err != nil {
					return err
				}
			}
			if features["shadowsocks"] && entry.ShadowsocksPort > 0 {
				inbound, err := ensureInbound(chooseName(entry.Name, "Shadowsocks"), "shadowsocks", entry.ShadowsocksPort, entry.Priority, entry.Enabled, entry.Notes)
				if err != nil {
					return err
				}
				if _, err := ensureIngress(inbound.ID, chooseName(entry.Name, "Shadowsocks入口"), entry.Domain, entry.ShadowsocksPort, entry.Priority, entry.Enabled, entry.Notes); err != nil {
					return err
				}
			}
		}

		type orderRow struct {
			ID                 uint
			DedicatedEntryID   *uint
			DedicatedProtocol  string
			DedicatedInboundID *uint
			DedicatedIngressID *uint
		}
		orders := []orderRow{}
		if err := tx.Model(&model.Order{}).Where("mode = ?", model.OrderModeDedicated).Find(&orders).Error; err != nil {
			return err
		}
		for _, order := range orders {
			if order.DedicatedInboundID != nil && *order.DedicatedInboundID > 0 && order.DedicatedIngressID != nil && *order.DedicatedIngressID > 0 {
				continue
			}
			if order.DedicatedEntryID == nil || *order.DedicatedEntryID == 0 {
				continue
			}
			entry := model.DedicatedEntry{}
			if err := tx.First(&entry, *order.DedicatedEntryID).Error; err != nil {
				continue
			}
			protocol := normalizeLegacyProtocol(order.DedicatedProtocol)
			port := choosePortByProtocol(entry, protocol)
			if port <= 0 {
				protocol = "mixed"
				port = choosePortByProtocol(entry, protocol)
			}
			if port <= 0 {
				continue
			}
			inbound := model.DedicatedInbound{}
			if err := tx.Where("protocol = ? and listen_port = ?", protocol, port).First(&inbound).Error; err != nil {
				continue
			}
			ingress := model.DedicatedIngress{}
			if err := tx.Where("dedicated_inbound_id = ? and domain = ? and ingress_port = ?", inbound.ID, entry.Domain, port).First(&ingress).Error; err != nil {
				continue
			}
			_ = tx.Model(&model.Order{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
				"dedicated_protocol":   protocol,
				"dedicated_inbound_id": inbound.ID,
				"dedicated_ingress_id": ingress.ID,
				"port":                 inbound.ListenPort,
			})
		}

		return nil
	})
}

func splitFeatureList(raw string) []string {
	parts := []string{}
	for _, part := range []string{"mixed", "vmess", "vless", "shadowsocks"} {
		_ = part
	}
	for _, item := range splitComma(raw) {
		v := item
		if v == "socks5" {
			v = "mixed"
		}
		if v == "mixed" || v == "vmess" || v == "vless" || v == "shadowsocks" {
			parts = append(parts, v)
		}
	}
	return parts
}

func splitComma(raw string) []string {
	out := []string{}
	for _, p := range strings.Split(raw, ",") {
		v := strings.ToLower(strings.TrimSpace(p))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func chooseName(base string, fallback string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return fallback
	}
	return base + "-" + fallback
}

func normalizeLegacyProtocol(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "socks5" {
		v = "mixed"
	}
	if v == "mixed" || v == "vmess" || v == "vless" || v == "shadowsocks" {
		return v
	}
	return "mixed"
}

func choosePortByProtocol(entry model.DedicatedEntry, protocol string) int {
	if protocol == "mixed" {
		return entry.MixedPort
	}
	if protocol == "vmess" {
		return entry.VmessPort
	}
	if protocol == "vless" {
		return entry.VlessPort
	}
	if protocol == "shadowsocks" {
		return entry.ShadowsocksPort
	}
	return 0
}
