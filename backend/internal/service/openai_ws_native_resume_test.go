package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const nativeResumeRateLimitEvent = `{"type":"error","status":429,"error":{"code":"rate_limit_exceeded","type":"rate_limit_error","message":"current turn limited"}}`

// 真实下游 socket 观察本轮 wrapper；上游使用既有连接夹具，避免另造转发实现。
func runOpenAIWSNativeResume(t *testing.T, requests []string, completedEvents []string, lastEvents []string, oauthAccount ...bool) ([][]byte, *openAIWSCaptureConn, []int, []int, []int, error) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.MaxConnsPerAccount = 1
	cfg.Gateway.OpenAIWS.MaxIdlePerAccount = 1
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 3
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 3
	upstream := &openAIWSCaptureConn{}
	for _, event := range append(append([]string(nil), completedEvents...), lastEvents...) {
		upstream.events = append(upstream.events, []byte(event))
	}
	pool := newOpenAIWSConnPool(cfg)
	pool.setClientDialerForTest(&openAIWSQueueDialer{conns: []openAIWSClientConn{upstream}})
	t.Cleanup(pool.Close)
	svc := &OpenAIGatewayService{
		cfg: cfg, cache: &stubGatewayCache{}, openaiWSPool: pool,
		openaiWSResolver: NewOpenAIWSProtocolResolver(cfg), toolCorrector: NewCodexToolCorrector(),
	}
	account := &Account{
		ID: 835, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-native-resume-test", "model_mapping": map[string]any{"route-model": "upstream-model"}},
		Extra:       map[string]any{"responses_websockets_v2_enabled": true},
	}
	if len(oauthAccount) > 0 && oauthAccount[0] {
		account.Type = AccountTypeOAuth
		account.Credentials["chatgpt_account_id"] = "native-resume-account"
		account.Credentials["chatgpt_user_id"] = "native-resume-user"
	}
	var beforeRequests, beforeTurns, afterTurns []int
	hooks := &OpenAIWSIngressHooks{
		MapRequestModel: func(_ int, model string) (string, error) {
			if model != "public-model" {
				return "", errors.New("unexpected original model")
			}
			return "route-model", nil
		},
		BeforeRequest: func(turn int, _ []byte, _ string) error {
			beforeRequests = append(beforeRequests, turn)
			return nil
		},
		BeforeTurn: func(turn int) error { beforeTurns = append(beforeTurns, turn); return nil },
		AfterTurn:  func(turn int, _ *OpenAIForwardResult, _ error) { afterTurns = append(afterTurns, turn) },
	}
	proxyErrCh := make(chan error, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			proxyErrCh <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		ginCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ginCtx.Request = r.Clone(r.Context())
		readCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		_, first, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			proxyErrCh <- err
			return
		}
		proxyErrCh <- svc.ProxyResponsesWebSocketFromClient(r.Context(), ginCtx, conn, account, "sk-native-resume-test", first, hooks)
	}))
	defer wsServer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(wsServer.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	var messages [][]byte
	for index, request := range requests {
		require.NoError(t, client.Write(ctx, websocket.MessageText, []byte(request)))
		if index < len(completedEvents) {
			_, event, readErr := client.Read(ctx)
			require.NoError(t, readErr)
			messages = append(messages, event)
		}
	}
	// 最后一次 429 必须在下游输出之前被截获；已有输出用例则观察完整两帧。
	if len(lastEvents) > 1 {
		for range lastEvents {
			_, event, readErr := client.Read(ctx)
			require.NoError(t, readErr)
			messages = append(messages, event)
		}
	}
	select {
	case proxyErr := <-proxyErrCh:
		return messages, upstream, beforeRequests, beforeTurns, afterTurns, proxyErr
	case <-ctx.Done():
		t.Fatal("等待 native 当前轮次结果超时")
		return nil, nil, nil, nil, nil, nil
	}
}

