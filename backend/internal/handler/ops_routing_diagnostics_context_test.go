//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type routingContextEmptyAccountRepo struct {
	service.AccountRepository
	listCalls int
}

func (r *routingContextEmptyAccountRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]service.Account, error) {
	r.listCalls++
	return []service.Account{}, nil
}

func newRoutingContextGateway(repo *routingContextEmptyAccountRepo) *service.OpenAIGatewayService {
	return service.NewOpenAIGatewayService(
		repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
}

func selectRoutingContextEmptyPool(t *testing.T, c *gin.Context, gateway *service.OpenAIGatewayService) *service.RoutingDiagnostics {
	t.Helper()
	selection, _, err := gateway.SelectAccountWithScheduler(c.Request.Context(), nil, "", "", "gpt-test", nil, service.OpenAIUpstreamTransportAny, false)
	require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
	require.Nil(t, selection)
	diagnostic := service.RoutingDiagnosticsFromError(err)
	require.NotNil(t, diagnostic)
	require.NotNil(t, diagnostic.CandidatePool)
	require.EqualValues(t, 0, *diagnostic.CandidatePool)
	return diagnostic
}

func TestSetOpsRequestContextRetainsRoutingSelectionAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil).WithContext(ctx)
	repo := &routingContextEmptyAccountRepo{}
	gateway := newRoutingContextGateway(repo)

	setOpsRequestContext(c, "", false)
	first := selectRoutingContextEmptyPool(t, c, gateway)
	require.EqualValues(t, 1, first.SelectionAttempt)
	require.Nil(t, first.Turn)

	setOpsRequestContext(c, " gpt-test ", true)
	second := selectRoutingContextEmptyPool(t, c, gateway)
	require.EqualValues(t, 2, second.SelectionAttempt)
	require.Equal(t, "gpt-test", c.Request.Context().Value(ctxkey.Model))
	require.Equal(t, "gpt-test", c.GetString(opsModelKey))
	require.True(t, c.GetBool(opsStreamKey))
	require.Equal(t, 2, repo.listCalls, "诊断不能引入额外账号查询")
	require.EqualValues(t, 1, first.SelectionAttempt, "后续选择不得修改先前返回快照")

	cancel()
	require.ErrorIs(t, c.Request.Context().Err(), context.Canceled)
}

func TestOpsStreamTurnClearsRoutingProducerState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil)
	repo := &routingContextEmptyAccountRepo{}
	gateway := newRoutingContextGateway(repo)
	setOpsRequestContext(c, "gpt-test", true)
	handshake := selectRoutingContextEmptyPool(t, c, gateway)
	require.Nil(t, handshake.Turn)

	service.BeginOpsStreamTurn(c, 1)
	require.Nil(t, service.GetRoutingDiagnostics(c.Request.Context()), "未再次选择的 turn 不借用建连池数")
	turnOne := selectRoutingContextEmptyPool(t, c, gateway)
	require.NotNil(t, turnOne.Turn)
	require.EqualValues(t, 1, *turnOne.Turn)
	require.EqualValues(t, 1, turnOne.SelectionAttempt)
	service.BeginOpsStreamTurn(c, 1)
	repeatedTurnOne := selectRoutingContextEmptyPool(t, c, gateway)
	require.EqualValues(t, 2, repeatedTurnOne.SelectionAttempt, "同 turn 的重选不重置序号")

	service.BeginOpsStreamTurn(c, 2)
	require.Nil(t, service.GetRoutingDiagnostics(c.Request.Context()))
	setOpsRequestContext(c, "gpt-test", true)
	turnTwo := selectRoutingContextEmptyPool(t, c, gateway)
	require.NotNil(t, turnTwo.Turn)
	require.EqualValues(t, 2, *turnTwo.Turn)
	require.EqualValues(t, 1, turnTwo.SelectionAttempt)
	require.EqualValues(t, 1, *turnOne.Turn)
	require.EqualValues(t, 1, turnOne.SelectionAttempt)
	require.Equal(t, 4, repo.listCalls)
}

