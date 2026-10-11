package admin

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type s33OpsDetailRepo struct {
	service.OpsRepository
	detail *service.OpsErrorLogDetail
	reads  int
}

func (r *s33OpsDetailRepo) GetErrorLogByID(context.Context, int64) (*service.OpsErrorLogDetail, error) {
	r.reads++
	return r.detail, nil
}

func TestOpsRoutingDiagnosticsPrivateResponseAndBoundedAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account, key, user := int64(17), int64(18), int64(19)
	repo := &s33OpsDetailRepo{detail: &service.OpsErrorLogDetail{OpsErrorLog: service.OpsErrorLog{
		CreatedAt: time.Now().UTC(), ID: 1, AccountID: &account, APIKeyID: &key, UserID: &user,
		AccountName: "private-account-canary", UserEmail: "private-user-canary", UpstreamEndpoint: "credential-domain-canary", Message: "raw-request-canary",
	}, ErrorBody: "raw-body-canary", APIKeyPrefix: "api-key-canary", UpstreamErrors: `[{"request":"raw-request-canary"}]`}}
	h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	r := gin.New()
	method := "jwt"
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", method)
	})
	r.GET("/diagnostics/:id", h.GetRoutingDiagnostics)
	request := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	w := request("/diagnostics/1")
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"data":null`)
	for _, field := range []string{"account", "api_key", "user_id", "canary", "request", "error_body"} {
		require.NotContains(t, w.Body.String(), field)
	}
	before := repo.reads
	for _, query := range []string{"format=csv", "range=90d", "group_id=1", "ids=1,2", "raw_request=canary", "export=json"} {
		require.Equal(t, 400, request("/diagnostics/1?"+query).Code)
	}
	require.Equal(t, before, repo.reads)
	method = "admin_api_key"
	require.Equal(t, 403, request("/diagnostics/1").Code)
	require.Equal(t, before, repo.reads)
	method = "jwt"
	repo.detail.CreatedAt = time.Now().UTC().Add(-31 * 24 * time.Hour)
	require.Equal(t, 404, request("/diagnostics/1").Code)
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
	repo := &s33OpsDetailRepo{detail: &service.OpsErrorLogDetail{OpsErrorLog: service.OpsErrorLog{CreatedAt: time.Now().UTC(), ID: 1, RequestID: "local-request", Message: "real failure"}, RoutingDiagnostics: d, UpstreamErrors: `[{"message":"attempt","future_field":1,"routing_diagnostics":` + string(encoded) + `}]`}}
	h := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", "jwt")
		c.Next()
	})
	r.GET("/errors/:id/routing-diagnostics", h.GetRoutingDiagnostics)
	r.GET("/errors/:id", h.GetErrorLogByID)
	r.GET("/request-errors/:id", h.GetRequestError)
	r.GET("/request-errors/:id/upstream-errors", h.ListRequestErrorUpstreamErrors)
	for _, path := range []string{"/errors/1/routing-diagnostics", "/errors/1", "/request-errors/1", "/request-errors/1/upstream-errors", "/request-errors/1/upstream-errors?include_detail=1", "/errors/1"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 200, w.Code, w.Body.String())
		if path == "/errors/1/routing-diagnostics" {
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
