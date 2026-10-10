package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// 目录 DTO 只包含展示白名单；不序列化 Group、Channel 或领域价格结构。
type userModelCatalog struct {
	Groups         []userCatalogGroup `json:"groups"`
	UserRateStatus string             `json:"user_rate_status"`
}

type userCatalogGroup struct {
	userAvailableGroup
	Description               string                     `json:"description"`
	UserRateMultiplier        *float64                   `json:"user_rate_multiplier"`
	LongContextPricingEnabled bool                       `json:"long_context_pricing_enabled"`
	ImageRateIndependent      bool                       `json:"image_rate_independent"`
	ImageRateMultiplier       float64                    `json:"image_rate_multiplier"`
	VideoRateIndependent      bool                       `json:"video_rate_independent"`
	VideoRateMultiplier       float64                    `json:"video_rate_multiplier"`
	RateMultipliers           userCatalogRateMultipliers `json:"rate_multipliers"`
	Models                    []userCatalogOffer         `json:"models"`
}

type userCatalogRateMultipliers struct {
	Token         float64   `json:"token"`
	Image         float64   `json:"image"`
	Video         float64   `json:"video"`
	ReferenceOnly bool      `json:"reference_only"`
	PricingAt     time.Time `json:"pricing_at"`
	Timezone      string    `json:"timezone"`
}

type userCatalogSource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type userCatalogOffer struct {
	OfferKey    string              `json:"offer_key"`
	Name        string              `json:"name"`
	Platform    string              `json:"platform"`
	Source      userCatalogSource   `json:"source"`
	BillingMode *string             `json:"billing_mode"`
	BillingUnit string              `json:"billing_unit"`
	PriceStatus string              `json:"price_status"`
	PriceReason *string             `json:"price_reason"`
	Pricing     *userCatalogPricing `json:"pricing"`
}

type userCatalogPricing struct {
	ReferenceAt                time.Time                `json:"reference_at"`
	ReferenceOnly              bool                     `json:"reference_only"`
	ServiceTiers               []userCatalogServiceTier `json:"service_tiers"`
	RequestPricing             *userCatalogRequestPrice `json:"request_pricing"`
	TimePricing                *userCatalogTimePricing  `json:"time_pricing"`
	ReasoningEffortMultipliers map[string]float64       `json:"reasoning_effort_multipliers"`
	UnsupportedComponents      []string                 `json:"unsupported_components"`
}

type userCatalogServiceTier struct {
	ServiceTier string              `json:"service_tier"`
	Context     *userCatalogContext `json:"context"`
}

type userCatalogContext struct {
	Basis     string                  `json:"basis"`
	Intervals []userCatalogTokenPrice `json:"intervals"`
}

type userCatalogTokenPrice struct {
	MinTokens       int      `json:"min_tokens"`
	MaxTokens       *int     `json:"max_tokens"`
	TierLabel       string   `json:"tier_label"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
	CacheWrite1h    *float64 `json:"cache_write_1h_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
}

type userCatalogRequestPrice struct {
	DefaultPrice *float64                 `json:"default_price"`
	ContextTiers []userCatalogRequestTier `json:"context_tiers"`
	SizeTiers    []userCatalogRequestTier `json:"size_tiers"`
}

type userCatalogRequestTier struct {
	MinTokens          int      `json:"min_tokens"`
	MaxTokens          *int     `json:"max_tokens"`
	Label              string   `json:"label"`
	Price              *float64 `json:"price"`
	FallsBackToContext bool     `json:"falls_back_to_context"`
}

type userCatalogTimePricing struct {
	Timezone     string                  `json:"timezone"`
	WeekdaysOnly bool                    `json:"weekdays_only"`
	Periods      []userCatalogTimePeriod `json:"periods"`
}

type userCatalogTimePeriod struct {
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}

func emptyUserModelCatalog() userModelCatalog {
	return userModelCatalog{Groups: []userCatalogGroup{}, UserRateStatus: "not_requested"}
}

