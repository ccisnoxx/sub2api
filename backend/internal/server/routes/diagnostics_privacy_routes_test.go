//go:build unit

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type diagnosticsAdminKeySettings struct{ service.SettingRepository }

func (diagnosticsAdminKeySettings) GetValue(_ context.Context, key string) (string, error) {
	if key == service.SettingKeyAdminAPIKey {
		return "test-admin-key", nil
	}
	return "", service.ErrSettingNotFound
}

func (r *routingDiagnosticsJWTUserRepo) GetFirstAdmin(context.Context) (*service.User, error) {
	return r.users[1], nil
}

func requireLegacyDiagnosticDetailProtected(t *testing.T, encoded []byte) {
	t.Helper()
	var response struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(encoded, &response))
	require.NotContains(t, response.Data, "routing_diagnostics")
	require.JSONEq(t, "true", string(response.Data["routing_diagnostics_enabled"]))
	if raw, present := response.Data["upstream_errors"]; present {
		var serialized string
		require.NoError(t, json.Unmarshal(raw, &serialized))
		var events []map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(serialized), &events))
		for _, event := range events {
			require.NotContains(t, event, "routing_diagnostics")
		}
	}
}

func TestDiagnosticsRoutesPreserveLegacyAdminKeyWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &routingDiagnosticsJWTUserRepo{users: map[int64]*service.User{1: {ID: 1, Role: service.RoleAdmin, Status: service.StatusActive}}}
	repo := &routingDiagnosticsJWTOpsRepo{detail: &service.OpsErrorLogDetail{OpsErrorLog: service.OpsErrorLog{CreatedAt: time.Now().UTC(), Message: "existing detail"}, RoutingDiagnostics: &service.RoutingDiagnostics{SchemaVersion: 1}}}
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{
		Admin: &handler.AdminHandlers{Ops: adminhandler.NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))},
	}, middleware.NewAdminAuthMiddleware(nil, service.NewUserService(users, nil, nil, nil), service.NewSettingService(diagnosticsAdminKeySettings{}, nil), nil),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil, nil)
	for _, path := range []string{"/api/v1/admin/ops/errors/1", "/api/v1/admin/ops/request-errors/1", "/api/v1/admin/ops/upstream-errors/1"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("x-api-key", "test-admin-key")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), "existing detail")
		requireLegacyDiagnosticDetailProtected(t, recorder.Body.Bytes())
	}
	before := repo.reads
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/errors/1/routing-diagnostics", nil)
	request.Header.Set("x-api-key", "test-admin-key")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	require.Equal(t, before, repo.reads)
}

func diagnosticsTestLimiter(t *testing.T) *middleware.PanelRateLimiter {
	t.Helper()
	r := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: r.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return middleware.NewPanelRateLimiter(client, nil)
}

func TestDiagnosticsRoutesShareEnumerationBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &serviceStatusJWTStore{}
	repo := &routingDiagnosticsJWTOpsRepo{detail: &service.OpsErrorLogDetail{OpsErrorLog: service.OpsErrorLog{CreatedAt: time.Now().UTC()}}}
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{
		Admin:         &handler.AdminHandlers{Ops: adminhandler.NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))},
		ServiceStatus: handler.NewServiceStatusHandler(service.NewServiceStatusService(store)),
	}, middleware.AdminAuthMiddleware(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 17})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", "jwt")
		c.Next()
	}), middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil, diagnosticsTestLimiter(t))
	for i := 0; i < 20; i++ {
		path := "/api/v1/admin/service-status/config"
		if i%2 == 0 {
			path = "/api/v1/admin/ops/errors/1/routing-diagnostics"
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}
	before := store.reads + repo.reads
	for _, path := range []string{"/api/v1/admin/service-status/snapshot", "/api/v1/admin/service-status/config", "/api/v1/admin/ops/errors/2/routing-diagnostics"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusTooManyRequests, recorder.Code, recorder.Body.String())
		require.NotEmpty(t, recorder.Header().Get("Retry-After"))
	}
	require.Equal(t, before, store.reads+repo.reads, "超限不能触达仓储")
}
