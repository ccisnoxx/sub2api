//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const chatTimingJSON = `{"id":"chat_timing","object":"chat.completion","model":"company-coding-model","choices":[{"index":0,"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":2},"completion_tokens_details":{"audio_tokens":0}}}`
const chatTimingSSE = "data: {\"id\":\"chat_timing\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"\"}}]}\n\n" +
	"data: {\"id\":\"chat_timing\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"}}]}\n\n" +
	"data: {\"id\":\"chat_timing\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
	"data: {\"id\":\"chat_timing\",\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":7,\"prompt_tokens_details\":{\"cached_tokens\":2},\"completion_tokens_details\":{\"audio_tokens\":0}}}\n\n" +
	"data: [DONE]\n\n"

func TestChatUsageTiming_ForwardOwners(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, chain := range []string{"raw_chat", "responses_to_chat", "chat_to_responses"} {
		for _, streaming := range []bool{false, true} {
			t.Run(chain+map[bool]string{false: "/json", true: "/sse"}[streaming], func(t *testing.T) {
				account := forceChatResponsesFallbackAccount()
				upstreamBody := chatTimingJSON
				if streaming {
					upstreamBody = chatTimingSSE
				}
				path := "/v1/chat/completions"
				request := `{"model":"company-coding-model","messages":[{"role":"user","content":"hello"}],"stream":` + map[bool]string{false: "false", true: "true"}[streaming] + `}`
				if chain == "responses_to_chat" {
					path = "/v1/responses"
					request = `{"model":"company-coding-model","input":"hello","stream":` + map[bool]string{false: "false", true: "true"}[streaming] + `}`
				}
				if chain == "chat_to_responses" {
					account = rawChatCompletionsTestAccount()
					account.Extra = map[string]any{}
					account.Extra[openai_compat.ExtraKeyResponsesSupported] = true
					upstreamBody = passthroughSSEData(`{"type":"response.output_text.delta","delta":"answer"}`) + passthroughSSEData(`{"type":"response.completed","response":{"id":"resp_timing","status":"completed","model":"company-coding-model","output":[{"type":"message","content":[{"type":"output_text","text":"answer"}]}],"usage":{"input_tokens":5,"output_tokens":7,"input_tokens_details":{"cached_tokens":2},"output_tokens_details":{"audio_tokens":0}}}}`)
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(request))
				contentType := "text/event-stream"
				if chain != "chat_to_responses" && !streaming {
					contentType = "application/json"
				}
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
				svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
				var result *OpenAIForwardResult
				var err error
				if chain == "responses_to_chat" {
					result, err = svc.Forward(context.Background(), c, account, []byte(request))
				} else {
					result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(request), "", "")
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, OpenAIUsage{InputTokens: 5, OutputTokens: 7, CacheReadInputTokens: 2}, result.Usage)
				require.EqualValues(t, 1, result.UsageTiming.TimingVersion)
				require.NotNil(t, result.UsageTiming.StrictFirstTokenMs)
				require.Equal(t, "text", *result.UsageTiming.FirstOutputKind)
				require.Equal(t, CompletionStatusCompleted, result.UsageTiming.CompletionStatus)
				require.True(t, *result.UsageTiming.IsComplete)
				require.Equal(t, UsageSourceUpstreamFinal, result.UsageTiming.UsageSource)
				require.Equal(t, 0, *result.UsageTiming.AudioOutputTokens)
				if !streaming {
					require.Equal(t, *result.UsageTiming.StrictFirstTokenMs, *result.UsageTiming.LastTokenMs)
				}
			})
		}
	}
}

func runChatTimingOwner(t *testing.T, chain string, c *gin.Context, resp *http.Response, stream bool, expectedChoices int) (*OpenAIForwardResult, error) {
	t.Helper()
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	account := rawChatCompletionsTestAccount()
	start := time.Now().Add(-2 * time.Second)
	switch chain {
	case "raw_chat":
		if stream {
			return svc.streamRawChatCompletions(c, resp, account, "model", "model", "model", nil, nil, start, 0, expectedChoices)
		}
		return svc.bufferRawChatCompletions(c, resp, account, "model", "model", "model", nil, nil, start, expectedChoices)
	case "responses_to_chat":
		if stream {
			return svc.streamChatCompletionsAsResponses(c, resp, "model", nil, nil, false, map[string]apicompat.NamespacedToolName{}, "model", "model", nil, nil, start)
		}
		return svc.bufferChatCompletionsAsResponses(c, resp, "model", nil, nil, false, nil, "model", "model", nil, nil, start)
	case "chat_to_responses":
		if stream {
			return svc.handleChatStreamingResponse(resp, c, account, "model", "model", "model", start, 0)
		}
		return svc.handleChatBufferedStreamingResponse(resp, c, account, "model", "model", "model", start)
	}
	t.Fatalf("未知测试链路 %s", chain)
	return nil, nil
}

