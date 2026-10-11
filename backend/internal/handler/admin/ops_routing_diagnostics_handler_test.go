package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type s33OpsDetailRepo struct {
	service.OpsRepository
	detail *service.OpsErrorLogDetail
}

func (r *s33OpsDetailRepo) GetErrorLogByID(context.Context, int64) (*service.OpsErrorLogDetail, error) {
	return r.detail, nil
}
func (r *s33OpsDetailRepo) ListErrorLogs(context.Context, *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	return &service.OpsErrorLogList{Errors: []*service.OpsErrorLog{&r.detail.OpsErrorLog}, Total: 1, Page: 1, PageSize: 10}, nil
}

func TestOpsRoutingDiagnosticsAdminDetailAndListBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pool, filtered := 0, 0
	layer, reason := "load_balance", "pool_empty"
	d := &service.RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: 1, SelectionLayer: &layer, SelectionReason: &reason, CandidatePool: &pool, FilteredCandidates: &filtered, FilterReasons: map[string]int{}, FilterCoverage: "complete"}
	encoded, err := json.Marshal(d)
	require.NoError(t, err)
	repo := &s33OpsDetailRepo{detail: &service.OpsErrorLogDetail{OpsErrorLog: service.OpsErrorLog{ID: 1, RequestID: "local-request", Message: "real failure"}, RoutingDiagnostics: d, UpstreamErrors: `[{"message":"attempt","future_field":1,"routing_diagnostics":` + string(encoded) + `}]`}}
	h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	r := gin.New()
	r.GET("/errors/:id", h.GetErrorLogByID)
	r.GET("/request-errors/:id", h.GetRequestError)
	r.GET("/request-errors/:id/upstream-errors", h.ListRequestErrorUpstreamErrors)
	for _, path := range []string{"/errors/1", "/request-errors/1", "/request-errors/1/upstream-errors", "/request-errors/1/upstream-errors?include_detail=1", "/errors/1"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 200, w.Code, w.Body.String())
		if path == "/errors/1" || path == "/request-errors/1" {
			require.Contains(t, w.Body.String(), `"candidate_pool":0`)
			require.Contains(t, w.Body.String(), `"filter_reasons":{}`)
		} else {
			require.NotContains(t, w.Body.String(), "routing_diagnostics")
			require.NotContains(t, w.Body.String(), "candidate_pool")
		}
		if path == "/request-errors/1/upstream-errors?include_detail=1" {
			require.Contains(t, w.Body.String(), "future_field")
			require.Contains(t, w.Body.String(), "real failure")
		}
	}
	require.Same(t, d, repo.detail.RoutingDiagnostics, "列表裁剪不得修改详情owner")
}