func isCatalogRequest(c *gin.Context) bool {
	values := c.Request.URL.Query()["view"]
	return len(values) == 1 && values[0] == "catalog"
}

func (h *AvailableChannelHandler) listCatalog(c *gin.Context, userID int64, groups []service.Group) {
	if len(groups) == 0 {
		response.Success(c, emptyUserModelCatalog())
		return
	}
	ctx := c.Request.Context()
	channels, err := h.channelService.ListCatalogChannels(ctx, groups)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	rates, rateErr := h.apiKeyService.GetUserGroupRates(ctx, userID)
	loaded := rateErr == nil
	// 只有个人倍率读取允许降级；授权、渠道及价格解析错误仍使整个请求失败。
	out, err := h.buildModelCatalog(ctx, groups, channels, rates, loaded, timezone.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, out)
}

func (h *AvailableChannelHandler) buildModelCatalog(ctx context.Context, groups []service.Group, channels []service.Channel, rates map[int64]float64, loaded bool, at time.Time) (userModelCatalog, error) {
	out := userModelCatalog{Groups: make([]userCatalogGroup, 0, len(groups)), UserRateStatus: "unavailable"}
	if loaded {
		out.UserRateStatus = "loaded"
	}
	// 授权 owner 返回活跃组的 sort_order/ID 顺序；按同一合同排序，不修改其切片。
	groups = slices.Clone(groups)
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})
	channelByGroup := make(map[int64]*service.Channel)
	for i := range channels {
		for _, groupID := range channels[i].GroupIDs {
			channelByGroup[groupID] = &channels[i]
		}
	}
	for i := range groups {
		g := &groups[i]
		var userRate *float64
		if rate, ok := rates[g.ID]; loaded && ok {
			userRate = &rate
		}
		r := service.ResolveCatalogRateMultipliers(g, userRate, loaded, at)
		row := userCatalogGroup{
			userAvailableGroup: userAvailableGroup{
				ID: g.ID, Name: g.Name, Platform: g.Platform, SubscriptionType: g.SubscriptionType,
				IsExclusive: g.IsExclusive, RateMultiplier: g.RateMultiplier,
				PeakRateEnabled: g.PeakRateEnabled, PeakStart: g.PeakStart, PeakEnd: g.PeakEnd, PeakRateMultiplier: g.PeakRateMultiplier,
			},
			Description: g.Description, UserRateMultiplier: userRate,
			LongContextPricingEnabled: g.LongContextPricingEnabled,
			ImageRateIndependent:      g.ImageRateIndependent, ImageRateMultiplier: g.ImageRateMultiplier,
			VideoRateIndependent: g.VideoRateIndependent, VideoRateMultiplier: g.VideoRateMultiplier,
			RateMultipliers: userCatalogRateMultipliers{Token: r.Token, Image: r.Image, Video: r.Video, ReferenceOnly: r.ReferenceOnly, PricingAt: r.PricingAt, Timezone: timezone.Location().String()},
			Models:          []userCatalogOffer{},
		}
		ch := channelByGroup[g.ID]
		models := ch.CatalogModelsForGroup(g)
		if len(models) > 0 {
			resolver, err := service.NewCatalogPricingResolver(h.billingService, g, ch)
			if err != nil {
				return userModelCatalog{}, err
			}
			for _, model := range models {
				// 目录没有实际媒体/按次入口事实，不凭 per_request 模式猜测单位。
				price, err := resolver.Resolve(ctx, service.CatalogPricingInput{Model: model.Name, Platform: model.Platform, ReferenceAt: at})
				if err != nil {
					return userModelCatalog{}, err
				}
				row.Models = append(row.Models, userCatalogOffer{
					OfferKey: catalogOfferKey(g.ID, ch.ID, model.Platform, model.Name),
					Name:     model.Name, Platform: model.Platform,
					Source:      userCatalogSource{Name: ch.Name, Description: ch.Description},
					BillingMode: catalogOptionalString(string(price.BillingMode)), BillingUnit: price.BillingUnit,
					PriceStatus: price.PriceStatus, PriceReason: catalogOptionalString(price.PriceReason),
					Pricing: toCatalogPricing(price.Pricing),
				})
			}
		}
		out.Groups = append(out.Groups, row)
	}
	return out, nil
}

