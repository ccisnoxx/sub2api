package service

import (
	"context"
	"errors"
	"math"
	"slices"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// CatalogPricingInput 的单位须由实际请求入口确认；仅 token 模式可省略 UsageKind。
// ReferenceAt 固定参考计费时点，不代表将来请求的价格。
type CatalogPricingInput struct {
	Model       string
	Platform    string
	UsageKind   string // token / request；媒体单位尚未接入时保持 unknown
	ReferenceAt time.Time
}

// CatalogPrice 是 S2.3 DTO 的只读输入，不能直接序列化领域对象。
// resolved 代表已有可解释的价格规则，不代表一个未发生请求的精确个人账单。
type CatalogPrice struct {
	BillingMode BillingMode
	BillingUnit string // USD/token、USD/request、unknown
	PriceStatus string // resolved / unknown
	PriceReason string // 空 / pricing_unavailable / request_dependent / unsupported_unit
	Pricing     *CatalogPricing
}

type CatalogPricing struct {
	ReferenceAt                time.Time
	ReferenceOnly              bool
	ServiceTiers               []CatalogServiceTierPricing
	RequestPricing             *CatalogRequestPricing
	TimePricing                *TimePricingSchedule
	ReasoningEffortMultipliers map[string]float64
	// 当前上下文表覆盖文本/缓存 token；图片 token 拆分不冒充文本价格。
	UnsupportedComponents []string
}

type CatalogServiceTierPricing struct {
	ServiceTier string
	Context     *ContextPricingSchedule
}

// CatalogRequestPricing 的价格均通过实际按次计费入口求值。
// 明确零档命中后的 fallback 也按现有扣费行为呈现，不照抄原配置当作最终价。
type CatalogRequestPricing struct {
	DefaultPrice *float64
	ContextTiers []CatalogRequestTier
	SizeTiers    []CatalogRequestTier
}

type CatalogRequestTier struct {
	MinTokens int
	MaxTokens *int
	Label     string
	Price     *float64
	// 标签单价为零时，计费 owner 继续按真实上下文和默认价求值。
	FallsBackToContext bool
}

// CatalogPricingResolver 只处理一个已授权分组及其绑定渠道的同次读取快照。
// 授权、active 过滤、模型枚举与白名单由 S2.3 调用方先完成；此处不扩大模型集合。
// 不读数据库/热缓存，不触发上游、调度、用量记录或结算。
type CatalogPricingResolver struct {
	billing  *BillingService
	group    *Group
	channel  *Channel
	cache    *channelCache
	resolver *ModelPricingResolver
}

var errCatalogPricingInput = errors.New("目录价格解析需要已授权的活跃分组及其绑定渠道")

func NewCatalogPricingResolver(billing *BillingService, group *Group, channel *Channel) (*CatalogPricingResolver, error) {
	if billing == nil || group == nil || channel == nil || group.ID <= 0 ||
		group.Status != StatusActive || !channel.IsActive() || !slices.Contains(channel.GroupIDs, group.ID) {
		return nil, errCatalogPricingInput
	}
	g := *group
	g.ModelPricing = make([]ChannelModelPricing, len(group.ModelPricing))
	for i := range group.ModelPricing {
		g.ModelPricing[i] = group.ModelPricing[i].Clone()
	}
	ch := channel.Clone()
	cache := populateChannelCache([]Channel{*ch}, map[int64]string{g.ID: g.Platform})
	r := &CatalogPricingResolver{billing: billing, group: &g, channel: cache.channelByGroupID[g.ID], cache: cache}
	r.resolver = &ModelPricingResolver{channelPricingSource: r, billingService: billing}
	return r, nil
}

// GetChannelModelPricing 实现统一解析器的渠道来源，复用缓存索引规则但不加载共享缓存。
func (r *CatalogPricingResolver) GetChannelModelPricing(ctx context.Context, groupID int64, model string) *ChannelModelPricing {
	if groupID != r.group.ID {
		return nil
	}
	pricing := lookupPricingAcrossPlatforms(r.cache, groupID, channelLookupPlatform(ctx, r.group.Platform), model)
	if pricing == nil {
		return nil
	}
	cp := pricing.Clone()
	return &cp
}

func unknownCatalogPrice(mode BillingMode, reason string) *CatalogPrice {
	return &CatalogPrice{BillingMode: mode, BillingUnit: "unknown", PriceStatus: "unknown", PriceReason: reason}
}

func (r *CatalogPricingResolver) Resolve(ctx context.Context, in CatalogPricingInput) (*CatalogPrice, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !isConcreteRequestPlatform(in.Platform) || (r.group.Platform != PlatformComposite && r.group.Platform != in.Platform) {
		return nil, errCatalogPricingInput
	}
	// 目录的具体平台来自已过滤的模型；不继承调用上下文的调度平台覆盖。
	ctx = context.WithValue(ctx, ctxkey.ForcePlatform, in.Platform)
	mapping := resolveMapping(&channelLookup{cache: r.cache, channel: r.channel, platform: in.Platform}, r.group.ID, in.Model)
	model := in.Model
	switch mapping.BillingModelSource {
	case BillingModelSourceRequested:
	case BillingModelSourceChannelMapped:
		model = mapping.MappedModel
	default:
		// 上游/response_model 来源需要最终账号或真实响应，目录没有这些事实。
		return unknownCatalogPrice("", "request_dependent"), nil
	}
	gid := r.group.ID
	resolved := r.resolver.Resolve(ctx, PricingInput{Model: model, Group: r.group, GroupID: &gid})
	if r.group.Platform == PlatformComposite && resolved.Source != PricingSourceChannel && resolved.Source != PricingSourceGroup {
		// composite 无显式价卡时按最终具体模型计费，公开别名的家族兜底不能当权威报价。
		return unknownCatalogPrice(resolved.Mode, "request_dependent"), nil
	}
	at := in.ReferenceAt
	if at.IsZero() {
		at = timezone.Now()
	}
	pricing := &CatalogPricing{
		ReferenceAt: at, ReferenceOnly: true,
		ServiceTiers:               make([]CatalogServiceTierPricing, 0),
		UnsupportedComponents:      []string{},
		TimePricing:                resolvedTimePricingSchedule(resolved),
		ReasoningEffortMultipliers: make(map[string]float64),
	}
	// 渠道分时倍率作为独立规则输出，基础单价不预乘该倍率，避免展示端重复乘。
	base := *resolved
	if resolved.channelPricing != nil {
		cp := resolved.channelPricing.Clone()
		cp.TimePricing = nil
		base.channelPricing = &cp
		for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
			if multiplier := reasoningEffortBillingMultiplier(effort, cp.ReasoningEffortMultipliers); multiplier != 1 {
				pricing.ReasoningEffortMultipliers[effort] = multiplier
			}
		}
	}
	switch resolved.Mode {
	case "", BillingModeToken:
		if in.UsageKind != "" && in.UsageKind != "token" {
			return unknownCatalogPrice(resolved.Mode, "unsupported_unit"), nil
		}
		pricing.UnsupportedComponents = []string{"image_input_token", "image_output_token", "image_cache_read_token", "audio"}
		tiers := []string{"default"}
		switch in.Platform {
		case PlatformOpenAI:
			tiers = append(tiers, "priority", "flex", OpenAIFastTierUltrafast)
		case PlatformAnthropic:
			tiers = append(tiers, "fast")
		}
		for _, tier := range tiers {
			billTier := tier
			if groupBillsOpenAIFastAtStandard(&APIKey{Group: r.group}, &Account{Platform: in.Platform}, tier) {
				billTier = "default"
			}
			schedule, err := r.billing.resolveTokenPricingSchedule(TokenCostRequest{
				Ctx: ctx, Model: model, Group: r.group, RateMultiplier: 1,
				PricingAt: at, ServiceTier: billTier, Resolver: r.resolver, Resolved: &base,
			})
			if errors.Is(err, ErrModelPricingUnavailable) {
				return unknownCatalogPrice(BillingModeToken, "pricing_unavailable"), nil
			}
			if err != nil {
				return nil, err
			}
			preserveCatalogZeroPrices(&base, schedule)
			if !catalogScheduleHasPrices(schedule) {
				return unknownCatalogPrice(BillingModeToken, "pricing_unavailable"), nil
			}
			pricing.ServiceTiers = append(pricing.ServiceTiers, CatalogServiceTierPricing{ServiceTier: tier, Context: schedule})
		}
		return &CatalogPrice{BillingMode: BillingModeToken, BillingUnit: "USD/token", PriceStatus: "resolved", Pricing: pricing}, nil
	case BillingModePerRequest:
		// 同一 per_request 配置也用于音频分钟/小时/字符，不能单凭模式猜单位。
		if in.UsageKind != "request" {
			return unknownCatalogPrice(resolved.Mode, "unsupported_unit"), nil
		}
		pricing.RequestPricing = r.requestPricing(ctx, model, &base)
		if pricing.RequestPricing == nil {
			return unknownCatalogPrice(resolved.Mode, "pricing_unavailable"), nil
		}
		pricing.TimePricing = nil // 现有按次 owner 不应用渠道分时倍率。
		return &CatalogPrice{BillingMode: resolved.Mode, BillingUnit: "USD/request", PriceStatus: "resolved", Pricing: pricing}, nil
	default:
		// image/video 的单位与价格优先级还取决于实际媒体入口、大小或时长。
		return unknownCatalogPrice(resolved.Mode, "unsupported_unit"), nil
	}
}

