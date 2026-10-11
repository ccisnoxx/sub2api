package repository

import (
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 关联键仅存在单轮内存中，绝不写入匿名表。
type statusObservation = service.ServiceStatusObservation
type statusScope struct {
	platform string
	groupID  int64
	model    string
}
type statusSourceRow struct {
	table       string
	id          int64
	created     time.Time
	scope       statusScope
	visible     bool
	countTokens bool
	requestType int
	complete    *bool
	completion  string
	metadata    []byte
	observation *statusObservation
}
type statusNormalized struct {
	scope        statusScope
	at           time.Time
	outcome, gap string
}
type statusFact struct {
	scope                      statusScope
	bucket                     time.Time
	counts                     service.ServiceStatusCounts
	lastQualified, lastSuccess *time.Time
	gap                        string
}

func parseStatusObservation(raw []byte) *statusObservation {
	o, err := service.DecodeServiceStatusObservation(raw)
	if err != nil {
		return nil
	}
	return o
}
func statusReasonOutcome(reason string) string {
	switch reason {
	case "client_cancelled", "user_invalid_request", "user_context_limit", "user_model_unsupported", "user_group_access", "user_authentication", "user_quota", "user_rate_limit", "content_policy":
		return "excluded"
	case "provider_authentication", "provider_quota", "provider_capacity", "provider_5xx", "account_pool_unavailable", "service_timeout", "service_transport", "service_internal":
		return "failure"
	}
	return "unknown"
}
func statusTerminalOutcome(r statusSourceRow) (string, string) {
	o := r.observation
	kind := *o.TerminalKind
	if r.table == "usage_logs" {
		if r.completion != kind || (kind == "completed" && (r.complete == nil || !*r.complete)) || (kind != "completed" && r.complete != nil && *r.complete) {
			return "unknown", "terminal_conflict"
		}
	}
	switch kind {
	case "completed":
		if r.table == "usage_logs" {
			return "success", ""
		}
		return "unknown", "terminal_missing"
	case "client_disconnected", "user_rejected":
		return "excluded", ""
	case "upstream_error", "interrupted":
		if o.ReasonCode != nil {
			outcome := statusReasonOutcome(*o.ReasonCode)
			if outcome != "unknown" {
				return outcome, ""
			}
		}
	}
	return "unknown", "unclassified"
}
func sameTurn(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }

// 最近十分钟候选只向前关联额外九十分钟；created_at 不代替有效终态发生时间。
func normalizeStatusRows(rows []statusSourceRow, start, end time.Time, epochs map[string]time.Time) ([]statusNormalized, error) {
	linked := map[string][]statusSourceRow{}
	for _, r := range rows {
		if r.countTokens || r.requestType == 4 || r.requestType == 6 || r.requestType == 7 {
			continue
		}
		r.observation = parseStatusObservation(r.metadata)
		rowAt := r.created
		if r.observation != nil {
			rowAt = r.observation.ObservedAt
		}
		epoch, enabled := epochs[r.scope.platform]
		if r.scope.platform == "" {
			for _, e := range epochs {
				if epoch.IsZero() || e.Before(epoch) {
					epoch = e
				}
			}
			enabled = len(epochs) > 0
		}
		if !enabled || rowAt.Before(epoch) {
			continue
		}
		key := fmt.Sprintf("%s:%d", r.table, r.id)
		if r.observation != nil {
			key = r.observation.ObservationKey
		}
		linked[key] = append(linked[key], r)
	}
	result := make([]statusNormalized, 0)
	for _, group := range linked {
		sort.Slice(group, func(i, j int) bool { return group[i].created.Before(group[j].created) })
		chosen := group[len(group)-1]
		outcome, gap := "unknown", "linkage_unavailable"
		at := chosen.created
		if chosen.observation != nil {
			at = chosen.observation.ObservedAt
			var terminal *statusSourceRow
			conflict := false
			turn := chosen.observation.LogicalTurn
			for i := range group {
				r := &group[i]
				if !sameTurn(turn, r.observation.LogicalTurn) {
					conflict = true
				}
				if r.observation.EventRole != "terminal" {
					continue
				}
				if r.table == "usage_logs" {
					_, problem := statusTerminalOutcome(*r)
					if problem == "terminal_conflict" {
						conflict = true
					}
				}
				if terminal != nil && (*terminal.observation.TerminalKind != *r.observation.TerminalKind || !terminal.observation.ObservedAt.Equal(r.observation.ObservedAt) || terminal.scope != r.scope || !statusSameReason(terminal.observation.ReasonCode, r.observation.ReasonCode)) {
					conflict = true
				}
				// 用量可信完成优先于同步到 Ops 的相同终态，attempt 不参与归属冲突。
				if terminal == nil || r.table == "usage_logs" {
					terminal = r
				}
			}
			gap = "terminal_missing"
			if terminal != nil {
				chosen = *terminal
				at = chosen.observation.ObservedAt
				outcome, gap = statusTerminalOutcome(chosen)
			}
			if conflict {
				outcome, gap = "unknown", "terminal_conflict"
			}
		}
		if !at.Before(end) || at.Before(start) {
			continue
		}
		if !chosen.visible {
			// 删除/停用分组不进入外部范围；缺失分组仍作为全站完整性缺口。
			if chosen.scope.groupID != 0 {
				continue
			}
		}
		epoch, enabled := epochs[chosen.scope.platform]
		if chosen.scope.platform == "" {
			// 无法证明平台归属时只保留匿名完整性提示，不猜测路径或模型名。
			for _, e := range epochs {
				if epoch.IsZero() || e.Before(epoch) {
					epoch = e
				}
			}
			enabled = len(epochs) > 0
		}
		if !enabled || at.Before(epoch) {
			continue
		}
		if chosen.scope.platform == "" || chosen.scope.groupID == 0 || chosen.scope.model == "" {
			outcome, gap = "unknown", "scope_unavailable"
			if chosen.scope.model == "" {
				chosen.scope.model = "unknown"
			}
		}
		if chosen.requestType == 5 {
			outcome, gap = "unknown", "unsupported_entry"
		}
		result = append(result, statusNormalized{scope: chosen.scope, at: at, outcome: outcome, gap: gap})
		if len(result) > 50000 {
			return nil, service.ErrServiceStatusObservationLimit
		}
	}
	return result, nil
}
func statusSameReason(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func statusTimeMax(dst **time.Time, t time.Time) {
	if *dst == nil || t.After(**dst) {
		v := t
		*dst = &v
	}
}
func statusFacts(observations []statusNormalized) []statusFact {
	type key struct {
		scope  statusScope
		minute time.Time
	}
	m := map[key]*statusFact{}
	for _, o := range observations {
		k := key{o.scope, o.at.UTC().Truncate(time.Minute)}
		f := m[k]
		if f == nil {
			f = &statusFact{scope: o.scope, bucket: k.minute}
			m[k] = f
		}
		switch o.outcome {
		case "success":
			f.counts.Success++
			statusTimeMax(&f.lastQualified, o.at)
			statusTimeMax(&f.lastSuccess, o.at)
		case "failure":
			f.counts.Failure++
			statusTimeMax(&f.lastQualified, o.at)
		case "excluded":
			f.counts.Excluded++
		default:
			f.counts.Unknown++
		}
		if o.gap == "scope_unavailable" {
			f.gap = o.gap
		}
	}
	result := make([]statusFact, 0, len(m))
	for _, f := range m {
		result = append(result, *f)
	}
	return result
}
