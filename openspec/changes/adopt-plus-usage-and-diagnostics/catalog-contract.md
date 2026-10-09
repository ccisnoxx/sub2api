# S2.1 模型与价格目录权限及接口合同

- 冻结日期：2026-10-09（America/Los_Angeles）。
- 状态：S2.1 合同冻结；S2.2 [权威价格服务](pricing-contract.md)已本地实现与验证；S2.3 已实现目录查询分支和 DTO；页面留在 S2.4。
- 范围：仅 S2.1；对应[计划](plan.md)、[任务](tasks.md)与[执行证据](implementation-evidence.md#s21-模型与价格目录权限与接口合同)。
- 本轮读取应用：`codex/plus-usage-s1` / `e3edb5666a03a582bfbb83a718aedda03ba6ea08`；其中已验证应用为 `3f04437572e2819f0313ccc2a3f1a618a2afcdf0`，后续 HEAD 只归档文档。
- 最新远端及本地 personal：`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；KIN 来源保持 `.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。

本文区分当前已核实行为与未来目录合同。S2.1 完成表示权限、接口选择和兼容边界已经确定，不表示 `view=catalog` 已经在线或能够返回新对象。

## 1. 当前接口基线

现有 `GET /api/v1/channels/available` 位于用户认证路由。顺序为 JWT 认证、后台模式用户限制、面板按用户限流与既有审计，再进入 handler；JWT 继续核对用户有效状态、TokenVersion 与现有会话绑定。

handler 读取认证主体的 UserID，先确认 `available_channels_enabled`，再调用 `APIKeyService.GetAvailableGroups` 和 `ChannelService.ListAvailable`。当前不读取 `view` 参数，所有查询值均返回旧渠道数组；当前前端 `userChannelsAPI.getAvailable()` 不传 `view`，返回类型为 `UserAvailableChannel[]`。

旧成功封装固定为 HTTP 200、`code=0`、`message="success"`，`data` 是渠道数组；渠道内为 `name/description/platforms`，平台内为 `platform/groups/supported_models`。空数组不得变成 `null`。旧价格 DTO 和 intervals 数组、错误封装、排序及 `/groups/rates` 接口保持现状。

当前旧数组只输出 active 渠道与可绑定的活跃分组的交集；没有可见分组或平台 section 的渠道不输出。普通分组留在自身平台，复合分组展开到渠道配置的具体模型平台；复合分组没有有效模型时，旧视图保留空 composite section。

## 2. 查询选择与兼容合同

继续使用同一 GET 路由和成功封装，不新增公开或管理员绕过入口。未来只有 URL 解码后的 `view` 参数恰好出现一次、值恰好等于大小写敏感的 `catalog` 时选择目录。

| 查询 | 未来响应 `data` |
|---|---|
| 无 `view` | 原渠道数组 |
| `view=catalog` | 目录对象 |
| `view=`、`view=Catalog`、`view=CATALOG`、未知值或带额外空白 | 原渠道数组 |
| 重复 `view`，包括两个相同的 `catalog` | 原渠道数组 |
| 仅 `user_id/group_id/admin` 等参数 | 原渠道数组；参数不授予任何额外权限 |
| 单个 `view=catalog` 加 `user_id/group_id/admin` | 当前认证用户目录；不切换目标用户或权限范围 |

这是明确的 opt-in 合同，沿用旧接口忽略其他查询值的行为；实现必须检查参数值数量，不能只使用 `c.Query("view")` 读取第一个值。此规则参考固定 Plus 提交，并保护当前无参数客户端。

未来前端新增独立的 `getCatalog()` 调用与目录类型，显式传入 `view=catalog`；现有 `getAvailable()` 的签名和数组类型不改成默认对象或模糊联合类型。不新增分页、服务端搜索或用户范围选择参数；后续确有需求时单独定义。

## 3. 权限、状态与空结果

### 3.1 用户与功能开关

两个视图使用相同 JWT 主体，不接受网关 API Key 代替面板 JWT。管理员在用户路由仍按自身可绑定分组查看；后台模式下允许管理员进入该路由，不因此赋予全站目录权限。

| 条件 | HTTP / 数据与读取边界 |
|---|---|
| 未登录、无效/失效 JWT、停用用户 | 沿用认证层 401；不读取目录数据 |
| 后台模式开启、普通用户 | 沿用现有 403；不绕过用户自服务限制 |
| 面板限流拒绝 | 沿用现有 429 与错误封装 |
| 已认证、开关关闭或读取开关失败 | 旧视图 `data=[]`；目录 `data={"groups":[],"user_rate_status":"not_requested"}` |
| 开关开启、没有可绑定活跃分组 | 目录空对象同上；不读取渠道和个人倍率 |
| 用户/分组/订阅授权读取失败 | 沿用 `response.ErrorFrom` 的状态与封装；不能当作空权限或全部公开分组 |
| 渠道读取失败 | 返回明确错误；不能当作没有配置模型 |

关闭功能时不读取用户分组、渠道、模型或个人倍率。现有开关读取 owner 已经采用失败关闭规则，本合同保持该行为；它独立于模型广场开关。错误返回不得夹带部分目录。

### 3.2 分组授权矩阵

唯一分组授权 owner 是 `APIKeyService.GetAvailableGroups`。不要用 `GetUserGroupVisibility` 替代；后者保留模型广场的橱窗可见性，语义不同。

| 分组情况 | 可进入目录的条件 |
|---|---|
| 标准公开组，用户未限制公开组 | 活跃分组可绑定；不要求列入 AllowedGroups |
| 标准公开组，用户限制公开组 | 必须列入该用户 AllowedGroups |
| 标准专属组 | 必须列入该用户 AllowedGroups |
| 订阅组，包括专属订阅组 | 必须有该用户的有效订阅；普通授权名单不能代替订阅 |
| 有效订阅 | 复用仓库谓词：属于该用户、status 为 active、expires_at 严格晚于检查时刻 |
| 过期/停用订阅、其他用户订阅 | 不授予订阅组访问 |
| 停用或删除分组 | 不进入活跃分组集合；订阅或普通授权不能重新授予 |

公开组限制、专属标志和订阅类型是独立概念；不要另写一套组合规则。只读目录不新增余额、API Key 配额或实时账号健康检查，也不改变实际网关准入。

### 3.3 渠道、模型与复合分组

先取授权分组，再关联 active 的绑定渠道，最后枚举与筛选模型、解析报价并组装目录。不得先按全站模型合并，再让前端隐藏无权分组。

- 每个分组至多绑定一个渠道，复用 `channel_groups(group_id)` 唯一索引；目录不改变该约束。
- 有权活跃分组即使没有绑定渠道、渠道已停用、无配置模型或所有模型被过滤，仍保留分组概要和 `models=[]`；不输出停用渠道的名称、描述或报价。旧数组的省略行为不变。
- 模型候选仅来自该绑定渠道的 `Channel.SupportedModels()`：mapping 与 pricing 的并集及既有有限通配展开。全局价格目录不能增加模型，目录查询不探测上游或拉取实时账号模型。
- 普通分组只保留自身平台；复合分组只使用该绑定渠道明确配置的具体平台。复合分组无具体模型时不创建一个虚构 composite 模型。
- **新目录按每个分组对公开请求模型名调用 `Group.ModelAllowlist.Allows`。** 已配置的白名单仍须约束目录；复用现有匹配/归一化，不对映射后的内部模型名判断，不另写通配规则。白名单只缩小候选集合，不通过白名单生成额外模型。
- 旧 handler 当前没有这一白名单过滤；S2.1 不修改旧数组。新目录的过滤属于 S2.3 的实现与 S2.5 的验证范围。
- 目录描述的是“可绑定分组中配置且通过模型白名单的模型”。它不保证账号健康、剩余额度、客户端/能力限制、实时路由成功，也不绕过具体 API Key 的其他限制。

## 4. 目录对象与字段边界

### 4.1 稳定外层

```typescript
interface AvailableModelCatalog {
  groups: CatalogGroup[]
  user_rate_status: 'loaded' | 'unavailable' | 'not_requested'
}

interface CatalogGroup {
  id: number
  name: string
  description: string
  platform: string
  subscription_type: string
  is_exclusive: boolean
  rate_multiplier: number
  user_rate_multiplier: number | null
  peak_rate_enabled: boolean
  peak_start: string
  peak_end: string
  peak_rate_multiplier: number
  models: CatalogOffer[]
}

interface CatalogOffer {
  offer_key: string
  name: string
  platform: string
  source: { name: string; description: string }
  billing_mode: string | null
  billing_unit: string
  price_status: 'resolved' | 'unknown'
  price_reason: 'pricing_unavailable' | 'request_dependent' | 'unsupported_unit' | null
  pricing: CatalogPricing | null
}
```

这些是未来数据合同的表达，不是当前已生成的 TypeScript 类型。`CatalogPricing` 以既有用户价格白名单为基础，其权威价格、阶梯/档位/媒体条件及新增可选字段在 S2.2 冻结，在 S2.3 实现；不得以现有展示合成价冒充已解析的实际报价。

分组可选价格上下文字段，如媒体独立倍率、长上下文开关和时区，须来自 KIN 的已有规则，由 S2.2 明确后添加；不照搬 Plus 当前产品不支持的配置。未定义字段不通过整个 service/domain 对象序列化带出。

`groups/models/intervals` 均为数组，空时为 `[]`。价格缺失为 `null`，可信的明确零价保持数值 `0`；不把零价用真假判断丢弃。`billing_mode` 不明时为 `null`，计费单位不明时为 `unknown`；已解析单位必须来自对应计费 owner，不能只按模式猜测。字段无效或上下文不足时，不输出貌似精确的报价。

`offer_key` 是用于渲染的稳定、不透明身份，按分组、具体平台、大小写无关的请求模型 ID 和绑定渠道派生；分组/渠道显示名称、描述、报价及排序变化不改变身份，公开请求模型 ID 或绑定渠道变化则改变身份。算法在 S2.3 一次确定，不将原始内部渠道 ID 编入可读字段；key 不作为授权凭证。

分组沿用活跃分组的 sort_order/ID 顺序；同组模型按具体平台、大小写无关模型 ID 稳定排序/去重，保留请求模型的原始拼写供复制。不同分组报价不在 API 层合并丢失；S2.4 允许前端按模型聚合卡片，但保留每份分组 offer。

### 4.2 个人倍率与失败状态

- `loaded`：该用户倍率读取成功，包含没有覆盖配置的情况；仅输出授权分组的覆盖。没有覆盖为 `user_rate_multiplier=null`，明确零倍率为 `0`。
- `unavailable`：授权和渠道读取成功，但个人倍率读取失败。保留允许看到的目录，覆盖值为 `null`，展示报价标为参考并提供重试；不能把默认倍率宣称为实际个人价格，也不能沿用上次用户/刷新结果。
- `not_requested`：开关关闭或没有授权分组，没有发起个人倍率读取。

只读取一次当前用户倍率，嵌入目录响应；新目录页面不拼接第二次 `/groups/rates` 结果。旧页面和旧倍率接口保持现状。授权信息读取失败不能使用 `unavailable` 降级。

价格可用状态与个人倍率状态独立。`resolved` 说明规则在报价上下文中已解析，不等于个人倍率必定成功；最终展示若缺少倍率，仍必须标明参考。目录价格采用 USD 每 Token 或对应原始单位、倍率应用前的规则；前端仅做单位/显示转换和应用已解析适用倍率一次，具体媒体/高峰条件以 S2.2 的 KIN 合同为准。

### 4.3 输出白名单与请求生命周期

允许输出用户可见的分组 ID/名称、上述分组与模型字段、渠道显示名称/描述和必要价格条件。不得输出账号 ID、凭据、API Key、上游 URL、内部映射目标、内部渠道/定价 ID、完整授权名单、其他用户身份/订阅或倍率。

目录在一次请求内组装，渠道枚举与后续价格解析使用同一份渠道配置读取结果，不混用默认展示的合成数据与另一次刷新。S2 不新增跨用户目录缓存或权限缓存，也不承诺超出现有读取链的事务一致性。用户切换或刷新后旧响应的处理留在 S2.4，并验证不会覆盖新上下文。

## 5. 权责与下一步

| 责任 | 权威 owner / 后续任务 |
|---|---|
| 用户主体、后台模式、面板限流 | 既有用户路由与 middleware，不新增绕过 |
| 可绑定分组 | `APIKeyService.GetAvailableGroups`、`User.CanBindGroup`、有效订阅仓库 |
| 具体模型候选 | `Channel.SupportedModels` 与当前绑定渠道配置 |
| 分组模型白名单 | `Group.ModelAllowlist.Allows`，新目录复用 |
| 最终报价规则和倍率条件 | KIN 现有计费解析；S2.2 接入与冻结细节 |
| 目录 DTO 与查询选择 | S2.3，在当前 handler 边界按白名单输出 |
| 页面、合并卡片和响应失效 | S2.4，保留旧 API 客户端类型 |
| 新分支与权限/报价验证 | S2.5–S2.6，以实际实现为对象 |

**下一项为 S2.2：接入 KIN 权威价格解析。** 本轮不实现 handler 分支、DTO、resolver、页面、迁移或测试代码，也不进入下一项。

## 6. 验收映射与当前验证边界

| 编号 | 后续实现必须验证的合同 | 当前已有证据 |
|---|---|---|
| AC01 | 无参数/空/未知/大小写不同/重复 view 保持数组，唯一个 catalog 返回对象 | 现有 handler 与客户端已核对；新分支待 S2.3/S2.5 |
| AC02 | 两种视图认证、后台模式与限流一致 | route、JWT、guard 源码；既有认证/guard 测试已定位，未重跑 |
| AC03 | 关闭功能在认证后返回对应空形状，不访问目录依赖 | 现有 handler/设置读取已核对；目录空形状待实现 |
| AC04 | 公开限制、专属授权、有效/过期/他人订阅遵循同一 owner | GetAvailableGroups/CanBindGroup/订阅 SQL；既有相关用例已核对 |
| AC05 | 无权限分组、渠道名称和报价不进入目录；用户参数不切换主体 | 现有交集过滤已核对；新目录跨用户合同待验证 |
| AC06 | 停用/无绑定渠道不贡献模型；有权组保持 models 空数组 | 现有 active/group 读取已核对；目录保留规则待实现 |
| AC07 | 普通平台隔离、复合平台有限展开，模型不从全局价格扩增 | SupportedModels/platform sections 与既有用例已核对 |
| AC08 | 白名单拒绝的公开请求名不输出，映射目标不用于准入判定 | 既有 Allows/网关 owner 已核对；目录接线待实现 |
| AC09 | 授权/渠道失败显式失败；仅个人倍率失败显示 unavailable 与参考价 | 既有错误传播与倍率 owner 已核对；新组合待实现 |
| AC10 | null/0 可区分、倍率应用一次、报价与实际 KIN 规则一致 | 旧展示合成价的限制已确认；权威解析待 S2.2 |
| AC11 | 白名单、稳定身份、空数组与旧客户端类型兼容 | 既有 DTO/类型核对；新 DTO 与身份待 S2.3 |
| AC12 | 查询不改变 Token、扣费、账号选择，也不发起上游探测 | 本轮应用文件未修改；后续实现须做对应检查 |

本轮只做源码、合同和文档一致性检查。计划规定文档阶段不运行应用测试或构建；既有用例定位表示已有保护边界，不表示本轮执行成功，也不表示新目录已经受测试保护。S1 用量/兼容验证记录保持原范围，不借来证明目录权限或报价。未使用真实用户/生产凭据，没有新增独立代码复核。

## 7. 来源与检查清单

应用源文件、固定 SHA、内容校验值与逐项检查记录见[本轮检查清单](evidence/s2.1-contract-checks.json)。核心现有源文件：

- `backend/internal/server/routes/user.go`；`backend/internal/server/middleware/jwt_auth.go`、`backend_mode_guard.go`。
- `backend/internal/handler/available_channel_handler.go` 及其既有测试；`backend/internal/pkg/response/response.go`。
- `backend/internal/service/api_key_service.go`、`user.go`、`group_model_allowlist.go`、`channel_available.go`、`channel.go`。
- `backend/internal/repository/group_repo.go`、`user_subscription_repo.go`；`backend/migrations/081_create_channels.sql`。
- `frontend/src/api/channels.ts`、`frontend/src/views/user/AvailableChannelsView.vue`。
- [Plus 固定提交的可用渠道合同](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/AVAILABLE_CHANNELS.md)：只参考 opt-in 选择与目录组织，KIN 现有权限和计费 owner 保持权威。

## S2.3 已实现的目录 DTO 与查询分支

S2.3 已在 personal 来源候选中实现，目录仍受现有 `available_channels_enabled` 开关与用户路由 middleware 控制。默认客户端继续读取旧渠道数组；仅单个精确 `view=catalog` 返回本文定义的目录外层。页面改造属于 S2.4。

目录外层以分组保留每份报价；每组 `models` 按具体平台、大小写无关请求模型 ID 排序/去重，使用 `Channel.SupportedModels()` 的有限集合与 `Group.ModelAllowlist.Allows()`。同平台同模型的不同分组 offer 继续分别输出，供 S2.4 聚合卡片。无绑定、停用渠道、无配置或被白名单全部过滤的有权组保留 `models=[]`。

`offer_key` v1 是 `offer_` 加 SHA-256 的64位小写十六进制：输入为 `catalog-offer-v1`、十进制分组 ID、十进制绑定渠道 ID、具体平台、`strings.ToLower` 后请求模型 ID，以 NUL 分隔。显示名称、描述、价格、分组排序及模型 ID 的大小写变化不改变 key；请求 ID 或绑定渠道身份改变会改变 key。内部渠道 ID 不以可读字段输出，key 不提供授权保证。

S2.2 允许的分组价格上下文补充为 `long_context_pricing_enabled`、`image_rate_independent/image_rate_multiplier`、`video_rate_independent/video_rate_multiplier` 与 `rate_multipliers`。后者包含 `token/image/video`、`reference_only`、`pricing_at`、`timezone`，由 `ResolveCatalogRateMultipliers` 求得；个人值覆盖默认值含0，读取失败忽略残留个人值。时点采用同一次请求的价格参考时点，时区来自真实高峰 owner。消费者使用适用的已解析倍率一次，不再同时乘默认/个人值；时点可能变化，仍应展示规则。目录不提供实际媒体单价。

`CatalogPricing` 的显式输出白名单如下；时间按 Go `time.Time` 的 RFC3339 格式序列化，数值使用美元原始单位：

| DTO | 字段与含义 |
|---|---|
| pricing | `reference_at/reference_only`、`service_tiers`、`request_pricing`、`time_pricing`、`reasoning_effort_multipliers`、`unsupported_components` |
| service_tiers[] | `service_tier`、`context`；服务档单价已含该档策略，不再次乘 Fast/Flex |
| context | `basis`、`intervals`；当前 basis 为 `whole_request` |
| intervals[] | `min_tokens/max_tokens/tier_label`、`input_price/output_price/cache_write_price/cache_write_1h_price/cache_read_price`；保留 (min,max] 和 nil/0 |
| request_pricing | `default_price/context_tiers/size_tiers`；每个 tier 为 `min_tokens/max_tokens/label/price/falls_back_to_context` |
| time_pricing | `timezone/weekdays_only/periods`；period 为 `start_time/end_time/multiplier`，与基础单价分开应用一次 |

所有列表为空时输出 `[]`，effort 映射为空时为 `{}`；未知价格、成功 reason、未知模式使用 `null`。`falls_back_to_context=true` 与 `price=null` 成对保留，消费者读取真实上下文规则，不将空标签价解释为免费。

目录 GET 没有实际请求入口或媒体分类事实，因此未显式提供 `UsageKind=request`；按次/图片/视频配置保留模式并返回 `unsupported_unit/unknown`。映射依赖上游或响应模型返回 `request_dependent`。DTO 支持 S2.2 的按次回退规则映射，当前 GET 不据 `per_request` 模式猜测 USD/request。

授权在渠道读取和模型聚合之前完成，渠道枚举及 resolver 复用同一次 `ListAll` 的克隆配置。用户倍率只读一次；仅此读取失败可形成 `unavailable` 与参考默认倍率。其他授权/渠道/价格错误沿用 `response.ErrorFrom`，不夹带部分目录。不新增缓存、结算、调度、上游探测或媒体计费算法。

本轮 HTTP 夹具验证新分支、主体不受查询参数切换、公开/专属/订阅授权、白名单/平台隔离、空组、错误和倍率状态、报价与字段白名单。夹具注入仓库边界但执行真实授权与价格 owner；没有真实 JWT/生产凭据或数据库读取，S2.5 的进一步一致性验收、S2.6 的浏览器查看流程仍未执行。


## S2.4 页面消费合同

已有可用渠道页面现在显式调用getCatalog，保留旧getAvailable数组客户端。卡片按平台与大小写无关模型名聚合，保留每份分组offer；本地搜索和分组过滤只缩小已授权响应，不扩增模型。刷新、用户ID变化及卸载清除/取消旧目录，用请求序号、主体和取消信号拒绝晚到成功或失败。

详情只应用rate_multipliers.token一次，Token价格转换为USD/1M token；服务档单价已经包含对应策略。分组默认与个人覆盖值作为解释字段，不再次相乘。分时与最终effort单独展示规则，并明确未进入表内参考单价。每份报价显示参考时点、高峰规则与未知/未支持原因；不猜测媒体单位、路由选择或最低价。个人倍率unavailable有明确参考提示及刷新重试。尚无媒体单价，USD/request只有服务端确认为该单位时才可显示。

本轮10个前端合同、4组中英/桌面/窄屏浏览器和fresh只读复核的实际边界见[执行证据S2.4](implementation-evidence.md#s24-可用渠道页面模型与分组报价目录)。这些证据不表示S2.5/S2.6已完成。
