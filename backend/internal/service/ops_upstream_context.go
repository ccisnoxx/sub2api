package service

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Gin context keys used by Ops error logger for capturing upstream error details.
// These keys are set by gateway services and consumed by handler/ops_error_logger.go.
const (
	OpsUpstreamStatusCodeKey         = "ops_upstream_status_code"
	OpsUpstreamErrorMessageKey       = "ops_upstream_error_message"
	OpsUpstreamErrorDetailKey        = "ops_upstream_error_detail"
	OpsUpstreamErrorsKey             = "ops_upstream_errors"
	OpsUpstreamModelKey              = "ops_upstream_model"
	OpsRoutingDiagnosticsAttemptKey  = "ops_routing_diagnostics_attempt"
	OpsUpstreamRoutingDiagnosticsKey = "ops_upstream_routing_diagnostics"

	// Optional stage latencies (milliseconds) for troubleshooting and alerting.
	OpsAuthLatencyMsKey      = "ops_auth_latency_ms"
	OpsRoutingLatencyMsKey   = "ops_routing_latency_ms"
	OpsUpstreamLatencyMsKey  = "ops_upstream_latency_ms"
	OpsResponseLatencyMsKey  = "ops_response_latency_ms"
	OpsTimeToFirstTokenMsKey = "ops_time_to_first_token_ms"
	// OpenAI WS 关键观测字段
	OpsOpenAIWSQueueWaitMsKey = "ops_openai_ws_queue_wait_ms"
	OpsOpenAIWSConnPickMsKey  = "ops_openai_ws_conn_pick_ms"
	OpsOpenAIWSConnReusedKey  = "ops_openai_ws_conn_reused"
	OpsOpenAIWSConnIDKey      = "ops_openai_ws_conn_id"

	// OpsSkipPassthroughKey 由 applyErrorPassthroughRule 在命中 skip_monitoring=true 的规则时设置。
	// ops_error_logger 中间件检查此 key，为 true 时跳过错误记录。
	OpsSkipPassthroughKey = "ops_skip_passthrough"

	// OpsStreamErrorKey 保存 handleStreamingAwareError 在「响应已固化为 HTTP 200 的 SSE 流」
	// 上就地(in-band)补发错误帧时记录的 OpsStreamError。因为 wire 状态码停留在 200，
	// ops_error_logger 的 status>=400 采集路径永远不会触发，这类流内失败
	//（例如等待并发槽位超时后回退的限流、Wait 后二次计费校验失败）本会在错误看板里隐形。
	OpsStreamErrorKey  = "ops_stream_error"
	OpsStreamErrorsKey = "ops_stream_errors"
	OpsStreamTurnKey   = "ops_stream_turn"

	// Client-side configuration denials should remain visible in ops_error_logs,
	// but should be excluded from SLA/error-rate calculations.
	// ResponseCommittedKey 由 handleErrorResponse 系列函数在写完 HTTP 错误响应后设置。
	// ensureForwardErrorResponse 检查此 key，为 true 时跳过兜底写入，避免在已完成的 JSON 后追加 SSE。
	ResponseCommittedKey = "response_committed"

	OpsClientBusinessLimitedKey                           = "ops_client_business_limited"
	OpsClientBusinessLimitedReasonKey                     = "ops_client_business_limited_reason"
	OpsClientBusinessLimitedReasonIPRestriction           = "api_key_ip_restriction"
	OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable  = "api_key_group_unavailable"
	OpsClientBusinessLimitedReasonAPIKeyGroupUnassigned   = "api_key_group_unassigned"
	OpsClientBusinessLimitedReasonLocalFeatureGate        = "local_feature_gate"
	OpsClientBusinessLimitedReasonLocalPolicyDenied       = "local_policy_denied"
	OpsClientBusinessLimitedReasonLocalModelConfiguration = "local_model_configuration"
)

func MarkResponseCommitted(c *gin.Context) { c.Set(ResponseCommittedKey, true) }

