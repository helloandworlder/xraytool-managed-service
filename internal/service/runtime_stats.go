package service

import (
	"context"
	"runtime"
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

type ioSample struct {
	At       time.Time
	Uplink   int64
	Downlink int64
}

type cpuSample struct {
	At      time.Time
	CPUTime time.Duration
}

type userTraffic struct {
	Uplink   int64
	Downlink int64
}

type runtimeCustomerMeta struct {
	Name string
	Code string
}

type runtimeDataset struct {
	now           time.Time
	customerUsers map[uint]map[string]struct{}
	customerMetas map[uint]runtimeCustomerMeta
	routeUsers    map[string]map[string]struct{}
	trafficByUser map[string]userTraffic
	onlineByUser  map[string]int64
}

type NodeTelemetryRouteStat struct {
	Key               string    `json:"key"`
	Label             string    `json:"label"`
	ActiveConnections int64     `json:"active_connections"`
	ActiveUsers       int64     `json:"active_users"`
	UploadBPS         float64   `json:"upload_bps"`
	DownloadBPS       float64   `json:"download_bps"`
	Traffic24H        int64     `json:"traffic_24h"`
	TrafficTotal      int64     `json:"traffic_total"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type NodeTelemetryStat struct {
	ActiveConnections int64                    `json:"active_connections"`
	ActiveUsers       int64                    `json:"active_users"`
	UploadBPS         float64                  `json:"upload_bps"`
	DownloadBPS       float64                  `json:"download_bps"`
	Traffic24H        int64                    `json:"traffic_24h"`
	TrafficTotal      int64                    `json:"traffic_total"`
	CPUPercent        *float64                 `json:"cpu_percent,omitempty"`
	MemoryBytes       uint64                   `json:"memory_bytes"`
	Routes            []NodeTelemetryRouteStat `json:"routes"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

type RuntimeStatsService struct {
	db   *gorm.DB
	xray *XrayManager

	mu           sync.Mutex
	last         map[uint]trafficSample
	histories    map[uint][]trafficSample
	totalLast    ioSample
	totalHistory []trafficSample
	routeLast    map[string]ioSample
	routeHistory map[string][]trafficSample
	cpuLast      cpuSample
}

func NewRuntimeStatsService(db *gorm.DB, xray *XrayManager) *RuntimeStatsService {
	return &RuntimeStatsService{
		db:           db,
		xray:         xray,
		last:         map[uint]trafficSample{},
		histories:    map[uint][]trafficSample{},
		routeLast:    map[string]ioSample{},
		routeHistory: map[string][]trafficSample{},
	}
}

func (s *RuntimeStatsService) Snapshot(ctx context.Context) ([]CustomerRuntimeStat, error) {
	data, err := s.collectRuntimeDataset(ctx)
	if err != nil {
		return nil, err
	}
	stats := make([]CustomerRuntimeStat, 0, len(data.customerUsers))

	s.mu.Lock()
	defer s.mu.Unlock()

	for customerID, users := range data.customerUsers {
		total := int64(0)
		online := int64(0)
		for username := range users {
			total += data.trafficByUser[username].Uplink + data.trafficByUser[username].Downlink
			online += data.onlineByUser[username]
		}

		realtime := 0.0
		if prev, ok := s.last[customerID]; ok && data.now.After(prev.At) && total >= prev.Total {
			realtime = float64(total-prev.Total) / data.now.Sub(prev.At).Seconds()
		}
		s.last[customerID] = trafficSample{At: data.now, Total: total}

		history := append(s.histories[customerID], trafficSample{At: data.now, Total: total})
		history = trimHistory(history, data.now.Add(-7*24*time.Hour))
		s.histories[customerID] = history

		meta := data.customerMetas[customerID]
		stats = append(stats, CustomerRuntimeStat{
			CustomerID:    customerID,
			CustomerName:  meta.Name,
			CustomerCode:  meta.Code,
			OnlineClients: online,
			RealtimeBPS:   realtime,
			Traffic1H:     trafficSince(history, total, data.now.Add(-1*time.Hour)),
			Traffic24H:    trafficSince(history, total, data.now.Add(-24*time.Hour)),
			Traffic7D:     trafficSince(history, total, data.now.Add(-7*24*time.Hour)),
			UpdatedAt:     data.now,
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

func (s *RuntimeStatsService) TelemetrySnapshot(ctx context.Context) (NodeTelemetryStat, error) {
	data, err := s.collectRuntimeDataset(ctx)
	if err != nil {
		return NodeTelemetryStat{}, err
	}

	totalUp := int64(0)
	totalDown := int64(0)
	activeConnections := int64(0)
	activeUsers := int64(0)
	allUsers := map[string]struct{}{}
	for _, users := range data.customerUsers {
		for username := range users {
			if _, ok := allUsers[username]; ok {
				continue
			}
			allUsers[username] = struct{}{}
			totalUp += data.trafficByUser[username].Uplink
			totalDown += data.trafficByUser[username].Downlink
			activeConnections += data.onlineByUser[username]
			if data.onlineByUser[username] > 0 {
				activeUsers++
			}
		}
	}

	routes := make([]NodeTelemetryRouteStat, 0, len(data.routeUsers))
	s.mu.Lock()
	defer s.mu.Unlock()

	totalUploadBPS := ioRate(data.now, s.totalLast.At, totalUp, s.totalLast.Uplink)
	totalDownloadBPS := ioRate(data.now, s.totalLast.At, totalDown, s.totalLast.Downlink)
	s.totalLast = ioSample{At: data.now, Uplink: totalUp, Downlink: totalDown}
	totalCombined := totalUp + totalDown
	s.totalHistory = append(s.totalHistory, trafficSample{At: data.now, Total: totalCombined})
	s.totalHistory = trimHistory(s.totalHistory, data.now.Add(-7*24*time.Hour))

	routeKeys := make([]string, 0, len(data.routeUsers))
	for key := range data.routeUsers {
		routeKeys = append(routeKeys, key)
	}
	sort.Strings(routeKeys)
	for _, key := range routeKeys {
		users := data.routeUsers[key]
		routeUp := int64(0)
		routeDown := int64(0)
		routeConnections := int64(0)
		routeActiveUsers := int64(0)
		for username := range users {
			routeUp += data.trafficByUser[username].Uplink
			routeDown += data.trafficByUser[username].Downlink
			routeConnections += data.onlineByUser[username]
			if data.onlineByUser[username] > 0 {
				routeActiveUsers++
			}
		}
		uploadBPS := ioRate(data.now, s.routeLast[key].At, routeUp, s.routeLast[key].Uplink)
		downloadBPS := ioRate(data.now, s.routeLast[key].At, routeDown, s.routeLast[key].Downlink)
		s.routeLast[key] = ioSample{At: data.now, Uplink: routeUp, Downlink: routeDown}

		combined := routeUp + routeDown
		history := append(s.routeHistory[key], trafficSample{At: data.now, Total: combined})
		history = trimHistory(history, data.now.Add(-7*24*time.Hour))
		s.routeHistory[key] = history

		routes = append(routes, NodeTelemetryRouteStat{
			Key:               key,
			Label:             telemetryRouteLabel(key),
			ActiveConnections: routeConnections,
			ActiveUsers:       routeActiveUsers,
			UploadBPS:         uploadBPS,
			DownloadBPS:       downloadBPS,
			Traffic24H:        trafficSince(history, combined, data.now.Add(-24*time.Hour)),
			TrafficTotal:      combined,
			UpdatedAt:         data.now,
		})
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var cpuPercent *float64
	if cpuTime, ok := currentProcessCPUTime(); ok {
		if percent, hasPercent := cpuRate(data.now, s.cpuLast, cpuTime); hasPercent {
			cpuPercent = &percent
		}
		s.cpuLast = cpuSample{At: data.now, CPUTime: cpuTime}
	}

	return NodeTelemetryStat{
		ActiveConnections: activeConnections,
		ActiveUsers:       activeUsers,
		UploadBPS:         totalUploadBPS,
		DownloadBPS:       totalDownloadBPS,
		Traffic24H:        trafficSince(s.totalHistory, totalCombined, data.now.Add(-24*time.Hour)),
		TrafficTotal:      totalCombined,
		CPUPercent:        cpuPercent,
		MemoryBytes:       mem.Alloc,
		Routes:            routes,
		UpdatedAt:         data.now,
	}, nil
}

func (s *RuntimeStatsService) collectRuntimeDataset(ctx context.Context) (runtimeDataset, error) {
	type row struct {
		CustomerID   uint
		CustomerName string
		CustomerCode string
		Username     string
		Mode         string
	}
	rows := []row{}
	err := s.db.Table("order_items oi").
		Select("o.customer_id as customer_id, c.name as customer_name, c.code as customer_code, oi.username as username, o.mode as mode").
		Joins("join orders o on o.id = oi.order_id").
		Joins("join customers c on c.id = o.customer_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Where("oi.username <> ''").
		Find(&rows).Error
	if err != nil {
		return runtimeDataset{}, err
	}

	data := runtimeDataset{
		now:           time.Now(),
		customerUsers: map[uint]map[string]struct{}{},
		customerMetas: map[uint]runtimeCustomerMeta{},
		routeUsers:    map[string]map[string]struct{}{},
		trafficByUser: map[string]userTraffic{},
		onlineByUser:  map[string]int64{},
	}
	for _, r := range rows {
		if _, ok := data.customerUsers[r.CustomerID]; !ok {
			data.customerUsers[r.CustomerID] = map[string]struct{}{}
		}
		data.customerUsers[r.CustomerID][r.Username] = struct{}{}
		data.customerMetas[r.CustomerID] = runtimeCustomerMeta{Name: r.CustomerName, Code: r.CustomerCode}

		routeKey := telemetryRouteKeyForMode(r.Mode)
		if _, ok := data.routeUsers[routeKey]; !ok {
			data.routeUsers[routeKey] = map[string]struct{}{}
		}
		data.routeUsers[routeKey][r.Username] = struct{}{}
	}

	trafficRaw, err := s.xray.QueryUserTraffic(ctx)
	if err != nil {
		return runtimeDataset{}, err
	}
	for key, val := range trafficRaw {
		user, kind, ok := parseUserTrafficKey(key)
		if !ok {
			continue
		}
		stat := data.trafficByUser[user]
		switch kind {
		case "uplink":
			stat.Uplink += val
		case "downlink":
			stat.Downlink += val
		}
		data.trafficByUser[user] = stat
	}

	onlineRaw, err := s.xray.GetAllOnlineUsers(ctx)
	if err != nil {
		return runtimeDataset{}, err
	}
	for _, onlineStatName := range onlineRaw {
		user, ok := parseUserOnlineKey(onlineStatName)
		if !ok {
			continue
		}
		count, err := s.xray.GetOnlineCount(ctx, onlineStatName)
		if err != nil {
			continue
		}
		data.onlineByUser[user] += count
	}
	return data, nil
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

func ioRate(now, prevAt time.Time, current, prev int64) float64 {
	if prevAt.IsZero() || !now.After(prevAt) || current < prev {
		return 0
	}
	seconds := now.Sub(prevAt).Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(current-prev) / seconds
}

func telemetryRouteKeyForMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case model.OrderModeDedicated:
		return "dedicated"
	default:
		return "home"
	}
}

func telemetryRouteLabel(key string) string {
	switch key {
	case "dedicated":
		return "专线"
	case "home":
		return "家宽"
	default:
		return key
	}
}

func cpuRate(now time.Time, prev cpuSample, current time.Duration) (float64, bool) {
	if prev.At.IsZero() || !now.After(prev.At) || current < prev.CPUTime {
		return 0, false
	}
	wall := now.Sub(prev.At)
	if wall <= 0 {
		return 0, false
	}
	cpuCount := runtime.NumCPU()
	if cpuCount <= 0 {
		cpuCount = 1
	}
	percent := float64(current-prev.CPUTime) / float64(wall) * 100 / float64(cpuCount)
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return percent, true
}
