package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type driveOptions struct {
	URL, Control, Scenario, Model, Run, APIKey, CA string
	Count                                          int
	Timeout, Pace, CancelDelay                     time.Duration
}

type turnSpec struct {
	Case, Terminal string
	Cancel         bool
}

func drive(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("drive", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var o driveOptions
	flags.StringVar(&o.URL, "url", "", "实际网关 Responses WS/HTTP URL")
	flags.StringVar(&o.Control, "control", "", "fixture 控制接口 URL")
	flags.StringVar(&o.Scenario, "scenario", "ws-success", "场景")
	flags.StringVar(&o.Model, "model", "gpt-5.1", "请求模型；fixture 记录实际上游模型")
	flags.StringVar(&o.Run, "run", "", "证据 run 标识；默认随机生成")
	flags.StringVar(&o.CA, "ca-file", "", "测试 CA 文件，仍执行 TLS 证书校验")
	keyEnv := flags.String("api-key-env", "S44_API_KEY", "从此环境变量读取 API Key，不写入证据")
	flags.IntVar(&o.Count, "count", 0, "成功/失败场景的真实请求数；默认5")
	flags.DurationVar(&o.Timeout, "timeout", 20*time.Second, "单次 dial/turn/HTTP 超时")
	flags.DurationVar(&o.Pace, "pace", 0, "turn 之间等待")
	flags.DurationVar(&o.CancelDelay, "cancel-delay", 750*time.Millisecond, "客户端关闭到上游 drain 放行的间隔")
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return fail("invalid_drive_flags")
	}
	o.APIKey = os.Getenv(*keyEnv)
	if o.Run == "" {
		var raw [8]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return fail("run_id_failed")
		}
		o.Run = hex.EncodeToString(raw[:])
	}
	if !safeLabel.MatchString(o.Run) || !safeLabel.MatchString(o.Model) || o.Count < 0 || o.Count > 1000 || o.Timeout <= 0 || o.Pace < 0 || o.CancelDelay < 0 {
		return fail("invalid_drive_flags")
	}
	turns, err := scenarioTurns(o.Scenario, o.Count)
	if err != nil {
		return err
	}
	if strings.HasPrefix(o.Scenario, "http-") {
		if !validEndpoint(o.URL, "http", "https") {
			return fail("invalid_http_url")
		}
	} else if !validEndpoint(o.URL, "ws", "wss") {
		return fail("invalid_ws_url")
	}
	if (o.Scenario == "ws-cancel" || o.Scenario == "ws-cancel-probe") && !validEndpoint(o.Control, "http", "https") {
		return fail("cancel_requires_control")
	}
	client, err := driverHTTPClient(o.CA)
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	encoder := json.NewEncoder(out)
	emit := func(v any) error {
		if encoder.Encode(v) != nil {
			return fail("evidence_write_failed")
		}
		return nil
	}
	if err := emit(map[string]any{"schema_version": 1, "kind": "run_started", "run": o.Run, "scenario": o.Scenario, "requested_model": o.Model, "turns": len(turns), "at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
		return err
	}
	if strings.HasPrefix(o.Scenario, "http-") {
		err = driveHTTP(ctx, client, o, turns, emit)
	} else {
		err = driveWS(ctx, client, o, turns, emit)
	}
	if err != nil {
		_ = emit(map[string]any{"kind": "run_failed", "run": o.Run, "code": safeErrorCode(err), "at": time.Now().UTC().Format(time.RFC3339Nano)})
		return err
	}
	return emit(map[string]any{"kind": "run_complete", "run": o.Run, "scenario": o.Scenario, "verified_turns": len(turns), "at": time.Now().UTC().Format(time.RFC3339Nano), "database_verified": false})
}

func scenarioTurns(scenario string, count int) ([]turnSpec, error) {
	if count == 0 {
		count = 5
	}
	var single turnSpec
	switch scenario {
	case "ws-multiturn":
		return []turnSpec{{Case: "retry_close_failure", Terminal: "response.failed"}, {Case: "success_zero", Terminal: "response.completed"}}, nil
	case "ws-retry":
		return []turnSpec{{Case: "retry_close", Terminal: "response.completed"}}, nil
	case "ws-retry-429":
		return []turnSpec{{Case: "retry_429", Terminal: "response.completed"}}, nil
	case "ws-cancel":
		return []turnSpec{{Case: "cancel_drain", Cancel: true}}, nil
	case "ws-cancel-probe":
		return []turnSpec{{Case: "cancel_probe_drain", Cancel: true}}, nil
	case "http-retry-zero":
		return []turnSpec{{Case: "retry_429", Terminal: "response.completed"}}, nil
	case "ws-failure", "http-failure":
		single = turnSpec{Case: "provider_failure", Terminal: "response.failed"}
	case "ws-auth":
		single = turnSpec{Case: "provider_auth", Terminal: "response.failed"}
	case "ws-quota":
		single = turnSpec{Case: "provider_quota", Terminal: "response.failed"}
	case "ws-success", "http-success":
		single = turnSpec{Case: "success_zero", Terminal: "response.completed"}
	default:
		return nil, fail("unknown_scenario")
	}
	turns := make([]turnSpec, count)
	for i := range turns {
		turns[i] = single
	}
	return turns, nil
}

