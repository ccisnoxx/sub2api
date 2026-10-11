//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/server/routes"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 复用真实迁移、SQL 仓库及用户路由；不注入 AuthSubject，也不模拟订阅谓词。
type catalogContractEnv struct {
	t        *testing.T
	ctx      context.Context
	users    service.UserRepository
	groups   service.GroupRepository
	channels service.ChannelRepository
	settings service.SettingRepository
	rates    service.UserGroupRateRepository
	auth     *service.AuthService
	billing  *service.BillingService
	userIDs  []int64
	groupIDs []int64
	chIDs    []int64
	prefix   string
}

func newCatalogContractEnv(t *testing.T) *catalogContractEnv {
	t.Helper()
	client := testEntClient(t)
	e := &catalogContractEnv{
		t: t, ctx: context.Background(), prefix: fmt.Sprintf("catalog-%d", time.Now().UnixNano()),
		users: NewUserRepository(client, integrationDB), groups: NewGroupRepository(client, integrationDB),
		channels: NewChannelRepository(integrationDB), settings: NewSettingRepository(client),
		rates: NewUserGroupRateRepository(integrationDB), billing: service.NewBillingService(&config.Config{}, nil),
	}
	cfg := &config.Config{JWT: config.JWTConfig{Secret: "catalog-contract-test-secret", AccessTokenExpireMinutes: 60}}
	e.auth = service.NewAuthService(nil, e.users, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	old, err := e.settings.GetAll(e.ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, id := range e.chIDs {
			require.NoError(t, e.channels.Delete(e.ctx, id))
		}
		for _, id := range e.userIDs {
			_, err := integrationDB.ExecContext(e.ctx, "DELETE FROM users WHERE id=$1", id)
			require.NoError(t, err)
		}
		for _, id := range e.groupIDs {
			_, err := integrationDB.ExecContext(e.ctx, "DELETE FROM groups WHERE id=$1", id)
			require.NoError(t, err)
		}
		e.setBackendMode(old[service.SettingKeyBackendModeEnabled] == "true")
		current, err := e.settings.GetAll(e.ctx)
		require.NoError(t, err)
		for key := range current {
			if _, ok := old[key]; !ok {
				require.NoError(t, e.settings.Delete(e.ctx, key))
			}
		}
		require.NoError(t, e.settings.SetMultiple(e.ctx, old))
	})
	require.NoError(t, e.settings.SetMultiple(e.ctx, map[string]string{
		service.SettingKeyAvailableChannelsEnabled: "true", "backend_mode_enabled": "false",
		"panel_rate_limit_settings": `{"enabled":false}`,
	}))
	e.setBackendMode(false)
	return e
}

func (e *catalogContractEnv) setBackendMode(enabled bool) {
	e.t.Helper()
	// 该设置有进程级缓存；遵循真实设置写入 owner，同步缓存而不等待 TTL。
	s := service.NewSettingService(e.settings, &config.Config{})
	current, err := s.GetAllSettings(e.ctx)
	require.NoError(e.t, err)
	current.BackendModeEnabled = enabled
	require.NoError(e.t, s.UpdateSettings(e.ctx, current))
}

func (e *catalogContractEnv) group(name string, mutate func(*service.Group)) *service.Group {
	e.t.Helper()
	g := &service.Group{Name: e.prefix + "-" + name, Platform: service.PlatformOpenAI,
		Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		RateMultiplier: 7, LongContextPricingEnabled: true}
	if mutate != nil {
		mutate(g)
	}
	require.NoError(e.t, e.groups.Create(e.ctx, g))
	e.groupIDs = append(e.groupIDs, g.ID)
	return g
}

func (e *catalogContractEnv) user(name, role string, allowed ...int64) *service.User {
	e.t.Helper()
	u := &service.User{Email: e.prefix + "-" + name + "@example.test", PasswordHash: "test-only",
		Status: service.StatusActive, Role: role, AllowedGroups: allowed, RestrictPublicGroups: true}
	require.NoError(e.t, e.users.Create(e.ctx, u))
	e.userIDs = append(e.userIDs, u.ID)
	return u
}

