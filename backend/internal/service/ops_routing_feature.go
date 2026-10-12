package service

import (
	"context"
	"errors"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyRoutingDiagnosticsEnabled = "routing_diagnostics_enabled"

type OpsRoutingDiagnosticsConfig struct {
	Enabled          bool `json:"enabled"`
	EffectiveEnabled bool `json:"effective_enabled"`
}

func parseRoutingDiagnosticsEnabled(values map[string]string) (bool, error) {
	raw, exists := values[SettingKeyRoutingDiagnosticsEnabled]
	if !exists {
		// 未配置的既有部署继续采集诊断；非法配置不是缺失配置。
		return true, nil
	}
	switch raw {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, infraerrors.ServiceUnavailable("ROUTING_DIAGNOSTICS_CONFIG_INVALID", "路由诊断开关配置必须为 true 或 false")
	}
}

// IsRoutingDiagnosticsEnabled 只读不可变快照，独立于 Ops 监控开关且不查询数据库。
func (s *OpsService) IsRoutingDiagnosticsEnabled(ctx context.Context) bool {
	_ = ctx
	if s == nil {
		return true
	}
	if snapshot := s.runtimeSettings.Load(); snapshot != nil {
		return snapshot.routingDiagnosticsEnabled
	}
	return true
}

// GetRoutingDiagnosticsConfig 同时报告持久配置和本实例的生效状态。
func (s *OpsService) GetRoutingDiagnosticsConfig(ctx context.Context) (*OpsRoutingDiagnosticsConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SETTINGS_UNAVAILABLE", "设置存储不可用")
	}
	s.runtimeSettingsMu.Lock()
	defer s.runtimeSettingsMu.Unlock()
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyRoutingDiagnosticsEnabled)
	values := map[string]string{}
	if err == nil {
		values[SettingKeyRoutingDiagnosticsEnabled] = raw
	} else if !errors.Is(err, ErrSettingNotFound) {
		return nil, infraerrors.ServiceUnavailable("ROUTING_DIAGNOSTICS_CONFIG_LOAD_FAILED", "读取路由诊断配置失败").WithCause(err)
	}
	enabled, err := parseRoutingDiagnosticsEnabled(values)
	if err != nil {
		return nil, err
	}
	return &OpsRoutingDiagnosticsConfig{Enabled: enabled, EffectiveEnabled: s.IsRoutingDiagnosticsEnabled(ctx)}, nil
}

// UpdateRoutingDiagnosticsConfig 在持久化与刷新共用的锁内发布成功配置，失败保留快照。
func (s *OpsService) UpdateRoutingDiagnosticsConfig(ctx context.Context, enabled bool) (*OpsRoutingDiagnosticsConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SETTINGS_UNAVAILABLE", "设置存储不可用")
	}
	s.runtimeSettingsMu.Lock()
	defer s.runtimeSettingsMu.Unlock()
	if err := s.settingRepo.Set(ctx, SettingKeyRoutingDiagnosticsEnabled, strconv.FormatBool(enabled)); err != nil {
		return nil, infraerrors.ServiceUnavailable("ROUTING_DIAGNOSTICS_CONFIG_SAVE_FAILED", "保存路由诊断配置失败").WithCause(err)
	}
	next := &opsRuntimeSettingsSnapshot{monitoringEnabled: true, advanced: *defaultOpsAdvancedSettingsForConfig(s.cfg)}
	if current := s.runtimeSettings.Load(); current != nil {
		*next = *current
	}
	next.routingDiagnosticsEnabled = enabled
	s.runtimeSettings.Store(next)
	return &OpsRoutingDiagnosticsConfig{Enabled: enabled, EffectiveEnabled: enabled}, nil
}

func (s *OpsService) filterRoutingDiagnosticsForPersistence(ctx context.Context, entries []*OpsInsertErrorLogInput) error {
	if s.IsRoutingDiagnosticsEnabled(ctx) {
		return nil
	}
	// 队列可能在关闭前捕获诊断；整批准备完成后在写入边界统一检查当前快照。
	for _, entry := range entries {
		entry.RoutingDiagnostics = nil
		entry.RoutingDiagnosticsJSON = nil
		if entry.UpstreamErrorsJSON != nil {
			stripped, err := OpsUpstreamErrorsWithoutRoutingDiagnostics(*entry.UpstreamErrorsJSON)
			if err != nil {
				return err
			}
			entry.UpstreamErrorsJSON = &stripped
		}
	}
	return nil
}
