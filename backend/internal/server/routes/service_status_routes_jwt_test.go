//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type serviceStatusJWTStore struct {
	service.ServiceStatusRepository
	reads int
}

func (r *serviceStatusJWTStore) GetConfig(context.Context) (*service.ServiceStatusConfig, error) {
	r.reads++
	return &service.ServiceStatusConfig{Version: 1, Platforms: []string{"openai"}}, nil
}
func (r *serviceStatusJWTStore) UpdateConfig(_ context.Context, cfg *service.ServiceStatusConfig) (*service.ServiceStatusConfig, error) {
	r.reads++
	return cfg, nil
}
func (r *serviceStatusJWTStore) Snapshot(context.Context, string, string) (*service.ServiceStatusSnapshot, error) {
	r.reads++
	return &service.ServiceStatusSnapshot{SchemaVersion: 1, Health: "unknown", UnknownReason: "no_recent_requests"}, nil
}

func TestServiceStatusRoutesActualAdminJWTBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &routingDiagnosticsJWTUserRepo{users: map[int64]*service.User{
		1: {ID: 1, Role: service.RoleAdmin, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true},
		2: {ID: 2, Role: service.RoleUser, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true},
	}}
	auth := service.NewAuthService(nil, users, nil, nil, &config.Config{JWT: config.JWTConfig{Secret: "service-status-jwt-test-secret", ExpireHour: 1}}, nil, nil, nil, nil, nil, nil, nil, nil)
	tokenFor := func(id, version int64) string {
		user := *users.users[id]
		user.TokenVersion = version
		token, err := auth.GenerateToken(context.Background(), &user)
		require.NoError(t, err)
		return token
	}
	store := &serviceStatusJWTStore{}
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{}, ServiceStatus: handler.NewServiceStatusHandler(service.NewServiceStatusService(store))},
		middleware.NewAdminAuthMiddleware(auth, service.NewUserService(users, nil, nil, nil), nil, nil),
		middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		middleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, diagnosticsTestLimiter(t))
	validConfig := `{"version":1,"enabled":false,"platforms":[],"minimum_samples":5,"warning_error_rate":0.05,"outage_error_rate":0.9,"abnormal_windows":2,"recovery_windows":3}`
	for _, endpoint := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/service-status/config", ""},
		{http.MethodPut, "/api/v1/admin/service-status/config", validConfig},
		{http.MethodGet, "/api/v1/admin/service-status/snapshot", ""},
	} {
		for _, tc := range []struct {
			name, token string
			status      int
		}{
			{"未认证", "", http.StatusUnauthorized},
			{"篡改签名", tokenFor(1, 2) + "invalid", http.StatusUnauthorized},
			{"普通用户", tokenFor(2, 2), http.StatusForbidden},
			{"已撤销", tokenFor(1, 1), http.StatusUnauthorized},
			{"管理员", tokenFor(1, 2), http.StatusOK},
		} {
			t.Run(endpoint.method+endpoint.path+"/"+tc.name, func(t *testing.T) {
				before := store.reads
				req := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
				if tc.token != "" {
					req.Header.Set("Authorization", "Bearer "+tc.token)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				require.Equal(t, tc.status, rec.Code, rec.Body.String())
				if tc.status == http.StatusOK {
					require.Equal(t, before+1, store.reads)
				} else {
					require.Equal(t, before, store.reads)
				}
			})
		}
	}
	for _, query := range []string{"platform=composite", "platform=unknown", "range=90d", "group_id=1", "admin=true"} {
		before := store.reads
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-status/snapshot?"+query, nil)
		req.Header.Set("Authorization", "Bearer "+tokenFor(1, 2))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, before, store.reads)
	}
}
