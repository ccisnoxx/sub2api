package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestServiceStatusRoutesInheritAdminAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	admin.Use(gin.HandlerFunc(middleware.NewAdminAuthMiddleware(nil, nil, nil, nil)))
	registerServiceStatusRoutes(admin, &handler.Handlers{
		ServiceStatus: handler.NewServiceStatusHandler(service.NewServiceStatusService(nil)),
	})
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/admin/service-status/config"},
		{http.MethodPut, "/api/v1/admin/service-status/config"},
		{http.MethodGet, "/api/v1/admin/service-status/snapshot"},
	} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, http.StatusUnauthorized, r.Code)
		require.Contains(t, r.Body.String(), "UNAUTHORIZED")
	}
	for _, path := range []string{"/api/v1/service-status/config", "/api/v1/service-status/snapshot"} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNotFound, r.Code)
	}
}

func TestServiceStatusRoutesRejectOrdinarySubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	admin.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 23})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
	})
	registerServiceStatusRoutes(admin, &handler.Handlers{
		ServiceStatus: handler.NewServiceStatusHandler(service.NewServiceStatusService(nil)),
	})
	for _, path := range []string{"config", "snapshot"} {
		r := httptest.NewRecorder()
		router.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/api/v1/admin/service-status/"+path, nil))
		require.Equal(t, http.StatusForbidden, r.Code)
	}
}
