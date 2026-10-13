//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type responsesPartialUsageLogRepo struct {
	service.UsageLogRepository
	mu       sync.Mutex
	logs     []*service.UsageLog
	recorded chan struct{}
}

func (r *responsesPartialUsageLogRepo) Create(_ context.Context, log *service.UsageLog) (bool, error) {
	r.mu.Lock()
	r.logs = append(r.logs, log)
	r.mu.Unlock()
	r.recorded <- struct{}{}
	return true, nil
}

type responsesPartialUsageUpstream struct {
	service.HTTPUpstream
	code  string
	calls int
}

func (u *responsesPartialUsageUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	u.calls++
	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_partial\"}}\n\n"
	if u.code == "image_failure" {
		body += "data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"img_partial\",\"type\":\"image_generation_call\",\"status\":\"completed\",\"result\":\"YWJj\"}}\n\n"
	}
	body += "data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_partial\",\"model\":\"gpt-5.1\",\"usage\":{\"input_tokens\":1200,\"output_tokens\":300,\"input_tokens_details\":{\"cached_tokens\":400}},\"error\":{\"code\":\"" + u.code + "\",\"message\":\"upstream failure\"}}}\n\n"
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}, "X-Request-Id": {"req-partial-handler"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestResponsesPartialUsageRecordsFailedRequestOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, code := range []string{"server_error", "cyber_policy", "image_failure"} {
			name := "native"
			if passthrough {
				name = "passthrough"
			}
			t.Run(name+"/"+code, func(t *testing.T) {
				cfg := &config.Config{RunMode: config.RunModeSimple}
				cfg.Default.RateMultiplier = 1
				cfg.Gateway.OpenAIStreamEarlyPreamble = true
				account := service.Account{ID: 1, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"api_key": "test-key"}, Extra: map[string]any{"openai_passthrough": passthrough}}
				repo := &responsesPartialUsageLogRepo{recorded: make(chan struct{}, 4)}
				upstream := &responsesPartialUsageUpstream{code: code}
				billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
				t.Cleanup(billingCache.Stop)
				gateway := service.NewOpenAIGatewayService(openAIImagesFailoverAccountRepo{accounts: []service.Account{account}}, repo, nil, nil, nil, nil, nil, cfg, nil, nil, service.NewBillingService(cfg, nil), nil, billingCache, upstream, &service.DeferredService{}, nil, nil, nil, nil, nil, nil, nil)
				h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billingCache, &service.APIKeyService{}, nil, nil, nil, nil, cfg)
				body := `{"model":"gpt-5.1","stream":true,"input":"hello"}`
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
				core, observedLogs := observer.New(zap.DebugLevel)
				c.Request = c.Request.WithContext(logger.IntoContext(context.Background(), zap.New(core)))
				c.Request.Header.Set("Content-Type", "application/json")
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 2, User: &service.User{ID: 3, Status: service.StatusActive}, Group: &service.Group{Platform: service.PlatformOpenAI}})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 3, Concurrency: 0})
				h.Responses(c)
				select {
				case <-repo.recorded:
				case <-time.After(time.Second):
					t.Fatal("失败请求没有进入用量记录入口")
				}
				if code == "cyber_policy" {
					select {
					case <-repo.recorded:
						t.Fatal("安全拒绝的完整 result 与 cyber 兜底重复记录了同一次用量")
					case <-time.After(200 * time.Millisecond):
					}
				}
				repo.mu.Lock()
				logs := append([]*service.UsageLog(nil), repo.logs...)
				repo.mu.Unlock()
				require.Len(t, logs, 1)
				require.Equal(t, 800, logs[0].InputTokens)
				require.Equal(t, 400, logs[0].CacheReadTokens, "失败请求必须沿用完整 result，保留缓存计费明细")
				require.Equal(t, 300, logs[0].OutputTokens)
				require.Equal(t, account.ID, logs[0].AccountID)
				require.Equal(t, 1, upstream.calls)
				if code == "image_failure" {
					require.Equal(t, 1, logs[0].ImageCount)
					require.NotEmpty(t, observedLogs.FilterMessage("openai.forward_failed").All(), "有图片的部分结果也必须按失败终止与上报")
					require.Empty(t, observedLogs.FilterMessage("openai.request_completed").All())
				}
				if code == "cyber_policy" {
					require.Equal(t, service.RequestTypeCyberBlocked, logs[0].RequestType)
				}
				require.Equal(t, 1, strings.Count(rec.Body.String(), `"type":"response.failed"`))
			})
		}
	}
}
