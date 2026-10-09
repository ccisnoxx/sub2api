package service

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func s33EmptyRoutingSnapshot() *RoutingDiagnostics {
	return &RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: 1, SelectionLayer: routingValue("load_balance"), SelectionReason: routingValue("pool_empty"), CandidatePool: routingValue(0), FilteredCandidates: routingValue(0), FilterReasons: map[string]int{}, FilterCoverage: "complete"}
}

func TestOpsRoutingDiagnosticsValidationAndNullContract(t *testing.T) {
	valid := s33EmptyRoutingSnapshot()
	require.NoError(t, valid.Validate())
	encoded, err := json.Marshal(valid)
	require.NoError(t, err)
	decoded, err := DecodeRoutingDiagnosticsJSON(encoded)
	require.NoError(t, err)
	require.Equal(t, valid, decoded)
	decoded, err = DecodeRoutingDiagnosticsJSON([]byte("null"))
	require.NoError(t, err)
	require.Nil(t, decoded)
	mutations := map[string]func(*RoutingDiagnostics){
		"version":          func(d *RoutingDiagnostics) { d.SchemaVersion = 2 },
		"negative":         func(d *RoutingDiagnostics) { d.CandidatePool = routingValue(-1) },
		"precision":        func(d *RoutingDiagnostics) { d.SelectionAttempt = 1 << 53 },
		"turn":             func(d *RoutingDiagnostics) { d.Turn = routingValue(0) },
		"layer":            func(d *RoutingDiagnostics) { d.SelectionLayer = routingValue("private-value") },
		"reason":           func(d *RoutingDiagnostics) { d.SelectionReason = routingValue("private-value") },
		"reason_layer":     func(d *RoutingDiagnostics) { d.SelectionReason = routingValue("sticky_hit") },
		"sum":              func(d *RoutingDiagnostics) { d.FilterReasons = map[string]int{"excluded": 1} },
		"unknown_filter":   func(d *RoutingDiagnostics) { d.FilterReasons = map[string]int{"private-value": 1} },
		"zero_filter":      func(d *RoutingDiagnostics) { d.FilterReasons = map[string]int{"excluded": 0} },
		"missing_reasons":  func(d *RoutingDiagnostics) { d.FilterReasons = nil },
		"incomplete_empty": func(d *RoutingDiagnostics) { d.FilterCoverage = "partial" },
		"unobserved_count": func(d *RoutingDiagnostics) { d.SelectionReason = nil; d.FilterCoverage = "unobserved" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) { d := valid.Clone(); mutate(d); require.Error(t, d.Validate()) })
	}
	for _, raw := range []string{
		strings.Replace(string(encoded), `"schema_version":1`, `"schema_version":1,"credentials":"private-value"`, 1),
		strings.Replace(string(encoded), `"turn":null,`, "", 1),
		strings.Replace(string(encoded), `"candidate_pool":0`, `"candidate_pool":0.5`, 1),
		strings.Replace(string(encoded), `"selection_attempt":1`, `"selection_attempt":9007199254740992`, 1),
	} {
		_, err := DecodeRoutingDiagnosticsJSON([]byte(raw))
		require.Error(t, err)
		require.NotContains(t, err.Error(), "private-value")
	}
	partial := &RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: 2, FilterCoverage: "partial", FilteredCandidates: routingValue(1), FilterReasons: map[string]int{"excluded": 1}}
	require.NoError(t, partial.Validate())
	unknown := &RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: 3, FilterCoverage: "unobserved"}
	require.NoError(t, unknown.Validate())
}