func (e *catalogContractEnv) channel(g *service.Group, name string, pricing ...service.ChannelModelPricing) *service.Channel {
	e.t.Helper()
	ch := &service.Channel{Name: e.prefix + "-" + name, Status: service.StatusActive,
		GroupIDs: []int64{g.ID}, BillingModelSource: service.BillingModelSourceChannelMapped,
		ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {"public-alias": "private-priced"}},
		ModelPricing: pricing}
	require.NoError(e.t, e.channels.Create(e.ctx, ch))
	e.chIDs = append(e.chIDs, ch.ID)
	return ch
}

func (e *catalogContractEnv) subscribe(userID, groupID int64, status string, expires time.Time) {
	e.t.Helper()
	_, err := testEntClient(e.t).UserSubscription.Create().SetUserID(userID).SetGroupID(groupID).
		SetStartsAt(time.Now().Add(-time.Hour)).SetExpiresAt(expires).SetStatus(status).Save(e.ctx)
	require.NoError(e.t, err)
}

func (e *catalogContractEnv) router() (*gin.Engine, *service.ChannelService) {
	e.t.Helper()
	gin.SetMode(gin.TestMode)
	s := service.NewSettingService(e.settings, &config.Config{})
	cs := service.NewChannelService(e.channels, e.groups, nil, nil, nil)
	h := handler.NewAvailableChannelHandler(cs,
		service.NewAPIKeyService(nil, e.users, e.groups, NewUserSubscriptionRepository(testEntClient(e.t)), e.rates, nil, nil), s, e.billing)
	r := gin.New()
	jwt := middleware.NewJWTAuthMiddleware(e.auth, service.NewUserService(e.users, nil, nil, nil), s, nil)
	// GET 不产生变更审计；其余鉴权、后台模式及 Redis 面板限流均使用生产 owner。
	audit := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	routes.RegisterUserRoutes(r.Group("/api/v1"), &handler.Handlers{AvailableChannel: h}, jwt, audit, s,
		middleware.NewPanelRateLimiter(testRedis(e.t), s))
	return r, cs
}

func catalogContractRequest(t *testing.T, r *gin.Engine, token, query string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/channels/available"+query, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type catalogContractResponse struct {
	Code int `json:"code"`
	Data struct {
		UserRateStatus string `json:"user_rate_status"`
		Groups         []struct {
			ID       int64    `json:"id"`
			UserRate *float64 `json:"user_rate_multiplier"`
			Rates    struct {
				Token float64   `json:"token"`
				At    time.Time `json:"pricing_at"`
			} `json:"rate_multipliers"`
			Models []struct {
				Name    string  `json:"name"`
				Status  string  `json:"price_status"`
				Reason  *string `json:"price_reason"`
				Pricing *struct {
					Effort map[string]float64 `json:"reasoning_effort_multipliers"`
					Time   *struct {
						Timezone     string `json:"timezone"`
						WeekdaysOnly bool   `json:"weekdays_only"`
						Periods      []struct {
							Start      string  `json:"start_time"`
							End        string  `json:"end_time"`
							Multiplier float64 `json:"multiplier"`
						} `json:"periods"`
					} `json:"time_pricing"`
					Tiers []struct {
						ServiceTier string `json:"service_tier"`
						Context     struct {
							Intervals []struct {
								Min       int      `json:"min_tokens"`
								Max       *int     `json:"max_tokens"`
								Input     *float64 `json:"input_price"`
								Output    *float64 `json:"output_price"`
								CacheRead *float64 `json:"cache_read_price"`
							} `json:"intervals"`
						} `json:"context"`
					} `json:"service_tiers"`
				} `json:"pricing"`
			} `json:"models"`
		} `json:"groups"`
	} `json:"data"`
}

func decodeCatalogContract(t *testing.T, w *httptest.ResponseRecorder) catalogContractResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got catalogContractResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Zero(t, got.Code)
	require.NotNil(t, got.Data.Groups)
	return got
}

