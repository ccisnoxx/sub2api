//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 仓库边界夹具保留真实 GetAvailableGroups、渠道筛选及价格 owner。
// 嵌入接口的未实现方法会失败，避免目录误触写入或其他读取时仍通过。
type catalogUserRepo struct {
	service.UserRepository
	users  map[int64]*service.User
	err    error
	events *[]string
}

func (r *catalogUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	*r.events = append(*r.events, "user")
	return r.users[id], r.err
}

type catalogGroupRepo struct {
	service.GroupRepository
	groups []service.Group
	err    error
	events *[]string
}

func (r *catalogGroupRepo) ListActive(context.Context) ([]service.Group, error) {
	*r.events = append(*r.events, "groups")
	out := make([]service.Group, 0)
	for _, g := range r.groups {
		if g.IsActive() {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, r.err
}

type catalogSubRepo struct {
	service.UserSubscriptionRepository
	subs   []service.UserSubscription
	err    error
	events *[]string
}

func (r *catalogSubRepo) ListActiveByUserID(_ context.Context, id int64) ([]service.UserSubscription, error) {
	*r.events = append(*r.events, "subscriptions")
	out := make([]service.UserSubscription, 0)
	// 与既有仓库的当前用户、active、严格未过期谓词一致。
	for _, s := range r.subs {
		if s.UserID == id && s.IsActive() {
			out = append(out, s)
		}
	}
	return out, r.err
}

type catalogRateRepo struct {
	service.UserGroupRateRepository
	rates     map[int64]map[int64]float64
	err       error
	afterRead func()
	events    *[]string
}

func (r *catalogRateRepo) GetByUserID(_ context.Context, id int64) (map[int64]float64, error) {
	*r.events = append(*r.events, "rates")
	if r.afterRead != nil {
		r.afterRead()
	}
	return r.rates[id], r.err
}

type catalogChannelRepo struct {
	service.ChannelRepository
	channels []service.Channel
	err      error
	events   *[]string
}

func (r *catalogChannelRepo) ListAll(context.Context) ([]service.Channel, error) {
	*r.events = append(*r.events, "channels")
	return r.channels, r.err
}

type catalogSettingRepo struct {
	service.SettingRepository
	enabled bool
	err     error
}

func (r *catalogSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	v := "false"
	if r.enabled {
		v = "true"
	}
	return map[string]string{service.SettingKeyAvailableChannelsEnabled: v}, r.err
}

type catalogFixture struct {
	handler  *AvailableChannelHandler
	users    *catalogUserRepo
	groups   *catalogGroupRepo
	subs     *catalogSubRepo
	rates    *catalogRateRepo
	channels *catalogChannelRepo
	settings *catalogSettingRepo
	events   []string
}

func catalogFloat(v float64) *float64 { return &v }
func catalogFixtureForHTTP() *catalogFixture {
	f := &catalogFixture{}
	f.users = &catalogUserRepo{events: &f.events, users: map[int64]*service.User{
		1: {ID: 1, AllowedGroups: []int64{1}, RestrictPublicGroups: true},
		2: {ID: 2, AllowedGroups: []int64{2}, RestrictPublicGroups: true},
	}}
	f.groups = &catalogGroupRepo{events: &f.events, groups: []service.Group{
		{ID: 1, Name: "public-group", Description: "visible description", Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 2},
		{ID: 2, Name: "hidden-group", Platform: service.PlatformOpenAI, Status: service.StatusActive, RateMultiplier: 4, IsExclusive: true},
	}}
	f.subs = &catalogSubRepo{events: &f.events}
	f.rates = &catalogRateRepo{events: &f.events, rates: map[int64]map[int64]float64{1: {1: 0, 2: 99}, 2: {2: 0.25}}}
	f.channels = &catalogChannelRepo{events: &f.events, channels: []service.Channel{
		{ID: 731, Name: "visible-source", Description: "source description", Status: service.StatusActive, GroupIDs: []int64{1},
			BillingModelSource: service.BillingModelSourceChannelMapped,
			ModelMapping:       map[string]map[string]string{service.PlatformOpenAI: {"Alias": "internal-target", "unpriced-private-model": "unpriced-private-model"}},
			ModelPricing: []service.ChannelModelPricing{{ID: 991, ChannelID: 731, Platform: service.PlatformOpenAI, Models: []string{"internal-target"}, BillingMode: service.BillingModeToken, InputPrice: catalogFloat(0), OutputPrice: catalogFloat(0)},
				{Platform: service.PlatformAnthropic, Models: []string{"claude-other-platform"}, InputPrice: catalogFloat(9e-6)}},
			AccountStatsPricingRules: []service.AccountStatsPricingRule{{ID: 995, AccountIDs: []int64{996}}},
		},
		{ID: 732, Name: "hidden-source", Status: service.StatusActive, GroupIDs: []int64{2}, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{"hidden-model"}, InputPrice: catalogFloat(9e-6)}}},
	}}
	f.settings = &catalogSettingRepo{enabled: true}
	f.handler = NewAvailableChannelHandler(
		service.NewChannelService(f.channels, f.groups, nil, nil, nil),
		service.NewAPIKeyService(nil, f.users, f.groups, f.subs, f.rates, nil, nil),
		service.NewSettingService(f.settings, &config.Config{}),
		service.NewBillingService(&config.Config{}, nil),
	)
	return f
}
func catalogRequest(t *testing.T, h *AvailableChannelHandler, query string, userID int64) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channels/available"+query, nil)
	if userID != 0 {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: userID})
	}
	h.List(c)
	return w
}
func readCatalog(t *testing.T, w *httptest.ResponseRecorder) userModelCatalog {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var envelope struct {
		Code    int              `json:"code"`
		Message string           `json:"message"`
		Data    userModelCatalog `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, "success", envelope.Message)
	return envelope.Data
}

func TestAvailableModelCatalog_QuerySelectionAndLegacyCompatibility(t *testing.T) {
	for _, tc := range []struct {
		query   string
		catalog bool
	}{
		{"", false}, {"?view=catalog", true}, {"?view=%63atalog", true}, {"?v%69ew=catalog", true},
		{"?view=", false}, {"?view=Catalog", false}, {"?view=CATALOG", false}, {"?view=other", false}, {"?view=catalog%20", false},
		{"?view=catalog&view=catalog", false}, {"?view=catalog&view=", false}, {"?view=&view=catalog", false},
		{"?user_id=2&group_id=2&admin=true", false}, {"?view=catalog&user_id=2&group_id=2&admin=true", true},
	} {
		t.Run(tc.query, func(t *testing.T) {
			f := catalogFixtureForHTTP()
			f.groups.groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"Alias"}}
			w := catalogRequest(t, f.handler, tc.query, 1)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			if tc.catalog {
				got := readCatalog(t, w)
				require.Len(t, got.Groups, 1)
				require.Equal(t, int64(1), got.Groups[0].ID)
				require.Len(t, got.Groups[0].Models, 1)
				require.Equal(t, "Alias", got.Groups[0].Models[0].Name)
				require.Equal(t, []string{"user", "groups", "subscriptions", "channels", "rates"}, f.events)
			} else {
				var envelope struct {
					Data []userAvailableChannel `json:"data"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
				require.Len(t, envelope.Data, 1)
				require.Equal(t, "visible-source", envelope.Data[0].Name)
				// 旧数组的候选模型与白名单行为保持原样，也不读取个人倍率。
				require.Len(t, envelope.Data[0].Platforms[0].SupportedModels, 3)
				require.Equal(t, []string{"user", "groups", "subscriptions", "channels", "groups"}, f.events)
			}
		})
	}
}

