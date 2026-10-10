package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type statusState struct {
	scope              statusScope
	lastEvidence       *time.Time
	lastObservation    time.Time
	abnormal, recovery int
	version            int64
	epoch              time.Time
	lastHealth         string
}
type statusWindow struct {
	counts             service.ServiceStatusCounts
	qualified, success *time.Time
	gap                string
}

func loadStatusIncidents(ctx context.Context, q statusQuerier, platforms []string, start, end time.Time, limitResolved bool) ([]service.ServiceStatusIncident, error) {
	query := `SELECT i.id,i.platform,i.group_id,g.name,i.requested_model,i.phase,i.detected_at,i.last_evidence_at,i.last_abnormal_at,i.resolved_at,i.updates FROM service_status_incidents i JOIN groups g ON g.id=i.group_id WHERE g.status='active' AND g.deleted_at IS NULL AND i.platform=ANY($1) AND i.resolved_at IS NULL ORDER BY i.detected_at DESC,i.id`
	result, err := queryStatusIncidents(ctx, q, query, pq.Array(platforms))
	if err != nil {
		return nil, err
	}
	query = `SELECT i.id,i.platform,i.group_id,g.name,i.requested_model,i.phase,i.detected_at,i.last_evidence_at,i.last_abnormal_at,i.resolved_at,i.updates FROM service_status_incidents i JOIN groups g ON g.id=i.group_id WHERE g.status='active' AND g.deleted_at IS NULL AND i.platform=ANY($1) AND i.resolved_at >= $2 AND i.detected_at < $3 ORDER BY i.resolved_at DESC,i.id`
	if limitResolved {
		query += ` LIMIT 200`
	}
	closed, err := queryStatusIncidents(ctx, q, query, pq.Array(platforms), start, end)
	if err != nil {
		return nil, err
	}
	return append(result, closed...), nil
}
func queryStatusIncidents(ctx context.Context, q statusQuerier, query string, args ...any) ([]service.ServiceStatusIncident, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []service.ServiceStatusIncident{}
	for rows.Next() {
		var i service.ServiceStatusIncident
		var updates []byte
		if err = rows.Scan(&i.ID, &i.Platform, &i.GroupID, &i.GroupName, &i.RequestedModel, &i.Phase, &i.DetectedAt, &i.LastEvidenceAt, &i.LastAbnormalAt, &i.ResolvedAt, &updates); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(updates, &i.Updates); err != nil {
			return nil, err
		}
		result = append(result, i)
	}
	return result, rows.Err()
}
func statusSetPhase(i *service.ServiceStatusIncident, phase string, at time.Time) {
	if i.Phase == phase {
		return
	}
	i.Phase = phase
	i.Updates = append(i.Updates, service.ServiceStatusIncidentUpdate{Phase: phase, At: at})
	if len(i.Updates) > 32 {
		i.Updates = append(i.Updates[:1:1], i.Updates[len(i.Updates)-31:]...)
	}
}
func writeStatusIncident(ctx context.Context, tx *sql.Tx, i *service.ServiceStatusIncident) error {
	updates, _ := json.Marshal(i.Updates)
	_, err := tx.ExecContext(ctx, `INSERT INTO service_status_incidents(id,platform,group_id,requested_model,phase,detected_at,last_evidence_at,last_abnormal_at,resolved_at,updates) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(id) DO UPDATE SET phase=EXCLUDED.phase,last_evidence_at=EXCLUDED.last_evidence_at,last_abnormal_at=EXCLUDED.last_abnormal_at,resolved_at=EXCLUDED.resolved_at,updates=EXCLUDED.updates`, i.ID, i.Platform, i.GroupID, i.RequestedModel, i.Phase, i.DetectedAt, i.LastEvidenceAt, i.LastAbnormalAt, i.ResolvedAt, string(updates))
	return err
}
func freezeStatusIncidents(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,phase,updates FROM service_status_incidents WHERE resolved_at IS NULL`)
	if err != nil {
		return err
	}
	var incidents []service.ServiceStatusIncident
	for rows.Next() {
		var i service.ServiceStatusIncident
		var raw []byte
		if err = rows.Scan(&i.ID, &i.Phase, &raw); err != nil {
			_ = rows.Close()
			return err
		}
		if err = json.Unmarshal(raw, &i.Updates); err != nil {
			_ = rows.Close()
			return err
		}
		incidents = append(incidents, i)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, i := range incidents {
		statusSetPhase(&i, "awaiting_data", now)
		raw, _ := json.Marshal(i.Updates)
		if _, err = tx.ExecContext(ctx, `UPDATE service_status_incidents SET phase=$1,updates=$2 WHERE id=$3`, i.Phase, string(raw), i.ID); err != nil {
			return err
		}
	}
	return nil
}

// 单个叶子唯一事件游标；重复扫描既不增加也不清零确认计数。
func statusTransition(state *statusState, incident *service.ServiceStatusIncident, w statusWindow, cfg service.ServiceStatusConfig, epoch, end time.Time, coverage bool) *service.ServiceStatusIncident {
	gap := w.gap
	if !coverage && gap == "" {
		gap = "coverage_gap"
	}
	_, health, _ := service.ServiceStatusEvaluate(w.counts, cfg, gap)
	if state.version != cfg.Version || !state.epoch.Equal(epoch) {
		state.abnormal, state.recovery = 0, 0
		// 切换区间只使用新区间证据；配置更新已在事务中冻结实际时间基线。
		if state.lastEvidence == nil || state.lastEvidence.Before(epoch) {
			baseline := epoch
			state.lastEvidence = &baseline
		}
		state.version, state.epoch = cfg.Version, epoch
	}
	if health == "unknown" {
		state.abnormal, state.recovery = 0, 0
		if incident != nil {
			statusSetPhase(incident, "awaiting_data", end)
		}
		state.lastHealth = health
		return incident
	}
	fresh := w.qualified != nil && (state.lastEvidence == nil || w.qualified.After(*state.lastEvidence))
	newSuccess := w.success != nil && (state.lastEvidence == nil || w.success.After(*state.lastEvidence)) && (incident == nil || w.success.After(incident.LastAbnormalAt))
	if !fresh || (health == "operational" && incident != nil && !newSuccess) {
		if incident != nil && health == "operational" && (state.lastHealth == "outage" || state.lastHealth == "degraded" || incident.Phase == "awaiting_data") {
			state.recovery = 0
			statusSetPhase(incident, "awaiting_data", end)
		}
		state.lastHealth = health
		return incident
	}
	evidence := *w.qualified
	if health == "operational" && incident != nil {
		evidence = *w.success
	}
	state.lastEvidence = &evidence
	if health == "outage" || health == "degraded" {
		state.abnormal++
		state.recovery = 0
		if incident == nil && state.abnormal >= cfg.AbnormalWindows {
			incident = &service.ServiceStatusIncident{ID: uuid.NewString(), ServiceStatusScope: service.ServiceStatusScope{Platform: state.scope.platform, GroupID: state.scope.groupID, RequestedModel: state.scope.model}, Phase: "detected", DetectedAt: evidence, LastEvidenceAt: evidence, LastAbnormalAt: evidence, Updates: []service.ServiceStatusIncidentUpdate{{Phase: "detected", At: evidence}}}
		} else if incident != nil {
			incident.LastEvidenceAt = evidence
			incident.LastAbnormalAt = evidence
			statusSetPhase(incident, "ongoing", evidence)
		}
	} else {
		state.abnormal = 0
		if incident != nil {
			state.recovery++
			incident.LastEvidenceAt = evidence
			statusSetPhase(incident, "recovering", evidence)
			if state.recovery >= cfg.RecoveryWindows {
				incident.ResolvedAt = &evidence
				statusSetPhase(incident, "resolved", evidence)
				state.recovery = 0
			}
		}
	}
	state.lastHealth = health
	return incident
}
func advanceStatusScopes(ctx context.Context, tx *sql.Tx, cfg statusConfig, end time.Time, gap string, coverage []service.ServiceStatusCoverage) error {
	facts, err := loadStatusFacts(ctx, tx, cfg.Platforms, end.Add(-5*time.Minute), end)
	if err != nil {
		return err
	}
	windows := map[statusScope]statusWindow{}
	states, err := loadStatusStates(ctx, tx, cfg.Platforms)
	if err != nil {
		return err
	}
	for scope := range states {
		windows[scope] = statusWindow{}
	}
	for _, f := range facts {
		if f.gap == "scope_unavailable" && gap == "" {
			gap = "scope_unavailable"
		}
		if f.scope.groupID == 0 || f.scope.platform == "" {
			if !f.bucket.Before(end.Add(-5*time.Minute)) && gap == "" {
				gap = "scope_unavailable"
			}
			continue
		}
		w := windows[f.scope]
		if !f.bucket.Before(end.Add(-5*time.Minute)) && !f.bucket.Before(cfg.epochs[f.scope.platform]) {
			addStatusCounts(&w.counts, f.counts)
			if f.lastQualified != nil {
				statusTimeMax(&w.qualified, *f.lastQualified)
			}
			if f.lastSuccess != nil {
				statusTimeMax(&w.success, *f.lastSuccess)
			}
			if f.gap != "" {
				w.gap = f.gap
			}
		}
		windows[f.scope] = w
	}
	incidents, err := loadStatusIncidents(ctx, tx, cfg.Platforms, end, end, false)
	if err != nil {
		return err
	}
	open := map[statusScope]*service.ServiceStatusIncident{}
	for j := range incidents {
		i := &incidents[j]
		if i.ResolvedAt == nil {
			scope := statusScope{i.Platform, i.GroupID, i.RequestedModel}
			open[scope] = i
			if _, ok := windows[scope]; !ok {
				windows[scope] = statusWindow{}
			}
		}
	}
	for scope, w := range windows {
		epoch := cfg.epochs[scope.platform]
		if !epoch.Before(end) {
			continue
		}
		s := states[scope]
		s.scope = scope
		if gap != "" {
			w.gap = gap
		}
		i := statusTransition(&s, open[scope], w, cfg.ServiceStatusConfig, epoch, end, statusCovered(coverage, scope.platform, end.Add(-5*time.Minute), end))
		if w.counts.Success+w.counts.Failure+w.counts.Excluded+w.counts.Unknown > 0 {
			s.lastObservation = end
		}
		if s.lastObservation.IsZero() {
			s.lastObservation = end
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO service_status_scope_states(platform,group_id,requested_model,last_evidence_at,last_observation_at,abnormal_count,recovery_count,config_version,observation_start,last_health) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(platform,group_id,requested_model) DO UPDATE SET last_evidence_at=EXCLUDED.last_evidence_at,last_observation_at=EXCLUDED.last_observation_at,abnormal_count=EXCLUDED.abnormal_count,recovery_count=EXCLUDED.recovery_count,config_version=EXCLUDED.config_version,observation_start=EXCLUDED.observation_start,last_health=EXCLUDED.last_health`, scope.platform, scope.groupID, scope.model, s.lastEvidence, s.lastObservation, s.abnormal, s.recovery, s.version, s.epoch, s.lastHealth)
		if err != nil {
			return err
		}
		if i != nil {
			if err = writeStatusIncident(ctx, tx, i); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadStatusStates(ctx context.Context, q statusQuerier, platforms []string) (map[statusScope]statusState, error) {
	rows, err := q.QueryContext(ctx, `SELECT s.platform,s.group_id,s.requested_model,s.last_evidence_at,s.last_observation_at,s.abnormal_count,s.recovery_count,s.config_version,s.observation_start,s.last_health FROM service_status_scope_states s JOIN groups g ON g.id=s.group_id WHERE g.status='active' AND g.deleted_at IS NULL AND s.platform=ANY($1)`, pq.Array(platforms))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	states := map[statusScope]statusState{}
	for rows.Next() {
		var s statusState
		if err = rows.Scan(&s.scope.platform, &s.scope.groupID, &s.scope.model, &s.lastEvidence, &s.lastObservation, &s.abnormal, &s.recovery, &s.version, &s.epoch, &s.lastHealth); err != nil {
			return nil, err
		}
		states[s.scope] = s
	}
	return states, rows.Err()
}
