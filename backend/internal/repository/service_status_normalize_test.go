package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func statusRow(key, kind, reason string, at time.Time) statusSourceRow {
	o := service.ServiceStatusObservation{SchemaVersion: 1, ObservationKey: key, EventRole: "terminal", TerminalKind: &kind, ObservedAt: at}
	if reason != "" {
		o.ReasonCode = &reason
	}
	raw, _ := json.Marshal(o)
	return statusSourceRow{table: "ops_error_logs", id: 1, created: at, scope: statusScope{"openai", 1, "gpt"}, visible: true, metadata: raw}
}
func statusNormalize(t *testing.T, rows ...statusSourceRow) []statusNormalized {
	t.Helper()
	at := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	out, err := normalizeStatusRows(rows, at.Add(-10*time.Minute), at.Add(time.Minute), map[string]time.Time{"openai": at.Add(-time.Hour)})
	require.NoError(t, err)
	return out
}
func TestServiceStatusNormalizeTerminalPriorityAndZeroCostSuccess(t *testing.T) {
	at := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	key := uuid.NewString()
	terminal := statusRow(key, "completed", "", at)
	terminal.table = "usage_logs"
	terminal.completion = "completed"
	complete := true
	terminal.complete = &complete
	attempt := statusRow(key, "upstream_error", "provider_capacity", at.Add(-time.Second))
	var o service.ServiceStatusObservation
	require.NoError(t, json.Unmarshal(attempt.metadata, &o))
	o.EventRole = "attempt"
	o.TerminalKind = nil
	attempt.metadata, _ = json.Marshal(o)
	out := statusNormalize(t, attempt, terminal, terminal)
	require.Len(t, out, 1)
	require.Equal(t, "success", out[0].outcome)
	terminal.completion = "client_disconnected"
	terminal.complete = nil
	k := "client_disconnected"
	o.TerminalKind = &k
	o.EventRole = "terminal"
	o.ReasonCode = nil
	o.ObservedAt = at
	terminal.metadata, _ = json.Marshal(o)
	out = statusNormalize(t, attempt, terminal)
	require.Equal(t, "excluded", out[0].outcome)
	out = statusNormalize(t, attempt)
	require.Equal(t, "unknown", out[0].outcome)
	require.Equal(t, "terminal_missing", out[0].gap)
}
func TestServiceStatusNormalizeConflictAndHistoricalKeys(t *testing.T) {
	at := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	key := uuid.NewString()
	a := statusRow(key, "upstream_error", "provider_5xx", at)
	b := statusRow(key, "completed", "", at)
	out := statusNormalize(t, a, b)
	require.Len(t, out, 1)
	require.Equal(t, "terminal_conflict", out[0].gap)
	b = a
	var o service.ServiceStatusObservation
	require.NoError(t, json.Unmarshal(b.metadata, &o))
	turn := 2
	o.LogicalTurn = &turn
	b.metadata, _ = json.Marshal(o)
	out = statusNormalize(t, a, b)
	require.Equal(t, "terminal_conflict", out[0].gap)
	a.metadata = nil
	b.metadata = nil
	b.table = "usage_logs"
	out = statusNormalize(t, a, b)
	require.Len(t, out, 2)
	for _, v := range out {
		require.Equal(t, "linkage_unavailable", v.gap)
	}
	b = a
	b.metadata = []byte(`{"schema_version":2}`)
	b.id = 2
	out = statusNormalize(t, a, b)
	require.Len(t, out, 2)
}
func TestServiceStatusNormalizeReasonPairsAndUnsupported(t *testing.T) {
	at := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	for _, tt := range []struct{ reason, want string }{{"user_authentication", "excluded"}, {"provider_authentication", "failure"}, {"user_quota", "excluded"}, {"provider_quota", "failure"}, {"user_rate_limit", "excluded"}, {"provider_capacity", "failure"}, {"unclassified", "unknown"}} {
		out := statusNormalize(t, statusRow(uuid.NewString(), "upstream_error", tt.reason, at))
		require.Equal(t, tt.want, out[0].outcome)
	}
	for _, typ := range []int{4, 6, 7} {
		r := statusRow(uuid.NewString(), "upstream_error", "provider_5xx", at)
		r.requestType = typ
		require.Empty(t, statusNormalize(t, r))
	}
	r := statusRow(uuid.NewString(), "upstream_error", "provider_5xx", at)
	r.requestType = 5
	require.Equal(t, "unsupported_entry", statusNormalize(t, r)[0].gap)
	r.requestType = 1
	r.scope.platform = ""
	require.Equal(t, "scope_unavailable", statusNormalize(t, r)[0].gap)
}
func TestServiceStatusNormalizeObservedAtEpochAndLimit(t *testing.T) {
	at := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	row := statusRow(uuid.NewString(), "upstream_error", "provider_5xx", at.Add(-11*time.Minute))
	row.created = at
	require.Empty(t, statusNormalize(t, row))
	var attempt service.ServiceStatusObservation
	require.NoError(t, json.Unmarshal(row.metadata, &attempt))
	attempt.EventRole = "attempt"
	attempt.TerminalKind = nil
	row.metadata, _ = json.Marshal(attempt)
	require.Empty(t, statusNormalize(t, row))
	row = statusRow(uuid.NewString(), "upstream_error", "provider_5xx", at)
	row.created = at.Add(-80 * time.Minute)
	require.Len(t, statusNormalize(t, row), 1)
	rows := make([]statusSourceRow, 50001)
	for j := range rows {
		rows[j] = statusSourceRow{table: "usage_logs", id: int64(j + 1), created: at, scope: statusScope{"openai", 1, "gpt"}, visible: true}
	}
	out, err := normalizeStatusRows(rows, at.Add(-time.Minute), at.Add(time.Minute), map[string]time.Time{"openai": at.Add(-time.Hour)})
	require.Nil(t, out)
	require.ErrorIs(t, err, service.ErrServiceStatusObservationLimit)
}

