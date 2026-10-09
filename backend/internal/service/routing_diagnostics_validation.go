package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
)

const maxRoutingDiagnosticsInteger = 1<<53 - 1

// Validate 只接受冻结的 v1 白名单和完整关系；异常不包含原始值。
func (d *RoutingDiagnostics) Validate() error {
	if d == nil {
		return nil
	}
	invalid := errors.New("无效的路由诊断快照")
	validCount := func(n *int) bool { return n == nil || (*n >= 0 && *n <= maxRoutingDiagnosticsInteger) }
	if d.SchemaVersion != 1 || d.SelectionAttempt <= 0 || d.SelectionAttempt > maxRoutingDiagnosticsInteger ||
		(d.Turn != nil && (*d.Turn <= 0 || *d.Turn > maxRoutingDiagnosticsInteger)) ||
		!validCount(d.CandidatePool) || !validCount(d.FilteredCandidates) {
		return invalid
	}
	layer := ""
	if d.SelectionLayer != nil {
		layer = *d.SelectionLayer
		switch layer {
		case "channel_pricing", "previous_response_id", "guardian_parent", "session_hash", "load_balance", "gateway_pool":
		default:
			return invalid
		}
	}
	if d.SelectionReason != nil {
		reason := *d.SelectionReason
		allowed := reason == "selection_failed"
		switch layer {
		case "channel_pricing":
			allowed = reason == "channel_pricing_restricted"
		case "previous_response_id", "guardian_parent", "session_hash":
			allowed = allowed || reason == "sticky_hit" || reason == "wait_plan"
		case "load_balance":
			switch reason {
			case "slot_acquired", "wait_plan", "pool_empty", "candidates_filtered", "compact_unsupported", "selection_exhausted", "account_list_failed":
				allowed = true
			}
		case "gateway_pool":
			allowed = allowed || reason == "gateway_pool_restricted"
		}
		if !allowed {
			return invalid
		}
		if reason == "pool_empty" && (d.CandidatePool == nil || *d.CandidatePool != 0 || d.FilterCoverage != "complete") {
			return invalid
		}
		if reason == "candidates_filtered" && (d.CandidatePool == nil || *d.CandidatePool == 0 || d.FilteredCandidates == nil ||
			*d.FilteredCandidates != *d.CandidatePool || d.FilterCoverage != "complete") {
			return invalid
		}
	}
	switch d.FilterCoverage {
	case "unobserved":
		if d.FilteredCandidates != nil || d.FilterReasons != nil {
			return invalid
		}
	case "complete", "partial":
		if d.FilteredCandidates == nil || d.FilterReasons == nil || (d.FilterCoverage == "complete" && d.CandidatePool == nil) {
			return invalid
		}
	default:
		return invalid
	}
	sum := 0
	for reason, count := range d.FilterReasons {
		if !routingFilterReasonAllowed(reason) || count <= 0 || count > maxRoutingDiagnosticsInteger-sum {
			return invalid
		}
		sum += count
	}
	if d.FilteredCandidates != nil && (sum != *d.FilteredCandidates || (d.CandidatePool != nil && *d.FilteredCandidates > *d.CandidatePool)) {
		return invalid
	}
	return nil
}

// DecodeRoutingDiagnosticsJSON 保持整体 null 与字段级未知；拒绝漏字段和额外字段。
func DecodeRoutingDiagnosticsJSON(raw []byte) (*RoutingDiagnostics, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	invalid := errors.New("无效的路由诊断 JSON")
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != 9 {
		return nil, invalid
	}
	for _, key := range []string{"schema_version", "turn", "selection_attempt", "selection_layer", "selection_reason", "candidate_pool", "filtered_candidates", "filter_reasons", "filter_coverage"} {
		if _, ok := fields[key]; !ok {
			return nil, invalid
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var d RoutingDiagnostics
	if decoder.Decode(&d) != nil || decoder.Decode(new(any)) != io.EOF || d.Validate() != nil {
		return nil, invalid
	}
	return &d, nil
}

// 无效诊断只丢弃对象，原故障必须继续记录。日志信号不含诊断原值。
func sanitizedRoutingDiagnostics(d *RoutingDiagnostics) *RoutingDiagnostics {
	if d.Validate() != nil {
		log.Print("[Ops] 已拒绝无效的路由诊断快照")
		return nil
	}
	return d.Clone()
}

func sanitizeOpsRoutingDiagnostics(entry *OpsInsertErrorLogInput) {
	d := entry.RoutingDiagnostics
	if d == nil && entry.RoutingDiagnosticsJSON != nil {
		var err error
		d, err = DecodeRoutingDiagnosticsJSON([]byte(*entry.RoutingDiagnosticsJSON))
		if err != nil {
			log.Print("[Ops] 已拒绝无效的路由诊断 JSON")
		}
	}
	d = sanitizedRoutingDiagnostics(d)
	entry.RoutingDiagnostics = nil
	entry.RoutingDiagnosticsJSON = nil
	if d != nil {
		// 合法的有限整数/字符串对象可序列化；队列只保留自有 JSON，避免双份内存。
		raw, _ := json.Marshal(d)
		value := string(raw)
		entry.RoutingDiagnosticsJSON = &value
	}
}