func TestChatUsageTiming_ProtocolOwnersContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, chain := range []string{"raw_chat", "responses_to_chat", "chat_to_responses"} {
		for _, stream := range []bool{false, true} {
			for _, kind := range []string{"text", "reasoning", "tool"} {
				t.Run(chain+map[bool]string{false: "/json/", true: "/sse/"}[stream]+kind, func(t *testing.T) {
					content := map[string]string{"text": `{"content":"answer"}`, "reasoning": `{"reasoning_content":"thinking"}`, "tool": `{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"run","arguments":"{}"}}]}`}[kind]
					body := `{"choices":[{"index":0,"message":` + content + `,"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7}}`
					if stream {
						body = passthroughSSEData(`{"choices":[{"index":0,"delta":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","function":{"name":"run","arguments":""}}]}}]}`) + passthroughSSEData(`{"choices":[{"index":0,"delta":`+content+`}]}`) + passthroughSSEData(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7}}`) + "data: [DONE]\n\n"
					}
					if chain == "chat_to_responses" {
						delta := map[string]string{"text": `{"type":"response.output_text.delta","delta":"answer"}`, "reasoning": `{"type":"response.reasoning_summary_text.delta","delta":"thinking"}`, "tool": `{"type":"response.function_call_arguments.delta","item_id":"fc_1","call_id":"call_1","name":"run","delta":"{}"}`}[kind]
						output := map[string]string{"text": `{"type":"message","content":[{"type":"output_text","text":"answer"}]}`, "reasoning": `{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking"}]}`, "tool": `{"type":"function_call","id":"fc_1","call_id":"call_1","name":"run","arguments":"{}"}`}[kind]
						body = passthroughSSEData(`{"type":"response.created","response":{"id":"resp_1"}}`) + passthroughSSEData(delta) + passthroughSSEData(`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[`+output+`],"usage":{"input_tokens":5,"output_tokens":7}}}`)
					}
					c, resp := timingHTTPContext(body)
					result, err := runChatTimingOwner(t, chain, c, resp, stream, 1)
					require.NoError(t, err)
					require.Equal(t, 5, result.Usage.InputTokens)
					require.Equal(t, 7, result.Usage.OutputTokens)
					require.Equal(t, kind, *result.UsageTiming.FirstOutputKind)
					require.GreaterOrEqual(t, *result.UsageTiming.StrictFirstTokenMs, 2000)
					require.Equal(t, CompletionStatusCompleted, result.UsageTiming.CompletionStatus)
					if !stream {
						require.Equal(t, *result.UsageTiming.StrictFirstTokenMs, *result.UsageTiming.LastTokenMs)
					}
				})
			}
		}
	}
}

type chatTimingSequenceReader struct {
	chunks []string
	before func(int)
	err    error
	index  int
}