func TestOpenAIWSNativeLaterTurnFailoverReplaysCompleteCurrentContext(t *testing.T) {
	first := `{"type":"response.create","model":"public-model","input":[{"role":"user","content":"first"}]}`
	second := `{"type":"response.create","previous_response_id":"resp_first","input":[{"type":"function_call_output","call_id":"call_1","output":"tool-second"}]}`
	third := `{"type":"response.create","previous_response_id":"resp_second","client_metadata":{"session_id":"client-session","thread_id":"client-thread"},"input":[{"role":"user","content":"third"}]}`
	completed := []string{
		`{"type":"response.completed","response":{"id":"resp_first","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"first-answer"}]},{"id":"fc_1","type":"function_call","call_id":"call_1","name":"inspect","arguments":"{}"}]}}`,
		`{"type":"response.completed","response":{"id":"resp_second","output":[{"id":"msg_2","type":"message","role":"assistant","content":[{"type":"output_text","text":"second-answer"}]}]}}`,
	}
	messages, upstream, beforeRequests, beforeTurns, afterTurns, proxyErr := runOpenAIWSNativeResume(t, []string{first, second, third}, completed, []string{nativeResumeRateLimitEvent})
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, proxyErr, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	retryPayload, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
	require.True(t, currentTurn, "后续轮次必须由 current-turn wrapper 持有，不能退回连接首包")
	require.NotEmpty(t, retryPayload)
	require.False(t, gjson.GetBytes(retryPayload, "previous_response_id").Exists())
	require.Equal(t, "public-model", gjson.GetBytes(retryPayload, "model").String())
	require.Equal(t, "client-session", gjson.GetBytes(retryPayload, "client_metadata.session_id").String())
	require.Equal(t, "client-thread", gjson.GetBytes(retryPayload, "client_metadata.thread_id").String())
	input := gjson.GetBytes(retryPayload, "input").Array()
	require.Len(t, input, 6)
	require.Equal(t, "first", input[0].Get("content").String())
	require.Equal(t, "first-answer", input[1].Get("content.0.text").String())
	require.Equal(t, "call_1", input[2].Get("call_id").String())
	require.Equal(t, "tool-second", input[3].Get("output").String())
	require.Equal(t, "second-answer", input[4].Get("content.0.text").String())
	require.Equal(t, "third", input[5].Get("content").String())
	require.Len(t, messages, 2, "429 不得写出到下游")
	require.Equal(t, []int{2, 3}, beforeRequests)
	require.Equal(t, []int{1, 2, 3}, beforeTurns)
	require.Equal(t, []int{1, 2, 3}, afterTurns)
	upstream.mu.Lock()
	defer upstream.mu.Unlock()
	require.Len(t, upstream.writes, 3)
	require.Equal(t, "upstream-model", upstream.writes[2]["model"])
}

