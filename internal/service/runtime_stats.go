package service

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
)

type CustomerRuntimeStat struct {
	CustomerID    uint      `json:"customer_id"`
	CustomerName  string    `json:"customer_name"`
	CustomerCode  string    `json:"customer_code"`
	OnlineClients int64     `json:"online_clients"`
	RealtimeBPS   float64   `json:"realtime_bps"`
	Traffic1H     int64     `json:"traffic_1h"`
	Traffic24H    int64     `json:"traffic_24h"`
	Traffic7D     int64     `json:"traffic_7d"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type trafficSample struct {
	At    time.Time
	Total int64
}

type RuntimeStatsService struct {
	db   *gorm.DB
	xray *XrayManager

	mu        sync.Mutex
	last      map[uint]trafficSample
	histories map[uint][]trafficSample
}

func NewRuntimeStatsService(db *gorm.DB, xray *XrayManager) *RuntimeStatsService {
	return &RuntimeStatsService{
		db:        db,
		xray:      xray,
		last:      map[uint]trafficSample{},
		histories: map[uint][]trafficSample{},
	}
}

func (s *RuntimeStatsService) Snapshot(ctx context.Context) ([]CustomerRuntimeStat, error) {
	type row struct {
		CustomerID   uint
		CustomerName string
		CustomerCode string
		Username     string
	}
	rows := []row{}
	err := s.db.Table("order_items oi").
		Select("o.customer_id as customer_id, c.name as customer_name, c.code as customer_code, oi.username as username").
		Joins("join orders o on o.id = oi.order_id").
		Joins("join customers c on c.id = o.customer_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Where("oi.username <> ''").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	type customerMeta struct {
		Name string
		Code string
	}
	customerUsers := map[uint]map[string]struct{}{}
	metas := map[uint]customerMeta{}
	for _, r := range rows {
		if _, ok := customerUsers[r.CustomerID]; !ok {
			customerUsers[r.CustomerID] = map[string]struct{}{}
		}
		customerUsers[r.CustomerID][r.Username] = struct{}{}
		metas[r.CustomerID] = customerMeta{Name: r.CustomerName, Code: r.CustomerCode}
	}

	trafficRaw, err := s.xray.QueryUserTraffic(ctx)
	if err != nil {
		return nil, err
	}
	onlineRaw, err := s.xray.GetAllOnlineUsers(ctx)
	if err != nil {
		return nil, err
	}

	trafficByUser := map[string]int64{}
	for key, val := range trafficRaw {
		user, kind, ok := parseUserTrafficKey(key)
		if !ok {
			continue
		}
		if kind == "uplink" || kind == "downlink" {
			trafficByUser[user] += val
		}
	}

	onlineByUser := map[string]int64{}
	for _, onlineStatName := range onlineRaw {
		user, ok := parseUserOnlineKey(onlineStatName)
		if !ok {
			continue
		}
		count, err := s.xray.GetOnlineCount(ctx, onlineStatName)
		if err != nil {
			continue
		}
		onlineByUser[user] += count
	}

	now := time.Now()
	stats := make([]CustomerRuntimeStat, 0, len(customerUsers))

	s.mu.Lock()
	defer s.mu.Unlock()

	for customerID, users := range customerUsers {
		total := int64(0)
		online := int64(0)
		for username := range users {
			total += trafficByUser[username]
			online += onlineByUser[username]
		}

		realtime := 0.0
		if prev, ok := s.last[customerID]; ok && now.After(prev.At) && total >= prev.Total {
			realtime = float64(total-prev.Total) / now.Sub(prev.At).Seconds()
		}
		s.last[customerID] = trafficSample{At: now, Total: total}

		history := append(s.histories[customerID], trafficSample{At: now, Total: total})
		history = trimHistory(history, now.Add(-7*24*time.Hour))
		s.histories[customerID] = history

		meta := metas[customerID]
		stats = append(stats, CustomerRuntimeStat{
			CustomerID:    customerID,
			CustomerName:  meta.Name,
			CustomerCode:  meta.Code,
			OnlineClients: online,
			RealtimeBPS:   realtime,
			Traffic1H:     trafficSince(history, total, now.Add(-1*time.Hour)),
			Traffic24H:    trafficSince(history, total, now.Add(-24*time.Hour)),
			Traffic7D:     trafficSince(history, total, now.Add(-7*24*time.Hour)),
			UpdatedAt:     now,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].OnlineClients == stats[j].OnlineClients {
			return stats[i].CustomerID < stats[j].CustomerID
		}
		return stats[i].OnlineClients > stats[j].OnlineClients
	})
	return stats, nil
}

func parseUserTrafficKey(key string) (string, string, bool) {
	parts := strings.Split(key, ">>>")
	if len(parts) != 4 {
		return "", "", false
	}
	if parts[0] != "user" || parts[2] != "traffic" {
		return "", "", false
	}
	if parts[1] == "" {
		return "", "", false
	}
	return parts[1], parts[3], true
}

func parseUserOnlineKey(key string) (string, bool) {
	parts := strings.Split(key, ">>>")
	if len(parts) != 3 {
		return "", false
	}
	if parts[0] != "user" || parts[2] != "online" || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func trimHistory(in []trafficSample, threshold time.Time) []trafficSample {
	if len(in) == 0 {
		return in
	}
	idx := 0
	for idx < len(in)-1 && in[idx].At.Before(threshold) {
		idx++
	}
	return append([]trafficSample(nil), in[idx:]...)
}

func trafficSince(history []trafficSample, current int64, threshold time.Time) int64 {
	if len(history) == 0 {
		return 0
	}
	base := history[0].Total
	for _, sample := range history {
		if !sample.At.Before(threshold) {
			base = sample.Total
			break
		}
		base = sample.Total
	}
	if current < base {
		return 0
	}
	return current - base
}
