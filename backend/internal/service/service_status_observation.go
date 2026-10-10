package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ServiceStatusObservation 只用于内部源日志，不进入用量、错误 DTO 或导出。
// 可空字段始终编码为 JSON null，使首版合同的字段集合稳定。
type ServiceStatusObservation struct {
	SchemaVersion  int       `json:"schema_version"`
	ObservationKey string    `json:"observation_key"`
	LogicalTurn    *int      `json:"logical_turn"`
	EventRole      string    `json:"event_role"`
	TerminalKind   *string   `json:"terminal_kind"`
	ObservedAt     time.Time `json:"observed_at"`
	ReasonCode     *string   `json:"reason_code"`
}

func (o *ServiceStatusObservation) Clone() *ServiceStatusObservation {
	if o == nil {
		return nil
	}
	out := *o
	if o.LogicalTurn != nil {
		v := *o.LogicalTurn
		out.LogicalTurn = &v
	}
	if o.TerminalKind != nil {
		v := *o.TerminalKind
		out.TerminalKind = &v
	}
	if o.ReasonCode != nil {
		v := *o.ReasonCode
		out.ReasonCode = &v
	}
	return &out
}

func serviceStatusReasonValid(reason string) bool {
	switch reason {
	case "client_cancelled", "user_invalid_request", "user_context_limit", "user_model_unsupported", "user_group_access", "user_authentication", "user_quota", "user_rate_limit", "content_policy",
		"provider_authentication", "provider_quota", "provider_capacity", "provider_5xx", "account_pool_unavailable", "service_timeout", "service_transport", "service_internal",
		"unclassified", "terminal_missing", "linkage_unavailable", "terminal_conflict", "scope_unavailable", "unsupported_entry":
		return true
	}
	return false
}

func (o *ServiceStatusObservation) Validate() error {
	if o == nil {
		return nil
	}
	if o.SchemaVersion != 1 || o.ObservedAt.IsZero() {
		return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
	}
	if key, err := uuid.Parse(o.ObservationKey); err != nil || key == uuid.Nil || key.String() != o.ObservationKey {
		return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
	}
	if o.LogicalTurn != nil && *o.LogicalTurn <= 0 {
		return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
	}
	if o.ReasonCode != nil && !serviceStatusReasonValid(*o.ReasonCode) {
		return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
	}
	switch o.EventRole {
	case "attempt":
		if o.TerminalKind != nil {
			return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
		}
	case "terminal":
		if o.TerminalKind == nil {
			return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
		}
		switch *o.TerminalKind {
		case CompletionStatusCompleted, CompletionStatusClientDisconnected, CompletionStatusUpstreamError, CompletionStatusInterrupted, "user_rejected", CompletionStatusUnknown:
		default:
			return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
		}
	default:
		return errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
	}
	return nil
}

// MarshalServiceStatusObservation 在持久化边界拒绝非法元数据，但不阻断原计费。
func MarshalServiceStatusObservation(o *ServiceStatusObservation) *string {
	if o == nil {
		return nil
	}
	o = o.Clone()
	if err := o.Validate(); err != nil {
		MarkServiceStatusSourceError(time.Now())
		return nil
	}
	raw, err := json.Marshal(o)
	if err != nil {
		MarkServiceStatusSourceError(time.Now())
		return nil
	}
	value := string(raw)
	return &value
}

func DecodeServiceStatusObservation(raw []byte) (*ServiceStatusObservation, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}
	var o ServiceStatusObservation
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, errors.New("SERVICE_STATUS_OBSERVATION_INVALID")
	}
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return o.Clone(), nil
}

type serviceStatusRequestKey struct{}
type serviceStatusRequest struct {
	mu       sync.Mutex
	key      string
	turn     int
	attempt  uint64
	pending  *ServiceStatusObservation
	terminal *ServiceStatusObservation
}

