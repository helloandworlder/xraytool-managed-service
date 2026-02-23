package model

import "time"

const (
	OrderStatusActive   = "active"
	OrderStatusExpired  = "expired"
	OrderStatusDisabled = "disabled"

	OrderItemStatusActive   = "active"
	OrderItemStatusExpired  = "expired"
	OrderItemStatusDisabled = "disabled"

	OrderModeAuto    = "auto"
	OrderModeManual  = "manual"
	OrderModeImport  = "import"
	OrderModeForward = "forward"

	OutboundTypeDirect = "direct"
	OutboundTypeSocks5 = "socks5"
)

type Admin struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Customer struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;not null;uniqueIndex" json:"name"`
	Code      string    `gorm:"size:64;index" json:"code"`
	Contact   string    `gorm:"size:255" json:"contact"`
	Notes     string    `gorm:"size:1024" json:"notes"`
	Status    string    `gorm:"size:32;default:active" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Orders []Order `json:"orders,omitempty"`
}

type HostIP struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	IP        string    `gorm:"size:64;uniqueIndex;not null" json:"ip"`
	IsPublic  bool      `gorm:"default:false" json:"is_public"`
	IsLocal   bool      `gorm:"default:true" json:"is_local"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Comment   string    `gorm:"size:255" json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	OrderItems []OrderItem `json:"order_items,omitempty"`
}

type XrayNode struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:128;uniqueIndex;not null" json:"name"`
	BaseURL   string    `gorm:"size:255;not null" json:"base_url"`
	Username  string    `gorm:"size:64;not null" json:"username"`
	Password  string    `gorm:"size:128;not null" json:"password"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	IsLocal   bool      `gorm:"default:false" json:"is_local"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SocksOutbound struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"size:128" json:"name"`
	Address      string     `gorm:"size:128;not null;index" json:"address"`
	Port         int        `gorm:"not null" json:"port"`
	Username     string     `gorm:"size:128;not null" json:"username"`
	Password     string     `gorm:"size:128;not null" json:"password"`
	RouteUser    string     `gorm:"size:128;index" json:"route_user"`
	ExitIP       string     `gorm:"size:64" json:"exit_ip"`
	CountryCode  string     `gorm:"size:8;index" json:"country_code"`
	Enabled      bool       `gorm:"default:true;index" json:"enabled"`
	ProbeStatus  string     `gorm:"size:32" json:"probe_status"`
	ProbeError   string     `gorm:"size:255" json:"probe_error"`
	LastProbedAt *time.Time `json:"last_probed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	OrderItems []OrderItem `json:"order_items,omitempty"`
}

type Order struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	CustomerID        uint      `gorm:"index;not null" json:"customer_id"`
	Name              string    `gorm:"size:128;not null" json:"name"`
	Mode              string    `gorm:"size:32;not null" json:"mode"`
	Status            string    `gorm:"size:32;not null;index" json:"status"`
	Quantity          int       `gorm:"not null" json:"quantity"`
	Port              int       `gorm:"not null;index" json:"port"`
	StartsAt          time.Time `gorm:"not null" json:"starts_at"`
	ExpiresAt         time.Time `gorm:"not null;index" json:"expires_at"`
	NotifyOneDaySent  bool      `gorm:"default:false" json:"notify_one_day_sent"`
	NotifyExpiredSent bool      `gorm:"default:false" json:"notify_expired_sent"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	Customer Customer    `json:"customer"`
	Items    []OrderItem `json:"items"`
}

type OrderItem struct {
	ID              uint  `gorm:"primaryKey" json:"id"`
	OrderID         uint  `gorm:"index;not null" json:"order_id"`
	HostIPID        *uint `gorm:"index" json:"host_ip_id,omitempty"`
	SocksOutboundID *uint `gorm:"index" json:"socks_outbound_id,omitempty"`

	IP              string `gorm:"size:64;not null;index" json:"ip"`
	Port            int    `gorm:"not null;index" json:"port"`
	Username        string `gorm:"size:64;not null;uniqueIndex:idx_order_items_auth" json:"username"`
	Password        string `gorm:"size:64;not null" json:"password"`
	OutboundType    string `gorm:"size:16;not null;default:direct;index" json:"outbound_type"`
	ForwardAddress  string `gorm:"size:128" json:"forward_address,omitempty"`
	ForwardPort     int    `json:"forward_port,omitempty"`
	ForwardUsername string `gorm:"size:128" json:"forward_username,omitempty"`
	ForwardPassword string `gorm:"size:128" json:"forward_password,omitempty"`
	Managed         bool   `gorm:"default:true" json:"managed"`
	Status          string `gorm:"size:32;not null;index" json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	HostIP        *HostIP        `json:"host_ip,omitempty"`
	SocksOutbound *SocksOutbound `json:"socks_outbound,omitempty"`
	Order         Order          `json:"-"`
	Resources     []XrayResource `json:"resources,omitempty"`
}

type XrayResource struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	OrderItemID uint      `gorm:"index;uniqueIndex" json:"order_item_id"`
	InboundTag  string    `gorm:"size:128;index" json:"inbound_tag"`
	OutboundTag string    `gorm:"size:128;uniqueIndex" json:"outbound_tag"`
	RuleTag     string    `gorm:"size:128;uniqueIndex" json:"rule_tag"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	OrderItem OrderItem `json:"-"`
}

type Setting struct {
	Key       string    `gorm:"primaryKey;size:128" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TaskLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Level     string    `gorm:"size:16;index" json:"level"`
	Message   string    `gorm:"size:255" json:"message"`
	Detail    string    `gorm:"type:text" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