func TestOpenAIWSNativeLaterTurnFailoverRejectsIncompleteContext(t *testing.T) {
	root := `{"type":"response.create","model":"public-model","input":[{"role":"user","content":"first"}]}`
	complete := `{"type":"response.completed","response":{"id":"resp_first","output":[{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"first-answer"}]}]}}`
	for _, tc := range []struct{ name, first, completed, current string }{
		{"missing_terminal_output", root, `{"type":"response.completed","response":{"id":"resp_first"}}`, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"untyped_terminal_output", root, `{"type":"response.completed","response":{"id":"resp_first","output":[{"role":"assistant","content":"untyped"}]}}`, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"unpaired_historical_tool_call", root, `{"type":"response.completed","response":{"id":"resp_first","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"inspect","arguments":"{}"}]}}`, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"unknown_initial_anchor", `{"type":"response.create","model":"public-model","previous_response_id":"resp_external","input":[{"role":"user","content":"first"}]}`, complete, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"unknown_current_anchor", root, complete, `{"type":"response.create","previous_response_id":"resp_external","input":[{"role":"user","content":"second"}]}`},
		{"missing_root_input", `{"type":"response.create","model":"public-model"}`, complete, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"null_root_input", `{"type":"response.create","model":"public-model","input":null}`, complete, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"orphan_tool_output", root, complete, `{"type":"response.create","previous_response_id":"resp_first","input":[{"type":"function_call_output","call_id":"unknown","output":"second"}]}`},
		{"item_reference_is_not_portable", root, complete, `{"type":"response.create","previous_response_id":"resp_first","input":[{"type":"item_reference","id":"msg_1"},{"role":"user","content":"second"}]}`},
		{"failed_previous_turn", root, `{"type":"response.failed","response":{"id":"resp_first","output":[],"error":{"code":"server_error","message":"failed"}}}`, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
		{"done_with_incomplete_status", root, `{"type":"response.done","response":{"id":"resp_first","status":"incomplete","output":[]}}`, `{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"second"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			messages, _, _, _, _, proxyErr := runOpenAIWSNativeResume(t, []string{tc.first, tc.current}, []string{tc.completed}, []string{nativeResumeRateLimitEvent})
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, proxyErr, &failoverErr)
			require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
			retryPayload, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
			require.True(t, currentTurn, "不安全的当前轮次也必须明确拒绝首包回退")
			require.Empty(t, retryPayload)
			require.Len(t, messages, 1)
		})
	}
}

func TestOpenAIWSNativeLaterTurnFailoverIndependentInputStartsNewRoot(t *testing.T) {
	_, _, _, _, _, proxyErr := runOpenAIWSNativeResume(t, []string{
		`{"type":"response.create","model":"public-model","input":[{"role":"user","content":"old"}]}`,
		`{"type":"response.create","input":[{"role":"user","content":"current-independent"}]}`,
	}, []string{`{"type":"response.completed","response":{"id":"resp_first"}}`}, []string{nativeResumeRateLimitEvent})
	retryPayload, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
	require.True(t, currentTurn)
	require.NotEmpty(t, retryPayload)
	require.Equal(t, "public-model", gjson.GetBytes(retryPayload, "model").String())
	input := gjson.GetBytes(retryPayload, "input").Array()
	require.Len(t, input, 1)
	require.Equal(t, "current-independent", input[0].Get("content").String())
}

func TestOpenAIWSNativeLaterTurnRateLimitAfterOutputDoesNotFailOver(t *testing.T) {
	messages, _, _, _, _, proxyErr := runOpenAIWSNativeResume(t, []string{
		`{"type":"response.create","model":"public-model","input":[]}`,
		`{"type":"response.create","input":[{"role":"user","content":"current"}]}`,
	}, []string{`{"type":"response.completed","response":{"id":"resp_first","output":[]}}`}, []string{
		`{"type":"response.output_text.delta","delta":"partial-current"}`, nativeResumeRateLimitEvent,
	})
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(proxyErr, &failoverErr))
	_, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
	require.False(t, currentTurn)
	require.Len(t, messages, 3)
	require.Equal(t, "partial-current", gjson.GetBytes(messages[1], "delta").String())
	require.Equal(t, "rate_limit_exceeded", gjson.GetBytes(messages[2], "error.code").String())
}

func TestOpenAIWSNativeLaterTurnFailoverRejectsAccountToolAliases(t *testing.T) {
	for _, tc := range []struct{ name, first, completed, current string }{
		{
			"historical_alias_when_current_tools_omitted",
			`{"type":"response.create","model":"public-model","tools":[{"type":"function","name":"python","parameters":{"type":"object"}}],"input":[{"role":"user","content":"first"}]}`,
			`{"type":"response.completed","response":{"id":"resp_first","output":[{"id":"fc_1","type":"function_call","call_id":"call_1","name":"python_tool","arguments":"{}"}]}}`,
			`{"type":"response.create","previous_response_id":"resp_first","input":[{"type":"function_call_output","call_id":"call_1","output":"second"}]}`,
		},
		{
			"current_independent_request_alias",
			`{"type":"response.create","model":"public-model","input":[{"role":"user","content":"first"}]}`,
			`{"type":"response.completed","response":{"id":"resp_first","output":[]}}`,
			`{"type":"response.create","tools":[{"type":"function","name":"python","parameters":{"type":"object"}}],"input":[{"role":"user","content":"second"}]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			messages, upstream, _, _, _, proxyErr := runOpenAIWSNativeResume(t, []string{tc.first, tc.current}, []string{tc.completed}, []string{nativeResumeRateLimitEvent}, true)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, proxyErr, &failoverErr)
			require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
			retryPayload, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
			require.True(t, currentTurn)
			require.Empty(t, retryPayload, "账号 attempt 的工具别名不能丢失反查 owner 后跨账号重放")
			upstream.mu.Lock()
			defer upstream.mu.Unlock()
			if tc.name == "historical_alias_when_current_tools_omitted" {
				require.Equal(t, "python_tool", gjson.GetBytes(upstream.rawWrites[0], "tools.0.name").String(), "必须经过真实 OAuth alias producer")
				require.Equal(t, "python", gjson.GetBytes(messages[0], "response.output.0.name").String(), "首轮客户端仍收到原始工具名称")
				require.False(t, gjson.GetBytes(upstream.rawWrites[1], "tools").Exists())
			} else {
				require.Equal(t, "python_tool", gjson.GetBytes(upstream.rawWrites[1], "tools.0.name").String())
			}
		})
	}
}