// WithServiceStatusRequest 仅接受服务端调用；客户端 header/body 不参与键生成。
func WithServiceStatusRequest(ctx context.Context, turn int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, serviceStatusRequestKey{}, &serviceStatusRequest{key: uuid.NewString(), turn: turn})
}
func EnsureServiceStatusRequest(ctx context.Context) context.Context {
	if serviceStatusOwner(ctx) != nil {
		return ctx
	}
	return WithServiceStatusRequest(ctx, 0)
}
func EnsureServiceStatusTurn(ctx context.Context, turn int) context.Context {
	if turn <= 0 {
		return ctx
	}
	if owner := serviceStatusOwner(ctx); owner != nil && owner.turn == turn {
		return ctx
	}
	return WithServiceStatusRequest(ctx, turn)
}
func WithServiceStatusOwner(ctx, ownerCtx context.Context) context.Context {
	if owner := serviceStatusOwner(ownerCtx); owner != nil {
		return context.WithValue(ctx, serviceStatusRequestKey{}, owner)
	}
	return ctx
}
func serviceStatusOwner(ctx context.Context) *serviceStatusRequest {
	if ctx == nil {
		return nil
	}
	owner, _ := ctx.Value(serviceStatusRequestKey{}).(*serviceStatusRequest)
	return owner
}
func (s *serviceStatusRequest) observation(role, kind, reason string, at time.Time) *ServiceStatusObservation {
	o := &ServiceStatusObservation{SchemaVersion: 1, ObservationKey: s.key, EventRole: role, ObservedAt: at.UTC()}
	if s.turn > 0 {
		turn := s.turn
		o.LogicalTurn = &turn
	}
	if kind != "" {
		o.TerminalKind = &kind
	}
	// 可信 S1 的完成/取消是已观察的首个终态，后续错误 owner 不能重排。
	frozenS1 := o.TerminalKind != nil && (*o.TerminalKind == CompletionStatusCompleted || *o.TerminalKind == CompletionStatusClientDisconnected)
	if reason != "" && serviceStatusReasonValid(reason) && !frozenS1 {
		o.ReasonCode = &reason
	}
	return o
}
func (s *serviceStatusRequest) beginAttempt() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempt++
	s.pending = nil
	return s.attempt
}
func (s *serviceStatusRequest) observeCandidate(o *ServiceStatusObservation) {
	s.observeAttemptCandidate(o, 0)
}

func (s *serviceStatusRequest) observeAttemptCandidate(o *ServiceStatusObservation, attempt uint64) {
	if s == nil || o == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if attempt != 0 && attempt != s.attempt {
		return
	}
	if s.terminal == nil && (s.pending == nil || s.pending.TerminalKind == nil || *s.pending.TerminalKind == CompletionStatusUnknown) {
		s.pending = o.Clone()
	}
}

func ServiceStatusAttemptObservation(ctx context.Context, reason string, at time.Time) *ServiceStatusObservation {
	owner := serviceStatusOwner(ctx)
	if owner == nil {
		return nil
	}
	return owner.observation("attempt", "", reason, at)
}

// SetServiceStatusAttemptFailure 保存当前发送的事实；仅最终 owner 能冻结为 terminal。
func SetServiceStatusAttemptFailure(ctx context.Context, kind, reason string, at time.Time) {
	owner := serviceStatusOwner(ctx)
	if owner == nil {
		return
	}
	owner.observeCandidate(owner.observation("terminal", kind, reason, at))
}

// serviceStatusAttemptReason 只复用当前发送 owner 已确认的原因；通用 Ops 状态码可能是合成值。
func serviceStatusAttemptReason(ctx context.Context) string {
	owner := serviceStatusOwner(ctx)
	if owner == nil {
		return "unclassified"
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.pending != nil && owner.pending.ReasonCode != nil {
		return *owner.pending.ReasonCode
	}
	return "unclassified"
}

func beginServiceStatusAttempt(ctx context.Context) {
	if owner := serviceStatusOwner(ctx); owner != nil {
		owner.beginAttempt()
	}
}

func observeServiceStatusTransportError(ctx context.Context, err error) {
	kind, reason := CompletionStatusUpstreamError, "service_transport"
	if errors.Is(err, context.Canceled) || (ctx != nil && errors.Is(ctx.Err(), context.Canceled)) {
		kind, reason = CompletionStatusClientDisconnected, "client_cancelled"
	} else if errors.Is(err, context.DeadlineExceeded) || (ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)) {
		reason = "service_timeout"
	}
	SetServiceStatusAttemptFailure(ctx, kind, reason, time.Now())
}

