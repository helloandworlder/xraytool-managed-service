package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"xraytool/internal/auth"
	"xraytool/internal/config"
	"xraytool/internal/model"
	"xraytool/internal/service"
	"xraytool/internal/store"
)

type API struct {
	db        *gorm.DB
	store     *store.Store
	orders    *service.OrderService
	singbox   *service.SingboxImportService
	nodes     *service.NodeService
	forward   *service.ForwardOutboundService
	dedicated *service.DedicatedEntryService
	hostIPs   *service.HostIPService
	backups   *service.BackupService
	bark      *service.BarkService
	runtime   *service.RuntimeStatsService
	cfg       config.Config
	logger    *zap.Logger
}

func New(db *gorm.DB, st *store.Store, orders *service.OrderService, singbox *service.SingboxImportService, nodes *service.NodeService, forward *service.ForwardOutboundService, hostIPs *service.HostIPService, backups *service.BackupService, bark *service.BarkService, runtime *service.RuntimeStatsService, cfg config.Config, logger *zap.Logger) *API {
	return &API{db: db, store: st, orders: orders, singbox: singbox, nodes: nodes, forward: forward, dedicated: service.NewDedicatedEntryService(db), hostIPs: hostIPs, backups: backups, bark: bark, runtime: runtime, cfg: cfg, logger: logger}
}

