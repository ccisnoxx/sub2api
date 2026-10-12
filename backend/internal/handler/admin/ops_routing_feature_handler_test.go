package admin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type routingFeatureHandlerSettings struct {
	service.SettingRepository
	values   map[string]string
	setError error
}

type routingFeatureHandlerErrorRepo struct {
	service.OpsRepository
	entries chan *service.OpsInsertErrorLogInput
}

func (r *routingFeatureHandlerErrorRepo) InsertErrorLog(_ context.Context, entry *service.OpsInsertErrorLogInput) (int64, error) {
	r.entries <- entry
	return 1, nil
}

func (r *routingFeatureHandlerErrorRepo) BatchInsertErrorLogs(_ context.Context, entries []*service.OpsInsertErrorLogInput) (int64, error) {
	for _, entry := range entries {
		r.entries <- entry
	}
	return int64(len(entries)), nil
}

func TestOpsRoutingFeatureMiddlewareDisabledKeepsCoreErrorAndObservation(t *testing.T) {
	repo := &routingFeatureHandlerSettings{values: map[string]string{service.SettingKeyRoutingDiagnosticsEnabled: "false"}}
	errorRepo := &routingFeatureHandlerErrorRepo{entries: make(chan *service.OpsInsertErrorLogInput, 1)}
	ops := service.NewOpsService(errorRepo, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := gin.New()
	r.Use(handler.OpsErrorLoggerMiddleware(ops))
	r.GET("/v1/responses", func(c *gin.Context) {
		// 已捕获的旧快照不能绕过当前持久化开关。
		d := &service.RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: 1, FilterCoverage: "unobserved"}
		c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{Message: "真实上游失败", UpstreamStatusCode: 502, RoutingDiagnostics: d}})
		service.SetServiceStatusAttemptFailure(c.Request.Context(), service.CompletionStatusUpstreamError, "provider_5xx", time.Now())
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "upstream_error", "message": "真实错误"}})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
	require.Equal(t, http.StatusBadGateway, w.Code)
	select {
	case entry := <-errorRepo.entries:
		require.Equal(t, "真实错误", entry.ErrorMessage)
		require.Equal(t, http.StatusBadGateway, entry.StatusCode)
		require.NotNil(t, entry.ServiceStatusObservationJSON)
		require.Nil(t, entry.RoutingDiagnosticsJSON)
		require.NotNil(t, entry.UpstreamErrorsJSON)
		require.NotContains(t, *entry.UpstreamErrorsJSON, "routing_diagnostics")
		require.Contains(t, *entry.UpstreamErrorsJSON, "真实上游失败")
	case <-time.After(2 * time.Second):
		t.Fatal("关闭诊断后核心错误没有到达持久化边界")
	}
}

func (r *routingFeatureHandlerSettings) GetValue(_ context.Context, key string) (string, error) {
	value, exists := r.values[key]
	if !exists {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *routingFeatureHandlerSettings) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string)
	for _, key := range keys {
		if value, exists := r.values[key]; exists {
			values[key] = value
		}
	}
	return values, nil
}

func (r *routingFeatureHandlerSettings) Set(_ context.Context, key, value string) error {
	if r.setError != nil {
		return r.setError
	}
	r.values[key] = value
	return nil
}

func routingFeatureHandlerRouter(ops *service.OpsService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := admin.NewOpsHandler(ops)
	r.GET("/config", h.GetRoutingDiagnosticsConfig)
	r.PUT("/config", h.UpdateRoutingDiagnosticsConfig)
	return r
}

func routingFeatureRequest(r *gin.Engine, method, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestOpsRoutingFeatureHandlerStrictBooleanAndReenable(t *testing.T) {
	repo := &routingFeatureHandlerSettings{values: map[string]string{service.SettingKeyOpsMonitoringEnabled: "false"}}
	ops := service.NewOpsService(nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := routingFeatureHandlerRouter(ops)
	w := routingFeatureRequest(r, http.MethodGet, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"enabled":true`)
	for _, body := range []string{`{}`, `null`, `{"enabled":null}`, `{"enabled":0}`, `{"enabled":"false"}`, `{"enabled":`, ""} {
		w = routingFeatureRequest(r, http.MethodPut, body)
		require.Equal(t, http.StatusBadRequest, w.Code, body)
	}
	for _, enabled := range []string{"false", "true"} {
		w = routingFeatureRequest(r, http.MethodPut, `{"enabled":`+enabled+`}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), `"effective_enabled":`+enabled)
		w = routingFeatureRequest(r, http.MethodGet, "")
		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), `"enabled":`+enabled)
	}
	require.False(t, ops.IsMonitoringEnabled(context.Background()), "管理接口不改变监控配置")
}

func TestOpsRoutingFeatureHandlerUnavailableAndInvalidConfig(t *testing.T) {
	for _, ops := range []*service.OpsService{nil, service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)} {
		r := routingFeatureHandlerRouter(ops)
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			w := routingFeatureRequest(r, method, `{"enabled":false}`)
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
		}
	}
	repo := &routingFeatureHandlerSettings{values: map[string]string{service.SettingKeyRoutingDiagnosticsEnabled: "false"}, setError: errors.New("store unavailable")}
	ops := service.NewOpsService(nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	r := routingFeatureHandlerRouter(ops)
	w := routingFeatureRequest(r, http.MethodPut, `{"enabled":true}`)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.False(t, ops.IsRoutingDiagnosticsEnabled(context.Background()))
	require.Equal(t, "false", repo.values[service.SettingKeyRoutingDiagnosticsEnabled])
	repo.values[service.SettingKeyRoutingDiagnosticsEnabled] = "on"
	w = routingFeatureRequest(r, http.MethodGet, "")
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

type routingFeatureEmptyAccounts struct{ service.AccountRepository }

func (routingFeatureEmptyAccounts) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	return nil, nil
}

func TestOpsRoutingFeatureMiddlewareAppliesRequestAndWSTurnSwitch(t *testing.T) {
	for _, enabled := range []string{"false", "true"} {
		t.Run(enabled, func(t *testing.T) {
			repo := &routingFeatureHandlerSettings{values: map[string]string{service.SettingKeyRoutingDiagnosticsEnabled: enabled}}
			ops := service.NewOpsService(nil, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			gateway := service.NewOpenAIGatewayService(routingFeatureEmptyAccounts{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			r := gin.New()
			r.Use(handler.OpsErrorLoggerMiddleware(ops))
			r.GET("/v1/responses", func(c *gin.Context) {
				for turn := 0; turn <= 2; turn++ {
					if turn > 0 {
						service.BeginOpsStreamTurn(c, turn)
					}
					ctx := service.EnsureRoutingDiagnosticsRequest(c.Request.Context())
					ctx = service.WithRoutingDiagnosticsOwner(context.Background(), ctx)
					selection, _, err := gateway.SelectAccountWithScheduler(ctx, nil, "", "", "gpt-test", nil, service.OpenAIUpstreamTransportAny, false)
					require.Nil(t, selection)
					require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
					if enabled == "true" {
						require.NotNil(t, service.RoutingDiagnosticsFromError(err))
					} else {
						require.Nil(t, service.RoutingDiagnosticsFromError(err))
					}
				}
				c.Status(http.StatusOK)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/responses", nil))
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}
