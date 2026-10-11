package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const timingFinalJSON = `{"id":"resp_timing","object":"response","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":5,"output_tokens":7}}`
const timingFinalEvent = `{"type":"response.completed","response":` + timingFinalJSON + `}`

func timingHTTPContext(body string) (*gin.Context, *http.Response) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	return c, &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func timingTestService() *OpenAIGatewayService {
	return &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}, toolCorrector: NewCodexToolCorrector()}
}

func TestResponsesUsageTiming_HTTPEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	body := passthroughSSEData(`{"type":"response.created","response":{"id":"resp_timing"}}`) + passthroughSSEData(`{"type":"response.output_text.delta","delta":"answer"}`) + passthroughSSEData(timingFinalEvent)
	for _, pass := range []bool{false, true} {
		for _, mode := range []string{"stream", "json", "sse_to_json"} {
			t.Run(map[bool]string{false: "normal", true: "passthrough"}[pass]+"/"+mode, func(t *testing.T) {
				svc := timingTestService()
				upstreamBody := body
				if mode == "json" {
					upstreamBody = timingFinalJSON
				}
				c, resp := timingHTTPContext(upstreamBody)
				if mode == "json" {
					resp.Header.Set("Content-Type", "application/json")
				}
				start := time.Now().Add(-2 * time.Second)
				var timing UsageTiming
				var usage *OpenAIUsage
				if pass {
					if mode == "stream" {
						r, e := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, start, "gpt-5.1", "gpt-5.1")
						require.NoError(t, e)
						timing = r.usageTiming
						usage = r.usage
					} else {
						r, e := svc.handleNonStreamingResponsePassthrough(context.Background(), resp, c, account, "gpt-5.1", "gpt-5.1", start)
						require.NoError(t, e)
						timing = r.usageTiming
						usage = r.usage
					}
				} else {
					if mode == "stream" {
						r, e := svc.handleStreamingResponse(context.Background(), resp, c, account, start, "gpt-5.1", "gpt-5.1")
						require.NoError(t, e)
						timing = r.usageTiming
						usage = r.usage
					} else {
						r, e := svc.handleNonStreamingResponse(context.Background(), resp, c, account, "gpt-5.1", "gpt-5.1", start)
						require.NoError(t, e)
						timing = r.usageTiming
						usage = r.usage
					}
				}
				require.EqualValues(t, 1, timing.TimingVersion)
				require.NotNil(t, timing.StrictFirstTokenMs)
				require.GreaterOrEqual(t, *timing.StrictFirstTokenMs, 2000)
				require.Equal(t, "text", *timing.FirstOutputKind)
				require.Equal(t, CompletionStatusCompleted, timing.CompletionStatus)
				require.Equal(t, UsageSourceUpstreamFinal, timing.UsageSource)
				require.Equal(t, 0, *timing.AudioOutputTokens)
				require.Equal(t, 5, usage.InputTokens)
				require.Equal(t, 7, usage.OutputTokens)
				if mode != "stream" {
					require.Equal(t, *timing.StrictFirstTokenMs, *timing.LastTokenMs)
				}
			})
		}
	}
}

func TestResponsesUsageTiming_HTTPFailureCancellationAndRetryIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	svc := timingTestService()
	partial := passthroughSSEData(`{"type":"response.output_text.delta","delta":"partial"}`)
	failed := passthroughSSEData(`{"type":"response.failed","response":{"id":"resp_timing","status":"failed","error":{"message":"upstream failed"},"usage":{"input_tokens":5,"output_tokens":2}}}`)
	for _, pass := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "passthrough"}[pass], func(t *testing.T) {
			c, resp := timingHTTPContext(partial + failed + "data: [DONE]\n\n")
			var first UsageTiming
			if pass {
				r, e := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				require.Error(t, e)
				first = r.usageTiming
			} else {
				r, e := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				require.Error(t, e)
				first = r.usageTiming
			}
			require.NotNil(t, first.StrictFirstTokenMs)
			require.Equal(t, CompletionStatusUpstreamError, first.CompletionStatus)
			require.Equal(t, UsageSourceUpstreamPartial, first.UsageSource)
			// 同一个 service 的下一次 attempt 只有终态聚合，不能借用前次输出。
			c, resp = timingHTTPContext(passthroughSSEData(timingFinalEvent))
			var next UsageTiming
			if pass {
				r, e := svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				require.NoError(t, e)
				next = r.usageTiming
			} else {
				r, e := svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				require.NoError(t, e)
				next = r.usageTiming
			}
			require.Nil(t, next.StrictFirstTokenMs)
			require.Equal(t, CompletionStatusCompleted, next.CompletionStatus)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writer := &cancelOnFirstWriteResponseWriter{cancel: cancel}
			c, _ = gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
			resp = &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(partial + passthroughSSEData(timingFinalEvent)))}
			var canceled UsageTiming
			if pass {
				r, e := svc.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				require.NoError(t, e)
				canceled = r.usageTiming
			} else {
				r, e := svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "gpt-5.1", "gpt-5.1")
				require.NoError(t, e)
				canceled = r.usageTiming
			}
			require.Equal(t, CompletionStatusClientDisconnected, canceled.CompletionStatus)
			require.False(t, *canceled.IsComplete)
			require.Equal(t, UsageSourceUpstreamFinal, canceled.UsageSource)
		})
	}
}

