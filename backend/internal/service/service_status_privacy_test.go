package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceStatusPrivacySuppressesSmallScopesAndComplementaryTotals(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 31, 47, 0, time.UTC)
	low := ServiceStatusScope{Platform: "openai", GroupID: 7, GroupName: "private-group", RequestedModel: "private-model"}
	high := ServiceStatusScope{Platform: "openai", GroupID: 8, GroupName: "visible-group", RequestedModel: "visible-model"}
	rate := 1.0
	id, phase := "private-incident", "detected"
	s := &ServiceStatusSnapshot{MinimumSamples: 1, Health: "operational",
		Scopes: []ServiceStatusLeaf{
			{ServiceStatusScope: low, ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 4, SuccessRate: &rate}, LastEvidenceAt: &at, IncidentID: &id, IncidentPhase: &phase},
			{ServiceStatusScope: high, ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 5, ServiceStatusCounts: ServiceStatusCounts{Success: 5}, SuccessRate: &rate}, LastEvidenceAt: &at},
		},
		History:   []ServiceStatusHistoryPoint{{ServiceStatusScope: low, ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 4, SuccessRate: &rate}}},
		Platforms: []ServiceStatusPlatform{{Platform: "openai", Health: "operational", ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 9, SuccessRate: &rate}}},
		Incidents: []ServiceStatusIncident{{ServiceStatusScope: low, ID: id, Phase: phase, DetectedAt: at, LastEvidenceAt: at, LastAbnormalAt: at}}, OpenIncidentCount: 1,
	}
	out := PrivateServiceStatusSnapshot(s)
	require.Equal(t, 5, out.PrivacyMinimumSamples)
	require.True(t, out.PrivacySuppressed)
	require.Equal(t, "unknown", out.Health)
	require.Len(t, out.Scopes, 1)
	require.Equal(t, high, out.Scopes[0].ServiceStatusScope)
	require.Equal(t, at.Truncate(time.Hour), *out.Scopes[0].LastEvidenceAt)
	require.Empty(t, out.History)
	require.Empty(t, out.Incidents)
	require.Zero(t, out.OpenIncidentCount)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	for _, hidden := range []string{"private-group", "private-model", "private-incident", "12:31:47"} {
		require.NotContains(t, string(raw), hidden)
	}
	require.Contains(t, string(raw), `"success":null`)
	require.Contains(t, string(raw), `"qualified":null`)
	require.Contains(t, string(raw), `"error_rate":null`)
	require.Contains(t, string(raw), `"success_rate":null`)
	require.Equal(t, at, *s.Scopes[1].LastEvidenceAt, "投影不能改写聚合 owner")
	require.Equal(t, id, *s.Scopes[0].IncidentID)
}

func TestServiceStatusPrivacyHidesRecoveringWhenIncidentEvidenceIsSmall(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 31, 0, 0, time.UTC)
	scope := ServiceStatusScope{Platform: "openai", GroupID: 8, RequestedModel: "model"}
	id, phase, rate := "hidden-incident", "recovering", 1.0
	metrics := ServiceStatusMetrics{ServiceStatusCounts: ServiceStatusCounts{Success: 5}, Qualified: 5, SuccessRate: &rate}
	s := &ServiceStatusSnapshot{MinimumSamples: 1, Health: phase,
		Scopes:    []ServiceStatusLeaf{{ServiceStatusScope: scope, ServiceStatusMetrics: metrics, Health: phase, IncidentID: &id, IncidentPhase: &phase}},
		Platforms: []ServiceStatusPlatform{{Platform: "openai", Health: phase, ServiceStatusMetrics: metrics}},
		History:   []ServiceStatusHistoryPoint{{ServiceStatusScope: scope, ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 1}, ServiceStatusInterval: ServiceStatusInterval{Start: at.Truncate(time.Hour), End: at.Truncate(time.Hour).Add(time.Hour)}}},
		Incidents: []ServiceStatusIncident{{ServiceStatusScope: scope, ID: id, Phase: phase, DetectedAt: at, LastEvidenceAt: at, LastAbnormalAt: at}},
	}
	out := PrivateServiceStatusSnapshot(s)
	require.Empty(t, out.Incidents)
	require.Equal(t, "unknown", out.Scopes[0].Health)
	require.Nil(t, out.Scopes[0].SuccessRate)
	require.Equal(t, "unknown", out.Platforms[0].Health)
	require.Equal(t, "unknown", out.Health)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	require.NotContains(t, string(raw), phase)
	require.NotContains(t, string(raw), id)
	require.Equal(t, phase, s.Scopes[0].Health)
}

