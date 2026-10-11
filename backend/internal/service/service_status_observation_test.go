package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
)

func TestServiceStatusSourceHTTPRetryUsesFinalS1Owner(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 0)
	start := time.Now().Add(-time.Second).UTC()
	account := &Account{Platform: PlatformOpenAI}
	first := newResponsesOutputTiming(ctx, start, account)
	first.observeEvent([]byte(`{"type":"response.failed","response":{"error":{"code":"rate_limit_exceeded"}}}`), "", start.Add(100*time.Millisecond))
	failed := first.snapshot(false).ServiceStatusObservation
	first.stop()
	require.Equal(t, "provider_capacity", *failed.ReasonCode)
	attempt := ServiceStatusAttemptObservation(ctx, "provider_capacity", failed.ObservedAt)
	require.Equal(t, "attempt", attempt.EventRole)
	require.Nil(t, attempt.TerminalKind)

	second := newResponsesOutputTiming(ctx, start, account)
	second.observeEvent([]byte(`{"type":"response.completed","response":{"status":"completed","output":[],"usage":{"input_tokens":0,"output_tokens":0}}}`), "", start.Add(500*time.Millisecond))
	final := second.snapshot(false)
	require.Equal(t, failed.ObservationKey, final.ServiceStatusObservation.ObservationKey)
	require.Nil(t, final.ServiceStatusObservation.LogicalTurn)
	require.Equal(t, CompletionStatusCompleted, *final.ServiceStatusObservation.TerminalKind)
	require.Equal(t, final.CompletionStatus, *final.ServiceStatusObservation.TerminalKind)
	require.Equal(t, start.Add(500*time.Millisecond), final.ServiceStatusObservation.ObservedAt)
	CommitServiceStatusObservation(ctx, final.ServiceStatusObservation)
	require.Equal(t, final.ServiceStatusObservation, ServiceStatusFinalObservation(ctx, ""))
	second.stop()
}

func TestServiceStatusSourceCancellationOrderMatchesS1(t *testing.T) {
	for _, cancelFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancel_then_drain", false: "completed_then_close"}[cancelFirst], func(t *testing.T) {
			ctx := WithServiceStatusRequest(context.Background(), 0)
			start := time.Now().Add(-time.Second).UTC()
			observer := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
			if cancelFirst {
				observer.clientDisconnected()
			}
			observer.observeEvent([]byte(`{"type":"response.completed","response":{"status":"completed"}}`), "", start.Add(400*time.Millisecond))
			before := observer.snapshot(false)
			if !cancelFirst {
				observer.clientDisconnected()
			}
			after := observer.snapshot(true)
			require.Equal(t, before.ServiceStatusObservation, after.ServiceStatusObservation, "关闭不能改写已冻结终态和时点")
			require.Equal(t, after.CompletionStatus, *after.ServiceStatusObservation.TerminalKind)
			if cancelFirst {
				require.Equal(t, CompletionStatusClientDisconnected, after.CompletionStatus)
			} else {
				require.Equal(t, CompletionStatusCompleted, after.CompletionStatus)
			}
			observer.stop()
		})
	}
}

func TestServiceStatusSourceWSTurnKeysSurviveProxyRetry(t *testing.T) {
	initial := WithServiceStatusRequest(context.Background(), 0)
	first := EnsureServiceStatusTurn(initial, 1)
	retry := EnsureServiceStatusTurn(first, 1)
	require.Same(t, serviceStatusOwner(first), serviceStatusOwner(retry))
	second := EnsureServiceStatusTurn(retry, 2)
	firstFact := ServiceStatusAttemptObservation(first, "provider_capacity", time.Now())
	secondFact := ServiceStatusAttemptObservation(second, "", time.Now())
	require.NotEqual(t, firstFact.ObservationKey, secondFact.ObservationKey)
	require.Equal(t, 1, *firstFact.LogicalTurn)
	require.Equal(t, 2, *secondFact.LogicalTurn)
	freshHTTP := WithServiceStatusRequest(first, 0)
	require.NotEqual(t, firstFact.ObservationKey, ServiceStatusAttemptObservation(freshHTTP, "", time.Now()).ObservationKey)
}