func IsResponseCommitted(c *gin.Context) bool {
	v, ok := c.Get(ResponseCommittedKey)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func SetOpsLatencyMs(c *gin.Context, key string, value int64) {
	if c == nil || strings.TrimSpace(key) == "" || value < 0 {
		return
	}
	c.Set(key, value)
}

// SetOpsUpstreamModel stores only the effective model slug for final Ops
// attribution. Call it immediately before an upstream attempt is dispatched.
func SetOpsUpstreamModel(c *gin.Context, model string) {
	if c == nil {
		return
	}
	if model = strings.TrimSpace(model); model != "" {
		c.Set(OpsUpstreamModelKey, model)
	}
}

// ClearOpsUpstreamModel invalidates attempt-scoped model attribution before a
// newly selected account starts credential resolution or upstream dispatch.
func ClearOpsUpstreamModel(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(OpsUpstreamModelKey, "")
}

type opsRoutingDiagnosticsAttempt struct {
	owner       *routingDiagnosticsRequest
	diagnostics *RoutingDiagnostics
}

// BindOpsRoutingDiagnosticsAttempt 在选定账号后绑定本次发送；重选不能改写旧失败。
func BindOpsRoutingDiagnosticsAttempt(c *gin.Context) {
	if c == nil || c.Request == nil {
		return
	}
	owner, _ := c.Request.Context().Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest)
	c.Set(OpsRoutingDiagnosticsAttemptKey, opsRoutingDiagnosticsAttempt{owner: owner, diagnostics: GetRoutingDiagnostics(c.Request.Context())})
}

func getOpsRoutingDiagnosticsAttempt(c *gin.Context) *RoutingDiagnostics {
	if c == nil || c.Request == nil {
		return nil
	}
	value, _ := c.Get(OpsRoutingDiagnosticsAttemptKey)
	bound, _ := value.(opsRoutingDiagnosticsAttempt)
	owner, _ := c.Request.Context().Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest)
	// beginProxy 可能先于 BeginOpsStreamTurn 推进逻辑 turn；归属不一致时旧连接绑定无效。
	if owner == nil || owner != bound.owner {
		return nil
	}
	return bound.diagnostics.Clone()
}

func MarkOpsClientBusinessLimited(c *gin.Context, reason string) {
	if c == nil {
		return
	}
	c.Set(OpsClientBusinessLimitedKey, true)
	if reason = strings.TrimSpace(reason); reason != "" {
		c.Set(OpsClientBusinessLimitedReasonKey, reason)
	}
}

func HasOpsClientBusinessLimited(c *gin.Context) bool {
	if c == nil {
		return false
	}
	v, ok := c.Get(OpsClientBusinessLimitedKey)
	if !ok {
		return false
	}
	marked, _ := v.(bool)
	return marked
}

func OpsClientBusinessLimitedReason(c *gin.Context) string {
	if c == nil {
		return ""
	}
	v, ok := c.Get(OpsClientBusinessLimitedReasonKey)
	if !ok {
		return ""
	}
	reason, _ := v.(string)
	return strings.TrimSpace(reason)
}

// OpsStreamError 描述承载在 2xx 响应上的带内错误：网关在响应状态已固化为 200 之后
// 就地以 SSE error 帧返回的错误（并发限流回退、Wait 后二次计费校验失败、流开始后才无
// 可用账号等），以及上游 2xx 正文或事件里携带的错误结果（NonStream 标记非流式正文）。
// 由于 HTTP 状态码停留在 2xx，ops_error_logger 中间件在 status<400 分支消费该标记并
// 补记错误日志；标记方是 handler.handleStreamingAwareError 或各 service 的带内检测。
type OpsStreamError struct {
	// ErrType 是写入 SSE 帧的对客错误类型（如 rate_limit_error / upstream_error / api_error）。
	ErrType string
	// Code 是可选的稳定错误分类；用于既保留通用 OpenAI error.type，又向客户端和 Ops
	// 暴露可编程判断的细分类（如 upstream_http2_stream_error）。
	Code string
	// Message 是写入 SSE 帧的对客错误消息。
	Message string
	// IntendedStatus 是流若未固化本应返回的 HTTP 状态码（如并发限流的 429）。
	// 默认仅用于错误分级；CountTowardsSLA 或 RequestScoped 为 true 时也作为 Ops 的逻辑状态码。
	IntendedStatus int
	// CountTowardsSLA 表示虽然 wire 状态已固化为 200，请求在应用语义上仍然失败，
	// Ops 应使用 IntendedStatus 计入错误率/SLA。
	CountTowardsSLA bool
	// Turn identifies a WebSocket turn. HTTP/SSE requests leave it at zero.
	Turn int
	// SkipMonitoring snapshots the rule decision for this visible failure.
	SkipMonitoring     bool
	AccountID          int64
	UpstreamModel      string
	UpstreamStatus     int
	UpstreamMessage    string
	UpstreamDetail     string
	UpstreamErrors     []*OpsUpstreamErrorEvent
	RoutingDiagnostics *RoutingDiagnostics
	// RequestScoped 表示该带内失败是请求级结果（如上游内容策略截停），与本请求此前的
	// 上游尝试无关：分类不受上游错误上下文影响、不快照也不落库上游归因、不继承透传规则的
	// skip_monitoring，按业务限制计，落库状态取 IntendedStatus 以便进入错误列表。
	RequestScoped bool
	// NonStream 表示带内信号来自非流式 2xx 响应体，落库 stream=false。
	NonStream bool
}

