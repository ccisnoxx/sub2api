//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 真实 socket 和生产选号、渠道映射、定价及用量 owner 共同保护换号后的请求归属。
func TestOpenAIWSCurrentTurnModelFailover(t *testing.T) {
	t.Run("select_only_current_model_account", func(t *testing.T) {
		runOpenAIWSCurrentTurnModelFailover(t, service.BillingModelSourceRequested, true, false, false)
	})
	t.Run("requested_price", func(t *testing.T) {
		runOpenAIWSCurrentTurnModelFailover(t, service.BillingModelSourceRequested, false, false, false)
	})
	t.Run("channel_mapped_price", func(t *testing.T) {
		runOpenAIWSCurrentTurnModelFailover(t, service.BillingModelSourceChannelMapped, false, false, false)
	})
	t.Run("current_image_intent_requires_responses", func(t *testing.T) {
		runOpenAIWSCurrentTurnModelFailover(t, service.BillingModelSourceRequested, false, true, false)
	})
	t.Run("current_string_input", func(t *testing.T) {
		runOpenAIWSCurrentTurnModelFailover(t, service.BillingModelSourceRequested, true, false, true)
	})
}

type openAIWSCurrentTurnModelHit struct {
	accountID int64
	model     string
	input     string
}

func runOpenAIWSCurrentTurnModelFailover(t *testing.T, billingSource string, restrictReplacement, imageIntent, currentStringInput bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	const (
		modelA    = "public-turn-a"
		modelB    = "public-turn-b"
		routeA    = "route-turn-a"
		routeB    = "route-turn-b"
		firstID   = int64(93601)
		onlyAID   = int64(93602)
		currentID = int64(93603)
	)
	groupID := int64(93604)
	var hitsMu sync.Mutex
	var hits []openAIWSCurrentTurnModelHit
	upstreamErrors := make(chan error, 8)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			upstreamErrors <- err
			return
		}
		defer func() { _ = conn.CloseNow() }()
		var accountID int64
		switch r.Header.Get("Authorization") {
		case "Bearer sk-model-first":
			accountID = firstID
		case "Bearer sk-model-only-a":
			accountID = onlyAID
		case "Bearer sk-model-current":
			accountID = currentID
		default:
			upstreamErrors <- fmt.Errorf("上游认证头不属于测试账号")
			return
		}
		for {
			readCtx, cancelRead := context.WithTimeout(r.Context(), 5*time.Second)
			_, payload, readErr := conn.Read(readCtx)
			cancelRead()
			if readErr != nil {
				return
			}
			model := gjson.GetBytes(payload, "model").String()
			input := gjson.GetBytes(payload, "input.0.content").String()
			if wireInput := gjson.GetBytes(payload, "input"); wireInput.Type == gjson.String {
				input = wireInput.String()
			}
			if currentStringInput && accountID == currentID {
				wireInput := gjson.GetBytes(payload, "input")
				if !wireInput.IsArray() {
					upstreamErrors <- fmt.Errorf("换号后的字符串请求没有展开为对象项数组")
					return
				}
				for _, item := range wireInput.Array() {
					if !item.IsObject() {
						t.Logf("实际换号载荷包含非法非对象项：%s", payload)
						upstreamErrors <- fmt.Errorf("换号后的 input 数组含非对象项")
						return
					}
				}
			}
			hitsMu.Lock()
			hits = append(hits, openAIWSCurrentTurnModelHit{accountID: accountID, model: model, input: input})
			hitsMu.Unlock()
			t.Logf("实际发送 account_id=%d model=%s input=%s", accountID, model, input)
			var event []byte
			if accountID == firstID && input == "turn-b" {
				event = []byte(`{"type":"error","error":{"code":"rate_limit_exceeded","type":"usage_limit_reached","message":"The usage limit has been reached"}}`)
			} else {
				event, err = json.Marshal(map[string]any{
					"type": "response.completed", "response": map[string]any{
						"id": "resp_" + input, "model": model, "status": "completed",
						"output": []any{map[string]any{"type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "complete"}}}},
						"usage":  map[string]any{"input_tokens": 11, "output_tokens": 2},
					},
				})
				if err != nil {
					upstreamErrors <- err
					return
				}
			}
			writeCtx, cancelWrite := context.WithTimeout(r.Context(), 5*time.Second)
			err = conn.Write(writeCtx, coderws.MessageText, event)
			cancelWrite()
			if err != nil {
				upstreamErrors <- err
				return
			}
			if accountID == firstID && input == "turn-b" {
				return
			}
		}
	}))
	t.Cleanup(upstream.Close)

	newAccount := func(id int64, token string, priority int, models map[string]any) service.Account {
		return service.Account{
			ID: id, Name: fmt.Sprintf("ws-current-model-%d", id), Platform: service.PlatformOpenAI,
			Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true,
			Concurrency: 1, Priority: priority,
			Credentials: map[string]any{"api_key": token, "base_url": upstream.URL, "model_mapping": models},
			Extra: map[string]any{
				"openai_apikey_responses_websockets_v2_enabled": true,
				"openai_apikey_responses_websockets_v2_mode":    service.OpenAIWSIngressModeDedicated,
			},
		}
	}
	currentModels := map[string]any{routeA: routeA, routeB: routeB}
	if restrictReplacement {
		currentModels = map[string]any{routeB: routeB}
	}
	accounts := []service.Account{
		newAccount(firstID, "sk-model-first", 1, map[string]any{routeA: routeA, routeB: routeB}),
		newAccount(currentID, "sk-model-current", 3, currentModels),
	}
	if restrictReplacement {
		onlyA := newAccount(onlyAID, "sk-model-only-a", 2, map[string]any{routeA: routeA})
		require.True(t, onlyA.IsModelSupported(routeA))
		require.False(t, onlyA.IsModelSupported(routeB))
		require.True(t, accounts[1].IsModelSupported(routeB))
		require.False(t, accounts[1].IsModelSupported(routeA))
		accounts = append(accounts, onlyA)
	}
	if imageIntent {
		chatOnly := newAccount(onlyAID, "sk-model-only-a", 2, map[string]any{routeA: routeA, routeB: routeB})
		chatOnly.Extra[openai_compat.ExtraKeyResponsesSupported] = false
		require.True(t, chatOnly.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityChatCompletions))
		require.False(t, chatOnly.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityResponses))
		require.True(t, accounts[1].SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityResponses))
		accounts = append(accounts, chatOnly)
	}
	repo := &grokCredentialHandlerRepo{accounts: accounts}
	usageRepo := &openAIWSUsageHandlerUsageLogRepoStub{created: make(chan *service.UsageLog, 4)}
	perRequestPricing := func(model string, price float64) service.ChannelModelPricing {
		return service.ChannelModelPricing{Platform: service.PlatformOpenAI, Models: []string{model}, BillingMode: service.BillingModePerRequest, PerRequestPrice: &price}
	}
	channelSvc := service.NewChannelService(&openAIWSUsageHandlerChannelRepoStub{
		channels: []service.Channel{{
			ID: 93605, Name: "ws-current-model-pricing", Status: service.StatusActive,
			GroupIDs: []int64{groupID}, BillingModelSource: billingSource,
			ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {modelA: routeA, modelB: routeB}},
			ModelPricing: []service.ChannelModelPricing{
				perRequestPricing(modelA, 0.125), perRequestPricing(modelB, 0.75),
				perRequestPricing(routeA, 2.5), perRequestPricing(routeB, 3.5),
			},
		}},
		groupPlatforms: map[int64]string{groupID: service.PlatformOpenAI},
	}, nil, nil, nil, nil)
	cfg := &config.Config{}
	cfg.RunMode = config.RunModeSimple
	cfg.Default.RateMultiplier = 1
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.ModeRouterV2Enabled = true
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 5
	cfg.Gateway.OpenAIWS.IngressInterTurnIdleTimeoutSeconds = 5
	billing := service.NewBillingService(cfg, nil)
	resolver := service.NewModelPricingResolver(channelSvc, billing)
	billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)
	gateway := service.NewOpenAIGatewayService(
		repo, usageRepo, nil, nil, nil, nil, nil, cfg, nil, nil,
		billing, service.NewRateLimitService(repo, nil, cfg, nil, nil), billingCache, nil,
		&service.DeferredService{}, nil, nil, resolver, channelSvc, nil, nil, nil,
	)
	t.Cleanup(gateway.CloseOpenAIWSPool)
	cache := &concurrencyCacheMock{
		acquireUserSlotFn:    func(context.Context, int64, int, string) (bool, error) { return true, nil },
		acquireAccountSlotFn: func(context.Context, int64, int, string) (bool, error) { return true, nil },
	}
	h := &OpenAIGatewayHandler{
		cfg: cfg, gatewayService: gateway, billingCacheService: billingCache,
		apiKeyService: &service.APIKeyService{}, maxAccountSwitches: 2,
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}
	apiKey := &service.APIKey{
		ID: 93606, UserID: 93607, GroupID: &groupID,
		User:  &service.User{ID: 93607, Status: service.StatusActive},
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 1, AllowImageGeneration: true},
	}
	finished := make(chan struct{}, 1)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), apiKey)
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.User.ID, Concurrency: 1})
		c.Next()
		finished <- struct{}{}
	})
	router.GET("/openai/v1/responses", h.ResponsesWebSocket)
	client := dialRoutingDiagnosticsWS(t, router)
	firstSelectionListCalls := 0
	for i, model := range []string{modelA, modelB} {
		input := []string{"turn-a", "turn-b"}[i]
		payload := fmt.Sprintf(`{"type":"response.create","model":%q,"input":[{"role":"user","content":%q}],"stream":false}`, model, input)
		if currentStringInput && i == 1 {
			payload = fmt.Sprintf(`{"type":"response.create","model":%q,"input":%q,"stream":false}`, model, input)
		}
		if imageIntent && i == 1 {
			payload = fmt.Sprintf(`{"type":"response.create","model":%q,"input":[{"role":"user","content":%q}],"tools":[{"type":"image_generation"}],"stream":false}`, model, input)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		require.NoError(t, client.Write(ctx, coderws.MessageText, []byte(payload)))
		_, event, readErr := client.Read(ctx)
		cancel()
		require.NoError(t, readErr)
		require.Equal(t, "response.completed", gjson.GetBytes(event, "type").String())
		require.Equal(t, "resp_"+input, gjson.GetBytes(event, "response.id").String())
		require.Equal(t, model, gjson.GetBytes(event, "response.model").String())
		if i == 0 {
			firstSelectionListCalls = repo.selectorCalls()
		}
	}
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("真实 handler 未退出")
	}
	hitsMu.Lock()
	actualHits := append([]openAIWSCurrentTurnModelHit(nil), hits...)
	hitsMu.Unlock()
	if restrictReplacement || imageIntent {
		require.Equal(t, []openAIWSCurrentTurnModelHit{
			{accountID: firstID, model: routeA, input: "turn-a"},
			{accountID: firstID, model: routeB, input: "turn-b"},
			{accountID: currentID, model: routeB, input: "turn-b"},
		}, actualHits, "当前 B 请求只能选支持当前模型及能力的候选")
	}
	logs := make(map[string]*service.UsageLog, 2)
	for range 2 {
		select {
		case log := <-usageRepo.created:
			logs[log.Model] = log
		case <-time.After(5 * time.Second):
			t.Fatal("用量未持久化")
		}
	}
	oracle := func(model string) *service.CostBreakdown {
		cost, err := billing.CalculateCostUnified(service.CostInput{
			Ctx: context.Background(), Model: model, GroupID: &apiKey.Group.ID, Group: apiKey.Group,
			Tokens: service.UsageTokens{InputTokens: 11, OutputTokens: 2}, RequestCount: 1,
			RateMultiplier: 1, Resolver: resolver,
		})
		require.NoError(t, err)
		return cost
	}
	priceModelA, priceModelB := modelA, modelB
	if billingSource == service.BillingModelSourceChannelMapped {
		priceModelA, priceModelB = routeA, routeB
	}
	oldCost, currentCost := oracle(priceModelA), oracle(priceModelB)
	require.NotEqual(t, oldCost.ActualCost, currentCost.ActualCost, "A/B 的真实定价 owner 必须产生不同费用")
	for _, expected := range []struct {
		model, route string
		account      int64
		cost         *service.CostBreakdown
	}{
		{modelA, routeA, firstID, oldCost}, {modelB, routeB, currentID, currentCost},
	} {
		log := logs[expected.model]
		require.NotNil(t, log)
		require.NotNil(t, log.UpstreamModel)
		t.Logf("实际用量 account_id=%d model=%s requested=%s upstream=%s total=%.6f actual=%.6f；定价 owner model=%s B=%.6f A=%.6f", log.AccountID, log.Model, log.RequestedModel, *log.UpstreamModel, log.TotalCost, log.ActualCost, priceModelB, currentCost.ActualCost, oldCost.ActualCost)
		require.InDelta(t, expected.cost.TotalCost, log.TotalCost, 1e-12, "费用必须来自所属请求的生产定价 owner")
		require.InDelta(t, expected.cost.ActualCost, log.ActualCost, 1e-12)
		require.Equal(t, expected.model, log.RequestedModel)
		require.Equal(t, expected.account, log.AccountID)
		require.Equal(t, expected.route, *log.UpstreamModel)
		require.NotNil(t, log.ModelMappingChain)
		require.Equal(t, expected.model+"→"+expected.route, *log.ModelMappingChain)
		require.NotNil(t, log.BillingMode)
		require.Equal(t, string(service.BillingModePerRequest), *log.BillingMode)
	}
	require.Empty(t, usageRepo.created, "429 的不可见 attempt 不应多记用量")
	require.Positive(t, firstSelectionListCalls)
	require.Equal(t, 2*firstSelectionListCalls, repo.selectorCalls(), "相对于首轮选号，恰好多一次当前轮次重选")
	require.Equal(t, []int64{firstID}, repo.rateLimitedAccountIDs())
	select {
	case err := <-upstreamErrors:
		require.NoError(t, err)
	default:
	}
}