func TestWSSelectionUsesCurrentRoutingOwnerWithoutReplacingPricingContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	connectionCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil).WithContext(connectionCtx)
	type pricingContextKey struct{}
	pricingCtx := context.WithValue(connectionCtx, pricingContextKey{}, "frozen-pricing")
	setOpsRequestContext(c, "gpt-test", true)
	service.BeginOpsStreamTurn(c, 2)
	repo := &routingContextEmptyAccountRepo{}
	gateway := newRoutingContextGateway(repo)

	selectionCtx := service.WithRoutingDiagnosticsOwner(pricingCtx, c.Request.Context())
	_, _, err := gateway.SelectAccountWithScheduler(selectionCtx, nil, "", "", "gpt-test", nil, service.OpenAIUpstreamTransportAny, false)
	require.ErrorIs(t, err, service.ErrNoAvailableAccounts)
	diagnostic := service.RoutingDiagnosticsFromError(err)
	require.NotNil(t, diagnostic)
	require.EqualValues(t, 2, *diagnostic.Turn)
	require.EqualValues(t, 1, diagnostic.SelectionAttempt)
	require.Equal(t, "frozen-pricing", selectionCtx.Value(pricingContextKey{}))
	require.Equal(t, diagnostic, service.GetRoutingDiagnostics(c.Request.Context()))
	require.Nil(t, service.GetRoutingDiagnostics(pricingCtx), "绑定只借用当前 owner，不修改连接 context")
	cancel()
	require.ErrorIs(t, selectionCtx.Err(), context.Canceled)
}

func TestWSRoutingTurnMappingKeepsSelectionAcrossProxyRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/responses", nil)
	setOpsRequestContext(c, "gpt-test", true)
	scope := &openAIWSRoutingTurns{}
	scope.beginProxy(c)
	scope.beginTurn(c, 2)
	gateway := newRoutingContextGateway(&routingContextEmptyAccountRepo{})
	first := selectRoutingContextEmptyPool(t, c, gateway)
	require.EqualValues(t, 2, *first.Turn)
	scope.beginProxy(c) // 换号或同账号重试都会重新进入 Proxy。
	scope.beginTurn(c, 1)
	second := selectRoutingContextEmptyPool(t, c, gateway)
	require.EqualValues(t, 2, *second.Turn)
	require.EqualValues(t, 2, second.SelectionAttempt)
	scope.beginProxy(c)   // 同一逻辑 turn 再次重试。
	scope.beginTurn(c, 2) // 新客户端请求的局部编号再次为 2。
	require.Nil(t, service.GetRoutingDiagnostics(c.Request.Context()))
	third := selectRoutingContextEmptyPool(t, c, gateway)
	require.EqualValues(t, 3, *third.Turn)
	require.EqualValues(t, 1, third.SelectionAttempt)
	require.Equal(t, 2, c.GetInt(service.OpsStreamTurnKey), "既有 Ops 局部 turn 编号保持")
	require.EqualValues(t, 2, *first.Turn)
}

func dialRoutingDiagnosticsWS(t *testing.T, router *gin.Engine) *coderws.Conn {
	t.Helper()
	server := httptest.NewServer(router)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.CloseNow()
		server.Close()
	})
	return conn
}