const maxOpsStreamErrorsPerRequest = 64

// BeginOpsStreamTurn scopes first-wins deduplication to one WebSocket turn.
func BeginOpsStreamTurn(c *gin.Context, turn int) {
	BeginOpsStreamTurnWithRoutingTurn(c, turn, turn)
}

// BeginOpsStreamTurnWithRoutingTurn 保留原转发 turn 的日志合同；诊断使用连接逻辑 turn。
func BeginOpsStreamTurnWithRoutingTurn(c *gin.Context, turn, routingTurn int) {
	if c == nil || turn <= 0 {
		return
	}
	c.Set(OpsStreamTurnKey, turn)
	newRoutingTurn := true
	if c.Request != nil {
		// 建连选号不代表本 turn 重新观察过池；每个逻辑 turn 使用新归属。
		previous := c.Request.Context()
		current := EnsureRoutingDiagnosticsTurn(previous, routingTurn)
		newRoutingTurn = current != previous
		c.Request = c.Request.WithContext(current)
	}
	// Rule and attempt state is turn-scoped on a long-lived WS connection.
	c.Set(OpsSkipPassthroughKey, false)
	c.Set(OpsUpstreamErrorsKey, []*OpsUpstreamErrorEvent{})
	c.Set(OpsUpstreamStatusCodeKey, 0)
	c.Set(OpsUpstreamErrorMessageKey, "")
	c.Set(OpsUpstreamErrorDetailKey, "")
	if newRoutingTurn {
		c.Set(OpsRoutingDiagnosticsAttemptKey, (*RoutingDiagnostics)(nil))
	}
	c.Set(OpsUpstreamRoutingDiagnosticsKey, (*RoutingDiagnostics)(nil))
}

// MarkOpsStreamError 记录一次就地 SSE 错误，供 ops 日志采集。
// 采用「首个标记生效」策略：同一请求若先后补发多帧（如上游透传错误后又追加通用兜底帧），
// 保留最先记录的根因错误，而不是被后续的 "Upstream request failed" 覆盖。
func MarkOpsStreamError(c *gin.Context, errType, message string, intendedStatus int) {
	markOpsStreamError(c, OpsStreamError{
		ErrType:        errType,
		Message:        message,
		IntendedStatus: intendedStatus,
	})
}

// MarkOpsStreamFailure records an in-band stream error that represents a failed
// request and therefore must count towards Ops error rate/SLA despite HTTP 200
// already being committed on the wire.
func MarkOpsStreamFailure(c *gin.Context, errType, code, message string, intendedStatus int) {
	markOpsStreamError(c, OpsStreamError{
		ErrType:         errType,
		Code:            code,
		Message:         message,
		IntendedStatus:  intendedStatus,
		CountTowardsSLA: true,
	})
}

// MarkOpsStreamErrorValue 以完整的 OpsStreamError 记录一次带内错误，供需要
// RequestScoped / NonStream 等附加语义的调用方使用；首个标记生效的规则不变。
// 调用方只填 ErrType / Code / Message / IntendedStatus / CountTowardsSLA / RequestScoped / NonStream。
// AccountID、UpstreamModel 与 Turn 由请求上下文接管；UpstreamStatus、UpstreamMessage、
// UpstreamDetail、UpstreamErrors 与 SkipMonitoring 在非 RequestScoped 时由上下文接管，
// RequestScoped 时被忽略。
func MarkOpsStreamErrorValue(c *gin.Context, streamErr OpsStreamError) {
	markOpsStreamError(c, streamErr)
}