func TestServiceStatusPrivacyUsesConfiguredFloorAndProtectsIncidentTimes(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 31, 47, 0, time.UTC)
	scope := ServiceStatusScope{Platform: "openai", GroupID: 8, RequestedModel: "model"}
	s := &ServiceStatusSnapshot{MinimumSamples: 10,
		Scopes:    []ServiceStatusLeaf{{ServiceStatusScope: scope, ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 10}, LastEvidenceAt: &at}},
		History:   []ServiceStatusHistoryPoint{{ServiceStatusScope: scope, ServiceStatusMetrics: ServiceStatusMetrics{Qualified: 10}, ServiceStatusInterval: ServiceStatusInterval{Start: at.Truncate(time.Hour), End: at.Truncate(time.Hour).Add(time.Hour)}}},
		Incidents: []ServiceStatusIncident{{ServiceStatusScope: scope, ID: "incident", Phase: "recovering", DetectedAt: at, LastEvidenceAt: at, LastAbnormalAt: at, Updates: []ServiceStatusIncidentUpdate{{Phase: "recovering", At: at}}}},
	}
	out := PrivateServiceStatusSnapshot(s)
	require.Equal(t, 10, out.PrivacyMinimumSamples)
	require.Len(t, out.Incidents, 1)
	require.Equal(t, 1, out.OpenIncidentCount)
	require.Equal(t, at.Truncate(time.Hour), out.Incidents[0].Updates[0].At)
	require.Equal(t, at, s.Incidents[0].Updates[0].At)
	s.History[0].Qualified = 9
	out = PrivateServiceStatusSnapshot(s)
	require.Len(t, out.Scopes, 1)
	require.Empty(t, out.Incidents, "不能从事件或更新时点重建低样本历史")
	require.Nil(t, out.Scopes[0].IncidentID)
	s.Scopes[0].Qualified = 9
	require.Empty(t, PrivateServiceStatusSnapshot(s).Scopes)
}

func TestServiceStatusPrivacyNoticeDoesNotTrackHiddenIncidentRecovery(t *testing.T) {
	at := time.Date(2026, 10, 10, 12, 19, 0, 0, time.UTC)
	scope := ServiceStatusScope{Platform: "openai", GroupID: 8, RequestedModel: "model"}
	metrics := ServiceStatusMetrics{ServiceStatusCounts: ServiceStatusCounts{Success: 5}, Qualified: 5}
	s := &ServiceStatusSnapshot{MinimumSamples: 1, Health: "operational",
		Scopes:       []ServiceStatusLeaf{{ServiceStatusScope: scope, ServiceStatusMetrics: metrics, Health: "operational"}},
		Platforms:    []ServiceStatusPlatform{{Platform: "openai", Health: "operational", ServiceStatusMetrics: metrics}},
		HistoryRange: ServiceStatusInterval{Start: at.Truncate(time.Hour).Add(-24 * time.Hour), End: at.Truncate(time.Hour)},
		Incidents:    []ServiceStatusIncident{{ServiceStatusScope: scope, ID: "hidden-current-hour", Phase: "recovering", DetectedAt: at.Add(-9 * time.Minute), LastEvidenceAt: at, LastAbnormalAt: at.Add(-9 * time.Minute)}},
	}
	for start := s.HistoryRange.Start; start.Before(s.HistoryRange.End); start = start.Add(time.Hour) {
		s.History = append(s.History, ServiceStatusHistoryPoint{ServiceStatusScope: scope, ServiceStatusMetrics: metrics, ServiceStatusInterval: ServiceStatusInterval{Start: start, End: start.Add(time.Hour)}})
	}
	before := PrivateServiceStatusSnapshot(s)
	require.Empty(t, before.Incidents)
	s.Incidents[0].Phase, s.Incidents[0].ResolvedAt = "resolved", &at
	after := PrivateServiceStatusSnapshot(s)
	require.Empty(t, after.Incidents)
	require.Equal(t, before.PrivacySuppressed, after.PrivacySuppressed, "提示不能暴露当前小时隐藏事件已恢复")
	require.False(t, after.PrivacySuppressed, "全部样本足量，提示只反映样本和合计指标裁剪")
}
