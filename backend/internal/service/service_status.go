package service

import (
	"context"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrServiceStatusDisabled         = apperrors.ServiceUnavailable("SERVICE_STATUS_DISABLED", "服务状态监控未启用")
	ErrServiceStatusConfigConflict   = apperrors.Conflict("SERVICE_STATUS_CONFIG_CONFLICT", "服务状态配置已更新，请重新读取")
	ErrServiceStatusInvalidConfig    = apperrors.BadRequest("SERVICE_STATUS_INVALID_CONFIG", "服务状态配置不完整或无效")
	ErrServiceStatusInvalidFilter    = apperrors.BadRequest("SERVICE_STATUS_INVALID_FILTER", "服务状态筛选无效")
	ErrServiceStatusUnavailable      = apperrors.ServiceUnavailable("SERVICE_STATUS_UNAVAILABLE", "服务状态数据暂不可用")
	ErrServiceStatusObservationLimit = apperrors.ServiceUnavailable("SERVICE_STATUS_OBSERVATION_LIMIT", "服务状态单轮观察数量超过限制")
)

// ServiceStatusRepository 的 Aggregate 在同一事务内维护事实、水位和事件。
type ServiceStatusRepository interface {
	GetConfig(context.Context) (*ServiceStatusConfig, error)
	UpdateConfig(context.Context, *ServiceStatusConfig) (*ServiceStatusConfig, error)
	Snapshot(context.Context, string, string) (*ServiceStatusSnapshot, error)
	Aggregate(context.Context, time.Time) error
	RecordSourceError(context.Context, time.Time) error
}

type ServiceStatusService struct{ repo ServiceStatusRepository }

func NewServiceStatusService(repo ServiceStatusRepository) *ServiceStatusService {
	return &ServiceStatusService{repo: repo}
}
func (s *ServiceStatusService) GetConfig(ctx context.Context) (*ServiceStatusConfig, error) {
	return s.repo.GetConfig(ctx)
}
func (s *ServiceStatusService) UpdateConfig(ctx context.Context, cfg *ServiceStatusConfig) (*ServiceStatusConfig, error) {
	if err := ValidateServiceStatusConfig(cfg); err != nil {
		return nil, err
	}
	return s.repo.UpdateConfig(ctx, cfg)
}
func (s *ServiceStatusService) Snapshot(ctx context.Context, historyRange, platform string) (*ServiceStatusSnapshot, error) {
	if historyRange == "" {
		historyRange = "24h"
	}
	if historyRange != "24h" && historyRange != "7d" && historyRange != "30d" {
		return nil, ErrServiceStatusInvalidFilter
	}
	if platform != "" && !ServiceStatusPlatformKnown(platform) {
		return nil, ErrServiceStatusInvalidFilter
	}
	return s.repo.Snapshot(ctx, historyRange, platform)
}

func ServiceStatusPlatformKnown(platform string) bool {
	switch platform {
	case domain.PlatformOpenAI, domain.PlatformAnthropic, domain.PlatformGemini, domain.PlatformAntigravity, domain.PlatformGrok,
		domain.PlatformKimi, domain.PlatformZhipu, domain.PlatformDeepseek, domain.PlatformMiniMax, domain.PlatformTypeSafe, domain.PlatformOpenCodeGo:
		return true
	}
	return false
}
func ValidateServiceStatusConfig(c *ServiceStatusConfig) error {
	if c == nil || c.Version < 1 || c.Platforms == nil || c.MinimumSamples < 1 || c.MinimumSamples > 10000 ||
		c.AbnormalWindows < 1 || c.AbnormalWindows > 10 || c.RecoveryWindows < 1 || c.RecoveryWindows > 10 ||
		math.IsNaN(c.WarningErrorRate) || math.IsInf(c.WarningErrorRate, 0) || c.WarningErrorRate <= 0 || c.WarningErrorRate >= 1 ||
		math.IsNaN(c.OutageErrorRate) || math.IsInf(c.OutageErrorRate, 0) || c.OutageErrorRate <= c.WarningErrorRate || c.OutageErrorRate > 1 {
		return ErrServiceStatusInvalidConfig
	}
	seen := map[string]bool{}
	for _, p := range c.Platforms {
		if !ServiceStatusPlatformKnown(p) || seen[p] {
			return ErrServiceStatusInvalidConfig
		}
		seen[p] = true
	}
	return nil
}

// ServiceStatusEvaluate 是当前与历史比例的共同口径；未知、覆盖缺口和小样本不输出比例。
func ServiceStatusEvaluate(c ServiceStatusCounts, cfg ServiceStatusConfig, gap string) (ServiceStatusMetrics, string, string) {
	m := ServiceStatusMetrics{ServiceStatusCounts: c, Qualified: c.Success + c.Failure}
	reason := gap
	if reason == "" {
		switch {
		case c.Success+c.Failure+c.Excluded+c.Unknown == 0:
			reason = "no_recent_requests"
		case c.Unknown > 0:
			reason = "terminal_unknown"
		case m.Qualified == 0:
			reason = "no_qualified_requests"
		case m.Qualified < int64(cfg.MinimumSamples):
			reason = "insufficient_samples"
		}
	}
	if reason != "" {
		return m, "unknown", reason
	}
	rate := float64(c.Failure) / float64(m.Qualified)
	success := 1 - rate
	m.ErrorRate, m.SuccessRate = &rate, &success
	if rate >= cfg.OutageErrorRate {
		return m, "outage", ""
	}
	if rate >= cfg.WarningErrorRate {
		return m, "degraded", ""
	}
	return m, "operational", ""
}

// 摘要按叶子状态取优先级；流量大的正常叶子不能抵消其他叶子的未知或事件。
func ServiceStatusSummary(leaves []ServiceStatusLeaf) (string, string) {
	if len(leaves) == 0 {
		return "unknown", "no_recent_requests"
	}
	outage, abnormal, unknown, recovering := 0, false, false, false
	for _, l := range leaves {
		if l.Health == "recovering" {
			recovering = true
		}
		if l.Health == "outage" {
			outage++
			abnormal = true
		}
		if l.Health == "degraded" {
			abnormal = true
		}
		if l.Health == "unknown" {
			unknown = true
		}
		if l.IncidentPhase != nil {
			if *l.IncidentPhase == "recovering" {
				recovering = true
			} else if l.Health == "operational" {
				unknown = true
			}
		}
	}
	if outage == len(leaves) && !unknown {
		return "outage", ""
	}
	if abnormal {
		return "degraded", ""
	}
	if unknown {
		return "unknown", "awaiting_data"
	}
	if recovering {
		return "recovering", ""
	}
	return "operational", ""
}
