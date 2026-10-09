# S2.2 KIN 权威价格解析合同

- 日期：2026-10-09（America/Los_Angeles）。
- 范围：只读价格解析与实际计费 owner 对账；权限、聚合 DTO、HTTP 分支和页面仍由 S2.3–S2.6 实现。
- 应用基础：personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92` → S1 候选 `e3edb5666a03a582bfbb83a718aedda03ba6ea08`；来源 `.5/c7aacf5d` 不变。
- 对应：[权限合同](catalog-contract.md)、[任务清单](tasks.md)、[执行证据](implementation-evidence.md)。

## 解析入口与责任

`NewCatalogPricingResolver(billing, group, channel)` 为单个已授权的活跃分组及其绑定活跃渠道创建只读快照；`Resolve(ctx, CatalogPricingInput)` 只消费该快照。构造检查不能代替用户授权。S2.3 必须先执行 GetAvailableGroups、active/平台过滤、SupportedModels 和 Group.ModelAllowlist.Allows，再调用此入口；不得使用它扩增模型列表。

目录复用 `populateChannelCache` 的平台索引、精确/通配与映射规则，以及 ModelPricingResolver 的分组卡 → 渠道 → LiteLLM/内置兜底链。计费模型采用渠道配置的 requested/channel_mapped；不输出映射目标。upstream/response_model 需要最终账号或响应事实，返回 request_dependent。Composite 没有显式分组/渠道价卡时，实际计费可能回退具体模型，目录不将公开别名的家族兜底冒充实际报价。

渠道快照不会再次加载共享缓存或数据库。单价来自真实 `CalculateTokenCostForRequest`/`CalculateCostUnified` 的探针；不调用结算、调度或上游。旧 resolver 构造入口仍使用原 ChannelService；旧模型广场仍使用原参数和行为。

## 价格状态与单位

| 情况 | 结果 |
|---|---|
| 文本/缓存 Token 规则可解析 | resolved、PriceReason 为空（DTO 映射 null）、USD/token；完整上下文阶梯和服务档位规则；ReferenceOnly=true |
| 确认是按请求次数的 per_request 入口 | 调用方显式 UsageKind=request；resolved、PriceReason 为空（DTO 映射 null）、USD/request；默认价、上下文与标签档位 |
| 模型查无价或空价卡没有任何有效价格 | unknown、pricing_unavailable、Pricing=nil |
| 最终模型依赖上游/响应或 Composite 具体路由 | unknown、request_dependent、Pricing=nil |
| image/video、未知媒体单位、per_request 未确认次数单位 | unknown、unsupported_unit、BillingUnit=unknown、Pricing=nil |

省略 UsageKind 仅在 token 模式表示文本/缓存 Token 规则。per_request 也被 KIN 用于音频分钟、小时或字符单位，不能仅凭模式推断“每次”。媒体列由 UnsupportedComponents 明确列出：image_input_token/image_output_token/image_cache_read_token/audio。图片与视频独立倍率已接入，但不据此声称媒体单价已支持。

明确数值零价保留 0；缺失价格保留 nil。分组/渠道字段的存在性沿用指针；LiteLLM 解析新增不序列化的字段存在标记，保留输入、输出、缓存读取、1h 写入的来源事实，不改变计费值。这些来源事实随同一次读取生成的 ModelPricing 一起复制；零价恢复不重新读取动态价格表，后台刷新不会混用两个版本的价格与存在性。模型无价不能被转换为免费。按次上下文档位的零值若现有 owner 实际回退默认价，目录返回 owner 求得的有效价格；默认价和有效档位均为零时才返回有效零价。标签的零值由 GetRequestTierPrice 的实际选择判定（保留首次有效标签命中），输出 FallsBackToContext=true、Price=nil，引用当前请求的 ContextTiers/default；不得将 nil 转成免费或把 context=0 的默认价当作固定标签价。非零标签单价保持独立固定价。per_request 合法重叠配置按 owner 的首次命中在所有上下文边界上切成有效 (min,max] 段，空洞回退明确默认价；没有任何明确来源的默认价保持 nil，不补免费。本任务不修改现有 fallback 语义。

## Token 阶梯与档位

`CatalogPricing.ServiceTiers` 的每项有 ServiceTier 和 ContextPricingSchedule。沿用 (min,max] 上下文、整单 basis、空洞回落、分组长上下文开关与官方阈值；分组 Token 卡的自定义 intervals 仍按原 resolver 规则移除。单价包含该服务档的真实计费策略；UI 不能再次乘 Fast/Flex。

OpenAI 给出 default/priority/flex/ultrafast 的价格规则，Anthropic 给出 default/fast，其余平台给出 default。它们不是上游能力列表，也不承诺某档一定被实际采用。FreeOpenAIFast 复用现有 groupBillsOpenAIFastAtStandard；priority/fast 在该策略下按 default 计价，ultrafast 保留自身规则。实际服务档位仍以转发与响应降档 owner 为准。

ReferenceAt 在一次 Resolve 内固定。DeepSeek 默认官方价会随真实计费时点变化，表中是该时点参考价，不能当作未来固定报价。渠道分时倍率从有效 owner 输出到外层 TimePricing；探针基础价不含这个倍率，Context.TimePricing 为空，展示时按请求时点仅应用一次。分组卡覆盖渠道时，渠道分时规则不再输出。按次 owner 不应用渠道分时，因此其 TimePricing=nil。

ReasoningEffortMultipliers 是真实 owner 对最终 none/minimal/low/medium/high/xhigh/max 的有效倍率，未列出为 1。它不预乘进档位单价；最终 effort 还可能被分组映射或降档，因此目录只展示规则，不推断用户请求原值就是最终计费等级。

## 倍率组合

`ResolveCatalogRateMultipliers(group,userRate,loaded,at)` 不读取仓库。loaded 成功且有个人值时覆盖分组默认值，包括 0；否则选择分组默认值。读取失败忽略残留个人值且 ReferenceOnly=true。它复用 computePeakAwareMultipliers、resolveImageRateMultiplier、resolveVideoRateMultiplier：Token 倍率包含订阅高峰因子，图片/视频独立倍率覆盖基础倍率，不受 Token 高峰影响。

价格表不含分组/用户倍率。展示端应用已经解析的适用倍率一次；不能同时乘 user_rate_multiplier 与 rate_multiplier。通用网关按次模型所在文本计费链沿用其已算出的 Token 倍率，不擅自改变高峰口径。价格表中的服务档位价、独立渠道分时规则、最终 effort 规则与个人/分组倍率分别各应用一次。涉及媒体入口、批量折扣、搜索附加费、账号长上下文能力及实际用量分类时，本轮规则不能代表整张请求账单。

## S2.3 接线要求及验证边界

本轮入口是服务层候选；`view=catalog` 仍未实现，默认数组不变。S2.3 按输出白名单映射以上价格数据，不直接 JSON 序列化 Group、Channel、ChannelModelPricing、PricingInterval 或 resolver。元数据内部 ID、映射目标、账号/上游信息不能带出。零标签的 FallsBackToContext 与 Price=null 必须一起映射，按真实上下文选择规则；尚无上下文时展示回退规则。数组无结果保持 []；未知模式在 DTO 中使用 null，不将服务层空枚举输出为已知模式。

本轮执行服务层定向用例、生产 owner 对账、受影响 resolver/阶梯/价格解析/旧模型广场回归及独立只读复核。现有 S0/S1 未改动输入的成功证据保留，不扩大为新目录权限/API/浏览器验证。后续 S2.5–S2.6 仍需对新聚合、跨用户隔离、HTTP 兼容与页面流程取得实际证据。