func TestServiceStatusSourcePassthroughRegistryUsesAdmittedTurn(t *testing.T) {
	start := time.Now().Add(-time.Second).UTC()
	firstCtx := WithServiceStatusRequest(context.Background(), 1)
	registry := &responsesWSTurnTimings{ctx: firstCtx, account: &Account{Platform: PlatformOpenAI}, source: serviceStatusOwner(firstCtx)}
	defer registry.close()
	registry.observe(openaiwsv2.RelayObservedEvent{ResponseID: "resp_first", StartedAt: start, ObservedAt: start.Add(100 * time.Millisecond), Payload: []byte(`{"type":"response.failed","response":{"id":"resp_first","error":{"code":"rate_limit_exceeded"}}}`)})
	first := registry.take("resp_first")
	secondCtx := EnsureServiceStatusTurn(firstCtx, 2)
	registry.bindSource(secondCtx)
	registry.observe(openaiwsv2.RelayObservedEvent{ResponseID: "resp_second", StartedAt: start.Add(200 * time.Millisecond), ObservedAt: start.Add(300 * time.Millisecond), Payload: []byte(`{"type":"response.completed","response":{"id":"resp_second","status":"completed"}}`)})
	second := registry.take("resp_second")
	require.Equal(t, CompletionStatusUpstreamError, *first.ServiceStatusObservation.TerminalKind)
	require.Equal(t, CompletionStatusCompleted, *second.ServiceStatusObservation.TerminalKind)
	require.Equal(t, 1, *first.ServiceStatusObservation.LogicalTurn)
	require.Equal(t, 2, *second.ServiceStatusObservation.LogicalTurn)
	require.NotEqual(t, first.ServiceStatusObservation.ObservationKey, second.ServiceStatusObservation.ObservationKey)
}

func TestServiceStatusSourceMetadataContractAndClone(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 0)
	fact := ServiceStatusAttemptObservation(ctx, "", time.Now().UTC())
	raw := MarshalServiceStatusObservation(fact)
	require.NotNil(t, raw)
	var fields map[string]any
	require.NoError(t, json.Unmarshal([]byte(*raw), &fields))
	require.Len(t, fields, 7)
	require.Contains(t, fields, "reason_code")
	require.Nil(t, fields["logical_turn"])
	require.Nil(t, fields["terminal_kind"])
	require.Nil(t, fields["reason_code"])
	decoded, err := DecodeServiceStatusObservation([]byte(*raw))
	require.NoError(t, err)
	require.Equal(t, fact, decoded)
	invalid := fact.Clone()
	invalid.SchemaVersion = 2
	require.Error(t, invalid.Validate())
	invalid = fact.Clone()
	kind := CompletionStatusCompleted
	invalid.TerminalKind = &kind
	require.Error(t, invalid.Validate())
	turn := 2
	fact.LogicalTurn = &turn
	copy := (UsageTiming{ServiceStatusObservation: fact}).Clone()
	turn = 99
	require.Equal(t, 2, *copy.ServiceStatusObservation.LogicalTurn)
	// 元数据是内部字段，直接序列化领域Timing也不暴露。
	encoded, err := json.Marshal(copy)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "observation_key")
}

func TestServiceStatusSourceReasonRequiresActualOwner(t *testing.T) {
	for _, code := range []string{"INVALID_API_KEY", "INSUFFICIENT_BALANCE", "API_KEY_QUOTA_EXHAUSTED"} {
		ctx := WithServiceStatusRequest(context.Background(), 0)
		local := ServiceStatusFinalObservation(ctx, ServiceStatusLocalErrorReason(code))
		require.Equal(t, "user_rejected", *local.TerminalKind)
	}
	require.Equal(t, "provider_authentication", ServiceStatusProviderReason(401, ""))
	require.Equal(t, "provider_quota", ServiceStatusProviderReason(402, ""))
	require.Equal(t, "provider_capacity", ServiceStatusProviderReason(429, ""))
	require.Equal(t, "unclassified", ServiceStatusProviderReason(403, ""))
	require.Empty(t, ServiceStatusLocalErrorReason("some quota text"))
}

