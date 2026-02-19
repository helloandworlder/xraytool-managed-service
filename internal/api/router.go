package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
	db      *gorm.DB
	store   *store.Store
	orders  *service.OrderService
	hostIPs *service.HostIPService
	backups *service.BackupService
	cfg     config.Config
	logger  *zap.Logger
}

func New(db *gorm.DB, st *store.Store, orders *service.OrderService, hostIPs *service.HostIPService, backups *service.BackupService, cfg config.Config, logger *zap.Logger) *API {
	return &API{db: db, store: st, orders: orders, hostIPs: hostIPs, backups: backups, cfg: cfg, logger: logger}
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

	secure.GET("/orders", a.listOrders)
	secure.GET("/orders/:id", a.getOrder)
	secure.POST("/orders", a.createOrder)
	secure.POST("/orders/:id/deactivate", a.deactivateOrder)
	secure.POST("/orders/:id/renew", a.renewOrder)
	secure.POST("/orders/batch/deactivate", a.batchDeactivateOrders)
	secure.POST("/orders/batch/renew", a.batchRenewOrders)
	secure.POST("/orders/batch/resync", a.batchResyncOrders)
	secure.POST("/orders/batch/test", a.batchTestOrders)
	secure.POST("/orders/batch/export", a.batchExportOrders)
	secure.GET("/orders/:id/export", a.exportOrder)
	secure.POST("/orders/:id/test", a.testOrder)
	secure.POST("/orders/import/preview", a.previewImport)
	secure.POST("/orders/import/confirm", a.confirmImport)

	secure.GET("/settings", a.getSettings)
	secure.PUT("/settings", a.updateSettings)
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
	if strings.TrimSpace(row.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
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
		"name":       req.Name,
		"contact":    req.Contact,
		"notes":      req.Notes,
		"status":     req.Status,
		"updated_at": time.Now(),
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
	rows := []struct {
		IP      string `json:"ip"`
		Count   int64  `json:"count"`
		Enabled bool   `json:"enabled"`
	}{}
	err := a.db.Table("host_ips h").
		Select("h.ip as ip, h.enabled as enabled, count(oi.id) as count").
		Joins("left join order_items oi on oi.ip = h.ip and oi.status = ?", model.OrderItemStatusActive).
		Joins("left join orders o on o.id = oi.order_id and o.status = ? and o.expires_at > ?", model.OrderStatusActive, time.Now()).
		Group("h.id").
		Order("count desc, h.ip asc").
		Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func (a *API) listOrders(c *gin.Context) {
	rows, err := a.orders.ListOrders()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
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
	var req service.CreateOrderInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	order, err := a.orders.CreateOrder(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.OrderIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order_ids is empty"})
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
	text, err := a.orders.ExportOrderLines(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(text))
}

func (a *API) testOrder(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	out, err := a.orders.TestOrder(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, out)
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
	var exp time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err == nil {
			exp = t
		}
	}
	order, err := a.orders.ImportOrder(c.Request.Context(), req.CustomerID, req.OrderName, exp, req.Rows)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, order)
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
	if err := a.store.SetSettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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
