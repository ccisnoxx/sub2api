//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemHandlerUnsupportedBuildRejectsBinaryOperations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.SetDefaultIdempotencyCoordinator(nil)
	t.Cleanup(func() { service.SetDefaultIdempotencyCoordinator(nil) })
	for _, build := range []struct {
		name, version, buildType string
	}{
		{"个人镜像", "0.2.15-klno.2-tps.1", "release"},
		{"个人源码", "0.2.15-klno.2-tps.1", "source"},
		{"KIN 源码", "0.2.15-klno.2", "source"},
	} {
		for _, operation := range []struct {
			method, path, body string
		}{
			{http.MethodPost, "/update", ""},
			{http.MethodPost, "/rollback", ""},
			{http.MethodPost, "/rollback", `{"version":"0.2.15-klno.2-tps.0"}`},
			{http.MethodGet, "/rollback-versions", ""},
		} {
			t.Run(build.name+operation.method+operation.path+operation.body, func(t *testing.T) {
				// 空客户端确保拒绝先于缓存查询、GitHub 请求或二进制下载。
				updateSvc := service.NewUpdateService(nil, nil, build.version, build.buildType)
				lockSvc := service.NewSystemOperationLockService(newMemoryIdempotencyRepoStub(), service.IdempotencyConfig{
					ProcessingTimeout: time.Second, SystemOperationTTL: time.Minute,
				})
				handler := NewSystemHandler(updateSvc, lockSvc)
				router := gin.New()
				router.POST("/update", handler.PerformUpdate)
				router.POST("/rollback", handler.Rollback)
				router.GET("/rollback-versions", handler.GetRollbackVersions)
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(operation.method, operation.path, strings.NewReader(operation.body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(rec, req)

				require.Equal(t, http.StatusConflict, rec.Code)
				var body struct {
					Reason string `json:"reason"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Equal(t, "IN_PLACE_UPDATE_NOT_SUPPORTED", body.Reason)
			})
		}
	}
}