func TestServiceStatusSourceRawCapacityTerminalCodes(t *testing.T) {
	for _, code := range []string{"server_is_overloaded", "slow_down"} {
		for _, eventType := range []string{"response.failed", "error"} {
			t.Run(eventType+"/"+code, func(t *testing.T) {
				ctx := WithServiceStatusRequest(context.Background(), 1)
				start := time.Now().UTC()
				observer := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
				defer observer.stop()
				errorFields := map[string]any{"code": code}
				frame := map[string]any{"type": eventType}
				if eventType == "response.failed" {
					frame["response"] = map[string]any{"status": "failed", "error": errorFields}
				} else {
					frame["error"] = errorFields
				}
				payload, err := json.Marshal(frame)
				require.NoError(t, err)
				observer.observeEvent(payload, "", start.Add(time.Millisecond))
				fact := observer.snapshot(false).ServiceStatusObservation
				require.NotNil(t, fact)
				require.Equal(t, CompletionStatusUpstreamError, *fact.TerminalKind)
				require.Equal(t, "provider_capacity", *fact.ReasonCode, "按原始上游稳定码归因，不依赖客户端改写或HTTP状态")
				stored := MarshalServiceStatusObservation(fact)
				require.NotNil(t, stored)
				decoded, err := DecodeServiceStatusObservation([]byte(*stored))
				require.NoError(t, err)
				require.Equal(t, fact, decoded)
			})
		}
	}
}

func TestServiceStatusSourceExplicitRawTerminalStatus(t *testing.T) {
	for _, eventType := range []string{"response.failed", "error", "json"} {
		for _, tc := range []struct {
			name, field string
			status      any
			want        string
		}{
			{"status_code_500", "status_code", 500, "provider_5xx"},
			{"status_503", "status", 503, "provider_5xx"},
			{"missing", "", nil, "unclassified"},
			{"string_is_not_status", "status_code", "500", "unclassified"},
			{"fraction_is_not_status", "status", 500.5, "unclassified"},
		} {
			t.Run(eventType+"/"+tc.name, func(t *testing.T) {
				ctx := WithServiceStatusRequest(context.Background(), 1)
				start := time.Now().UTC()
				observer := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
				defer observer.stop()
				errorFields := map[string]any{"code": "server_error"}
				if tc.field != "" {
					errorFields[tc.field] = tc.status
				}
				frame := map[string]any{"type": eventType}
				if eventType == "response.failed" {
					frame["response"] = map[string]any{"status": "failed", "error": errorFields}
				} else {
					frame["error"] = errorFields
					if eventType == "json" {
						frame["status"] = "failed"
					}
				}
				payload, err := json.Marshal(frame)
				require.NoError(t, err)
				if eventType == "json" {
					observer.observeJSON(payload, start.Add(time.Millisecond))
				} else {
					observer.observeEvent(payload, "", start.Add(time.Millisecond))
				}
				fact := observer.snapshot(false).ServiceStatusObservation
				require.NotNil(t, fact)
				require.Equal(t, CompletionStatusUpstreamError, *fact.TerminalKind)
				require.Equal(t, tc.want, *fact.ReasonCode)
				stored := MarshalServiceStatusObservation(fact)
				require.NotNil(t, stored)
				decoded, err := DecodeServiceStatusObservation([]byte(*stored))
				require.NoError(t, err)
				require.Equal(t, fact, decoded)
			})
		}
	}
}