func (a *API) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), a.logMiddleware(), corsMiddleware())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "ts": time.Now().Unix()})
	})

	r.GET("/", func(c *gin.Context) {
		content, err := os.ReadFile(filepath.Join("web", "dist", "index.html"))
		if err != nil {
			c.String(http.StatusNotFound, "web ui not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})
	r.Static("/assets", filepath.Join("web", "dist", "assets"))

	api := r.Group("/api")
	api.POST("/auth/login", a.handleLogin)

	secure := api.Group("/")
	secure.Use(a.authMiddleware())
	secure.GET("/auth/me", a.handleMe)
	secure.POST("/auth/reset-password", a.handleResetPassword)

	secure.GET("/customers", a.listCustomers)
	secure.POST("/customers", a.createCustomer)
	secure.PUT("/customers/:id", a.updateCustomer)
	secure.DELETE("/customers/:id", a.deleteCustomer)

	secure.GET("/host-ips", a.listHostIPs)
	secure.POST("/host-ips/scan", a.scanHostIPs)
	secure.POST("/host-ips/probe", a.probeHostPort)
	secure.POST("/host-ips/:id/toggle", a.toggleHostIP)
	secure.GET("/oversell", a.oversellView)

	secure.GET("/nodes", a.listNodes)
	secure.POST("/nodes", a.createNode)
	secure.PUT("/nodes/:id", a.updateNode)
	secure.DELETE("/nodes/:id", a.deleteNode)
	secure.POST("/migrations/singbox/scan", a.scanSingboxConfigs)
	secure.POST("/migrations/singbox/preview", a.previewSingboxImport)
	secure.POST("/migrations/socks5/preview", a.previewSocksMigration)
	secure.GET("/forward-outbounds", a.listForwardOutbounds)
	secure.POST("/forward-outbounds", a.createForwardOutbound)
	secure.PUT("/forward-outbounds/:id", a.updateForwardOutbound)
	secure.DELETE("/forward-outbounds/:id", a.deleteForwardOutbound)
	secure.POST("/forward-outbounds/:id/toggle", a.toggleForwardOutbound)
	secure.POST("/forward-outbounds/import", a.importForwardOutbounds)
	secure.POST("/forward-outbounds/:id/probe", a.probeForwardOutbound)
	secure.POST("/forward-outbounds/probe-all", a.probeAllForwardOutbounds)
	secure.GET("/orders/forward-outbounds", a.listForwardOutbounds)
	secure.POST("/orders/forward-outbounds", a.createForwardOutbound)
	secure.PUT("/orders/forward-outbounds/:id", a.updateForwardOutbound)
	secure.DELETE("/orders/forward-outbounds/:id", a.deleteForwardOutbound)
	secure.POST("/orders/forward-outbounds/:id/toggle", a.toggleForwardOutbound)
	secure.POST("/orders/forward-outbounds/import", a.importForwardOutbounds)
	secure.POST("/orders/forward-outbounds/:id/probe", a.probeForwardOutbound)
	secure.POST("/orders/forward-outbounds/probe-all", a.probeAllForwardOutbounds)
	secure.GET("/orders/dedicated-entries", a.listDedicatedEntries)
	secure.POST("/orders/dedicated-entries", a.createDedicatedEntry)
	secure.PUT("/orders/dedicated-entries/:id", a.updateDedicatedEntry)
	secure.DELETE("/orders/dedicated-entries/:id", a.deleteDedicatedEntry)
	secure.POST("/orders/dedicated-entries/:id/toggle", a.toggleDedicatedEntry)

	secure.GET("/orders", a.listOrders)
	secure.POST("/orders/forward/reuse-warnings", a.forwardReuseWarnings)
	secure.GET("/orders/allocation/preview", a.orderAllocationPreview)
	secure.GET("/orders/:id", a.getOrder)
	secure.POST("/orders", a.createOrder)
	secure.PUT("/orders/:id", a.updateOrder)
	secure.POST("/orders/:id/split", a.splitOrder)
	secure.POST("/orders/:id/group/update-socks5", a.updateOrderGroupSocks5)
	secure.POST("/orders/:id/group/update-credentials", a.updateOrderGroupCredentials)
	secure.POST("/orders/:id/deactivate", a.deactivateOrder)
	secure.POST("/orders/:id/renew", a.renewOrder)
	secure.POST("/orders/batch/deactivate", a.batchDeactivateOrders)
	secure.POST("/orders/batch/renew", a.batchRenewOrders)
	secure.POST("/orders/batch/resync", a.batchResyncOrders)
	secure.POST("/orders/batch/test", a.batchTestOrders)
	secure.POST("/orders/batch/export", a.batchExportOrders)
	secure.GET("/orders/:id/export", a.exportOrder)
	secure.POST("/orders/:id/test", a.testOrder)
	secure.POST("/orders/:id/test/stream", a.testOrderStream)
	secure.POST("/orders/import/preview", a.previewImport)
	secure.POST("/orders/import/confirm", a.confirmImport)

	secure.GET("/settings", a.getSettings)
	secure.PUT("/settings", a.updateSettings)
	secure.POST("/settings/bark/test", a.testBark)
	secure.GET("/runtime/customers", a.customerRuntimeStats)
	secure.GET("/db/backups", a.listBackups)
	secure.POST("/db/backups", a.createBackup)
	secure.GET("/db/backup/export", a.exportBackup)
	secure.GET("/db/backups/:name/download", a.downloadBackup)
	secure.DELETE("/db/backups/:name", a.deleteBackup)
	secure.POST("/db/restore", a.restoreBackup)
	secure.GET("/task-logs", a.taskLogs)

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		content, err := os.ReadFile(filepath.Join("web", "dist", "index.html"))
		if err != nil {
			c.String(http.StatusNotFound, "web ui not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	})

	return r
}

func (a *API) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var admin model.Admin
	if err := a.db.First(&admin, "username = ?", strings.TrimSpace(req.Username)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !auth.VerifyPassword(admin.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := auth.GenerateToken(a.cfg.JWTSecret, admin.ID, admin.Username, 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "username": admin.Username})
}

func (a *API) handleMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"admin_id": c.GetUint("admin_id"),
		"username": c.GetString("username"),
	})
}

func (a *API) handleResetPassword(c *gin.Context) {
	var req struct {
		Username    string `json:"username"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Username == "" || len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password(>=8) required"})
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := a.store.ResetAdminPassword(req.Username, hash); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) listCustomers(c *gin.Context) {
	var rows []model.Customer
	if err := a.db.Order("id desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) createCustomer(c *gin.Context) {
	var row model.Customer
	if err := c.ShouldBindJSON(&row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row.Name = strings.TrimSpace(row.Name)
	row.Code = strings.TrimSpace(row.Code)
	if row.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if row.Code != "" {
		var cnt int64
		if err := a.db.Model(&model.Customer{}).Where("code = ?", row.Code).Count(&cnt).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if cnt > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer code already exists"})
			return
		}
	}
	if row.Status == "" {
		row.Status = "active"
	}
	if err := a.db.Create(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) updateCustomer(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req model.Customer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{
		"name":       strings.TrimSpace(req.Name),
		"code":       strings.TrimSpace(req.Code),
		"contact":    req.Contact,
		"notes":      req.Notes,
		"status":     req.Status,
		"updated_at": time.Now(),
	}
	if code, ok := updates["code"].(string); ok && code != "" {
		var cnt int64
		if err := a.db.Model(&model.Customer{}).Where("code = ? and id <> ?", code, id).Count(&cnt).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if cnt > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer code already exists"})
			return
		}
	}
	if err := a.db.Model(&model.Customer{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) deleteCustomer(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var orderCount int64
	if err := a.db.Model(&model.Order{}).Where("customer_id = ?", id).Count(&orderCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if orderCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer has orders, cannot delete"})
		return
	}
	if err := a.db.Delete(&model.Customer{}, id).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) listNodes(c *gin.Context) {
	rows, err := a.nodes.ListNodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) createNode(c *gin.Context) {
	var req service.NodeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := a.nodes.CreateNode(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) updateNode(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req service.NodeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := a.nodes.UpdateNode(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) deleteNode(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := a.nodes.DeleteNode(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) scanSingboxConfigs(c *gin.Context) {
	if a.singbox == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sing-box import service unavailable"})
		return
	}
	result, err := a.singbox.Scan()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *API) previewSingboxImport(c *gin.Context) {
	if a.singbox == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "sing-box import service unavailable"})
		return
	}
	var req struct {
		Files []string `json:"files"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	rows, err := a.singbox.Preview(req.Files)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) previewSocksMigration(c *gin.Context) {
	var req struct {
		Lines string `json:"lines"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := a.nodes.PreviewSocksMigration(c.Request.Context(), req.Lines)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (a *API) listForwardOutbounds(c *gin.Context) {
	rows, err := a.forward.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) createForwardOutbound(c *gin.Context) {
	var req service.ForwardOutboundInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := a.forward.Create(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) updateForwardOutbound(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req service.ForwardOutboundInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := a.forward.Update(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) toggleForwardOutbound(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.forward.Toggle(id, req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) deleteForwardOutbound(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := a.forward.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) importForwardOutbounds(c *gin.Context) {
	var req struct {
		Lines string `json:"lines"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := a.forward.ImportLines(req.Lines)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) probeForwardOutbound(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	row, err := a.forward.Probe(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) probeAllForwardOutbounds(c *gin.Context) {
	var req struct {
		EnabledOnly *bool `json:"enabled_only"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	enabledOnly := true
	if req.EnabledOnly != nil {
		enabledOnly = *req.EnabledOnly
	}
	rows, err := a.forward.ProbeAll(enabledOnly)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) forwardReuseWarnings(c *gin.Context) {
	var req struct {
		CustomerID         uint   `json:"customer_id"`
		ForwardOutboundIDs []uint `json:"forward_outbound_ids"`
		ExcludeOrderID     uint   `json:"exclude_order_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	warnings, err := a.orders.ForwardOutboundReuseWarnings(req.CustomerID, req.ExcludeOrderID, req.ForwardOutboundIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"warnings": warnings})
}

func (a *API) listDedicatedEntries(c *gin.Context) {
	rows, err := a.dedicated.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) createDedicatedEntry(c *gin.Context) {
	var req service.DedicatedEntryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := a.dedicated.Create(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) updateDedicatedEntry(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req service.DedicatedEntryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := a.dedicated.Update(id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (a *API) toggleDedicatedEntry(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.dedicated.Toggle(id, req.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) deleteDedicatedEntry(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := a.dedicated.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) listHostIPs(c *gin.Context) {
	rows, err := a.hostIPs.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) scanHostIPs(c *gin.Context) {
	rows, err := a.hostIPs.ScanAndSync()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) probeHostPort(c *gin.Context) {
	var req struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	occupied, err := service.ProbePort(req.IP, req.Port)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"occupied": occupied, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"occupied": occupied})
}

func (a *API) toggleHostIP(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.db.Model(&model.HostIP{}).Where("id = ?", id).Update("enabled", req.Enabled).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) oversellView(c *gin.Context) {
	type ipRow struct {
		IP       string `json:"ip"`
		Enabled  bool   `json:"enabled"`
		IsPublic bool   `json:"is_public"`
		IsLocal  bool   `json:"is_local"`
	}
	hostRows := []ipRow{}
	if err := a.db.Table("host_ips").Select("ip, enabled, is_public, is_local").Order("ip asc").Scan(&hostRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	customerID, _ := strconv.ParseUint(strings.TrimSpace(c.DefaultQuery("customer_id", "0")), 10, 64)

	type countRow struct {
		IP    string
		Count int64
	}
	totalCountMap := map[string]int64{}
	customerCountMap := map[string]int64{}

	totalRows := []countRow{}
	if err := a.db.Table("order_items oi").
		Select("oi.ip as ip, count(1) as count").
		Joins("join orders o on o.id = oi.order_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Group("oi.ip").Scan(&totalRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, row := range totalRows {
		totalCountMap[row.IP] = row.Count
	}

	if customerID > 0 {
		customerRows := []countRow{}
		if err := a.db.Table("order_items oi").
			Select("oi.ip as ip, count(1) as count").
			Joins("join orders o on o.id = oi.order_id").
			Where("o.customer_id = ? and o.status = ? and o.expires_at > ? and oi.status = ?", uint(customerID), model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
			Group("oi.ip").Scan(&customerRows).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, row := range customerRows {
			customerCountMap[row.IP] = row.Count
		}
	}

	customerCountRows := []struct {
		IP    string
		Count int64
	}{}
	if err := a.db.Table("order_items oi").
		Select("oi.ip as ip, count(distinct o.customer_id) as count").
		Joins("join orders o on o.id = oi.order_id").
		Where("o.status = ? and o.expires_at > ? and oi.status = ?", model.OrderStatusActive, time.Now(), model.OrderItemStatusActive).
		Group("oi.ip").Scan(&customerCountRows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	uniqueCustomerMap := map[string]int64{}
	for _, row := range customerCountRows {
		uniqueCustomerMap[row.IP] = row.Count
	}

	rows := make([]gin.H, 0, len(hostRows))
	for _, h := range hostRows {
		total := totalCountMap[h.IP]
		oversold := int64(0)
		if total > 1 {
			oversold = total - 1
		}
		rate := 0.0
		if total > 0 {
			rate = float64(oversold) * 100 / float64(total)
		}
		rows = append(rows, gin.H{
			"ip":                    h.IP,
			"count":                 total,
			"total_active_count":    total,
			"customer_active_count": customerCountMap[h.IP],
			"unique_customer_count": uniqueCustomerMap[h.IP],
			"oversold_count":        oversold,
			"oversell_rate":         rate,
			"enabled":               h.Enabled,
			"is_public":             h.IsPublic,
			"is_local":              h.IsLocal,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]["count"].(int64)
		right := rows[j]["count"].(int64)
		if left == right {
			return rows[i]["ip"].(string) < rows[j]["ip"].(string)
		}
		return left > right
	})

	c.JSON(http.StatusOK, gin.H{
		"rows":        rows,
		"customer_id": customerID,
		"total_ips":   len(hostRows),
	})
}

func (a *API) listOrders(c *gin.Context) {
	rows, err := a.orders.ListOrders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) orderAllocationPreview(c *gin.Context) {
	customerID, err := strconv.ParseUint(strings.TrimSpace(c.Query("customer_id")), 10, 64)
	if err != nil || customerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id is required"})
		return
	}
	excludeID, _ := strconv.ParseUint(strings.TrimSpace(c.DefaultQuery("exclude_order_id", "0")), 10, 64)
	preview, err := a.orders.AllocationPreview(uint(customerID), uint(excludeID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (a *API) getOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	order, err := a.orders.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (a *API) createOrder(c *gin.Context) {
	var req struct {
		CustomerID           uint   `json:"customer_id"`
		Name                 string `json:"name"`
		Quantity             int    `json:"quantity"`
		DurationDay          int    `json:"duration_day"`
		ExpiresAt            string `json:"expires_at"`
		Mode                 string `json:"mode"`
		Port                 int    `json:"port"`
		ManualIPIDs          []uint `json:"manual_ip_ids"`
		ForwardOutboundIDs   []uint `json:"forward_outbound_ids"`
		DedicatedEntryID     uint   `json:"dedicated_entry_id"`
		DedicatedEgressLines string `json:"dedicated_egress_lines"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input := service.CreateOrderInput{
		CustomerID:           req.CustomerID,
		Name:                 req.Name,
		Quantity:             req.Quantity,
		DurationDay:          req.DurationDay,
		Mode:                 req.Mode,
		Port:                 req.Port,
		ManualIPIDs:          req.ManualIPIDs,
		ForwardOutboundIDs:   req.ForwardOutboundIDs,
		DedicatedEntryID:     req.DedicatedEntryID,
		DedicatedEgressLines: req.DedicatedEgressLines,
	}
	if strings.TrimSpace(req.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at, expect RFC3339"})
			return
		}
		input.ExpiresAt = t
	}
	warnings := []string{}
	if strings.EqualFold(strings.TrimSpace(req.Mode), model.OrderModeForward) {
		warnRows, warnErr := a.orders.ForwardOutboundReuseWarnings(req.CustomerID, 0, req.ForwardOutboundIDs)
		if warnErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": warnErr.Error()})
			return
		}
		warnings = warnRows
	}
	order, err := a.orders.CreateOrder(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order, "warnings": warnings})
}

func (a *API) updateOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name                           string `json:"name"`
		Quantity                       int    `json:"quantity"`
		Port                           int    `json:"port"`
		ExpiresAt                      string `json:"expires_at"`
		ManualIPIDs                    []uint `json:"manual_ip_ids"`
		ForwardOutboundIDs             []uint `json:"forward_outbound_ids"`
		DedicatedEntryID               uint   `json:"dedicated_entry_id"`
		DedicatedEgressLines           string `json:"dedicated_egress_lines"`
		DedicatedCredentialLines       string `json:"dedicated_credential_lines"`
		RegenerateDedicatedCredentials bool   `json:"regenerate_dedicated_credentials"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input := service.UpdateOrderInput{
		Name:                           req.Name,
		Quantity:                       req.Quantity,
		Port:                           req.Port,
		ManualIPIDs:                    req.ManualIPIDs,
		ForwardOutboundIDs:             req.ForwardOutboundIDs,
		DedicatedEntryID:               req.DedicatedEntryID,
		DedicatedEgressLines:           req.DedicatedEgressLines,
		DedicatedCredentialLines:       req.DedicatedCredentialLines,
		RegenerateDedicatedCredentials: req.RegenerateDedicatedCredentials,
	}
	if strings.TrimSpace(req.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid expires_at, expect RFC3339"})
			return
		}
		input.ExpiresAt = t
	}
	warnings := []string{}
	current, err := a.orders.GetOrder(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.EqualFold(strings.TrimSpace(current.Mode), model.OrderModeForward) && len(req.ForwardOutboundIDs) > 0 {
		warnings, err = a.orders.ForwardOutboundReuseWarnings(current.CustomerID, id, req.ForwardOutboundIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	order, err := a.orders.UpdateOrder(c.Request.Context(), id, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order, "warnings": warnings})
}

func (a *API) splitOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	rows, err := a.orders.SplitOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"children": rows})
}

func (a *API) updateOrderGroupSocks5(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Lines string `json:"lines"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.orders.UpdateGroupSocks5(c.Request.Context(), id, req.Lines); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) updateOrderGroupCredentials(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Lines      string `json:"lines"`
		Regenerate bool   `json:"regenerate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.orders.UpdateGroupCredentials(c.Request.Context(), id, req.Lines, req.Regenerate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) deactivateOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := a.orders.DeactivateOrder(c.Request.Context(), id, model.OrderStatusDisabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) renewOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		MoreDays int `json:"more_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.orders.RenewOrder(c.Request.Context(), id, req.MoreDays); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) batchDeactivateOrders(c *gin.Context) {
	var req struct {
		OrderIDs []uint `json:"order_ids"`
		Status   string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_ids is empty"})
		return
	}
	if req.Status == "" {
		req.Status = model.OrderStatusDisabled
	}
	results := a.orders.BatchDeactivate(c.Request.Context(), req.OrderIDs, req.Status)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (a *API) batchRenewOrders(c *gin.Context) {
	var req struct {
		OrderIDs []uint `json:"order_ids"`
		MoreDays int    `json:"more_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_ids is empty"})
		return
	}
	if req.MoreDays <= 0 {
		req.MoreDays = 30
	}
	results := a.orders.BatchRenew(c.Request.Context(), req.OrderIDs, req.MoreDays)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (a *API) batchResyncOrders(c *gin.Context) {
	var req struct {
		OrderIDs []uint `json:"order_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_ids is empty"})
		return
	}
	results := a.orders.BatchResync(c.Request.Context(), req.OrderIDs)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (a *API) batchTestOrders(c *gin.Context) {
	var req struct {
		OrderIDs []uint `json:"order_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_ids is empty"})
		return
	}
	results := a.orders.BatchTest(req.OrderIDs)
	c.JSON(http.StatusOK, gin.H{"results": results})
}

func (a *API) batchExportOrders(c *gin.Context) {
	var req struct {
		OrderIDs []uint `json:"order_ids"`
		Format   string `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_ids is empty"})
		return
	}
	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "txt"
	}
	if format == "xlsx" {
		data, filename, err := a.orders.BatchExportXLSX(req.OrderIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
		return
	}
	text, err := a.orders.BatchExport(req.OrderIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=batch-orders-export.txt")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(text))
}

func (a *API) exportOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	count, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("count", "0")))
	shuffle := strings.ToLower(strings.TrimSpace(c.DefaultQuery("shuffle", "true"))) != "false"
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "txt")))
	if format == "xlsx" {
		data, filename, err := a.orders.ExportOrderXLSX(id, service.ExportOrderOptions{Count: count, Shuffle: shuffle})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
		return
	}
	text, filename, err := a.orders.ExportOrderLinesWithMeta(id, service.ExportOrderOptions{Count: count, Shuffle: shuffle})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(text))
}

func (a *API) testOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		SamplePercent int `json:"sample_percent"`
	}
	_ = c.ShouldBindJSON(&req)
	out, err := a.orders.TestOrderSampled(id, req.SamplePercent)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) testOrderStream(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		SamplePercent int `json:"sample_percent"`
	}
	_ = c.ShouldBindJSON(&req)

	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	if err := a.orders.TestOrderStream(id, req.SamplePercent, func(event service.TestOrderStreamEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := c.Writer.Write(append(payload, '\n')); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}); err != nil {
		errPayload, _ := json.Marshal(service.TestOrderStreamEvent{Type: "error", Error: err.Error()})
		_, _ = c.Writer.Write(append(errPayload, '\n'))
		c.Writer.Flush()
	}
}

func (a *API) previewImport(c *gin.Context) {
	var req struct {
		Lines string `json:"lines"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rows, err := a.orders.PreviewImport(req.Lines)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) confirmImport(c *gin.Context) {
	var req struct {
		CustomerID uint                       `json:"customer_id"`
		OrderName  string                     `json:"order_name"`
		ExpiresAt  string                     `json:"expires_at"`
		Rows       []service.ImportPreviewRow `json:"rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	customerID := req.CustomerID
	if customerID == 0 {
		id, err := a.ensureUnassignedCustomer()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		customerID = id
	}
	var exp time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err == nil {
			exp = t
		}
	}
	order, err := a.orders.ImportOrder(c.Request.Context(), customerID, req.OrderName, exp, req.Rows)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
}

func (a *API) ensureUnassignedCustomer() (uint, error) {
	const customerName = "未分配客户"
	const customerCode = "UNASSIGNED"

	row := model.Customer{}
	err := a.db.First(&row, "name = ?", customerName).Error
	if err == nil {
		if row.Status != model.OrderStatusActive {
			_ = a.db.Model(&model.Customer{}).Where("id = ?", row.ID).Update("status", model.OrderStatusActive).Error
		}
		return row.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	create := model.Customer{
		Name:    customerName,
		Code:    customerCode,
		Contact: "",
		Notes:   "系统自动创建: 导入未指定客户",
		Status:  model.OrderStatusActive,
	}
	if createErr := a.db.Create(&create).Error; createErr == nil {
		return create.ID, nil
	}

	if err := a.db.First(&row, "name = ?", customerName).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

func (a *API) getSettings(c *gin.Context) {
	settings, err := a.store.GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (a *API) updateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	clean := sanitizeSettingsUpdate(req)
	if len(clean) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no valid settings provided"})
		return
	}
	if err := a.store.SetSettings(clean); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) testBark(c *gin.Context) {
	title := "XrayTool Bark 测试通知"
	body := fmt.Sprintf("测试时间: %s", time.Now().Format("2006-01-02 15:04:05"))
	if err := a.bark.Notify(title, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "test notification sent"})
}

func (a *API) customerRuntimeStats(c *gin.Context) {
	rows, err := a.runtime.Snapshot(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) taskLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := a.db.Model(&model.TaskLog{})
	if level := strings.TrimSpace(c.Query("level")); level != "" {
		query = query.Where("level = ?", level)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("message like ? or detail like ?", like, like)
	}
	if start := strings.TrimSpace(c.Query("start")); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if end := strings.TrimSpace(c.Query("end")); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}

	var rows []model.TaskLog
	if err := query.Order("id desc").Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) listBackups(c *gin.Context) {
	rows, err := a.backups.ListBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) createBackup(c *gin.Context) {
	info, err := a.backups.CreateBackup()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.store.AddTaskLog("info", "db backup created", info.Name)
	c.JSON(http.StatusOK, info)
}

func (a *API) exportBackup(c *gin.Context) {
	path, downloadName, err := a.backups.CreateTempExport()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", downloadName))
	c.File(path)
	go func() {
		time.Sleep(10 * time.Second)
		_ = os.Remove(path)
	}()
}

func (a *API) downloadBackup(c *gin.Context) {
	name := c.Param("name")
	path, err := a.backups.BackupPath(name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(path)))
	c.File(path)
}

func (a *API) deleteBackup(c *gin.Context) {
	name := c.Param("name")
	if err := a.backups.DeleteBackup(name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.store.AddTaskLog("warn", "db backup deleted", name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *API) restoreBackup(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if err := a.backups.RestoreBackup(req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.store.AddTaskLog("warn", "db restored from backup", req.Name)
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "restore completed, service will restart"})
	go func() {
		time.Sleep(700 * time.Millisecond)
		os.Exit(0)
	}()
}

func (a *API) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := auth.ParseToken(a.cfg.JWTSecret, parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("admin_id", claims.AdminID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

func (a *API) logMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		a.logger.Info("http request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
		)
	}
}

func sanitizeSettingsUpdate(in map[string]string) map[string]string {
	out := map[string]string{}
	allowed := map[string]struct{}{
		"default_inbound_port": {},
		"bark_enabled":         {},
		"bark_base_url":        {},
		"bark_device_key":      {},
		"bark_group":           {},
	}
	for k, v := range in {
		k = strings.TrimSpace(k)
		if _, ok := allowed[k]; !ok {
			continue
		}
		if k == "bark_enabled" {
			vv := strings.ToLower(strings.TrimSpace(v))
			switch vv {
			case "1", "true", "on", "yes":
				out[k] = "true"
			default:
				out[k] = "false"
			}
			continue
		}
		out[k] = strings.TrimSpace(v)
	}
	return out
}

func parseUintParam(c *gin.Context, key string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(v), true
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
