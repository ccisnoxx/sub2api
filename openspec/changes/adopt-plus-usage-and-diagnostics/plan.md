# KIN 借鉴 Plus 功能的实施计划

- 编制日期：2026-10-09（America/Los_Angeles）。
- 状态：S0.1、S1.1–S1.4本地部分、S2.1–S2.7及S3.1–S3.6完成；S4.1–S4.3本地完成；S1.5、S4.4和S5未开始。S4.3基于personal累计候选afcc7852，在独立分支codex/plus-service-status-s43固定应用`35962a802fdc499639c9c861072f25774ccba50b`，默认关闭，尚未启用。原S3.6 Personal CI 38020112510仅证明afcc7852，草稿PR #6/#5保持原样；本次无push、PR更新、新CI、合并、镜像发布或生产部署。
- 执行入口：[任务清单](tasks.md)；当前结果见[执行证据](implementation-evidence.md)。
- 推荐顺序：S0 现有修复交付 → S1 用量计时和请求完成状态 → S2 模型价格展示 → S3 错误诊断；S4 服务状态、S5 用户协助视图按使用规模另行排期。
- 本轮仅完成S4.3：源关联/独立聚合/管理员API与展示已实现，应用`35962a802fdc499639c9c861072f25774ccba50b`；默认关闭。检查与边界见[执行证据](implementation-evidence.md#s43-独立聚合与展示实现)。下一项S4.4未开始，不自动执行。

## 1. 目标与范围

继续以 KIN 的 `personal` 分支为应用基础，选择性移植 Plus 的指标采集与展示能力，形成可独立审查、验证和撤回的小补丁。优先解决 TPS 含义不清、首字口径混淆、完整与中断请求难以区分的问题，再改善模型价格和排障体验。

本计划保持以下行为：

- KIN 的指纹默认值、出站身份、账号调度、gwpool、重试与切换策略。
- 现有 Token 计数、计费模型、倍率、余额/订阅结算、幂等与异步用量写入合同。
- 已有 API 的默认响应结构及权限；新增数据采用可选字段或明确选择的新视图。
- 现有 `first_token_ms`、`openai_ttft_mode` 和消费旧计时的统计；新口径单独采集、单独标识。
- 既有同步与个人发布流程，不合并 Plus 整条应用历史，不改为部署 Plus 镜像。

本期不引入连续断连自动封禁、内容审计扩展、计费算法替换、供应商身份配置替换、全站 TPS 排序或新的生产发布机制。客户端取消状态属于 S1 的记录范围；完整断连风控系统不属于 S1。

## 2. 已核实基线与证据边界

| 项目 | 编制时事实 | 实施要求 |
|---|---|---|
| 应用候选 | `codex/fix-version-usage-help`，HEAD `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，含未提交修复 | 开始实施前核对最新 `personal` 和候选，保留现有改动 |
| KIN 来源 | `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` | 以 `deploy/personal-source.json` 和实际来源关系为准 |
| Plus 参考 | `v0.2.14+custom.002`，固定提交 `90da415c62b94c9417d9ce2b72b1507ed22f0303` | 记录每项借鉴的文件及本地适配，避免实施期间追随移动的 main |
| 已有用量 | `output_tokens`、图片 Token、`duration_ms`、`first_token_ms`；管理员与用户复用 `UsageTable.vue` | 普通文本 TPS 可以先用现有数据；完整详情需要新增采集与持久化 |
| 旧首字口径 | OpenAI 支持 `semantic` / `visible` 设置，其他转发路径也使用 `FirstTokenMs` | 不把旧值直接标记为 Plus 式严格首 Token |
| 已有导出 | 管理员 Excel、用户 CSV，当前导出旧首字与总耗时 | S1 增加统一计算、说明与状态字段，保留原列语义 |
| 已有渠道展示 | `AvailableChannelsView.vue`、可用渠道 API、用户分组过滤及倍率展示 | S2 扩展已有入口，复用 KIN 的权限与价格解析 |
| 已有错误能力 | 已有错误阶段、来源、责任方及详情组件 | S3 增加调度快照，不重复建设错误系统 |
| 当前交付状态 | 版本提示、TPS 点击说明和旧首字文案已有本地候选与验证；本次未确认线上是否采用该候选 | S0 将本地证据和线上验收分开登记 |

本方案最初根据本地源码与固定 Plus 提交制定；S1.1–S1.4 的实际实现、兼容验证与未接入范围现已分别登记，不能据此推导其他平台完成。`main` 是维护与调度宿主，应用实施从 `personal` 出发；运行证据遵循 `docs/conventions/personal-maintenance.md` 的登记职责。旧计划内的历史进度不替代当前源码与实际运行状态。

## 3. 阶段与交付边界

| 阶段 | 交付内容 | 前置条件 | 改动范围 |
|---|---|---|---|
| S0 | 现有版本提示与说明交互修复的交付记录 | 核对现有候选 | 现有候选，无新用量迁移 |
| S1 | 严格首字详情、平均 TPS、完成状态、页面与导出一致 | S0 候选稳定，采集合同冻结 | 转发结果、用量持久化、DTO、前端 |
| S2 | 按模型聚合的可用渠道与价格目录 | S1 第一批交付后推荐执行；技术上不依赖 S1 的新字段 | 只读价格解析、权限、模型目录页面 |
| S3 | 候选池与过滤原因等错误诊断 | S1 的请求/尝试归属明确 | 调度诊断快照、错误详情、必要存储 |
| S4 | 基于真实请求的服务状态 | S1/S3 提供可靠终态；存在持续请求和状态查看需求 | 独立聚合、状态表、配置、页面 |
| S5 | 管理员只读用户协助视图 | 有实际用户支持需求 | 专用只读路由、访问审计、用户页面上下文 |

S0–S3 是推荐主线，不把它们合成一个大型发布。S4/S5 默认不进入首期；在前置条件成立后单独选择与排期。每个阶段达到自己的验收条件即可交付，不等待后续可选功能。

S1 的开发准备不依赖 S0 已经更新生产；进入合并和发布前核对现有修复与新功能的实际基础。S1 第一批交付后即可进入 S2，第二批平台适配按实际使用入口推进，不强制等待所有未使用的平台。

## 4. S1：用量计时与请求完成状态

### 4.1 推荐的数据合同

KIN 的旧 `first_token_ms` 已有设置和统计消费者。推荐保留旧字段，新增 `strict_first_token_ms`，避免同名字段在页面、统计和历史数据中混用两种口径。这是本方案有意与 Plus 不同的兼容决定；复用 Plus 前端辅助函数时，显式适配新字段。

下表字段已在 S1.1 冻结并落地。类型、历史值及事件边界以 [冻结合同](spec.md) 为准；实际第一批 owner、提交入口与限制见 [覆盖表](coverage.md)。

| 字段 | 类型与历史值 | 冻结含义 |
|---|---|---|
| `first_token_ms` | 沿用现状 | KIN 原口径，不覆盖、不回填 |
| `timing_version` | 整数，历史为 `0` | `1` 表示本记录使用已验证的新采集器；不是全局功能开关 |
| `strict_first_token_ms` | 可空整数，历史为 `NULL` | 首个有效文本、推理或工具输出的相对毫秒数 |
| `last_token_ms` | 可空整数，历史为 `NULL` | 同一计时起点下最后一个有效 Token 类输出的时点 |
| `first_output_ms` | 可空整数，历史为 `NULL` | 首个有效输出时点，包括可识别的媒体输出 |
| `first_output_kind` | 可空字符串 | 使用 `text/reasoning/tool/image/audio/compaction`，采集不到不猜测 |
| `audio_output_tokens` | 可空整数，历史为 `NULL` | 有可信统计才记录；新纯文本记录能确认无音频输出时为 `0` |
| `completion_status` | 字符串，历史为 `unknown` | 使用 `completed/client_disconnected/upstream_error/interrupted/unknown` |
| `is_complete` | 可空布尔，历史为 `NULL` | 从已观察到的终态派生；未知不等同于成功 |
| `usage_source` | 字符串，历史为 `unknown` | 标识当前实际用量的来源；upstream_final/upstream_partial/unknown |

新字段采用增量迁移，不删除旧设置，不清空旧首字聚合，不修改已应用的迁移。Plus 的迁移文件只作结构参考，不复制其编号或历史清理语句。Ent 生成代码由仓库既有生成入口产生，不手改生成文件。

### 4.2 采集与状态归属

- 新计时观察器复用各转发路径用于生成 `duration_ms` 的起点，不把排队或鉴权时间混入新时点；不能确认共同起点的路径保持 `timing_version=0`。
- 文本、推理、工具的非空有效输出可形成严格首 Token；元数据、心跳、usage、空 delta、签名、结束标记不形成首 Token。媒体首次输出单独记录。
- 非流式响应使用实际观察到完整响应内容的时点，明确其含义，不声称测到了上游首个生成 Token。
- 计时和终态属于一次转发尝试或一个 WebSocket turn；重试、账号切换、同连接的多个 response ID 不共享可变采集状态。
- 正常协议完成、上游失败、传输中断和客户端取消分别判断。一次记录同时观察到取消与上游完成时，依据真实事件顺序和客户端输出生命周期冻结规则，不能只看 HTTP 状态码或是否扣费。
- handler/转发 owner 形成不可变快照，随现有 ForwardResult 和用量任务传递；异步 worker 不再读取已结束的请求上下文。
- 首期只增强已有用量行和已有错误记录。没有用量的失败请求不伪造 Token、费用或新的成功用量行；记录失败或队列失败明确保留错误。
- `usage_source` 只描述已有 Token 数据来源，不新增估算、不影响扣费。没有来源证据时保持 `unknown`。

### 4.3 接入批次与可见行为

第一批覆盖 OpenAI Responses HTTP/SSE 和 WebSocket turn，这是当前 Codex 使用链路的实施起点。第二批按实际生产入口接入 Chat Completions、Anthropic、Gemini/Antigravity 及转换链路；逐条登记已覆盖路径，未接入路径保持旧口径。

新增字段部署后，前端按记录选择展示：有 `timing_version=1` 的记录显示“首 Token”及首次输出详情；旧记录继续展示“首字（旧口径）”，使用已有说明，严格首 Token 详情显示未采集。新旧记录可以同时出现在同一表格。

平均 TPS 使用现有总耗时：

```text
平均 TPS =（输出 Token - 图片输出 Token - 音频输出 Token）× 1000 / duration_ms
```

普通文本无需首字或末字时点。媒体统计不完整、输出或耗时无效时显示 `—` 并给出原因；明确纯文本的旧记录保留现有适用范围，不能把未知音频字段一律解释为已验证的零。live、探测、gwpool 降级、纯媒体和不适用的 compaction 记录不混入文本 TPS。

不完整请求若 Token、媒体拆分和耗时仍可用于计算，可显示平均 TPS，同时标明“部分响应”。它不是可见文本的纯生成速度，也不能证明 Fast 生效。

界面保留用户要求：TPS 和首字各有独立圆圈按钮；点击或 Enter/Space 打开，Escape/点击外部关闭；打开另一个说明时关闭前一个。删除数值/标签上的原生悬停说明，描述与对应字段一致。

当页面与导出都使用计算逻辑时，提取统一的 `usageTiming.ts`。用户 CSV 和管理员 Excel 增加平均 TPS、不可用原因/部分响应说明、新计时字段和完成状态；保留旧列含义，导出原始数值，未知保持空值或明确未知文本，不写成 `0`/成功。

### 4.4 主要代码位置

| 责任 | 已有位置或新增建议 |
|---|---|
| 计时观察器 | 新增建议 `backend/internal/service/stream_output_timing.go` |
| 转发结果 | `backend/internal/service/gateway_service.go`、`backend/internal/service/openai_gateway_service.go`；相关 HTTP/SSE 与 `backend/internal/service/openai_ws_forwarder_*.go` |
| 用量快照构建 | `backend/internal/service/gateway_usage_billing.go`、`backend/internal/service/openai_gateway_usage.go`、相应 handler 用量提交入口 |
| 领域与持久化 | `backend/internal/service/usage_log.go`、`backend/ent/schema/usage_log.go`、`backend/internal/repository/usage_log_repo*.go`、`backend/migrations/` 新增迁移 |
| API | `backend/internal/handler/dto/types.go`、`backend/internal/handler/dto/mappers.go`、既有用户/管理员用量接口 |
| 前端数据与计算 | `frontend/src/types/index.ts`、相关用量 API 类型；新增建议 `frontend/src/utils/usageTiming.ts` |
| 页面与导出 | `frontend/src/components/admin/usage/UsageTps.vue`、同目录 `UsageTable.vue`、`frontend/src/views/{admin,user}/UsageView.vue`、双语 dashboard 文案 |

上表使用仓库相对路径；通配符表示同一职责的现有文件集合。新增建议文件尚不存在。

### 4.5 S1 验收条件

新请求能够解释首 Token、首次媒体输出、总耗时和是否完整；历史请求不被伪造为新口径。管理员和用户页面、CSV/Excel 对同一记录给出一致的 TPS 与状态。HTTP 重试和 WS 多 turn 不串计时；新增字段经过实际 SQL 写入/读取及 DTO 验证，单条、批量、幂等和 best-effort 路径不漏字段。相同既有用量输入的 Token、费用、倍率和余额/订阅变化保持一致。

S1 第一批完成只能登记“OpenAI Responses HTTP/WS 已覆盖”，不能宣布所有平台完成。第二批完成时附采集覆盖表与明确不适用入口。

## 5. S2：模型与价格展示

S2.1 已完成，查询选择、权限矩阵、目录外层、空值及个人倍率失败状态以[冻结合同](catalog-contract.md)为准。S2.2 已在 personal 候选实现[权威价格服务](pricing-contract.md)，应用提交 `e82287300d1b7cc625295c5307ebaa83c707c019`。S2.3 应用 `49c209a77059d2927f78bbd31c1dd479b509dc6f` 已实现目录聚合、DTO 和单个精确 `view=catalog` 查询分支；默认请求仍返回旧渠道数组。目录复用 KIN 分组模型白名单、授权与同次渠道配置快照，旧数组保持原合同；S2.4应用`b8cf49ca25007cbc23338a9443b71df4c0411155`已接入页面、分组报价与单位展示，本地检查完成；S2.5验证候选`3722c48ccdc0e4468dbc5c6ebb13584624c566d6`已通过真实JWT/PG16/Redis权限与64组HTTP/计费对账，复用原有效证据，三项测试保护缺口已修正并独立关闭；S2.6已在S2.5候选完成两组新浏览器查看/无权限流程、原证据校验复用与fresh只读复核，未改生产源码；S2.7最终累计候选`88156f09fcf980a771a8aab570f0dbaec5de25fb`已建立草稿PR #5，现行Personal CI `37982040213`及personal-ready通过；未合并/发布/部署。

扩展现有 `/api/v1/channels/available`，推荐以显式 `view=catalog` 请求模型目录，默认请求继续返回旧结构。先执行 KIN 现有用户分组、订阅/专属分组、渠道状态和平台过滤，再聚合模型卡片，禁止聚合前泄漏其他分组报价。

模型卡片按平台与模型名称聚合，保留各分组报价，支持查看上下文阶梯、服务档位和媒体计费单位。未知价格、明确零价与正常价格分别展示；复杂价格所需上下文不足时展示对应规则或参考价，不能伪造单一精确报价。

报价只复用 KIN 的模型价格解析与倍率组合，不引入 Plus 的第二套计费公式。若某个字段在 KIN 暂无可靠解析入口，先明确标为未支持，不为完成目录修改扣费逻辑。模型目录不承诺实时健康，也不把最低展示价承诺为路由必选价。

涉及 `available_channel_handler.go`、`channel_available.go`、`channel_service.go`、现有价格 resolver、`AvailableChannelsView.vue` 及对应 API 类型。新增只读目录服务按实际复用需要决定，不改账户调度。

验收重点：默认接口兼容；有权限与无权限用户只能看到各自可见模型；展示倍率只应用一次；给定相同计费上下文，目录报价与现有扣费解析一致；零价和未知可区分。用户在分组、模型详情之间的完整查看流程做一次定向浏览器验收。

## 6. S3：错误诊断增强

S3.1 [冻结合同](routing-diagnostics-contract.md)的v1语义保持。S3.2应用`cee1e908261c68880040b740aae8054410e03070`已接入OpenAI主调度advanced/legacy、渠道限制、Grok/阈值/compact、gwpool及图片fallback，并建立HTTP/SSE请求与WS逻辑turn owner、完整不可变结果/错误快照。公开OpenAI `SelectAccountWithLoadAwareness`纳入legacy；其他平台、独立旧入口与TokenCount未接入时保持未知。实际覆盖、验证和独立复核见[执行证据](implementation-evidence.md#s32-实际决策-producer-接入)及[覆盖补充](routing-diagnostics-contract.md#8-s32-实际-producer-覆盖与验证)。S3.3应用`ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`已贯通发送/终态、Ops队列、nullable JSONB及管理员单记录DTO；列表和用户白名单不扩展。[本轮证据](implementation-evidence.md#s33-贯通错误存储与-dto)记录真实PG、固定旧源码往返和独立复核边界。S3.4应用`d23474171035320eda06a35d76d455d7f8d7aae4`已扩展共用管理员详情。S3.5最终应用`da581c6846d1bf89926ca9730abf2290ea8b65ea`完成native安全重放、Ops逻辑turn去重及跨模型选号/请求价修复与复验；历史P1及新复核问题均已关闭，S3.6已完成独立复核与阶段交付，见[阶段证据](implementation-evidence.md#s36-独立复核与阶段交付)；见[本轮证据](implementation-evidence.md#s35-native-当前轮次安全重放修复与复验)。

在既有错误详情内增加可选 `routing_diagnostics`，由调度决策 owner 提供一次完整快照。拟记录选择层、选择原因、已观察的候选池数量、已知过滤数量及稳定过滤原因，不从自然语言错误信息推测。

`candidate_pool=0` 表示确实观察到空池；缺失/NULL 表示未知。账号选择前被渠道定价/模型规则拒绝时，不写成空候选池。传输、身份等独立 producer 只补自己的信息，不复用前一次选择的数量。

快照按请求和尝试归属传到 `ops_error_logger.go`、错误存储与现有 `OpsErrorDetailModal.vue` 等详情 owner；若需要持久化字段，使用新增可空 JSONB 列和本地迁移。保持当前冷却、重试、账号切换和错误权限；详情不含凭据、完整请求体或原始敏感头值。

验收重点：空池与未知可区分；过滤原因来自实际决策；重试与 WS 后续 turn 不覆盖前次诊断；同样故障的调度结果、重试次数和计费行为不变。已有错误归因测试作为回归依据，新增测试只补尚未覆盖的快照合同。

## 7. S4/S5：按需求排期

S4.1已确认持续真实请求与仅管理员全站需求；S4.2已完成[独立规格](service-status-contract.md)，定义终态/归属、排除与未知、水位/样本、故障/恢复、保留/开关及管理员可见范围。旧看板只佐证流量；KIN两表计费/错误ID差异要求S4.3增加可空监控关联元数据，不能直接套用Plus按request_id归并。S4.3已在独立personal候选`35962a802fdc499639c9c861072f25774ccba50b`实现，[执行证据](implementation-evidence.md#s43-独立聚合与展示实现)登记；默认关闭，S4.4未开始。

S4 适用于持续有请求、需要查看全站服务状态的部署。参考 Plus V3，以真实请求终态关联用量和错误；终态成功覆盖中间重试错误，排除客户端取消和用户输入/权限问题。无近期请求、样本不足、聚合水位过期显示未知；故障恢复必须有新请求证据。独立表与配置保留原 V1/V2。S4.1已选定仅认证管理员全站可见，普通用户/匿名不新增S4入口；后续规格和实现须服务端落实该边界，不直接复制 Plus 对所有登录用户开放全站状态的权限设计。

S5 适用于需要帮助其他用户排查问题的部署。管理员保留自己的身份，在专用 GET 路由读取指定用户的受限数据；后端无对应写入入口，前端隐藏修改操作。审计记录真实管理员、目标用户、路径与结果；切换目标用户时取消或忽略旧响应。不仅复制用户页入口，不默认暴露完整 API Key 或其他非排障必需数据。

S4/S5 都先记录具体需求、数据可见范围和启用条件，再形成独立实施规格与任务。它们不阻塞 S1–S3 的交付。

## 8. 验证与独立复核

验证按实际风险选择，以下是对应边界，不是每阶段必须累加的检查清单：

| 边界 | 最小有效验证 |
|---|---|
| TPS、首字与状态展示 | 辅助函数及组件定向测试；管理员/用户页面与导出同记录核对 |
| 流式采集、重试与 WS | 合成协议事件测试；metadata/空 delta、部分输出、取消与完成、多 response ID/turn 的交错 |
| 新字段持久化 | 临时 PostgreSQL 的真实迁移与 SQL 读写；覆盖实际单条/批量/重复写入分支 |
| 计费保持原样 | 复用现有计费与幂等用例，补充元数据变化不改变结算结果的有意义合同 |
| 价格目录与权限 | handler/resolver 定向合同测试；已授权与未授权分组、零价、未知价、倍率、阶梯价格 |
| 错误快照 | producer 到日志/DTO 的定向测试，保护空值、重试归属和固定原因 |
| 浏览器交互 | 关键页面的点击与键盘流程、窄屏和双语；不无依据扩展浏览器矩阵 |

前端脚本使用实际 `package.json` 的 `pnpm test:run`、定向 ESLint、`pnpm build`。Go 使用相关包与测试名筛选；测试名在新增合同明确后确定，不提前编造命令。Ent 和迁移使用仓库现有工具。文档阶段不运行应用构建或测试。

S1 的持久化与异步快照、S2 的权限与报价、S3 的诊断归属，以及未来 S4/S5 的可见范围，进入相应候选后需要独立只读复核。复核针对实际 diff 和具体风险；本计划编写不等于已获得独立代码复核。完整 CI 只按仓库必需门禁对稳定候选运行，不为中间文档或小修重复触发。

## 9. 交付与回退

每阶段从最新 `personal` 建立 `codex/` 功能分支，记录 Plus 固定来源、本地适配和最终候选 SHA。S1 建议按“数据合同与迁移 / 采集与快照 / 页面和导出”形成可审查提交，依赖按顺序合入；不单独部署会读取不存在字段的消费者。

迁移采用加字段方式，历史未知不回填。先在临时数据库验证旧版本是否能够在扩展后的 schema 上运行，并检查原始 SQL、生成代码及实际部署工具的兼容性；只有验证成功才声明可仅回退应用镜像。回退不删除新列，不修改 migration ledger。S4 独立表和 S5 新路由也需要各自的回退说明。

合并、镜像发布与生产更新沿用既有 Personal CI、来源 SHA 和固定 digest 合同；实际操作以当前授权为准。本轮不触发外部写入或生产动作。包含迁移的阶段需在部署前准备数据库备份与恢复方法，不能沿用无迁移的 S0 兼容证明。

每项完成记录任务编号、最终 SHA、实际检查与结果、覆盖平台/入口、未执行检查、独立复核结果及发布/部署状态。只有实际获得证据才能勾选实现或线上验收任务；计划编制、代码完成、检查通过、发布、线上生效分别登记。

## 10. 固定参考来源

- [Plus 用量计时合同](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/USAGE_TIMING.md)：指标与未知数据边界。
- [Plus 计时辅助函数](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/frontend/src/utils/usageTiming.ts)：计算、可用性与导出复用参考。
- [Plus 用量 schema](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/backend/ent/schema/usage_log.go)：字段参考；KIN 的严格首字独立字段及音频 NULL 语义是本方案的适配。
- [Plus 模型目录](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/AVAILABLE_CHANNELS.md)：目录与价格展示。
- [Plus 错误诊断](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/ERROR_REQUEST_DIAGNOSTICS.md)：结构化选择事实。
- [Plus V3 服务状态](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/CHANNEL_MONITOR.md)：终态聚合与未知状态。
- [Plus 用户协助视图](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/USER_SUPPORT_VIEW.md)：真实管理员与只读目标用户的职责分离。

引用只说明借鉴来源。实施以 KIN 本地合同为准，保留适用的代码来源与项目许可声明；S1 第一批的实现声明仅限覆盖表与执行证据；其他阶段的拟议入口和文件不宣称已在当前产品实现。
