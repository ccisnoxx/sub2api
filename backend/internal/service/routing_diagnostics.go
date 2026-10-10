package service

import (
	"context"
	"errors"
	"sync"
)

// RoutingDiagnostics 是一次真实选择评估的内部快照。未知值保留 nil；不得从旧指标回填。
type RoutingDiagnostics struct {
	SchemaVersion      int            `json:"schema_version"`
	Turn               *int           `json:"turn"`
	SelectionAttempt   int            `json:"selection_attempt"`
	SelectionLayer     *string        `json:"selection_layer"`
	SelectionReason    *string        `json:"selection_reason"`
	CandidatePool      *int           `json:"candidate_pool"`
	FilteredCandidates *int           `json:"filtered_candidates"`
	FilterReasons      map[string]int `json:"filter_reasons"`
	FilterCoverage     string         `json:"filter_coverage"`
}

func routingValue[T any](value T) *T { return &value }

// Clone 复制所有可变引用，后续选择与异步记录不能修改已有快照。
func (d *RoutingDiagnostics) Clone() *RoutingDiagnostics {
	if d == nil {
		return nil
	}
	copy := *d
	if d.Turn != nil {
		copy.Turn = routingValue(*d.Turn)
	}
	if d.SelectionLayer != nil {
		copy.SelectionLayer = routingValue(*d.SelectionLayer)
	}
	if d.SelectionReason != nil {
		copy.SelectionReason = routingValue(*d.SelectionReason)
	}
	if d.CandidatePool != nil {
		copy.CandidatePool = routingValue(*d.CandidatePool)
	}
	if d.FilteredCandidates != nil {
		copy.FilteredCandidates = routingValue(*d.FilteredCandidates)
	}
	if d.FilterReasons != nil {
		copy.FilterReasons = make(map[string]int, len(d.FilterReasons))
		for reason, count := range d.FilterReasons {
			copy.FilterReasons[reason] = count
		}
	}
	return &copy
}

type routingDiagnosticsRequestKey struct{}
type routingDiagnosticsSelectionKey struct{}
type routingDiagnosticsPoolObservationKey struct{}

type routingDiagnosticsRequest struct {
	mu      sync.Mutex
	turn    *int
	attempt int
	current *RoutingDiagnostics
}

// WithRoutingDiagnosticsRequest 创建全新请求或 WS turn；HTTP 与建连前评估传 0。
func WithRoutingDiagnosticsRequest(ctx context.Context, turn int) context.Context {
	owner := &routingDiagnosticsRequest{}
	if turn > 0 {
		owner.turn = routingValue(turn)
	}
	ctx = context.WithValue(ctx, routingDiagnosticsRequestKey{}, owner)
	return context.WithValue(ctx, routingDiagnosticsSelectionKey{}, (*routingDiagnosticsBuilder)(nil))
}

// EnsureRoutingDiagnosticsRequest 只补齐缺失的归属，不重置已有 turn 或序号。
func EnsureRoutingDiagnosticsRequest(ctx context.Context) context.Context {
	if owner, _ := ctx.Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest); owner != nil {
		return ctx
	}
	return WithRoutingDiagnosticsRequest(ctx, 0)
}

// EnsureRoutingDiagnosticsTurn 只在进入新的连接逻辑 turn 时重置归属。
func EnsureRoutingDiagnosticsTurn(ctx context.Context, turn int) context.Context {
	owner, _ := ctx.Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest)
	if owner != nil {
		owner.mu.Lock()
		sameTurn := owner.turn != nil && *owner.turn == turn
		owner.mu.Unlock()
		if sameTurn {
			return ctx
		}
	}
	return WithRoutingDiagnosticsRequest(ctx, turn)
}

// WithRoutingDiagnosticsOwner 重绑现有请求归属，保留业务 ctx 的取消、定价与其他值。
// ownerCtx 只提供诊断 owner，不传播它的取消或其他上下文。
func WithRoutingDiagnosticsOwner(ctx, ownerCtx context.Context) context.Context {
	owner, _ := ownerCtx.Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest)
	if owner == nil {
		return ctx
	}
	ctx = context.WithValue(ctx, routingDiagnosticsRequestKey{}, owner)
	return context.WithValue(ctx, routingDiagnosticsSelectionKey{}, (*routingDiagnosticsBuilder)(nil))
}

// GetRoutingDiagnostics 返回当前评估的完成快照；评估开始后、完成前为 nil。
func GetRoutingDiagnostics(ctx context.Context) *RoutingDiagnostics {
	owner, _ := ctx.Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest)
	if owner == nil {
		return nil
	}
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.current.Clone()
}

type routingDiagnosticsError struct {
	cause       error
	diagnostics *RoutingDiagnostics
}

func (e *routingDiagnosticsError) Error() string { return e.cause.Error() }
func (e *routingDiagnosticsError) Unwrap() error { return e.cause }

// RoutingDiagnosticsFromError 通过结构化包装读取，保留原错误链与 sentinel。
func RoutingDiagnosticsFromError(err error) *RoutingDiagnostics {
	var attached *routingDiagnosticsError
	if errors.As(err, &attached) {
		return attached.diagnostics.Clone()
	}
	return nil
}

