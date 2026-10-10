package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func fixtureServer(t *testing.T) (*fixture, *httptest.Server, *httptest.Server) {
	t.Helper()
	f := newFixture()
	upstream := httptest.NewServer(http.HandlerFunc(f.responses))
	control := httptest.NewServer(http.HandlerFunc(f.control))
	t.Cleanup(upstream.Close)
	t.Cleanup(control.Close)
	return f, upstream, control
}

func dialFixture(t *testing.T, target string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(target, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func readFixture(t *testing.T, c *websocket.Conn) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, raw, err := c.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var e struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return e.Type
}

func writeFixture(t *testing.T, c *websocket.Conn, m marker) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, requestPayload(m, "gpt-5.1", true)); err != nil {
		t.Fatal(err)
	}
}

func TestRealSocketRetryAndNewTurnBoundary(t *testing.T) {
	f, server, _ := fixtureServer(t)
	m := marker{"retryboundary", "retry_close_failure", "1"}
	first := dialFixture(t, server.URL)
	writeFixture(t, first, m)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_, _, err := first.Read(ctx)
	cancel()
	if err == nil {
		t.Fatal("首尝试必须在任何输出前断开")
	}
	second := dialFixture(t, server.URL)
	writeFixture(t, second, m)
	if got := readFixture(t, second); got != "response.created" {
		t.Fatal(got)
	}
	if got := readFixture(t, second); got != "response.failed" {
		t.Fatal(got)
	}
	writeFixture(t, second, marker{"retryboundary", "success_zero", "2"})
	if got := readFixture(t, second); got != "response.created" {
		t.Fatal(got)
	}
	if got := readFixture(t, second); got != "response.completed" {
		t.Fatal(got)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var hits []fixtureEvent
	for _, e := range f.events {
		if e.Kind == "request_received" {
			hits = append(hits, e)
		}
	}
	if len(hits) != 3 || hits[0].Attempt != 1 || hits[1].Attempt != 2 || hits[2].Attempt != 1 {
		t.Fatalf("attempt边界错误: %+v", hits)
	}
	if hits[0].Session == hits[1].Session || hits[1].Session != hits[2].Session || hits[1].LocalTurn != 1 || hits[2].LocalTurn != 2 {
		t.Fatalf("socket局部轮次错误: %+v", hits)
	}
}

func TestActualDriversZeroHTTPAndFiveWSFailures(t *testing.T) {
	_, server, _ := fixtureServer(t)
	t.Setenv("S44_API_KEY", "sk-s44-fixture-second")
	for _, scenario := range []string{"http-success", "ws-failure", "ws-success"} {
		t.Run(scenario, func(t *testing.T) {
			target := server.URL
			if strings.HasPrefix(scenario, "ws-") {
				target = "ws" + strings.TrimPrefix(target, "http")
			}
			var out bytes.Buffer
			if err := drive(context.Background(), []string{"--url", target, "--scenario", scenario, "--run", scenario}, &out); err != nil {
				t.Fatalf("%s: %s", safeErrorCode(err), out.String())
			}
			if !strings.Contains(out.String(), `"kind":"run_complete"`) || !strings.Contains(out.String(), `"verified_turns":5`) {
				t.Fatal(out.String())
			}
			if strings.Contains(out.String(), "sk-s44") || strings.Contains(out.String(), server.URL) || strings.Contains(out.String(), "synthetic output") {
				t.Fatal("证据泄漏非白名单内容")
			}
		})
	}
}

func TestHTTPCapacityRetryAndFrozenControlPlan(t *testing.T) {
	f, server, control := fixtureServer(t)
	m := marker{"httpcapacity", "retry_429", "1"}
	for _, expected := range []int{429, 200} {
		resp, err := http.Post(server.URL, "application/json", bytes.NewReader(requestPayload(m, "gpt-5.1", false)))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != expected {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		if expected == 200 && (!bytes.Contains(body, []byte(`"input_tokens":0`)) || !bytes.Contains(body, []byte(`"status":"completed"`))) {
			t.Fatal(string(body))
		}
	}
	resp, err := http.Post(control.URL+"/control/plan", "application/json", strings.NewReader(`{"run":"httpcapacity","case":"retry_429","behavior":"provider_failure"}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatal("已开始请求的计划必须拒绝修改")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.attempts[m.key()] != 2 {
		t.Fatal("真实HTTP请求应复用marker而累计attempt")
	}
}

func TestCancelBeforeUpstreamDrainCompletion(t *testing.T) {
	f, upstream, control := fixtureServer(t)
	// 此 relay 只透传真实帧；下游关闭后继续读上游，模拟 gateway 的 drain 边界。
	// 应用终态及 observation key 由父代理在真实部署中另行验证。
	drained := make(chan bool, 1)
	var handlers sync.WaitGroup
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlers.Add(1)
		defer handlers.Done()
		down, err := websocket.Accept(w, r, nil)
		if err != nil {
			drained <- false
			return
		}
		defer down.CloseNow()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		up, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(upstream.URL, "http"), nil)
		if err != nil {
			drained <- false
			return
		}
		defer up.CloseNow()
		_, raw, err := down.Read(ctx)
		if err != nil || up.Write(ctx, websocket.MessageText, raw) != nil {
			drained <- false
			return
		}
		_, created, err := up.Read(ctx)
		if err != nil || down.Write(ctx, websocket.MessageText, created) != nil {
			drained <- false
			return
		}
		_, completed, err := up.Read(ctx)
		drained <- err == nil && bytes.Contains(completed, []byte(`"type":"response.completed"`))
	}))
	t.Cleanup(relay.Close)
	var out bytes.Buffer
	err := drive(context.Background(), []string{"--url", "ws" + strings.TrimPrefix(relay.URL, "http"), "--control", control.URL, "--scenario", "ws-cancel", "--run", "canceldrain", "--cancel-delay", "20ms"}, &out)
	if err != nil {
		t.Fatalf("%s: %s", safeErrorCode(err), out.String())
	}
	select {
	case ok := <-drained:
		if !ok {
			t.Fatal("真实上游socket未得到drain完成")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("drain超时")
	}
	handlers.Wait()
	f.mu.Lock()
	defer f.mu.Unlock()
	var release, completion int64
	for _, e := range f.events {
		if e.Kind == "gate_released" {
			release = e.Sequence
		}
		if e.EventType == "response.completed" && e.Written != nil && *e.Written {
			completion = e.Sequence
		}
	}
	if release == 0 || completion <= release {
		t.Fatalf("取消放行与上游完成顺序错误: release=%d completion=%d", release, completion)
	}
	if !strings.Contains(out.String(), `"kind":"client_closed_before_terminal"`) {
		t.Fatal(out.String())
	}
}

func TestControlSocketBoundaryAndEvidenceLimit(t *testing.T) {
	prefixes, err := privatePrefixes("172.30.44.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := privatePrefixes("0.0.0.0/0"); err == nil {
		t.Fatal("不得允许公网CIDR")
	}
	h := privateOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), prefixes)
	for _, addr := range []string{"203.0.113.1:4000", "172.30.44.2:4000"} {
		r := httptest.NewRequest("GET", "/control/evidence", nil)
		r.RemoteAddr = addr
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		expected := 403
		if strings.HasPrefix(addr, "172.30.") {
			expected = 204
		}
		if w.Code != expected {
			t.Fatalf("socket ACL status=%d", w.Code)
		}
	}
	f := newFixture()
	f.limit = 1
	f.record(fixtureEvent{Kind: "test"})
	if f.record(fixtureEvent{Kind: "test"}) {
		t.Fatal("满额证据不得默默覆盖")
	}
	w := httptest.NewRecorder()
	f.control(w, httptest.NewRequest("GET", "/control/evidence", nil))
	if w.Code != 507 || !strings.Contains(w.Body.String(), `"evidence_complete":false`) {
		t.Fatal("证据缺口必须显式报告")
	}
}

func TestNonterminalProbeAndCompletionHaveIndependentGates(t *testing.T) {
	f, upstream, control := fixtureServer(t)
	c := dialFixture(t, upstream.URL)
	m := marker{"independentgates", "cancel_probe_drain", "1"}
	writeFixture(t, c, m)
	if got := readFixture(t, c); got != "response.created" {
		t.Fatal(got)
	}
	o := driveOptions{Control: control.URL, Timeout: 3 * time.Second}
	if err := controlGate(context.Background(), http.DefaultClient, o, m, "probe"); err != nil {
		t.Fatal(err)
	}
	if got := readFixture(t, c); got != "response.output_text.delta" {
		t.Fatal(got)
	}
	f.mu.Lock()
	for _, e := range f.events {
		if e.EventType == "response.completed" {
			f.mu.Unlock()
			t.Fatal("probe 不能放行完成帧")
		}
	}
	f.mu.Unlock()
	if err := releaseGate(context.Background(), http.DefaultClient, o, m); err != nil {
		t.Fatal(err)
	}
	if got := readFixture(t, c); got != "response.completed" {
		t.Fatal(got)
	}
}