func TestAvailableModelCatalog_AuthenticationAndFeatureShortCircuit(t *testing.T) {
	for _, query := range []string{"", "?view=catalog"} {
		require.Equal(t, http.StatusUnauthorized, catalogRequest(t, &AvailableChannelHandler{}, query, 0).Code)
		w := catalogRequest(t, &AvailableChannelHandler{}, query, 1)
		require.Equal(t, http.StatusOK, w.Code)
		if query == "" {
			require.JSONEq(t, `{"code":0,"message":"success","data":[]}`, w.Body.String())
		} else {
			require.JSONEq(t, `{"code":0,"message":"success","data":{"groups":[],"user_rate_status":"not_requested"}}`, w.Body.String())
		}
	}
	for _, failure := range []bool{false, true} {
		f := catalogFixtureForHTTP()
		f.settings.enabled = false
		if failure {
			f.settings.enabled = true
			f.settings.err = errors.New("setting read failed")
		}
		got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
		require.NotNil(t, got.Groups)
		require.Empty(t, got.Groups)
		require.Equal(t, "not_requested", got.UserRateStatus)
		require.Empty(t, f.events)
	}
}

func TestAvailableModelCatalog_EmptyAuthorizationAvoidsChannelAndRateReads(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.users.users[1].AllowedGroups = nil
	got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Empty(t, got.Groups)
	require.NotNil(t, got.Groups)
	require.Equal(t, "not_requested", got.UserRateStatus)
	require.Equal(t, []string{"user", "groups", "subscriptions"}, f.events)
}