func markOpsStreamError(c *gin.Context, streamErr OpsStreamError) {
	if c == nil {
		return
	}
	streamErr.ErrType = strings.TrimSpace(streamErr.ErrType)
	streamErr.Code = strings.TrimSpace(streamErr.Code)
	streamErr.Message = strings.TrimSpace(streamErr.Message)
	if !streamErr.RequestScoped {
		streamErr.SkipMonitoring = currentOpsFailureSkipMonitoring(c)
	}
	snapshotOpsStreamErrorContext(c, &streamErr)
	if GetOpenAIClientTransport(c) == OpenAIClientTransportWS {
		if value, ok := c.Get(OpsStreamTurnKey); ok {
			streamErr.Turn, _ = value.(int)
		}
		var errorsForRequest []OpsStreamError
		if value, ok := c.Get(OpsStreamErrorsKey); ok {
			errorsForRequest, _ = value.([]OpsStreamError)
		}
		if len(errorsForRequest) > 0 && errorsForRequest[len(errorsForRequest)-1].Turn == streamErr.Turn {
			return
		}
		errorsForRequest = append(errorsForRequest, streamErr)
		if len(errorsForRequest) > maxOpsStreamErrorsPerRequest {
			errorsForRequest = append([]OpsStreamError(nil), errorsForRequest[len(errorsForRequest)-maxOpsStreamErrorsPerRequest:]...)
		}
		c.Set(OpsStreamErrorsKey, errorsForRequest)
		c.Set(OpsStreamErrorKey, streamErr)
		return
	}
	if _, exists := c.Get(OpsStreamErrorKey); exists {
		return
	}
	c.Set(OpsStreamErrorKey, streamErr)
}

func snapshotOpsStreamErrorContext(c *gin.Context, streamErr *OpsStreamError) {
	if c == nil || streamErr == nil {
		return
	}
	if c.Request != nil {
		if accountID, ok := c.Request.Context().Value(ctxkey.AccountID).(int64); ok && accountID > 0 {
			streamErr.AccountID = accountID
		}
	}
	if value, ok := c.Get(OpsUpstreamModelKey); ok {
		streamErr.UpstreamModel, _ = value.(string)
		streamErr.UpstreamModel = strings.TrimSpace(streamErr.UpstreamModel)
	}
	streamErr.RoutingDiagnostics = nil
	if streamErr.RequestScoped {
		return
	}
	if c.Request != nil {
		streamErr.RoutingDiagnostics = GetRoutingDiagnostics(c.Request.Context())
	}
	if value, ok := c.Get(OpsUpstreamStatusCodeKey); ok {
		switch status := value.(type) {
		case int:
			streamErr.UpstreamStatus = status
		case int64:
			streamErr.UpstreamStatus = int(status)
		}
	}
	if value, ok := c.Get(OpsUpstreamErrorMessageKey); ok {
		streamErr.UpstreamMessage, _ = value.(string)
		streamErr.UpstreamMessage = strings.TrimSpace(streamErr.UpstreamMessage)
	}
	if value, ok := c.Get(OpsUpstreamErrorDetailKey); ok {
		streamErr.UpstreamDetail, _ = value.(string)
		streamErr.UpstreamDetail = strings.TrimSpace(streamErr.UpstreamDetail)
	}
	if streamErr.UpstreamStatus > 0 || streamErr.UpstreamMessage != "" || streamErr.UpstreamDetail != "" {
		value, _ := c.Get(OpsUpstreamRoutingDiagnosticsKey)
		d, _ := value.(*RoutingDiagnostics)
		streamErr.RoutingDiagnostics = d.Clone()
	}
	if value, ok := c.Get(OpsUpstreamErrorsKey); ok {
		if events, ok := value.([]*OpsUpstreamErrorEvent); ok {
			streamErr.UpstreamErrors = make([]*OpsUpstreamErrorEvent, 0, len(events))
			for _, event := range events {
				if event == nil {
					streamErr.UpstreamErrors = append(streamErr.UpstreamErrors, nil)
					continue
				}
				streamErr.UpstreamErrors = append(streamErr.UpstreamErrors, event.Clone())
			}
		}
	}
}

