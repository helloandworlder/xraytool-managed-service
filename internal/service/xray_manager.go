package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"xraytool/internal/config"
	"xraytool/internal/model"

	handlercmd "github.com/xtls/xray-core/app/proxyman/command"
	routercmd "github.com/xtls/xray-core/app/router/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	cserial "github.com/xtls/xray-core/common/serial"
	core "github.com/xtls/xray-core/core"
	xconf "github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/infra/conf/serial"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type XrayManager struct {
	cfg config.Config
	db  *gorm.DB
	log *zap.Logger

	mu  sync.Mutex
	cmd *exec.Cmd
}

func NewXrayManager(cfg config.Config, db *gorm.DB, log *zap.Logger) *XrayManager {
	return &XrayManager{cfg: cfg, db: db, log: log}
}

func InboundTag(port int) string {
	return fmt.Sprintf("xtool-in-%d", port)
}

func dedicatedInboundTag(protocol string, port int) string {
	return fmt.Sprintf("xtool-in-%s-%d", strings.ToLower(strings.TrimSpace(protocol)), port)
}

func OutboundTag(itemID uint) string {
	return fmt.Sprintf("xtool-out-%d", itemID)
}

func RuleTag(itemID uint) string {
	return fmt.Sprintf("xtool-rule-%d", itemID)
}

func (m *XrayManager) StartManaged() error {
	if !m.cfg.ManagedXrayEnabled {
		return nil
	}
	if err := m.RebuildConfigFile(context.Background()); err != nil {
		return err
	}
	return m.RestartManaged()
}

func (m *XrayManager) StopManaged() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	m.cmd = nil
}

