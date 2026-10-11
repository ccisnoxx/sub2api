//go:build unit

package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 通过真实选号、转发、冷却和 Ops 中间件验证恢复行的归属，避免用手填事件绕过生产入口。
func TestS35HTTPRecoveredRoutingBelongsToFailedAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, mode := range []string{"first_402", "first_429"} {
		t.Run(mode, func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 4)
			stored := &ingressRejectOpsRepo{}
			ops := service.NewOpsService(stored, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			var successfulSelection *service.RoutingDiagnostics
			_, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, mode,
				OpsErrorLoggerMiddleware(ops),
				func(c *gin.Context) {
					c.Next()
					successfulSelection = service.GetRoutingDiagnostics(c.Request.Context())
				})
			defer cleanup()
			send := func() *httptest.ResponseRecorder {
				response := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, "/openai/v1/responses", bytes.NewBufferString(`{"model":"grok","input":"hello","stream":false}`))
				request.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(response, request)
				return response
			}

			response := send()
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, []int64{801, 802}, upstream.accountHits())
			require.Equal(t, 2, repo.selectorCalls(), "观测不能追加选号或查询")
			require.NotNil(t, successfulSelection)
			require.Equal(t, 2, successfulSelection.SelectionAttempt)
			require.EqualValues(t, 1, OpsErrorLogQueueLength())
			job := <-opsErrorLogQueue
			require.NotNil(t, job.entry.RoutingDiagnosticsJSON)
			failed, err := service.DecodeRoutingDiagnosticsJSON([]byte(*job.entry.RoutingDiagnosticsJSON))
			require.NoError(t, err)
			require.Equal(t, 1, failed.SelectionAttempt)
			require.Nil(t, failed.Turn)
			require.NotNil(t, failed.CandidatePool)
			require.Equal(t, 2, *failed.CandidatePool)
			events, err := service.ParseOpsUpstreamErrors(*job.entry.UpstreamErrorsJSON)
			require.NoError(t, err)
			require.Len(t, events, 1)
			require.Equal(t, int64(801), events[0].AccountID)
			require.Equal(t, failed, events[0].RoutingDiagnostics)
			status := http.StatusPaymentRequired
			if mode == "first_429" {
				status = http.StatusTooManyRequests
				require.Equal(t, []int64{801}, repo.rateLimitedAccountIDs())
			} else {
				require.Equal(t, []int64{801}, repo.setTempIDs)
			}
			require.Equal(t, status, events[0].UpstreamStatusCode)
			frozen := *job.entry.RoutingDiagnosticsJSON
			frozenEvents := *job.entry.UpstreamErrorsJSON

			// 后续请求受既有冷却影响，且不能改写已排队的失败发送快照。
			response = send()
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Equal(t, []int64{801, 802, 802}, upstream.accountHits())
			require.Equal(t, 3, repo.selectorCalls())
			require.Equal(t, 1, successfulSelection.SelectionAttempt)
			require.Empty(t, opsErrorLogQueue, "后续成功请求没有生成新的错误行")
			require.Equal(t, frozen, *job.entry.RoutingDiagnosticsJSON)
			require.Equal(t, frozenEvents, *job.entry.UpstreamErrorsJSON)

			flushOpsErrorLogBatch([]opsErrorLogJob{job})
			require.Equal(t, 1, stored.insertCalls)
			require.Len(t, stored.entries, 1)
			persisted := stored.entries[0]
			require.Equal(t, http.StatusOK, persisted.StatusCode, "恢复遥测保持在失败 SLA 之外")
			require.Equal(t, "upstream", persisted.ErrorPhase)
			require.Equal(t, "upstream_error", persisted.ErrorType)
			require.Equal(t, frozen, *persisted.RoutingDiagnosticsJSON)
			require.Equal(t, frozenEvents, *persisted.UpstreamErrorsJSON)
		})
	}
}