func currentOpsFailureSkipMonitoring(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if value, ok := c.Get(OpsSkipPassthroughKey); ok {
		if skip, _ := value.(bool); skip {
			return true
		}
	}
	if value, ok := c.Get(OpsUpstreamErrorsKey); ok {
		if events, ok := value.([]*OpsUpstreamErrorEvent); ok {
			for i := len(events) - 1; i >= 0; i-- {
				if events[i] != nil {
					return events[i].SkipMonitoring
				}
			}
		}
	}
	return false
}

// GetOpsStreamError 返回本请求记录的就地 SSE 错误（若有）。
func GetOpsStreamError(c *gin.Context) (OpsStreamError, bool) {
	if c == nil {
		return OpsStreamError{}, false
	}
	v, ok := c.Get(OpsStreamErrorKey)
	if !ok {
		return OpsStreamError{}, false
	}
	se, ok := v.(OpsStreamError)
	return cloneOpsStreamError(se), ok
}

func GetOpsStreamErrors(c *gin.Context) []OpsStreamError {
	if c == nil {
		return nil
	}
	if value, ok := c.Get(OpsStreamErrorsKey); ok {
		if errorsForRequest, ok := value.([]OpsStreamError); ok && len(errorsForRequest) > 0 {
			out := make([]OpsStreamError, len(errorsForRequest))
			for i, se := range errorsForRequest {
				out[i] = cloneOpsStreamError(se)
			}
			return out
		}
	}
	if streamErr, ok := GetOpsStreamError(c); ok {
		return []OpsStreamError{streamErr}
	}
	return nil
}

func cloneOpsStreamError(se OpsStreamError) OpsStreamError {
	se.RoutingDiagnostics = se.RoutingDiagnostics.Clone()
	if se.UpstreamErrors != nil {
		events := make([]*OpsUpstreamErrorEvent, len(se.UpstreamErrors))
		for i, event := range se.UpstreamErrors {
			events[i] = event.Clone()
		}
		se.UpstreamErrors = events
	}
	return se
}

// SetOpsUpstreamError is the exported wrapper for setOpsUpstreamError, used by
// handler-layer code (e.g. failover-exhausted paths) that needs to record the
// original upstream status code before mapping it to a client-facing code.
func SetOpsUpstreamError(c *gin.Context, upstreamStatusCode int, upstreamMessage, upstreamDetail string) {
	setOpsUpstreamError(c, upstreamStatusCode, upstreamMessage, upstreamDetail)
}

func setOpsUpstreamError(c *gin.Context, upstreamStatusCode int, upstreamMessage, upstreamDetail string) {
	if c == nil {
		return
	}
	c.Set(OpsUpstreamRoutingDiagnosticsKey, getOpsRoutingDiagnosticsAttempt(c))
	if upstreamStatusCode > 0 {
		c.Set(OpsUpstreamStatusCodeKey, upstreamStatusCode)
	}
	if msg := strings.TrimSpace(upstreamMessage); msg != "" {
		c.Set(OpsUpstreamErrorMessageKey, msg)
	}
	if detail := strings.TrimSpace(upstreamDetail); detail != "" {
		c.Set(OpsUpstreamErrorDetailKey, detail)
	}
}

