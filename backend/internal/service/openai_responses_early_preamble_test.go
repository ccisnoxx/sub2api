package service

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const earlyPreambleCreated = "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_early\"}}\n\n"

func requireEarlyPreambleFailedTerminal(t *testing.T, body, code string) {
	t.Helper()
	var failures int
	forEachOpenAISSEFrame(body, func(eventType string, payload []byte) {
		require.True(t, gjson.ValidBytes(payload), "每个事件必须包含独立 JSON")
		if eventType != "response.failed" {
			return
		}
		failures++
		require.Equal(t, "resp_early", gjson.GetBytes(payload, "response.id").String())
		require.Equal(t, "failed", gjson.GetBytes(payload, "response.status").String())
		require.Equal(t, code, gjson.GetBytes(payload, "response.error.code").String())
		require.True(t, gjson.GetBytes(payload, "sequence_number").Exists())
		require.Positive(t, gjson.GetBytes(payload, "response.created_at").Int())
	})
	require.Equal(t, 1, failures, "必须恰好发送一个当前响应的失败终止事件")
}

func forwardEarlyPreambleTest(c *gin.Context, resp *http.Response, passthrough bool, cfg config.GatewayConfig, start time.Time) (*int, *OpenAIUsage, error) {
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: cfg}, toolCorrector: NewCodexToolCorrector()}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	if passthrough {
		result, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, start, "gpt-5", "gpt-5")
		if result == nil {
			return nil, nil, err
		}
		return result.firstTokenMs, result.usage, err
	}
	result, err := svc.handleStreamingResponse(c.Request.Context(), resp, c, account, start, "gpt-5", "gpt-5")
	if result == nil {
		return nil, nil, err
	}
	return result.firstTokenMs, result.usage, err
}

// 用真实 HTTP 客户端接收开场帧后才允许上游生成，证明网关没有等待完整响应。
func TestOpenAIResponsesEarlyPreambleReachesHTTPClientBeforeGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		t.Run(name, func(t *testing.T) {
			allowGeneration := make(chan struct{})
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(allowGeneration) }) }
			inProgress := "event: response.in_progress\ndata: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_early\"}}\n\n"
			emptyReasoning := "data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"reasoning\",\"summary\":[]}}\n\n"
			tail := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_early\",\"usage\":{\"input_tokens\":3,\"output_tokens\":1}}}\n\n"
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("X-Request-Id", "request-early")
				_, _ = io.WriteString(w, earlyPreambleCreated+inProgress+emptyReasoning)
				flusher, ok := w.(http.Flusher)
				if !ok {
					t.Error("测试上游不支持流刷新")
					return
				}
				flusher.Flush()
				select {
				case <-allowGeneration:
					_, _ = io.WriteString(w, tail)
				case <-r.Context().Done():
				}
			}))
			t.Cleanup(upstream.Close)
			forwardErr := make(chan error, 1)
			router := gin.New()
			router.POST("/v1/responses", func(c *gin.Context) {
				req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, upstream.URL, nil)
				if err != nil {
					forwardErr <- err
					return
				}
				resp, err := upstream.Client().Do(req)
				if err != nil {
					forwardErr <- err
					return
				}
				defer func() { _ = resp.Body.Close() }()
				_, _, err = forwardEarlyPreambleTest(c, resp, passthrough, config.GatewayConfig{
					OpenAIStreamEarlyPreamble: true, StreamKeepaliveInterval: 1,
				}, time.Now())
				forwardErr <- err
			})
			gateway := httptest.NewServer(router)
			t.Cleanup(gateway.Close)
			t.Cleanup(release)
			client := gateway.Client()
			client.Timeout = 5 * time.Second
			resp, err := client.Post(gateway.URL+"/v1/responses", "application/json", strings.NewReader(`{"stream":true}`))
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
			require.Equal(t, "request-early", resp.Header.Get("X-Request-Id"))
			reader := bufio.NewReader(resp.Body)
			keepalive := ":\n\n"
			if passthrough {
				keepalive = ": keepalive\n\n"
			}
			for _, expected := range []string{earlyPreambleCreated, inProgress, emptyReasoning, keepalive} {
				var event strings.Builder
				for {
					line, err := reader.ReadString('\n')
					require.NoError(t, err)
					_, _ = event.WriteString(line)
					if line == "\n" {
						break
					}
				}
				require.Equal(t, expected, event.String())
			}
			release()
			rest, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Contains(t, string(rest), `"delta":"hello"`)
			require.Contains(t, string(rest), `"type":"response.completed"`)
			require.NoError(t, <-forwardErr)
		})
	}
}

func TestOpenAIResponsesEarlyPreambleRetryBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	failed := "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_early\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"busy\"},\"usage\":{\"input_tokens\":7,\"output_tokens\":0}}}\n\n"
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		for _, tc := range []struct {
			name     string
			early    bool
			preamble string
			failover bool
		}{
			{"before_preamble", true, "", true},
			{"after_preamble", true, earlyPreambleCreated, false},
			{"buffering_restored", false, earlyPreambleCreated, true},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(tc.preamble + failed))}
				_, usage, err := forwardEarlyPreambleTest(c, resp, passthrough, config.GatewayConfig{OpenAIStreamEarlyPreamble: tc.early}, time.Now())
				require.Error(t, err)
				var failover *UpstreamFailoverError
				require.Equal(t, tc.failover, errors.As(err, &failover))
				require.Equal(t, 7, usage.InputTokens)
				if tc.failover {
					require.Empty(t, rec.Body.String())
				} else {
					require.Contains(t, rec.Body.String(), earlyPreambleCreated)
					require.Equal(t, 1, strings.Count(rec.Body.String(), `"type":"response.failed"`))
				}
			})
		}
	}
}

