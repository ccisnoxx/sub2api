//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func catalogPricingFixture(t *testing.T, g *Group, ch *Channel, p *PricingService) (*BillingService, *CatalogPricingResolver) {
	t.Helper()
	g.Status = StatusActive
	ch.Status = StatusActive
	ch.GroupIDs = []int64{g.ID}
	b := NewBillingService(&config.Config{}, p)
	r, err := NewCatalogPricingResolver(b, g, ch)
	require.NoError(t, err)
	return b, r
}

func TestCatalogPricing_SnapshotMappingAndPlatform(t *testing.T) {
	g := &Group{ID: 7, Platform: PlatformComposite, LongContextPricingEnabled: true}
	ch := &Channel{ID: 1, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"public": "gpt-5.6-luna-high"}, PlatformAnthropic: {"public": "claude-sonnet-4"}}, ModelPricing: []ChannelModelPricing{
		tokenPricingForModels([]string{"gpt-5.6-luna"}, 0.4), {Platform: PlatformAnthropic, Models: []string{"claude-sonnet-4"}, InputPrice: testPtrFloat64(9e-6)},
	}}
	_, r := catalogPricingFixture(t, g, ch, nil)
	ch.ModelMapping[PlatformOpenAI]["public"] = "unknown"
	ch.ModelPricing[0].Models[0] = "unknown"
	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAnthropic)
	for _, v := range []struct {
		platform string
		price    float64
	}{{PlatformOpenAI, 0.4e-6}, {PlatformAnthropic, 9e-6}} {
		price, err := r.Resolve(ctx, CatalogPricingInput{Model: "public", Platform: v.platform})
		require.NoError(t, err)
		require.Equal(t, "resolved", price.PriceStatus)
		requirePrice(t, testPtrFloat64(v.price), price.Pricing.ServiceTiers[0].Context.Tiers[0].Input, "input")
	}
}

func TestCatalogPricing_GroupZeroOverridesChannel(t *testing.T) {
	g := &Group{ID: 7, Platform: PlatformOpenAI, ModelPricing: []ChannelModelPricing{{Models: []string{"private"}, InputPrice: testPtrFloat64(0), OutputPrice: testPtrFloat64(0)}}}
	_, r := catalogPricingFixture(t, g, &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"private"}, InputPrice: testPtrFloat64(9e-6)}}}, nil)
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "private", Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, "resolved", price.PriceStatus)
	first := price.Pricing.ServiceTiers[0].Context.Tiers[0]
	requirePrice(t, testPtrFloat64(0), first.Input, "input")
	requirePrice(t, testPtrFloat64(0), first.Output, "output")
	require.Nil(t, first.CacheRead)
}

func TestCatalogPricing_GlobalExplicitZeroAndMissing(t *testing.T) {
	p := newStubPricingServiceFromJSON(t, `{"free-model":{"input_cost_per_token":0,"output_cost_per_token":0,"cache_read_input_token_cost":0}}`)
	_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1}, p)
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "free-model", Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, "resolved", price.PriceStatus)
	first := price.Pricing.ServiceTiers[0].Context.Tiers[0]
	requirePrice(t, testPtrFloat64(0), first.Input, "input")
	requirePrice(t, testPtrFloat64(0), first.Output, "output")
	requirePrice(t, testPtrFloat64(0), first.CacheRead, "cache_read")
	require.Nil(t, first.CacheWrite)
}

func TestCatalogPricing_UnknownAndRequestDependent(t *testing.T) {
	for _, source := range []string{BillingModelSourceRequested, BillingModelSourceUpstream, BillingModelSourceResponse} {
		t.Run(source, func(t *testing.T) {
			_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1, BillingModelSource: source}, nil)
			price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "unpriced-private-model", Platform: PlatformOpenAI})
			require.NoError(t, err)
			require.Equal(t, "unknown", price.PriceStatus)
			require.Nil(t, price.Pricing)
			reason := "request_dependent"
			if source == BillingModelSourceRequested {
				reason = "pricing_unavailable"
			}
			require.Equal(t, reason, price.PriceReason)
		})
	}
	_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformComposite}, &Channel{ID: 1}, nil)
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "all/claude-opus", Platform: PlatformAnthropic})
	require.NoError(t, err)
	require.Equal(t, "request_dependent", price.PriceReason)
	require.Nil(t, price.Pricing)
}

func TestCatalogPricing_EmptyCardDoesNotBecomeFree(t *testing.T) {
	_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"unpriced-private-model"}}}}, nil)
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "unpriced-private-model", Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, "pricing_unavailable", price.PriceReason)
	require.Nil(t, price.Pricing)
}

