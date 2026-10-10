package service

import "time"

// ServiceStatusConfig 仅表示管理员可编辑的独立配置；启用区间由仓储维护。
type ServiceStatusConfig struct {
	Version          int64    `json:"version"`
	Enabled          bool     `json:"enabled"`
	Platforms        []string `json:"platforms"`
	MinimumSamples   int      `json:"minimum_samples"`
	WarningErrorRate float64  `json:"warning_error_rate"`
	OutageErrorRate  float64  `json:"outage_error_rate"`
	AbnormalWindows  int      `json:"abnormal_windows"`
	RecoveryWindows  int      `json:"recovery_windows"`
}
type ServiceStatusCounts struct {
	Success  int64 `json:"success"`
	Failure  int64 `json:"failure"`
	Excluded int64 `json:"excluded"`
	Unknown  int64 `json:"unknown"`
}
type ServiceStatusMetrics struct {
	ServiceStatusCounts
	Qualified   int64    `json:"qualified"`
	ErrorRate   *float64 `json:"error_rate"`
	SuccessRate *float64 `json:"success_rate"`
}
type ServiceStatusScope struct {
	Platform       string `json:"platform"`
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	RequestedModel string `json:"requested_model"`
}
type ServiceStatusLeaf struct {
	ServiceStatusScope
	ServiceStatusMetrics
	Health         string     `json:"health"`
	UnknownReason  string     `json:"unknown_reason,omitempty"`
	LastEvidenceAt *time.Time `json:"last_evidence_at"`
	IncidentID     *string    `json:"incident_id"`
	IncidentPhase  *string    `json:"incident_phase"`
}
type ServiceStatusInterval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
type ServiceStatusCoverage struct {
	Platform string `json:"platform"`
	ServiceStatusInterval
}
type ServiceStatusHistoryPoint struct {
	ServiceStatusScope
	ServiceStatusMetrics
	ServiceStatusInterval
	Covered       bool   `json:"covered"`
	UnknownReason string `json:"unknown_reason,omitempty"`
}
type ServiceStatusIncidentUpdate struct {
	Phase string    `json:"phase"`
	At    time.Time `json:"at"`
}
type ServiceStatusIncident struct {
	ServiceStatusScope
	ID             string                        `json:"id"`
	Phase          string                        `json:"phase"`
	DetectedAt     time.Time                     `json:"detected_at"`
	LastEvidenceAt time.Time                     `json:"last_evidence_at"`
	LastAbnormalAt time.Time                     `json:"last_abnormal_at"`
	ResolvedAt     *time.Time                    `json:"resolved_at"`
	Updates        []ServiceStatusIncidentUpdate `json:"updates"`
}
type ServiceStatusPlatform struct {
	Platform      string `json:"platform"`
	Health        string `json:"health"`
	UnknownReason string `json:"unknown_reason,omitempty"`
	ServiceStatusMetrics
}
type ServiceStatusSnapshot struct {
	SchemaVersion            int                         `json:"schema_version"`
	ConfigVersion            int64                       `json:"config_version"`
	GeneratedAt              time.Time                   `json:"generated_at"`
	ObservedThrough          *time.Time                  `json:"observed_through"`
	Window                   ServiceStatusInterval       `json:"window"`
	HistoryRange             ServiceStatusInterval       `json:"history_range"`
	MonitoringEnabled        bool                        `json:"monitoring_enabled"`
	EnabledPlatforms         []string                    `json:"enabled_platforms"`
	Health                   string                      `json:"health"`
	UnknownReason            string                      `json:"unknown_reason,omitempty"`
	Coverage                 []ServiceStatusCoverage     `json:"coverage"`
	Gaps                     []string                    `json:"gaps"`
	Platforms                []ServiceStatusPlatform     `json:"platforms"`
	Scopes                   []ServiceStatusLeaf         `json:"scopes"`
	History                  []ServiceStatusHistoryPoint `json:"history"`
	Incidents                []ServiceStatusIncident     `json:"incidents"`
	OpenIncidentCount        int                         `json:"open_incident_count"`
	BoundedObservationNotice string                      `json:"bounded_observation_notice"`
}
