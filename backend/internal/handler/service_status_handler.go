package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type serviceStatusReader interface {
	GetConfig(context.Context) (*service.ServiceStatusConfig, error)
	UpdateConfig(context.Context, *service.ServiceStatusConfig) (*service.ServiceStatusConfig, error)
	Snapshot(context.Context, string, string) (*service.ServiceStatusSnapshot, error)
}

// ServiceStatusHandler 只服务现有管理员路由，不接收用户或分组模拟范围。
type ServiceStatusHandler struct {
	service serviceStatusReader
}

func NewServiceStatusHandler(svc *service.ServiceStatusService) *ServiceStatusHandler {
	return &ServiceStatusHandler{service: svc}
}

func (h *ServiceStatusHandler) authorize(c *gin.Context) bool {
	subject, authenticated := middleware.GetAuthSubjectFromContext(c)
	if !authenticated || subject.UserID <= 0 {
		response.Unauthorized(c, "需要管理员认证")
		return false
	}
	role, ok := middleware.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		response.Forbidden(c, "需要管理员权限")
		return false
	}
	return true
}

func (h *ServiceStatusHandler) GetConfig(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	if values, err := url.ParseQuery(c.Request.URL.RawQuery); err != nil || len(values) != 0 {
		response.BadRequest(c, "服务状态配置不接受查询参数")
		return
	}
	cfg, err := h.service.GetConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *ServiceStatusHandler) UpdateConfig(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	if values, err := url.ParseQuery(c.Request.URL.RawQuery); err != nil || len(values) != 0 {
		response.BadRequest(c, "服务状态配置不接受查询参数")
		return
	}
	// 指针区分缺失字段与合法的 false / 空平台；不把缺失配置补成默认值。
	var input struct {
		Version          *int64    `json:"version"`
		Enabled          *bool     `json:"enabled"`
		Platforms        *[]string `json:"platforms"`
		MinimumSamples   *int      `json:"minimum_samples"`
		WarningErrorRate *float64  `json:"warning_error_rate"`
		OutageErrorRate  *float64  `json:"outage_error_rate"`
		AbnormalWindows  *int      `json:"abnormal_windows"`
		RecoveryWindows  *int      `json:"recovery_windows"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 8192))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&input)
	if err == nil {
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}
	if err != nil || input.Version == nil || input.Enabled == nil || input.Platforms == nil ||
		input.MinimumSamples == nil || input.WarningErrorRate == nil || input.OutageErrorRate == nil ||
		input.AbnormalWindows == nil || input.RecoveryWindows == nil {
		response.BadRequest(c, "需要完整且有效的服务状态配置")
		return
	}
	cfg := &service.ServiceStatusConfig{
		Version: *input.Version, Enabled: *input.Enabled, Platforms: *input.Platforms,
		MinimumSamples: *input.MinimumSamples, WarningErrorRate: *input.WarningErrorRate,
		OutageErrorRate: *input.OutageErrorRate, AbnormalWindows: *input.AbnormalWindows,
		RecoveryWindows: *input.RecoveryWindows,
	}
	updated, err := h.service.UpdateConfig(c.Request.Context(), cfg)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ServiceStatusHandler) Snapshot(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		response.BadRequest(c, "无效的服务状态查询参数")
		return
	}
	for key, entries := range values {
		if (key != "range" && key != "platform") || len(entries) != 1 || entries[0] == "" {
			response.BadRequest(c, "服务状态仅接受单个 range 和 platform 参数")
			return
		}
	}
	historyRange := values.Get("range")
	if historyRange == "" {
		historyRange = "24h"
	}
	if historyRange != "24h" && historyRange != "7d" && historyRange != "30d" {
		response.BadRequest(c, "服务状态 range 必须是 24h、7d 或 30d")
		return
	}
	snapshot, err := h.service.Snapshot(c.Request.Context(), historyRange, values.Get("platform"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}
