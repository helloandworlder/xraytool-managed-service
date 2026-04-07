package service

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	runtimeScopeCustomer = "customer"
	runtimeScopeGroup    = "group"
	runtimeScopeOrder    = "order"
	runtimeScopeRoute    = "route"
	runtimeScopeTotal    = "total"
)

type CustomerRuntimeStat struct {
	CustomerID             uint      `json:"customer_id"`
	CustomerName           string    `json:"customer_name"`
	CustomerCode           string    `json:"customer_code"`
	HomeItems              int       `json:"home_items"`
	DedicatedItems         int       `json:"dedicated_items"`
	HomeOnlineClients      int64     `json:"home_online_clients"`
	DedicatedOnlineClients int64     `json:"dedicated_online_clients"`
	OnlineClients          int64     `json:"online_clients"`
	RealtimeBPS            float64   `json:"realtime_bps"`
	Traffic1H              int64     `json:"traffic_1h"`
	Traffic24H             int64     `json:"traffic_24h"`
	Traffic7D              int64     `json:"traffic_7d"`
	TrafficTotal           int64     `json:"traffic_total"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type OrderGroupRuntimeStat struct {
	GroupID       uint      `json:"group_id"`
	GroupOrderNo  string    `json:"group_order_no"`
	GroupName     string    `json:"group_name"`
	CustomerID    uint      `json:"customer_id"`
	CustomerName  string    `json:"customer_name"`
	CustomerCode  string    `json:"customer_code"`
	Mode          string    `json:"mode"`
	OrderCount    int       `json:"order_count"`
	ActiveItems   int       `json:"active_items"`
	OnlineClients int64     `json:"online_clients"`
	RealtimeBPS   float64   `json:"realtime_bps"`
	Traffic1H     int64     `json:"traffic_1h"`
	Traffic24H    int64     `json:"traffic_24h"`
	Traffic7D     int64     `json:"traffic_7d"`
	TrafficTotal  int64     `json:"traffic_total"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type OrderRuntimeStat struct {
	OrderID       uint      `json:"order_id"`
	OrderNo       string    `json:"order_no"`
	OrderName     string    `json:"order_name"`
	GroupID       uint      `json:"group_id"`
	GroupOrderNo  string    `json:"group_order_no"`
	CustomerID    uint      `json:"customer_id"`
	CustomerName  string    `json:"customer_name"`
	CustomerCode  string    `json:"customer_code"`
	Mode          string    `json:"mode"`
	Quantity      int       `json:"quantity"`
	ActiveItems   int       `json:"active_items"`
	OnlineClients int64     `json:"online_clients"`
	RealtimeBPS   float64   `json:"realtime_bps"`
	Traffic1H     int64     `json:"traffic_1h"`
	Traffic24H    int64     `json:"traffic_24h"`
	Traffic7D     int64     `json:"traffic_7d"`
	TrafficTotal  int64     `json:"traffic_total"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RuntimeOverviewStat struct {
	Customers []CustomerRuntimeStat   `json:"customers"`
	Groups    []OrderGroupRuntimeStat `json:"groups"`
	Orders    []OrderRuntimeStat      `json:"orders"`
	Warnings  []string                `json:"warnings,omitempty"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type trafficSample struct {
	At       time.Time
	Uplink   int64
	Downlink int64
	Total    int64
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

type runtimeMeasure struct {
	Uplink   int64
	Downlink int64
	Online   int64
}

func (m runtimeMeasure) Total() int64 {
	return m.Uplink + m.Downlink
}

type runtimeCustomerMeta struct {
	Name string
	Code string
}

type runtimeGroupMeta struct {
	GroupOrderNo string
	GroupName    string
	CustomerID   uint
	CustomerName string
	CustomerCode string
	Mode         string
	OrderCount   int
	ActiveItems  int
}

type runtimeOrderMeta struct {
	OrderNo      string
	OrderName    string
	GroupID      uint
	GroupOrderNo string
	CustomerID   uint
	CustomerName string
	CustomerCode string
	Mode         string
	Quantity     int
	ActiveItems  int
}

type runtimeDataset struct {
	now                time.Time
	customerUsers      map[uint]map[string]struct{}
	customerMetas      map[uint]runtimeCustomerMeta
	customerRouteUsers map[uint]map[string]map[string]struct{}
	customerRouteItems map[uint]map[string]int
	groupUsers         map[uint]map[string]struct{}
	groupMetas         map[uint]runtimeGroupMeta
	orderUsers         map[uint]map[string]struct{}
	orderMetas         map[uint]runtimeOrderMeta
	routeUsers         map[string]map[string]struct{}
	trafficByUser      map[string]userTraffic
	onlineByUser       map[string]int64
	conflictedUsers    map[string]struct{}
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

type runtimeCapture struct {
	overview          RuntimeOverviewStat
	routeStats        []NodeTelemetryRouteStat
	totalUploadBPS    float64
	totalDownloadBPS  float64
	totalTraffic24H   int64
	totalMeasure      runtimeMeasure
	activeUsers       int64
	activeConnections int64
}

type RuntimeStatsService struct {
	db   *gorm.DB
	xray *XrayManager

	mu                   sync.Mutex
	rateLast             map[string]ioSample
	cpuLast              cpuSample
	lastCleanupAt        time.Time
	nowFn                func() time.Time
	trafficProvider      func(context.Context) (map[string]int64, error)
	onlineListProvider   func(context.Context) ([]string, error)
	onlineCountsProvider func(context.Context, []string) (map[string]int64, error)
}

func NewRuntimeStatsService(db *gorm.DB, xray *XrayManager) *RuntimeStatsService {
	svc := &RuntimeStatsService{
		db:       db,
		xray:     xray,
		rateLast: map[string]ioSample{},
		nowFn:    time.Now,
	}
	if xray != nil {
		svc.trafficProvider = xray.QueryUserTraffic
		svc.onlineListProvider = xray.GetAllOnlineUsers
		svc.onlineCountsProvider = xray.GetOnlineCounts
	} else {
		svc.trafficProvider = func(context.Context) (map[string]int64, error) { return map[string]int64{}, nil }
		svc.onlineListProvider = func(context.Context) ([]string, error) { return []string{}, nil }
		svc.onlineCountsProvider = func(context.Context, []string) (map[string]int64, error) { return map[string]int64{}, nil }
	}
	return svc
}

func (s *RuntimeStatsService) Capture(ctx context.Context) error {
	_, err := s.capture(ctx, 30)
	return err
}

func (s *RuntimeStatsService) Snapshot(ctx context.Context) ([]CustomerRuntimeStat, error) {
	capture, err := s.capture(ctx, 30)
	if err != nil {
		return nil, err
	}
	return capture.overview.Customers, nil
}

func (s *RuntimeStatsService) Overview(ctx context.Context, limit int) (RuntimeOverviewStat, error) {
	capture, err := s.capture(ctx, limit)
	if err != nil {
		return RuntimeOverviewStat{}, err
	}
	return capture.overview, nil
}

func (s *RuntimeStatsService) TelemetrySnapshot(ctx context.Context) (NodeTelemetryStat, error) {
	capture, err := s.capture(ctx, 30)
	if err != nil {
		return NodeTelemetryStat{}, err
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	s.mu.Lock()
	defer s.mu.Unlock()

	var cpuPercent *float64
	if cpuTime, ok := currentProcessCPUTime(); ok {
		if percent, hasPercent := cpuRate(s.nowFn(), s.cpuLast, cpuTime); hasPercent {
			cpuPercent = &percent
		}
		s.cpuLast = cpuSample{At: s.nowFn(), CPUTime: cpuTime}
	}

	return NodeTelemetryStat{
		ActiveConnections: capture.activeConnections,
		ActiveUsers:       capture.activeUsers,
		UploadBPS:         capture.totalUploadBPS,
		DownloadBPS:       capture.totalDownloadBPS,
		Traffic24H:        capture.totalTraffic24H,
		TrafficTotal:      capture.totalMeasure.Total(),
		CPUPercent:        cpuPercent,
		MemoryBytes:       mem.Alloc,
		Routes:            capture.routeStats,
		UpdatedAt:         capture.overview.UpdatedAt,
	}, nil
}

func (s *RuntimeStatsService) capture(ctx context.Context, limit int) (runtimeCapture, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 200 {
		limit = 200
	}

	data, err := s.collectRuntimeDataset(ctx)
	if err != nil {
		return runtimeCapture{}, err
	}

	customerMeasures := make(map[uint]runtimeMeasure, len(data.customerUsers))
	groupMeasures := make(map[uint]runtimeMeasure, len(data.groupUsers))
	orderMeasures := make(map[uint]runtimeMeasure, len(data.orderUsers))
	routeMeasures := make(map[string]runtimeMeasure, len(data.routeUsers))

	allUsers := map[string]struct{}{}
	for customerID, users := range data.customerUsers {
		measure := aggregateUsers(users, data)
		customerMeasures[customerID] = measure
		for username := range users {
			allUsers[username] = struct{}{}
		}
	}
	for groupID, users := range data.groupUsers {
		groupMeasures[groupID] = aggregateUsers(users, data)
	}
	for orderID, users := range data.orderUsers {
		orderMeasures[orderID] = aggregateUsers(users, data)
	}
	for routeKey, users := range data.routeUsers {
		routeMeasures[routeKey] = aggregateUsers(users, data)
	}
	totalMeasure := aggregateUsers(allUsers, data)

	if err := s.persistSnapshots(ctx, data.now, customerMeasures, groupMeasures, orderMeasures, routeMeasures, totalMeasure); err != nil {
		return runtimeCapture{}, err
	}

	oldestThreshold := data.now.Add(-7 * 24 * time.Hour)
	customerHistory, err := s.loadSnapshotHistory(ctx, runtimeScopeCustomer, uintKeys(customerMeasures), oldestThreshold)
	if err != nil {
		return runtimeCapture{}, err
	}
	groupHistory, err := s.loadSnapshotHistory(ctx, runtimeScopeGroup, uintKeys(groupMeasures), oldestThreshold)
	if err != nil {
		return runtimeCapture{}, err
	}
	orderHistory, err := s.loadSnapshotHistory(ctx, runtimeScopeOrder, uintKeys(orderMeasures), oldestThreshold)
	if err != nil {
		return runtimeCapture{}, err
	}
	routeHistory, err := s.loadSnapshotHistory(ctx, runtimeScopeRoute, stringKeys(routeMeasures), oldestThreshold)
	if err != nil {
		return runtimeCapture{}, err
	}
	totalHistory, err := s.loadSnapshotHistory(ctx, runtimeScopeTotal, []string{"all"}, oldestThreshold)
	if err != nil {
		return runtimeCapture{}, err
	}

	customers := make([]CustomerRuntimeStat, 0, len(customerMeasures))
	for customerID, measure := range customerMeasures {
		meta := data.customerMetas[customerID]
		history := customerHistory[uintKey(customerID)]
		routeUsers := data.customerRouteUsers[customerID]
		homeUsers := routeUsers["home"]
		dedicatedUsers := routeUsers["dedicated"]
		customers = append(customers, CustomerRuntimeStat{
			CustomerID:             customerID,
			CustomerName:           meta.Name,
			CustomerCode:           meta.Code,
			HomeItems:              data.customerRouteItems[customerID]["home"],
			DedicatedItems:         data.customerRouteItems[customerID]["dedicated"],
			HomeOnlineClients:      aggregateUsers(homeUsers, data).Online,
			DedicatedOnlineClients: aggregateUsers(dedicatedUsers, data).Online,
			OnlineClients:          measure.Online,
			RealtimeBPS:            s.measureRate(runtimeScopeCustomer, uintKey(customerID), data.now, measure, history),
			Traffic1H:              trafficSince(history, measure.Total(), data.now.Add(-1*time.Hour)),
			Traffic24H:             trafficSince(history, measure.Total(), data.now.Add(-24*time.Hour)),
			Traffic7D:              trafficSince(history, measure.Total(), data.now.Add(-7*24*time.Hour)),
			TrafficTotal:           measure.Total(),
			UpdatedAt:              data.now,
		})
	}
	sort.Slice(customers, func(i, j int) bool {
		return runtimeRowLess(customers[i].OnlineClients, customers[i].RealtimeBPS, customers[i].Traffic24H, customers[i].CustomerID, customers[j].OnlineClients, customers[j].RealtimeBPS, customers[j].Traffic24H, customers[j].CustomerID)
	})
	if len(customers) > limit {
		customers = customers[:limit]
	}

	groups := make([]OrderGroupRuntimeStat, 0, len(groupMeasures))
	for groupID, measure := range groupMeasures {
		meta := data.groupMetas[groupID]
		history := groupHistory[uintKey(groupID)]
		groups = append(groups, OrderGroupRuntimeStat{
			GroupID:       groupID,
			GroupOrderNo:  meta.GroupOrderNo,
			GroupName:     meta.GroupName,
			CustomerID:    meta.CustomerID,
			CustomerName:  meta.CustomerName,
			CustomerCode:  meta.CustomerCode,
			Mode:          meta.Mode,
			OrderCount:    meta.OrderCount,
			ActiveItems:   meta.ActiveItems,
			OnlineClients: measure.Online,
			RealtimeBPS:   s.measureRate(runtimeScopeGroup, uintKey(groupID), data.now, measure, history),
			Traffic1H:     trafficSince(history, measure.Total(), data.now.Add(-1*time.Hour)),
			Traffic24H:    trafficSince(history, measure.Total(), data.now.Add(-24*time.Hour)),
			Traffic7D:     trafficSince(history, measure.Total(), data.now.Add(-7*24*time.Hour)),
			TrafficTotal:  measure.Total(),
			UpdatedAt:     data.now,
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return runtimeRowLess(groups[i].OnlineClients, groups[i].RealtimeBPS, groups[i].Traffic24H, groups[i].GroupID, groups[j].OnlineClients, groups[j].RealtimeBPS, groups[j].Traffic24H, groups[j].GroupID)
	})
	if len(groups) > limit {
		groups = groups[:limit]
	}

	orders := make([]OrderRuntimeStat, 0, len(orderMeasures))
	for orderID, measure := range orderMeasures {
		meta := data.orderMetas[orderID]
		history := orderHistory[uintKey(orderID)]
		orders = append(orders, OrderRuntimeStat{
			OrderID:       orderID,
			OrderNo:       meta.OrderNo,
			OrderName:     meta.OrderName,
			GroupID:       meta.GroupID,
			GroupOrderNo:  meta.GroupOrderNo,
			CustomerID:    meta.CustomerID,
			CustomerName:  meta.CustomerName,
			CustomerCode:  meta.CustomerCode,
			Mode:          meta.Mode,
			Quantity:      meta.Quantity,
			ActiveItems:   meta.ActiveItems,
			OnlineClients: measure.Online,
			RealtimeBPS:   s.measureRate(runtimeScopeOrder, uintKey(orderID), data.now, measure, history),
			Traffic1H:     trafficSince(history, measure.Total(), data.now.Add(-1*time.Hour)),
			Traffic24H:    trafficSince(history, measure.Total(), data.now.Add(-24*time.Hour)),
			Traffic7D:     trafficSince(history, measure.Total(), data.now.Add(-7*24*time.Hour)),
			TrafficTotal:  measure.Total(),
			UpdatedAt:     data.now,
		})
	}
	sort.Slice(orders, func(i, j int) bool {
		return runtimeRowLess(orders[i].OnlineClients, orders[i].RealtimeBPS, orders[i].Traffic24H, orders[i].OrderID, orders[j].OnlineClients, orders[j].RealtimeBPS, orders[j].Traffic24H, orders[j].OrderID)
	})
	if len(orders) > limit {
		orders = orders[:limit]
	}

	routeStats := make([]NodeTelemetryRouteStat, 0, len(routeMeasures))
	for _, key := range stringKeys(routeMeasures) {
		measure := routeMeasures[key]
		history := routeHistory[key]
		uploadBPS, downloadBPS := s.routeRates(key, data.now, measure, history)
		routeStats = append(routeStats, NodeTelemetryRouteStat{
			Key:               key,
			Label:             telemetryRouteLabel(key),
			ActiveConnections: measure.Online,
			ActiveUsers:       activeUserCount(data.routeUsers[key], data),
			UploadBPS:         uploadBPS,
			DownloadBPS:       downloadBPS,
			Traffic24H:        trafficSince(history, measure.Total(), data.now.Add(-24*time.Hour)),
			TrafficTotal:      measure.Total(),
			UpdatedAt:         data.now,
		})
	}

	totalUploadBPS, totalDownloadBPS := s.routeRates("all", data.now, totalMeasure, totalHistory["all"])
	s.mu.Lock()
	s.rateLast[rateKey(runtimeScopeTotal, "all")] = ioSample{At: data.now, Uplink: totalMeasure.Uplink, Downlink: totalMeasure.Downlink}
	s.rateLast[rateKey(runtimeScopeTotal+":down", "all")] = ioSample{At: data.now, Uplink: totalMeasure.Uplink, Downlink: totalMeasure.Downlink}
	s.mu.Unlock()

	return runtimeCapture{
		overview: RuntimeOverviewStat{
			Customers: customers,
			Groups:    groups,
			Orders:    orders,
			Warnings:  runtimeWarningsForDataset(data),
			UpdatedAt: data.now,
		},
		routeStats:        routeStats,
		totalUploadBPS:    totalUploadBPS,
		totalDownloadBPS:  totalDownloadBPS,
		totalTraffic24H:   trafficSince(totalHistory["all"], totalMeasure.Total(), data.now.Add(-24*time.Hour)),
		totalMeasure:      runtimeMeasure{Uplink: totalMeasure.Uplink, Downlink: totalMeasure.Downlink, Online: totalMeasure.Online},
		activeUsers:       activeUserCount(allUsers, data),
		activeConnections: totalMeasure.Online,
	}, nil
}

func (s *RuntimeStatsService) persistSnapshots(ctx context.Context, now time.Time, customerMeasures map[uint]runtimeMeasure, groupMeasures map[uint]runtimeMeasure, orderMeasures map[uint]runtimeMeasure, routeMeasures map[string]runtimeMeasure, totalMeasure runtimeMeasure) error {
	rows := make([]model.RuntimeTrafficSnapshot, 0, len(customerMeasures)+len(groupMeasures)+len(orderMeasures)+len(routeMeasures)+1)
	bucketAt := now.Truncate(time.Minute)
	for _, key := range sortedUintMapKeys(customerMeasures) {
		measure := customerMeasures[key]
		rows = append(rows, runtimeSnapshotRow(runtimeScopeCustomer, uintKey(key), bucketAt, now, measure))
	}
	for _, key := range sortedUintMapKeys(groupMeasures) {
		measure := groupMeasures[key]
		rows = append(rows, runtimeSnapshotRow(runtimeScopeGroup, uintKey(key), bucketAt, now, measure))
	}
	for _, key := range sortedUintMapKeys(orderMeasures) {
		measure := orderMeasures[key]
		rows = append(rows, runtimeSnapshotRow(runtimeScopeOrder, uintKey(key), bucketAt, now, measure))
	}
	for _, key := range stringKeys(routeMeasures) {
		rows = append(rows, runtimeSnapshotRow(runtimeScopeRoute, key, bucketAt, now, routeMeasures[key]))
	}
	rows = append(rows, runtimeSnapshotRow(runtimeScopeTotal, "all", bucketAt, now, totalMeasure))

	if len(rows) > 0 {
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "scope"}, {Name: "entity_key"}, {Name: "bucket_at"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"sampled_at":     now,
				"uplink_bytes":   gorm.Expr("excluded.uplink_bytes"),
				"downlink_bytes": gorm.Expr("excluded.downlink_bytes"),
				"total_bytes":    gorm.Expr("excluded.total_bytes"),
				"online_clients": gorm.Expr("excluded.online_clients"),
				"updated_at":     now,
			}),
		}).Create(&rows).Error; err != nil {
			return err
		}
	}

	s.mu.Lock()
	needsCleanup := s.lastCleanupAt.IsZero() || now.Sub(s.lastCleanupAt) >= time.Hour
	if needsCleanup {
		s.lastCleanupAt = now
	}
	s.mu.Unlock()
	if needsCleanup {
		cutoff := now.Add(-8 * 24 * time.Hour)
		if err := s.db.WithContext(ctx).Where("bucket_at < ?", cutoff).Delete(&model.RuntimeTrafficSnapshot{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func runtimeSnapshotRow(scope string, entityKey string, bucketAt time.Time, now time.Time, measure runtimeMeasure) model.RuntimeTrafficSnapshot {
	return model.RuntimeTrafficSnapshot{
		Scope:         scope,
		EntityKey:     entityKey,
		BucketAt:      bucketAt,
		SampledAt:     now,
		UplinkBytes:   measure.Uplink,
		DownlinkBytes: measure.Downlink,
		TotalBytes:    measure.Total(),
		OnlineClients: measure.Online,
	}
}

func (s *RuntimeStatsService) loadSnapshotHistory(ctx context.Context, scope string, keys []string, oldestThreshold time.Time) (map[string][]trafficSample, error) {
	out := make(map[string][]trafficSample, len(keys))
	if len(keys) == 0 {
		return out, nil
	}

	type row struct {
		EntityKey     string
		SampledAt     time.Time
		UplinkBytes   int64
		DownlinkBytes int64
		TotalBytes    int64
	}
	cutoff := oldestThreshold.Add(-time.Minute)
	for start := 0; start < len(keys); start += 300 {
		end := start + 300
		if end > len(keys) {
			end = len(keys)
		}
		rows := make([]row, 0)
		if err := s.db.WithContext(ctx).
			Model(&model.RuntimeTrafficSnapshot{}).
			Select("entity_key", "sampled_at", "uplink_bytes", "downlink_bytes", "total_bytes").
			Where("scope = ? and entity_key in ? and sampled_at >= ?", scope, keys[start:end], cutoff).
			Order("sampled_at asc").
			Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, item := range rows {
			out[item.EntityKey] = append(out[item.EntityKey], trafficSample{
				At:       item.SampledAt,
				Uplink:   item.UplinkBytes,
				Downlink: item.DownlinkBytes,
				Total:    item.TotalBytes,
			})
		}
	}
	return out, nil
}

func (s *RuntimeStatsService) collectRuntimeDataset(ctx context.Context) (runtimeDataset, error) {
	type row struct {
		CustomerID     uint
		CustomerName   string
		CustomerCode   string
		OrderID        uint
		OrderNo        string
		OrderName      string
		Mode           string
		Quantity       int
		Username       string
		GroupOrderID   uint
		GroupOrderNo   string
		GroupOrderName string
	}
	rows := []row{}
	if err := s.db.WithContext(ctx).Table("order_items oi").
		Select(`
			o.customer_id as customer_id,
			c.name as customer_name,
			c.code as customer_code,
			o.id as order_id,
			o.order_no as order_no,
			o.name as order_name,
			o.mode as mode,
			o.quantity as quantity,
			oi.username as username,
			coalesce(p.id, o.id) as group_order_id,
			coalesce(p.order_no, o.order_no) as group_order_no,
			coalesce(p.name, o.name) as group_order_name
		`).
		Joins("join orders o on o.id = oi.order_id").
		Joins("join customers c on c.id = o.customer_id").
		Joins("left join orders p on p.id = o.parent_order_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, s.nowFn(), model.OrderItemStatusActive).
		Where("oi.username <> ''").
		Find(&rows).Error; err != nil {
		return runtimeDataset{}, err
	}

	data := runtimeDataset{
		now:                s.nowFn(),
		customerUsers:      map[uint]map[string]struct{}{},
		customerMetas:      map[uint]runtimeCustomerMeta{},
		customerRouteUsers: map[uint]map[string]map[string]struct{}{},
		customerRouteItems: map[uint]map[string]int{},
		groupUsers:         map[uint]map[string]struct{}{},
		groupMetas:         map[uint]runtimeGroupMeta{},
		orderUsers:         map[uint]map[string]struct{}{},
		orderMetas:         map[uint]runtimeOrderMeta{},
		routeUsers:         map[string]map[string]struct{}{},
		trafficByUser:      map[string]userTraffic{},
		onlineByUser:       map[string]int64{},
		conflictedUsers:    map[string]struct{}{},
	}
	groupOrders := map[uint]map[uint]struct{}{}
	usernameOrders := map[string]map[uint]struct{}{}

	for _, r := range rows {
		user := strings.TrimSpace(r.Username)
		if user == "" {
			continue
		}
		if _, ok := usernameOrders[user]; !ok {
			usernameOrders[user] = map[uint]struct{}{}
		}
		usernameOrders[user][r.OrderID] = struct{}{}
	}
	for user, orderIDs := range usernameOrders {
		if len(orderIDs) > 1 {
			data.conflictedUsers[user] = struct{}{}
		}
	}

	for _, r := range rows {
		if _, conflicted := data.conflictedUsers[strings.TrimSpace(r.Username)]; conflicted {
			continue
		}
		if _, ok := data.customerUsers[r.CustomerID]; !ok {
			data.customerUsers[r.CustomerID] = map[string]struct{}{}
		}
		data.customerUsers[r.CustomerID][r.Username] = struct{}{}
		data.customerMetas[r.CustomerID] = runtimeCustomerMeta{Name: r.CustomerName, Code: r.CustomerCode}

		routeKey := telemetryRouteKeyForMode(r.Mode)
		if _, ok := data.customerRouteUsers[r.CustomerID]; !ok {
			data.customerRouteUsers[r.CustomerID] = map[string]map[string]struct{}{}
		}
		if _, ok := data.customerRouteItems[r.CustomerID]; !ok {
			data.customerRouteItems[r.CustomerID] = map[string]int{}
		}
		if _, ok := data.customerRouteUsers[r.CustomerID][routeKey]; !ok {
			data.customerRouteUsers[r.CustomerID][routeKey] = map[string]struct{}{}
		}
		data.customerRouteUsers[r.CustomerID][routeKey][r.Username] = struct{}{}
		data.customerRouteItems[r.CustomerID][routeKey]++

		if r.GroupOrderID != r.OrderID {
			if _, ok := data.groupUsers[r.GroupOrderID]; !ok {
				data.groupUsers[r.GroupOrderID] = map[string]struct{}{}
			}
			data.groupUsers[r.GroupOrderID][r.Username] = struct{}{}
			if _, ok := groupOrders[r.GroupOrderID]; !ok {
				groupOrders[r.GroupOrderID] = map[uint]struct{}{}
			}
			groupOrders[r.GroupOrderID][r.OrderID] = struct{}{}
			meta := data.groupMetas[r.GroupOrderID]
			meta.GroupOrderNo = r.GroupOrderNo
			meta.GroupName = r.GroupOrderName
			meta.CustomerID = r.CustomerID
			meta.CustomerName = r.CustomerName
			meta.CustomerCode = r.CustomerCode
			meta.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
			meta.ActiveItems++
			meta.OrderCount = len(groupOrders[r.GroupOrderID])
			data.groupMetas[r.GroupOrderID] = meta
		}

		if _, ok := data.orderUsers[r.OrderID]; !ok {
			data.orderUsers[r.OrderID] = map[string]struct{}{}
		}
		data.orderUsers[r.OrderID][r.Username] = struct{}{}
		meta := data.orderMetas[r.OrderID]
		meta.OrderNo = r.OrderNo
		meta.OrderName = r.OrderName
		meta.GroupID = r.GroupOrderID
		meta.GroupOrderNo = r.GroupOrderNo
		meta.CustomerID = r.CustomerID
		meta.CustomerName = r.CustomerName
		meta.CustomerCode = r.CustomerCode
		meta.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
		meta.Quantity = r.Quantity
		meta.ActiveItems++
		data.orderMetas[r.OrderID] = meta

		if _, ok := data.routeUsers[routeKey]; !ok {
			data.routeUsers[routeKey] = map[string]struct{}{}
		}
		data.routeUsers[routeKey][r.Username] = struct{}{}
	}

	trafficRaw, err := s.trafficProvider(ctx)
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

	onlineRaw, err := s.onlineListProvider(ctx)
	if err != nil {
		return runtimeDataset{}, err
	}
	onlineCounts, err := s.onlineCountsProvider(ctx, onlineRaw)
	if err != nil {
		return runtimeDataset{}, err
	}
	for _, onlineStatName := range onlineRaw {
		user, ok := parseUserOnlineKey(onlineStatName)
		if !ok {
			continue
		}
		count := onlineCounts[onlineStatName]
		data.onlineByUser[user] += count
	}
	return data, nil
}

func aggregateUsers(users map[string]struct{}, data runtimeDataset) runtimeMeasure {
	if len(users) == 0 {
		return runtimeMeasure{}
	}
	measure := runtimeMeasure{}
	for username := range users {
		traffic := data.trafficByUser[username]
		measure.Uplink += traffic.Uplink
		measure.Downlink += traffic.Downlink
		measure.Online += data.onlineByUser[username]
	}
	return measure
}

func activeUserCount(users map[string]struct{}, data runtimeDataset) int64 {
	count := int64(0)
	for username := range users {
		if data.onlineByUser[username] > 0 {
			count++
		}
	}
	return count
}

func (s *RuntimeStatsService) measureRate(scope string, entityKey string, now time.Time, measure runtimeMeasure, history []trafficSample) float64 {
	s.mu.Lock()
	prev := s.rateLast[rateKey(scope, entityKey)]
	s.rateLast[rateKey(scope, entityKey)] = ioSample{At: now, Uplink: measure.Uplink, Downlink: measure.Downlink}
	s.mu.Unlock()
	return realtimeRateFromMeasure(prev, history, measure, now)
}

func (s *RuntimeStatsService) routeRates(entityKey string, now time.Time, measure runtimeMeasure, history []trafficSample) (float64, float64) {
	s.mu.Lock()
	prev := s.rateLast[rateKey(runtimeScopeRoute, entityKey)]
	s.rateLast[rateKey(runtimeScopeRoute, entityKey)] = ioSample{At: now, Uplink: measure.Uplink, Downlink: measure.Downlink}
	s.mu.Unlock()
	return uploadDownloadRates(prev, history, measure, now)
}

func realtimeRateFromMeasure(prev ioSample, history []trafficSample, measure runtimeMeasure, now time.Time) float64 {
	if prev.At.IsZero() || !now.After(prev.At) || measure.Uplink < prev.Uplink || measure.Downlink < prev.Downlink {
		if rate, ok := historicalRate(history, measure.Total(), now, func(sample trafficSample) int64 { return sample.Total }); ok {
			return rate
		}
		return 0
	}
	seconds := now.Sub(prev.At).Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(measure.Total()-(prev.Uplink+prev.Downlink)) / seconds
}

func uploadDownloadRates(prev ioSample, history []trafficSample, measure runtimeMeasure, now time.Time) (float64, float64) {
	if prev.At.IsZero() || !now.After(prev.At) || measure.Uplink < prev.Uplink || measure.Downlink < prev.Downlink {
		upload, _ := historicalRate(history, measure.Uplink, now, func(sample trafficSample) int64 { return sample.Uplink })
		download, _ := historicalRate(history, measure.Downlink, now, func(sample trafficSample) int64 { return sample.Downlink })
		return upload, download
	}
	seconds := now.Sub(prev.At).Seconds()
	if seconds <= 0 {
		return 0, 0
	}
	return float64(measure.Uplink-prev.Uplink) / seconds, float64(measure.Downlink-prev.Downlink) / seconds
}

func downloadRateFromMeasure(prev ioSample, history []trafficSample, measure runtimeMeasure, now time.Time) float64 {
	_, download := uploadDownloadRates(prev, history, measure, now)
	return download
}

func historicalRate(history []trafficSample, current int64, now time.Time, selector func(trafficSample) int64) (float64, bool) {
	if len(history) == 0 {
		return 0, false
	}
	for i := len(history) - 1; i >= 0; i-- {
		sample := history[i]
		base := selector(sample)
		if !now.After(sample.At) || current < base {
			continue
		}
		seconds := now.Sub(sample.At).Seconds()
		if seconds <= 0 {
			continue
		}
		return float64(current-base) / seconds, true
	}
	return 0, false
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

func runtimeRowLess(leftOnline int64, leftRealtime float64, leftTraffic24H int64, leftID uint, rightOnline int64, rightRealtime float64, rightTraffic24H int64, rightID uint) bool {
	if leftOnline != rightOnline {
		return leftOnline > rightOnline
	}
	if leftRealtime != rightRealtime {
		return leftRealtime > rightRealtime
	}
	if leftTraffic24H != rightTraffic24H {
		return leftTraffic24H > rightTraffic24H
	}
	return leftID < rightID
}

func uintKey(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func uintKeys[V any](in map[uint]V) []string {
	keys := make([]string, 0, len(in))
	for _, key := range sortedUintMapKeys(in) {
		keys = append(keys, uintKey(key))
	}
	return keys
}

func stringKeys[V any](in map[string]V) []string {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedUintMapKeys[V any](in map[uint]V) []uint {
	keys := make([]uint, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func rateKey(scope string, entityKey string) string {
	return scope + ":" + entityKey
}

func runtimeWarningsForDataset(data runtimeDataset) []string {
	if len(data.conflictedUsers) == 0 {
		return nil
	}
	names := stringKeys(data.conflictedUsers)
	display := names
	if len(display) > 5 {
		display = display[:5]
	}
	return []string{
		fmt.Sprintf("检测到 %d 个跨订单重复用户名，相关流量未计入客户/订单组/订单归属统计：%s", len(names), strings.Join(display, ", ")),
	}
}