func (r *chatTimingSequenceReader) Read(p []byte) (int, error) {
	if r.before != nil {
		r.before(r.index)
	}
	if r.index == len(r.chunks) {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	n := copy(p, chunk)
	if n == len(chunk) {
		r.index++
	} else {
		r.chunks[r.index] = chunk[n:]
	}
	return n, nil
}
func (*chatTimingSequenceReader) Close() error { return nil }

func TestChatUsageTiming_ProtocolOwnersFailuresAndDrain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, chain := range []string{"raw_chat", "responses_to_chat", "chat_to_responses"} {
		for _, ending := range []string{"eof", "read_error", "upstream_error", "cancel_then_complete", "write_failure_then_complete", "complete_then_cancel"} {
			t.Run(chain+"/"+ending, func(t *testing.T) {
				partial := passthroughSSEData(`{"choices":[{"index":0,"delta":{"content":"partial"}}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`)
				complete := passthroughSSEData(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`) + passthroughSSEData(`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":7}}`) + "data: [DONE]\n\n"
				failure := passthroughSSEData(`{"error":{"code":"provider_failure","message":"failed"}}`) + "data: [DONE]\n\n"
				if chain == "chat_to_responses" {
					partial = passthroughSSEData(`{"type":"response.output_text.delta","delta":"partial","usage":{"input_tokens":5,"output_tokens":2}}`)
					complete = passthroughSSEData(timingFinalEvent)
					failure = passthroughSSEData(`{"type":"response.failed","response":{"status":"failed","error":{"message":"failed"},"usage":{"input_tokens":5,"output_tokens":2}}}`)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				c, resp := timingHTTPContext("")
				c.Request = c.Request.WithContext(ctx)
				if ending == "write_failure_then_complete" {
					c.Writer = &openAIRawStreamDisconnectedWriter{ResponseWriter: c.Writer}
				}
				reader := &chatTimingSequenceReader{chunks: []string{partial}}
				want := CompletionStatusInterrupted
				switch ending {
				case "read_error":
					reader.err = errors.New("connection reset by peer")
				case "upstream_error":
					reader.chunks = append(reader.chunks, failure)
					want = CompletionStatusUpstreamError
				case "cancel_then_complete", "write_failure_then_complete", "complete_then_cancel":
					reader.chunks = append(reader.chunks, complete)
					want = CompletionStatusClientDisconnected
					if ending == "cancel_then_complete" {
						reader.before = func(index int) {
							if index == 1 {
								cancel()
								reader.before = nil
							}
						}
					}
					if ending == "complete_then_cancel" {
						want = CompletionStatusCompleted
					}
				}
				resp.Body = reader
				result, err := runChatTimingOwner(t, chain, c, resp, true, 1)
				if (chain == "responses_to_chat" && ending == "read_error") || (chain == "chat_to_responses" && (ending == "eof" || ending == "read_error" || ending == "upstream_error")) {
					require.Error(t, err)
				} else {
					require.NoError(t, err, "保留原 owner 的返回合同")
				}
				require.NotNil(t, result)
				if ending == "complete_then_cancel" {
					cancel()
				}
				require.Equal(t, want, result.UsageTiming.CompletionStatus)
				require.NotNil(t, result.UsageTiming.StrictFirstTokenMs)
				require.Equal(t, 5, result.Usage.InputTokens)
				if strings.Contains(ending, "complete") {
					require.Equal(t, 7, result.Usage.OutputTokens)
					require.Equal(t, UsageSourceUpstreamFinal, result.UsageTiming.UsageSource)
				} else {
					require.Equal(t, 2, result.Usage.OutputTokens)
					require.Equal(t, UsageSourceUpstreamPartial, result.UsageTiming.UsageSource)
				}
			})
		}
	}
}

func TestChatUsageTiming_ResponsesAcceptedUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		for _, tc := range []struct {
			name, final string
			output      int
			audio       *int
		}{
			{name: "nested_terminal_overrides_top", final: `{"type":"response.completed","usage":{"input_tokens":99,"output_tokens":99,"output_tokens_details":{"audio_tokens":9}},"response":{"status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":7,"output_tokens_details":{"audio_tokens":3}}}}`, output: 7, audio: intPointer(3)},
			{name: "top_zero_overrides_progressive", final: `{"type":"response.completed","usage":{"input_tokens":0,"output_tokens":0},"response":{"status":"completed","output":[]}}`, output: 0},
			{name: "missing_final_split_discards_partial", final: `{"type":"response.completed","response":{"status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":7}}}`, output: 7},
			{name: "completion_alias_split", final: `{"type":"response.completed","response":{"status":"completed","output":[],"usage":{"prompt_tokens":5,"completion_tokens":7,"completion_tokens_details":{"audio_tokens":2}}}}`, output: 7, audio: intPointer(2)},
		} {
			t.Run(map[bool]string{false: "json/", true: "sse/"}[stream]+tc.name, func(t *testing.T) {
				partial := passthroughSSEData(`{"type":"response.output_text.delta","delta":"answer","usage":{"input_tokens":5,"output_tokens":2,"output_tokens_details":{"audio_tokens":4}}}`)
				c, resp := timingHTTPContext(partial + passthroughSSEData(tc.final))
				result, err := runChatTimingOwner(t, "chat_to_responses", c, resp, stream, 1)
				require.NoError(t, err)
				require.Equal(t, tc.output, result.Usage.OutputTokens)
				require.Equal(t, tc.audio, result.UsageTiming.AudioOutputTokens)
				require.Equal(t, UsageSourceUpstreamFinal, result.UsageTiming.UsageSource)
				require.NotNil(t, result.UsageTiming.StrictFirstTokenMs, "buffered 空终态从真实增量重建完整内容")
			})
		}
	}
}

