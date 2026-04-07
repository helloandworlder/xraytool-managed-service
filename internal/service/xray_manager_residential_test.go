package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"xraytool/internal/config"
	"xraytool/internal/model"

	"go.uber.org/zap"
)

func TestRebuildConfigFileSplitsManagedMixedInboundsByListenIP(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	now := time.Now()

	customer := model.Customer{Name: "cfg-home", Code: "cfg-home", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	order := model.Order{
		CustomerID: customer.ID,
		Name:       "cfg-home-order",
		Mode:       model.OrderModeAuto,
		Status:     model.OrderStatusActive,
		Quantity:   2,
		Port:       residentialTestPort,
		StartsAt:   now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	items := []model.OrderItem{
		{
			OrderID:      order.ID,
			IP:           "203.0.113.51",
			Port:         residentialTestPort,
			Username:     "dup-user",
			Password:     "pass-a",
			OutboundType: model.OutboundTypeDirect,
			Managed:      true,
			Status:       model.OrderItemStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			OrderID:      order.ID,
			IP:           "203.0.113.52",
			Port:         residentialTestPort,
			Username:     "dup-user",
			Password:     "pass-b",
			OutboundType: model.OutboundTypeDirect,
			Managed:      true,
			Status:       model.OrderItemStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	for _, item := range items {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create item failed: %v", err)
		}
	}

	cfgPath := filepath.Join(t.TempDir(), "managed-xray.json")
	mgr := NewXrayManager(config.Config{
		XrayConfigPath: cfgPath,
		XrayAPIServer:  "127.0.0.1:10085",
	}, db, zap.NewNop())
	if err := mgr.RebuildConfigFile(context.Background()); err != nil {
		t.Fatalf("rebuild config failed: %v", err)
	}

	body, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	var payload struct {
		Inbounds []struct {
			Tag      string `json:"tag"`
			Listen   string `json:"listen"`
			Port     int    `json:"port"`
			Protocol string `json:"protocol"`
			Settings struct {
				Accounts []struct {
					User string `json:"user"`
					Pass string `json:"pass"`
				} `json:"accounts"`
			} `json:"settings"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal config failed: %v", err)
	}

	listens := map[string]string{}
	for _, inbound := range payload.Inbounds {
		if inbound.Protocol != "mixed" || inbound.Port != residentialTestPort {
			continue
		}
		if len(inbound.Settings.Accounts) != 1 {
			t.Fatalf("expected one account per mixed inbound, got %d", len(inbound.Settings.Accounts))
		}
		listens[inbound.Listen] = inbound.Settings.Accounts[0].Pass
		if inbound.Settings.Accounts[0].User != "dup-user" {
			t.Fatalf("expected duplicated username on per-ip inbound, got %s", inbound.Settings.Accounts[0].User)
		}
	}
	if len(listens) != 2 {
		t.Fatalf("expected 2 mixed inbounds for residential port, got %d", len(listens))
	}
	if listens["203.0.113.51"] != "pass-a" || listens["203.0.113.52"] != "pass-b" {
		t.Fatalf("unexpected per-ip inbound accounts: %#v", listens)
	}
}

func TestRebuildConfigFileRejectsDuplicateDedicatedMixedUsernameOnSharedPort(t *testing.T) {
	db := setupOrderServiceTestDB(t)
	now := time.Now()

	customer := model.Customer{Name: "cfg-dedicated", Code: "cfg-dedicated", Status: model.OrderStatusActive}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer failed: %v", err)
	}
	inbound := model.DedicatedInbound{Name: "mixed-in", Protocol: model.DedicatedFeatureMixed, ListenPort: 9878, Enabled: true}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatalf("create inbound failed: %v", err)
	}
	head := model.Order{
		CustomerID:         customer.ID,
		Name:               "dedicated-head",
		Mode:               model.OrderModeDedicated,
		DedicatedProtocol:  model.DedicatedFeatureMixed,
		Status:             model.OrderStatusActive,
		Quantity:           2,
		Port:               9878,
		IsGroupHead:        true,
		DedicatedInboundID: uintPtrOrNil(inbound.ID),
		StartsAt:           now,
		ExpiresAt:          now.Add(24 * time.Hour),
	}
	if err := db.Create(&head).Error; err != nil {
		t.Fatalf("create head failed: %v", err)
	}
	parentID := head.ID
	children := []model.Order{
		{
			CustomerID:         customer.ID,
			GroupID:            head.ID,
			ParentOrderID:      &parentID,
			SequenceNo:         1,
			DedicatedInboundID: uintPtrOrNil(inbound.ID),
			DedicatedProtocol:  model.DedicatedFeatureMixed,
			Name:               "child-1",
			Mode:               model.OrderModeDedicated,
			Status:             model.OrderStatusActive,
			Quantity:           1,
			Port:               9878,
			StartsAt:           now,
			ExpiresAt:          now.Add(24 * time.Hour),
		},
		{
			CustomerID:         customer.ID,
			GroupID:            head.ID,
			ParentOrderID:      &parentID,
			SequenceNo:         2,
			DedicatedInboundID: uintPtrOrNil(inbound.ID),
			DedicatedProtocol:  model.DedicatedFeatureMixed,
			Name:               "child-2",
			Mode:               model.OrderModeDedicated,
			Status:             model.OrderStatusActive,
			Quantity:           1,
			Port:               9878,
			StartsAt:           now,
			ExpiresAt:          now.Add(24 * time.Hour),
		},
	}
	for i := range children {
		if err := db.Create(&children[i]).Error; err != nil {
			t.Fatalf("create child failed: %v", err)
		}
	}
	items := []model.OrderItem{
		{
			OrderID:         children[0].ID,
			IP:              "127.0.0.1",
			Port:            9878,
			Username:        "daoge888",
			Password:        "daoge888",
			OutboundType:    model.OutboundTypeSocks5,
			ForwardAddress:  "207.21.125.48",
			ForwardPort:     9878,
			ForwardUsername: "egress-a",
			ForwardPassword: "egress-a",
			Managed:         true,
			Status:          model.OrderItemStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			OrderID:         children[1].ID,
			IP:              "127.0.0.1",
			Port:            9878,
			Username:        "daoge888",
			Password:        "daoge888",
			OutboundType:    model.OutboundTypeSocks5,
			ForwardAddress:  "207.21.125.190",
			ForwardPort:     9878,
			ForwardUsername: "egress-b",
			ForwardPassword: "egress-b",
			Managed:         true,
			Status:          model.OrderItemStatusActive,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
	for _, item := range items {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create item failed: %v", err)
		}
	}

	cfgPath := filepath.Join(t.TempDir(), "managed-xray.json")
	mgr := NewXrayManager(config.Config{
		XrayConfigPath: cfgPath,
		XrayAPIServer:  "127.0.0.1:10085",
	}, db, zap.NewNop())
	err := mgr.RebuildConfigFile(context.Background())
	if err == nil {
		t.Fatalf("expected duplicate dedicated mixed username to be rejected")
	}
	if err != nil && err.Error() != "duplicate dedicated mixed username daoge888 found on shared port 9878" {
		t.Fatalf("unexpected error: %v", err)
	}
}
