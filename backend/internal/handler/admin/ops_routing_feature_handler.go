package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// GetRoutingDiagnosticsConfig 不受诊断或监控开关拦截，管理员始终能读取配置。
func (h *OpsHandler) GetRoutingDiagnosticsConfig(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "运维服务不可用")
		return
	}
	cfg, err := h.opsService.GetRoutingDiagnosticsConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

// UpdateRoutingDiagnosticsConfig 明确要求布尔值，成功写入后记录允许的审计摘要。
func (h *OpsHandler) UpdateRoutingDiagnosticsConfig(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "运维服务不可用")
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled 必须是明确的布尔值")
		return
	}
	cfg, err := h.opsService.UpdateRoutingDiagnosticsConfig(c.Request.Context(), *req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditAction(c, "admin.ops.routing_diagnostics.config.update")
	middleware.SetAuditExtra(c, map[string]any{"enabled": cfg.Enabled, "result": "success"})
	response.Success(c, cfg)
}