func TestAvailableModelCatalog_AuthorizationAndCrossUserIsolation(t *testing.T) {
	f := catalogFixtureForHTTP()
	for _, tc := range []struct {
		userID  int64
		allowed bool
		want    []int64
	}{
		{1, false, []int64{1}}, {2, false, []int64{2}}, {1, true, []int64{1}},
	} {
		if tc.allowed {
			f.users.users[1].RestrictPublicGroups = false
		}
		got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog&user_id=2", tc.userID))
		ids := make([]int64, 0)
		for _, g := range got.Groups {
			ids = append(ids, g.ID)
		}
		require.Equal(t, tc.want, ids)
		if tc.userID == 1 {
			require.NotContains(t, string(mustCatalogJSON(t, got)), "hidden-")
			require.Equal(t, catalogFloat(0), got.Groups[0].UserRateMultiplier)
		}
		if tc.userID == 2 {
			require.Equal(t, catalogFloat(0.25), got.Groups[0].UserRateMultiplier)
		}
	}
}

func TestAvailableModelCatalog_SubscriptionsUseAuthorizationOwner(t *testing.T) {
	f := catalogFixtureForHTTP()
	now := time.Now()
	for id := int64(3); id <= 7; id++ {
		f.groups.groups = append(f.groups.groups, service.Group{ID: id, Name: "subscription", Platform: service.PlatformOpenAI, Status: service.StatusActive, IsExclusive: true, SubscriptionType: service.SubscriptionTypeSubscription})
	}
	f.groups.groups[6].Status = "inactive"
	f.users.users[1].AllowedGroups = []int64{1, 3, 4, 5, 6, 7}
	f.subs.subs = []service.UserSubscription{
		{UserID: 1, GroupID: 3, Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour)},
		{UserID: 1, GroupID: 4, Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(-time.Hour)},
		{UserID: 1, GroupID: 5, Status: "expired", ExpiresAt: now.Add(time.Hour)},
		{UserID: 2, GroupID: 6, Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour)},
		{UserID: 1, GroupID: 7, Status: service.SubscriptionStatusActive, ExpiresAt: now.Add(time.Hour)},
	}
	got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Len(t, got.Groups, 2)
	require.Equal(t, int64(3), got.Groups[1].ID)
	require.Empty(t, got.Groups[1].Models)
}