func TestCatalogPricing_TiersMatchBillingWithMultipliers(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 30, 0, 0, timezone.Location())
	g := &Group{ID: 7, Platform: PlatformOpenAI, RateMultiplier: 9, LongContextPricingEnabled: true, SubscriptionType: SubscriptionTypeSubscription, PeakRateEnabled: true, PeakStart: "12:00", PeakEnd: "13:00", PeakRateMultiplier: 3}
	ch := &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"gpt-5.4"}, InputPrice: testPtrFloat64(2e-6), FastMultiplier: testPtrFloat64(4), FlexMultiplier: testPtrFloat64(0.2), ReasoningEffortMultipliers: map[string]float64{"high": 1.5}, Intervals: []PricingInterval{{MinTokens: 0, MaxTokens: intPtr(100), InputMultiplier: testPtrFloat64(1)}, {MinTokens: 100, InputMultiplier: testPtrFloat64(2), OutputMultiplier: testPtrFloat64(1.5)}}, TimePricing: &ChannelTimePricing{Timezone: timezone.Location().String(), Periods: []ChannelTimePricingPeriod{{StartTime: "12:00", EndTime: "13:00", Multiplier: 2}}}}}}
	b, r := catalogPricingFixture(t, g, ch, openAILadderCatalog())
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "gpt-5.4", Platform: PlatformOpenAI, ReferenceAt: at})
	require.NoError(t, err)
	require.NotNil(t, price.Pricing.TimePricing)
	require.Equal(t, 1.5, price.Pricing.ReasoningEffortMultipliers["high"])
	rate := ResolveCatalogRateMultipliers(g, testPtrFloat64(0.5), true, at)
	require.Equal(t, 1.5, rate.Token)
	// 用生产 ChannelService 与独立解析器对账；不复用目录的快照来源作为期望值。
	cs := &ChannelService{}
	cs.cache.Store(populateChannelCache([]Channel{*ch}, map[int64]string{g.ID: g.Platform}))
	production := NewModelPricingResolver(cs, b)
	for _, tier := range price.Pricing.ServiceTiers {
		for _, n := range []int{1, 100, 101, 300} {
			for _, tokens := range []UsageTokens{{InputTokens: n, OutputTokens: 17}, {CacheReadTokens: n, OutputTokens: 17}, {CacheCreationTokens: n, CacheCreation1hTokens: n, OutputTokens: 17}} {
				gid := g.ID
				cost, err := b.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: "gpt-5.4", Group: g, GroupID: &gid, Tokens: tokens, ServiceTier: tier.ServiceTier, ReasoningEffort: "high", RateMultiplier: rate.Token, PricingAt: at, Resolver: production})
				require.NoError(t, err)
				p := tierAt(tier.Context.Tiers, n)
				expected := (float64(tokens.InputTokens)*catalogPriceOrZero(p.Input) + float64(tokens.CacheReadTokens)*catalogPriceOrZero(p.CacheRead) + float64(tokens.CacheCreationTokens)*catalogPriceOrZero(p.CacheWrite1h) + float64(tokens.OutputTokens)*catalogPriceOrZero(p.Output)) * rate.Token * 2 * 1.5
				require.InDelta(t, expected, cost.ActualCost, 1e-10, "%s/%d", tier.ServiceTier, n)
			}
		}
	}
}
func catalogPriceOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func TestCatalogPricing_LongContextAndFreeFast(t *testing.T) {
	g := &Group{ID: 7, Platform: PlatformOpenAI, FreeOpenAIFast: true}
	for _, enabled := range []bool{false, true} {
		g.LongContextPricingEnabled = enabled
		_, r := catalogPricingFixture(t, g, &Channel{ID: 1}, openAILadderCatalog())
		price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "gpt-5.4", Platform: PlatformOpenAI})
		require.NoError(t, err)
		count := 1
		if enabled {
			count = 2
		}
		require.Len(t, price.Pricing.ServiceTiers[0].Context.Tiers, count)
		require.Equal(t, price.Pricing.ServiceTiers[0].Context.Tiers, price.Pricing.ServiceTiers[1].Context.Tiers)
	}
}