func TestOpsRoutingDiagnosticsAttemptBindingAndTurnSnapshots(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	c.Request = c.Request.WithContext(WithRoutingDiagnosticsRequest(c.Request.Context(), 0))
	_, b := beginRoutingDiagnosticsSelection(c.Request.Context())
	b.observePool([]Account{{ID: 1}, {ID: 2}})
	b.reject(1, "excluded")
	b.pass(2)
	_, first, err := finishRoutingDiagnosticsSelection(b, &AccountSelectionResult{Acquired: true}, "load_balance", nil)
	require.NoError(t, err)
	BindOpsRoutingDiagnosticsAttempt(c)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{UpstreamStatusCode: 502, Message: "first"})
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{UpstreamStatusCode: 502, Message: "same account retry"})
	_, next := beginRoutingDiagnosticsSelection(c.Request.Context())
	next.outcome("channel_pricing", "channel_pricing_restricted")
	_, _, _ = finishRoutingDiagnosticsSelection(next, nil, "", ErrNoAvailableAccounts)
	// 失败事件属于已绑定的发送，不借后来重选的未知池。
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{UpstreamStatusCode: 502, Message: "final provider owner"})
	events, _ := c.Get(OpsUpstreamErrorsKey)
	for _, event := range events.([]*OpsUpstreamErrorEvent) {
		require.Equal(t, first, event.RoutingDiagnostics)
	}
	MarkOpsStreamFailure(c, "upstream_error", "upstream_error", "failure", 502)
	snapshot, ok := GetOpsStreamError(c)
	require.True(t, ok)
	require.Equal(t, first, snapshot.UpstreamErrors[2].RoutingDiagnostics)
	snapshot.UpstreamErrors[0].RoutingDiagnostics.FilterReasons["excluded"] = 99
	again, _ := GetOpsStreamError(c)
	require.Equal(t, 1, again.UpstreamErrors[0].RoutingDiagnostics.FilterReasons["excluded"])

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest("GET", "/v1/responses", nil)
	SetOpenAIClientTransport(c2, OpenAIClientTransportWS)
	BeginOpsStreamTurn(c2, 1)
	_, turn1 := beginRoutingDiagnosticsSelection(c2.Request.Context())
	turn1.observePool(nil)
	_, _, _ = finishRoutingDiagnosticsSelection(turn1, nil, "load_balance", ErrNoAvailableAccounts)
	BindOpsRoutingDiagnosticsAttempt(c2)
	// 同一逻辑 turn 的 Proxy 重启保留新选号已绑定的发送快照。
	BeginOpsStreamTurnWithRoutingTurn(c2, 1, 1)
	appendOpsUpstreamError(c2, OpsUpstreamErrorEvent{UpstreamStatusCode: 503, Message: "turn one"})
	MarkOpsStreamFailure(c2, "upstream_error", "upstream_error", "turn one", 503)
	BeginOpsStreamTurn(c2, 2)
	appendOpsUpstreamError(c2, OpsUpstreamErrorEvent{UpstreamStatusCode: 502, Message: "connection reuse"})
	MarkOpsStreamFailure(c2, "upstream_error", "upstream_error", "turn two", 502)
	turns := GetOpsStreamErrors(c2)
	require.Len(t, turns, 2)
	require.Equal(t, 1, *turns[0].RoutingDiagnostics.Turn)
	require.Nil(t, turns[1].RoutingDiagnostics)
	require.Nil(t, turns[1].UpstreamErrors[0].RoutingDiagnostics)
	BeginOpsStreamTurn(c2, 3)
	_, turn3 := beginRoutingDiagnosticsSelection(c2.Request.Context())
	_, _, _ = finishRoutingDiagnosticsSelection(turn3, &AccountSelectionResult{Acquired: true}, "session_hash", nil)
	MarkOpsStreamErrorValue(c2, OpsStreamError{ErrType: "invalid_request_error", Message: "policy", IntendedStatus: 400, RequestScoped: true, RoutingDiagnostics: first})
	turns = GetOpsStreamErrors(c2)
	require.Nil(t, turns[2].RoutingDiagnostics)
	require.Nil(t, turns[2].UpstreamErrors)
}

