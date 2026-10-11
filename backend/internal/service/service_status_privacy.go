package service

import "time"

// ServiceStatusPrivacyMinimum 不受管理员健康阈值配置影响；旧配置仍可读取和停用。
const ServiceStatusPrivacyMinimum = 5

// 平台合计隐藏时使用 null，不能用零值冒充没有请求。
type ServiceStatusPrivateMetrics struct {
	Success     *int64   `json:"success"`
	Failure     *int64   `json:"failure"`
	Excluded    *int64   `json:"excluded"`
	Unknown     *int64   `json:"unknown"`
	Qualified   *int64   `json:"qualified"`
	ErrorRate   *float64 `json:"error_rate"`
	SuccessRate *float64 `json:"success_rate"`
}

type ServiceStatusPrivatePlatform struct {
	Platform      string `json:"platform"`
	Health        string `json:"health"`
	UnknownReason string `json:"unknown_reason,omitempty"`
	ServiceStatusPrivateMetrics
}

// 只投影管理页所需字段；原始事实、请求身份和聚合状态不属于响应合同。
type ServiceStatusPrivateSnapshot struct {
	SchemaVersion            int                            `json:"schema_version"`
	ConfigVersion            int64                          `json:"config_version"`
	GeneratedAt              time.Time                      `json:"generated_at"`
	ObservedThrough          *time.Time                     `json:"observed_through"`
	Window                   ServiceStatusInterval          `json:"window"`
	HistoryRange             ServiceStatusInterval          `json:"history_range"`
	MonitoringEnabled        bool                           `json:"monitoring_enabled"`
	EnabledPlatforms         []string                       `json:"enabled_platforms"`
	Health                   string                         `json:"health"`
	UnknownReason            string                         `json:"unknown_reason,omitempty"`
	Coverage                 []ServiceStatusCoverage        `json:"coverage"`
	Gaps                     []string                       `json:"gaps"`
	Platforms                []ServiceStatusPrivatePlatform `json:"platforms"`
	Scopes                   []ServiceStatusLeaf            `json:"scopes"`
	History                  []ServiceStatusHistoryPoint    `json:"history"`
	Incidents                []ServiceStatusIncident        `json:"incidents"`
	OpenIncidentCount        int                            `json:"open_incident_count"`
	BoundedObservationNotice string                         `json:"bounded_observation_notice"`
	PrivacyMinimumSamples    int                            `json:"privacy_minimum_samples"`
	PrivacySuppressed        bool                           `json:"privacy_suppressed"`
}