func TestChatUsageTiming_RawForwardMultipleChoices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ending := range []string{"stop", "length", "content_filter", "missing"} {
		t.Run(ending, func(t *testing.T) {
			body := passthroughSSEData(`{"choices":[{"index":0,"delta":{"content":"one"},"finish_reason":"stop"},{"index":1,"delta":{"content":"two"}}]}`)
			if ending != "missing" {
				body += passthroughSSEData(`{"choices":[{"index":1,"delta":{},"finish_reason":"` + ending + `"}]}`)
			}
			body += passthroughSSEData(`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":7}}`) + "data: [DONE]\n\n"
			request := []byte(`{"model":"company-coding-model","n":2,"messages":[{"role":"user","content":"hello"}],"stream":true}`)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(request)))
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			result, err := svc.ForwardAsChatCompletions(context.Background(), c, forceChatResponsesFallbackAccount(), request, "", "")
			require.NoError(t, err, "已有 raw owner 收到 usage 或 DONE 后仍正常返回")
			want := CompletionStatusInterrupted
			if ending == "stop" {
				want = CompletionStatusCompleted
			}
			require.Equal(t, want, result.UsageTiming.CompletionStatus)
			require.Equal(t, body, rec.Body.String(), "透传输出协议保持原样")
			require.Equal(t, 7, result.Usage.OutputTokens)
			if ending == "missing" {
				require.Equal(t, UsageSourceUpstreamPartial, result.UsageTiming.UsageSource)
			} else {
				require.Equal(t, UsageSourceUpstreamFinal, result.UsageTiming.UsageSource)
			}
		})
	}
}

func TestChatUsageTiming_UncollectedMessagesStayLegacy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig()}
	for _, stream := range []bool{false, true} {
		c, resp := timingHTTPContext(map[bool]string{false: chatTimingJSON, true: chatTimingSSE}[stream])
		var result *OpenAIForwardResult
		var err error
		if stream {
			result, err = svc.streamChatCompletionsAsAnthropic(c, resp, "model", "model", "model", nil, nil, time.Now())
		} else {
			result, err = svc.bufferChatCompletionsAsAnthropic(c, resp, "model", "model", "model", nil, nil, time.Now())
		}
		require.NoError(t, err)
		require.Equal(t, 7, result.Usage.OutputTokens)
		require.Zero(t, result.UsageTiming.TimingVersion)
	}
	c, resp := timingHTTPContext(passthroughSSEData(timingFinalEvent))
	result, err := svc.handleAnthropicBufferedStreamingResponse(resp, c, rawChatCompletionsTestAccount(), "model", "model", "model", time.Now())
	require.NoError(t, err)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Zero(t, result.UsageTiming.TimingVersion)
}

func TestChatUsageTiming_UsageOnlyDoesNotInventOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, chain := range []string{"raw_chat", "responses_to_chat", "chat_to_responses"} {
		t.Run(chain, func(t *testing.T) {
			body := passthroughSSEData(`{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":7}}`) + "data: [DONE]\n\n"
			wantStatus, wantSource := CompletionStatusInterrupted, UsageSourceUpstreamPartial
			if chain == "chat_to_responses" {
				body = passthroughSSEData(timingFinalEvent)
				wantStatus, wantSource = CompletionStatusCompleted, UsageSourceUpstreamFinal
			}
			c, resp := timingHTTPContext(body)
			result, err := runChatTimingOwner(t, chain, c, resp, true, 1)
			require.NoError(t, err)
			require.Nil(t, result.UsageTiming.StrictFirstTokenMs)
			require.Nil(t, result.UsageTiming.LastTokenMs)
			require.Nil(t, result.UsageTiming.FirstOutputMs)
			require.Equal(t, wantStatus, result.UsageTiming.CompletionStatus)
			require.Equal(t, wantSource, result.UsageTiming.UsageSource)
			require.Equal(t, 7, result.Usage.OutputTokens)
		})
	}
}