func TestAvailableModelCatalog_EmptyGroupsAndConfiguredPlatforms(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.users.users[1].RestrictPublicGroups = false
	f.groups.groups = append(f.groups.groups,
		service.Group{ID: 3, Name: "no binding", Platform: service.PlatformOpenAI, Status: service.StatusActive, SortOrder: -2},
		service.Group{ID: 4, Name: "inactive channel", Platform: service.PlatformOpenAI, Status: service.StatusActive},
		service.Group{ID: 5, Name: "filtered", Platform: service.PlatformOpenAI, Status: service.StatusActive, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"no-match"}}},
		service.Group{ID: 6, Name: "composite", Platform: service.PlatformComposite, Status: service.StatusActive},
		service.Group{ID: 7, Name: "empty composite", Platform: service.PlatformComposite, Status: service.StatusActive},
	)
	f.channels.channels[0].GroupIDs = append(f.channels.channels[0].GroupIDs, 5, 6)
	f.channels.channels = append(f.channels.channels,
		service.Channel{ID: 734, Name: "disabled-source", Status: "inactive", GroupIDs: []int64{4}, ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {"disabled-model": "disabled-model"}}},
		service.Channel{ID: 737, Name: "empty-source", Status: service.StatusActive, GroupIDs: []int64{7}},
	)
	got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Len(t, got.Groups, 6)
	require.Equal(t, int64(3), got.Groups[0].ID)
	for _, g := range got.Groups {
		require.NotNil(t, g.Models)
		switch g.ID {
		case 3, 4, 5, 7:
			require.Empty(t, g.Models)
		case 1:
			for _, m := range g.Models {
				require.Equal(t, service.PlatformOpenAI, m.Platform)
			}
		case 6:
			require.Equal(t, service.PlatformAnthropic, g.Models[0].Platform)
			require.Len(t, g.Models, 4)
		}
	}
	require.NotContains(t, string(mustCatalogJSON(t, got)), "disabled-source")
	require.Equal(t, []string{"user", "groups", "subscriptions", "channels", "rates"}, f.events)
}

func TestAvailableModelCatalog_ErrorsAreExplicitAndRateFailureIsReference(t *testing.T) {
	for _, failure := range []string{"user", "groups", "subscriptions", "channels", "rates"} {
		t.Run(failure, func(t *testing.T) {
			f := catalogFixtureForHTTP()
			err := errors.New("fixture repository unavailable")
			switch failure {
			case "user":
				f.users.err = err
			case "groups":
				f.groups.err = err
			case "subscriptions":
				f.subs.err = err
			case "channels":
				f.channels.err = err
			case "rates":
				f.rates.err = err
			}
			w := catalogRequest(t, f.handler, "?view=catalog", 1)
			if failure == "rates" {
				got := readCatalog(t, w)
				require.Equal(t, "unavailable", got.UserRateStatus)
				require.Nil(t, got.Groups[0].UserRateMultiplier)
				require.True(t, got.Groups[0].RateMultipliers.ReferenceOnly)
				require.Equal(t, 2.0, got.Groups[0].RateMultipliers.Token)
			} else {
				require.Equal(t, http.StatusInternalServerError, w.Code)
				require.NotContains(t, w.Body.String(), `"data"`)
				require.NotContains(t, f.events, "rates")
			}
		})
	}
}

func TestAvailableModelCatalog_ZeroUnknownRatesAndSingleSnapshot(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.groups.groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"Alias", "unpriced-private-model"}}
	f.rates.afterRead = func() {
		f.channels.channels[0].Name = "changed-source"
		f.channels.channels[0].ModelMapping[service.PlatformOpenAI]["Alias"] = "unpriced-private-model"
	}
	got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Equal(t, "loaded", got.UserRateStatus)
	g := got.Groups[0]
	require.Equal(t, 0.0, *g.UserRateMultiplier)
	require.Equal(t, 0.0, g.RateMultipliers.Token)
	require.False(t, g.RateMultipliers.ReferenceOnly)
	require.Len(t, g.Models, 2)
	zero, unknown := g.Models[0], g.Models[1]
	require.Equal(t, "Alias", zero.Name)
	require.Equal(t, "visible-source", zero.Source.Name)
	require.Equal(t, "resolved", zero.PriceStatus)
	require.Nil(t, zero.PriceReason)
	require.Equal(t, "USD/token", zero.BillingUnit)
	require.Equal(t, 0.0, *zero.Pricing.ServiceTiers[0].Context.Intervals[0].InputPrice)
	require.Nil(t, zero.Pricing.ServiceTiers[0].Context.Intervals[0].CacheReadPrice)
	require.Equal(t, "unknown", unknown.PriceStatus)
	require.Equal(t, "pricing_unavailable", *unknown.PriceReason)
	require.Nil(t, unknown.Pricing)
	require.Equal(t, g.RateMultipliers.PricingAt, zero.Pricing.ReferenceAt)
	require.Equal(t, []string{"user", "groups", "subscriptions", "channels", "rates"}, f.events)
}