func TestCatalogPricing_RequestZeroUsesRealFallback(t *testing.T) {
	for _, d := range []float64{0, 0.25} {
		_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"private"}, BillingMode: BillingModePerRequest, PerRequestPrice: testPtrFloat64(d), Intervals: []PricingInterval{{MinTokens: 0, MaxTokens: intPtr(100), TierLabel: "short", PerRequestPrice: testPtrFloat64(0)}, {MinTokens: 100, TierLabel: "long", PerRequestPrice: testPtrFloat64(0.5)}}}}}, nil)
		price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "private", Platform: PlatformOpenAI, UsageKind: "request"})
		require.NoError(t, err)
		require.Equal(t, "USD/request", price.BillingUnit)
		requirePrice(t, testPtrFloat64(d), price.Pricing.RequestPricing.ContextTiers[0].Price, "zero tier fallback")
		requirePrice(t, testPtrFloat64(0.5), price.Pricing.RequestPricing.SizeTiers[1].Price, "long tier")
		require.Nil(t, price.Pricing.TimePricing)
		price, err = r.Resolve(context.Background(), CatalogPricingInput{Model: "private", Platform: PlatformOpenAI})
		require.NoError(t, err)
		require.Equal(t, "unsupported_unit", price.PriceReason)
	}
}

func TestCatalogPricing_MediaUnitsUnsupported(t *testing.T) {
	for _, mode := range []BillingMode{BillingModeImage, BillingModeVideo} {
		_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"media"}, BillingMode: mode, PerRequestPrice: testPtrFloat64(1)}}}, nil)
		price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "media", Platform: PlatformOpenAI})
		require.NoError(t, err)
		require.Equal(t, "unknown", price.BillingUnit)
		require.Equal(t, "unsupported_unit", price.PriceReason)
		require.Nil(t, price.Pricing)
	}
}

func TestCatalogRateMultipliers_OverrideZeroPeakMediaAndFailure(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 30, 0, 0, timezone.Location())
	g := &Group{RateMultiplier: 8, SubscriptionType: SubscriptionTypeSubscription, PeakRateEnabled: true, PeakStart: "12:00", PeakEnd: "13:00", PeakRateMultiplier: 3, ImageRateIndependent: true, ImageRateMultiplier: 2, VideoRateIndependent: true, VideoRateMultiplier: 4}
	p := ResolveCatalogRateMultipliers(g, testPtrFloat64(0.5), true, at)
	require.Equal(t, 1.5, p.Token)
	require.Equal(t, 2.0, p.Image)
	require.Equal(t, 4.0, p.Video)
	require.False(t, p.ReferenceOnly)
	p = ResolveCatalogRateMultipliers(g, testPtrFloat64(0), true, at)
	require.Zero(t, p.Token)
	p = ResolveCatalogRateMultipliers(g, testPtrFloat64(0), false, at)
	require.Equal(t, 24.0, p.Token)
	require.True(t, p.ReferenceOnly)
	p = ResolveCatalogRateMultipliers(g, nil, true, at.Add(time.Hour))
	require.Equal(t, 8.0, p.Token)
}

func TestCatalogPricing_InvalidBindingAndCancellation(t *testing.T) {
	_, err := NewCatalogPricingResolver(newTestBillingService(), &Group{ID: 7, Status: StatusActive}, &Channel{Status: StatusActive, GroupIDs: []int64{8}})
	require.Error(t, err)
	_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1}, nil)
	_, err = r.Resolve(context.Background(), CatalogPricingInput{Model: "gpt-5.4", Platform: PlatformAnthropic})
	require.Error(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = r.Resolve(ctx, CatalogPricingInput{Model: "gpt-5.4", Platform: PlatformOpenAI})
	require.ErrorIs(t, err, context.Canceled)
}

func TestCatalogPricing_RequestOverlapsMatchActualBilling(t *testing.T) {
	g := &Group{ID: 7, Platform: PlatformOpenAI}
	ch := &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"private"}, BillingMode: BillingModePerRequest, PerRequestPrice: testPtrFloat64(0.3), Intervals: []PricingInterval{
		{MinTokens: 0, MaxTokens: intPtr(100), PerRequestPrice: testPtrFloat64(0.1)},
		{MinTokens: 0, MaxTokens: intPtr(1000), PerRequestPrice: testPtrFloat64(0.2)},
	}}}}
	b, r := catalogPricingFixture(t, g, ch, nil)
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "private", Platform: PlatformOpenAI, UsageKind: "request"})
	require.NoError(t, err)
	for _, n := range []int{1, 100, 101, 1000, 1001} {
		var displayed *float64
		for _, tier := range price.Pricing.RequestPricing.ContextTiers {
			if n > tier.MinTokens && (tier.MaxTokens == nil || n <= *tier.MaxTokens) {
				displayed = tier.Price
				break
			}
		}
		if displayed == nil {
			displayed = price.Pricing.RequestPricing.DefaultPrice
		}
		cost, err := b.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: "private", Group: g, GroupID: &g.ID, Tokens: UsageTokens{InputTokens: n}, RequestCount: 1, RateMultiplier: 1, Resolver: r.resolver})
		require.NoError(t, err)
		require.NotNil(t, displayed)
		require.InDelta(t, cost.ActualCost, *displayed, 1e-12, "context=%d", n)
	}
}