// builder 只由同步选择栈持有；请求 owner 只发布完成后的深副本。
// settled 表示该行已最终排除或已完成实际兼容复核，排序落后和未 probe 行不算覆盖。
type routingDiagnosticsBuilder struct {
	owner       *routingDiagnosticsRequest
	diagnostics RoutingDiagnostics
	pool        map[int64]struct{}
	rejected    map[int64]string
	rechecks    map[int64]string
	settled     map[int64]bool
	finished    bool
}

func beginRoutingDiagnosticsSelection(ctx context.Context) (context.Context, *routingDiagnosticsBuilder) {
	ctx = EnsureRoutingDiagnosticsRequest(ctx)
	owner, _ := ctx.Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest)
	owner.mu.Lock()
	owner.attempt++
	owner.current = nil
	b := &routingDiagnosticsBuilder{owner: owner, diagnostics: RoutingDiagnostics{
		SchemaVersion: 1, Turn: owner.turn, SelectionAttempt: owner.attempt, FilterCoverage: "unobserved",
	}}
	owner.mu.Unlock()
	return context.WithValue(ctx, routingDiagnosticsSelectionKey{}, b), b
}

func ensureRoutingDiagnosticsSelection(ctx context.Context) (context.Context, *routingDiagnosticsBuilder) {
	if b := routingDiagnosticsBuilderFromContext(ctx); b != nil && !b.finished {
		return ctx, b
	}
	return beginRoutingDiagnosticsSelection(ctx)
}

func routingDiagnosticsBuilderFromContext(ctx context.Context) *routingDiagnosticsBuilder {
	b, _ := ctx.Value(routingDiagnosticsSelectionKey{}).(*routingDiagnosticsBuilder)
	return b
}

func (b *routingDiagnosticsBuilder) outcome(layer, reason string) {
	if b == nil {
		return
	}
	b.diagnostics.SelectionLayer = routingValue(layer)
	b.diagnostics.SelectionReason = nil
	if reason != "" {
		b.diagnostics.SelectionReason = routingValue(reason)
	}
}

func routingAccountIDs(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for i := range accounts {
		ids = append(ids, accounts[i].ID)
	}
	return ids
}

func routingAccountPointerIDs(accounts []*Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		if account != nil {
			ids = append(ids, account.ID)
		}
	}
	return ids
}

func (b *routingDiagnosticsBuilder) observePool(accounts []Account) {
	if b == nil || b.pool != nil {
		return
	}
	b.pool = make(map[int64]struct{}, len(accounts))
	b.rejected = make(map[int64]string)
	b.rechecks = make(map[int64]string)
	b.settled = make(map[int64]bool)
	for _, id := range routingAccountIDs(accounts) {
		b.pool[id] = struct{}{}
	}
	b.diagnostics.CandidatePool = routingValue(len(b.pool))
}

func (b *routingDiagnosticsBuilder) reject(id int64, reason string) {
	if b == nil || b.pool == nil || !routingFilterReasonAllowed(reason) {
		return
	}
	if _, member := b.pool[id]; !member {
		return
	}
	if _, exists := b.rejected[id]; !exists {
		b.rejected[id] = reason
	}
	delete(b.rechecks, id)
	b.settled[id] = true
}

// 可重试复核拒绝先暂存；同轮后续复核恢复时 pass 撤销，避免 compact stale 误计。
func (b *routingDiagnosticsBuilder) recheckRejected(id int64, reason string) {
	if b == nil || b.pool == nil || !routingFilterReasonAllowed(reason) {
		return
	}
	if _, member := b.pool[id]; !member {
		return
	}
	if _, exists := b.rechecks[id]; !exists {
		b.rechecks[id] = reason
	}
	b.settled[id] = true
}

func (b *routingDiagnosticsBuilder) recheckStart(id int64) {
	if b != nil && b.pool != nil {
		delete(b.rechecks, id)
		if _, rejected := b.rejected[id]; !rejected {
			delete(b.settled, id)
		}
	}
}

func (b *routingDiagnosticsBuilder) pass(id int64) {
	if b == nil || b.pool == nil {
		return
	}
	if _, member := b.pool[id]; member {
		delete(b.rechecks, id)
		delete(b.rejected, id)
		b.settled[id] = true
	}
}

func (b *routingDiagnosticsBuilder) removed(before, after []int64, reason string) {
	if b == nil {
		return
	}
	remaining := make(map[int64]struct{}, len(after))
	for _, id := range after {
		remaining[id] = struct{}{}
	}
	for _, id := range before {
		if _, kept := remaining[id]; !kept {
			b.reject(id, reason)
		}
	}
}

