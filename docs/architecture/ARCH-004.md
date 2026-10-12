# ARCH-004：个人功能独立开关与降级

## 基准与任务选择

前置 UPSTREAM-007 已整合；本轮按维护者授权普通合并 [PR #47](https://github.com/ccisnoxx/sub2api/pull/47) 并关闭 ARCH-001。固定 personal 基准为 `2867ad10d46d7b774a2e8c2b3406f9e6ad75f0e0`，干净隔离分支 `codex/arch-004-personal-feature-flags` 复用既有工作树。固定运行实现为 `3303ff1bc27ffa69d292a281ea820cfabc189412`，文档单独提交，不改变验证输入。

下一可执行未完成任务是 ARCH-004。ARCH-002 依赖尚未完成的 REL-002；本轮暂不发布或部署，REL-002/003 不执行，ARCH-003 已完成。ARCH-005 保持独立任务。

原文档工作区 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca` 和计划原暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。本轮只定向更新原台账，不把原暂存内容纳入 PR。

## 已确认的所有权与行为

| 功能 | 配置 owner / 默认值 | 运维检查与禁用状态 |
|---|---|---|
| 模型目录 | 既有 settings `available_channels_enabled`，默认 false | 管理设置 GET/PUT；认证后 `?view=catalog` 明确返回 503 / MODEL_CATALOG_DISABLED；配置读取失败为 MODEL_CATALOG_UNAVAILABLE |
| 路由诊断 | settings `routing_diagnostics_enabled`，缺少 key 默认 true，保持既有部署行为 | 独立管理员配置 GET/PUT；enabled 为持久配置，effective_enabled 为本实例采集快照；诊断详情可见性由同次快照读取决定 |
| 服务状态 | 既有 service_status_config.enabled，默认 false | 独立管理员配置 GET/PUT；快照已有 503 / SERVICE_STATUS_DISABLED；配置 API 在关闭时仍可访问 |

目录复用原设置和审计，不建立第二份开关。用户身份检查仍先于设置；禁用或读取失败不查询分组、渠道、价格或个人倍率。旧无 view 数组接口保留已有关闭返回，目录不能再用成功空数据掩盖禁用。现有 Vue 目录页已经通过错误分支展示服务器安全 message，不修改 UI 或价格/授权/网关逻辑。重新启用读取当前授权与价格，不恢复旧目录快照。

路由诊断复用 OpsService 既有 atomic 不可变快照、发布/刷新锁及约 30 秒加 jitter 的跨实例刷新。缺少 key 保留默认 true，实际读取失败的冷启动只关闭诊断；非法配置显式报错，刷新错误保留已知诊断快照，其它 Ops 配置继续原刷新合同。成功持久化后才发布，保存失败不改变有效值；监控和 advanced settings 更新保留诊断字段。该诊断开关独立于 Ops 监控开关，配置接口不会关闭自身，管理更新不改变监控配置。

关闭的新请求不创建诊断 owner/builder，WS turn/owner 重绑及 transport/gateway-pool fallback 沿用请求的采集状态。关闭前已有请求或 WS 连接不被中断；RecordError/Batch 在整批准备后的最终写入边界重新检查开关，裁剪顶层与嵌套诊断但保留真实错误和服务状态 observation。已经越过最终检查并开始的数据库写入不被取消，跨实例不承诺即时原子关闭。

管理单记录详情返回独立 DTO，routing_diagnostics_enabled 是同次检查的状态；关闭时裁剪历史顶层/嵌套诊断且不修改原存储对象，重开可读既有历史。列表与用户白名单不输出该配置字段；关闭诊断后的历史裁剪遇到非法事件格式时显式失败；诊断启用时沿用原 normalization 失败兼容行为。成功配置 PUT 通过原认证后的审计队列，action 为 admin.ops.routing_diagnostics.config.update，摘要仅含允许的 enabled/result；失败也按原通用审计保留 HTTP 状态。目录与服务状态已有管理配置审计复用。

服务状态每轮在原 45 秒预算内读取当前配置，关闭、没有启用平台或读取失败时跳过 Aggregate。同一调度实例保留原一分钟 ticker，用于发现重新启用；不调用永久 Stop 充当运行期开关，不新增 goroutine 或锁。仓储事务内原 enabled 检查保留，用于调度读取后并发关闭的交错。

禁用期间不再定期运行聚合/TTL 清理，重新启用后的首次聚合会补做原清理；历史数据/schema 不删除。请求日志里的安全源元数据与有待提交时的 source gap reporter 保留完整性职责，不属于主动聚合或探测。

重新启用复用原 UpdateConfig 事务：配置 version 递增、scope abnormal/recovery count 立即清零、open incident 冻结 awaiting_data；新平台 epoch 为数据库时间的下一完整分钟。归一化依原观察时间和 epoch 排除旧区间证据，coverage 保留禁用间隙，事件要求新 evidence 才推进。该持久化 owner 本轮未修改。

## 定向验证与覆盖边界

实际 Go1.27.2 darwin/arm64，`GOTOOLCHAIN=local`，默认 vet。命令均从 backend 执行，go 指缓存的 1.27.2 工具链二进制。

- `go test -tags=unit -timeout=60s -run '^TestAvailableModelCatalog_' ./internal/handler`：exit 0，0.932s。
- `go test -tags=unit -timeout=60s -run '^(TestServiceStatusAggregator|TestServiceStatusHandler|TestServiceStatusConfig|TestAvailableModelCatalog_Reenable)' ./internal/service ./internal/handler`：exit 0，service 1.098s、handler 0.943s。包含目录重新启用读取更新价格的追加严格断言。
- 仓储纯状态定向检查曾在并行路由诊断实现尚未写完时编译失败（未定义新符号）；这是候选未稳定的执行时点问题，不是测试行为失败。等待实现稳定后按失败检查优先执行，实现稳定后同一失败检查通过（exit 0，0.812s）。

- 仓储纯状态：`go test -tags=unit -timeout=60s -run '^TestServiceStatus(IncidentNeedsNewEvidenceAndThreeNewRecoveries|IncidentUnknownConfigAndAbnormalResetRecovery|CoverageLeavesDisabledAndFailureIntervalsBlank)$' ./internal/repository`：稳定候选 exit 0，0.812s。
- 目录不可用原因保留后：`go test -tags=unit -timeout=60s -run '^TestAvailableModelCatalog_AuthenticationAndFeatureShortCircuit$' ./internal/handler`：exit 0，0.890s。
- 实际正式路由/JWT/审计：首轮包含新检查、ServiceStatusRoutes 和 OpsRoutingDiagnosticsAdminRoutesJWTBoundary，因夹具错误将独立诊断生效状态与 Ops 监控状态合并而失败。修正为诊断独立生效且更新不改变监控配置后，按失败检查优先执行 `go test -tags=unit -timeout=60s -run '^TestPersonalFeatureFlagsRoutingConfigJWTAndAudit$' ./internal/server/routes`：exit 0，0.905s。检查签名/撤销/普通用户拒绝、管理员关开、必填值、保存失败保留配置、监控关闭时配置仍可访问和实际审计队列交付的四条 PUT 状态/身份。

路由诊断实施代理执行：

```sh
go test -tags=unit ./internal/service ./internal/handler ./internal/handler/admin \
  -run 'Test(OpsRoutingFeature|OpsRoutingDiagnostics|OpsRuntimeSettings|OpsServiceRecordError|RoutingDiagnostics|OpenAIRoutingDiagnostics|SetOpsRequestContextRetainsRoutingSelectionAttempts|OpsStreamTurnClearsRoutingProducerState|WSSelectionUsesCurrentRoutingOwner)' -count=1
```

service 1.158s、handler 0.921s 成功；admin 首轮发现批量详情新增配置元数据违反列表白名单。主代理在原裁剪点清空 metadata，按失败及直接相关范围运行：

```sh
go test -tags=unit ./internal/handler/admin \
  -run 'TestOpsRouting(DiagnosticsAdminDetailAndListBoundary|Feature)' -count=1
```

exit 0，1.027s。相关并发快照、刷新/管理更新序列、请求/WS owner、队列副本与诊断关闭边界的 Race：

```sh
go test -race -tags=unit ./internal/service ./internal/handler ./internal/handler/admin \
  -run 'Test(OpsRoutingFeature|OpsRuntimeSettings|RoutingDiagnosticsConcurrentSequenceAndPublish|OpsRoutingDiagnosticsQueuedCopyAndBudget|SetOpsRequestContextRetainsRoutingSelectionAttempts|OpsStreamTurnClearsRoutingProducerState|WSSelectionUsesCurrentRoutingOwner)' -count=1
```

exit 0，service 2.476s、handler 2.201s、admin 2.307s。最后新增用户/列表 metadata 白名单断言，仅重跑受影响单项 `go test -tags=unit ./internal/service -run '^TestOpsRoutingFeaturePersistenceAndHistoryPreserveCoreError$' -count=1`：exit 0，1.100s。测试初轮构造参数个数和 json 隐藏 observation 的错误断言已修正，没有修改业务合同掩盖失败。

所有修改 Go 文件 gofmt、精确 diff --check 和15个本地文档链接通过。前端源码、计费 owner、持久化 schema/迁移和原服务状态配置事务未修改，复用旧证据，不重复全量检查。

本地默认 `/var/run/docker.sock` 和已配置 Colima `/Users/sc/.colima/default/docker.sock` 均不存在，无法执行真实 PostgreSQL/Redis 集成 harness；未启动一个会以 exit 0 跳过的检查并冒称通过。原配置事务未修改，纯状态回归和独立审查用于核对计数/epoch owner，真实 DB 关开生命周期本轮未动态验证。

## 风险、数据库与回退

实际风险是禁用响应合同、诊断配置的并发发布/刷新、冷加载和保存失败、历史/在途诊断裁剪以及服务状态调度启停。需要定向实际 HTTP/JWT/审计、服务状态单轮/取消预算、相关纯状态和诊断边界测试；不由修改文件数量推导全量门禁。

数据库 schema/迁移影响 none。路由诊断使用既有 settings 键值存储，服务状态复用独立配置，保留 SQL、已执行迁移身份/checksum、历史、计费、来源与权限边界。没有生产数据或部署操作。

回退采用普通 `git revert 3303ff1bc27ffa69d292a281ea820cfabc189412` 回退本任务实现并运行直接相关检查，保留既有开关、schema、历史/迁移账本和 ARCH-001 五组件；文档独立回退。回退应用后新诊断 key 可保留，旧版本忽略它，旧版本会恢复本任务之前的诊断/调度行为。禁止 reset/强推、删除历史或回滚 schema。

仓库 AGENTS 引用的 codex-outbound-identity conventions 与 docs/tasks 缺失已查找确认，本任务不改出站身份、账号 extra 或 gwpool 选路/目录缓存。

## 候选验收状态

- [x] 目录和服务状态复用独立开关与既有审计，默认值和明确禁用接口已核对。
- [x] 服务状态禁用轮次不调用聚合，重新启用恢复，配置读取失败跳过；原停止取消及预算保持。
- [x] 重新启用的确认计数、epoch、coverage owner 保持，相关真实 DB 缺口如实说明。
- [x] 路由诊断独立开关、管理配置、捕获/持久化/历史详情及并发验证完成。
- [x] 最终固定候选的真实 JWT/审计验证、独立只读复核和执行审计闭合。
- [x] 固定运行实现已提交，候选报告、一次推送/草稿 PR 交付策略及普通回退已记录。

本轮不发布或部署，不 dispatch Quick/Full，不运行应用全量、安全扫描或发布 dry-run。

## 独立复核、审计与交付

fresh critical_reviewer 以隔离上下文对固定实现 `3303ff1bc27ffa69d292a281ea820cfabc189412` 的16文件完整 diff、新测试、原台账及直接合同进行只读复核，未确认 actionable findings 或普通候选交付阻断。核对目录认证/短路、发布与旧刷新序列、冷加载/非法值/保存失败、WS/transport/终检/nil builder、队列与历史裁剪、列表/用户白名单、正式管理认证/审计和服务状态原配置事务，四项验收均有支持。

复核独立运行精确 diff --check，通过；没有重复已通过测试。Go/Race 执行结果采用主代理和实施者提供的工具证据，原始 stdout/exit 保存在各线程的工具返回中，没有独立导出的原始日志文件。真实 PG/Redis 关开、事务提交/锁等待/进程重启恢复未动态验证；原仓储事务和迁移未变，纯状态回归、调度检查及静态事务复核支持源码候选验收，不构成生产部署结论。

审计 `20261012T124459Z-arch-004-personal-feature-flags-b0495878` 已闭合，audit-verify 通过：3个计划/3次执行均验收通过，1次独立复核，无异常、未执行 ready task 或活跃残留。实施代理的8个写入路径符合已授权所有权；分析与复核未观察到写入。排除主代理3份文档后，复核前后源码内容指纹均为 `33a339be7d22b584737a1b8c484db07c8a491d218cd0a388489d43f3ccad39e2`。Agent TOML 模型/推理档位为配置证据，不冒称运行时遥测。

源代码冻结后仅补三份文档，验证输入不变，复用成功证据。交付采用一次推送和面向 personal 的草稿 PR，具体文档 head/PR URL/Actions 状态按 GitHub 回读写入原维护台账；候选不在本轮合并。所有操作保持不发布或部署。