func TestResponsesUsageTiming_WSHTTPBridgeSeparateTurnsAndIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}
	for i, body := range []string{passthroughSSEData(`{"type":"response.created","response":{"id":"resp_timing"}}`) + passthroughSSEData(`{"type":"response.output_text.delta","response_id":"other","delta":"foreign"}`) + passthroughSSEData(`{"type":"response.output_text.delta","response_id":"resp_timing","delta":"answer"}`) + passthroughSSEData(timingFinalEvent), passthroughSSEData(timingFinalEvent)} {
		svc := timingTestService()
		svc.httpUpstream = &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}}
		c, _ := timingHTTPContext("")
		payload := []byte(`{"type":"response.create","model":"gpt-5.1","input":"hello"}`)
		result, e := svc.proxyOpenAIWSHTTPBridgeTurn(context.Background(), c, account, "test-token", payload, len(payload), "gpt-5.1", "", "", "", "", i+1, func([]byte) error { return nil })
		require.NoError(t, e)
		require.EqualValues(t, 1, result.UsageTiming.TimingVersion)
		require.Equal(t, CompletionStatusCompleted, result.UsageTiming.CompletionStatus)
		if i == 0 {
			require.NotNil(t, result.UsageTiming.StrictFirstTokenMs)
		} else {
			require.Nil(t, result.UsageTiming.StrictFirstTokenMs)
		}
	}
}

func TestResponsesUsageTiming_WSPassthroughAdmittedTurns(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controlCtx, cancel := context.WithCancelCause(context.Background())
	defer cancel(context.Canceled)
	upstream := newStagedPassthroughConn()
	svc := newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream)
	results := make(chan *OpenAIForwardResult, 2)
	server, serverErr := startPassthroughLifecycleServerWithHooks(t, controlCtx, svc, passthroughLifecycleAccount(), func(*gin.Context) *OpenAIWSIngressHooks {
		return &OpenAIWSIngressHooks{InitialTurnStartedAt: time.Now().Add(-2 * time.Second), AfterTurn: func(_ int, r *OpenAIForwardResult, e error) {
			if r != nil {
				results <- r
			}
		}}
	})
	defer server.Close()
	client := dialPassthroughLifecycleClient(t, server)
	defer func() { _ = client.CloseNow() }()
	requirePassthroughUpstreamWrite(t, upstream, time.Second)
	upstream.Send(`{"type":"response.created","response":{"id":"first"}}`)
	upstream.Send(`{"type":"response.output_text.delta","delta":"answer"}`)
	upstream.Send(`{"type":"response.completed","usage":{"input_tokens":99,"output_tokens":99,"output_tokens_details":{"audio_tokens":99}},"response":{"id":"first","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":7,"output_tokens_details":{"audio_tokens":2}}}}`)
	for i := 0; i < 3; i++ {
		_, e := readPassthroughLifecycleFrame(t, client, time.Second)
		require.NoError(t, e)
	}
	first := <-results
	require.EqualValues(t, 1, first.UsageTiming.TimingVersion)
	require.GreaterOrEqual(t, *first.UsageTiming.StrictFirstTokenMs, 2000)
	require.Equal(t, CompletionStatusCompleted, first.UsageTiming.CompletionStatus)
	require.Equal(t, 5, first.Usage.InputTokens)
	require.Equal(t, 7, first.Usage.OutputTokens)
	require.Equal(t, 2, *first.UsageTiming.AudioOutputTokens)
	require.Equal(t, UsageSourceUpstreamFinal, first.UsageTiming.UsageSource)
	writeCtx, stop := context.WithTimeout(context.Background(), time.Second)
	require.NoError(t, client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","input":"next"}`)))
	stop()
	requirePassthroughUpstreamWrite(t, upstream, time.Second)
	upstream.Send(`{"type":"response.completed","response":{"id":"second","status":"completed","output":[],"usage":{"input_tokens":3,"output_tokens":4}}}`)
	_, e := readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, e)
	second := <-results
	require.EqualValues(t, 1, second.UsageTiming.TimingVersion)
	require.Nil(t, second.UsageTiming.StrictFirstTokenMs)
	require.Equal(t, CompletionStatusCompleted, second.UsageTiming.CompletionStatus)
	require.Equal(t, 3, second.Usage.InputTokens)
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	select {
	case <-serverErr:
	case <-time.After(3 * time.Second):
		t.Fatal("passthrough did not end")
	}
	require.Equal(t, CompletionStatusCompleted, first.UsageTiming.CompletionStatus)
}

