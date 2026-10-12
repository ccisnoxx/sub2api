package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticsLimitsAcrossInstancesAndFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: r.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	first, second := NewPanelRateLimiter(client, nil), NewPanelRateLimiter(client, nil)
	userID := int64(17)
	identity := func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: userID})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", "jwt")
		c.Next()
	}
	routers := []*gin.Engine{gin.New(), gin.New()}
	for i, limiter := range []*PanelRateLimiter{first, second} {
		routers[i].Use(identity, limiter.Diagnostics())
		routers[i].GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	}
	for i := 0; i < 20; i++ {
		require.Equal(t, http.StatusOK, performPanelRequest(routers[i%2], "127.0.0.1:1234").Code)
	}
	denied := performPanelRequest(routers[0], "192.0.2.1:4321")
	require.Equal(t, http.StatusTooManyRequests, denied.Code)
	require.Equal(t, "60", denied.Header().Get("Retry-After"))
	userID = 18
	require.Equal(t, http.StatusOK, performPanelRequest(routers[0], "127.0.0.1:1234").Code)
	userID = 17
	r.FastForward(time.Minute)
	require.Equal(t, http.StatusOK, performPanelRequest(routers[1], "127.0.0.1:1234").Code)
	r.Close()
	require.Equal(t, http.StatusServiceUnavailable, performPanelRequest(routers[1], "127.0.0.1:1234").Code)
	nilRouter := gin.New()
	nilRouter.Use(identity, (*PanelRateLimiter)(nil).Diagnostics())
	nilRouter.GET("/test", func(c *gin.Context) { t.Fatal("缺限流 owner 不能放行") })
	require.Equal(t, http.StatusServiceUnavailable, performPanelRequest(nilRouter, "127.0.0.1:1234").Code)
	missingRedis := gin.New()
	missingRedis.Use(identity, NewPanelRateLimiter(nil, nil).Diagnostics())
	missingRedis.GET("/test", func(c *gin.Context) { t.Fatal("缺 Redis 不能放行") })
	require.Equal(t, http.StatusServiceUnavailable, performPanelRequest(missingRedis, "127.0.0.1:1234").Code)
}

func TestDiagnosticsAuditOmitsInvalidPathValuesOnFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			repo := &auditCaptureRepository{}
			svc := service.NewAuditLogService(repo, nil)
			svc.Start()
			router := gin.New()
			router.Use(gin.HandlerFunc(NewAuditLogMiddleware(svc)))
			router.GET("/api/v1/admin/ops/errors/:id/routing-diagnostics", func(c *gin.Context) { c.AbortWithStatus(status) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/errors/raw-request-canary/routing-diagnostics", nil))
			svc.Stop()
			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Len(t, repo.logs, 1)
			require.Equal(t, status, repo.logs[0].StatusCode)
			raw, err := json.Marshal(repo.logs)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "raw-request-canary")
			require.NotContains(t, string(raw), `"params"`)
		})
	}
}

func TestDiagnosticsAuditUsesSafeSummariesWithoutBodiesOrQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &auditCaptureRepository{}
	svc := service.NewAuditLogService(repo, nil)
	svc.Start()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 17})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Set("auth_method", "jwt")
		c.Next()
	}, gin.HandlerFunc(NewAuditLogMiddleware(svc)))
	router.PUT("/api/v1/admin/service-status/config", func(c *gin.Context) {
		SetAuditExtra(c, map[string]any{"enabled": false, "config_version": int64(2), "minimum_samples": 10})
		c.Status(http.StatusOK)
	})
	router.GET("/api/v1/admin/service-status/snapshot", func(c *gin.Context) {
		SetAuditExtra(c, map[string]any{"history_range": "24h", "privacy_suppressed": true})
		c.Status(http.StatusOK)
	})
	router.GET("/api/v1/admin/ops/errors/:id/routing-diagnostics", func(c *gin.Context) { c.Status(http.StatusBadRequest) })
	for _, req := range []*http.Request{
		httptest.NewRequest(http.MethodPut, "/api/v1/admin/service-status/config?raw_request=query-canary", strings.NewReader(`{"unexpected":"body-canary","api_key":"secret-canary"}`)),
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-status/snapshot?raw_request=query-canary", nil),
		httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/errors/1/routing-diagnostics?export=csv&raw_request=query-canary", nil),
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
	}
	svc.Stop()
	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.logs, 3)
	raw, err := json.Marshal(repo.logs)
	require.NoError(t, err)
	for _, canary := range []string{"body-canary", "secret-canary", "query-canary", "raw_request"} {
		require.NotContains(t, string(raw), canary)
	}
	require.Contains(t, string(raw), "admin.service_status.config.update")
	require.Contains(t, string(raw), "admin.service_status.snapshot.read")
	require.Contains(t, string(raw), "admin.diagnostics.export.denied")
	require.Contains(t, string(raw), `"config_version":2`)
	require.Contains(t, string(raw), `"privacy_suppressed":true`)
	require.Equal(t, http.StatusBadRequest, repo.logs[2].StatusCode)
}