// OpsUpstreamErrorEvent describes one upstream error attempt during a single gateway request.
// It is stored in ops_error_logs.upstream_errors as a JSON array.
type OpsUpstreamErrorEvent struct {
	AtUnixMs int64 `json:"at_unix_ms,omitempty"`

	// Passthrough 表示本次请求是否命中“原样透传（仅替换认证）”分支。
	// 该字段用于排障与灰度评估；存入 JSON，不涉及 DB schema 变更。
	Passthrough bool `json:"passthrough,omitempty"`

	// Context
	Platform    string `json:"platform,omitempty"`
	AccountID   int64  `json:"account_id,omitempty"`
	AccountName string `json:"account_name,omitempty"`

	// Proxy attribution is an immutable, credential-free snapshot of the route
	// used by this attempt. ProxyID is null for direct and unknown routes;
	// ProxyName distinguishes direct/no_proxy from unknown.
	// Never add proxy URLs or credentials to this event.
	ProxyID   *int64 `json:"proxy_id"`
	ProxyName string `json:"proxy_name"`

	// DroppedEarlierAttempts is set on the oldest retained event when queue
	// bounds forced earlier attempts of the same request to be discarded.
	DroppedEarlierAttempts int `json:"dropped_earlier_attempts,omitempty"`

	// Outcome
	UpstreamStatusCode int    `json:"upstream_status_code,omitempty"`
	UpstreamRequestID  string `json:"upstream_request_id,omitempty"`

	// UpstreamURL is the actual upstream URL that was called (host + path, query/fragment stripped).
	// Helps debug 404/routing errors by showing which endpoint was targeted.
	UpstreamURL string `json:"upstream_url,omitempty"`

	// Best-effort upstream response capture (sanitized+trimmed).
	UpstreamResponseBody string `json:"upstream_response_body,omitempty"`

	// Kind: http_error | request_error | retry_exhausted | failover
	Kind string `json:"kind,omitempty"`
	// Stage/Scope/Reason distinguish credential acquisition from inference
	// without overloading upstream_status_code with a synthetic HTTP status.
	Stage  string `json:"stage,omitempty"`
	Scope  string `json:"scope,omitempty"`
	Reason string `json:"reason,omitempty"`

	Message string `json:"message,omitempty"`
	Detail  string `json:"detail,omitempty"`

	// SkipMonitoring is request-local rule state. It is intentionally excluded
	// from persisted attempt JSON. The logger consults it only when this event is
	// the final client-visible failure; recovered attempts remain provider-health
	// telemetry and do not count as failed requests.
	SkipMonitoring     bool                `json:"-"`
	RoutingDiagnostics *RoutingDiagnostics `json:"routing_diagnostics,omitempty"`
}

func (ev *OpsUpstreamErrorEvent) Clone() *OpsUpstreamErrorEvent {
	if ev == nil {
		return nil
	}
	out := *ev
	out.RoutingDiagnostics = ev.RoutingDiagnostics.Clone()
	if ev.ProxyID != nil {
		id := *ev.ProxyID
		out.ProxyID = &id
	}
	return &out
}

const (
	opsProxyNameDirect  = "direct/no_proxy"
	opsProxyNameUnknown = "unknown"
	// opsProxyNameUnnamed labels a managed proxy whose name is blank. The
	// proxies.name column is NOT NULL/non-empty, so this is a defensive value.
	opsProxyNameUnnamed = "proxy"
)

func appendOpsUpstreamError(c *gin.Context, ev OpsUpstreamErrorEvent) {
	if c == nil {
		return
	}
	// 只携带实际发送绑定值，不能借用后来发生的选择评估。
	ev.RoutingDiagnostics = getOpsRoutingDiagnosticsAttempt(c)
	if ev.AtUnixMs <= 0 {
		ev.AtUnixMs = time.Now().UnixMilli()
	}
	ev.Platform = strings.TrimSpace(ev.Platform)
	normalizeOpsUpstreamProxyAttribution(&ev)
	ev.UpstreamRequestID = strings.TrimSpace(ev.UpstreamRequestID)
	ev.UpstreamResponseBody = strings.TrimSpace(ev.UpstreamResponseBody)
	ev.Kind = strings.TrimSpace(ev.Kind)
	ev.Stage = strings.TrimSpace(ev.Stage)
	ev.Scope = strings.TrimSpace(ev.Scope)
	ev.Reason = strings.TrimSpace(ev.Reason)
	ev.UpstreamURL = strings.TrimSpace(ev.UpstreamURL)
	ev.Message = strings.TrimSpace(ev.Message)
	ev.Detail = strings.TrimSpace(ev.Detail)
	if ev.Message != "" {
		ev.Message = sanitizeUpstreamErrorMessage(ev.Message)
	}

	var existing []*OpsUpstreamErrorEvent
	if v, ok := c.Get(OpsUpstreamErrorsKey); ok {
		if arr, ok := v.([]*OpsUpstreamErrorEvent); ok {
			existing = arr
		}
	}

	evCopy := ev
	existing = append(existing, &evCopy)
	c.Set(OpsUpstreamErrorsKey, existing)

	checkSkipMonitoringForUpstreamEvent(c, &evCopy)
}