func TestChatUsageTiming_ForwardAttemptsStayIsolated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, chain := range []string{"raw_chat", "responses_to_chat", "chat_to_responses"} {
		t.Run(chain, func(t *testing.T) {
			partial := passthroughSSEData(`{"choices":[{"index":0,"delta":{"content":"partial"}}]}`)
			final := passthroughSSEData(`{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":7}}`) + "data: [DONE]\n\n"
			account := forceChatResponsesFallbackAccount()
			if chain == "chat_to_responses" {
				partial = passthroughSSEData(`{"type":"response.output_text.delta","delta":"partial"}`)
				final = passthroughSSEData(timingFinalEvent)
				account = rawChatCompletionsTestAccount()
				account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: true}
			}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: &chatTimingSequenceReader{chunks: []string{partial}, err: errors.New("connection reset by peer")}},
				{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(final))},
			}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			forward := func() (*OpenAIForwardResult, error) {
				request := `{"model":"company-coding-model","messages":[{"role":"user","content":"hello"}],"stream":true}`
				if chain == "responses_to_chat" {
					request = `{"model":"company-coding-model","input":"hello","stream":true}`
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(request))
				if chain == "responses_to_chat" {
					return svc.Forward(context.Background(), c, account, []byte(request))
				}
				return svc.ForwardAsChatCompletions(context.Background(), c, account, []byte(request), "", "")
			}
			first, err := forward()
			require.Error(t, err)
			require.NotNil(t, first)
			require.NotNil(t, first.UsageTiming.StrictFirstTokenMs)
			require.Equal(t, CompletionStatusInterrupted, first.UsageTiming.CompletionStatus)
			next, err := forward()
			require.NoError(t, err)
			require.Equal(t, CompletionStatusCompleted, next.UsageTiming.CompletionStatus)
			require.Equal(t, UsageSourceUpstreamFinal, next.UsageTiming.UsageSource)
			require.Nil(t, next.UsageTiming.StrictFirstTokenMs, "下一次 attempt 不借用前次输出，也不从聚合 completed 补造时点")
			require.Nil(t, next.UsageTiming.LastTokenMs)
			require.Equal(t, CompletionStatusInterrupted, first.UsageTiming.CompletionStatus)
		})
	}
}

func TestChatUsageTiming_InvalidAliasAudioRemainsUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		partial := passthroughSSEData(`{"type":"response.output_text.delta","delta":"answer"}`)
		final := passthroughSSEData(`{"type":"response.completed","response":{"status":"completed","output":[],"usage":{"prompt_tokens":5,"completion_tokens":7,"completion_tokens_details":{"audio_tokens":-1}}}}`)
		c, resp := timingHTTPContext(partial + final)
		result, err := runChatTimingOwner(t, "chat_to_responses", c, resp, stream, 1)
		require.NoError(t, err)
		require.Nil(t, result.UsageTiming.AudioOutputTokens, "非法拆分不能因为 output 是空数组而猜为零")
	}
}

func TestChatUsageTiming_PartialAudioAliasSurvivesFinalMissingSplit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, stream := range []bool{false, true} {
		for _, location := range []string{"top", "nested"} {
			t.Run(map[bool]string{false: "json/", true: "sse/"}[stream]+location, func(t *testing.T) {
				usage := `{"prompt_tokens":5,"completion_tokens":4,"completion_tokens_details":{"audio_tokens":3}}`
				partial := `{"type":"response.output_text.delta","delta":"answer","usage":` + usage + `}`
				if location == "nested" {
					partial = `{"type":"response.output_text.delta","delta":"answer","response":{"usage":` + usage + `}}`
				}
				final := `{"type":"response.completed","response":{"status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":7}}}`
				c, resp := timingHTTPContext(passthroughSSEData(partial) + passthroughSSEData(final))
				result, err := runChatTimingOwner(t, "chat_to_responses", c, resp, stream, 1)
				require.NoError(t, err)
				require.Equal(t, OpenAIUsage{InputTokens: 5, OutputTokens: 7}, result.Usage)
				require.Equal(t, CompletionStatusCompleted, result.UsageTiming.CompletionStatus)
				require.Equal(t, UsageSourceUpstreamFinal, result.UsageTiming.UsageSource)
				require.Nil(t, result.UsageTiming.AudioOutputTokens, "已接受的部分正音频是持续事实；最终缺少拆分不沿用部分值也不猜零")
			})
		}
	}
}
