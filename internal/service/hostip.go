package service

import (
	"net"
	"strconv"
	"strings"
	"time"

	"xraytool/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type HostIPService struct {
	db *gorm.DB
}

func NewHostIPService(db *gorm.DB) *HostIPService {
	return &HostIPService{db: db}
}

func (s *HostIPService) ScanAndSync() ([]model.HostIP, error) {
	ipMap := map[string]model.HostIP{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip := extractIPv4(a)
			if ip == nil {
				continue
			}
			addr := ip.String()
			if addr == "" {
				continue
			}
			isPublic := isPublicIPv4(ip)
			ipMap[addr] = model.HostIP{
				IP:       addr,
				IsPublic: isPublic,
				IsLocal:  true,
				Enabled:  true,
			}
		}
	}

	if len(ipMap) == 0 {
		return []model.HostIP{}, nil
	}

	now := time.Now()
	for _, row := range ipMap {
		row.CreatedAt = now
		row.UpdatedAt = now
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "ip"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"is_public":  row.IsPublic,
				"is_local":   row.IsLocal,
				"updated_at": row.UpdatedAt,
			}),
		}).Create(&row).Error; err != nil {
			return nil, err
		}
	}

	var rows []model.HostIP
	if err := s.db.Order("ip asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *HostIPService) List() ([]model.HostIP, error) {
	var rows []model.HostIP
	err := s.db.Order("ip asc").Find(&rows).Error
	return rows, err
}

func ProbePort(ip string, port int) (occupied bool, err error) {
	addr := net.JoinHostPort(ip, intToString(port))
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		_ = ln.Close()
		return false, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "address already in use") {
		return true, nil
	}
	conn, dialErr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
	if dialErr == nil {
		_ = conn.Close()
		return true, nil
	}
	return true, err
}

func extractIPv4(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPNet:
		return v.IP.To4()
	case *net.IPAddr:
		return v.IP.To4()
	default:
		return nil
	}
}

func isPublicIPv4(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
		return false
	}
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"100.64.0.0/10",
	}
	for _, cidr := range privateCIDRs {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func intToString(v int) string {
	return strconv.Itoa(v)
}