func exchangeRoutingDiagnosticsWSTurn(t *testing.T, conn *coderws.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"grok","input":[{"role":"user","content":"hello"}],"stream":false}`)))
	for {
		_, payload, err := conn.Read(ctx)
		require.NoError(t, err)
		if bytes.Contains(payload, []byte("response.completed")) {
			return
		}
	}
}

func TestResponsesWebSocketHandshakeFailoverKeepsRoutingOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "revoked")
	t.Cleanup(cleanup)
	observed := make(chan *service.RoutingDiagnostics, 1)
	upstream.onRequest = func(req *http.Request) {
		observed <- service.GetRoutingDiagnostics(req.Context())
	}
	conn := dialRoutingDiagnosticsWS(t, router)
	exchangeRoutingDiagnosticsWSTurn(t, conn)
	diagnostic := <-observed
	require.NotNil(t, diagnostic, "可续期 context 写回不能丢掉建连 owner")
	require.EqualValues(t, 2, diagnostic.SelectionAttempt)
	require.Nil(t, diagnostic.Turn, "建连凭据 failover 发生在 admitted turn 前")
	require.Equal(t, 2, repo.selectorCalls())
	require.Equal(t, []int64{802}, upstream.accountHits())
}

func TestResponsesWebSocketTurnAfterFailoverClearsRoutingSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	observed := make(chan *service.RoutingDiagnostics, 4)
	var upstream *grokCredentialHandlerUpstream
	call := 0
	testLogger, err := zap.NewDevelopment()
	require.NoError(t, err)
	_, repo, fixtureUpstream, router, cleanup := newGrokCredentialFailoverHandler(t, "first_402", func(c *gin.Context) {
		c.Request = c.Request.WithContext(logger.IntoContext(c.Request.Context(), testLogger))
		upstream.onRequest = func(*http.Request) {
			call++
			observed <- service.GetRoutingDiagnostics(c.Request.Context())
			upstream.mu.Lock()
			upstream.failAccountID = 0
			upstream.rateLimitIDs = nil
			if call == 2 {
				upstream.rateLimitIDs = map[int64]bool{801: true}
			}
			upstream.mu.Unlock()
		}
		c.Next()
	})
	upstream = fixtureUpstream
	t.Cleanup(cleanup)
	conn := dialRoutingDiagnosticsWS(t, router)
	exchangeRoutingDiagnosticsWSTurn(t, conn)
	exchangeRoutingDiagnosticsWSTurn(t, conn)
	exchangeRoutingDiagnosticsWSTurn(t, conn)
	require.Nil(t, <-observed)
	require.Nil(t, <-observed)
	retry := <-observed
	require.NotNil(t, retry)
	require.EqualValues(t, 2, *retry.Turn)
	require.EqualValues(t, 1, retry.SelectionAttempt)
	require.Nil(t, <-observed, "第3条请求不能借用 failover 后第2 turn 的池数")
	require.Equal(t, 2, repo.selectorCalls())
	require.Equal(t, []int64{801, 801, 802, 802}, upstream.accountHits())
}

func TestGrokAudioSelectionRetriesShareRequestRoutingOwner(t *testing.T) {
	for _, endpoint := range []string{"voice", "realtime"} {
		t.Run(endpoint, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			var final *service.RoutingDiagnostics
			var attempts []*service.OpsUpstreamErrorEvent
			h, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "all_revoked", func(c *gin.Context) {
				c.Next()
				final = service.GetRoutingDiagnostics(c.Request.Context())
				if value, ok := c.Get(service.OpsUpstreamErrorsKey); ok {
					attempts, _ = value.([]*service.OpsUpstreamErrorEvent)
				}
			})
			t.Cleanup(cleanup)
			if endpoint == "voice" {
				router.POST("/routing/voice", func(c *gin.Context) { h.GrokVoice(c, "tts") })
			} else {
				router.GET("/routing/realtime", h.GrokRealtime)
			}
			method := http.MethodPost
			if endpoint == "realtime" {
				method = http.MethodGet
			}
			request := httptest.NewRequest(method, "/routing/"+endpoint, bytes.NewBufferString(`{"input":"hello"}`))
			request.Header.Set("Content-Type", "application/json")
			if endpoint == "realtime" {
				request.Header.Set("Connection", "Upgrade")
				request.Header.Set("Upgrade", "websocket")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.GreaterOrEqual(t, recorder.Code, 400)
			require.NotNil(t, final)
			require.EqualValues(t, 3, final.SelectionAttempt)
			require.Equal(t, 3, repo.selectorCalls())
			// 实际凭据失败事件必须绑定各自选择，不借最后一轮的快照。
			require.Len(t, attempts, 2, "第三次评估已耗尽候选，不伪造第三个上游失败事件")
			for i, event := range attempts {
				require.NotNil(t, event.RoutingDiagnostics)
				require.Equal(t, i+1, event.RoutingDiagnostics.SelectionAttempt)
				require.Nil(t, event.RoutingDiagnostics.Turn)
			}
			require.Empty(t, upstream.accountHits(), "凭据失败不得调用真实语音上游")
		})
	}
}