// opsUpstreamProxyAttribution derives both attribution fields from one
// decision so they can never disagree. The event is assembled at the failure
// site; forwarding code builds managed proxy routes from Proxy only when the
// binding ID is also set, so the same rule decides the label here:
//
//   - nil account                     -> (nil, unknown)
//   - no binding or no hydrated Proxy -> (nil, direct/no_proxy)
//   - hydrated Proxy without durable ID -> (nil, unknown): the transport did
//     use that proxy, but nothing durable identifies it
//   - otherwise                       -> (Proxy.ID, Proxy.Name or "proxy")
//
// Invariant: proxy_id == null implies proxy_name is one of the two sentinels.
func opsUpstreamProxyAttribution(account *Account) (*int64, string) {
	if account == nil {
		return nil, opsProxyNameUnknown
	}
	if account.ProxyID == nil || account.Proxy == nil {
		return nil, opsProxyNameDirect
	}
	if account.Proxy.ID <= 0 {
		return nil, opsProxyNameUnknown
	}
	proxyID := account.Proxy.ID
	name := strings.TrimSpace(account.Proxy.Name)
	if name == "" {
		name = opsProxyNameUnnamed
	}
	return &proxyID, name
}

func opsUpstreamProxyID(account *Account) *int64 {
	proxyID, _ := opsUpstreamProxyAttribution(account)
	return proxyID
}

func opsUpstreamProxyName(account *Account) string {
	_, name := opsUpstreamProxyAttribution(account)
	return name
}

// opsUpstreamWSProxyAttribution is the OpenAI WebSocket variant. The WS dialer
// sets an explicit proxy client only when the account has a usable managed
// proxy; otherwise coder/websocket falls back to http.DefaultClient, which
// honors HTTP_PROXY/HTTPS_PROXY/NO_PROXY. A missing managed proxy is therefore
// unknown, not direct.
func opsUpstreamWSProxyAttribution(account *Account) (*int64, string) {
	proxyID, name := opsUpstreamProxyAttribution(account)
	if proxyID == nil {
		return nil, opsProxyNameUnknown
	}
	return proxyID, name
}

func setUnknownOpsUpstreamProxy(ev *OpsUpstreamErrorEvent) {
	if ev == nil {
		return
	}
	ev.ProxyID = nil
	ev.ProxyName = opsProxyNameUnknown
}

// normalizeOpsUpstreamProxyAttribution makes legacy events explicit without
// pretending that the account's current proxy is historical evidence.
func normalizeOpsUpstreamProxyAttribution(ev *OpsUpstreamErrorEvent) {
	if ev == nil {
		return
	}
	if ev.ProxyID != nil && *ev.ProxyID <= 0 {
		// A non-positive ID never identifies a managed proxy.
		ev.ProxyID = nil
	}
	ev.ProxyName = strings.TrimSpace(ev.ProxyName)
	if ev.ProxyID != nil {
		if ev.ProxyName == "" {
			ev.ProxyName = opsProxyNameUnnamed
		}
		return
	}
	// Invariant: proxy_id == null implies a sentinel name. Any other name
	// without a durable ID is not historical evidence of a managed route.
	if ev.ProxyName != opsProxyNameDirect {
		setUnknownOpsUpstreamProxy(ev)
	}
}

// checkSkipMonitoringForUpstreamEvent snapshots whether this attempt matches a
// skip_monitoring passthrough rule. The final failure decides whether the
// request error is hidden; an intermediate recovered attempt cannot suppress a
// later client-visible failure.
func checkSkipMonitoringForUpstreamEvent(c *gin.Context, ev *OpsUpstreamErrorEvent) {
	if ev.UpstreamStatusCode == 0 {
		return
	}

	svc := getBoundErrorPassthroughService(c)
	if svc == nil {
		return
	}

	// Use the best available body representation for keyword matching.
	// Even when body is empty, MatchRule can still match rules that only
	// specify ErrorCodes (no Keywords), so we always call it.
	body := ev.Detail
	if body == "" {
		body = ev.Message
	}

	rule := svc.MatchRule(ev.Platform, ev.UpstreamStatusCode, []byte(body))
	if rule != nil && rule.SkipMonitoring {
		ev.SkipMonitoring = true
	}
}

func marshalOpsUpstreamErrors(events []*OpsUpstreamErrorEvent) *string {
	if len(events) == 0 {
		return nil
	}
	// Ensure we always store a valid JSON value.
	raw, err := json.Marshal(events)
	if err != nil || len(raw) == 0 {
		return nil
	}
	s := string(raw)
	return &s
}

