package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type serviceStatusHandlerStub struct {
	calls    int
	rangeArg string
	platform string
	config   *service.ServiceStatusConfig
	err      error
}

func (s *serviceStatusHandlerStub) GetConfig(context.Context) (*service.ServiceStatusConfig, error) {
	s.calls++
	return s.config, s.err
}
func (s *serviceStatusHandlerStub) UpdateConfig(_ context.Context, cfg *service.ServiceStatusConfig) (*service.ServiceStatusConfig, error) {
	s.calls++
	s.config = cfg
	return cfg, s.err
}
func (s *serviceStatusHandlerStub) Snapshot(_ context.Context, historyRange, platform string) (*service.ServiceStatusSnapshot, error) {
	s.calls++
	s.rangeArg, s.platform = historyRange, platform
	return &service.ServiceStatusSnapshot{SchemaVersion: 1, Health: "unknown", UnknownReason: "insufficient_samples"}, s.err
}

func serviceStatusHandlerContext(method, target, body, role string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if role != "" {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 17})
		c.Set(string(middleware.ContextKeyUserRole), role)
	}
	return c, r
}

func TestServiceStatusHandlerRejectsNonAdminBeforeDataRead(t *testing.T) {
	for _, role := range []string{"", service.RoleUser, "support"} {
		for _, action := range []string{"config", "update", "snapshot"} {
			t.Run(role+"/"+action, func(t *testing.T) {
				s := &serviceStatusHandlerStub{}
				h := &ServiceStatusHandler{service: s}
				c, r := serviceStatusHandlerContext(http.MethodGet, "/", "", role)
				switch action {
				case "config":
					h.GetConfig(c)
				case "update":
					h.UpdateConfig(c)
				default:
					h.Snapshot(c)
				}
				if role == "" {
					require.Equal(t, http.StatusUnauthorized, r.Code)
				} else {
					require.Equal(t, http.StatusForbidden, r.Code)
				}
				require.Zero(t, s.calls)
			})
		}
	}
}

func TestServiceStatusSnapshotRejectsScopeSimulationAndDuplicateParameters(t *testing.T) {
	for _, query := range []string{
		"group_id=1", "user_id=17", "admin=true", "model=gpt-5", "range=24h&range=7d",
		"platform=openai&platform=openai", "range=", "platform=", "range=90d", "platform=%zz",
	} {
		t.Run(query, func(t *testing.T) {
			s := &serviceStatusHandlerStub{}
			c, r := serviceStatusHandlerContext(http.MethodGet, "/snapshot?"+query, "", service.RoleAdmin)
			(&ServiceStatusHandler{service: s}).Snapshot(c)
			require.Equal(t, http.StatusBadRequest, r.Code)
			require.Zero(t, s.calls)
		})
	}
	for _, query := range []string{"", "range=7d&platform=openai"} {
		s := &serviceStatusHandlerStub{}
		c, r := serviceStatusHandlerContext(http.MethodGet, "/snapshot?"+query, "", service.RoleAdmin)
		(&ServiceStatusHandler{service: s}).Snapshot(c)
		require.Equal(t, http.StatusOK, r.Code)
		if query == "" {
			require.Equal(t, "24h", s.rangeArg)
		} else {
			require.Equal(t, "7d", s.rangeArg)
			require.Equal(t, "openai", s.platform)
		}
	}
}

func TestServiceStatusConfigRequiresAllFieldsAndPreservesExplicitDisable(t *testing.T) {
	valid := `{"version":1,"enabled":false,"platforms":[],"minimum_samples":5,"warning_error_rate":0.05,"outage_error_rate":0.9,"abnormal_windows":2,"recovery_windows":3}`
	for _, body := range []string{"{}", strings.Replace(valid, `"enabled":false,`, "", 1), strings.Replace(valid, `"platforms":[]`, `"platforms":null`, 1), valid + " {}", strings.Replace(valid, `"version":1`, `"version":1,"observation_start":"2026-10-09"`, 1)} {
		c, r := serviceStatusHandlerContext(http.MethodPut, "/config", body, service.RoleAdmin)
		s := &serviceStatusHandlerStub{}
		(&ServiceStatusHandler{service: s}).UpdateConfig(c)
		require.Equal(t, http.StatusBadRequest, r.Code)
		require.Zero(t, s.calls)
	}
	c, r := serviceStatusHandlerContext(http.MethodPut, "/config", valid, service.RoleAdmin)
	s := &serviceStatusHandlerStub{}
	(&ServiceStatusHandler{service: s}).UpdateConfig(c)
	require.Equal(t, http.StatusOK, r.Code)
	require.False(t, s.config.Enabled)
	require.NotNil(t, s.config.Platforms)
	require.Empty(t, s.config.Platforms)
	require.EqualValues(t, 1, s.config.Version)
}

func TestServiceStatusHandlerPreservesFixedErrorsAndNullRatios(t *testing.T) {
	for _, tc := range []struct {
		status int
		reason string
	}{
		{http.StatusServiceUnavailable, "SERVICE_STATUS_DISABLED"},
		{http.StatusConflict, "SERVICE_STATUS_CONFIG_CONFLICT"},
	} {
		c, r := serviceStatusHandlerContext(http.MethodGet, "/snapshot", "", service.RoleAdmin)
		s := &serviceStatusHandlerStub{err: infraerrors.New(tc.status, tc.reason, "服务状态暂不可用")}
		(&ServiceStatusHandler{service: s}).Snapshot(c)
		require.Equal(t, tc.status, r.Code)
		require.Contains(t, r.Body.String(), tc.reason)
	}
	raw, err := json.Marshal(service.ServiceStatusLeaf{Health: "unknown", UnknownReason: "insufficient_samples"})
	require.NoError(t, err)
	require.Contains(t, string(raw), `"error_rate":null`)
	require.Contains(t, string(raw), `"success_rate":null`)
}