func TestOpenAIResponsesEarlyPreambleEmptyCompletionIsNotSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
		body := earlyPreambleCreated + "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_early\",\"output\":[]}}\n\n"
		resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
		firstToken, _, err := forwardEarlyPreambleTest(c, resp, passthrough, config.GatewayConfig{OpenAIStreamEarlyPreamble: true}, time.Now())
		require.ErrorContains(t, err, openAIResponsesEmptyCompletedMessage)
		var failover *UpstreamFailoverError
		require.False(t, errors.As(err, &failover))
		require.Nil(t, firstToken)
		require.NotContains(t, rec.Body.String(), `"type":"response.completed"`)
		requireEarlyPreambleFailedTerminal(t, rec.Body.String(), "upstream_error")
		require.True(t, IsResponseCommitted(c))
	}
}

func TestOpenAIResponsesEarlyPreambleTimeoutDoesNotReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	recorder := newOpenAIResponseFlushRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(context.Background())
	result := make(chan error, 1)
	go func() {
		resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}
		_, _, err := forwardEarlyPreambleTest(c, resp, false, config.GatewayConfig{
			OpenAIStreamEarlyPreamble: true, OpenAIFirstOutputTimeoutSeconds: 1,
		}, time.Now())
		result <- err
	}()
	_, err := io.WriteString(pw, earlyPreambleCreated)
	require.NoError(t, err)
	waitOpenAIResponseFlushCount(t, recorder, 1)
	select {
	case err = <-result:
	case <-time.After(3 * time.Second):
		t.Fatal("首输出超时未结束流")
	}
	require.ErrorContains(t, err, "first output timeout")
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover))
	body, _ := recorder.snapshot()
	require.Contains(t, body, `"code":"first_output_timeout"`)
	requireEarlyPreambleFailedTerminal(t, body, "first_output_timeout")
	require.True(t, IsResponseCommitted(c), "handler 不得追加第二个失败事件")
}

func TestOpenAIResponsesEarlyPreambleTimeoutSeparatesPendingEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close(); _ = pw.Close() })
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	pending := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n"
	go func() { _, _ = io.WriteString(pw, earlyPreambleCreated+pending) }()
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: pr}
	_, _, err := forwardEarlyPreambleTest(c, resp, false, config.GatewayConfig{
		OpenAIStreamEarlyPreamble: true, OpenAIFirstOutputTimeoutSeconds: 1,
	}, time.Now())
	require.ErrorContains(t, err, "first output timeout")
	require.Contains(t, rec.Body.String(), pending+"\nevent: response.failed\n", "超时错误必须与前一个未闭合的 SSE 事件分帧")
	requireEarlyPreambleFailedTerminal(t, rec.Body.String(), "first_output_timeout")
}

func TestOpenAIResponsesEarlyPreambleInterruptedStreamKeepsResponseIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		for _, tc := range []struct {
			name    string
			readErr error
			code    string
		}{
			{"eof", nil, OpenAIUpstreamStreamTruncatedCode},
			{"read_error", io.ErrUnexpectedEOF, OpenAIUpstreamStreamReadErrorCode},
			{"upstream_cancelled", context.Canceled, OpenAIUpstreamStreamReadErrorCode},
		} {
			for _, observedUsage := range []bool{false, true} {
				usageName := "no_usage"
				if observedUsage {
					usageName = "partial_usage"
				}
				t.Run(name+"/"+tc.name+"/"+usageName, func(t *testing.T) {
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
					payload := earlyPreambleCreated
					if observedUsage {
						// 最后一帧不带空行，终止帧必须先补齐其 SSE 边界。
						payload += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\",\"usage\":{\"input_tokens\":7,\"output_tokens\":1}}\n"
					} else {
						payload += "event: response.in_progress\ndata: {\"type\":\"response.in_progress\",\"response\":{\"id\":\"resp_early\"}}\n"
					}
					var reader io.Reader = strings.NewReader(payload)
					if tc.readErr != nil {
						reader = io.MultiReader(reader, iotest.ErrReader(tc.readErr))
					}
					resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(reader)}
					_, usage, err := forwardEarlyPreambleTest(c, resp, passthrough, config.GatewayConfig{OpenAIStreamEarlyPreamble: true}, time.Now())
					require.Error(t, err)
					var failover *UpstreamFailoverError
					require.False(t, errors.As(err, &failover), "已发送开场身份后不能透明重放")
					require.Contains(t, rec.Body.String(), earlyPreambleCreated)
					requireEarlyPreambleFailedTerminal(t, rec.Body.String(), tc.code)
					require.True(t, IsResponseCommitted(c), "handler 不得追加另一个响应身份的失败帧")
					if observedUsage {
						require.Equal(t, 7, usage.InputTokens)
						require.Equal(t, 1, usage.OutputTokens)
					}
				})
			}
		}
	}
}