func TestResponsesUsageTiming_WSRegistryInterleavedIDs(t *testing.T) {
	start := time.Unix(100, 0)
	registry := &responsesWSTurnTimings{ctx: context.Background(), account: &Account{Platform: PlatformOpenAI}}
	defer registry.close()
	events := []struct {
		id, payload string
		start, at   time.Time
		accepted    bool
	}{
		{"a", `{"type":"response.output_text.delta","delta":"a"}`, start, start.Add(10 * time.Millisecond), false},
		{"b", `{"type":"response.output_text.delta","delta":"b"}`, start.Add(time.Second), start.Add(time.Second + 20*time.Millisecond), false},
		{"a", `{"type":"response.completed","response":{"usage":{"input_tokens":2,"output_tokens":3}}}`, start, start.Add(2 * time.Second), true},
		{"b", `{"type":"response.failed","response":{"usage":{"input_tokens":4,"output_tokens":5}}}`, start.Add(time.Second), start.Add(3 * time.Second), true},
	}
	for _, event := range events {
		registry.observe(openaiwsv2.RelayObservedEvent{Payload: []byte(event.payload), ResponseID: event.id, StartedAt: event.start, ObservedAt: event.at, UsageAccepted: event.accepted, AcceptedUsage: gjson.Get(event.payload, "response.usage").Raw})
	}
	a := registry.take("a")
	b := registry.take("b")
	require.Equal(t, 10, *a.StrictFirstTokenMs)
	require.Equal(t, 20, *b.StrictFirstTokenMs)
	require.Equal(t, CompletionStatusCompleted, a.CompletionStatus)
	require.Equal(t, CompletionStatusUpstreamError, b.CompletionStatus)
	require.Equal(t, UsageSourceUpstreamFinal, a.UsageSource)
	require.Equal(t, UsageSourceUpstreamPartial, b.UsageSource)
}

func TestResponsesUsageTiming_SSEToJSONUsesAcceptedFinalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, pass := range []bool{false, true} {
		for _, zero := range []bool{false, true} {
			t.Run(map[bool]string{false: "normal", true: "passthrough"}[pass]+map[bool]string{false: "/dual_usage", true: "/zero_final"}[zero], func(t *testing.T) {
				body := passthroughSSEData(`{"type":"response.audio.delta","delta":"bytes"}`) + passthroughSSEData(`{"type":"response.in_progress","response":{"usage":{"input_tokens":2,"output_tokens":3,"output_tokens_details":{"audio_tokens":1}}}}`)
				final := `{"type":"response.completed","usage":{"input_tokens":99,"output_tokens":99,"output_tokens_details":{"audio_tokens":99}},"response":{"id":"resp_timing","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":7,"output_tokens_details":{"audio_tokens":2}}}}`
				if zero {
					final = `{"type":"response.completed","response":{"id":"resp_timing","status":"completed","output":[],"usage":{"input_tokens":0,"output_tokens":0}}}`
				}
				body += passthroughSSEData(final)
				c, resp := timingHTTPContext(body)
				svc := timingTestService()
				account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
				var timing UsageTiming
				var usage *OpenAIUsage
				if pass {
					r, err := svc.handleNonStreamingResponsePassthrough(context.Background(), resp, c, account, "gpt-5.1", "gpt-5.1", time.Now())
					require.NoError(t, err)
					timing, usage = r.usageTiming, r.usage
				} else {
					r, err := svc.handleNonStreamingResponse(context.Background(), resp, c, account, "gpt-5.1", "gpt-5.1", time.Now())
					require.NoError(t, err)
					timing, usage = r.usageTiming, r.usage
				}
				require.Equal(t, UsageSourceUpstreamFinal, timing.UsageSource)
				if zero {
					require.Zero(t, usage.OutputTokens)
					require.Nil(t, timing.AudioOutputTokens)
				} else {
					require.Equal(t, 7, usage.OutputTokens)
					require.Equal(t, 2, *timing.AudioOutputTokens)
				}
			})
		}
	}
}