// ServiceStatusFinalObservation 由请求/可见失败 owner 调用。重试中的错误不调用它。
func ServiceStatusFinalObservation(ctx context.Context, reason string) *ServiceStatusObservation {
	owner := serviceStatusOwner(ctx)
	if owner == nil {
		return nil
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.terminal != nil {
		return owner.terminal.Clone()
	}
	o := owner.pending.Clone()
	if o == nil {
		o = owner.observation("terminal", CompletionStatusUnknown, "terminal_missing", time.Now())
	}
	// 可信 S1 的完成/取消是已观察的首个终态，后续错误 owner 不能重排。
	frozenS1 := o.TerminalKind != nil && (*o.TerminalKind == CompletionStatusCompleted || *o.TerminalKind == CompletionStatusClientDisconnected)
	if reason != "" && serviceStatusReasonValid(reason) && !frozenS1 {
		o.ReasonCode = &reason
		switch {
		case reason == "client_cancelled":
			kind := CompletionStatusClientDisconnected
			o.TerminalKind = &kind
		case strings.HasPrefix(reason, "user_") || reason == "content_policy":
			kind := "user_rejected"
			o.TerminalKind = &kind
			o.ObservedAt = time.Now().UTC()
		case o.TerminalKind != nil && *o.TerminalKind == CompletionStatusUnknown && (strings.HasPrefix(reason, "provider_") || strings.HasPrefix(reason, "service_") || reason == "account_pool_unavailable"):
			kind := CompletionStatusUpstreamError
			o.TerminalKind = &kind
		}
	}
	owner.terminal = o.Clone()
	return o
}

// CommitServiceStatusObservation 在异步提交之前发布已有 S1 终态，后续关闭不改写它。
func CommitServiceStatusObservation(ctx context.Context, o *ServiceStatusObservation) {
	owner := serviceStatusOwner(ctx)
	if owner == nil || o == nil || o.EventRole != "terminal" || o.ObservationKey != owner.key {
		return
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.terminal == nil {
		owner.terminal = o.Clone()
	}
}

// ServiceStatusProviderReason 必须在确认直接上游身份的 owner 处调用。
// 状态码只有与该来源事实合用才有意义；403 不猜内容策略或用户权限。
func ServiceStatusProviderReason(status int, code string) string {
	switch code {
	case "insufficient_quota", "billing_hard_limit_reached", "usage_limit_reached":
		return "provider_quota"
	case "invalid_api_key", "authentication_error":
		return "provider_authentication"
	case "rate_limit_exceeded", "server_overloaded", "overloaded_error", "server_is_overloaded", "slow_down":
		return "provider_capacity"
	case "context_length_exceeded":
		return "user_context_limit"
	case "content_policy_violation", "content_filter":
		return "content_policy"
	}
	switch {
	case status == 401:
		return "provider_authentication"
	case status == 402:
		return "provider_quota"
	case status == 429:
		return "provider_capacity"
	case status >= 500 && status <= 599:
		return "provider_5xx"
	default:
		return "unclassified"
	}
}

// MarkServiceStatusUserRejection 供明确的本部署拒绝 owner 使用，不解析错误正文。
func MarkServiceStatusUserRejection(c *gin.Context, reason string) {
	if c == nil || c.Request == nil || !serviceStatusReasonValid(reason) {
		return
	}
	ServiceStatusFinalObservation(c.Request.Context(), reason)
}

// ServiceStatusLocalErrorReason 只供本部署明确产生的稳定错误码使用。
func ServiceStatusLocalErrorReason(code string) string {
	switch code {
	case "INVALID_API_KEY", "API_KEY_REQUIRED", "API_KEY_EXPIRED", "API_KEY_DISABLED", "USER_NOT_FOUND", "USER_INACTIVE", "api_key_in_query_deprecated":
		return "user_authentication"
	case "GROUP_DELETED", "GROUP_DISABLED", "GROUP_NOT_ALLOWED", "ACCESS_DENIED":
		return "user_group_access"
	case "INSUFFICIENT_BALANCE", "USAGE_LIMIT_EXCEEDED", "SUBSCRIPTION_NOT_FOUND", "SUBSCRIPTION_INVALID", "API_KEY_QUOTA_EXHAUSTED", "insufficient_quota":
		return "user_quota"
	case "INVALID_AUTH_RATE_LIMITED", "user_concurrency_limit_exceeded", "user_wait_queue_full":
		return "user_rate_limit"
	}
	return ""
}