func TestServiceStatusIncidentNeedsNewEvidenceAndThreeNewRecoveries(t *testing.T) {
	base := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	cfg := service.ServiceStatusConfig{Version: 1, MinimumSamples: 5, WarningErrorRate: .05, OutageErrorRate: .9, AbnormalWindows: 2, RecoveryWindows: 3}
	s := statusState{scope: statusScope{"openai", 1, "gpt"}}
	epoch := base.Add(-time.Hour)
	w := statusWindow{counts: service.ServiceStatusCounts{Failure: 5}, qualified: &base}
	i := statusTransition(&s, nil, w, cfg, epoch, base.Add(time.Minute), true)
	require.Nil(t, i)
	require.Equal(t, 1, s.abnormal)
	i = statusTransition(&s, i, w, cfg, epoch, base.Add(2*time.Minute), true)
	require.Nil(t, i)
	require.Equal(t, 1, s.abnormal)
	second := base.Add(time.Minute)
	w.qualified = &second
	i = statusTransition(&s, i, w, cfg, epoch, base.Add(2*time.Minute), true)
	require.NotNil(t, i)
	require.Equal(t, "detected", i.Phase)
	oldSuccess := base.Add(-time.Minute)
	w = statusWindow{counts: service.ServiceStatusCounts{Success: 5}, qualified: &oldSuccess, success: &oldSuccess}
	i = statusTransition(&s, i, w, cfg, epoch, base.Add(3*time.Minute), true)
	require.Equal(t, "awaiting_data", i.Phase)
	require.Nil(t, i.ResolvedAt)
	for j := 0; j < 3; j++ {
		at := base.Add(time.Duration(j+3) * time.Minute)
		w.qualified, w.success = &at, &at
		i = statusTransition(&s, i, w, cfg, epoch, at.Add(time.Minute), true)
		if j < 2 {
			require.Equal(t, "recovering", i.Phase)
			require.Equal(t, j+1, s.recovery)
			i = statusTransition(&s, i, w, cfg, epoch, at.Add(time.Minute), true)
			require.Equal(t, j+1, s.recovery)
		}
	}
	require.Equal(t, "resolved", i.Phase)
	require.Equal(t, base.Add(5*time.Minute), *i.ResolvedAt)
}
func TestServiceStatusIncidentUnknownConfigAndAbnormalResetRecovery(t *testing.T) {
	base := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	cfg := service.ServiceStatusConfig{Version: 1, MinimumSamples: 5, WarningErrorRate: .05, OutageErrorRate: .9, AbnormalWindows: 1, RecoveryWindows: 3}
	epoch := base.Add(-time.Hour)
	s := statusState{scope: statusScope{"openai", 1, "gpt"}}
	i := statusTransition(&s, nil, statusWindow{counts: service.ServiceStatusCounts{Failure: 5}, qualified: &base}, cfg, epoch, base, true)
	at := base.Add(time.Minute)
	w := statusWindow{counts: service.ServiceStatusCounts{Success: 5}, qualified: &at, success: &at}
	i = statusTransition(&s, i, w, cfg, epoch, at, true)
	require.Equal(t, 1, s.recovery)
	at = at.Add(time.Minute)
	i = statusTransition(&s, i, statusWindow{counts: service.ServiceStatusCounts{Failure: 5}, qualified: &at}, cfg, epoch, at, true)
	require.Equal(t, "ongoing", i.Phase)
	require.Zero(t, s.recovery)
	i = statusTransition(&s, i, statusWindow{counts: service.ServiceStatusCounts{Success: 4}, qualified: &at, success: &at}, cfg, epoch, at, true)
	require.Equal(t, "awaiting_data", i.Phase)
	require.Zero(t, s.recovery)
	cfg.Version = 2
	cfg.WarningErrorRate = .8
	i = statusTransition(&s, i, w, cfg, epoch, at, true)
	require.Equal(t, "awaiting_data", i.Phase)
	require.Nil(t, i.ResolvedAt)
	for j := 0; j < 100; j++ {
		statusSetPhase(i, []string{"ongoing", "recovering"}[j%2], at.Add(time.Duration(j)*time.Second))
	}
	require.Len(t, i.Updates, 32)
	require.Equal(t, "detected", i.Updates[0].Phase)
}
func TestServiceStatusCoverageLeavesDisabledAndFailureIntervalsBlank(t *testing.T) {
	base := time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)
	epochs := map[string]time.Time{"openai": base}
	c := updateStatusCoverage(nil, epochs, base, base.Add(10*time.Minute), base.Add(-time.Hour), true)
	delete(epochs, "openai")
	c = updateStatusCoverage(c, epochs, base.Add(10*time.Minute), base.Add(20*time.Minute), base.Add(-time.Hour), true)
	epochs["openai"] = base.Add(20 * time.Minute)
	c = updateStatusCoverage(c, epochs, base.Add(20*time.Minute), base.Add(30*time.Minute), base.Add(-time.Hour), true)
	require.False(t, statusCovered(c, "openai", base.Add(10*time.Minute), base.Add(20*time.Minute)))
	require.Len(t, c, 2)
	c = updateStatusCoverage(c, epochs, base.Add(25*time.Minute), base.Add(30*time.Minute), base.Add(-time.Hour), false)
	require.False(t, statusCovered(c, "openai", base.Add(20*time.Minute), base.Add(30*time.Minute)))
}
