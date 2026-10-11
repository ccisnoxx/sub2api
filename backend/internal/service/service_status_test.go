package service

import (
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"github.com/stretchr/testify/require"
)

func statusTestConfig() ServiceStatusConfig {
	return ServiceStatusConfig{Version: 1, Platforms: []string{"openai"}, MinimumSamples: 5, WarningErrorRate: 0.05, OutageErrorRate: 0.9, AbnormalWindows: 2, RecoveryWindows: 3}
}
func TestServiceStatusConfigValidation(t *testing.T) {
	base := statusTestConfig()
	require.NoError(t, ValidateServiceStatusConfig(&base))
	for _, change := range []func(*ServiceStatusConfig){
		func(c *ServiceStatusConfig) { c.Version = 0 }, func(c *ServiceStatusConfig) { c.Platforms = nil }, func(c *ServiceStatusConfig) { c.Platforms = []string{"composite"} }, func(c *ServiceStatusConfig) { c.Platforms = []string{"openai", "openai"} },
		func(c *ServiceStatusConfig) { c.MinimumSamples = 0 }, func(c *ServiceStatusConfig) { c.MinimumSamples = 10001 }, func(c *ServiceStatusConfig) { c.WarningErrorRate = math.NaN() }, func(c *ServiceStatusConfig) { c.OutageErrorRate = math.Inf(1) }, func(c *ServiceStatusConfig) { c.OutageErrorRate = c.WarningErrorRate }, func(c *ServiceStatusConfig) { c.RecoveryWindows = 11 },
	} {
		c := base
		change(&c)
		require.ErrorIs(t, ValidateServiceStatusConfig(&c), ErrServiceStatusInvalidConfig)
	}
	base.Platforms = []string{}
	require.NoError(t, ValidateServiceStatusConfig(&base))
}
func TestServiceStatusMetricsUnknownAndThresholds(t *testing.T) {
	c := statusTestConfig()
	for _, tt := range []struct {
		name                string
		counts              ServiceStatusCounts
		gap, health, reason string
		errorRate           float64
	}{
		{name: "零请求", health: "unknown", reason: "no_recent_requests"},
		{name: "全排除", counts: ServiceStatusCounts{Excluded: 5}, health: "unknown", reason: "no_qualified_requests"},
		{name: "全未知", counts: ServiceStatusCounts{Unknown: 5}, health: "unknown", reason: "terminal_unknown"},
		{name: "成功不能抵消未知", counts: ServiceStatusCounts{Success: 10000, Unknown: 1}, health: "unknown", reason: "terminal_unknown"},
		{name: "最低样本之前", counts: ServiceStatusCounts{Success: 4}, health: "unknown", reason: "insufficient_samples"},
		{name: "排除不补足样本", counts: ServiceStatusCounts{Success: 4, Excluded: 100}, health: "unknown", reason: "insufficient_samples"},
		{name: "最低样本边界", counts: ServiceStatusCounts{Success: 5}, health: "operational"},
		{name: "warning之前", counts: ServiceStatusCounts{Success: 96, Failure: 4}, health: "operational", errorRate: .04},
		{name: "warning边界", counts: ServiceStatusCounts{Success: 95, Failure: 5}, health: "degraded", errorRate: .05},
		{name: "outage之前", counts: ServiceStatusCounts{Success: 11, Failure: 89}, health: "degraded", errorRate: .89},
		{name: "outage边界", counts: ServiceStatusCounts{Success: 10, Failure: 90}, health: "outage", errorRate: .9},
		{name: "全失败", counts: ServiceStatusCounts{Failure: 5}, health: "outage", errorRate: 1},
		{name: "排除不计入分母", counts: ServiceStatusCounts{Success: 19, Failure: 1, Excluded: 100}, health: "degraded", errorRate: .05},
		{name: "缺口优先于零请求", gap: "source_error", health: "unknown", reason: "source_error"},
		{name: "缺口优先于未知终态", counts: ServiceStatusCounts{Unknown: 5}, gap: "coverage_gap", health: "unknown", reason: "coverage_gap"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, h, r := ServiceStatusEvaluate(tt.counts, c, tt.gap)
			require.Equal(t, tt.health, h)
			require.Equal(t, tt.reason, r)
			require.Equal(t, tt.counts, m.ServiceStatusCounts)
			require.Equal(t, tt.counts.Success+tt.counts.Failure, m.Qualified)
			if h == "unknown" {
				require.Nil(t, m.ErrorRate)
				require.Nil(t, m.SuccessRate)
			} else {
				require.NotNil(t, m.ErrorRate)
				require.NotNil(t, m.SuccessRate)
				require.InDelta(t, tt.errorRate, *m.ErrorRate, 1e-12)
				require.InDelta(t, 1-tt.errorRate, *m.SuccessRate, 1e-12)
			}
		})
	}
}
func TestServiceStatusSummaryDoesNotWeightAwayUnknownOrIncident(t *testing.T) {
	phase := "awaiting_data"
	recovering := "recovering"
	for _, tt := range []struct {
		name         string
		leaves       []ServiceStatusLeaf
		want, reason string
	}{
		{"空范围", nil, "unknown", "no_recent_requests"},
		{"全正常", []ServiceStatusLeaf{{Health: "operational"}, {Health: "operational"}}, "operational", ""},
		{"正常与未知", []ServiceStatusLeaf{{Health: "operational", ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 10000}}, {Health: "unknown"}}, "unknown", "awaiting_data"},
		{"中断与未知", []ServiceStatusLeaf{{Health: "outage"}, {Health: "unknown"}}, "degraded", ""},
		{"全中断", []ServiceStatusLeaf{{Health: "outage"}, {Health: "outage"}}, "outage", ""},
		{"中断与正常", []ServiceStatusLeaf{{Health: "outage"}, {Health: "operational"}}, "degraded", ""},
		{"告警与恢复", []ServiceStatusLeaf{{Health: "degraded"}, {Health: "recovering"}}, "degraded", ""},
		{"未知优先于恢复", []ServiceStatusLeaf{{Health: "unknown"}, {Health: "recovering"}}, "unknown", "awaiting_data"},
		{"恢复窗口未满", []ServiceStatusLeaf{{Health: "recovering"}, {Health: "operational"}}, "recovering", ""},
		{"恢复事件", []ServiceStatusLeaf{{Health: "operational", IncidentPhase: &recovering}, {Health: "operational"}}, "recovering", ""},
		{"等待新证据", []ServiceStatusLeaf{{Health: "operational", IncidentPhase: &phase}, {Health: "operational"}}, "unknown", "awaiting_data"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h, reason := ServiceStatusSummary(tt.leaves)
			require.Equal(t, tt.want, h)
			require.Equal(t, tt.reason, reason)
			if len(tt.leaves) == 2 {
				tt.leaves[0], tt.leaves[1] = tt.leaves[1], tt.leaves[0]
				h, reason = ServiceStatusSummary(tt.leaves)
				require.Equal(t, tt.want, h)
				require.Equal(t, tt.reason, reason)
			}
		})
	}
}

func TestServiceStatusPlatformCatalogUsesConcreteDomainOwner(t *testing.T) {
	for _, platform := range domain.ConcretePlatformIDs() {
		t.Run(platform, func(t *testing.T) {
			require.True(t, ServiceStatusPlatformKnown(platform))
			cfg := statusTestConfig()
			cfg.Platforms = []string{platform}
			require.NoError(t, ValidateServiceStatusConfig(&cfg))
		})
	}
	for _, platform := range []string{"composite", "unknown", "", "OPENAI"} {
		require.False(t, ServiceStatusPlatformKnown(platform))
	}
}