func routingFilterReasonAllowed(reason string) bool {
	switch reason {
	case "excluded", "not_schedulable", "platform_mismatch", "account_model_not_owned", "model_not_supported", "channel_upstream_restricted",
		"runtime_blocked", "privacy_not_set", "proxy_stream_quarantined", "shadow_parent_unhealthy", "model_rate_limited", "turn_state_hold",
		"quota_auto_pause", "quota_auto_pause_5h", "quota_auto_pause_7d", "quota_auto_pause_retry_after", "quota_auto_pause_requests", "quota_auto_pause_tokens",
		"quota_auto_reset_pending_5h", "quota_auto_reset_pending_7d", "quota_auto_reset_credit_check_5h", "quota_auto_reset_credit_check_7d",
		"profit_threshold", "profit_invalid_account_rate", "capability_mismatch", "transport_incompatible", "compact_unsupported", "scheduling_threshold",
		"grok_free_quota_soft_gate", "grok_team_model_rate_limit", "grok_model_quota_block", "gateway_pool_duplicate", "session_limit", "capacity_limited", "recheck_rejected":
		return true
	}
	return false
}

func (b *routingDiagnosticsBuilder) snapshot(selection *AccountSelectionResult, layer string, err error) *RoutingDiagnostics {
	d := b.diagnostics.Clone()
	if b.pool != nil {
		reasons := make(map[string]int)
		count := 0
		for id := range b.pool {
			reason := b.rejected[id]
			if reason == "" {
				reason = b.rechecks[id]
			}
			if reason != "" {
				count++
				reasons[reason]++
			}
		}
		d.FilteredCandidates, d.FilterReasons, d.FilterCoverage = routingValue(count), reasons, "partial"
		if len(b.settled) == len(b.pool) {
			d.FilterCoverage = "complete"
		}
	}
	if d.SelectionLayer == nil {
		if layer != "" {
			d.SelectionLayer = routingValue(layer)
		}
	}
	if d.SelectionReason == nil {
		reason := "selection_failed"
		if err == nil && selection != nil {
			if selection.WaitPlan != nil && !selection.Acquired {
				reason = "wait_plan"
			} else if selection.Acquired {
				reason = "slot_acquired"
				if d.SelectionLayer != nil && *d.SelectionLayer != "load_balance" {
					reason = "sticky_hit"
				}
			}
		} else if errors.Is(err, ErrNoAvailableCompactAccounts) {
			reason = "compact_unsupported"
		} else if errors.Is(err, ErrNoAvailableAccounts) {
			reason = "selection_exhausted"
			if d.CandidatePool != nil && *d.CandidatePool == 0 {
				reason = "pool_empty"
			} else if d.FilterCoverage == "complete" && d.CandidatePool != nil && *d.FilteredCandidates == *d.CandidatePool {
				reason = "candidates_filtered"
			}
		}
		d.SelectionReason = routingValue(reason)
	}
	return d
}

func (b *routingDiagnosticsBuilder) publish(d *RoutingDiagnostics) {
	b.owner.mu.Lock()
	if b.owner.attempt == d.SelectionAttempt {
		b.owner.current = d.Clone()
	}
	b.owner.mu.Unlock()
}

func finishRoutingDiagnosticsSelection(b *routingDiagnosticsBuilder, selection *AccountSelectionResult, layer string, err error) (*AccountSelectionResult, *RoutingDiagnostics, error) {
	if b == nil {
		return selection, nil, err
	}
	// 内层 owner 已完成实际评估时，外层保留该结果所属快照，不能借第一轮 builder。
	d := RoutingDiagnosticsFromError(err)
	if d == nil && selection != nil {
		d = selection.RoutingDiagnostics.Clone()
	}
	if d == nil {
		d = b.snapshot(selection, layer, err)
		b.finished = true
		b.publish(d)
	}
	if selection != nil {
		selection.RoutingDiagnostics = d.Clone()
	}
	if err != nil && RoutingDiagnosticsFromError(err) == nil {
		err = &routingDiagnosticsError{cause: err, diagnostics: d.Clone()}
	}
	return selection, d, err
}

// 外层准入终检输出同次评估的新副本；已返回的选择与错误副本不被修改。
func rejectRoutingDiagnosticsSelection(ctx context.Context, selection *AccountSelectionResult, previous *RoutingDiagnostics, reason string, err error) (*RoutingDiagnostics, error) {
	d := previous.Clone()
	if selection != nil && selection.RoutingDiagnostics != nil {
		d = selection.RoutingDiagnostics.Clone()
	}
	if d == nil {
		_, b := beginRoutingDiagnosticsSelection(ctx)
		d = b.snapshot(nil, "gateway_pool", err)
	}
	d.SelectionLayer, d.SelectionReason = routingValue("gateway_pool"), routingValue(reason)
	// 布尔终检不提供稳定过滤子码，保留已知下界但不声称完整排除覆盖。
	if selection != nil && selection.Account != nil && d.FilteredCandidates != nil {
		d.FilterCoverage = "partial"
	}
	if owner, _ := ctx.Value(routingDiagnosticsRequestKey{}).(*routingDiagnosticsRequest); owner != nil {
		owner.mu.Lock()
		if owner.attempt == d.SelectionAttempt {
			owner.current = d.Clone()
		}
		owner.mu.Unlock()
	}
	return d, &routingDiagnosticsError{cause: err, diagnostics: d.Clone()}
}