func TestServiceStatusSourceCancelledJSONCannotReplaceTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(WithServiceStatusRequest(context.Background(), 1))
	start := time.Now().UTC()
	observer := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
	defer observer.stop()
	cancel()
	observer.observeJSON([]byte(`{"status":"failed","error":{"code":"server_error","status_code":500}}`), start.Add(time.Millisecond))
	fact := observer.snapshot(false).ServiceStatusObservation
	require.NotNil(t, fact)
	require.Equal(t, CompletionStatusClientDisconnected, *fact.TerminalKind)
	require.Nil(t, fact.ReasonCode, "已冻结的完成/取消终态只按 kind 分类")
}

func TestServiceStatusSourceFinalOwnerPreservesS1AndDoesNotGuessOpsStatus(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 0)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil).WithContext(ctx)
	// 通用 Ops 状态可能是本地合成 502，不能凭它判供应商 5xx。
	setOpsUpstreamError(c, 502, "synthetic", "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{Platform: PlatformOpenAI, Kind: "failover", UpstreamStatusCode: 502})
	unknown := ServiceStatusFinalObservation(ctx, "")
	require.Equal(t, CompletionStatusUnknown, *unknown.TerminalKind)
	require.Equal(t, "terminal_missing", *unknown.ReasonCode)

	ctx = WithServiceStatusRequest(context.Background(), 0)
	start := time.Now().UTC()
	observer := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
	observer.observeEvent([]byte(`{"type":"response.completed","response":{"status":"completed"}}`), "", start.Add(time.Millisecond))
	before := observer.snapshot(false).ServiceStatusObservation
	final := ServiceStatusFinalObservation(ctx, "client_cancelled")
	require.Equal(t, before, final)
	observer.stop()
}

func TestServiceStatusSourceLatePriorAttemptCannotReplaceCurrentFact(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 0)
	start := time.Now().UTC()
	previous := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
	current := newResponsesOutputTiming(ctx, start, &Account{Platform: PlatformOpenAI})
	current.observeEvent([]byte(`{"type":"response.failed","response":{"error":{"code":"rate_limit_exceeded"}}}`), "", start.Add(time.Millisecond))
	previous.observeEvent([]byte(`{"type":"response.completed","response":{"status":"completed"}}`), "", start.Add(2*time.Millisecond))
	previous.stop()
	final := ServiceStatusFinalObservation(ctx, "")
	require.Equal(t, CompletionStatusUpstreamError, *final.TerminalKind)
	require.Equal(t, "provider_capacity", *final.ReasonCode)
	require.Equal(t, start.Add(time.Millisecond), final.ObservedAt)
	current.stop()
}

// 若意外进入写入，这个嵌入接口会 panic，证明失败发生在源持久化之前。
type serviceStatusUncalledUsageRepo struct{ UsageLogRepository }

func TestServiceStatusSourceUsageFailureBeforePersistenceIsObservable(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 0)
	fact := ServiceStatusAttemptObservation(ctx, "", time.Now())
	parentID := int64(100)
	s := &OpenAIGatewayService{
		accountRepo:  newStubCredRepo(&Account{ID: parentID, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}),
		usageLogRepo: &serviceStatusUncalledUsageRepo{},
	}
	before := time.Now()
	err := s.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result:  &OpenAIForwardResult{UsageTiming: UsageTiming{ServiceStatusObservation: fact}},
		Account: &Account{ID: 200, ParentAccountID: &parentID, Platform: PlatformOpenAI, Type: AccountTypeOAuth},
	})
	require.Error(t, err)
	require.True(t, ServiceStatusSourceErrorSince(before))
}