func TestOpsRoutingDiagnosticsQueueServiceAndInvalidObject(t *testing.T) {
	d := s33EmptyRoutingSnapshot()
	entry := &OpsInsertErrorLogInput{ErrorMessage: "real failure", RoutingDiagnostics: d, UpstreamErrors: []*OpsUpstreamErrorEvent{{UpstreamStatusCode: 502, Message: "failure", RoutingDiagnostics: d}}}
	require.NoError(t, SanitizeOpsUpstreamErrorsForQueue(entry))
	require.Nil(t, entry.RoutingDiagnostics)
	*d.CandidatePool = 99
	d.FilterReasons["excluded"] = 99
	var got *OpsInsertErrorLogInput
	svc := NewOpsService(&opsRepoMock{InsertErrorLogFn: func(_ context.Context, e *OpsInsertErrorLogInput) (int64, error) { got = e; return 1, nil }}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, svc.RecordError(context.Background(), entry))
	require.NotNil(t, got)
	decoded, err := DecodeRoutingDiagnosticsJSON([]byte(*got.RoutingDiagnosticsJSON))
	require.NoError(t, err)
	require.Equal(t, 0, *decoded.CandidatePool)
	require.Empty(t, decoded.FilterReasons)
	events, err := ParseOpsUpstreamErrors(*got.UpstreamErrorsJSON)
	require.NoError(t, err)
	require.Equal(t, 0, *events[0].RoutingDiagnostics.CandidatePool)
	bad := s33EmptyRoutingSnapshot()
	bad.FilterReasons["secret-code"] = 1
	require.NoError(t, svc.RecordError(context.Background(), &OpsInsertErrorLogInput{ErrorMessage: "preserve failure", RoutingDiagnostics: bad, UpstreamErrors: []*OpsUpstreamErrorEvent{{UpstreamStatusCode: 502, Message: "preserve attempt", RoutingDiagnostics: bad}}}))
	require.Equal(t, "preserve failure", got.ErrorMessage)
	require.Nil(t, got.RoutingDiagnosticsJSON)
	require.NotContains(t, *got.UpstreamErrorsJSON, "secret-code")
	require.NotContains(t, *got.UpstreamErrorsJSON, "routing_diagnostics")
}

func TestOpsRoutingDiagnosticsHistoryAndListProjection(t *testing.T) {
	raw, err := json.Marshal(s33EmptyRoutingSnapshot())
	require.NoError(t, err)
	source := `[{"message":"real failure","future_field":1,"routing_diagnostics":` + string(raw) + `}]`
	normalized, err := normalizeOpsUpstreamErrorsJSON(source)
	require.NoError(t, err)
	require.Contains(t, normalized, "routing_diagnostics")
	list, err := OpsUpstreamErrorsWithoutRoutingDiagnostics(normalized)
	require.NoError(t, err)
	require.NotContains(t, list, "routing_diagnostics")
	require.Contains(t, list, "future_field")
	invalid := strings.Replace(source, `"schema_version":1`, `"schema_version":2,"secret":"value"`, 1)
	normalized, err = normalizeOpsUpstreamErrorsJSON(invalid)
	require.NoError(t, err)
	require.NotContains(t, normalized, "routing_diagnostics")
	require.Contains(t, normalized, "real failure")
}

func TestOpsRoutingDiagnosticsTurnOwnerCannotReuseConnectionBinding(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil)
	c.Request = c.Request.WithContext(WithRoutingDiagnosticsRequest(c.Request.Context(), 0))
	_, b := beginRoutingDiagnosticsSelection(c.Request.Context())
	b.observePool([]Account{{ID: 1}})
	b.pass(1)
	_, _, _ = finishRoutingDiagnosticsSelection(b, &AccountSelectionResult{Acquired: true}, "load_balance", nil)
	BindOpsRoutingDiagnosticsAttempt(c)
	// beginProxy 先改变 owner，随后 BeforeRequest 进入同一 turn；后者不能借建连绑定。
	c.Request = c.Request.WithContext(EnsureRoutingDiagnosticsTurn(c.Request.Context(), 1))
	BeginOpsStreamTurnWithRoutingTurn(c, 1, 1)
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{UpstreamStatusCode: 502, Message: "reused connection"})
	events, _ := c.Get(OpsUpstreamErrorsKey)
	require.Nil(t, events.([]*OpsUpstreamErrorEvent)[0].RoutingDiagnostics)
}
