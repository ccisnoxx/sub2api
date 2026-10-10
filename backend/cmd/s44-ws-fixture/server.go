package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

var safeLabel = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)
var markerPattern = regexp.MustCompile(`^s44:([A-Za-z0-9_.-]{1,80}):([a-z0-9_]{1,40}):([1-9][0-9]{0,5})$`)

type marker struct {
	Run   string `json:"run"`
	Case  string `json:"case"`
	Index string `json:"index"`
}

func (m marker) key() string        { return "s44:" + m.Run + ":" + m.Case + ":" + m.Index }
func (m marker) responseID() string { return "resp_s44_" + m.Run + "_" + m.Case + "_" + m.Index }

type fixtureEvent struct {
	Sequence  int64  `json:"sequence"`
	At        string `json:"at"`
	Kind      string `json:"kind"`
	Transport string `json:"transport,omitempty"`
	Session   int64  `json:"session,omitempty"`
	LocalTurn int    `json:"local_turn,omitempty"`
	Attempt   int    `json:"attempt,omitempty"`
	Run       string `json:"run,omitempty"`
	Case      string `json:"case,omitempty"`
	Index     string `json:"index,omitempty"`
	Model     string `json:"upstream_model,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Status    int    `json:"status,omitempty"`
	Written   *bool  `json:"written,omitempty"`
}

type plan struct {
	Run      string `json:"run"`
	Case     string `json:"case"`
	Behavior string `json:"behavior"`
}

type gate struct {
	release chan struct{}
	done    bool
}

// attempts、控制计划、取消 gate 和证据只由此 owner 维护。
// 重连会生成新的 session/local_turn；marker 的 attempt 跨连接保持稳定。
type fixture struct {
	mu       sync.Mutex
	attempts map[string]int
	plans    map[string]string
	gates    map[string]*gate
	events   []fixtureEvent
	overflow bool
	session  int64
	limit    int
}

func newFixture() *fixture {
	return &fixture{attempts: map[string]int{}, plans: map[string]string{}, gates: map[string]*gate{}, limit: 10000}
}

func (f *fixture) record(e fixtureEvent) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recordLocked(e)
}

func (f *fixture) recordLocked(e fixtureEvent) bool {
	if len(f.events) >= f.limit {
		f.overflow = true
		return false
	}
	e.Sequence = int64(len(f.events) + 1)
	e.At = time.Now().UTC().Format(time.RFC3339Nano)
	f.events = append(f.events, e)
	return true
}

func (f *fixture) next(m marker, model, transport string, session int64, turn int) (fixtureEvent, string, *gate, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// 每次请求为后续写出/关闭预留空间；达到上限显式拒绝，不覆盖旧证据。
	if len(f.events)+8 >= f.limit {
		return fixtureEvent{}, "", nil, false
	}
	f.attempts[m.key()]++
	behavior := m.Case
	if override, ok := f.plans[m.Run+":"+m.Case]; ok {
		behavior = override
	}
	var g *gate
	if behavior == "cancel_drain" {
		g = &gate{release: make(chan struct{})}
		f.gates[m.key()] = g
	}
	e := fixtureEvent{Transport: transport, Session: session, LocalTurn: turn, Attempt: f.attempts[m.key()], Run: m.Run, Case: m.Case, Index: m.Index, Model: model}
	return e, behavior, g, true
}

func validBehavior(b string) bool {
	switch b {
	case "success", "success_zero", "provider_failure", "provider_auth", "provider_quota", "retry_close", "retry_close_failure", "retry_429", "cancel_drain", "missing_terminal":
		return true
	}
	return false
}

func parseRequest(raw []byte, ws bool) (marker, string, bool, error) {
	var request struct {
		Type   string          `json:"type"`
		Model  string          `json:"model"`
		Input  json.RawMessage `json:"input"`
		Stream bool            `json:"stream"`
	}
	if json.Unmarshal(raw, &request) != nil || (ws && request.Type != "response.create") || !safeLabel.MatchString(request.Model) {
		return marker{}, "", false, fail("invalid_fixture_request")
	}
	var input string
	if json.Unmarshal(request.Input, &input) != nil {
		var messages []struct {
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(request.Input, &messages) == nil && len(messages) == 1 {
			if json.Unmarshal(messages[0].Content, &input) != nil {
				var content []struct {
					Text string `json:"text"`
				}
				if json.Unmarshal(messages[0].Content, &content) == nil && len(content) == 1 {
					input = content[0].Text
				}
			}
		}
	}
	parts := markerPattern.FindStringSubmatch(input)
	if len(parts) != 4 || !validBehavior(parts[2]) {
		return marker{}, "", false, fail("fixture_marker_required")
	}
	return marker{parts[1], parts[2], parts[3]}, request.Model, request.Stream, nil
}

func response(m marker, model, behavior string, completed bool) map[string]any {
	input, output := 3, 1
	if behavior == "success_zero" || behavior == "retry_429" {
		input, output = 0, 0
	}
	r := map[string]any{"id": m.responseID(), "object": "response", "created_at": time.Now().Unix(), "model": model, "status": "completed", "output": []any{}, "usage": map[string]any{"input_tokens": input, "output_tokens": output, "total_tokens": input + output, "input_tokens_details": map[string]int{"cached_tokens": 0}}}
	if completed {
		if output > 0 {
			r["output"] = []any{map[string]any{"id": "msg_s44", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]string{"type": "output_text", "text": "s44 synthetic output"}}}}
		}
		return r
	}
	code, status, kind := "server_error", 500, "server_error"
	if behavior == "provider_auth" {
		code, status, kind = "invalid_api_key", 401, "authentication_error"
	} else if behavior == "provider_quota" {
		code, status, kind = "insufficient_quota", 402, "usage_limit_reached"
	}
	r["status"] = "failed"
	r["error"] = map[string]any{"code": code, "type": kind, "status_code": status, "message": "s44 synthetic provider failure"}
	return r
}

func isFakeAuthorization(r *http.Request) bool {
	switch r.Header.Get("Authorization") {
	case "", "Bearer sk-s44-fixture-first", "Bearer sk-s44-fixture-second", "Bearer s44-fixture-oauth":
		return true
	}
	return false
}

func (f *fixture) responses(w http.ResponseWriter, r *http.Request) {
	if !isFakeAuthorization(r) {
		jsonReply(w, 401, map[string]string{"code": "fixture_fake_credentials_required"})
		return
	}
	if r.Method == http.MethodGet && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		f.websocket(w, r)
		return
	}
	if r.Method != http.MethodPost {
		jsonReply(w, 405, map[string]string{"code": "method_not_allowed"})
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		jsonReply(w, 400, map[string]string{"code": "invalid_fixture_request"})
		return
	}
	m, model, stream, err := parseRequest(raw, false)
	if err != nil {
		jsonReply(w, 400, map[string]string{"code": safeErrorCode(err)})
		return
	}
	e, behavior, _, ok := f.next(m, model, "http", 0, 0)
	if !ok {
		jsonReply(w, 507, map[string]string{"code": "evidence_limit"})
		return
	}
	e.Kind = "request_received"
	f.record(e)
	if behavior == "retry_429" && e.Attempt == 1 {
		e.Kind, e.Status = "http_response", 429
		f.record(e)
		jsonReply(w, 429, map[string]any{"error": map[string]string{"code": "rate_limit_exceeded", "type": "rate_limit_error", "message": "s44 synthetic capacity"}})
		return
	}
	completed := behavior != "provider_failure" && behavior != "provider_auth" && behavior != "provider_quota" && behavior != "retry_close_failure"
	terminal := "response.completed"
	if !completed {
		terminal = "response.failed"
	}
	result := response(m, model, behavior, completed)
	w.Header().Set("X-Request-ID", m.responseID())
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		created := map[string]any{"type": "response.created", "response": map[string]any{"id": m.responseID(), "model": model, "status": "in_progress"}}
		_, err = w.Write(append(append([]byte("event: response.created\ndata: "), encodeJSON(created)...), []byte("\n\n")...))
		if err == nil {
			_, err = w.Write(append(append([]byte("event: "+terminal+"\ndata: "), encodeJSON(map[string]any{"type": terminal, "response": result})...), []byte("\n\n")...))
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(encodeJSON(result))
	}
	written := err == nil
	e.Kind, e.EventType, e.Written, e.Status = "http_response", terminal, &written, 200
	f.record(e)
}

func (f *fixture) websocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	f.mu.Lock()
	f.session++
	session := f.session
	f.mu.Unlock()
	for turn := 1; ; turn++ {
		_, raw, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		m, model, _, err := parseRequest(raw, true)
		if err != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "fixture marker required")
			return
		}
		e, behavior, g, ok := f.next(m, model, "ws", session, turn)
		if !ok {
			_ = conn.Close(websocket.StatusInternalError, "evidence limit")
			return
		}
		e.Kind = "request_received"
		f.record(e)
		if (behavior == "retry_close" || behavior == "retry_close_failure") && e.Attempt == 1 {
			e.Kind = "closed_before_output"
			f.record(e)
			return
		}
		if behavior == "retry_429" && e.Attempt == 1 {
			f.writeWS(conn, r.Context(), e, "error", map[string]any{"type": "error", "error": map[string]string{"code": "rate_limit_exceeded", "type": "usage_limit_reached", "message": "The usage limit has been reached"}})
			return
		}
		if !f.writeWS(conn, r.Context(), e, "response.created", map[string]any{"type": "response.created", "response": map[string]any{"id": m.responseID(), "model": model, "status": "in_progress"}}) {
			return
		}
		if behavior == "missing_terminal" {
			return
		}
		if g != nil {
			select {
			case <-g.release:
			case <-time.After(30 * time.Second):
				e.Kind = "gate_timeout"
				f.record(e)
				return
			case <-r.Context().Done():
				return
			}
		}
		completed := behavior != "provider_failure" && behavior != "provider_auth" && behavior != "provider_quota" && behavior != "retry_close_failure"
		terminal := "response.completed"
		if !completed {
			terminal = "response.failed"
		}
		written := f.writeWS(conn, r.Context(), e, terminal, map[string]any{"type": terminal, "response": response(m, model, behavior, completed)})
		if g != nil {
			f.mu.Lock()
			g.done = true
			f.mu.Unlock()
		}
		if !written {
			return
		}
	}
}

func (f *fixture) writeWS(conn *websocket.Conn, ctx context.Context, e fixtureEvent, typ string, payload any) bool {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err := conn.Write(writeCtx, websocket.MessageText, encodeJSON(payload))
	cancel()
	written := err == nil
	e.Kind, e.EventType, e.Written = "frame_written", typ, &written
	f.record(e)
	return written
}

func (f *fixture) control(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/control/evidence" {
		run := r.URL.Query().Get("run")
		if run != "" && !safeLabel.MatchString(run) {
			jsonReply(w, 400, map[string]string{"code": "invalid_run"})
			return
		}
		f.mu.Lock()
		events := make([]fixtureEvent, 0, len(f.events))
		for _, e := range f.events {
			if run == "" || e.Run == run {
				events = append(events, e)
			}
		}
		overflow := f.overflow
		f.mu.Unlock()
		status := 200
		if overflow {
			status = 507
		}
		jsonReply(w, status, map[string]any{"schema_version": 1, "events": events, "evidence_complete": !overflow})
		return
	}
	if r.Method != http.MethodPost {
		jsonReply(w, 405, map[string]string{"code": "method_not_allowed"})
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	switch r.URL.Path {
	case "/control/plan":
		var p plan
		if decoder.Decode(&p) != nil || !safeLabel.MatchString(p.Run) || !validBehavior(p.Case) || !validBehavior(p.Behavior) {
			jsonReply(w, 400, map[string]string{"code": "invalid_plan"})
			return
		}
		f.mu.Lock()
		// 计划冻结到一个 run/case；禁止在已开始请求中改变行为。
		started := false
		for key := range f.attempts {
			if strings.HasPrefix(key, "s44:"+p.Run+":"+p.Case+":") {
				started = true
				break
			}
		}
		if !started {
			f.plans[p.Run+":"+p.Case] = p.Behavior
		}
		f.mu.Unlock()
		if started {
			jsonReply(w, 409, map[string]string{"code": "plan_already_started"})
			return
		}
		jsonReply(w, 200, map[string]string{"code": "plan_ready"})
	case "/control/release":
		var m marker
		if decoder.Decode(&m) != nil || !markerPattern.MatchString(m.key()) {
			jsonReply(w, 400, map[string]string{"code": "invalid_marker"})
			return
		}
		f.mu.Lock()
		g := f.gates[m.key()]
		if g != nil && !g.done {
			select {
			case <-g.release:
			default:
				f.recordLocked(fixtureEvent{Kind: "gate_released", Run: m.Run, Case: m.Case, Index: m.Index})
				close(g.release)
			}
		}
		f.mu.Unlock()
		if g == nil {
			jsonReply(w, 404, map[string]string{"code": "gate_not_found"})
			return
		}
		jsonReply(w, 200, map[string]string{"code": "gate_released"})
	default:
		jsonReply(w, 404, map[string]string{"code": "control_not_found"})
	}
}

func jsonReply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(encodeJSON(body))
}

// 仅看 socket 对端，不信任代理头。附加 CIDR 必须完全落在 RFC1918/ULA/loopback 内。
func privatePrefixes(value string) ([]netip.Prefix, error) {
	prefixes := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}
	allowed := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("fc00::/7")}
	if value == "" {
		return prefixes, nil
	}
	for _, part := range strings.Split(value, ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fail("invalid_private_cidr")
		}
		p = p.Masked()
		ok := false
		for _, block := range append(prefixes[:2:2], allowed...) {
			if block.Addr().BitLen() == p.Addr().BitLen() && p.Bits() >= block.Bits() && block.Contains(p.Addr()) {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fail("public_cidr_forbidden")
		}
		prefixes = append(prefixes, p)
	}
	return prefixes, nil
}

func privateOnly(next http.Handler, prefixes []netip.Prefix) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		addr, parseErr := netip.ParseAddr(host)
		if err == nil && parseErr == nil {
			addr = addr.Unmap()
			for _, p := range prefixes {
				if p.Contains(addr) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}
		jsonReply(w, 403, map[string]string{"code": "private_network_required"})
	})
}

func serve(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	listen := flags.String("listen", "127.0.0.1:8080", "上游地址")
	control := flags.String("control-listen", "127.0.0.1:8081", "独立控制地址")
	cidr := flags.String("allow-cidr", "", "允许 socket 对端的私网 CIDR，逗号分隔")
	cert := flags.String("tls-cert", "", "TLS 公钥证书")
	key := flags.String("tls-key", "", "TLS 私钥文件")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*cert == "") != (*key == "") {
		return fail("invalid_serve_flags")
	}
	prefixes, err := privatePrefixes(*cidr)
	if err != nil {
		return err
	}
	f := newFixture()
	mux := http.NewServeMux()
	for _, path := range []string{"/v1/responses", "/backend-api/codex/responses"} {
		mux.HandleFunc(path, f.responses)
	}
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { jsonReply(w, 200, map[string]string{"status": "ready"}) })
	upstream := &http.Server{Addr: *listen, Handler: privateOnly(mux, prefixes), ReadHeaderTimeout: 5 * time.Second}
	controls := &http.Server{Addr: *control, Handler: privateOnly(http.HandlerFunc(f.control), prefixes), ReadHeaderTimeout: 5 * time.Second}
	failed := make(chan error, 2)
	go func() {
		if *cert != "" {
			failed <- upstream.ListenAndServeTLS(*cert, *key)
		} else {
			failed <- upstream.ListenAndServe()
		}
	}()
	go func() { failed <- controls.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case <-failed:
		err = fail("fixture_listener_failed")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = controls.Shutdown(shutdownCtx)
	_ = upstream.Shutdown(shutdownCtx)
	return err
}
