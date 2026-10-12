//go:build unit

package routes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type featureFlagRouteSettings struct {
	service.SettingRepository
	mu     sync.Mutex
	values map[string]string
	fail   bool
}

func (r *featureFlagRouteSettings) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make(map[string]string)
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			values[key] = value
		}
	}
	return values, nil
}

func (r *featureFlagRouteSettings) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *featureFlagRouteSettings) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("测试保存失败")
	}
	r.values[key] = value
	return nil
}

type featureFlagRouteAudit struct {
	service.AuditLogRepository
	mu      sync.Mutex
	entries []*service.AuditLog
}

func (r *featureFlagRouteAudit) BatchInsert(_ context.Context, entries []*service.AuditLog) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entries...)
	return int64(len(entries)), nil
}

// 通过正式注册、JWT 与真实审计队列验证关开及失败交付，而非直接调用 handler。
func TestPersonalFeatureFlagsRoutingConfigJWTAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &routingDiagnosticsJWTUserRepo{users: map[int64]*service.User{
		1: {ID: 1, Role: service.RoleAdmin, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true},
		2: {ID: 2, Role: service.RoleUser, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true},
	}}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "personal-feature-jwt-test-secret", ExpireHour: 1}, Ops: config.OpsConfig{Enabled: true}}
	auth := service.NewAuthService(nil, users, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	tokenFor := func(id, version int64) string {
		user := *users.users[id]
		user.TokenVersion = version
		token, err := auth.GenerateToken(context.Background(), &user)
		require.NoError(t, err)
		return token
	}
	settings := &featureFlagRouteSettings{values: map[string]string{}}
	ops := service.NewOpsService(nil, settings, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	auditRepo := &featureFlagRouteAudit{}
	audit := service.NewAuditLogService(auditRepo, nil)
	audit.Start()
	t.Cleanup(audit.Stop)
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/v1"), &handler.Handlers{Admin: &handler.AdminHandlers{Ops: adminhandler.NewOpsHandler(ops)}},
		servermiddleware.NewAdminAuthMiddleware(auth, service.NewUserService(users, nil, nil, nil), nil, nil),
		servermiddleware.NewAuditLogMiddleware(audit), servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	const path = "/api/v1/admin/ops/routing-diagnostics/config"
	request := func(method, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, tc := range []struct {
			token  string
			status int
		}{
			{"", http.StatusUnauthorized},
			{tokenFor(1, 2) + "bad", http.StatusUnauthorized},
			{tokenFor(2, 2), http.StatusForbidden},
			{tokenFor(1, 1), http.StatusUnauthorized},
		} {
			w := request(method, tc.token, `{"enabled":false}`)
			require.Equal(t, tc.status, w.Code, w.Body.String())
		}
	}
	admin := tokenFor(1, 2)
	w := request(http.MethodGet, admin, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"enabled":true`)
	require.Contains(t, w.Body.String(), `"effective_enabled":true`)
	w = request(http.MethodPut, admin, `{}`)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = request(http.MethodPut, admin, `{"enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.False(t, ops.IsRoutingDiagnosticsEnabled(context.Background()))
	ops.SetMonitoringEnabled(false)
	settings.fail = true
	w = request(http.MethodPut, admin, `{"enabled":true}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
	w = request(http.MethodGet, admin, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"enabled":false`)
	settings.fail = false
	w = request(http.MethodPut, admin, `{"enabled":true}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"enabled":true`)
	require.Contains(t, w.Body.String(), `"effective_enabled":true`)
	require.False(t, ops.IsMonitoringEnabled(context.Background()), "独立诊断开关不能改写 Ops 监控设置")
	ops.SetMonitoringEnabled(true)
	require.True(t, ops.IsRoutingDiagnosticsEnabled(context.Background()))
	audit.Stop()
	auditRepo.mu.Lock()
	defer auditRepo.mu.Unlock()
	require.Len(t, auditRepo.entries, 4, "拒绝认证及普通 GET 不产生配置变更审计")
	for _, entry := range auditRepo.entries {
		require.Equal(t, path, entry.Path)
		require.Equal(t, http.MethodPut, entry.Method)
		require.Equal(t, int64(1), *entry.ActorUserID)
		require.Equal(t, service.RoleAdmin, entry.ActorRole)
		require.Contains(t, entry.Action, "routing_diagnostics")
		require.NotContains(t, entry.RequestBody, admin)
	}
	statuses := []int{}
	for _, entry := range auditRepo.entries {
		statuses = append(statuses, entry.StatusCode)
	}
	require.ElementsMatch(t, []int{http.StatusBadRequest, http.StatusOK, http.StatusServiceUnavailable, http.StatusOK}, statuses)
}