func validEndpoint(raw string, schemes ...string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	for _, scheme := range schemes {
		if u.Scheme == scheme {
			return true
		}
	}
	return false
}

func driverHTTPClient(ca string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// 隔离驱动不受进程继承的 HTTP_PROXY 影响，避免发送 fake 凭据到外部代理。
	transport.Proxy = nil
	if ca != "" {
		pem, err := os.ReadFile(ca)
		if err != nil {
			return nil, fail("ca_read_failed")
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, fail("system_ca_failed")
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fail("invalid_ca")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

func requestPayload(m marker, model string, ws bool) []byte {
	p := map[string]any{"model": model, "store": false, "stream": true, "input": []any{map[string]any{"role": "user", "content": []any{map[string]string{"type": "input_text", "text": m.key()}}}}}
	if ws {
		p["type"] = "response.create"
	} else {
		p["stream"] = false
	}
	return encodeJSON(p)
}

func driveWS(ctx context.Context, client *http.Client, o driveOptions, turns []turnSpec, emit func(any) error) error {
	headers := http.Header{}
	if o.APIKey != "" {
		headers.Set("Authorization", "Bearer "+o.APIKey)
	}
	headers.Set("OpenAI-Beta", "responses_websockets=2026-02-06")
	// 固定 header 同一连接所有 turn 共用，不能据此合并 observation_key。
	headers.Set("X-Client-Request-Id", "s44-"+o.Run)
	dialCtx, cancel := context.WithTimeout(ctx, o.Timeout)
	conn, resp, err := websocket.Dial(dialCtx, o.URL, &websocket.DialOptions{HTTPClient: client, HTTPHeader: headers, CompressionMode: websocket.CompressionContextTakeover})
	cancel()
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		_ = emit(map[string]any{"kind": "dial_failed", "run": o.Run, "status": status})
		return fail("websocket_dial_failed")
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1 << 20)
	for index, spec := range turns {
		m := marker{o.Run, spec.Case, strconv.Itoa(index + 1)}
		turnCtx, turnCancel := context.WithTimeout(ctx, o.Timeout)
		err = conn.Write(turnCtx, websocket.MessageText, requestPayload(m, o.Model, true))
		if err != nil {
			turnCancel()
			return fail("websocket_write_failed")
		}
		if err := emit(map[string]any{"kind": "turn_sent", "run": o.Run, "case": m.Case, "index": m.Index, "logical_turn_expected": index + 1, "at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
			turnCancel()
			return err
		}
		gotTerminal := false
		for frames := 0; frames < 100; frames++ {
			_, raw, readErr := conn.Read(turnCtx)
			if readErr != nil {
				turnCancel()
				return fail("websocket_read_failed")
			}
			var event struct {
				Type     string `json:"type"`
				Response struct {
					ID     string `json:"id"`
					Status string `json:"status"`
					Usage  *struct {
						Input  int `json:"input_tokens"`
						Output int `json:"output_tokens"`
					} `json:"usage"`
				} `json:"response"`
			}
			if json.Unmarshal(raw, &event) != nil {
				turnCancel()
				return fail("invalid_ws_json")
			}
			if !knownEventType(event.Type) {
				turnCancel()
				return fail("unexpected_ws_event")
			}
			if err := emit(map[string]any{"kind": "frame_received", "run": o.Run, "case": m.Case, "index": m.Index, "event_type": event.Type, "response_id_matches": event.Response.ID == m.responseID(), "at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
				turnCancel()
				return err
			}
			if spec.Cancel && event.Type == "response.created" {
				if event.Response.ID != m.responseID() {
					turnCancel()
					return fail("response_id_mismatch")
				}
				_ = conn.CloseNow()
				turnCancel()
				if err := emit(map[string]any{"kind": "client_closed_before_terminal", "run": o.Run, "case": m.Case, "index": m.Index, "at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
					return err
				}
				if err := waitPace(ctx, o.CancelDelay); err != nil {
					return err
				}
				if o.Scenario == "ws-cancel-probe" {
					if err := controlGate(ctx, client, o, m, "probe"); err != nil {
						return err
					}
					if err := emit(map[string]any{"kind": "nonterminal_probe_released", "run": o.Run, "at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
						return err
					}
					if err := waitPace(ctx, o.CancelDelay); err != nil {
						return err
					}
				}
				if err := releaseGate(ctx, client, o, m); err != nil {
					return err
				}
				return emit(map[string]any{"kind": "drain_released_after_client_close", "run": o.Run, "case": m.Case, "index": m.Index, "at": time.Now().UTC().Format(time.RFC3339Nano)})
			}
			if event.Type == "response.completed" || event.Type == "response.failed" {
				if event.Type != spec.Terminal || event.Response.ID != m.responseID() || "response."+event.Response.Status != spec.Terminal {
					turnCancel()
					return fail("unexpected_terminal")
				}
				if (m.Case == "success_zero" || m.Case == "retry_429") && (event.Response.Usage == nil || event.Response.Usage.Input != 0 || event.Response.Usage.Output != 0) {
					turnCancel()
					return fail("expected_zero_usage")
				}
				gotTerminal = true
				break
			}
		}
		turnCancel()
		if !gotTerminal {
			return fail("terminal_not_received")
		}
		if err := waitPace(ctx, o.Pace); err != nil {
			return err
		}
	}
	if err := conn.Close(websocket.StatusNormalClosure, "s44 completed"); err != nil {
		return fail("normal_close_failed")
	}
	return emit(map[string]any{"kind": "client_closed_after_terminal", "run": o.Run, "at": time.Now().UTC().Format(time.RFC3339Nano)})
}

func knownEventType(typ string) bool {
	switch typ {
	case "response.created", "response.in_progress", "response.completed", "response.failed", "response.output_item.added", "response.output_item.done", "response.content_part.added", "response.content_part.done", "response.output_text.delta", "response.output_text.done", "error":
		return true
	}
	return false
}

func driveHTTP(ctx context.Context, client *http.Client, o driveOptions, turns []turnSpec, emit func(any) error) error {
	for index, spec := range turns {
		m := marker{o.Run, spec.Case, strconv.Itoa(index + 1)}
		requestCtx, cancel := context.WithTimeout(ctx, o.Timeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, o.URL, bytes.NewReader(requestPayload(m, o.Model, false)))
		if err != nil {
			cancel()
			return fail("http_request_failed")
		}
		req.Header.Set("Content-Type", "application/json")
		if o.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+o.APIKey)
		}
		req.Header.Set("X-Client-Request-Id", "s44-"+o.Run)
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			return fail("http_send_failed")
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
		_ = resp.Body.Close()
		cancel()
		if readErr != nil || len(raw) > 1<<20 {
			return fail("http_read_failed")
		}
		var result struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Usage  *struct {
				Input  int `json:"input_tokens"`
				Output int `json:"output_tokens"`
			} `json:"usage"`
		}
		if json.Unmarshal(raw, &result) != nil {
			return fail("invalid_http_json")
		}
		terminal := "response." + result.Status
		zeroUsage := result.Usage != nil && result.Usage.Input == 0 && result.Usage.Output == 0
		if err := emit(map[string]any{"kind": "http_response_received", "run": o.Run, "case": m.Case, "index": m.Index, "status": resp.StatusCode, "response_id_matches": result.ID == m.responseID(), "zero_usage": zeroUsage, "at": time.Now().UTC().Format(time.RFC3339Nano)}); err != nil {
			return err
		}
		if resp.StatusCode != 200 || terminal != spec.Terminal || result.ID != m.responseID() {
			return fail("unexpected_http_terminal")
		}
		if (spec.Case == "retry_429" || spec.Case == "success_zero") && !zeroUsage {
			return fail("expected_zero_usage")
		}
		if err := waitPace(ctx, o.Pace); err != nil {
			return err
		}
	}
	return nil
}

func releaseGate(ctx context.Context, client *http.Client, o driveOptions, m marker) error {
	return controlGate(ctx, client, o, m, "release")
}

func controlGate(ctx context.Context, client *http.Client, o driveOptions, m marker, action string) error {
	releaseCtx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(releaseCtx, http.MethodPost, strings.TrimRight(o.Control, "/")+"/control/"+action, bytes.NewReader(encodeJSON(m)))
	if err != nil {
		return fail("control_request_failed")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fail("control_release_failed")
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		return fail("control_release_rejected")
	}
	return nil
}

func waitPace(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
