package service

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"xraytool/internal/model"

	"golang.org/x/net/proxy"
	"gorm.io/gorm"
)

type ForwardOutboundService struct {
	db *gorm.DB
}

type ForwardOutboundInput struct {
	Name      string `json:"name"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	RouteUser string `json:"route_user"`
	Enabled   *bool  `json:"enabled"`
}

type ForwardOutboundImportRow struct {
	Raw       string `json:"raw"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	RouteUser string `json:"route_user"`
	Error     string `json:"error,omitempty"`
}

func NewForwardOutboundService(db *gorm.DB) *ForwardOutboundService {
	return &ForwardOutboundService{db: db}
}

func (s *ForwardOutboundService) List() ([]model.SocksOutbound, error) {
	rows := []model.SocksOutbound{}
	if err := s.db.Order("enabled desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *ForwardOutboundService) Create(in ForwardOutboundInput) (*model.SocksOutbound, error) {
	row, err := normalizeForwardOutboundInput(in)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.RouteUser) != "" {
		if err := s.ensureRouteUserAvailable(row.RouteUser, 0); err != nil {
			return nil, err
		}
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *ForwardOutboundService) Update(id uint, in ForwardOutboundInput) (*model.SocksOutbound, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	base := model.SocksOutbound{}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	row, err := normalizeForwardOutboundInput(in)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.RouteUser) == "" {
		row.RouteUser = base.RouteUser
	}
	if strings.TrimSpace(row.RouteUser) != "" {
		if err := s.ensureRouteUserAvailable(row.RouteUser, id); err != nil {
			return nil, err
		}
	}
	updates := map[string]interface{}{
		"name":       row.Name,
		"address":    row.Address,
		"port":       row.Port,
		"username":   row.Username,
		"password":   row.Password,
		"route_user": row.RouteUser,
		"enabled":    row.Enabled,
		"updated_at": time.Now(),
	}
	if err := s.db.Model(&model.SocksOutbound{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&base, id).Error; err != nil {
		return nil, err
	}
	return &base, nil
}

func (s *ForwardOutboundService) Toggle(id uint, enabled bool) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.db.Model(&model.SocksOutbound{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

func (s *ForwardOutboundService) Delete(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	var count int64
	if err := s.db.Model(&model.OrderItem{}).Joins("join orders o on o.id = order_items.order_id").
		Where("order_items.socks_outbound_id = ? and order_items.status = ? and o.status = ? and o.expires_at > ?", id, model.OrderItemStatusActive, model.OrderStatusActive, time.Now()).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("outbound is used by %d active order items", count)
	}
	return s.db.Delete(&model.SocksOutbound{}, id).Error
}

func (s *ForwardOutboundService) ImportLines(lines string) ([]ForwardOutboundImportRow, error) {
	scanner := bufio.NewScanner(strings.NewReader(lines))
	rows := make([]ForwardOutboundImportRow, 0)
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		row := ForwardOutboundImportRow{Raw: raw}
		parts := strings.Split(raw, ":")
		if len(parts) != 4 && len(parts) != 5 {
			row.Error = "format must be ip:port:user:pass[:route_user]"
			rows = append(rows, row)
			continue
		}
		port, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || port <= 0 || port > 65535 {
			row.Error = "invalid port"
			rows = append(rows, row)
			continue
		}
		row.Address = strings.TrimSpace(parts[0])
		row.Port = port
		row.Username = strings.TrimSpace(parts[2])
		row.Password = strings.TrimSpace(parts[3])
		if len(parts) == 5 {
			row.RouteUser = strings.TrimSpace(parts[4])
		}
		if row.Address == "" || row.Username == "" || row.Password == "" {
			row.Error = "address/user/pass required"
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	for i := range rows {
		if rows[i].Error != "" {
			continue
		}
		key := strings.ToLower(fmt.Sprintf("%s:%d:%s", rows[i].Address, rows[i].Port, rows[i].Username))
		if _, ok := seen[key]; ok {
			rows[i].Error = "duplicate in import list"
			continue
		}
		seen[key] = struct{}{}
	}

	for i := range rows {
		if rows[i].Error != "" {
			continue
		}
		row := rows[i]
		rec := model.SocksOutbound{}
		err := s.db.Where("address = ? and port = ? and username = ?", row.Address, row.Port, row.Username).First(&rec).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			rows[i].Error = err.Error()
			continue
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			routeUser := strings.TrimSpace(row.RouteUser)
			if routeUser != "" {
				if ensureErr := s.ensureRouteUserAvailable(routeUser, 0); ensureErr != nil {
					rows[i].Error = ensureErr.Error()
					continue
				}
			}
			create := model.SocksOutbound{
				Name:      "",
				Address:   row.Address,
				Port:      row.Port,
				Username:  row.Username,
				Password:  row.Password,
				RouteUser: routeUser,
				Enabled:   true,
			}
			if createErr := s.db.Create(&create).Error; createErr != nil {
				rows[i].Error = createErr.Error()
			}
			continue
		}

		routeUser := strings.TrimSpace(row.RouteUser)
		if routeUser == "" {
			routeUser = rec.RouteUser
		}
		if routeUser != "" {
			if ensureErr := s.ensureRouteUserAvailable(routeUser, rec.ID); ensureErr != nil {
				rows[i].Error = ensureErr.Error()
				continue
			}
		}
		updateErr := s.db.Model(&model.SocksOutbound{}).Where("id = ?", rec.ID).Updates(map[string]interface{}{
			"address":    row.Address,
			"port":       row.Port,
			"username":   row.Username,
			"password":   row.Password,
			"route_user": routeUser,
			"enabled":    true,
			"updated_at": time.Now(),
		}).Error
		if updateErr != nil {
			rows[i].Error = updateErr.Error()
		}
	}

	return rows, nil
}

func (s *ForwardOutboundService) Probe(id uint) (*model.SocksOutbound, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	row := model.SocksOutbound{}
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	updated, err := s.probeRow(row)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ForwardOutboundService) ProbeAll(enabledOnly bool) ([]model.SocksOutbound, error) {
	rows := []model.SocksOutbound{}
	q := s.db
	if enabledOnly {
		q = q.Where("enabled = 1")
	}
	if err := q.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		_, _ = s.probeRow(row)
	}
	return s.List()
}

func (s *ForwardOutboundService) normalizeRouteUser(id uint, fallbackCountry string) (string, error) {
	row := model.SocksOutbound{}
	if err := s.db.First(&row, id).Error; err != nil {
		return "", err
	}
	route := strings.TrimSpace(row.RouteUser)
	if route != "" {
		return route, nil
	}
	route, err := s.nextAvailableRouteUser(fallbackCountry)
	if err != nil {
		return "", err
	}
	if err := s.db.Model(&model.SocksOutbound{}).Where("id = ?", id).Updates(map[string]interface{}{
		"route_user": route,
		"updated_at": time.Now(),
	}).Error; err != nil {
		return "", err
	}
	return route, nil
}

func (s *ForwardOutboundService) probeRow(row model.SocksOutbound) (*model.SocksOutbound, error) {
	exitIP, country, probeErr := probeSocksOutbound(row.Address, row.Port, row.Username, row.Password)
	now := time.Now()
	updates := map[string]interface{}{
		"exit_ip":        strings.TrimSpace(exitIP),
		"country_code":   strings.ToLower(strings.TrimSpace(country)),
		"last_probed_at": &now,
		"updated_at":     now,
	}
	if probeErr != nil {
		updates["probe_status"] = "failed"
		updates["probe_error"] = probeErr.Error()
	} else {
		updates["probe_status"] = "ok"
		updates["probe_error"] = ""
		if strings.TrimSpace(row.RouteUser) == "" {
			route, err := s.nextAvailableRouteUser(country)
			if err != nil {
				return nil, err
			}
			updates["route_user"] = route
		}
	}
	if err := s.db.Model(&model.SocksOutbound{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	if probeErr != nil {
		return nil, probeErr
	}
	latest := model.SocksOutbound{}
	if err := s.db.First(&latest, row.ID).Error; err != nil {
		return nil, err
	}
	return &latest, nil
}

func normalizeForwardOutboundInput(in ForwardOutboundInput) (model.SocksOutbound, error) {
	name := strings.TrimSpace(in.Name)
	address := strings.TrimSpace(in.Address)
	username := strings.TrimSpace(in.Username)
	password := strings.TrimSpace(in.Password)
	routeUser := strings.TrimSpace(in.RouteUser)
	if address == "" {
		return model.SocksOutbound{}, errors.New("address is required")
	}
	if in.Port <= 0 || in.Port > 65535 {
		return model.SocksOutbound{}, errors.New("invalid port")
	}
	if username == "" || password == "" {
		return model.SocksOutbound{}, errors.New("username and password are required")
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	return model.SocksOutbound{
		Name:      name,
		Address:   address,
		Port:      in.Port,
		Username:  username,
		Password:  password,
		RouteUser: routeUser,
		Enabled:   enabled,
	}, nil
}

func (s *ForwardOutboundService) ensureRouteUserAvailable(routeUser string, excludeID uint) error {
	routeUser = strings.TrimSpace(routeUser)
	if routeUser == "" {
		return errors.New("route_user is required")
	}
	var count int64
	q := s.db.Model(&model.SocksOutbound{}).Where("route_user = ?", routeUser)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("route_user %s already exists", routeUser)
	}
	itemQuery := s.db.Model(&model.OrderItem{}).Where("username = ?", routeUser)
	if excludeID > 0 {
		itemQuery = itemQuery.Where("socks_outbound_id is null or socks_outbound_id <> ?", excludeID)
	}
	if err := itemQuery.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("route_user %s conflicts with existing order account", routeUser)
	}
	return nil
}

func (s *ForwardOutboundService) nextAvailableRouteUser(country string) (string, error) {
	prefix := normalizeCountryPrefix(country)
	for i := 0; i < 12; i++ {
		candidate := fmt.Sprintf("%s-%s", prefix, randomString(10))
		if err := s.ensureRouteUserAvailable(candidate, 0); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("failed to generate unique route user")
}

func normalizeCountryPrefix(country string) string {
	country = strings.ToLower(strings.TrimSpace(country))
	if len(country) == 2 {
		return country
	}
	return "xx"
}

func probeSocksOutbound(address string, port int, username, password string) (string, string, error) {
	exitIP, country, _, err := probeSocksOutboundGeo(address, port, username, password)
	return exitIP, country, err
}

func probeSocksOutboundGeo(address string, port int, username, password string) (string, string, string, error) {
	socksAddr := fmt.Sprintf("%s:%d", strings.TrimSpace(address), port)
	dialer, err := proxy.SOCKS5("tcp", socksAddr, &proxy.Auth{User: strings.TrimSpace(username), Password: strings.TrimSpace(password)}, proxy.Direct)
	if err != nil {
		return "", "", "", err
	}
	httpTransport := &http.Transport{Dial: dialer.Dial}
	client := &http.Client{Timeout: 8 * time.Second, Transport: httpTransport}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", "", fmt.Errorf("ip probe status %d", resp.StatusCode)
	}
	body := make([]byte, 128)
	n, _ := resp.Body.Read(body)
	exitIP := strings.TrimSpace(string(body[:n]))
	if exitIP == "" {
		return "", "", "", errors.New("empty exit ip")
	}
	country, region, err := lookupCountryRegion(exitIP)
	if err != nil {
		country = "xx"
		region = ""
	}
	return exitIP, country, region, nil
}

func lookupCountryCode(ip string) (string, error) {
	country, _, err := lookupCountryRegion(ip)
	return country, err
}

func lookupCountryRegion(ip string) (string, string, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", "", errors.New("ip is empty")
	}
	client := &http.Client{Timeout: 6 * time.Second}
	type provider func(*http.Client, string) (string, string, error)
	providers := []provider{lookupViaIPAPICo, lookupViaIPWhoIs, lookupViaIPInfo}
	var lastErr error
	for _, fn := range providers {
		country, region, err := fn(client, ip)
		if err != nil {
			lastErr = err
			continue
		}
		country = strings.ToLower(strings.TrimSpace(country))
		if len(country) == 2 {
			return country, strings.TrimSpace(region), nil
		}
		lastErr = errors.New("country code unavailable")
	}
	if lastErr == nil {
		lastErr = errors.New("country code unavailable")
	}
	return "", "", lastErr
}

func lookupViaIPAPICo(client *http.Client, ip string) (string, string, error) {
	url := fmt.Sprintf("https://ipapi.co/%s/json/", ip)
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("ipapi.co status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var payload struct {
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		Error       bool   `json:"error"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	if payload.Error {
		if strings.TrimSpace(payload.Reason) != "" {
			return "", "", errors.New(strings.TrimSpace(payload.Reason))
		}
		return "", "", errors.New("ipapi.co returned error")
	}
	return payload.CountryCode, payload.Region, nil
}

func lookupViaIPWhoIs(client *http.Client, ip string) (string, string, error) {
	url := fmt.Sprintf("https://ipwho.is/%s", ip)
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("ipwho.is status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var payload struct {
		Success     bool   `json:"success"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		Message     string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	if !payload.Success {
		if strings.TrimSpace(payload.Message) != "" {
			return "", "", errors.New(strings.TrimSpace(payload.Message))
		}
		return "", "", errors.New("ipwho.is returned unsuccessful")
	}
	return payload.CountryCode, payload.Region, nil
}

func lookupViaIPInfo(client *http.Client, ip string) (string, string, error) {
	url := fmt.Sprintf("https://ipinfo.io/%s/json", ip)
	resp, err := client.Get(url)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("ipinfo.io status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	var payload struct {
		Country string `json:"country"`
		Region  string `json:"region"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(payload.Country) == "" {
		return "", "", errors.New("ipinfo country is empty")
	}
	return payload.Country, payload.Region, nil
}

func sortForwardRowsByRouteUser(rows []model.SocksOutbound) {
	sort.Slice(rows, func(i, j int) bool {
		left := strings.TrimSpace(rows[i].RouteUser)
		right := strings.TrimSpace(rows[j].RouteUser)
		if left == right {
			return rows[i].ID < rows[j].ID
		}
		return left < right
	})
}