// v1 固定以分组、绑定渠道、平台和大小写无关请求 ID 派生；不使用显示名或价格。
// SHA-256 隐去内部渠道 ID 的可读形式，key 仅用于渲染，不作为授权凭证。
func catalogOfferKey(groupID, channelID int64, platform, model string) string {
	identity := fmt.Sprintf("catalog-offer-v1\x00%d\x00%d\x00%s\x00%s", groupID, channelID, platform, strings.ToLower(model))
	return fmt.Sprintf("offer_%x", sha256.Sum256([]byte(identity)))
}

func catalogOptionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func toCatalogPricing(src *service.CatalogPricing) *userCatalogPricing {
	if src == nil {
		return nil
	}
	out := &userCatalogPricing{
		ReferenceAt: src.ReferenceAt, ReferenceOnly: src.ReferenceOnly,
		ServiceTiers: []userCatalogServiceTier{}, ReasoningEffortMultipliers: make(map[string]float64),
		UnsupportedComponents: append([]string{}, src.UnsupportedComponents...),
		TimePricing:           toCatalogTimePricing(src.TimePricing),
	}
	for effort, multiplier := range src.ReasoningEffortMultipliers {
		out.ReasoningEffortMultipliers[effort] = multiplier
	}
	for _, tier := range src.ServiceTiers {
		row := userCatalogServiceTier{ServiceTier: tier.ServiceTier}
		if tier.Context != nil {
			row.Context = &userCatalogContext{Basis: string(tier.Context.Basis), Intervals: []userCatalogTokenPrice{}}
			for _, interval := range tier.Context.Tiers {
				row.Context.Intervals = append(row.Context.Intervals, userCatalogTokenPrice{
					MinTokens: interval.MinTokens, MaxTokens: interval.MaxTokens, TierLabel: interval.Label,
					InputPrice: interval.Input, OutputPrice: interval.Output, CacheWritePrice: interval.CacheWrite,
					CacheWrite1h: interval.CacheWrite1h, CacheReadPrice: interval.CacheRead,
				})
			}
		}
		out.ServiceTiers = append(out.ServiceTiers, row)
	}
	if src.RequestPricing != nil {
		out.RequestPricing = &userCatalogRequestPrice{
			DefaultPrice: src.RequestPricing.DefaultPrice,
			ContextTiers: toCatalogRequestTiers(src.RequestPricing.ContextTiers),
			SizeTiers:    toCatalogRequestTiers(src.RequestPricing.SizeTiers),
		}
	}
	return out
}

func toCatalogRequestTiers(src []service.CatalogRequestTier) []userCatalogRequestTier {
	out := make([]userCatalogRequestTier, 0, len(src))
	for _, tier := range src {
		out = append(out, userCatalogRequestTier{MinTokens: tier.MinTokens, MaxTokens: tier.MaxTokens, Label: tier.Label, Price: tier.Price, FallsBackToContext: tier.FallsBackToContext})
	}
	return out
}

func toCatalogTimePricing(src *service.TimePricingSchedule) *userCatalogTimePricing {
	if src == nil {
		return nil
	}
	out := &userCatalogTimePricing{Timezone: src.Timezone, WeekdaysOnly: src.WeekdaysOnly, Periods: []userCatalogTimePeriod{}}
	for _, period := range src.Periods {
		out.Periods = append(out.Periods, userCatalogTimePeriod{StartTime: period.StartTime, EndTime: period.EndTime, Multiplier: period.Multiplier})
	}
	return out
}
