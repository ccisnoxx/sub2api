//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type routingDiagnosticsJWTUserRepo struct {
	service.UserRepository
	users map[int64]*service.User
}

func (r *routingDiagnosticsJWTUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	user := r.users[id]
	if user == nil {
		return nil, service.ErrUserNotFound
	}
	clone := *user
	return &clone, nil
}

func (*routingDiagnosticsJWTUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type routingDiagnosticsJWTOpsRepo struct {
	service.OpsRepository
	detail *service.OpsErrorLogDetail
	reads  int
}

func (r *routingDiagnosticsJWTOpsRepo) GetErrorLogByID(context.Context, int64) (*service.OpsErrorLogDetail, error) {
	r.reads++
	clone := *r.detail
	return &clone, nil
}

// 使用实际 JWT 校验与路由注册，验证内部诊断只会跨过管理员详情边界。
func TestOpsRoutingDiagnosticsAdminRoutesJWTBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	users := &routingDiagnosticsJWTUserRepo{users: map[int64]*service.User{
		1: {ID: 1, Role: service.RoleAdmin, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true},
		2: {ID: 2, Role: service.RoleUser, Status: service.StatusActive, TokenVersion: 2, TokenVersionResolved: true},
	}}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "routing-diagnostics-jwt-test-secret", ExpireHour: 1}}
	auth := service.NewAuthService(nil, users, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	tokenFor := func(id int64, version int64) string {
		user := *users.users[id]
		user.TokenVersion = version
		token, err := auth.GenerateToken(context.Background(), &user)
		require.NoError(t, err)
		return token
	}
	pool, filtered := 0, 0
	layer, reason := "load_balance", "pool_empty"
	repo := &routingDiagnosticsJWTOpsRepo{detail: &service.OpsErrorLogDetail{
		OpsErrorLog: service.OpsErrorLog{ID: 1, Message: "真实选择失败"},
		RoutingDiagnostics: &service.RoutingDiagnostics{SchemaVersion: 1, SelectionAttempt: 1,
			SelectionLayer: &layer, SelectionReason: &reason, CandidatePool: &pool, FilteredCandidates: &filtered,
			FilterReasons: map[string]int{}, FilterCoverage: "complete"},
	}}
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Ops: adminhandler.NewOpsHandler(
		service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))}}
	router := gin.New()
	RegisterAdminRoutes(router.Group("/api/v1"), handlers,
		servermiddleware.NewAdminAuthMiddleware(auth, service.NewUserService(users, nil, nil, nil), nil, nil),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)

	for _, path := range []string{"/api/v1/admin/ops/errors/1", "/api/v1/admin/ops/request-errors/1"} {
		for _, tc := range []struct {
			name   string
			token  string
			status int
		}{
			{name: "未认证", status: http.StatusUnauthorized},
			{name: "篡改签名", token: tokenFor(1, 2) + "invalid", status: http.StatusUnauthorized},
			{name: "普通用户", token: tokenFor(2, 2), status: http.StatusForbidden},
			{name: "已撤销", token: tokenFor(1, 1), status: http.StatusUnauthorized},
			{name: "管理员", token: tokenFor(1, 2), status: http.StatusOK},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				before := repo.reads
				request := httptest.NewRequest(http.MethodGet, path, nil)
				if tc.token != "" {
					request.Header.Set("Authorization", "Bearer "+tc.token)
				}
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				require.Equal(t, tc.status, recorder.Code, recorder.Body.String())
				if tc.status == http.StatusOK {
					require.Equal(t, before+1, repo.reads)
					require.Contains(t, recorder.Body.String(), `"candidate_pool":0`)
					require.Contains(t, recorder.Body.String(), "真实选择失败")
				} else {
					require.Equal(t, before, repo.reads, "拒绝请求不得查询内部详情")
					require.NotContains(t, recorder.Body.String(), "routing_diagnostics")
					require.NotContains(t, recorder.Body.String(), "真实选择失败")
				}
			})
		}
	}
}