func (m *XrayManager) RestartManaged() error {
	if !m.cfg.ManagedXrayEnabled {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		m.cmd = nil
	}

	if _, err := os.Stat(m.cfg.XrayBinaryPath); err != nil {
		return fmt.Errorf("xray binary not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.XrayConfigPath), 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(m.cfg.XrayWorkDir, "xray.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}

	cmd := exec.Command(m.cfg.XrayBinaryPath, "run", "-c", m.cfg.XrayConfigPath)
	cmd.Dir = m.cfg.XrayWorkDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	m.cmd = cmd

	go func() {
		err := cmd.Wait()
		_ = logFile.Close()
		if err != nil {
			m.log.Warn("managed xray exited", zap.Error(err))
		}
	}()

	return nil
}

func (m *XrayManager) ApplyOrFallback(ctx context.Context, fn func(ctx context.Context) error) error {
	err := fn(ctx)
	if err == nil {
		return nil
	}
	m.log.Warn("grpc apply failed, fallback to config rebuild", zap.Error(err))
	if rebuildErr := m.RebuildConfigFile(ctx); rebuildErr != nil {
		return fmt.Errorf("grpc err: %w, rebuild err: %v", err, rebuildErr)
	}
	if restartErr := m.RestartManaged(); restartErr != nil {
		return fmt.Errorf("grpc err: %w, restart err: %v", err, restartErr)
	}
	return nil
}

func (m *XrayManager) dial(ctx context.Context) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return grpc.DialContext(ctx, m.cfg.XrayAPIServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

func (m *XrayManager) QueryUserTraffic(ctx context.Context) (map[string]int64, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := statscmd.NewStatsServiceClient(conn)
	resp, err := client.QueryStats(ctx, &statscmd.QueryStatsRequest{Pattern: "user>>>", Reset_: false})
	if err != nil {
		if isStatsUnsupportedErr(err) {
			return map[string]int64{}, nil
		}
		return nil, err
	}
	out := make(map[string]int64, len(resp.Stat))
	for _, row := range resp.Stat {
		if row == nil {
			continue
		}
		out[row.Name] = row.Value
	}
	return out, nil
}

func (m *XrayManager) GetAllOnlineUsers(ctx context.Context) ([]string, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := statscmd.NewStatsServiceClient(conn)
	resp, err := client.GetAllOnlineUsers(ctx, &statscmd.GetAllOnlineUsersRequest{})
	if err != nil {
		if isStatsUnsupportedErr(err) {
			return []string{}, nil
		}
		return nil, err
	}
	return resp.Users, nil
}

func (m *XrayManager) GetOnlineCount(ctx context.Context, onlineStatName string) (int64, error) {
	conn, err := m.dial(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	client := statscmd.NewStatsServiceClient(conn)
	resp, err := client.GetStatsOnline(ctx, &statscmd.GetStatsRequest{Name: onlineStatName, Reset_: false})
	if err != nil {
		if isStatsUnsupportedErr(err) {
			return 0, nil
		}
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return 0, nil
		}
		return 0, err
	}
	if resp.Stat == nil {
		return 0, nil
	}
	return resp.Stat.Value, nil
}

func (m *XrayManager) ApplyOrderItem(ctx context.Context, item model.OrderItem, inboundTag string) (model.XrayResource, error) {
	resource := model.XrayResource{
		OrderItemID: item.ID,
		InboundTag:  inboundTag,
		OutboundTag: OutboundTag(item.ID),
		RuleTag:     RuleTag(item.ID),
	}

	conn, err := m.dial(ctx)
	if err != nil {
		return resource, err
	}
	defer conn.Close()

	handlerClient := handlercmd.NewHandlerServiceClient(conn)
	routerClient := routercmd.NewRoutingServiceClient(conn)

	if err := m.addOutbound(ctx, handlerClient, resource.OutboundTag, item); err != nil {
		return resource, err
	}
	if err := m.addRule(ctx, routerClient, resource.RuleTag, resource.InboundTag, item.Username, resource.OutboundTag); err != nil {
		return resource, err
	}
	return resource, nil
}

func (m *XrayManager) RemoveItemResource(ctx context.Context, resource model.XrayResource) error {
	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	handlerClient := handlercmd.NewHandlerServiceClient(conn)
	routerClient := routercmd.NewRoutingServiceClient(conn)

	if err := m.removeRule(ctx, routerClient, resource.RuleTag); err != nil && !isNotFoundErr(err) {
		return err
	}
	if err := m.removeOutbound(ctx, handlerClient, resource.OutboundTag); err != nil && !isNotFoundErr(err) {
		return err
	}
	return nil
}

func (m *XrayManager) RebuildInboundForPort(ctx context.Context, port int, accounts map[string]string) error {
	conn, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	handlerClient := handlercmd.NewHandlerServiceClient(conn)
	tag := InboundTag(port)

	if err := m.removeInbound(ctx, handlerClient, tag); err != nil && !isNotFoundErr(err) {
		return err
	}
	if len(accounts) == 0 {
		return nil
	}
	return m.addInbound(ctx, handlerClient, tag, port, accounts)
}

func (m *XrayManager) addOutbound(ctx context.Context, client handlercmd.HandlerServiceClient, tag string, item model.OrderItem) error {
	obj := map[string]interface{}{}
	if strings.EqualFold(strings.TrimSpace(item.OutboundType), model.OutboundTypeSocks5) && strings.TrimSpace(item.ForwardAddress) != "" && item.ForwardPort > 0 {
		server := map[string]interface{}{
			"address": item.ForwardAddress,
			"port":    item.ForwardPort,
		}
		if strings.TrimSpace(item.ForwardUsername) != "" || strings.TrimSpace(item.ForwardPassword) != "" {
			server["users"] = []map[string]string{{
				"user": item.ForwardUsername,
				"pass": item.ForwardPassword,
			}}
		}
		obj = map[string]interface{}{
			"outbounds": []map[string]interface{}{{
				"tag":      tag,
				"protocol": "socks",
				"settings": map[string]interface{}{"servers": []map[string]interface{}{server}},
			}},
		}
	} else {
		obj = map[string]interface{}{
			"outbounds": []map[string]interface{}{{
				"tag":         tag,
				"protocol":    "freedom",
				"sendThrough": item.IP,
				"settings":    map[string]interface{}{},
			}},
		}
	}
	outbound, err := decodeOutbound(obj)
	if err != nil {
		return err
	}
	_, err = client.AddOutbound(ctx, &handlercmd.AddOutboundRequest{Outbound: outbound})
	return err
}

func (m *XrayManager) removeOutbound(ctx context.Context, client handlercmd.HandlerServiceClient, tag string) error {
	_, err := client.RemoveOutbound(ctx, &handlercmd.RemoveOutboundRequest{Tag: tag})
	return err
}

func (m *XrayManager) addRule(ctx context.Context, client routercmd.RoutingServiceClient, ruleTag, inboundTag, user, outboundTag string) error {
	rule := map[string]interface{}{
		"routing": map[string]interface{}{
			"rules": []map[string]interface{}{{
				"type":        "field",
				"ruleTag":     ruleTag,
				"inboundTag":  []string{inboundTag},
				"user":        []string{user},
				"outboundTag": outboundTag,
			}},
		},
	}
	tmsg, err := decodeRoutingTypedMessage(rule)
	if err != nil {
		return err
	}
	_, err = client.AddRule(ctx, &routercmd.AddRuleRequest{Config: tmsg, ShouldAppend: true})
	return err
}

func (m *XrayManager) removeRule(ctx context.Context, client routercmd.RoutingServiceClient, ruleTag string) error {
	_, err := client.RemoveRule(ctx, &routercmd.RemoveRuleRequest{RuleTag: ruleTag})
	return err
}

func (m *XrayManager) addInbound(ctx context.Context, client handlercmd.HandlerServiceClient, tag string, port int, accounts map[string]string) error {
	keys := make([]string, 0, len(accounts))
	for k := range accounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	accs := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		accs = append(accs, map[string]string{"user": k, "pass": accounts[k]})
	}

	inboundObj := map[string]interface{}{
		"inbounds": []map[string]interface{}{{
			"tag":      tag,
			"listen":   "0.0.0.0",
			"port":     port,
			"protocol": "mixed",
			"settings": map[string]interface{}{
				"auth":     "password",
				"accounts": accs,
				"udp":      false,
			},
		}},
	}
	inbound, err := decodeInbound(inboundObj)
	if err != nil {
		return err
	}
	_, err = client.AddInbound(ctx, &handlercmd.AddInboundRequest{Inbound: inbound})
	return err
}

func (m *XrayManager) removeInbound(ctx context.Context, client handlercmd.HandlerServiceClient, tag string) error {
	_, err := client.RemoveInbound(ctx, &handlercmd.RemoveInboundRequest{Tag: tag})
	return err
}

func decodeInbound(obj map[string]interface{}) (*core.InboundHandlerConfig, error) {
	confObj, err := decodeConfig(obj)
	if err != nil {
		return nil, err
	}
	if len(confObj.InboundConfigs) == 0 {
		return nil, errors.New("no inbound found")
	}
	return confObj.InboundConfigs[0].Build()
}

func decodeOutbound(obj map[string]interface{}) (*core.OutboundHandlerConfig, error) {
	confObj, err := decodeConfig(obj)
	if err != nil {
		return nil, err
	}
	if len(confObj.OutboundConfigs) == 0 {
		return nil, errors.New("no outbound found")
	}
	return confObj.OutboundConfigs[0].Build()
}

func decodeRoutingTypedMessage(obj map[string]interface{}) (*cserial.TypedMessage, error) {
	confObj, err := decodeConfig(obj)
	if err != nil {
		return nil, err
	}
	if confObj.RouterConfig == nil {
		return nil, errors.New("no routing config found")
	}
	routeCfg, err := confObj.RouterConfig.Build()
	if err != nil {
		return nil, err
	}
	tmsg := cserial.ToTypedMessage(routeCfg)
	if tmsg == nil {
		return nil, errors.New("failed to convert routing config to typed message")
	}
	return tmsg, nil
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist")
}

func (m *XrayManager) RebuildConfigFile(ctx context.Context) error {
	type activeRow struct {
		ItemID          uint
		IP              string
		Port            int
		Username        string
		Password        string
		VmessUUID       string
		Managed         bool
		OrderMode       string
		OrderProtocol   string
		OutboundType    string
		ForwardAddress  string
		ForwardPort     int
		ForwardUsername string
		ForwardPassword string
	}

	var rows []activeRow
	err := m.db.WithContext(ctx).
		Table("order_items oi").
		Select("oi.id as item_id, oi.ip, oi.port, oi.username, oi.password, oi.vmess_uuid, oi.managed, o.mode as order_mode, o.dedicated_protocol as order_protocol, oi.outbound_type, oi.forward_address, oi.forward_port, oi.forward_username, oi.forward_password").
		Joins("join orders o on o.id = oi.order_id").
		Where("oi.status = ? and o.status = ? and o.expires_at > ?", model.OrderItemStatusActive, model.OrderStatusActive, time.Now()).
		Scan(&rows).Error
	if err != nil {
		return err
	}

	legacyMixedAccountsByPort := map[int]map[string]string{}
	dedicatedMixedAccountsByPort := map[int]map[string]string{}
	vmessClientsByPort := map[int]map[string]string{}
	vlessClientsByPort := map[int]map[string]string{}
	ssClientsByPort := map[int]map[string]string{}
	type managedItem struct {
		itemID          uint
		ip              string
		user            string
		password        string
		vmessUUID       string
		inboundTags     []string
		outboundType    string
		forwardAddress  string
		forwardPort     int
		forwardUsername string
		forwardPassword string
	}
	items := make([]managedItem, 0)

	for _, row := range rows {
		if !row.Managed {
			continue
		}
		inboundTags := make([]string, 0, 4)
		if strings.EqualFold(strings.TrimSpace(row.OrderMode), model.OrderModeDedicated) && row.Port > 0 {
			protocol := strings.ToLower(strings.TrimSpace(row.OrderProtocol))
			if protocol == "" {
				protocol = model.DedicatedFeatureMixed
			}
			switch protocol {
			case model.DedicatedFeatureVmess:
				if strings.TrimSpace(row.VmessUUID) != "" {
					if _, exists := vmessClientsByPort[row.Port]; !exists {
						vmessClientsByPort[row.Port] = map[string]string{}
					}
					vmessClientsByPort[row.Port][row.Username] = strings.TrimSpace(row.VmessUUID)
					inboundTags = append(inboundTags, dedicatedInboundTag(model.DedicatedFeatureVmess, row.Port))
				}
			case model.DedicatedFeatureVless:
				if strings.TrimSpace(row.VmessUUID) != "" {
					if _, exists := vlessClientsByPort[row.Port]; !exists {
						vlessClientsByPort[row.Port] = map[string]string{}
					}
					vlessClientsByPort[row.Port][row.Username] = strings.TrimSpace(row.VmessUUID)
					inboundTags = append(inboundTags, dedicatedInboundTag(model.DedicatedFeatureVless, row.Port))
				}
			case model.DedicatedFeatureShadowsocks:
				if _, exists := ssClientsByPort[row.Port]; !exists {
					ssClientsByPort[row.Port] = map[string]string{}
				}
				ssClientsByPort[row.Port][row.Username] = row.Password
				inboundTags = append(inboundTags, dedicatedInboundTag(model.DedicatedFeatureShadowsocks, row.Port))
			default:
				if _, exists := dedicatedMixedAccountsByPort[row.Port]; !exists {
					dedicatedMixedAccountsByPort[row.Port] = map[string]string{}
				}
				dedicatedMixedAccountsByPort[row.Port][row.Username] = row.Password
				inboundTags = append(inboundTags, dedicatedInboundTag(model.DedicatedFeatureMixed, row.Port))
			}
		}
		if len(inboundTags) == 0 && row.Port > 0 {
			if _, ok := legacyMixedAccountsByPort[row.Port]; !ok {
				legacyMixedAccountsByPort[row.Port] = map[string]string{}
			}
			legacyMixedAccountsByPort[row.Port][row.Username] = row.Password
			inboundTags = append(inboundTags, InboundTag(row.Port))
		}
		items = append(items, managedItem{
			itemID:          row.ItemID,
			ip:              row.IP,
			user:            row.Username,
			password:        row.Password,
			vmessUUID:       row.VmessUUID,
			inboundTags:     inboundTags,
			outboundType:    row.OutboundType,
			forwardAddress:  row.ForwardAddress,
			forwardPort:     row.ForwardPort,
			forwardUsername: row.ForwardUsername,
			forwardPassword: row.ForwardPassword,
		})
	}

	inbounds := []map[string]interface{}{
		{
			"tag":      "api-in",
			"listen":   "127.0.0.1",
			"port":     parsePortFromAddress(m.cfg.XrayAPIServer),
			"protocol": "dokodemo-door",
			"settings": map[string]interface{}{"address": "127.0.0.1"},
		},
	}
	ports := make([]int, 0, len(legacyMixedAccountsByPort))
	for p := range legacyMixedAccountsByPort {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	for _, p := range ports {
		accounts := legacyMixedAccountsByPort[p]
		users := make([]string, 0, len(accounts))
		for u := range accounts {
			users = append(users, u)
		}
		sort.Strings(users)
		accs := make([]map[string]string, 0, len(users))
		for _, u := range users {
			accs = append(accs, map[string]string{"user": u, "pass": accounts[u]})
		}
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      InboundTag(p),
			"listen":   "0.0.0.0",
			"port":     p,
			"protocol": "mixed",
			"settings": map[string]interface{}{
				"auth":     "password",
				"accounts": accs,
				"udp":      false,
			},
		})
	}

	dedicatedMixedPorts := make([]int, 0, len(dedicatedMixedAccountsByPort))
	for p := range dedicatedMixedAccountsByPort {
		dedicatedMixedPorts = append(dedicatedMixedPorts, p)
	}
	sort.Ints(dedicatedMixedPorts)
	for _, p := range dedicatedMixedPorts {
		accounts := dedicatedMixedAccountsByPort[p]
		users := make([]string, 0, len(accounts))
		for u := range accounts {
			users = append(users, u)
		}
		sort.Strings(users)
		accs := make([]map[string]string, 0, len(users))
		for _, u := range users {
			accs = append(accs, map[string]string{"user": u, "pass": accounts[u]})
		}
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      dedicatedInboundTag(model.DedicatedFeatureMixed, p),
			"listen":   "0.0.0.0",
			"port":     p,
			"protocol": "mixed",
			"settings": map[string]interface{}{
				"auth":     "password",
				"accounts": accs,
				"udp":      false,
			},
		})
	}

	vmessPorts := make([]int, 0, len(vmessClientsByPort))
	for p := range vmessClientsByPort {
		vmessPorts = append(vmessPorts, p)
	}
	sort.Ints(vmessPorts)
	for _, p := range vmessPorts {
		clientsMap := vmessClientsByPort[p]
		users := make([]string, 0, len(clientsMap))
		for u := range clientsMap {
			users = append(users, u)
		}
		sort.Strings(users)
		clients := make([]map[string]interface{}, 0, len(users))
		for _, user := range users {
			clients = append(clients, map[string]interface{}{
				"id":    clientsMap[user],
				"level": 0,
				"email": user,
			})
		}
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      dedicatedInboundTag(model.DedicatedFeatureVmess, p),
			"listen":   "0.0.0.0",
			"port":     p,
			"protocol": "vmess",
			"settings": map[string]interface{}{
				"clients": clients,
			},
		})
	}

	vlessPorts := make([]int, 0, len(vlessClientsByPort))
	for p := range vlessClientsByPort {
		vlessPorts = append(vlessPorts, p)
	}
	sort.Ints(vlessPorts)
	for _, p := range vlessPorts {
		clientsMap := vlessClientsByPort[p]
		users := make([]string, 0, len(clientsMap))
		for u := range clientsMap {
			users = append(users, u)
		}
		sort.Strings(users)
		clients := make([]map[string]interface{}, 0, len(users))
		for _, user := range users {
			clients = append(clients, map[string]interface{}{
				"id":         clientsMap[user],
				"level":      0,
				"email":      user,
				"decryption": "none",
			})
		}
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      dedicatedInboundTag(model.DedicatedFeatureVless, p),
			"listen":   "0.0.0.0",
			"port":     p,
			"protocol": "vless",
			"settings": map[string]interface{}{
				"clients":    clients,
				"decryption": "none",
			},
		})
	}

	ssPorts := make([]int, 0, len(ssClientsByPort))
	for p := range ssClientsByPort {
		ssPorts = append(ssPorts, p)
	}
	sort.Ints(ssPorts)
	for _, p := range ssPorts {
		clientsMap := ssClientsByPort[p]
		users := make([]string, 0, len(clientsMap))
		for u := range clientsMap {
			users = append(users, u)
		}
		sort.Strings(users)
		clients := make([]map[string]interface{}, 0, len(users))
		for _, user := range users {
			clients = append(clients, map[string]interface{}{
				"password": clientsMap[user],
				"method":   DedicatedShadowsocksMethod,
				"level":    0,
				"email":    user,
			})
		}
		inbounds = append(inbounds, map[string]interface{}{
			"tag":      dedicatedInboundTag(model.DedicatedFeatureShadowsocks, p),
			"listen":   "0.0.0.0",
			"port":     p,
			"protocol": "shadowsocks",
			"settings": map[string]interface{}{
				"network": "tcp",
				"clients": clients,
			},
		})
	}

	outbounds := []map[string]interface{}{
		{"tag": "api", "protocol": "freedom", "settings": map[string]interface{}{}},
		{"tag": "direct", "protocol": "freedom", "settings": map[string]interface{}{}},
	}
	rules := []map[string]interface{}{
		{"type": "field", "inboundTag": []string{"api-in"}, "outboundTag": "api"},
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.outboundType), model.OutboundTypeSocks5) && strings.TrimSpace(item.forwardAddress) != "" && item.forwardPort > 0 {
			server := map[string]interface{}{
				"address": item.forwardAddress,
				"port":    item.forwardPort,
			}
			if strings.TrimSpace(item.forwardUsername) != "" || strings.TrimSpace(item.forwardPassword) != "" {
				server["users"] = []map[string]string{{
					"user": item.forwardUsername,
					"pass": item.forwardPassword,
				}}
			}
			outbounds = append(outbounds, map[string]interface{}{
				"tag":      OutboundTag(item.itemID),
				"protocol": "socks",
				"settings": map[string]interface{}{"servers": []map[string]interface{}{server}},
			})
		} else {
			outbounds = append(outbounds, map[string]interface{}{
				"tag":         OutboundTag(item.itemID),
				"protocol":    "freedom",
				"sendThrough": item.ip,
				"settings":    map[string]interface{}{},
			})
		}
		for idx, inTag := range item.inboundTags {
			ruleTag := RuleTag(item.itemID)
			if idx > 0 {
				ruleTag = fmt.Sprintf("%s-%d", RuleTag(item.itemID), idx+1)
			}
			rules = append(rules, map[string]interface{}{
				"type":        "field",
				"ruleTag":     ruleTag,
				"inboundTag":  []string{inTag},
				"user":        []string{item.user},
				"outboundTag": OutboundTag(item.itemID),
			})
		}
	}

	payload := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"api": map[string]interface{}{
			"tag":      "api",
			"services": []string{"HandlerService", "RoutingService", "StatsService"},
		},
		"stats": map[string]interface{}{},
		"policy": map[string]interface{}{
			"levels": map[string]interface{}{
				"0": map[string]interface{}{
					"statsUserUplink":   true,
					"statsUserDownlink": true,
					"statsUserOnline":   true,
				},
			},
			"system": map[string]interface{}{
				"statsInboundUplink":    true,
				"statsInboundDownlink":  true,
				"statsOutboundUplink":   true,
				"statsOutboundDownlink": true,
			},
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing": map[string]interface{}{
			"domainStrategy": "AsIs",
			"rules":          rules,
		},
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.cfg.XrayConfigPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.cfg.XrayConfigPath, body, 0o644)
}

func isStatsUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "querystats only works its own stats.manager") {
		return true
	}
	if strings.Contains(msg, "getstats") && strings.Contains(msg, "stats.manager") {
		return true
	}
	if strings.Contains(msg, "stats not enabled") {
		return true
	}
	if st, ok := status.FromError(err); ok {
		if st.Code() == codes.Unimplemented || st.Code() == codes.FailedPrecondition {
			return true
		}
	}
	return false
}

func decodeConfig(obj map[string]interface{}) (*xconf.Config, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return serial.DecodeJSONConfig(bytes.NewReader(b))
}

func parsePortFromAddress(address string) int {
	_, p, err := net.SplitHostPort(address)
	if err != nil {
		parts := strings.Split(address, ":")
		if len(parts) < 2 {
			return 10085
		}
		p = parts[len(parts)-1]
	}
	v, err := strconv.Atoi(p)
	if err != nil {
		return 10085
	}
	return v
}
