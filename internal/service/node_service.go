package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"xraytool/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type NodeService struct {
	db     *gorm.DB
	log    *zap.Logger
	client *http.Client
}

type NodeInput struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
	IsLocal  bool   `json:"is_local"`
}

type SocksMigrationPreviewRow struct {
	Raw      string `json:"raw"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	NodeID   uint   `json:"node_id,omitempty"`
	NodeName string `json:"node_name,omitempty"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
}

type SocksMigrationNodeSummary struct {
	NodeID         uint   `json:"node_id,omitempty"`
	NodeName       string `json:"node_name"`
	IsLocal        bool   `json:"is_local"`
	Reachable      bool   `json:"reachable"`
	AssignedCount  int    `json:"assigned_count"`
	ReadyCount     int    `json:"ready_count"`
	Blocked        bool   `json:"blocked"`
	PortConflicts  []int  `json:"port_conflicts,omitempty"`
	Error          string `json:"error,omitempty"`
	ActionHint     string `json:"action_hint,omitempty"`
	HighlightColor string `json:"highlight_color"`
}

type SocksMigrationPreviewResult struct {
	Rows              []SocksMigrationPreviewRow  `json:"rows"`
	Nodes             []SocksMigrationNodeSummary `json:"nodes"`
	ReadyRows         int                         `json:"ready_rows"`
	BlockedRows       int                         `json:"blocked_rows"`
	UnmatchedRows     int                         `json:"unmatched_rows"`
	AmbiguousRows     int                         `json:"ambiguous_rows"`
	InvalidRows       int                         `json:"invalid_rows"`
	BlockedNodeCount  int                         `json:"blocked_node_count"`
	ReachableNodeSize int                         `json:"reachable_node_size"`
}

type previewNode struct {
	ID       uint
	Name     string
	IsLocal  bool
	BaseURL  string
	Username string
	Password string
}

