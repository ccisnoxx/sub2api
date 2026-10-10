package service

import (
	"math"
	"testing"

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
		counts              ServiceStatusCounts
		gap, health, reason string
	}{
		{health: "unknown", reason: "no_recent_requests"}, {counts: ServiceStatusCounts{Excluded: 5}, health: "unknown", reason: "no_qualified_requests"},
		{counts: ServiceStatusCounts{Success: 4}, health: "unknown", reason: "insufficient_samples"}, {counts: ServiceStatusCounts{Success: 5, Unknown: 1}, health: "unknown", reason: "terminal_unknown"},
		{counts: ServiceStatusCounts{Success: 5}, gap: "source_error", health: "unknown", reason: "source_error"},
		{counts: ServiceStatusCounts{Success: 19, Failure: 1}, health: "degraded"}, {counts: ServiceStatusCounts{Success: 1, Failure: 9}, health: "outage"}, {counts: ServiceStatusCounts{Success: 5, Excluded: 100}, health: "operational"},
	} {
		m, h, r := ServiceStatusEvaluate(tt.counts, c, tt.gap)
		require.Equal(t, tt.health, h)
		require.Equal(t, tt.reason, r)
		if h == "unknown" {
			require.Nil(t, m.ErrorRate)
			require.Nil(t, m.SuccessRate)
		} else {
			require.NotNil(t, m.ErrorRate)
		}
	}
}
func TestServiceStatusSummaryDoesNotWeightAwayUnknownOrIncident(t *testing.T) {
	phase := "awaiting_data"
	for _, tt := range []struct {
		leaves []ServiceStatusLeaf
		want   string
	}{
		{[]ServiceStatusLeaf{{Health: "operational"}, {Health: "unknown"}}, "unknown"},
		{[]ServiceStatusLeaf{{Health: "outage"}, {Health: "unknown"}}, "degraded"},
		{[]ServiceStatusLeaf{{Health: "outage"}, {Health: "outage"}}, "outage"},
		{[]ServiceStatusLeaf{{Health: "operational", IncidentPhase: &phase}, {Health: "operational"}}, "unknown"},
	} {
		h, _ := ServiceStatusSummary(tt.leaves)
		require.Equal(t, tt.want, h)
		tt.leaves[0], tt.leaves[1] = tt.leaves[1], tt.leaves[0]
		h, _ = ServiceStatusSummary(tt.leaves)
		require.Equal(t, tt.want, h)
	}
}
