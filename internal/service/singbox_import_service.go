package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

var defaultSingboxConfigPatterns = []string{
	"/etc/sing-box/conf/*.json",
	"/etc/sing-box/*.json",
}

type SingboxScanFile struct {
	Path       string `json:"path"`
	EntryCount int    `json:"entry_count"`
	Selectable bool   `json:"selectable"`
	Error      string `json:"error,omitempty"`
}

type SingboxScanResult struct {
	Files        []SingboxScanFile `json:"files"`
	TotalFiles   int               `json:"total_files"`
	TotalEntries int               `json:"total_entries"`
}

type SingboxImportService struct {
	db     *gorm.DB
	orders *OrderService
}

func NewSingboxImportService(db *gorm.DB, orders *OrderService) *SingboxImportService {
	return &SingboxImportService{db: db, orders: orders}
}

func (s *SingboxImportService) Scan() (SingboxScanResult, error) {
	files, err := discoverSingboxJSONFiles()
	if err != nil {
		return SingboxScanResult{}, err
	}
	result := SingboxScanResult{Files: make([]SingboxScanFile, 0, len(files))}
	for _, path := range files {
		rows, parseErr := parseSingboxRowsFromFile(path)
		entry := SingboxScanFile{
			Path:       path,
			EntryCount: len(rows),
			Selectable: parseErr == nil && len(rows) > 0,
		}
		if parseErr != nil {
			entry.Error = parseErr.Error()
		}
		result.TotalEntries += entry.EntryCount
		result.Files = append(result.Files, entry)
	}
	result.TotalFiles = len(result.Files)
	return result, nil
}

func (s *SingboxImportService) Preview(files []string) ([]ImportPreviewRow, error) {
	targets := normalizeSelectedSingboxFiles(files)
	if len(targets) == 0 {
		all, err := discoverSingboxJSONFiles()
		if err != nil {
			return nil, err
		}
		targets = all
	}

	rows := make([]ImportPreviewRow, 0)
	for _, path := range targets {
		if !isAllowedSingboxPath(path) {
			return nil, fmt.Errorf("file not allowed: %s", path)
		}
		parsed, err := parseSingboxRowsFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("parse %s failed: %w", path, err)
		}
		rows = append(rows, parsed...)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no valid socks5 entries found in selected files")
	}

	hostIPs, err := s.localHostIPSet()
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].IP = strings.TrimSpace(rows[i].IP)
		rows[i].Username = strings.TrimSpace(rows[i].Username)
		rows[i].Password = strings.TrimSpace(rows[i].Password)
		if rows[i].IP == "" || rows[i].Port <= 0 || rows[i].Port > 65535 {
			continue
		}
		_, rows[i].IsLocalIP = hostIPs[rows[i].IP]
		if rows[i].IsLocalIP {
			occupied, _ := ProbePort(rows[i].IP, rows[i].Port)
			rows[i].PortOccupied = occupied
		}
	}

	return s.orders.PreviewImportRows(rows)
}

func (s *SingboxImportService) localHostIPSet() (map[string]uint, error) {
	type hostRow struct {
		ID uint
		IP string
	}
	rows := []hostRow{}
	if err := s.db.Table("host_ips").Select("id, ip").Find(&rows).Error; err != nil {
		return nil, err
	}
	set := map[string]uint{}
	for _, row := range rows {
		ip := strings.TrimSpace(row.IP)
		if ip == "" {
			continue
		}
		set[ip] = row.ID
	}
	return set, nil
}

func discoverSingboxJSONFiles() ([]string, error) {
	seen := map[string]struct{}{}
	files := make([]string, 0)
	for _, pattern := range defaultSingboxConfigPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			clean := filepath.Clean(strings.TrimSpace(match))
			if clean == "" || !isAllowedSingboxPath(clean) {
				continue
			}
			if _, exists := seen[clean]; exists {
				continue
			}
			seen[clean] = struct{}{}
			files = append(files, clean)
		}
	}
	sort.Strings(files)
	return files, nil
}

func normalizeSelectedSingboxFiles(files []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(files))
	for _, path := range files {
		clean := filepath.Clean(strings.TrimSpace(path))
		if clean == "" {
			continue
		}
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func isAllowedSingboxPath(path string) bool {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".json") {
		return false
	}
	return strings.HasPrefix(clean, "/etc/sing-box/")
}

func parseSingboxRowsFromFile(path string) ([]ImportPreviewRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root map[string]interface{}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}

	inbounds := asSlice(root["inbounds"])
	rows := make([]ImportPreviewRow, 0)
	for _, item := range inbounds {
		inbound, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if !isSocksInbound(inbound) {
			continue
		}
		listen := firstNonEmpty(
			asString(inbound["listen"]),
			asString(inbound["listen_address"]),
			asString(inbound["bind"]),
		)
		port := asInt(inbound["listen_port"])
		if port <= 0 {
			port = asInt(inbound["port"])
		}
		users := extractInboundUsers(inbound)
		for _, user := range users {
			row := ImportPreviewRow{
				SourceFile: path,
				IP:         strings.TrimSpace(listen),
				Port:       port,
				Username:   strings.TrimSpace(user[0]),
				Password:   strings.TrimSpace(user[1]),
			}
			row.Raw = fmt.Sprintf("%s:%d:%s:%s", row.IP, row.Port, row.Username, row.Password)
			if row.IP == "" || row.Port <= 0 || row.Username == "" || row.Password == "" {
				row.Error = "missing ip/port/user/pass in sing-box entry"
			}
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func isSocksInbound(inbound map[string]interface{}) bool {
	t := strings.ToLower(strings.TrimSpace(asString(inbound["type"])))
	if t == "" {
		t = strings.ToLower(strings.TrimSpace(asString(inbound["protocol"])))
	}
	return t == "socks" || t == "socks5"
}

func extractInboundUsers(inbound map[string]interface{}) [][2]string {
	users := make([][2]string, 0)
	appendUser := func(user, pass string) {
		user = strings.TrimSpace(user)
		pass = strings.TrimSpace(pass)
		if user == "" && pass == "" {
			return
		}
		users = append(users, [2]string{user, pass})
	}

	for _, item := range asSlice(inbound["users"]) {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		appendUser(firstNonEmpty(asString(m["username"]), asString(m["user"])), firstNonEmpty(asString(m["password"]), asString(m["pass"])))
	}

	settings, _ := inbound["settings"].(map[string]interface{})
	for _, item := range asSlice(settings["accounts"]) {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		appendUser(firstNonEmpty(asString(m["username"]), asString(m["user"])), firstNonEmpty(asString(m["password"]), asString(m["pass"])))
	}

	if len(users) == 0 {
		appendUser(firstNonEmpty(asString(inbound["username"]), asString(inbound["user"])), firstNonEmpty(asString(inbound["password"]), asString(inbound["pass"])))
	}
	if len(users) == 0 {
		users = append(users, [2]string{"", ""})
	}
	return users
}

func asSlice(v interface{}) []interface{} {
	s, ok := v.([]interface{})
	if !ok {
		return []interface{}{}
	}
	return s
}

func asString(v interface{}) string {
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func asInt(v interface{}) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case json.Number:
		n, _ := x.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		clean := strings.TrimSpace(v)
		if clean != "" {
			return clean
		}
	}
	return ""
}