func TestAvailableCatalogJWTDatabasePermissions(t *testing.T) {
	e := newCatalogContractEnv(t)
	public := e.group("public", nil)
	hidden := e.group("hidden", func(g *service.Group) { g.IsExclusive = true })
	active := e.group("sub-active", func(g *service.Group) {
		g.IsExclusive = true
		g.SubscriptionType = service.SubscriptionTypeSubscription
	})
	expired := e.group("sub-expired", func(g *service.Group) { g.SubscriptionType = service.SubscriptionTypeSubscription })
	other := e.group("sub-other", func(g *service.Group) { g.SubscriptionType = service.SubscriptionTypeSubscription })
	paused := e.group("sub-paused", func(g *service.Group) { g.SubscriptionType = service.SubscriptionTypeSubscription })
	disabled := e.group("disabled", func(g *service.Group) { g.Status = service.StatusDisabled })
	deleted := e.group("deleted", nil)
	_, err := integrationDB.ExecContext(e.ctx, "UPDATE groups SET deleted_at=NOW() WHERE id=$1", deleted.ID)
	require.NoError(t, err)
	empty := e.group("empty", nil)
	a := e.user("a", service.RoleUser, public.ID, expired.ID, other.ID, paused.ID, disabled.ID, deleted.ID, empty.ID)
	b := e.user("b", service.RoleUser, hidden.ID)
	admin := e.user("admin", service.RoleAdmin)
	e.subscribe(a.ID, active.ID, service.SubscriptionStatusActive, time.Now().Add(time.Hour))
	e.subscribe(a.ID, expired.ID, service.SubscriptionStatusActive, time.Now().Add(-time.Hour))
	e.subscribe(b.ID, other.ID, service.SubscriptionStatusActive, time.Now().Add(time.Hour))
	e.subscribe(a.ID, paused.ID, "expired", time.Now().Add(time.Hour))
	e.channel(public, "public-source")
	e.channel(hidden, "hidden-source")
	e.channel(expired, "expired-source")
	r, _ := e.router()
	tokenA, err := e.auth.GenerateToken(e.ctx, a)
	require.NoError(t, err)
	tokenB, err := e.auth.GenerateToken(e.ctx, b)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, token string
		ids         []int64
	}{
		{"用户A", tokenA, []int64{public.ID, active.ID, empty.ID}},
		{"用户B", tokenB, []int64{hidden.ID, other.ID}},
		{"再次用户A", tokenA, []int64{public.ID, active.ID, empty.ID}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := catalogContractRequest(t, r, tc.token, fmt.Sprintf("?view=catalog&user_id=%d&group_id=%d&admin=true", b.ID, hidden.ID))
			got := decodeCatalogContract(t, w)
			var ids []int64
			for _, g := range got.Data.Groups {
				ids = append(ids, g.ID)
				require.NotNil(t, g.Models)
			}
			require.ElementsMatch(t, tc.ids, ids)
			if tc.token == tokenA {
				require.NotContains(t, w.Body.String(), "hidden")
			}
		})
	}
	t.Run("旧数组与查询选择", func(t *testing.T) {
		for _, q := range []string{"", "?view=Catalog", "?view=catalog&view=catalog"} {
			w := catalogContractRequest(t, r, tokenA, q)
			require.Equal(t, http.StatusOK, w.Code)
			var got struct {
				Data []struct {
					Name string `json:"name"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			require.Len(t, got.Data, 1)
			require.Equal(t, e.prefix+"-public-source", got.Data[0].Name)
		}
	})
	t.Run("两个视图均拒绝无效主体", func(t *testing.T) {
		for _, q := range []string{"", "?view=catalog"} {
			for _, token := range []string{"", "invalid-jwt"} {
				require.Equal(t, http.StatusUnauthorized, catalogContractRequest(t, r, token, q).Code)
			}
		}
		// TokenVersion 由密码指纹派生；使用持久化密码变化检验旧 JWT 撤销。
		require.NoError(t, testEntClient(t).User.UpdateOneID(a.ID).SetPasswordHash("changed-test-only").Exec(e.ctx))
		for _, q := range []string{"", "?view=catalog"} {
			require.Equal(t, http.StatusUnauthorized, catalogContractRequest(t, r, tokenA, q).Code)
		}
		require.NoError(t, testEntClient(t).User.UpdateOneID(b.ID).SetStatus(service.StatusDisabled).Exec(e.ctx))
		for _, q := range []string{"", "?view=catalog"} {
			require.Equal(t, http.StatusUnauthorized, catalogContractRequest(t, r, tokenB, q).Code)
		}
	})
	t.Run("后台模式管理员仍只看自身", func(t *testing.T) {
		e.setBackendMode(true)
		user := e.user("mode", service.RoleUser, public.ID)
		token, err := e.auth.GenerateToken(e.ctx, user)
		require.NoError(t, err)
		adminToken, err := e.auth.GenerateToken(e.ctx, admin)
		require.NoError(t, err)
		r, _ := e.router()
		for _, q := range []string{"", "?view=catalog"} {
			require.Equal(t, http.StatusForbidden, catalogContractRequest(t, r, token, q).Code)
		}
		got := decodeCatalogContract(t, catalogContractRequest(t, r, adminToken, "?view=catalog&admin=true"))
		require.Empty(t, got.Data.Groups)
	})
	t.Run("两个视图共用真实Redis用户限流", func(t *testing.T) {
		e.setBackendMode(false)
		require.NoError(t, e.settings.SetMultiple(e.ctx, map[string]string{"backend_mode_enabled": "false",
			"panel_rate_limit_settings": `{"enabled":true,"user_rpm":1,"heavy_rpm":1,"exempt_admin":false}`}))
		user := e.user("rate", service.RoleUser, public.ID)
		token, err := e.auth.GenerateToken(e.ctx, user)
		require.NoError(t, err)
		r, _ := e.router()
		require.Equal(t, http.StatusOK, catalogContractRequest(t, r, token, "").Code)
		w := catalogContractRequest(t, r, token, "?view=catalog")
		require.Equal(t, http.StatusTooManyRequests, w.Code)
		require.NotEmpty(t, w.Header().Get("Retry-After"))
		require.NotContains(t, w.Body.String(), `"groups"`)
	})
}

func TestAvailableCatalogHTTPBillingParity(t *testing.T) {
	e := newCatalogContractEnv(t)
	g := e.group("priced", func(g *service.Group) {
		g.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"public-alias", "free-private", "unpriced-private"}}
	})
	ptr := func(v float64) *float64 { return &v }
	max := 100
	e.channel(g, "priced-source",
		service.ChannelModelPricing{Platform: service.PlatformOpenAI, Models: []string{"private-priced"},
			InputPrice: ptr(2e-6), OutputPrice: ptr(5e-6), CacheReadPrice: ptr(0.25e-6),
			FastMultiplier: ptr(4), FlexMultiplier: ptr(0.2), ReasoningEffortMultipliers: map[string]float64{"high": 1.5},
			TimePricing: &service.ChannelTimePricing{Timezone: "UTC", Periods: []service.ChannelTimePricingPeriod{{StartTime: "00:00", EndTime: "23:59", Multiplier: 2}}},
			Intervals:   []service.PricingInterval{{MinTokens: 0, MaxTokens: &max}, {MinTokens: 100, InputMultiplier: ptr(2), OutputMultiplier: ptr(1.5)}}},
		service.ChannelModelPricing{Platform: service.PlatformOpenAI, Models: []string{"free-private"}, InputPrice: ptr(0), OutputPrice: ptr(0)},
		service.ChannelModelPricing{Platform: service.PlatformOpenAI, Models: []string{"unpriced-private"}},
		service.ChannelModelPricing{Platform: service.PlatformAnthropic, Models: []string{"cross-platform"}, InputPrice: ptr(9e-6)},
	)
	u := e.user("billing", service.RoleUser, g.ID)
	token, err := e.auth.GenerateToken(e.ctx, u)
	require.NoError(t, err)
	r, cs := e.router()
	production := service.NewModelPricingResolver(cs, e.billing)
	actualGroup, err := e.groups.GetByID(e.ctx, g.ID)
	require.NoError(t, err)
	// HTTP 返回的公开名先走真实映射 owner，计费输入复用最终计费模型。
	mapped := cs.ResolveChannelMapping(e.ctx, g.ID, "public-alias")
	require.Equal(t, "private-priced", mapped.MappedModel)
	comparisons := 0
	for _, rate := range []float64{0.5, 0} {
		t.Run(fmt.Sprintf("个人倍率%g", rate), func(t *testing.T) {
			_, err := integrationDB.ExecContext(e.ctx, `INSERT INTO user_group_rate_multipliers(user_id,group_id,rate_multiplier) VALUES($1,$2,$3) ON CONFLICT(user_id,group_id) DO UPDATE SET rate_multiplier=EXCLUDED.rate_multiplier`, u.ID, g.ID, rate)
			require.NoError(t, err)
			got := decodeCatalogContract(t, catalogContractRequest(t, r, token, "?view=catalog"))
			require.Equal(t, "loaded", got.Data.UserRateStatus)
			require.Len(t, got.Data.Groups, 1)
			row := got.Data.Groups[0]
			require.Equal(t, ptr(rate), row.UserRate)
			require.Equal(t, rate, row.Rates.Token)
			require.Len(t, row.Models, 3)
			modelNames := make([]string, 0, len(row.Models))
			for _, offer := range row.Models {
				modelNames = append(modelNames, offer.Name)
			}
			require.ElementsMatch(t, []string{"public-alias", "free-private", "unpriced-private"}, modelNames)
			for _, offer := range row.Models {
				switch offer.Name {
				case "free-private":
					require.Equal(t, "resolved", offer.Status)
					require.Equal(t, ptr(0), offer.Pricing.Tiers[0].Context.Intervals[0].Input)
				case "unpriced-private":
					require.Equal(t, "unknown", offer.Status)
					require.Equal(t, "pricing_unavailable", *offer.Reason)
					require.Nil(t, offer.Pricing)
				case "public-alias":
					require.Equal(t, "resolved", offer.Status)
					require.NotNil(t, offer.Pricing)
					tierNames := make([]string, 0, len(offer.Pricing.Tiers))
					for _, tier := range offer.Pricing.Tiers {
						tierNames = append(tierNames, tier.ServiceTier)
					}
					require.ElementsMatch(t, []string{"default", "priority", "flex", "ultrafast"}, tierNames)
					require.Equal(t, map[string]float64{"high": 1.5}, offer.Pricing.Effort)
					require.NotNil(t, offer.Pricing.Time)
					require.Equal(t, "UTC", offer.Pricing.Time.Timezone)
					require.False(t, offer.Pricing.Time.WeekdaysOnly)
					require.Len(t, offer.Pricing.Time.Periods, 1)
					period := offer.Pricing.Time.Periods[0]
					require.Equal(t, "00:00", period.Start)
					require.Equal(t, "23:59", period.End)
					require.Equal(t, 2.0, period.Multiplier)
					for _, tier := range offer.Pricing.Tiers {
						for _, n := range []int{1, 100, 101, 300} {
							for _, cached := range []bool{false, true} {
								tokens := service.UsageTokens{InputTokens: n, OutputTokens: 17}
								if cached {
									tokens.InputTokens = 0
									tokens.CacheReadTokens = n
								}
								cost, err := e.billing.CalculateCostUnified(service.CostInput{Ctx: e.ctx, Model: mapped.MappedModel, Group: actualGroup, GroupID: &g.ID,
									Tokens: tokens, ServiceTier: tier.ServiceTier, ReasoningEffort: "high", RateMultiplier: rate, PricingAt: row.Rates.At, Resolver: production})
								require.NoError(t, err)
								matched := false
								for _, interval := range tier.Context.Intervals {
									if n <= interval.Min || (interval.Max != nil && n > *interval.Max) {
										continue
									}
									matched = true
									input := interval.Input
									if cached {
										input = interval.CacheRead
									}
									require.NotNil(t, input)
									require.NotNil(t, interval.Output)
									// 单价已包含服务档策略；个人倍率、分时和最终 effort 各应用一次。
									timeRate := 1.0
									if row.Rates.At.Hour() != 23 || row.Rates.At.Minute() < 59 {
										timeRate = period.Multiplier
									}
									expected := (float64(n)*(*input) + 17*(*interval.Output)) * row.Rates.Token * timeRate * offer.Pricing.Effort["high"]
									require.InDelta(t, expected, cost.ActualCost, 1e-12, "%s/%d/cache=%t/rate=%g", tier.ServiceTier, n, cached, rate)
									comparisons++
								}
								require.True(t, matched, "HTTP 阶梯必须覆盖计费上下文")
							}
						}
					}
				default:
					t.Fatalf("目录不应扩增模型：%s", offer.Name)
				}
			}
		})
	}
	require.Equal(t, 64, comparisons, "报价对账不能因响应缺项而减少")
}
