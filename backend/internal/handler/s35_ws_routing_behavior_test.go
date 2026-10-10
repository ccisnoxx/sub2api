//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 这些用例驱动客户端及上游真实 socket，不向上下文预填路由诊断或失败事件。
// Ops 中间件在连接结束后才排队；第一条连接的队列保留到第二条连接完成后再读取。
func TestS35WSMultiTurnFailureDiagnosticsAndQueuedLogs(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			runS35WSRoutingBehavior(t, mode, false)
		})
	}
}

// native 的发送前断连恢复会重建同账号 socket，但不重新选号；它与 handler 换号是不同边界。
func TestS35WSNativeSameAccountRetryKeepsRoutingSelection(t *testing.T) {
	runS35WSRoutingBehavior(t, service.OpenAIWSIngressModeDedicated, true)
}

type s35WSHit struct {
	accountID int64
	input     string
}

type s35WSSessionSnapshot struct {
	errors  []service.OpsStreamError
	current *service.RoutingDiagnostics
}

func runS35WSRoutingBehavior(t *testing.T, mode string, transportRetry bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	setupOpsErrorLogTestQueue(t, 8)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })

	const firstAccountID, healthyAccountID = int64(93501), int64(93502)
	var hitsMu sync.Mutex
	var hits []s35WSHit
	upstreamErrors := make(chan error, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, &coderws.AcceptOptions{CompressionMode: coderws.CompressionContextTakeover})
		if err != nil {
			upstreamErrors <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		var accountID int64
		switch r.Header.Get("Authorization") {
		case "Bearer sk-s35-first":
			accountID = firstAccountID
		case "Bearer sk-s35-healthy":
			accountID = healthyAccountID
		default:
			upstreamErrors <- fmt.Errorf("上游收到不属于测试账号的认证头")
			return
		}
		for {
			readCtx, cancelRead := context.WithTimeout(r.Context(), 5*time.Second)
			_, payload, readErr := conn.Read(readCtx)
			cancelRead()
			if readErr != nil {
				if status := coderws.CloseStatus(readErr); status != coderws.StatusNormalClosure && status != -1 {
					upstreamErrors <- readErr
				}
				return
			}
			input := gjson.GetBytes(payload, "input.0.content").String()
			hitsMu.Lock()
			hits = append(hits, s35WSHit{accountID: accountID, input: input})
			firstSend := len(hits) == 1
			hitsMu.Unlock()
			if transportRetry && firstSend {
				// 尚未写出任何下游事件，触发 native 现有一次同账号重建连接恢复。
				return
			}
			var event []byte
			if accountID == firstAccountID {
				event = []byte(`{"type":"error","error":{"code":"rate_limit_exceeded","type":"usage_limit_reached","message":"The usage limit has been reached"}}`)
			} else {
				terminal := "response.failed"
				response := map[string]any{
					"id": "resp_" + input, "model": "gpt-5.1", "status": "failed",
					"usage": map[string]any{"input_tokens": 11, "output_tokens": 2},
					"error": map[string]any{
						"code": "invalid_prompt", "type": "invalid_request_error",
						"message": "s35-visible-" + input, "status_code": http.StatusBadRequest,
					},
				}
				if strings.HasSuffix(input, "turn-2") {
					terminal = "response.completed"
					response["status"] = "completed"
					delete(response, "error")
				}
				event, err = json.Marshal(map[string]any{"type": terminal, "response": response})
				if err != nil {
					upstreamErrors <- err
					return
				}
			}
			writeCtx, cancelWrite := context.WithTimeout(r.Context(), 5*time.Second)
			err = conn.Write(writeCtx, coderws.MessageText, event)
			cancelWrite()
			if err != nil {
				upstreamErrors <- err
				return
			}
			if accountID == firstAccountID {
				return
			}
		}
	}))
	t.Cleanup(upstream.Close)

	newAccount := func(id int64, token string, priority int) service.Account {
		return service.Account{
			ID: id, Name: fmt.Sprintf("s35-ws-%d", id), Platform: service.PlatformOpenAI,
			Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
			Concurrency: 1, Priority: priority,
			Credentials: map[string]any{"api_key": token, "base_url": upstream.URL},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
				"openai_apikey_responses_websockets_v2_mode":    mode,
			},
		}
	}
	accounts := []service.Account{newAccount(healthyAccountID, "sk-s35-healthy", 2)}
	if !transportRetry {
		accounts = append([]service.Account{newAccount(firstAccountID, "sk-s35-first", 1)}, accounts...)
	}
	// 复用同包线程安全仓库、用量仓库和并发夹具，只提供生产 owner 所需的账号及槽位。
	repo := &grokCredentialHandlerRepo{accounts: accounts}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 6)}
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 5
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	rateLimit := service.NewRateLimitService(repo, nil, cfg, nil, nil)
	gateway := service.NewOpenAIGatewayService(
		repo, usageRepo, nil, nil, nil, nil, nil, cfg, nil, nil,
		service.NewBillingService(cfg, nil), rateLimit, billingCache, nil, &service.DeferredService{},
		nil, nil, nil, nil, nil, nil, nil,
	)
	t.Cleanup(gateway.CloseOpenAIWSPool)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := &OpenAIGatewayHandler{
		cfg: cfg, gatewayService: gateway, billingCacheService: billingCache,
		apiKeyService: &service.APIKeyService{}, maxAccountSwitches: 2,
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
	groupID := int64(93503)
	apiKey := &service.APIKey{
		ID: 93504, UserID: 93505, GroupID: &groupID,
		User:  &service.User{ID: 93505, Status: service.StatusActive},
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	}
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	finished := make(chan s35WSSessionSnapshot, 2)
	var connectionNo atomic.Int64
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		requestID := fmt.Sprintf("s35-connection-%d", connectionNo.Add(1))
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, requestID))
		c.Next()
		finished <- s35WSSessionSnapshot{errors: service.GetOpsStreamErrors(c), current: service.GetRoutingDiagnostics(c.Request.Context())}
	})
	router.Use(OpsErrorLoggerMiddleware(ops))
	router.GET("/openai/v1/responses", h.ResponsesWebSocket)

	var snapshots []s35WSSessionSnapshot
	var firstTurnListCalls []int
	for session := 1; session <= 2; session++ {
		client := dialRoutingDiagnosticsWS(t, router)
		for turn := 1; turn <= 3; turn++ {
			input := fmt.Sprintf("session-%d-turn-%d", session, turn)
			payload := fmt.Sprintf(`{"type":"response.create","model":"gpt-5.1","input":[{"role":"user","content":%q}],"stream":false}`, input)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(payload)))
			_, event, readErr := client.Read(ctx)
			cancel()
			require.NoError(t, readErr)
			if turn == 1 {
				firstTurnListCalls = append(firstTurnListCalls, repo.selectorCalls())
			}
			require.Equal(t, "resp_"+input, gjson.GetBytes(event, "response.id").String(), "重试或换号只能重放所属请求")
			if turn == 2 {
				require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
			} else {
				require.Equal(t, "response.failed", gjson.GetBytes(event, "type").String())
				require.Equal(t, "s35-visible-"+input, gjson.GetBytes(event, "response.error.message").String())
			}
		}
		require.NoError(t, client.Close(coderws.StatusNormalClosure, "s35-done"))
		select {
		case snapshot := <-finished:
			snapshots = append(snapshots, snapshot)
		case <-time.After(5 * time.Second):
			t.Fatal("等待真实 WS handler 与 Ops 中间件退出超时")
		}
		require.Equal(t, int64(session*2), OpsErrorLogQueueLength(), "每条连接只记录两次客户端可见失败，成功 turn 不产生失败行")
		require.Equal(t, firstTurnListCalls[session-1], repo.selectorCalls(), "后续 turn 不能因诊断追加账号查询或重新选号")
	}
	for session, snapshot := range snapshots {
		require.Nil(t, snapshot.current, "第3 turn 未重新选号，不得借用建连或重试的选择快照")
		require.Len(t, snapshot.errors, 2)
		first, last := snapshot.errors[0], snapshot.errors[1]
		require.Equal(t, "s35-visible-"+fmt.Sprintf("session-%d-turn-1", session+1), first.Message)
		require.Equal(t, healthyAccountID, first.AccountID)
		require.Equal(t, "gpt-5.1", first.UpstreamModel)
		require.True(t, first.CountTowardsSLA)
		if session == 0 && !transportRetry {
			// 初始选择属于握手；入场后真正换号才产生逻辑 turn 1 的第1次评估。
			require.NotNil(t, first.RoutingDiagnostics)
			require.EqualValues(t, 1, *first.RoutingDiagnostics.Turn)
			require.Equal(t, 1, first.RoutingDiagnostics.SelectionAttempt)
			require.EqualValues(t, 1, *first.RoutingDiagnostics.CandidatePool)
		} else {
			require.Nil(t, first.RoutingDiagnostics, "连接复用或同账号重建不能把握手选择伪装成 turn 选择")
		}
		require.Nil(t, last.RoutingDiagnostics, "后续真实失败必须保持本 turn 的未知诊断")
		require.Equal(t, 3, last.Turn)
		require.Equal(t, "s35-visible-"+fmt.Sprintf("session-%d-turn-3", session+1), last.Message)
		for _, streamErr := range snapshot.errors {
			job := <-opsErrorLogQueue
			require.Equal(t, fmt.Sprintf("s35-connection-%d", session+1), job.entry.RequestID)
			require.Equal(t, streamErr.Message, job.entry.ErrorMessage)
			require.Equal(t, http.StatusBadRequest, job.entry.StatusCode)
			require.Equal(t, healthyAccountID, *job.entry.AccountID)
			require.EqualValues(t, service.RequestTypeWSV2, *job.entry.RequestType)
			if streamErr.RoutingDiagnostics == nil {
				require.Nil(t, job.entry.RoutingDiagnosticsJSON)
			} else {
				require.NotNil(t, job.entry.RoutingDiagnosticsJSON)
				decoded, decodeErr := service.DecodeRoutingDiagnosticsJSON([]byte(*job.entry.RoutingDiagnosticsJSON))
				require.NoError(t, decodeErr)
				require.Equal(t, streamErr.RoutingDiagnostics, decoded, "真实 producer 的旧 turn 快照经中间件与延后消费队列保持不变")
			}
			routingJSON := "null"
			if job.entry.RoutingDiagnosticsJSON != nil {
				routingJSON = *job.entry.RoutingDiagnosticsJSON
			}
			t.Logf("WS日志事实 mode=%s request=%s status=%d phase=%s owner=%s routing=%s", mode, job.entry.RequestID, job.entry.StatusCode, job.entry.ErrorPhase, job.entry.ErrorOwner, routingJSON)
		}
	}

	var usageIDs []string
	for i := 0; i < 6; i++ {
		select {
		case usage := <-usageRepo.created:
			require.Equal(t, healthyAccountID, usage.AccountID)
			require.Equal(t, 11, usage.InputTokens)
			require.Equal(t, 2, usage.OutputTokens)
			usageIDs = append(usageIDs, usage.RequestID)
		case <-time.After(5 * time.Second):
			t.Fatal("真实 AfterTurn 用量没有落入既有 RecordUsage owner")
		}
	}
	expectedHits := []s35WSHit{{accountID: firstAccountID, input: "session-1-turn-1"}}
	if transportRetry {
		expectedHits[0].accountID = healthyAccountID
	}
	var expectedUsageIDs []string
	for session := 1; session <= 2; session++ {
		for turn := 1; turn <= 3; turn++ {
			input := fmt.Sprintf("session-%d-turn-%d", session, turn)
			expectedHits = append(expectedHits, s35WSHit{accountID: healthyAccountID, input: input})
			expectedUsageIDs = append(expectedUsageIDs, "resp_"+input)
		}
	}
	hitsMu.Lock()
	actualHits := append([]s35WSHit(nil), hits...)
	hitsMu.Unlock()
	require.Equal(t, expectedHits, actualHits, "真实认证账号与发送顺序必须符合一次恢复或切换，不能重放旧 turn")
	require.Equal(t, expectedUsageIDs, usageIDs, "隐藏重试不得新增用量行或换掉所属响应 ID")
	// gwpool 偏好预检查与最终 legacy 选择均会读账号列表，比较正常连接与恢复连接的真实次数。
	normalConnectionListCalls := firstTurnListCalls[1] - firstTurnListCalls[0]
	if transportRetry {
		require.Equal(t, normalConnectionListCalls, firstTurnListCalls[0], "同账号 socket 重建没有额外选择评估")
	} else {
		require.Equal(t, 2*normalConnectionListCalls, firstTurnListCalls[0], "一次真实换号恰好多一次完整选择评估")
	}
	if transportRetry {
		require.Empty(t, repo.rateLimitedAccountIDs())
	} else {
		require.Equal(t, []int64{firstAccountID}, repo.rateLimitedAccountIDs(), "现有真实冷却副作用保持一次")
	}
	require.Zero(t, OpsErrorLogDroppedTotal())
	select {
	case err := <-upstreamErrors:
		require.NoError(t, err)
	default:
	}
}
