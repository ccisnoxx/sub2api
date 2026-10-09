package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func s33HandlerRoutingSnapshot(attempt int) *service.RoutingDiagnostics {
	pool, count := 3, 1
	layer, reason := "load_balance", "slot_acquired"
	return &service.RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: attempt, SelectionLayer: &layer, SelectionReason: &reason, CandidatePool: &pool, FilteredCandidates: &count, FilterReasons: map[string]int{"excluded": 1}, FilterCoverage: "partial"}
}

func TestOpsRoutingDiagnosticsQueuedCopyAndBudget(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	d := s33HandlerRoutingSnapshot(1)
	entry := &service.OpsInsertErrorLogInput{ErrorMessage: "before", RoutingDiagnostics: d, UpstreamErrors: []*service.OpsUpstreamErrorEvent{{Message: "failed", UpstreamStatusCode: 502, RoutingDiagnostics: d}}}
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	enqueueOpsErrorLog(ops, entry)
	entry.ErrorMessage = "after"
	*d.CandidatePool = 99
	d.FilterReasons["excluded"] = 99
	job := <-opsErrorLogQueue
	require.NotSame(t, entry, job.entry)
	require.Equal(t, "before", job.entry.ErrorMessage)
	decoded, err := service.DecodeRoutingDiagnosticsJSON([]byte(*job.entry.RoutingDiagnosticsJSON))
	require.NoError(t, err)
	require.Equal(t, 3, *decoded.CandidatePool)
	require.Equal(t, 1, decoded.FilterReasons["excluded"])
	events, err := service.ParseOpsUpstreamErrors(*job.entry.UpstreamErrorsJSON)
	require.NoError(t, err)
	require.Equal(t, decoded, events[0].RoutingDiagnostics)
	copied := *job.entry
	copied.RoutingDiagnosticsJSON = nil
	require.Equal(t, int64(len(*job.entry.RoutingDiagnosticsJSON)), estimateOpsErrorLogJobBytes(job.entry)-estimateOpsErrorLogJobBytes(&copied))
}

func TestOpsRoutingDiagnosticsFinalOwnerUsesFailedAttempt(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	first := s33HandlerRoutingSnapshot(1)
	later := s33HandlerRoutingSnapshot(2)
	events := []*service.OpsUpstreamErrorEvent{{Message: "failed provider", UpstreamStatusCode: 502, RoutingDiagnostics: first}}
	c.Set(service.OpsUpstreamErrorsKey, events)
	c.Set(service.OpsRoutingDiagnosticsAttemptKey, later)
	entry := &service.OpsInsertErrorLogInput{}
	applyOpsUpstreamFieldsFromContext(c, entry)
	require.Equal(t, first, entry.RoutingDiagnostics)
	*first.CandidatePool = 99
	require.Equal(t, 3, *entry.RoutingDiagnostics.CandidatePool)
	snapshot := service.OpsStreamError{Turn: 1, RoutingDiagnostics: later, UpstreamErrors: events}
	applyOpsStreamErrorSnapshot(entry, snapshot)
	require.Equal(t, 1, entry.RoutingDiagnostics.SelectionAttempt)
	require.Equal(t, "upstream", entry.ErrorPhase)
	// 历史未绑定事件保持未知，不由当前发送状态补回。
	events[0].RoutingDiagnostics = nil
	applyOpsUpstreamFieldsFromContext(c, entry)
	require.Nil(t, entry.RoutingDiagnostics)
}