func TestServiceStatusSourceHTTPBridgeUsesActualUpstreamFailure(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		transport    bool
	}{
		{name: "transport", reason: "service_transport", transport: true},
		{name: "http_500", reason: "provider_5xx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithServiceStatusRequest(context.Background(), 1)
			upstream := &httpUpstreamRecorder{}
			if tc.transport {
				upstream.err = io.EOF
			} else {
				upstream.resp = &http.Response{StatusCode: 500, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"code":"server_error","message":"failure"}}`))}
			}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil).WithContext(ctx)
			payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi"}`)
			result, err := svc.proxyOpenAIWSHTTPBridgeTurn(ctx, c, &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}, "sk-test", payload, len(payload), "gpt-5", "", "", "", "", 1, func([]byte) error { return nil })
			require.Error(t, err)
			require.Nil(t, result, "失败不能制造用量结果")
			fact := ServiceStatusFinalObservation(ctx, "")
			require.Equal(t, CompletionStatusUpstreamError, *fact.TerminalKind)
			require.Equal(t, tc.reason, *fact.ReasonCode, "只采纳实际上游请求 owner 的事实")
		})
	}
}

func TestServiceStatusSourceHTTPBridgeRepairUsesRecoveredTerminal(t *testing.T) {
	ctx := WithServiceStatusRequest(context.Background(), 2)
	upstream := &httpUpstreamSequenceRecorder{responses: []*http.Response{
		{StatusCode: 400, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"code":"invalid_parameter","param":"truncation","message":"Unsupported parameter: truncation"}}`))},
		{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_retry\",\"status\":\"completed\",\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n"))},
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil).WithContext(ctx)
	payload := []byte(`{"type":"response.create","model":"gpt-5","input":"hi","truncation":"auto"}`)
	result, err := svc.proxyOpenAIWSHTTPBridgeTurn(ctx, c, &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1}, "sk-test", payload, len(payload), "gpt-5", "", "", "", "", 2, func([]byte) error { return nil })
	require.NoError(t, err)
	require.Equal(t, 2, upstream.callCount)
	require.Equal(t, CompletionStatusCompleted, *result.UsageTiming.ServiceStatusObservation.TerminalKind)
	CommitServiceStatusObservation(ctx, result.UsageTiming.ServiceStatusObservation)
	require.Equal(t, result.UsageTiming.ServiceStatusObservation, ServiceStatusFinalObservation(ctx, ""), "已恢复终态不能保留修复前失败")
}

func TestServiceStatusSourcePassthroughInterleavedResponseIDs(t *testing.T) {
	start := time.Now().UTC()
	firstCtx := WithServiceStatusRequest(context.Background(), 1)
	secondCtx := EnsureServiceStatusTurn(firstCtx, 2)
	r := &responsesWSTurnTimings{ctx: firstCtx, account: &Account{Platform: PlatformOpenAI}, source: serviceStatusOwner(firstCtx)}
	defer r.close()
	r.observe(openaiwsv2.RelayObservedEvent{ResponseID: "first", StartedAt: start, ObservedAt: start.Add(time.Millisecond), Payload: []byte(`{"type":"response.output_text.delta","delta":"first"}`)})
	r.bindSource(secondCtx)
	r.observe(openaiwsv2.RelayObservedEvent{ResponseID: "second", StartedAt: start.Add(time.Second), ObservedAt: start.Add(time.Second + time.Millisecond), Payload: []byte(`{"type":"response.output_text.delta","delta":"second"}`)})
	r.observe(openaiwsv2.RelayObservedEvent{ResponseID: "first", StartedAt: start, ObservedAt: start.Add(2 * time.Second), Payload: []byte(`{"type":"response.completed","response":{"status":"completed"}}`)})
	r.disconnect()
	first, second := r.take("first"), r.take("second")
	require.Equal(t, 1, *first.ServiceStatusObservation.LogicalTurn)
	require.Equal(t, 2, *second.ServiceStatusObservation.LogicalTurn)
	require.NotEqual(t, first.ServiceStatusObservation.ObservationKey, second.ServiceStatusObservation.ObservationKey)
	require.Equal(t, CompletionStatusCompleted, *first.ServiceStatusObservation.TerminalKind)
	require.Equal(t, CompletionStatusClientDisconnected, *second.ServiceStatusObservation.TerminalKind)
	require.Equal(t, start.Add(2*time.Second), first.ServiceStatusObservation.ObservedAt)
}