// 上游是既有 capture conn；下游使用真实 client socket，重放正文由生产 native owner 生成。
func TestOpenAIWSNativeLaterTurnFailoverIndependentStringInput(t *testing.T) {
	for _, tc := range []struct{ name, inputJSON, text string }{
		{"unicode_escapes_newline_quote", `"\u6c49字🌦\n\"quoted\"\\path\t\u2028"`, "汉字🌦\n\"quoted\"\\path\t\u2028"},
		{"empty_string", `""`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := []string{
				`{"type":"response.create","model":"public-model","input":"old-root"}`,
				fmt.Sprintf(`{"type":"response.create","model":"public-model","client_metadata":{"thread_id":"current-string"},"input":%s}`, tc.inputJSON),
			}
			completed := `{"type":"response.completed","response":{"id":"resp_first","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"old-answer"}]}]}}`
			messages, upstream, _, _, _, proxyErr := runOpenAIWSNativeResume(t, requests, []string{completed}, []string{nativeResumeRateLimitEvent})
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, proxyErr, &failoverErr)
			require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
			retryPayload, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
			require.True(t, currentTurn)
			require.NotEmpty(t, retryPayload)
			require.Len(t, messages, 1, "429 必须在当前轮次下游输出之前截获")
			require.Equal(t, "public-model", gjson.GetBytes(retryPayload, "model").String())
			require.Equal(t, "current-string", gjson.GetBytes(retryPayload, "client_metadata.thread_id").String(), "不能回退连接首包")
			require.False(t, gjson.GetBytes(retryPayload, "previous_response_id").Exists())
			input := gjson.GetBytes(retryPayload, "input")
			require.True(t, input.IsArray())
			items := input.Array()
			require.Len(t, items, 1, "独立当前请求不能拼入旧根和 assistant output")
			require.True(t, items[0].IsObject(), "Responses input 数组成员必须是 object item，不能是字符串 scalar")
			require.Equal(t, "user", items[0].Get("role").String())
			require.Equal(t, gjson.String, items[0].Get("content").Type)
			require.Equal(t, tc.text, items[0].Get("content").String())
			upstream.mu.Lock()
			defer upstream.mu.Unlock()
			require.Len(t, upstream.rawWrites, 2)
			for i, expected := range []string{"old-root", tc.text} {
				original := gjson.GetBytes(upstream.rawWrites[i], "input")
				require.Equal(t, gjson.String, original.Type, "首发及同账号当前请求保持合法顶层字符串")
				require.Equal(t, expected, original.String())
				require.Equal(t, []string{`"old-root"`, tc.inputJSON}[i], original.Raw, "原始字符串转义不应被换号历史归一化改写")
			}
			t.Logf("captureconn 原发送 input[0]=%s input[1]=%s；生产 wrapper=%s", gjson.GetBytes(upstream.rawWrites[0], "input").Raw, gjson.GetBytes(upstream.rawWrites[1], "input").Raw, retryPayload)
		})
	}
}

func TestOpenAIWSNativeLaterTurnFailoverStringRootContinuation(t *testing.T) {
	for _, tc := range []struct{ name, inputJSON, text string }{
		{"unicode_escapes_newline_quote", `"\u6c49字🌦\n\"quoted\"\\path\t\u2028"`, "汉字🌦\n\"quoted\"\\path\t\u2028"},
		{"empty_string", `""`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := []string{
				fmt.Sprintf(`{"type":"response.create","model":"public-model","input":%s}`, tc.inputJSON),
				`{"type":"response.create","previous_response_id":"resp_first","input":[{"role":"user","content":"continuation-current"}]}`,
			}
			completed := `{"type":"response.completed","response":{"id":"resp_first","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"完整回答\n\"assistant\""}]}]}}`
			messages, upstream, _, _, _, proxyErr := runOpenAIWSNativeResume(t, requests, []string{completed}, []string{nativeResumeRateLimitEvent})
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, proxyErr, &failoverErr)
			require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
			retryPayload, currentTurn := OpenAIWSCurrentTurnRetryPayload(proxyErr)
			require.True(t, currentTurn)
			require.NotEmpty(t, retryPayload)
			require.Len(t, messages, 1)
			require.False(t, gjson.GetBytes(retryPayload, "previous_response_id").Exists(), "只有完整本地历史才能去除已知锚点")
			require.Equal(t, "public-model", gjson.GetBytes(retryPayload, "model").String())
			input := gjson.GetBytes(retryPayload, "input")
			require.True(t, input.IsArray())
			items := input.Array()
			require.Len(t, items, 3)
			for _, item := range items {
				require.True(t, item.IsObject(), "Responses input 数组成员必须是 object item，不能混合字符串 scalar")
			}
			require.Equal(t, "user", items[0].Get("role").String())
			require.Equal(t, gjson.String, items[0].Get("content").Type)
			require.Equal(t, tc.text, items[0].Get("content").String())
			require.Equal(t, "assistant", items[1].Get("role").String())
			require.Equal(t, "完整回答\n\"assistant\"", items[1].Get("content.0.text").String())
			require.Equal(t, "user", items[2].Get("role").String())
			require.Equal(t, "continuation-current", items[2].Get("content").String())
			upstream.mu.Lock()
			defer upstream.mu.Unlock()
			require.Len(t, upstream.rawWrites, 2)
			original := gjson.GetBytes(upstream.rawWrites[0], "input")
			require.Equal(t, gjson.String, original.Type, "字符串根首发保持原协议形态")
			require.Equal(t, tc.text, original.String())
			require.Equal(t, tc.inputJSON, original.Raw)
			require.Equal(t, "continuation-current", gjson.GetBytes(upstream.rawWrites[1], "input.0.content").String())
			t.Logf("captureconn 原字符串根=%s；生产 wrapper=%s", original.Raw, retryPayload)
		})
	}
}
