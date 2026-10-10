# S4.2 管理员真实请求服务状态独立规格

- 日期：2026-10-09（America/Los_Angeles）。
- 状态：S4.2规格冻结；本文件定义S4.3/S4.4的实现与验收合同，S4.3本地应用`35962a802fdc499639c9c861072f25774ccba50b`已实现代码、表、API与页面；独立开关代码默认关闭，S4.4进行中；hostdzire隔离PG18/真实JWT/浏览器子任务已完成，只有测试实例启用，尚未生产启用。
- 需求依据：[S4.1](service-status-demand.md)。第一批仅认证管理员查看全站聚合状态，普通用户和匿名不新增入口。
- 应用依据：personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`上的累计候选`afcc7852b36a073dfc5fc0721ae8d24b90b7a335`，来源KlN `.5/c7aacf5d`；源码读取自`/Users/sc/.codex/worktrees/plus-routing-storage-s33/sub2api-kin`。main来源文档树仅登记规格。
- 固定借鉴：[Plus CHANNEL_MONITOR.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/CHANNEL_MONITOR.md)。借鉴终态优先、匿名分钟事实、水位与故障事件；KIN的管理员权限、计费ID、逻辑turn、旧首字及独立开关按以下合同适配。
- 执行与检查：[执行证据](implementation-evidence.md#s42-独立规格编制)、[来源清单](evidence/s4.2-candidate.json)、[文档检查](evidence/s4.2-document-checks.json)。

## 1. 目标、边界与S4.2编制时源码事实

管理员需要回答：在指定平台、分组、请求模型的近期真实请求中，最终成功或服务失败的证据是什么，样本是否足够，聚合是否新鲜，故障是否得到新请求的恢复证明。页面文案使用“本部署已观察请求的服务状态”；不宣称供应商全球健康、未来调用成功率或根因已定位。

全站表示跨所有用户读取当前未删除且`active`的分组；平台范围受S4独立配置限制，并明确显示。管理员查看全站聚合不授予其他主体模型调用权限。状态不参与调度、冷却、账号切换、重试、封禁或收费。

| personal候选事实 / 权威位置 | S4决定 |
|---|---|
| `channel_monitor_v2_aggregation.go`使用`actual_cost > 0`计成功，错误表独立按request_id归并。 | 不复用V2成功计数或把费用/Token当终态；另建S4规范化与匿名聚合。 |
| `gateway_usage_billing.go:resolveUsageBillingRequestID`优先形成`client:`/`local:`等计费键；`openai_gateway_usage.go:RecordUsage`在WS上可改用上游ID。Ops保存request_id/client_request_id，两表字符串不保证同义。 | 不直接按现有request_id跨表关联，不修改计费ID和原幂等键。S4.3需新增可空监控关联元数据。 |
| `openai_ws_routing_diagnostics.go`与`routing_diagnostics.go`维护连接逻辑turn；Proxy重试的局部turn会重置。诊断对象不是每条成功用量都具备。 | 每个准入逻辑turn单独归属，Proxy重建保持当前归属；不以连接ID、局部turn或可空诊断对象代替监控键。 |
| `ops_error_logger.go`保留恢复后的上游尝试，2xx恢复行与最终SSE/WS失败有不同语义；日志可能被配置过滤或队列丢弃。 | 显式区分attempt/terminal；不能根据“存在错误行”或“没有错误行”推断请求终态。 |
| [S1合同](spec.md)及[覆盖表](coverage.md)仅证明原生OpenAI Responses HTTP/SSE/WS第一批；HTTP失败原本不新增用量行。 | 成功复用可信用量终态；无用量的最终失败依赖错误owner。未接入入口、历史记录及缺失事实保持未知，不伪造用量行。 |
| `channel_monitor_v2_error_taxonomy.go`混用状态码与自然语言匹配，401/429/额度文本不能单独区分用户与供应商责任。 | S4独立白名单按实际错误owner/稳定原因判断；无法分辨责任为未知，不搬用V2字符串分类作为排除证据。 |
| `setting_public.go`只接受v1/v2；两模式已有用户路由与权限。 | S4使用独立管理员入口和开关；不改写旧mode，不替换`/monitor`，不停止或重定义V1/V2。 |

第一批只判定可用性。TTFT、TPS和性能下降不参与S4健康或事件判定：KIN旧`first_token_ms`不是严格首Token，`strict_first_token_ms`的非流式值又是完整响应观察时点，当前WS DTO不能可靠区别两种观察方式。后续若增加性能状态，须另行冻结有效采样口径；本轮不新增延迟阈值。

## 2. 请求/turn关联与源记录合同

### 2.1 一个实际生命周期，一个监控键

S4.3在既有HTTP请求与准入WS逻辑turn owner创建服务端随机UUID `observation_key`。HTTP的全部选号/发送尝试共用一个键；WS每个已准入逻辑turn有新键，同turn重建或换号沿用原键。新的HTTP请求即使携带相同客户端header也必须用新键。该键与计费、API Key、请求/response ID独立，不由客户端指定；建连前拒绝是单独生命周期，不与后续turn合并。

最小新增源字段为`usage_logs.service_status_observation`与`ops_error_logs.service_status_observation`的nullable JSONB；领域值、显式SQL、Ent维护源/正常生成、批量/幂等/best-effort与Ops队列贯通按现有责任完成。它们是源日志元数据，匿名S4表不保存键。历史NULL不回填；不得修改原request_id、唯一约束、扣费去重或制造新的计费用量行。新增迁移身份在S4.3核对占用后确定，不能预占编号或修改已应用迁移。

| 元数据字段 | 类型 / 合同 |
|---|---|
| `schema_version` | 整数，第一版固定1；不支持的版本归为未知。 |
| `observation_key` | 服务端生成UUID，同一实际请求/逻辑turn内稳定。 |
| `logical_turn` | 可空正整数；HTTP/建连前为NULL，WS使用连接逻辑turn，不能借用Proxy局部turn。键本身已隔离turn，此字段用于一致性检查。 |
| `event_role` | `attempt`或`terminal`；恢复的上游尝试只能为attempt。 |
| `terminal_kind` | terminal使用`completed/client_disconnected/upstream_error/interrupted/user_rejected/unknown`；attempt为NULL。`user_rejected`仅用于明确用户侧拒绝，不替换S1枚举。 |
| `observed_at` | 服务端UTC RFC3339时点；terminal为实际最终生命周期冻结时刻，attempt为该尝试结束时刻。不用日志入库时间代替终态发生时间。 |
| `reason_code` | 可空的下节稳定原因码；与实际owner同步形成，不读取以后变化的请求ctx，也不从原始正文推测。 |

用量的terminal_kind必须与既有`UsageTiming`冻结终态一致；用户拒绝、无用量失败由handler/Ops终态owner提供。已有S1未知终态不会因接入S4改为完成。观察到`completed`之后的连接关闭保持完成；取消早于完成保持客户端取消；S4不重新排列S1协议事件。源元数据提交前深复制，异步worker只消费不可变值。旧DTO、用量导出、错误列表和用户错误详情不扩展该内部对象。

元数据记录可随部署后的真实请求形成，S4停用时聚合不读取它；启用区间由聚合配置筛选。监控元数据失败不得阻断原请求或改变扣费/重试，但必须保留显式错误；已知写入/队列失败须让监控数据完整性标记为未知，不能吞错后宣称健康。S4.3必须核对实际可观测的写入失败入口；无法观测的完全缺失日志属于有界日志限制，不声称端到端完整采样。

### 2.2 规范终态与优先级

关联只在有效v1元数据的同一observation_key内发生。不同键绝不因模型、时刻、账号或客户端ID相近而合并。键相同但logical_turn不一致、不可协调的两个最终终态或归属冲突时，该生命周期为unknown并登记固定内部原因；不任意选择“最新一条”掩盖冲突。

1. 先移除探测/辅助尝试和明确不适用记录：`is_count_tokens=true`、`request_type=4`策略阻断、6猎手探测、7 gwpool降级尝试；Live=5第一批不支持。它们不计成功或服务失败，也不构成恢复证据。普通真实compaction若有可靠终态则使用同一规则。
2. 同键有可信用量终态时，使用该最终终态，attempt错误仅作过程背景，不能重复计故障。`completed`且`is_complete=true`为成功，包括零费用、零输出Token。矛盾值、未知或缺失完成证据为unknown；不以`is_complete=true`单独升级历史unknown。
3. `client_disconnected`终态覆盖同键中间错误，归为excluded；取消之后drain所得上游完成不覆盖取消。已完成之后关闭仍为success。
4. `upstream_error/interrupted`终态先按同键、同最终尝试的可靠reason_code判断用户侧排除；明确服务侧为failure；责任不明为unknown。先前失败尝试的原因不能替代最终原因。
5. 没有可信用量终态时，仅`event_role=terminal`的Ops最终事实参与分类：completed仍须可信完成事实；user_rejected/客户端取消为excluded；明确服务失败为failure；其余unknown。只有attempt或2xx恢复行、没有最终事实时为unknown，不能报故障或成功。
6. 无有效关联元数据的普通源行使用表名+日志ID作为独立观察，不与其他行猜测关联，只计unknown。已明确的辅助/策略记录仍可按步骤1排除。没有任何源行不伪造观察，状态以no_recent_requests表示。

一个规范生命周期只贡献一个`success/failure/excluded/unknown`，成功和排除都优先于中间attempt错误；真正冲突的terminal不是重试恢复。真实终态若由同一owner写到两表，observed_at/kind/归属一致时只计一次。被重算替换的旧观察不是新请求证据。

## 3. 用户侧排除与服务失败

原因码由S4内部分类owner管理一个白名单，producer只传已确认的事实。现有`error_owner/error_phase/error_source/error_type`及诊断稳定码可作为输入，但不能仅凭HTTP状态、自然语言或所有过滤候选耗尽推导责任。只在实际拒绝/失败owner能证明时生成以下码；不引入第二套网关决策。

| 结果 | 稳定reason_code | 必要证据 |
|---|---|---|
| excluded | `client_cancelled` | 客户端生命周期明确先取消；不是泛化的服务端context超时。 |
| excluded | `user_invalid_request`, `user_context_limit`, `user_model_unsupported` | 最终请求输入、上下文或请求模型合同拒绝；模型本应受支持但库存不足不能用此码。 |
| excluded | `user_group_access`, `user_authentication`, `user_quota`, `user_rate_limit` | 本部署用户权限/鉴权/额度/用户限流owner明确拒绝。 |
| excluded | `content_policy` | 明确内容策略拒绝；不把普通403归为内容策略。 |
| failure | `provider_authentication`, `provider_quota`, `provider_capacity`, `provider_5xx` | 最终上游账号身份、供应商额度/容量、上游5xx事实；不是用户认证/余额/限流。 |
| failure | `account_pool_unavailable` | 面向已支持请求模型的实际最终空池/服务能力耗尽；部分过滤计数本身不证明原因。 |
| failure | `service_timeout`, `service_transport`, `service_internal` | 最终服务截止时间、传输/缺失终态错误或内部服务失败；客户端主动取消另行排除。 |
| unknown | `unclassified`, `terminal_missing`, `linkage_unavailable`, `terminal_conflict`, `scope_unavailable`, `unsupported_entry` | 无法证明上述合同或缺少归属；不向成功/故障或用户排除填默认值。 |

相同401、402/额度信息、429分别可能属于用户或供应商；S4.4须有成对反例保护归因。S1未接入的平台/转换链路不能因HTTP 200或已扣费算成功；原生OpenAI Responses以外范围仅在获得等价终态owner证据后扩展，S1.5不由本规格提前完成。

## 4. 维度、聚合、水位与观测区间

叶子范围为`platform + group_id + requested_model`。普通分组使用其有效平台；复合分组使用此次实际最终账号平台，没有账号事实时保留未知，协议路径/模型名不能猜平台。模型优先requested_model，历史model只作已记录字段回退；缺失模型建立unknown范围，不能悄悄删除。重试跨账号/平台只将最终终态贡献给最终有效范围，中间失败不分别形成叶子故障。

没有有效分组、已删除/停用分组的源行不进入对外维度。管理员可见当前active组ID/名称、已启用具体平台、已观察的模型；不暴露账号或上游模型映射。无法归属的近期观察以全站数据完整性提示反映，不能偷偷忽略后把摘要报正常。仅用于该提示的匿名计数不暴露主体。

| 时间与容量 | 冻结值 / 含义 |
|---|---|
| 时区、分钟边界 | UTC，区间一律左闭右开。终态按observed_at分桶；created_at用于限定源日志扫描，不当作恢复时间。 |
| 运行周期 / 单轮预算 | 60秒；45秒取消预算。一个成功提交至多推进一次事件观测。 |
| 当前窗口 | 以已提交`observed_through`为右边界的5分钟完整窗口，同时返回该范围及实际请求证据时间。 |
| 最近重算 | 每轮最近10分钟完整分钟，幂等替换该区间事实；候选键仅有界关联额外90分钟，不扫描全历史。 |
| 过期 | 服务端now距observed_through大于180秒，或没有成功提交水位，当前状态unknown。时钟倒退/未来水位为invalid_watermark，同样未知。 |
| 单轮上限 | 规范观察最多50,000；先检查上限，超限整轮回滚并报`SERVICE_STATUS_OBSERVATION_LIMIT`，不静默截断。 |
| 历史 | 24h/7d/30d；只使用已实际观测区间。首启/重启不全量回填，停用区间保持空白。 |

新启用/重新开启某平台时，服务端持久化下一个完整分钟为observation_start，并重置连续确认游标；不补停用请求。每轮重算与关联都受本次启用区间限制，不能删除停用前事实或填停用期间的分钟。长期请求按最终observed_at归属，入库迟到在10分钟重算范围内可修正；超出范围/关联边界的迟到记录不自动回填，返回有界观测说明。配置/进程重启不丢失启用区间。

水位、分钟事实、叶子当前状态和事件在同一事务提交，使用S4专属事务级领导锁；V2锁/水位不共用。失败回滚保留上次成功水位，记录固定stage/code，禁止输出原始错误正文。客户端看到的查询时间不能刷新水位；空扫描可提交“该区间已扫描”的水位，但没有请求仍是未知。配置读取失败、数据库异常、已知源写入缺口均不得返回缓存中的operational作为当前结论。

有界日志说明始终保留：水位证明完成了指定范围的聚合，不证明所有请求都被写入源日志。没有源记录的请求、过滤/丢弃的Ops日志或范围外迟到记录不能由S4重建；健康文案只能描述已观察样本。

## 5. 当前状态、样本与故障/恢复

### 5.1 可用性计算

`qualified = success + failure`，`error_rate = failure / qualified`；excluded、unknown均不进入分母，qualified=0时比例为NULL，不能写0%失败或100%成功。excluded不推进故障或恢复计数。unknown单独记录；当前叶子unknown>0时健康状态为unknown，保留已确认的未结束事件，以免在未知终态中宣布正常或精确故障率。计算中的已知样本比例仅用于内部分析；DTO的当前/历史点遇到unknown、样本不足或覆盖缺口时比例为NULL，同时返回计数和原因。

默认阈值是初始运营选择，参考真实流量需求和固定Plus思路；不是从S4.1的7 RPM或模型榜推导出的统计保证。S4.4及实际启用前可依据目标入口的充分样本调整，阈值变化遵循下一节配置版本规则。

| 配置 | 默认 | 合法值 |
|---|---|---|
| `minimum_samples` | 5 | 整数1–10,000。 |
| `warning_error_rate` | 0.05 | 有限数，0 < value < 1。 |
| `outage_error_rate` | 0.90 | 有限数，warning_error_rate < value <= 1。 |
| `abnormal_windows` | 2 | 整数1–10；具有新请求证据的异常观测次数。 |
| `recovery_windows` | 3 | 整数1–10；具有新请求证据的正常观测次数。 |

按优先级判当前叶子：停用不参与摘要；无水位/过期/完整性缺口→unknown；无近期观察→unknown/no_recent_requests；只有排除→unknown/no_qualified_requests；含未知→unknown/terminal_unknown；qualified少于minimum_samples→unknown/insufficient_samples；达到中断阈值→outage；达到异常阈值→degraded；其余→operational。等于阈值算达到。当前样本判定可以先显示异常，事件尚须连续证据确认。

平台与全站摘要按叶子事实确定，结果不受遍历顺序影响：有异常优先显示异常；只有所有相关叶子均outage且无unknown时才显示outage，否则有outage/degraded为degraded；无异常但含unknown或等待数据的未结束事件为unknown；全部相关叶子operational且没有未结束事件才为operational。有未结束事件正在恢复时显示recovering，不称已确认正常。范围为空为unknown，不是正常。

相关叶子包括保留期内已观察范围和全部未结束事件范围；没有任何观察的已启用平台/active组保留“尚无请求证据”提示。低流量模型的未结束事件或未知状态不能被高流量健康模型的加权成功率掩盖。平台历史合计比例另行展示，不能反过来代替叶子当前状态。

### 5.2 事件证据与状态机

事件唯一范围为platform/group/model，同一范围至多一个未结束事件。维护`last_evidence_at`、连续异常/恢复计数、配置版本与观测区间；不保存请求键。一次可推进的观测必须含此范围`observed_at > last_evidence_at`的新增合格请求，且新鲜、样本足够、无unknown。相等时间只保守地不推进；重复扫描、补写旧终态、查询页面、时间流逝均不推进。

| 条件 | 事件动作 |
|---|---|
| 无未结束事件，具有新合格请求的异常窗口 | 异常计数+1，达到abnormal_windows建立detected；此前显示当前异常但不补造故障事件。异常等级变化不打断连续异常。 |
| 未结束事件再次获得新异常证据 | ongoing，更新实际证据时刻；清空恢复计数。 |
| 未结束事件，获得新正常请求且窗口正常 | recovering并+1；新成功的observed_at必须晚于该事件最后异常证据和上一恢复证据。达到recovery_windows才resolved。 |
| 无近期合格请求、仅排除、unknown、样本不足或水位过期 | awaiting_data；恢复和待确认异常连续计数清零，保留未结束事件。不能把故障终止时间设为当前时间。 |
| 同一批仍新鲜、足够的样本重复扫描，无新的observed_at | 不增加、清零或推进连续计数/事件阶段；事务重算不是新证据。若旧失败仅因移出窗口使窗口从异常转正常而又没有新成功，则改为awaiting_data并重置恢复计数。 |
| recovering时有新异常 | 回到ongoing，撤销恢复确认；先前恢复样本不能累积到下一次恢复。 |
| 停用平台/总开关或active组失去可见性 | 冻结未结束事件；不resolved、不推进。重新开启后等待新启用区间内的证据，计数重新积累。 |
| 调整阈值 | 重新计算当前指标并重置连续计数/证据基线；旧事件保留，新请求才可开始恢复，不能仅靠配置放宽结束事件。 |

故障旧样本移出5分钟窗口导致指标转好，不构成恢复。迟到的较早成功终态可修正历史和当前统计，不能作为发生在最后异常之后的新成功。已经错误计入的旧故障被迟到终态修正时保留“证据修正/等待新请求”阶段，不虚构服务已恢复；本期不引入自动删除已确认事件或已结束历史的追溯改写。事件描述仅说明检测/持续/恢复观察，不声称正在人工修复或根因已知。

## 6. 独立存储、保留与开关

S4.3建立独立`service_status_config`单例、`service_status_facts_1m`、`service_status_scope_states`、`service_status_incidents`、`service_status_watermark`。配置拥有enabled、平台集合、阈值、version与服务端启用区间；事实拥有每分钟scope及四类计数；状态拥有事件连续证据游标；事件拥有阶段与时间；水位拥有成功聚合范围/版本及安全的新鲜度原因。没有复用V2表或事件状态。

匿名表白名单：platform、group_id、requested_model、UTC分钟/证据时点、success/failure/excluded/unknown计数、健康/阶段、配置版本、确认计数与固定数据缺口码。事件最多32个阶段更新且保留首次detected；事件ID是独立事件身份，不是请求ID。禁止存用户/API Key/账号、请求/response/observation_key、上游地址、原始错误/请求/响应、Token、费用或余额。源日志的新增元数据受既有源日志保留职责管理，不复制到S4事实表，不改变原源日志保留设置。

- 匿名分钟事实保留31天；历史视图最长30天，空白/unknown区间不填正常。
- 已resolved事件从resolved_at起保留31天；未结束事件保留至有真实恢复证据，不能TTL自动解决。无未结束事件且31天无观测的叶子状态可删除。
- 普通快照最多200条resolved事件，按时间稳定排序；平台/active分组/时间过滤先于limit。全部未结束事件及其计数不受200条上限影响。
- 停用停止源扫描和事件推进，保留当前未结束事件及历史；独立保留清理仍按上述TTL执行，不能将“保留历史”解释为永久不清理匿名旧数据。

初始`enabled=false`，具体平台目录来自KIN维护的实际平台常量，首期默认配置platforms=[openai]但总开关关闭。扩展平台不默认自动开启；无可信终态的平台即使由管理员选中也保持未知。platforms=[]明确表示无监控平台，enabled=true时只执行保留清理，页面显示“暂无启用的监控平台”。不新增部署环境变量、不写channel_monitor_enabled/mode、不自动改变V1/V2。

配置完整更新必须带version；过期返回409 `SERVICE_STATUS_CONFIG_CONFLICT`，非法阈值、未知/重复/复合平台返回400，缺字段不默默补默认。管理员认证、现有管理审计和面板限流继续生效。配置更新与聚合对同一配置行加锁，保存成功前旧配置事务已完成；随后新版本才可推进。启用区间仅服务端写入，不接受客户端覆盖。配置读取失败时停止S4扫描和当前健康结论，保留明确错误。

迁移仅新增表/可空源字段，旧版本继续原查询；兼容性必须在S4.4用固定旧候选与真实PostgreSQL验证后才能声明。回退仅关闭S4/回退应用，保留新增表、列及migration ledger；禁止删列、清旧历史或改已应用迁移。正式生产前另核实真实运行树、备份/恢复和固定镜像digest，本规格不替代这些证明。

## 7. 管理员API、展示与隐私

以下合同已由S4.3本地候选实现。所有S4路由挂在现有admin鉴权与审计之下，没有`/api/v1/service-status`用户版本，也不接入用户协助视图。

| 路由 | 合同 |
|---|---|
| `GET /api/v1/admin/service-status/config` | 管理员读取独立配置；disabled时仍可读取。只返回可编辑配置和version，启用区间不由客户端编辑。 |
| `PUT /api/v1/admin/service-status/config` | 管理员完整更新和版本比较；此写接口仅配置，不改变调用权限。 |
| `GET /api/v1/admin/service-status/snapshot` | 管理员只读；range=24h/7d/30d，缺省24h；可选一个已知具体platform。重复值、未知参数/平台或group_id/user_id/admin参数返回400，不支持扩大/模拟主体。enabled=false返回503 `SERVICE_STATUS_DISABLED`；无监控平台返回安全空快照。 |

未认证/普通用户按现有认证合同拒绝；前端隐藏不是权限保证。快照在同一只读可重复读事务读取配置版本、active分组、已提交事实/状态/事件。平台筛选同时应用摘要、卡片、分组/模型详情、历史和事件；不能混用不同提交批次。

响应白名单：schema_version、config_version、generated_at、observed_through、当前窗口/历史范围、monitoring_enabled、启用平台集合、已覆盖区间/数据缺口、platform与active group ID/名称、requested_model、health、unknown_reason、四类聚合计数、nullable error_rate/success_rate、事件ID/阶段/检测及证据/恢复时间。历史比例附qualified/unknown计数及覆盖区间，不能把30天未覆盖时间算成功。生成时间和查询成功不表示服务健康。

小样本不输出精确健康比例：qualified < minimum_samples时当前比例为NULL并给insufficient_samples；历史点使用同一门槛，unknown或未覆盖点灰显。管理员可查看聚合计数解释样本，不提供逐请求钻取、用户排行、账号库存或原始错误链接；计数只在管理员响应中出现。

管理区新增“服务状态”入口，保留V1/V2入口。页面区分当前5分钟状态、历史成功率、最近成功聚合时间、未结束事件和已恢复历史；disabled、无监控平台、no_recent_requests、insufficient_samples、terminal_unknown、watermark_stale、source_error、scope_unavailable及invalid_watermark都有明确文案。历史故障在stale时可以查看，当前摘要仍未知。支持中文/英文、窄屏、键盘；主体/路由/筛选切换使旧请求失效，403/停用时清空旧全站数据，晚到响应不能复原。恢复阶段文案为“恢复观察”，resolved才表示新请求确认恢复。

## 8. 必须保护的场景与后续验证责任

下表是S4.3/S4.4的验收输入与预期；S4.3已完成的定向检查见执行证据，不能据此宣称SS01–SS21完整阶段验收。按风险选择相关owner/真实SQL/权限/浏览器边界；不要求无依据地叠加全量或浏览器矩阵。

| 编号 | 输入/事件序列 | 必须得到的结果 |
|---|---|---|
| SS01 | 同HTTP键attempt 429→可信completed，费用=0/Token=0 | success=1，failure=0；中间错误不建故障。 |
| SS02 | 取消→drain completed；或completed→close | 前者excluded，后者success；保持S1事件顺序。 |
| SS03 | 5个最终provider失败与同批重复日志/重扫 | 只计5个生命周期；重复扫描不增加abnormal_windows。 |
| SS04 | 相同状态码的用户鉴权/额度/限流与供应商鉴权/额度/容量 | 用户excluded，供应商failure；不按状态码混排除。 |
| SS05 | 只有attempt/恢复2xx，无最终用量或错误终态 | unknown/terminal_missing，不能算success。 |
| SS06 | 同连接turn1失败、turn2成功；turn1内Proxy局部turn重置 | 两个独立键；turn2不覆盖turn1，同turn重建不重计。 |
| SS07 | 用量client:ID与Ops裸ID；客户端重用header；历史NULL | 仅新元数据能关联；不同请求不合并；旧记录linkage_unavailable。 |
| SS08 | 相同键终态互相矛盾或逻辑turn冲突 | unknown/terminal_conflict，不挑最新掩盖冲突。 |
| SS09 | TokenCount、cyber、猎手probe、gwpool降级attempt、Live | 排除/不支持按合同；不是故障、健康或恢复证据。 |
| SS10 | 没有请求、只有excluded、4个合格请求或unknown混入 | 分别no_recent_requests/no_qualified_requests/insufficient_samples/terminal_unknown；未知比例NULL。 |
| SS11 | 聚合失败/超限/45秒超时，旧水位随后超过180秒 | 事务回滚，不刷新水位；stale未知；原故障事件保留。 |
| SS12 | 旧失败移出窗口，无新请求；或仅放宽阈值 | 不recovering/resolved；等待新数据，连续恢复清零。 |
| SS13 | 足够新鲜正常样本分别形成3次新成功观测，期间重扫 | 达3次才resolved；重扫不计数，新的异常撤销恢复。 |
| SS14 | 迟到旧成功修正同键失败；或终态超重算/关联边界 | 可修统计但旧成功不证恢复；超界不回填、不补造覆盖。 |
| SS15 | 低流量模型有未结束故障/unknown，高流量模型健康 | 平台不能用加权比例显示已确认正常；遍历顺序不影响结果。 |
| SS16 | 普通/复合组；停用/删除组；未知平台/模型归属 | 有效最终范围一致；不可见组裁剪，缺失归属明确提示。 |
| SS17 | 停用→重启→重新启用；全部平台关闭；进程重启 | 持久化启用区间；不补停用流量；旧事件等待新证据，空范围非正常。 |
| SS18 | 两个聚合者、旧配置事务与配置更新竞争、读快照 | 单领导/原子提交/版本一致；更新成功后没有旧配置事件推进。 |
| SS19 | 匿名、普通用户、管理员，以及伪造范围参数/403晚到响应 | 服务端拒绝未授权；管理员全站白名单；旧全站数据清空。 |
| SS20 | 31天TTL、201条resolved、未结束事件、平台过滤 | 过滤先于limit，全部未结束计数保留，TTL不自动恢复。 |
| SS21 | 新schema→固定旧应用→新schema读写 | 原计费/ID/DTO/调度保持；NULL历史、批量/幂等/fallback可读，源元数据与匿名表不泄漏。 |

S4.3先落实源关联/终态元数据及独立配置/表，再实现幂等聚合、状态/事件、管理员DTO与页面。它不能只增加页面读取旧V2统计。S4.4对实际diff取得独立只读复核，覆盖归属、原子提交、权限和兼容；取得部署入口的真实终态覆盖后再决定发布与启用。文档检查只能证明规格/样例/链接/状态一致，不能提前勾选这些运行验收。

## 9. S4.2完成与下一项

本规格冻结终态优先、排除/未知语义、观察窗口/水位、故障与新请求恢复、独立保留/开关及管理员白名单；S4.2完成时新增源元数据尚未实现；S4.3本地结果见后续执行证据。S4.1线上流量摘要继续只佐证需求，S3.6成功门禁继续只证明未改变的应用候选，均不证明S4运行。

S4.2结束时下一项S4.3尚未开始。现S4.3已实现并完成必要定向验证/独立复核，见[执行证据](implementation-evidence.md#s43-独立聚合与展示实现)。分组可见性同事务触发器冻结计数/事件，使用配置共享锁并在获锁后形成基线；等待数据未知比例为NULL。S4.4进行中，已完成隔离PG18/真实JWT/数据库/浏览器贯通，见[执行证据](implementation-evidence.md#s44-隔离实例与真实-jwtpg18浏览器贯通)。代码默认关闭，仅隔离测试实例启用。固定旧应用、真实多轮WS与部署入口终态仍待；S4.4总体未勾选，无push/PR更新、合并、正式发布或生产部署。
