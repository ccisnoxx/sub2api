// s44-ws-fixture 仅用于隔离网络中的真实 Responses HTTP/WS 验证。
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if len(os.Args) < 2 {
		err = errors.New("command_required")
	} else {
		switch os.Args[1] {
		case "serve":
			err = serve(ctx, os.Args[2:])
		case "drive":
			err = drive(ctx, os.Args[2:], os.Stdout)
		default:
			err = errors.New("unknown_command")
		}
	}
	if err != nil {
		// 不输出网络错误原文、请求 URL、正文或认证头。
		_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"kind": "command_failed", "code": safeErrorCode(err)})
		os.Exit(1)
	}
}

type codedError struct{ code string }

func (e codedError) Error() string { return e.code }

func fail(code string) error { return codedError{code} }

func safeErrorCode(err error) string {
	var coded codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "fixture_error"
}

func encodeJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("fixture JSON 类型错误: %T", v))
	}
	return b
}