func preserveCatalogZeroPrices(resolved *ResolvedPricing, schedule *ContextPricingSchedule) {
	if resolved.BasePricing == nil {
		return
	}
	p := resolved.BasePricing.sourcePricePresence
	for i := range schedule.Tiers {
		t := &schedule.Tiers[i]
		for _, field := range []struct {
			price    **float64
			explicit bool
		}{
			{&t.Input, p.input}, {&t.Output, p.output}, {&t.CacheRead, p.cacheRead},
			{&t.CacheWrite, p.cacheWrite}, {&t.CacheWrite1h, p.cacheWrite1h},
		} {
			if *field.price == nil && field.explicit {
				zero := 0.0
				*field.price = &zero
			}
		}
	}
}

func catalogScheduleHasPrices(schedule *ContextPricingSchedule) bool {
	for _, tier := range schedule.Tiers {
		for _, p := range []*float64{tier.Input, tier.Output, tier.CacheWrite, tier.CacheWrite1h, tier.CacheRead} {
			if p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0) && *p >= 0 {
				return true
			}
		}
	}
	return false
}

func (r *CatalogPricingResolver) requestPricing(ctx context.Context, model string, resolved *ResolvedPricing) *CatalogRequestPricing {
	cp := resolved.channelPricing
	if cp == nil {
		return nil
	}
	out := &CatalogRequestPricing{ContextTiers: []CatalogRequestTier{}, SizeTiers: []CatalogRequestTier{}}
	hasPrice := cp.PerRequestPrice != nil
	for _, tier := range resolved.RequestTiers {
		hasPrice = hasPrice || tier.PerRequestPrice != nil
	}
	if !hasPrice {
		return nil
	}
	probe := func(contextTokens int, label string) *float64 {
		cost, err := r.billing.CalculateCostUnified(CostInput{
			Ctx: ctx, Model: model, Group: r.group, Tokens: UsageTokens{InputTokens: contextTokens},
			RequestCount: 1, SizeTier: label, RateMultiplier: 1, Resolver: r.resolver, Resolved: resolved,
		})
		if err != nil || cost == nil {
			return nil
		}
		v := cost.TotalCost
		return &v
	}
	if cp.PerRequestPrice != nil {
		out.DefaultPrice = probe(0, "")
	}
	if len(resolved.RequestTiers) > 0 {
		// per_request 允许重叠。复用所有边界切段，每段仍由 owner 首次命中求价。
		plan := r.billing.contextPricingBreakpoints(r.resolver, &ResolvedPricing{
			Intervals: resolved.RequestTiers, longContextPricingEnabled: true,
		}, model)
		for _, seg := range buildContextSegments(plan.bounds) {
			matched := FindMatchingInterval(resolved.RequestTiers, seg.min+1)
			var price *float64
			if cp.PerRequestPrice != nil || (matched != nil && matched.PerRequestPrice != nil) {
				price = probe(seg.min+1, "")
			}
			next := CatalogRequestTier{MinTokens: seg.min, MaxTokens: seg.max, Price: price}
			if n := len(out.ContextTiers); n > 0 && samePricePtr(out.ContextTiers[n-1].Price, next.Price) {
				out.ContextTiers[n-1].MaxTokens = next.MaxTokens
			} else {
				out.ContextTiers = append(out.ContextTiers, next)
			}
		}
	}
	for _, tier := range resolved.RequestTiers {
		if tier.PerRequestPrice == nil {
			continue
		}
		if tier.TierLabel != "" {
			entry := CatalogRequestTier{Label: tier.TierLabel}
			if r.resolver.GetRequestTierPrice(resolved, tier.TierLabel) == 0 {
				entry.FallsBackToContext = true
			} else {
				entry.Price = probe(0, tier.TierLabel)
			}
			out.SizeTiers = append(out.SizeTiers, entry)
		}
	}
	if out.DefaultPrice == nil && len(out.ContextTiers) == 0 && len(out.SizeTiers) == 0 {
		return nil
	}
	return out
}

// CatalogRateMultipliers 与单价分离。个人倍率覆盖分组默认值；只在最后应用一次。
// 读取失败时忽略可能残留的用户值，ReferenceOnly=true 明确默认倍率只是参考。
type CatalogRateMultipliers struct {
	Token, Image, Video float64
	ReferenceOnly       bool
	PricingAt           time.Time
}

func ResolveCatalogRateMultipliers(group *Group, userRate *float64, loaded bool, at time.Time) CatalogRateMultipliers {
	if at.IsZero() {
		at = timezone.Now()
	}
	base := group.RateMultiplier
	if loaded && userRate != nil {
		base = *userRate
	}
	key := &APIKey{Group: group}
	text, image := computePeakAwareMultipliers(key, base, at)
	return CatalogRateMultipliers{Token: math.Max(0, text), Image: math.Max(0, image), Video: math.Max(0, resolveVideoRateMultiplier(key, base)), ReferenceOnly: !loaded, PricingAt: at}
}