// PrivateServiceStatusSnapshot 不改写仓储 owner，也不参与事件推进或计费。
func PrivateServiceStatusSnapshot(s *ServiceStatusSnapshot) *ServiceStatusPrivateSnapshot {
	minimum := max(ServiceStatusPrivacyMinimum, s.MinimumSamples)
	out := &ServiceStatusPrivateSnapshot{
		SchemaVersion: 2, ConfigVersion: s.ConfigVersion, GeneratedAt: s.GeneratedAt,
		ObservedThrough: s.ObservedThrough, Window: s.Window, HistoryRange: s.HistoryRange,
		MonitoringEnabled: s.MonitoringEnabled, EnabledPlatforms: s.EnabledPlatforms,
		Health: s.Health, UnknownReason: s.UnknownReason, Coverage: s.Coverage, Gaps: s.Gaps,
		Platforms: []ServiceStatusPrivatePlatform{}, Scopes: []ServiceStatusLeaf{},
		History: []ServiceStatusHistoryPoint{}, Incidents: []ServiceStatusIncident{},
		BoundedObservationNotice: s.BoundedObservationNotice, PrivacyMinimumSamples: minimum,
	}
	visible := map[ServiceStatusScope]bool{}
	suppressedPlatforms := map[string]bool{}
	for _, leaf := range s.Scopes {
		if leaf.Qualified < int64(minimum) {
			out.PrivacySuppressed = true
			suppressedPlatforms[leaf.Platform] = true
			continue
		}
		leaf.LastEvidenceAt = statusPrivateTime(leaf.LastEvidenceAt)
		// 事件关联只在事件本身获准展示后补入，不能泄漏隐藏事件 ID 或阶段。
		leaf.IncidentID, leaf.IncidentPhase = nil, nil
		out.Scopes = append(out.Scopes, leaf)
		visible[leaf.ServiceStatusScope] = true
	}
	historyByScope := map[ServiceStatusScope][]ServiceStatusInterval{}
	for _, point := range s.History {
		if point.Qualified < int64(minimum) {
			out.PrivacySuppressed = true
			continue
		}
		out.History = append(out.History, point)
		historyByScope[point.ServiceStatusScope] = append(historyByScope[point.ServiceStatusScope], point.ServiceStatusInterval)
	}
	// 事件的每个证据时点都需落入可展示历史桶；否则整个事件隐藏。
	coveredEvidence := func(scope ServiceStatusScope, at time.Time) bool {
		for _, point := range historyByScope[scope] {
			if !at.Before(point.Start) && at.Before(point.End) {
				return true
			}
		}
		return false
	}
	incidents := make([]ServiceStatusIncident, 0, len(s.Incidents)+len(s.HistoryIncidents))
	for _, incident := range s.Incidents {
		if incident.ResolvedAt == nil {
			incidents = append(incidents, incident)
		}
	}
	incidents = append(incidents, s.HistoryIncidents...)
	for _, incident := range incidents {
		allowed := visible[incident.ServiceStatusScope]
		for _, at := range []time.Time{incident.DetectedAt, incident.LastEvidenceAt, incident.LastAbnormalAt} {
			allowed = allowed && coveredEvidence(incident.ServiceStatusScope, at)
		}
		if incident.ResolvedAt != nil {
			allowed = allowed && coveredEvidence(incident.ServiceStatusScope, *incident.ResolvedAt)
		}
		for _, update := range incident.Updates {
			allowed = allowed && coveredEvidence(incident.ServiceStatusScope, update.At)
		}
		if !allowed {
			out.PrivacySuppressed = true
			continue
		}
		incident.DetectedAt = incident.DetectedAt.UTC().Truncate(time.Hour)
		incident.LastEvidenceAt = incident.LastEvidenceAt.UTC().Truncate(time.Hour)
		incident.LastAbnormalAt = incident.LastAbnormalAt.UTC().Truncate(time.Hour)
		incident.ResolvedAt = statusPrivateTime(incident.ResolvedAt)
		incident.Updates = append([]ServiceStatusIncidentUpdate{}, incident.Updates...)
		for i := range incident.Updates {
			incident.Updates[i].At = incident.Updates[i].At.UTC().Truncate(time.Hour)
		}
		if incident.ResolvedAt == nil {
			out.OpenIncidentCount++
			for i := range out.Scopes {
				if out.Scopes[i].ServiceStatusScope == incident.ServiceStatusScope {
					out.Scopes[i].IncidentID, out.Scopes[i].IncidentPhase = &incident.ID, &incident.Phase
				}
			}
		}
		out.Incidents = append(out.Incidents, incident)
	}
	// recovering 来自事件阶段；隐藏事件后不能经 health 字段泄漏，也不能冒充已恢复。
	for i := range out.Scopes {
		leaf := &out.Scopes[i]
		if leaf.Health == "recovering" && leaf.IncidentID == nil {
			leaf.Health, leaf.UnknownReason = "unknown", "awaiting_data"
			leaf.ErrorRate, leaf.SuccessRate = nil, nil
		}
	}
	summaryLeaves := append([]ServiceStatusLeaf{}, out.Scopes...)
	for _, platform := range s.Platforms {
		leaves := []ServiceStatusLeaf{}
		for _, leaf := range out.Scopes {
			if leaf.Platform == platform.Platform {
				leaves = append(leaves, leaf)
			}
		}
		health, reason := ServiceStatusSummary(leaves)
		p := ServiceStatusPrivatePlatform{Platform: platform.Platform, Health: health, UnknownReason: reason}
		// 合计减去可见叶子会重建隐藏叶子，因此隐藏整个受影响平台的计数和比例。
		if !suppressedPlatforms[p.Platform] && platform.Qualified >= int64(minimum) {
			p.ServiceStatusPrivateMetrics = ServiceStatusPrivateMetrics{
				Success: &platform.Success, Failure: &platform.Failure, Excluded: &platform.Excluded,
				Unknown: &platform.Unknown, Qualified: &platform.Qualified,
				ErrorRate: platform.ErrorRate, SuccessRate: platform.SuccessRate,
			}
		} else {
			out.PrivacySuppressed = true
			p.Health, p.UnknownReason = "unknown", "insufficient_samples"
			summaryLeaves = append(summaryLeaves, ServiceStatusLeaf{Health: "unknown", UnknownReason: "insufficient_samples"})
		}
		out.Platforms = append(out.Platforms, p)
	}
	out.Health, out.UnknownReason = ServiceStatusSummary(summaryLeaves)
	// 仓储的全局缺口不能被展示投影抹掉。
	if s.Health == "unknown" && len(s.Gaps) > 0 {
		out.Health, out.UnknownReason = s.Health, s.UnknownReason
	}
	return out
}

func statusPrivateTime(at *time.Time) *time.Time {
	if at == nil {
		return nil
	}
	coarse := at.UTC().Truncate(time.Hour)
	return &coarse
}