func NewNodeService(db *gorm.DB, log *zap.Logger) *NodeService {
	return &NodeService{
		db:  db,
		log: log,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (s *NodeService) ListNodes() ([]model.XrayNode, error) {
	rows := []model.XrayNode{}
	err := s.db.Order("is_local desc, id asc").Find(&rows).Error
	return rows, err
}

func (s *NodeService) CreateNode(in NodeInput) (*model.XrayNode, error) {
	row, err := normalizeNodeInput(in)
	if err != nil {
		return nil, err
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *NodeService) UpdateNode(id uint, in NodeInput) (*model.XrayNode, error) {
	if id == 0 {
		return nil, fmt.Errorf("node id is required")
	}
	existing := model.XrayNode{}
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, err
	}
	row, err := normalizeNodeInput(in)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"name":       row.Name,
		"base_url":   row.BaseURL,
		"username":   row.Username,
		"password":   row.Password,
		"enabled":    row.Enabled,
		"is_local":   row.IsLocal,
		"updated_at": time.Now(),
	}
	if err := s.db.Model(&model.XrayNode{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (s *NodeService) DeleteNode(id uint) error {
	if id == 0 {
		return fmt.Errorf("node id is required")
	}
	return s.db.Delete(&model.XrayNode{}, id).Error
}

func (s *NodeService) PreviewSocksMigration(ctx context.Context, lines string) (SocksMigrationPreviewResult, error) {
	result := SocksMigrationPreviewResult{}
	rows := parseMigrationRows(lines)
	result.Rows = rows

	nodes, localIPs, err := s.loadPreviewNodesAndLocalIPs()
	if err != nil {
		return result, err
	}

	ipOwners := map[string][]previewNode{}
	for ip := range localIPs {
		ipOwners[ip] = append(ipOwners[ip], previewNode{ID: 0, Name: "本机 xraytool", IsLocal: true})
	}

	nodeSummaries := make(map[string]*SocksMigrationNodeSummary)
	nodeSummaries[nodeKey(previewNode{ID: 0, Name: "本机 xraytool", IsLocal: true})] = &SocksMigrationNodeSummary{
		NodeID:         0,
		NodeName:       "本机 xraytool",
		IsLocal:        true,
		Reachable:      true,
		HighlightColor: "green",
	}
	for _, node := range nodes {
		if node.IsLocal {
			continue
		}
		summary := &SocksMigrationNodeSummary{
			NodeID:         node.ID,
			NodeName:       node.Name,
			IsLocal:        node.IsLocal,
			Reachable:      true,
			HighlightColor: "green",
		}
		nodeSummaries[nodeKey(node)] = summary
		ips, fetchErr := s.fetchNodeIPs(ctx, node)
		if fetchErr != nil {
			summary.Reachable = false
			summary.HighlightColor = "red"
			summary.Error = fetchErr.Error()
			summary.ActionHint = "节点不可达，请检查地址与账号密码"
			continue
		}
		for ip := range ips {
			ipOwners[ip] = append(ipOwners[ip], node)
		}
	}

	assignedPorts := map[string]map[int]struct{}{}
	for i := range result.Rows {
		row := &result.Rows[i]
		if row.State == "invalid" {
			continue
		}
		owners := ipOwners[row.IP]
		if len(owners) == 0 {
			row.State = "unmatched"
			row.Reason = "未匹配到任何 xraytool 节点"
			continue
		}
		if len(owners) > 1 {
			names := make([]string, 0, len(owners))
			for _, owner := range owners {
				names = append(names, owner.Name)
			}
			sort.Strings(names)
			row.State = "ambiguous"
			row.Reason = "IP 命中多个节点: " + strings.Join(names, ", ")
			continue
		}
		owner := owners[0]
		row.NodeID = owner.ID
		row.NodeName = owner.Name
		row.State = "assigned"
		key := nodeKey(owner)
		if _, ok := assignedPorts[key]; !ok {
			assignedPorts[key] = map[int]struct{}{}
		}
		assignedPorts[key][row.Port] = struct{}{}
		if summary, ok := nodeSummaries[key]; ok {
			summary.AssignedCount++
		}
	}

	for key, ports := range assignedPorts {
		node, ok := findNodeByKey(nodes, key)
		if !ok {
			if key == nodeKey(previewNode{ID: 0, Name: "本机 xraytool", IsLocal: true}) {
				node = previewNode{ID: 0, Name: "本机 xraytool", IsLocal: true}
			} else {
				continue
			}
		}
		occupied := make([]int, 0)
		for port := range ports {
			busy, probeErr := s.probeNodePort(ctx, node, port)
			if probeErr != nil {
				if summary, exists := nodeSummaries[key]; exists {
					summary.Reachable = false
					summary.HighlightColor = "red"
					summary.Error = probeErr.Error()
					summary.ActionHint = "节点探测失败，请检查节点状态"
				}
				busy = true
			}
			if busy {
				occupied = append(occupied, port)
			}
		}
		sort.Ints(occupied)
		if len(occupied) > 0 {
			for i := range result.Rows {
				if result.Rows[i].State != "assigned" {
					continue
				}
				if nodeKey(previewNode{ID: result.Rows[i].NodeID, Name: result.Rows[i].NodeName, IsLocal: result.Rows[i].NodeID == 0}) != key {
					continue
				}
				for _, p := range occupied {
					if result.Rows[i].Port == p {
						result.Rows[i].State = "blocked"
						result.Rows[i].Reason = fmt.Sprintf("目标节点端口 %d 已占用，请先到目标服务器释放占用进程", p)
						break
					}
				}
			}
			if summary, exists := nodeSummaries[key]; exists {
				summary.Blocked = true
				summary.HighlightColor = "red"
				summary.PortConflicts = occupied
				summary.ActionHint = "请到目标服务器释放占用端口后重试"
			}
		}
	}

	for i := range result.Rows {
		if result.Rows[i].State == "assigned" {
			result.Rows[i].State = "ready"
		}
	}

	keys := make([]string, 0, len(nodeSummaries))
	for key := range nodeSummaries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		summary := nodeSummaries[key]
		if summary.AssignedCount > 0 {
			for _, row := range result.Rows {
				if row.NodeName != summary.NodeName {
					continue
				}
				if row.State == "ready" {
					summary.ReadyCount++
				}
			}
		}
		result.Nodes = append(result.Nodes, *summary)
		if summary.Blocked {
			result.BlockedNodeCount++
		}
		if summary.Reachable {
			result.ReachableNodeSize++
		}
	}

	for _, row := range result.Rows {
		switch row.State {
		case "ready":
			result.ReadyRows++
		case "blocked":
			result.BlockedRows++
		case "unmatched":
			result.UnmatchedRows++
		case "ambiguous":
			result.AmbiguousRows++
		default:
			result.InvalidRows++
		}
	}

	return result, nil
}

func normalizeNodeInput(in NodeInput) (model.XrayNode, error) {
	name := strings.TrimSpace(in.Name)
	baseURL := strings.TrimSpace(in.BaseURL)
	username := strings.TrimSpace(in.Username)
	password := strings.TrimSpace(in.Password)
	if name == "" {
		return model.XrayNode{}, fmt.Errorf("name is required")
	}
	if baseURL == "" {
		return model.XrayNode{}, fmt.Errorf("base_url is required")
	}
	if username == "" || password == "" {
		return model.XrayNode{}, fmt.Errorf("username and password are required")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return model.XrayNode{
		Name:     name,
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Enabled:  enabled,
		IsLocal:  in.IsLocal,
	}, nil
}

func parseMigrationRows(lines string) []SocksMigrationPreviewRow {
	out := make([]SocksMigrationPreviewRow, 0)
	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		row := SocksMigrationPreviewRow{Raw: raw}
		parts := strings.Split(raw, ":")
		if len(parts) != 4 {
			row.State = "invalid"
			row.Reason = "格式必须是 ip:port:user:pass"
			out = append(out, row)
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || port <= 0 || port > 65535 {
			row.State = "invalid"
			row.Reason = "端口非法"
			out = append(out, row)
			continue
		}
		row.IP = strings.TrimSpace(parts[0])
		row.Port = port
		row.Username = strings.TrimSpace(parts[2])
		row.Password = strings.TrimSpace(parts[3])
		if row.IP == "" || row.Username == "" || row.Password == "" {
			row.State = "invalid"
			row.Reason = "ip/user/pass 不能为空"
			out = append(out, row)
			continue
		}
		row.State = "parsed"
		out = append(out, row)
	}
	return out
}

func (s *NodeService) loadPreviewNodesAndLocalIPs() ([]previewNode, map[string]struct{}, error) {
	nodes := []model.XrayNode{}
	if err := s.db.Where("enabled = 1").Order("id asc").Find(&nodes).Error; err != nil {
		return nil, nil, err
	}
	hosts := []model.HostIP{}
	if err := s.db.Where("enabled = 1").Find(&hosts).Error; err != nil {
		return nil, nil, err
	}
	local := map[string]struct{}{}
	for _, host := range hosts {
		ip := strings.TrimSpace(host.IP)
		if ip == "" {
			continue
		}
		local[ip] = struct{}{}
	}
	items := make([]previewNode, 0, len(nodes))
	for _, node := range nodes {
		items = append(items, previewNode{
			ID:       node.ID,
			Name:     node.Name,
			IsLocal:  node.IsLocal,
			BaseURL:  strings.TrimRight(strings.TrimSpace(node.BaseURL), "/"),
			Username: strings.TrimSpace(node.Username),
			Password: strings.TrimSpace(node.Password),
		})
	}
	return items, local, nil
}

func (s *NodeService) fetchNodeIPs(ctx context.Context, node previewNode) (map[string]struct{}, error) {
	token, err := s.loginNode(ctx, node)
	if err != nil {
		return nil, err
	}
	url := node.BaseURL + "/api/host-ips"
	var rows []struct {
		IP      string `json:"ip"`
		Enabled bool   `json:"enabled"`
	}
	if err := s.requestJSON(ctx, http.MethodGet, url, token, nil, &rows); err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, row := range rows {
		if !row.Enabled {
			continue
		}
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		set[ip] = struct{}{}
	}
	return set, nil
}

func (s *NodeService) probeNodePort(ctx context.Context, node previewNode, port int) (bool, error) {
	if node.ID == 0 {
		return ProbePort("0.0.0.0", port)
	}
	token, err := s.loginNode(ctx, node)
	if err != nil {
		return true, err
	}
	url := node.BaseURL + "/api/host-ips/probe"
	reqBody := map[string]interface{}{
		"ip":   "0.0.0.0",
		"port": port,
	}
	var resp struct {
		Occupied bool   `json:"occupied"`
		Error    string `json:"error"`
	}
	if err := s.requestJSON(ctx, http.MethodPost, url, token, reqBody, &resp); err != nil {
		return true, err
	}
	if strings.TrimSpace(resp.Error) != "" {
		return true, fmt.Errorf("%s", resp.Error)
	}
	return resp.Occupied, nil
}

func (s *NodeService) loginNode(ctx context.Context, node previewNode) (string, error) {
	url := node.BaseURL + "/api/auth/login"
	reqBody := map[string]string{
		"username": node.Username,
		"password": node.Password,
	}
	var resp struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	if err := s.requestJSON(ctx, http.MethodPost, url, "", reqBody, &resp); err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.Token) == "" {
		if strings.TrimSpace(resp.Error) != "" {
			return "", fmt.Errorf("%s", resp.Error)
		}
		return "", fmt.Errorf("login token empty")
	}
	return resp.Token, nil
}

func (s *NodeService) requestJSON(ctx context.Context, method, url, token string, payload interface{}, out interface{}) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("request %s failed: %d", url, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	return decoder.Decode(out)
}

func nodeKey(node previewNode) string {
	if node.ID == 0 {
		return "local"
	}
	return fmt.Sprintf("node:%d", node.ID)
}

func findNodeByKey(nodes []previewNode, key string) (previewNode, bool) {
	for _, node := range nodes {
		if nodeKey(node) == key {
			return node, true
		}
	}
	return previewNode{}, false
}
