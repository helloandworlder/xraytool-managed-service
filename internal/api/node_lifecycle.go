package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"xraytool/internal/store"
)

func (a *API) nodeLifecycleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isSafeMethod(c.Request.Method) {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		state, err := a.store.GetNodeLifecycle()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if state.Status == store.NodeLifecycleStatusActive || nodeLifecycleAllowsMutation(state.Status, c.Request.Method, path) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"error":  "node lifecycle blocks write operations on this node",
			"code":   "node_lifecycle_blocked",
			"status": state.Status,
		})
	}
}

func (a *API) getNodeLifecycle(c *gin.Context) {
	state, err := a.store.GetNodeLifecycle()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}

func (a *API) updateNodeLifecycle(c *gin.Context) {
	var req struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	state, err := a.store.SetNodeLifecycle(req.Status, req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}

func nodeLifecycleAllowsMutation(status string, method string, path string) bool {
	if isSafeMethod(method) {
		return true
	}
	normalizedPath := normalizeLifecyclePath(path)
	switch {
	case normalizedPath == "/api/node-lifecycle":
		return true
	case strings.HasPrefix(normalizedPath, "/api/db/"):
		return true
	case normalizedPath == "/api/orders/import/preview":
		return true
	case normalizedPath == "/api/migrations/singbox/preview":
		return true
	case normalizedPath == "/api/migrations/socks5/preview":
		return true
	case normalizedPath == "/api/dedicated/check":
		return true
	case normalizedPath == "/api/orders/dedicated-inbounds/validate":
		return true
	case normalizedPath == "/api/orders/dedicated-inbounds/reality-keypair":
		return true
	case strings.HasPrefix(normalizedPath, "/api/orders/") && strings.HasSuffix(normalizedPath, "/deactivate"):
		return true
	case normalizedPath == "/api/orders/batch/deactivate":
		return true
	case strings.HasPrefix(normalizedPath, "/api/orders/") && strings.HasSuffix(normalizedPath, "/test"):
		return true
	case strings.HasPrefix(normalizedPath, "/api/orders/") && strings.HasSuffix(normalizedPath, "/test/stream"):
		return true
	case normalizedPath == "/api/orders/batch/test":
		return true
	case normalizedPath == "/api/orders/batch/export":
		return true
	default:
		return false
	}
}

func isSafeMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func normalizeLifecyclePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if idx := strings.Index(trimmed, "?"); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}