func TestAvailableModelCatalog_PersonalRateAbsenceAndUnsupportedUnits(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.rates.rates[1] = nil
	f.channels.channels[0].ModelMapping = nil
	f.channels.channels[0].ModelPricing = []service.ChannelModelPricing{
		{Platform: service.PlatformOpenAI, Models: []string{"audio-private"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: catalogFloat(0.5)},
		{Platform: service.PlatformOpenAI, Models: []string{"image-private"}, BillingMode: service.BillingModeImage, PerRequestPrice: catalogFloat(0.2)},
	}
	got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Equal(t, "loaded", got.UserRateStatus)
	require.Nil(t, got.Groups[0].UserRateMultiplier)
	for _, offer := range got.Groups[0].Models {
		require.Equal(t, "unknown", offer.PriceStatus)
		require.Equal(t, "unsupported_unit", *offer.PriceReason)
		require.Equal(t, "unknown", offer.BillingUnit)
		require.Nil(t, offer.Pricing)
	}
	f.channels.channels[0].BillingModelSource = service.BillingModelSourceResponse
	got = readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	for _, offer := range got.Groups[0].Models {
		require.Nil(t, offer.BillingMode)
		require.Equal(t, "request_dependent", *offer.PriceReason)
	}
}

func TestAvailableModelCatalog_OfferIdentityAndPerGroupPrices(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.users.users[1].AllowedGroups = []int64{1, 2}
	f.channels.channels[0].GroupIDs = []int64{1, 2}
	f.channels.channels = f.channels.channels[:1]
	f.groups.groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"Alias"}}
	f.groups.groups[1].ModelAllowlist = f.groups.groups[0].ModelAllowlist
	f.groups.groups[1].ModelPricing = []service.ChannelModelPricing{{Models: []string{"internal-target"}, InputPrice: catalogFloat(3e-6)}}
	first := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Len(t, first.Groups, 2)
	require.Len(t, first.Groups[0].Models, 1)
	require.Len(t, first.Groups[1].Models, 1)
	require.NotEqual(t, first.Groups[0].Models[0].OfferKey, first.Groups[1].Models[0].OfferKey)
	require.InDelta(t, 3e-6, *first.Groups[1].Models[0].Pricing.ServiceTiers[0].Context.Intervals[0].InputPrice, 1e-14)
	key := first.Groups[0].Models[0].OfferKey
	f.groups.groups[0].Name = "renamed-group"
	f.groups.groups[0].SortOrder = 9
	f.channels.channels[0].Name = "renamed-source"
	f.channels.channels[0].Description = "renamed description"
	f.channels.channels[0].ModelPricing[0].InputPrice = catalogFloat(2e-6)
	delete(f.channels.channels[0].ModelMapping[service.PlatformOpenAI], "Alias")
	f.channels.channels[0].ModelMapping[service.PlatformOpenAI]["ALIAS"] = "internal-target"
	second := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.Equal(t, key, second.Groups[1].Models[0].OfferKey)
	f.channels.channels[0].ID++
	third := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	require.NotEqual(t, key, third.Groups[1].Models[0].OfferKey)
}

func mustCatalogJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}
func requireCatalogKeys(t *testing.T, value map[string]any, keys ...string) {
	t.Helper()
	actual := make([]string, 0, len(value))
	for k := range value {
		actual = append(actual, k)
	}
	require.ElementsMatch(t, keys, actual)
}
func TestAvailableModelCatalog_DTOWhitelistAndNullableRules(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.groups.groups[0].ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"Alias"}}
	got := readCatalog(t, catalogRequest(t, f.handler, "?view=catalog", 1))
	raw := mustCatalogJSON(t, got)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	requireCatalogKeys(t, decoded, "groups", "user_rate_status")
	group := decoded["groups"].([]any)[0].(map[string]any)
	requireCatalogKeys(t, group, "id", "name", "description", "platform", "subscription_type", "is_exclusive", "rate_multiplier", "user_rate_multiplier", "peak_rate_enabled", "peak_start", "peak_end", "peak_rate_multiplier", "long_context_pricing_enabled", "image_rate_independent", "image_rate_multiplier", "video_rate_independent", "video_rate_multiplier", "rate_multipliers", "models")
	offer := group["models"].([]any)[0].(map[string]any)
	requireCatalogKeys(t, offer, "offer_key", "name", "platform", "source", "billing_mode", "billing_unit", "price_status", "price_reason", "pricing")
	requireCatalogKeys(t, offer["source"].(map[string]any), "name", "description")
	pricing := offer["pricing"].(map[string]any)
	requireCatalogKeys(t, pricing, "reference_at", "reference_only", "service_tiers", "request_pricing", "time_pricing", "reasoning_effort_multipliers", "unsupported_components")
	tier := pricing["service_tiers"].([]any)[0].(map[string]any)
	requireCatalogKeys(t, tier, "service_tier", "context")
	schedule := tier["context"].(map[string]any)
	requireCatalogKeys(t, schedule, "basis", "intervals")
	interval := schedule["intervals"].([]any)[0].(map[string]any)
	requireCatalogKeys(t, interval, "min_tokens", "max_tokens", "tier_label", "input_price", "output_price", "cache_write_price", "cache_write_1h_price", "cache_read_price")
	for _, forbidden := range []string{"internal-target", "account_ids", "channel_id", "pricing_id", "sort_order", "model_mapping", "billing_model_source", "restrict_models", "upstream_url", "api_key", "hidden-source"} {
		require.NotContains(t, string(raw), forbidden)
	}
	require.NotNil(t, pricing["unsupported_components"])
}

func TestAvailableModelCatalog_RequestFallbackAndEmptyDTOArrays(t *testing.T) {
	group := &service.Group{ID: 1, Status: service.StatusActive, Platform: service.PlatformOpenAI}
	ch := &service.Channel{ID: 2, Status: service.StatusActive, GroupIDs: []int64{1}, BillingModelSource: service.BillingModelSourceRequested, ModelPricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{"request-private"}, BillingMode: service.BillingModePerRequest, PerRequestPrice: catalogFloat(0.25), Intervals: []service.PricingInterval{{MinTokens: 0, TierLabel: "small", PerRequestPrice: catalogFloat(0)}}}}}
	resolver, err := service.NewCatalogPricingResolver(service.NewBillingService(&config.Config{}, nil), group, ch)
	require.NoError(t, err)
	price, err := resolver.Resolve(context.Background(), service.CatalogPricingInput{Model: "request-private", Platform: service.PlatformOpenAI, UsageKind: "request"})
	require.NoError(t, err)
	dto := toCatalogPricing(price.Pricing)
	require.NotNil(t, dto.RequestPricing)
	require.Len(t, dto.RequestPricing.SizeTiers, 1)
	require.True(t, dto.RequestPricing.SizeTiers[0].FallsBackToContext)
	require.Nil(t, dto.RequestPricing.SizeTiers[0].Price)
	require.Equal(t, catalogFloat(0.25), dto.RequestPricing.DefaultPrice)
	require.NotNil(t, dto.ServiceTiers)
	empty := toCatalogPricing(&service.CatalogPricing{ServiceTiers: []service.CatalogServiceTierPricing{{ServiceTier: "default", Context: &service.ContextPricingSchedule{}}}, RequestPricing: &service.CatalogRequestPricing{}, TimePricing: &service.TimePricingSchedule{}})
	raw := string(mustCatalogJSON(t, empty))
	for _, fragment := range []string{`"intervals":[]`, `"context_tiers":[]`, `"size_tiers":[]`, `"periods":[]`, `"unsupported_components":[]`} {
		require.Contains(t, raw, fragment)
	}
}

func TestAvailableModelCatalog_ResolverFailureReturnsNoPartialCatalog(t *testing.T) {
	f := catalogFixtureForHTTP()
	f.handler.billingService = nil
	w := catalogRequest(t, f.handler, "?view=catalog", 1)
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.False(t, strings.Contains(w.Body.String(), `"data"`))
}