type catalogRefreshSource struct {
	pricing *PricingService
	next    map[string]*LiteLLMModelPricing
	calls   int
}

func (s *catalogRefreshSource) GetChannelModelPricing(context.Context, int64, string) *ChannelModelPricing {
	s.calls++
	if s.calls == 2 {
		s.pricing.mu.Lock()
		s.pricing.pricingData = s.next
		s.pricing.mu.Unlock()
	}
	return nil
}
func TestCatalogPricing_ZeroPresenceUsesResolvedVersion(t *testing.T) {
	for _, v := range []struct {
		name, old, next string
		expected        *float64
	}{
		{"missing_to_positive", `{"refresh-model":{"input_cost_per_token":1e-6}}`, `{"refresh-model":{"input_cost_per_token":1e-6,"output_cost_per_token":2e-6}}`, nil},
		{"zero_to_missing", `{"refresh-model":{"input_cost_per_token":1e-6,"output_cost_per_token":0}}`, `{"refresh-model":{"input_cost_per_token":1e-6}}`, testPtrFloat64(0)},
	} {
		t.Run(v.name, func(t *testing.T) {
			p := newStubPricingServiceFromJSON(t, v.old)
			next := newStubPricingServiceFromJSON(t, v.next)
			_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1}, p)
			source := &catalogRefreshSource{pricing: p, next: next.pricingData}
			r.resolver.channelPricingSource = source
			price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "refresh-model", Platform: PlatformOpenAI})
			require.NoError(t, err)
			require.GreaterOrEqual(t, source.calls, 2)
			requirePrice(t, v.expected, price.Pricing.ServiceTiers[0].Context.Tiers[0].Output, "resolved output")
		})
	}
}
func TestCatalogPricing_ResolvedReasonIsEmpty(t *testing.T) {
	_, r := catalogPricingFixture(t, &Group{ID: 7, Platform: PlatformOpenAI}, &Channel{ID: 1}, nil)
	price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "gpt-5.4", Platform: PlatformOpenAI})
	require.NoError(t, err)
	require.Equal(t, "resolved", price.PriceStatus)
	require.Empty(t, price.PriceReason)
	require.True(t, price.Pricing.ReferenceOnly)
}

func TestCatalogPricing_ZeroLabelFollowsContext(t *testing.T) {
	for _, v := range []struct {
		name         string
		defaultPrice float64
	}{{"zero_default", 0}, {"positive_default", 0.25}} {
		t.Run(v.name, func(t *testing.T) {
			g := &Group{ID: 7, Platform: PlatformOpenAI}
			ch := &Channel{ID: 1, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"private"}, BillingMode: BillingModePerRequest, PerRequestPrice: testPtrFloat64(v.defaultPrice), Intervals: []PricingInterval{
				{MinTokens: 0, MaxTokens: intPtr(100), TierLabel: "short", PerRequestPrice: testPtrFloat64(0)},
				{MinTokens: 100, TierLabel: "long", PerRequestPrice: testPtrFloat64(0.5)},
			}}}}
			b, r := catalogPricingFixture(t, g, ch, nil)
			price, err := r.Resolve(context.Background(), CatalogPricingInput{Model: "private", Platform: PlatformOpenAI, UsageKind: "request"})
			require.NoError(t, err)
			short := price.Pricing.RequestPricing.SizeTiers[0]
			for _, n := range []int{0, 1, 100, 101, 300} {
				displayed := short.Price
				if short.FallsBackToContext {
					displayed = price.Pricing.RequestPricing.DefaultPrice
					for _, tier := range price.Pricing.RequestPricing.ContextTiers {
						if n > tier.MinTokens && (tier.MaxTokens == nil || n <= *tier.MaxTokens) {
							displayed = tier.Price
							break
						}
					}
				}
				cost, err := b.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: "private", Group: g, GroupID: &g.ID, Tokens: UsageTokens{InputTokens: n}, RequestCount: 1, SizeTier: "short", RateMultiplier: 1, Resolver: r.resolver})
				require.NoError(t, err)
				require.NotNil(t, displayed)
				require.InDelta(t, cost.ActualCost, *displayed, 1e-12, "short/context=%d", n)
			}
			require.True(t, short.FallsBackToContext)
			require.Nil(t, short.Price)
			require.False(t, price.Pricing.RequestPricing.SizeTiers[1].FallsBackToContext)
		})
	}
}