func ParseOpsUpstreamErrors(raw string) ([]*OpsUpstreamErrorEvent, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []*OpsUpstreamErrorEvent{}, nil
	}
	var out []*OpsUpstreamErrorEvent
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	for i, ev := range out {
		normalizeOpsUpstreamProxyAttribution(ev)
		if ev != nil {
			if encoded := gjson.Get(raw, strconv.Itoa(i)+".routing_diagnostics"); encoded.Exists() {
				d, err := DecodeRoutingDiagnosticsJSON([]byte(encoded.Raw))
				if err != nil {
					log.Print("[Ops] 已拒绝无效的历史事件路由诊断")
				}
				ev.RoutingDiagnostics = d
			}
		}
	}
	return out, nil
}

// normalizeOpsUpstreamErrorsJSON materializes missing proxy attribution on
// stored JSON for detail reads. It edits only the attribution keys of events
// that lack them, so keys written by older struct versions and the original
// key order survive; nothing is re-marshaled through the current struct.
func normalizeOpsUpstreamErrorsJSON(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw, nil
	}
	if !gjson.Valid(raw) {
		return "", errors.New("upstream_errors is not valid JSON")
	}
	parsed := gjson.Parse(raw)
	if !parsed.IsArray() {
		return "", errors.New("upstream_errors is not a JSON array")
	}
	out := raw
	for i, ev := range parsed.Array() {
		if !ev.IsObject() {
			continue
		}
		prefix := strconv.Itoa(i) + "."
		if encoded := ev.Get("routing_diagnostics"); encoded.Exists() {
			d, decodeErr := DecodeRoutingDiagnosticsJSON([]byte(encoded.Raw))
			if decodeErr != nil {
				log.Print("[Ops] 已拒绝无效的历史事件路由诊断")
				var err error
				out, err = sjson.Delete(out, prefix+"routing_diagnostics")
				if err != nil {
					return "", err
				}
			} else if d != nil {
				encoded, _ := json.Marshal(d)
				var err error
				out, err = sjson.SetRaw(out, prefix+"routing_diagnostics", string(encoded))
				if err != nil {
					return "", err
				}
			}
		}
		proxyID := ev.Get("proxy_id")
		proxyName := strings.TrimSpace(ev.Get("proxy_name").String())
		hasValidID := proxyID.Exists() && proxyID.Type == gjson.Number && proxyID.Int() > 0
		var err error
		switch {
		case hasValidID:
			if proxyName == "" {
				out, err = sjson.Set(out, prefix+"proxy_name", opsProxyNameUnnamed)
			}
		case proxyName == opsProxyNameDirect:
			if !proxyID.Exists() || proxyID.Type != gjson.Null {
				out, err = sjson.Set(out, prefix+"proxy_id", nil)
			}
		default:
			if out, err = sjson.Set(out, prefix+"proxy_id", nil); err == nil {
				out, err = sjson.Set(out, prefix+"proxy_name", opsProxyNameUnknown)
			}
		}
		if err != nil {
			return "", err
		}
	}
	return out, nil
}

// OpsUpstreamErrorsWithoutRoutingDiagnostics 保留旧事件字段，列表不扩展内部诊断可见范围。
func OpsUpstreamErrorsWithoutRoutingDiagnostics(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return raw, nil
	}
	if !gjson.Valid(raw) || !gjson.Parse(raw).IsArray() {
		return "", errors.New("无法裁剪上游事件的路由诊断")
	}
	for i, event := range gjson.Parse(raw).Array() {
		if event.Get("routing_diagnostics").Exists() {
			var err error
			raw, err = sjson.Delete(raw, strconv.Itoa(i)+".routing_diagnostics")
			if err != nil {
				return "", errors.New("无法裁剪上游事件的路由诊断")
			}
		}
	}
	return raw, nil
}

// safeUpstreamURL returns scheme + host + path from a URL, stripping query/fragment
// to avoid leaking sensitive query parameters (e.g. OAuth tokens).
func safeUpstreamURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if idx := strings.IndexByte(rawURL, '?'); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	if idx := strings.IndexByte(rawURL, '#'); idx >= 0 {
		rawURL = rawURL[:idx]
	}
	return rawURL
}
