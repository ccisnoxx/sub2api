# S3.1 结构化路由诊断合同

- 冻结日期：2026-10-09（America/Los_Angeles）；状态：S3.1–S3.6完成，草稿候选门禁通过，未合并/发布/部署。第1–7节保留冻结来源，第8–12节保留各轮历史，第13节登记S3.6阶段交付；v1语义未改。
- 应用来源：personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；只读累计候选 `88156f09fcf980a771a8aab570f0dbaec5de25fb`，工作树 `/Users/sc/.codex/worktrees/verify-personal-ci-plus-catalog-s27/sub2api-kin`。
- KIN 来源：`v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`；[草稿 PR #5](https://github.com/ccisnoxx/sub2api/pull/5) 未合并。
- 对应[计划](plan.md)、[任务](tasks.md)及[执行证据](implementation-evidence.md#s36-独立复核与阶段交付)。第1–7节的“当前候选/未来”均指S3.1冻结候选，第8–12节保留历史时点；最新验证及覆盖限制以第13节为准。

## 1. 已核实的事实与 owner

下列源码定位均来自上述 personal 累计候选；main 来源维护工作树只登记本文。

| owner / 候选内定位 | 当前事实 | 对未来合同的约束 |
|---|---|---|
| `service/openai_account_scheduler.go:26,382`，`defaultOpenAIAccountScheduler.Select` | previous response、guardian parent、session sticky、load balance 按实际分支选择；决策含整数 `CandidateCount` | 层名沿用既有机器码；旧整数默认 0 没有“已观察”语义，不能直接转为池 0 |
| 同文件 `:2185,2384`，`selectAccountWithScheduler` / `selectAccountWithSchedulerOnce` | gwpool 轮换/复核、代理隔离第二轮、legacy 和 advanced 路径；图片 native→basic 另跑选择 | 每次实际重新评估独立形成完整快照；不拼接不同轮次的数量 |
| `service/openai_gateway_service.go:682`、`gateway_forward.go:987`，两个 `checkChannelPricingRestriction` | requested/channel mapped/response 定价预检查；upstream 定价留到逐账号检查 | 预检查拒绝是 `channel_pricing`，未列举池时数量未知；逐账号限制是过滤事实 |
| `service/openai_gateway_scheduling.go:1517,1718`、`gateway_scheduling.go:1014,1539`，列表及阈值 owner | 快照/SQL 返回后还会做阈值过滤，Grok 还做免费配额过滤；SQL/快照已限制平台、分组及可调度范围 | 池的观察点须在这些本地后置过滤前；不能用最终空返回值冒充入口池，也不能称为全库总量 |
| `service/openai_account_scheduler.go:1366,1407,1787`，过滤统计与兼容检查 | `openAISelectionFilterStats` 只覆盖部分首个拒绝；Grok 前置过滤、compact、去重、fresh/DB 复核不全在其中 | 用实际结构化分支形成统计；现有 summary 文案与旧 `CandidateCount` 不构成完整诊断 |
| `service/openai_gateway_scheduling.go:371,536`、`openai_profit_control.go:302`、Grok 三项过滤 owner | legacy/advanced 的模型、配额、利润等拒绝码并不完全相同；免费额度未知时现有代码放行并异步刷新 | 保留各路径真实判定；未知额度不生成“配额拒绝”；不得为了统一诊断再调用有副作用的检查 |
| `service/openai_account_scheduler.go:860,1178,1680`，排序、抢槽、等待 | TopK/订阅优先级/compact 暂缓复核/预算会限制探查；抢槽失败可能转 WaitPlan | 排序落后、未探查和可等待账号不计为已过滤；不能由选择失败推断全池容量耗尽 |
| `handler/openai_gateway_handler.go:667`、`handler/failover_loop.go` | handler 重选、同账号重试、失败排除、利润终检、取消退出各有现有规则 | 诊断只观察；不改变次数、冷却、切换、释放、粘性和扣费上下文 |
| `service/ops_upstream_context.go:177,260,380` | WS turn 清理上游状态；带内失败快照账号/模型/尝试；尝试归因含 stage/scope/reason；请求级结果隔离残留归因 | 选择快照按请求、turn 和实际选择绑定；尝试及历史失败保留各自不可变副本 |
| `handler/ops_error_logger.go:211,1283,1437,1587,1716`、`service/ops_service.go:461,569` | 队列前脱敏与预算；最后尝试归因；恢复错误仍是 provider telemetry；account_auth 清理旧推理状态 | 日志链路传递快照，不重新选号、统计或改变 SLA/归因/skip_monitoring |
| `service/ops_port.go:64`、`ops_models.go:79`、`repository/ops_repo.go:397` | 输入、存储、错误详情目前无新字段 | S3.3 另做 nullable 字段与迁移，历史未知不回填 |
| `server/routes/admin.go:28,243`、`handler/admin/ops_handler.go:22`、`service/ops_service.go:770`、`ops_user_error.go:13,127` | 管理路由须 admin 认证和监控开关；用户详情须直接 user_id 归属并走独立白名单 | v1 只在既有管理员详情可见；用户列表/详情不新增诊断或内部字段 |

上表文件位于 `backend/internal/`。完整文件校验值、符号与检查边界见 [S3.1 来源证据](evidence/s3.1-candidate.json)。已有 handler 会为旧对客错误分类解析 summary，且模型可用性诊断使用独立数据库查询；S3 不借用这两种结果重建选择统计，也不改其现有对客分类。

## 2. v1 数据合同

`routing_diagnostics` 是管理员错误详情的可选对象；缺失或整体 `null` 表示没有该次选择快照。新 producer 一旦产生对象，下列字段全部输出，字段级未知显式为 JSON `null`。持久化的 SQL NULL 与字段级 JSON null 不能改写成 0。

| 字段 | 类型 | 冻结含义 |
|---|---|---|
| `schema_version` | integer，固定 `1` | 只标记本文合同；未来语义变化另行版本化 |
| `turn` | positive integer / null | WS 的实际逻辑 turn；HTTP/SSE 为 null，不把连接当一个 turn |
| `selection_attempt` | positive integer | 请求/turn 内实际完整选择评估的序号，从 1 开始；区别于切换次数和上游发送次数 |
| `selection_layer` | 白名单 string / null | 最终产生该次选择结果或拒绝的 owner 层；未观察到层时为 null |
| `selection_reason` | 白名单 string / null | 该层实际返回分支的机器码；不是 error.type、HTTP 状态或自由文案 |
| `candidate_pool` | nonnegative integer / null | 本次观察到的入口候选列表大小，观察点见第 3 节 |
| `filtered_candidates` | nonnegative integer / null | 同一入口池内，已确认在本次评估中被排除的不同账号数量；部分统计是已知下界 |
| `filter_reasons` | 白名单 code→positive integer 对象 / null | 上述已知排除的首个确定理由；已观察无排除为 `{}`，未观察为 null |
| `filter_coverage` | `complete` / `partial` / `unobserved` | 过滤统计是否覆盖本次池的实际排除；不声明全库、未来或全部容量可用性 |

数量与序号须能被 JSON 客户端精确表示（整数不超过 `2^53-1`）。负数、非整数、未知版本、非法码、总数不一致是无效诊断，不修成 0。后续接收边界拒绝该诊断对象并保留原错误记录及不含原始值的内部异常信号；不得因此覆盖或丢弃真实故障。

### 选择层与原因

| `selection_layer` | 可观察分支 / `selection_reason` |
|---|---|
| `channel_pricing` | `channel_pricing_restricted`：现有渠道定价预检查明确拒绝 |
| `previous_response_id`、`guardian_parent`、`session_hash` | `sticky_hit`：实际命中并取得槽位；`wait_plan`：实际返回等待计划；该层终止失败但原因未细分时 `selection_failed` |
| `load_balance` | `slot_acquired`、`wait_plan`、`pool_empty`、`candidates_filtered`、`compact_unsupported`、`selection_exhausted`、`account_list_failed`、`selection_failed` |
| `gateway_pool` | `gateway_pool_restricted`：已有轮换/休息/准入终检明确拒绝；该层检查失败且没有确定拒绝原因时 `selection_failed` |
| null | 没有实际层事实；若 owner 只知道选择过程失败可用 `selection_failed`，连原因也未观察则 null |

这些是未来的规范码，并非声明当前代码已有同名 producer。`pool_empty` 必须观察到入口池 0；`candidates_filtered` 必须完整确认同池全部排除；compact 错误使用实际 compact sentinel/拒绝分支，不能由旧 count=0 推断。`selection_exhausted` 只表示现有选择顺序/探查预算未取得结果，不等于全池账号都不可用。`wait_plan` 是已选择待获取容量，不能称成功发送。

加权 sticky 仅参与 load balance 排序时仍为 `load_balance`；实际执行 weighted sticky fallback 时由该分支报告 `session_hash`。legacy 与 advanced 按实际命中分支对齐，不能照抄 legacy 预填的 `decision.Layer=load_balance`。非 OpenAI owner 没有对应层时保持 null，不伪造 previous response 或 guardian。

### 过滤原因白名单与对应分支

| code | 实际判定来源 |
|---|---|
| `excluded`、`not_schedulable`、`platform_mismatch` | 同池首个失败排除、可调度或平台检查 |
| `account_model_not_owned`、`model_not_supported`、`channel_upstream_restricted` | account composite 模型归属、模型支持、逐账号上游模型限制 |
| `runtime_blocked`、`privacy_not_set`、`proxy_stream_quarantined`、`shadow_parent_unhealthy` | 现有运行期、隐私、代理隔离及母账号健康检查 |
| `model_rate_limited`、`turn_state_hold` | 现有模型级限流/暂停判定；暂停码只按现存数据解释，不启用已废弃功能 |
| `quota_auto_pause`、`quota_auto_pause_5h`、`quota_auto_pause_7d`、`quota_auto_pause_retry_after`、`quota_auto_pause_requests`、`quota_auto_pause_tokens` | OpenAI 5h/7d 与 Grok retry_after/requests/tokens 的实际配额判定；没有窗口用无后缀码 |
| `quota_auto_reset_pending_5h`、`quota_auto_reset_pending_7d`、`quota_auto_reset_credit_check_5h`、`quota_auto_reset_credit_check_7d` | advanced 现有自动重置配额决定；legacy 若只返回 auto_pause，保持它真实返回的码 |
| `profit_threshold`、`profit_invalid_account_rate` | 现有利润准入 owner；只记录码与计数，不记录成本/售价/阈值 |
| `capability_mismatch`、`transport_incompatible`、`compact_unsupported` | 已有能力、传输或最终 compact 兼容拒绝 |
| `scheduling_threshold` | 列表 owner 的 `ApplyAccountSchedulingThreshold` 明确过滤 |
| `grok_free_quota_soft_gate`、`grok_team_model_rate_limit`、`grok_model_quota_block` | 三个现有 Grok 过滤 owner 的实际移除；缓存未知且放行不计拒绝 |
| `gateway_pool_duplicate` | 现有凭据域去重移除账号行；只计行数，不输出凭据域或凭据容量 |
| `session_limit`、`capacity_limited` | 对应 owner 明确把账号排出本轮；抢槽失败但仍可等待/重试不计 |
| `recheck_rejected` | fresh/DB 复核使账号最终退出本轮但未提供更细稳定理由；只说明复核拒绝，不猜测底层故障 |

空账号指针不是合法池成员，不能凭 `account_nil` 增加候选/过滤数。未知 owner 原因不直接透传字符串；没有可核实的白名单原因时统计保持未知/部分，并保留原错误。白名单扩展须先更新合同及相应观察证据。

## 3. 池、过滤统计与 NULL

观察点固定为**本次实际选路范围内，快照或仓库成功返回候选列表之后、KIN 列表 owner 的本地阈值/Grok 等后置过滤之前**。SQL/快照固有的活跃、schedulable、分组、平台等限制已经生效；数据库未返回的账号数未知，不计入池或过滤。计数单位为账号行，同一 ID 只算一次，不是凭据、用户、组、TopK 或所有配置账号总量。

不得为诊断额外拉全库、刷新缓存、重跑配额/利润/并发检查或延长 probe budget。现有代码确实取到了入口列表时，在这个 owner 保留事实；只接收到过滤后的列表而入口没有传递观察值时，入口 `candidate_pool` 必须为 null。后置全过滤后仍保留入口正数，不能覆盖成 0。原位过滤（Grok team）须在执行前取得观察值；诊断不能改变它的原位算法。

一次评估只用一次入口观察集合。同池账号在首个**最终排除**处登记一次，不重复累加 fresh/DB 检查、抢槽、等待或重试；temporary compact tier=0 若后来经 stale-snapshot recheck 重新合格，不计最终过滤。优先级、排序、TopK、订阅/普通子池分区、加权 sticky、负载未知和预算未探查都不属于过滤。

| 实际观察 | `candidate_pool` | `filtered_candidates` / `filter_reasons` / `filter_coverage` |
|---|---:|---|
| 入口成功列举为空 | 0 | `0 / {} / complete` |
| 未列举：渠道定价先拒绝、列表报错、仅 sticky 单账号查询、未接入路径 | null | `null / null / unobserved` |
| 池 4，完整观察排除 3，另一账号取得槽位或等待 | 4 | `3 / {实际原因:3} / complete`；不能把剩余 1 称为实时健康 |
| 池 4，只确定 1 个排除，其余因预算/复核缺失不明 | 4 | `1 / {实际原因:1} / partial`；不能据此写“仅剩 3 个可用” |
| 池 4，只观察池，没有任何过滤观察 | 4 | `null / null / unobserved` |
| 已观察的过滤检查暂未发现排除，但完整覆盖不明 | 正数 | `0 / {} / partial`，与未观察有别 |
| Grok/compact 确认把入口 2 全部最终排除 | 2 | `2 / {实际原因:2} / complete`；不能改池为 0 |

`sum(filter_reasons.values()) == filtered_candidates`；每个原因数量必须大于 0。池已知时 `0 <= filtered_candidates <= candidate_pool`。`complete` 必须池及过滤数均已知；`partial` 必须有已知过滤数与原因对象，可在入口未知时只表达已知排除下界；`unobserved` 的过滤数与原因均为 null。池未观察但只知道若干实际排除时，不能从排除数反推池。`complete` 只说明实际排除已完整记录，不保证尚未探查账号可获得槽位。

## 4. 请求、选择评估、发送尝试与 WS 归属

沿用现有服务端 RequestID、认证 user_id/APIKey/group 归属；诊断不接受客户端传入的 owner、序号或对象。不把客户端请求 ID、session hash、previous response ID、gwpool 凭据域用作新诊断标签。

`selection_attempt` 在一次 HTTP/SSE 请求或一个 WS turn 中单调递增。外层 handler 重选、代理隔离第二轮、gwpool 重新选择、图片 native→basic 重新评估都是新选择；内部同一入口池的排序/fresh/DB/抢槽/等待属于原评估。订阅/普通子池切换仍属于同一入口池，不能把最终子池 count 当入口池数；真正重新列举则是新评估。

一次选择可支持多个同账号上游发送尝试。未重新选择的发送只携带绑定的不可变选择快照；一旦重新评估就生成新序号及全新快照，未观察字段恢复 null。序号不充当现有 SwitchCount、SameAccountRetryCount、计费幂等键或上游重试预算。

- 调度/限制 owner 返回成功选择或错误时均交付完整快照。携带错误须保留 `errors.Is` 对既有 no-account/compact sentinel 的识别；日志读取结构化附带值，不解析 `err.Error()`。
- 每次选择开始清空“当前选择”临时状态；完整快照通过结果或错误到 handler。handler 只绑定到实际选择/发送/终态，不拼接旧字段。外层准入终检若改变结果，由对应选择 owner 重新输出完整结果，保持本次入口观察范围；不能给前次已落日志的副本打补丁。
- 上游失败事件保存失败尝试对应的副本；最终本地重选失败保存该次重选诊断，先前 provider 事件保存各自旧快照。若最终失败的现有归因 owner 是先前上游尝试，顶层诊断也须取该尝试绑定值；后来的选号失败保留在对应结构化错误上下文，不与 provider 归因混合。
- WS `BeginOpsStreamTurn` 的后续扩展要同时清理当前选择状态、重置选择序号；已捕获旧 turn 的 `OpsStreamError` 不可变。同 turn 首个标记生效与现有 64 个失败快照上限继续适用；未再次选号的连接复用不能把上一个 turn 的池数伪装成本 turn 已观察数，本 turn 无选择时新诊断为 null。
- `RequestScoped` 内容策略等终态不继承先前上游选择归因；新诊断为 null。恢复成功的中间 provider 错误继续是既有 telemetry，不能因带诊断变成失败请求或计入失败 SLA。
- 传输/身份/认证 producer 只写现有自身的 stage/scope/reason、代理等字段；不覆盖选择层、原因、池与过滤，也不从当前 gin context 借一个不匹配的选择快照。其发送尝试可携带已经绑定的选择快照，不能补猜选择事实。

进入 stream snapshot、异步队列之前，嵌套 map/对象须复制到记录 owner，不能保留仍可被下一轮修改的引用。S3.2/S3.3 需要针对“先排队再修改请求”和“后一 WS turn”直接保护此合同；本轮未实现复制或任何生产接入。

## 5. 日志、可见范围与敏感信息边界

v1 只向既有管理员错误详情返回对象（`/api/v1/admin/ops/errors/:id` 与复用该详情 owner 的 request-errors 详情），复用现有 admin 认证、审计、合规及监控开关。普通用户 `UserErrorRequest` / `UserErrorRequestDetail` 白名单不增加新字段，直接 user_id 归属检查继续生效；列表、导出、公开响应/SSE/WS 错误帧、系统日志自由文本均不新增候选计数或内部原因。若以后开放用户可见信息，须另冻结脱敏合同。

诊断只含本文的版本、序号、层/原因白名单及数字；不得记录账号列表/名称/ID、组/用户详情、成本/售价/利润阈值、credentials、API Key（包括前缀）、cookie/token、session/previous response ID、gwpool 域、代理 URL、原始请求/响应体、完整 URL 查询、原始头/trailer 或签名声明值。既有管理员详情的账号/代理字段属于已有归因，不复制到新对象；敏感边界不以“管理员可见”为豁免。

`OpsInsertErrorLogInput → 队列 → OpsService → repository → OpsErrorLogDetail` 传同一白名单快照，不在读取时用当前账号状态回填历史。可选持久化采用新增 nullable JSONB（具体迁移编号、SQL/Ent/DTO 实现留 S3.3），无旧记录回填、无数据库默认 `{}`/0、无更新 migration ledger；新旧应用兼容与回退证据也留 S3.3/交付阶段。

有限字段及有限原因码保证对象有界；后续队列估算必须计入新对象，沿用已有 bytes/event 界限与较早尝试丢弃标记，不另建无界请求历史。现有 upstream 数组最多 256 个事件、512 KiB、最近 16 个正文窗口；诊断不绕过这些界限，也不新增无实际错误的上游事件来保存选路过程。无法绑定的历史条目为 null，不能用最后一个快照回填整个数组。

增加该对象不改变 phase/type/owner/source、HTTP/wire/intended status、account_auth 的状态 0、business-limited、skip_monitoring、统计/SLA、错误处理返回路径、冷却、重试、切换、gwpool、释放槽位、身份、配额/利润与扣费语义。

## 6. 固定示例与后续验收边界

以下示例是合同样例，不是当前 API 的运行响应。

入口确实为空：

```json
{"routing_diagnostics":{"schema_version":1,"turn":null,"selection_attempt":1,"selection_layer":"load_balance","selection_reason":"pool_empty","candidate_pool":0,"filtered_candidates":0,"filter_reasons":{},"filter_coverage":"complete"}}
```

账号列表之前被渠道定价拒绝（下一次选择不能借上次的池）：

```json
{"routing_diagnostics":{"schema_version":1,"turn":null,"selection_attempt":2,"selection_layer":"channel_pricing","selection_reason":"channel_pricing_restricted","candidate_pool":null,"filtered_candidates":null,"filter_reasons":null,"filter_coverage":"unobserved"}}
```

WS turn 2 已观察池 4，只知道一个模型拒绝，预算内未取得结果：

```json
{"routing_diagnostics":{"schema_version":1,"turn":2,"selection_attempt":1,"selection_layer":"load_balance","selection_reason":"selection_exhausted","candidate_pool":4,"filtered_candidates":1,"filter_reasons":{"model_not_supported":1},"filter_coverage":"partial"}}
```

S3.2 先接入真实选择/限制 owner，并逐入口登记覆盖：advanced、legacy、channel pricing、compact、Grok、gwpool 重选；HTTP/SSE、WS turn、图片 capability fallback 与其他平台各用实际 owner，未接入保持未知。不能把本文列出平台当作已完成覆盖。S3.3 再验证队列/持久化/详情 NULL 和权限；S3.4 只在管理员详情展示观察与部分/未知状态；S3.5 对新增可观察合同补充定向测试并确认调度/重试/计费不变。

后续需直接观察的反例：列表报错不写空池；提前拒绝清掉旧计数；Grok 原位全过滤保留入口正数；compact 暂缓后恢复不算最终过滤；WaitPlan 不算容量排除；子池/TopK 不冒充入口；重复复核不重复计数；同账号重试与重新选择分开；终态归因与对应尝试一致；WS 旧 turn/队列副本不被后来覆盖；RequestScoped/恢复 telemetry 不改变既有分类；用户详情不泄露新字段。这些是未来实现验收项，本轮只完成源码对照和文档样例核对。

## 7. 参考与验证状态

参考固定 Plus 提交 `90da415c62b94c9417d9ce2b72b1507ed22f0303` 的 [ERROR_REQUEST_DIAGNOSTICS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/ERROR_REQUEST_DIAGNOSTICS.md)：借鉴由决策 owner 提供完整快照、已观察空池与未知分开、独立 producer 不继承选择计数。本文的池观察点、部分统计、序号/turn、白名单、权限及 KIN 分支映射根据本地 personal 源码冻结；不移植 Plus 整条历史或其其他安全/持久化功能。

S3.1 已做来源/工作树/PR 核对、实际 owner 源码与既有测试边界检查、JSON 样例/机器码/文档链接与保留检查。S2.7 的 24 个证据产物校验一致，现行 [Personal CI 37982040213](https://github.com/ccisnoxx/sub2api/actions/runs/37982040213) 按未变化候选的原边界复用；它不证明新诊断实现。本轮没有应用测试/构建、数据库/浏览器、完整门禁或新的独立代码复核；源码未改，S3 实现独立复核留到实际候选。完成 S3.1 后停止，下一项 **S3.2 接入实际决策 producer**。


## 8. S3.2 实际 producer 覆盖与验证

本地应用提交`cee1e908261c68880040b740aae8054410e03070`，基于上述personal累计候选88156f09，在独立`codex/plus-routing-producer-s32`应用树实现；没有更新草稿PR #5或personal。第1–7节的“当前/本轮/未来”描述属于S3.1冻结时点，以下覆盖不改变其中v1字段、NULL、计数、归属或敏感合同。

| 实际owner/入口 | S3.2状态与边界 |
|---|---|
| `defaultOpenAIAccountScheduler.Select` advanced及legacy选择 | 已接入；真实previous/guardian/session/load_balance及weighted fallback报告层与原因 |
| OpenAI公开`SelectAccountWithLoadAwareness` | 已接入legacy；不是未接入旧入口；generic其他平台/独立直接账号选择仍nil |
| 列表/阈值/Grok/模型与限制/compact/fresh/DB | 已接入实际最终排除；列表返回前未知，成功入口按不同ID计数，后置全过滤保留正池；compact恢复、TopK/子池/未probe/WaitPlan不计排除 |
| proxy第二轮、legacy传输再评估、gwpool重选及终检、图片capability fallback | 已接入；真实新评估生成新序号和全新字段，同池终检形成所属完整副本 |
| HTTP/SSE主handler入口 | 已建立请求owner，重复登记映射模型保持选择序号；实际选择结果/错误可供后续日志链读取 |
| Responses WS建连/多turn/代理重启 | 建连选择Turn=NULL；Proxy中用连接逻辑turn隔离，保留已有Ops局部编号；同turn重试保持owner，新turn清空当前诊断及重置序号 |
| Grok Voice、Realtime | Voice及Realtime预accept实际选号循环已保存请求owner；Realtime音频帧未新接入turn producer |
| 其他平台/独立旧兼容入口/TokenCount | 未接入时nil，不生成空池或补猜层/过滤 |
| provider发送/终态、Ops事件/队列/存储/DTO/API/页面 | 未实现；S3.3起继续，不以当前请求快照替代尝试绑定或历史字段 |

结果、调度决策、结构化错误与当前状态分别深复制，错误文本及`errors.Is/As`保持；结果/决策诊断字段`json:"-"`，没有对客或用户DTO新增字段。接收边界完整合法性验证、队列大小预算和持久化兼容仍由S3.3完成。

最终定向服务17顶层/59 PASS、handler17顶层/22 PASS、既有调度回归及核心/handler race通过。fresh只读独立复核关闭重复ID统计、WS初始owner、Proxy逻辑turn和音频owner四项问题；未独立运行测试。实际WS新反例为本地HTTP bridge，native/passthrough重试后第三turn未新增同等E2E断言，复用共享映射源码/scope及既有WS回归；不宣称日志队列或权限链已验收。见[执行证据](implementation-evidence.md#s32-实际决策-producer-接入)、[验证清单](evidence/s3.2-validation.json)和[保留检查](evidence/s3.2-document-checks.json)。下一项S3.3未开始；S3.5/S3.6未提前完成。

## 9. S3.3 日志、持久化与 DTO 实际贯通

本节登记应用`ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`，personal来源S3.2完整候选`d9b06f4fe78a8588282958783d91e9966ff05231`；v1第1–7节冻结语义未修改，第8节为S3.2当时的覆盖记录。实际发送绑定保存不可变诊断及owner身份：同账号/同turn重试保留；真实重新选择重绑，新owner不能读取旧连接诊断。provider最终日志取实际最后失败事件，routing终态取所属当前评估，RequestScoped为nil。stream、事件和队列不持有原始map，0/NULL/partial均保持。

接收边界校验全部9字段、白名单/整数/数量/coverage；无效诊断省略并发出不含原值的固定信号，真实错误保留。队列自有JSON计入原256事件/512 KiB及job预算，原最近16正文和drop marker不变。migration252仅新增nullable JSONB，无默认/回填；单条/批量SQL贯通，历史NULL保持未知，严格读回坏对象不隐藏真实错误。

新字段只在管理员`/api/v1/admin/ops/errors/:id`及`/api/v1/admin/ops/request-errors/:id`单记录详情出现；普通列表和`upstream-errors?include_detail=1`裁剪顶层及事件内诊断，用户DTO白名单/直接user_id归属及admin认证接线保持。PG16和固定旧源码d9b06f4的migration runner/Ops repository往返通过，非完整服务器或生产兼容证明。

定向14顶层/36 PASS、既有归因/队列/用户、真实PG及受影响race通过；fresh只读复核首turn继承建连诊断问题已关闭。其他平台、独立旧入口、TokenCount、无新选择的连接复用仍未知，Realtime帧没有新turn producer。页面留S3.4，综合行为验收与阶段交付留S3.5/S3.6；未取得完整JWT服务器、native/passthrough完整多turn日志E2E、新CI或生产证据。见[执行证据](implementation-evidence.md#s33-贯通错误存储与-dto)、[验证](evidence/s3.3-validation.json)及[复核](evidence/s3.3-reviews.json)。


## 10. S3.4 管理员错误详情实际展示

应用`d23474171035320eda06a35d76d455d7f8d7aae4`在personal来源S3.3完整032db7982之上扩展Usage/Ops共用管理员单错误详情；第1–9节按原时点完整保留。v1快照真实0、NULL/缺失、完整/部分/未观察及原因map空/未知分别展示，partial为已知下界；选择层/原因/35个过滤码使用双语白名单，不推导可用池或回显未知原始码/额外属性。WS逻辑turn只在已知时展示，选择评估不是发送或切换次数。原phase/owner/source取当前详情，根因与载荷保持；没有增加敏感字段、用户/列表可见性、迁移或实际调度行为。

详情与关联列表按show/errorId/errorType生成请求generation，关闭/切换/卸载后旧成功、错误与finally不得修改当前状态。78项定向、lint/类型/build及两组实际前端浏览器流程通过，fresh只读源码复核无确认阻断。浏览器使用合成GET API，并不证明完整JWT后端；Ops上游单记录别名仍走既有管理员GetErrorLogByID owner，关联列表仍裁剪诊断。

既有Ops深链接首次列表不加载已登记，正常页面入口通过，未扩展修复；其他平台和未接入入口、综合行为及完整多turn日志验收仍依第8/9节边界。见[执行证据](implementation-evidence.md#s34-扩展现有错误详情)与[验证](evidence/s3.4-validation.json)。下一项S3.5未开始，S3.6交付及发布/生产仍未执行。

## 11. S3.5 实际验证与开放验收风险

测试候选`3101cb41636a658dbac47e882ebdc37f34e84641`仅新增HTTP/WS测试。真实402/429恢复、native/passthrough首turn换号后多turn、native同账号首turn重建、后续未选号NULL及延后日志队列验证通过；旧/新16同oracle各25 PASS，标准费用/去重与调度/冷却/限次在选定范围一致。原S3.2/S3.3未变输入和原日志按范围复用；新覆盖和边界见[S3.5执行证据](implementation-evidence.md#s35-验证归属与行为保持部分完成既有-p1-阻断)。

额外native后续turn换号探针自然失败：第2turn的429换号误重放建连第1turn，业务载荷/响应ID错配，日志仍属逻辑turn2。pre-S3整个forwarder及三个关键分支相同，按源码对照归类既有P1；旧WS动态复现未执行。生产源码未改，完整后续换号验收未通过，S3.5保持未勾选，先另行修复当前turn安全重放再复验，S3.6未开始。simple模式WS用量不替代标准扣费；帧内隐藏429没有恢复event入口，完整JWT/付费上游/生产及passthrough后续换号未伪称完成。

## 12. S3.5 native 安全重放修复与行为复验

第1–7节v1冻结语义及第8–11节各自历史时点记录完整保留。本轮最终应用`da581c6846d1bf89926ca9730abf2290ea8b65ea`关闭第11节既有native后续turn误重放首包：只有已证明完整的当前上下文可跨账号重放，删除原账号锚点并保留当前请求模型；无法证明安全时明确停止，不使用首包兜底。输出后不换号、原同账号重试预算保持。

Ops按逻辑turn owner去重，同turn代理重启不重复，下一turn局部编号重复不漏记；原Ops Turn局部编号及routing逻辑turn表示保持。跨模型A→B后换号选号、渠道映射和请求价跟随当前B，当前尝试的诊断与费用owner一致。

native安全7顶层/24 PASS、既有native9/9、handler18/27及实际WS race4/11通过；Ops41/61、未变并发owner race22/55、既有模型/计费11/25和shared replay helpers6/32按边界复用。旧HTTP恢复/调度/标准费用对照、未变存储/权限/前端证据按原边界复用，三次fresh只读修复复核；先后确认的跨模型P1及字符串形态P2均已关闭，最终无确认阻断。新增native后续轮次换号、独立输入/完整工具上下文及跨模型映射和实际费用计算验证；WS使用simple模式的真实RecordUsage和仓库夹具，不证明实际余额扣款。标准扣费/幂等与调度对照按未变owner复用。完整JWT服务器、付费上游、生产、passthrough后续轮次换号及后续轮次同账号重建未新增运行时覆盖；隐藏WS 429未增加恢复telemetry producer，历史未知入口仍未知。

S3.5本地完成；[执行证据](implementation-evidence.md#s35-native-当前轮次安全重放修复与复验)、[验证](evidence/s3.5-native-fix-validation.json)、[复核](evidence/s3.5-native-fix-reviews.json)。S3.6未开始，发布/生产未执行。


## 13. S3.6 阶段复核与交付

阶段fresh只读复核覆盖初始cc068ceb9完整S3，后续5文件等价lint修正按未改合同复用；最终累计候选`afcc7852b36a073dfc5fc0721ae8d24b90b7a335`的现行Personal CI `38020112510`/personal-ready通过；v1冻结语义及各历史时点记录保持。当前覆盖、未验证范围、迁移251/252及发布/部署状态以[S3.6执行证据](implementation-evidence.md#s36-独立复核与阶段交付)和[交付说明](delivery.md#s3-阶段交付s36)为准；本候选未合并或上线，S4/S5未排期。
