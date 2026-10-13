//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const partialOpenAIUsageJSON = `{"input_tokens":1200,"output_tokens":300,"input_tokens_details":{"cached_tokens":400,"cache_write_tokens":100}}`

func TestOpenAIForwardPartialUsageRecordsAndBillsFailedStream(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		name := "native"
		if passthrough {
			name = "passthrough"
		}
		for _, failure := range []string{"failed", "read_error", "cancelled", "auth_403"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
				billingRepo := &openAIRecordUsageBillingRepoStub{}
				svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				svc.cfg.Gateway.OpenAIStreamEarlyPreamble = true
				svc.cache = &stubGatewayCache{}
				auth := newOpenAI403TestHarness(t, 5151, 1)
				svc.rateLimitService = auth.svc
				account := newOpenAIImageGenerationControlTestAccount()
				account.Type = AccountTypeOAuth
				account.Credentials = map[string]any{"access_token": "test-token"}
				account.Extra = map[string]any{"openai_passthrough": passthrough}
				payload := earlyPreambleCreated
				var body io.ReadCloser
				if failure == "failed" || failure == "auth_403" {
					errorJSON := `{"code":"server_error","message":"busy"}`
					if failure == "auth_403" {
						errorJSON = `{"status_code":403,"type":"authentication_error","code":"access_denied","message":"account access forbidden"}`
					}
					payload += "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_early\",\"model\":\"gpt-5.1\",\"usage\":" + partialOpenAIUsageJSON + ",\"error\":" + errorJSON + "}}\n\n"
					body = io.NopCloser(strings.NewReader(payload))
				} else {
					payload += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\",\"usage\":" + partialOpenAIUsageJSON + "}\n\n"
					readErr := error(io.ErrUnexpectedEOF)
					if failure == "cancelled" {
						readErr = context.Canceled
					}
					body = &streamReadCloser{payload: []byte(payload), err: readErr}
				}
				tracked := &passthroughCloseTrackingReadCloser{Reader: body}
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}, "X-Request-Id": {"req-partial"}}, Body: tracked}}
				svc.httpUpstream = upstream
				request := []byte(`{"model":"gpt-5.1","stream":true,"instructions":"test","input":"hello","reasoning":{"effort":"low"}}`)
				c := newOpenAIRejectedFieldTestContext(request)
				result, err := svc.Forward(context.Background(), c, account, request)
				require.Error(t, err)
				var failover *UpstreamFailoverError
				require.False(t, errors.As(err, &failover))
				require.NotNil(t, result, "失败流中已观测的用量必须穿过 Forward 边界")
				require.True(t, tracked.closed)
				require.Len(t, upstream.requests, 1)
				require.Equal(t, "req-partial", result.RequestID)
				require.Equal(t, "resp_early", result.ResponseID)
				require.Equal(t, "gpt-5.1", result.Model)
				require.Equal(t, "low", *result.ReasoningEffort)
				require.True(t, result.Stream)
				require.True(t, result.ForwardFailed)
				require.False(t, result.SucceededForScheduling())
				want := OpenAIUsage{InputTokens: 1200, OutputTokens: 300, CacheReadInputTokens: 400, CacheCreationInputTokens: 100}
				require.Equal(t, want, result.Usage)
				quota := &openAIRecordUsageAPIKeyQuotaStub{}
				apiKey := &APIKey{ID: 2, Quota: 100}
				err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: apiKey, User: &User{ID: 1}, Account: account, APIKeyService: quota, RequestPayloadHash: HashUsageRequestPayload(request)})
				require.NoError(t, err)
				require.Equal(t, 1, usageRepo.calls)
				require.Equal(t, 1, billingRepo.calls)
				require.Equal(t, 700, usageRepo.lastLog.InputTokens, "缓存桶不得重复计入普通输入")
				require.Equal(t, 300, usageRepo.lastLog.OutputTokens)
				require.Equal(t, 400, usageRepo.lastLog.CacheReadTokens)
				require.Equal(t, 100, usageRepo.lastLog.CacheCreationTokens)
				cost := expectedOpenAICost(t, svc, "gpt-5.1", want, 1.1)
				require.Positive(t, billingRepo.lastCmd.BalanceCost)
				require.InDelta(t, cost.ActualCost, billingRepo.lastCmd.BalanceCost, 1e-8)
				require.Equal(t, account.ID, billingRepo.lastCmd.AccountID)
				require.Equal(t, 700, billingRepo.lastCmd.InputTokens)
				require.Equal(t, billingRepo.lastCmd.BalanceCost, billingRepo.lastCmd.APIKeyQuotaCost)
				require.Empty(t, auth.counter.resetCalls, "失败流入账不能清除连续 403 计数")
				if failure == "auth_403" {
					require.Equal(t, 1, auth.counter.increments)
					require.Equal(t, 1, auth.repo.tempCalls)
				}
			})
		}
	}
}

func TestOpenAIForwardPartialUsageDoesNotBillUnobservedOrRetryableStream(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, retryable := range []bool{false, true} {
			name := "native"
			if passthrough {
				name = "passthrough"
			}
			if retryable {
				name += "/retryable"
			} else {
				name += "/no_usage"
			}
			t.Run(name, func(t *testing.T) {
				payload := earlyPreambleCreated
				if retryable {
					payload = ""
				}
				payload += "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"busy\"}"
				if retryable {
					payload += ",\"usage\":" + partialOpenAIUsageJSON
				}
				payload += "}}\n\n"
				upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(payload))}}
				svc := newOpenAIImageGenerationControlTestService(upstream)
				svc.cfg.Gateway.OpenAIStreamEarlyPreamble = true
				account := newOpenAIImageGenerationControlTestAccount()
				account.Extra = map[string]any{"openai_passthrough": passthrough}
				request := []byte(`{"model":"gpt-5.1","stream":true,"input":"hello"}`)
				result, err := svc.Forward(context.Background(), newOpenAIRejectedFieldTestContext(request), account, request)
				require.Error(t, err)
				var failover *UpstreamFailoverError
				require.Equal(t, retryable, errors.As(err, &failover))
				require.Nil(t, result, "无用量或准备换号重试的流不得产生额外账单结果")
			})
		}
	}
}
