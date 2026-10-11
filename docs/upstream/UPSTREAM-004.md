# UPSTREAM-004：S3 结构化路由诊断后端迁移

Task: UPSTREAM-004

## Summary

已完成本地后端候选：在真实选择 owner 处采集不可变 v1 诊断，经过发送绑定、WS turn 快照、异步队列和已有 nullable JSONB 传给管理员单记录错误详情。未采集对象保持 nil/SQL NULL；未知计数保持 null；用户 DTO、列表与批量 include_detail 不扩展内部可见范围。没有改 SQL、Ent、安装身份、发布控制、frontend 或其他任务物理树。

独立只读复核、与 S1/S4 的共同 HTTP/WS owner 整合、远端 PR CI/SecurityScan 由父任务接手。此报告不把这些环节记为通过。

## 来源与具体 owner 比较

| 项目 | 冻结来源 |
| --- | --- |
| 目标 KIN | `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71` |
| 旧 KIN 比较点 | `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| 唯一 personal 补丁来源 A | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| 本分支起点 MIG-001 | `c0e7ab68b8479fc9d6f3b888d548eca4f4192974` |
| 分支 | `codex/upstream-004-s3-routing` |
| 合同 | A 中 `openspec/changes/adopt-plus-usage-and-diagnostics/routing-diagnostics-contract.md` |

读取 A 的完整最小单元后投影 S3 差异；扣除 A 中 S1 计时和 S4 状态接线，再应用到目标 owner，没有合并或覆盖旧历史。`cee1e908261c68880040b740aae8054410e03070` 是 producer 来源；storage 主提交实际是 `e159d6f3168500ce2956ad8b98332e88afba7940`；`ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1` 是后续 WS turn owner 修复。`da581c6846d1bf89926ca9730abf2290ea8b65ea` 的字符串 input 安全重放也在 A 中，已迁入必要的 native WS 部分。UI 的 `d23474171035320eda06a35d76d455d7f8d7aae4` 不属于本次范围。

旧、新 KIN 的 `openai_account_scheduler.go` 没有差异；`openai_gateway_scheduling.go` 的新平台归一化改动保留；Ops 原有 maintained owner 逐一比较后接入。新 KIN 的其他 gwpool owner 保持原实现，使用其现有入口池、去重、回合、终检和重选栈。HTTP/WS handler 保留新 KIN 的 per-turn 认证缓存分组、利润门与计费 API Key owner；实际比较确认该 maintained helper 和平台归一化完整函数仍与目标基线逐字一致。

仓库要求的 `docs/conventions/codex-outbound-identity.md`、`gwpool-client-dispatch.md` 在冻结源及已检查历史缺失，父任务已有缺口记录；本工作树的 `account-extra-freshness.md` 也不可读。未据此发明身份、Extra 新写入合同或另一套调度规则。

## 关键不变量、权威所有者与失败路径

| 不变量 | 权威所有者与实际行为 |
| --- | --- |
| 完整重新评估才递增序号 | `selectAccountWithSchedulerOnce` 创建 builder；嵌套 advanced/legacy 选择复用它。handler 重选、gwpool 新一轮、images 能力回退独立计数，同池 probe/复核/子池沿用一次计数。 |
| 入口池与最终过滤事实 | 列表取得后、本地阈值/Grok 后置过滤前观察实际池；ID 去重。只记录已执行且白名单允许的最终拒绝。TopK、未 probe、排序落后和可等待对象不计过滤。 |
| 未知不会猜零 | 没取得池为 unobserved；部分复核为 partial，下界与原因数相等；完整池为空才返回 0/0/{} complete。compact stale 的可恢复复核拒绝在 pass 时撤销。 |
| 错误与诊断分离 | 结构化错误包装保留 Unwrap 与 errors.Is/As。输入、历史详情及事件解析拒绝非法诊断时仅去掉对象，真实故障继续；固定异常信号不含诊断原值。 |
| 发送与最终归因 | `setOpsSelectedAccount` 绑定发送副本；上游事件取该尝试副本。routing 终态取当前完整评估；provider 终态取实际失败尝试；RequestScoped 不继承先前上游对象。恢复中间事件继续是既有 telemetry。 |
| HTTP、连接和 WS turn 隔离 | 请求上下文拥有序号，逻辑 WS turn 进入新 owner；proxy 局部 turn 重启保持同一逻辑 turn。建连池不能借给未选号首轮或复用后续轮；已有 first-wins 和 64 个失败快照上限保留。 |
| 不可变异步记录 | stream/event/queue 深复制诊断及 map，队列只保留自有 JSON，并把新增 JSON bytes 计入既有 job 预算；既有事件数量、正文和字节界限继续使用。 |
| 管理员详情权限 | 只在 errors/:id 和 request-errors/:id 详情返回；用户仍走白名单及 user_id 归属检查；列表和 include_detail 的副本裁剪不修改 detail owner。 |
| native 后续轮安全重放 | 换号须有可证明完整的 current input 与已完成 output；无法证明、工具别名或 item_reference 等情况显式终止，不能重放连接首包；已有输出后不切号。使用当前模型、渠道映射及计费 owner。 |

新 KIN 新增的 per-turn 分组计费带来一个迁移交点：后续 turn 换号重启 proxy 后局部编号归 1，会错误重新采用建连倍率。补丁继续携带原失败 turn 已冻结的计费 Key、pricingAt 和利润准入上下文，后续新 turn 仍调用原认证缓存刷新 owner。既有首包/重试/计费钩子局部编号不改。

另外确认并修正 A 中的事件读入漏洞：先向 typed event 解码会让分数或字符串诊断字段阻止真实错误解析。现在先按诊断接收边界裁剪，再 typed decode；整体 null 的旧合同保留。两项修复都有先失败、后通过的回归证据。

## Files changed

- `backend/internal/handler/admin/ops_handler.go`
- `backend/internal/handler/admin/ops_routing_diagnostics_handler_test.go`
- `backend/internal/handler/grok_audio.go`
- `backend/internal/handler/openai_gateway_credential_failover_loop_test.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/openai_gateway_ws_current_turn_model_test.go`
- `backend/internal/handler/openai_ws_routing_diagnostics.go`
- `backend/internal/handler/ops_error_logger.go`
- `backend/internal/handler/ops_routing_diagnostics_context_test.go`
- `backend/internal/handler/ops_routing_diagnostics_log_test.go`
- `backend/internal/handler/s35_http_routing_behavior_test.go`
- `backend/internal/handler/s35_ws_routing_behavior_test.go`
- `backend/internal/repository/ops_repo.go`
- `backend/internal/repository/ops_repo_args_test.go`
- `backend/internal/repository/ops_repo_routing_diagnostics_integration_test.go`
- `backend/internal/server/routes/ops_routing_diagnostics_routes_test.go`
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_account_scheduler.go`
- `backend/internal/service/openai_gateway_scheduling.go`
- `backend/internal/service/openai_routing_diagnostics_test.go`
- `backend/internal/service/openai_ws_forwarder_ingress.go`
- `backend/internal/service/openai_ws_native_resume_test.go`
- `backend/internal/service/ops_models.go`
- `backend/internal/service/ops_port.go`
- `backend/internal/service/ops_routing_diagnostics_dto_test.go`
- `backend/internal/service/ops_routing_diagnostics_log_test.go`
- `backend/internal/service/ops_service.go`
- `backend/internal/service/ops_upstream_context.go`
- `backend/internal/service/routing_diagnostics.go`
- `backend/internal/service/routing_diagnostics_test.go`
- `backend/internal/service/routing_diagnostics_validation.go`
- `docs/upstream/UPSTREAM-004.md`

## Tests run

以下命令均在 `backend/` 执行，Go `1.27.2`。unit/race 使用项目标签；integration 使用本地 Colima 的真实 PostgreSQL `18.1-alpine3.23`、Redis `8.4-alpine` 和 Testcontainers `v0.40.0`，没有禁用 Ryuk。

```sh
go test -tags=unit -timeout=120s -run '^(Test(RoutingDiagnostics|OpenAIRoutingDiagnostics|OpsRoutingDiagnostics|S35|OpenAIWSNative|OpenAIWSCurrentTurn|SetOpsRequestContextRetainsRouting|OpsStreamTurnClearsRouting|WSSelectionUsesCurrentRouting|WSRoutingTurnMapping|ResponsesWebSocket.*Routing|ResponsesWebSocketHandshakeFailover|ResponsesWebSocketTurnAfterFailover|GrokAudioSelectionRetries|OpsInsertErrorLogArgs))' ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository
```

通过：service `1.099s`、handler `3.367s`、admin `0.844s`、repository `0.901s`。覆盖字段未知/零、过滤下界、调度真实池、哨兵、HTTP 故障恢复及第二请求、WS 多轮及第二连接、深复制、队列预算、管理员 DTO/list。

```sh
go test -tags=unit -timeout=120s -run '^(TestOpenAIGatewayService_SelectAccountWithScheduler_|TestOpenAISelectAccount(ForModelWithExclusions|WithLoadAwareness)_|TestOpenAIGatewayService_(Guardian|PreviousResponse)|TestGatewayPoolSelection|TestGatewayPool(RoundsDoNotWait|BusinessRetryNewWSTurn|BusinessRetryHandler|ContinuousWaitSharedDomain)|TestResponsesCredentialFailoverLoop|TestResponsesWebSocketCredentialFailoverLoop|TestGrokOAuthCredentialFailoverAcrossHTTPHandlers|TestOpenAIGatewayServiceRecordUsage_UpdatesAPIKeyQuotaWhenConfigured)' ./internal/service ./internal/handler
```

通过：service `0.809s`、handler `0.652s`。保护目标 KIN sticky、guardian、previous-response、weighted/subpool/排序/等待、gwpool 入口和回合重选、凭据 failover、API Key quota 既有合同。

```sh
go test -tags=unit -timeout=60s -run '^(TestOpsRoutingDiagnosticsAdminRoutesJWTBoundary|TestAdminAuthJWTValidatesTokenVersion|TestGetUserErrorRequestDetail_|TestListUserErrorRequests_|TestToUserErrorRequest)' ./internal/server/routes ./internal/server/middleware ./internal/service
```

通过：routes `0.882s`、middleware `0.641s`、service `0.889s`。真实 JWT 签发/校验及实际 RegisterAdminRoutes 证明两条详情路由的无认证 401、篡改签名 401、普通用户 403、撤销版本 401、管理员 200；拒绝路径不查询详情。复用用户归属与白名单测试。认证用户仓库为最小替身，没有宣称完整登录/会话绑定/审计/合规/浏览器 E2E。

```sh
go test -tags=unit -timeout=120s -run '^(TestOpenAIResponsesWebSocket_(GroupRate|KeyMoved|FailedKey|FirstFrame|SubsequentTurn|DisabledAllowlist|Duplicate|SessionUpdate)|Test(RefreshOpenAIWSTurnBillingAPIKey|OpenAIWSTurnBillingAPIKeys|OpenAIWSTurnPricing|ParseOpsUpstreamErrors))' ./internal/handler ./internal/service
```

通过：handler `0.695s`、service `0.813s`。目标 KIN per-turn 分组调价、利润门、Key 换组与刷新失败、模型白名单及旧事件代理归因保留。

```sh
go test -tags=unit -timeout=120s -run '^(TestOpenAIWSCurrentTurnModelFailover|TestOpenAIResponsesWebSocket_(GroupRate|KeyMoved|FailedKey)|TestOpenAIWSTurnBillingAPIKeys|TestS35WS|TestResponsesWebSocketTurnAfterFailover)' ./internal/handler
```

通过：handler `3.360s`。实际 socket + 生产选号/渠道/定价/用量 owner 检查换号后只选择当前模型、当前能力、requested/channel-mapped 定价、字符串 input 及当前 turn 分组倍率。该用量 fixture 为 simple 模式，没有真实余额扣费。

```sh
go test -race -tags=unit -timeout=180s -run '^(Test(RoutingDiagnostics|OpenAIRoutingDiagnostics|OpsRoutingDiagnostics|S35|OpenAIWSNative|OpenAIWSCurrentTurn|SetOpsRequestContextRetainsRouting|OpsStreamTurnClearsRouting|WSSelectionUsesCurrentRouting|WSRoutingTurnMapping|ResponsesWebSocket.*Routing|ResponsesWebSocketHandshakeFailover|ResponsesWebSocketTurnAfterFailover|GrokAudioSelectionRetries|OpsInsertErrorLogArgs|OpenAIResponsesWebSocket_(GroupRate|KeyMoved|FailedKey)|OpenAIWSTurnBillingAPIKeys))' ./internal/service ./internal/handler ./internal/handler/admin ./internal/repository ./internal/server/routes
```

最终候选通过：service `2.573s`、handler `4.895s`、admin `2.245s`、repository `2.549s`、routes `2.524s`，没有 race 报告。包括最终事件解析修复、倍率修复及 JWT 路由边界。

```sh
DOCKER_HOST="$(docker context inspect colima --format '{{.Endpoints.docker.Host}}')" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock CI=true go test -v -tags=integration -timeout=180s -run '^(TestOpsRoutingDiagnostics_|TestMigration252_)' ./internal/repository
```

通过：首次真实容器运行 repository `3.040s`；最终命令复用相同有效输入的缓存。确认 single/batch 写入、已冻结 JSON 为存储来源、历史 NULL/JSON null、非法版本/字段/分数/关系丢诊断不丢错误、同批数据库失败原子性、列表不泄露、252 历史和旧写入保持 NULL、nullable/no-default 及重复执行幂等。Redis 容器启动并 PING，但该 SQL scope 不覆盖 Redis 业务写入。

```sh
go build ./...
```

最终 backend build 退出 `0`。`git diff --check` 退出 `0`。没有执行整仓测试、生产部署或远端 CI。

收敛时的失败已定位：第一次 unit 编译发现新 KIN 已有 `routingTestAccount` 测试 helper，新增 S3 helper 改名以保留原测试；无 Docker 环境变量的 integration harness 曾直接跳过，未计为通过，补上已验证环境后真实运行成功；事件分数诊断回归在修复前以 typed unmarshal 错误失败；后续 turn 倍率回归在修复前得到 `1` 而预期 `0.25`，修复后通过。无无法解释的失败或盲重试。

## Database impact 与恢复路径

MIG-001 已按原字节保留 migration 252，本次只将 Ops raw SQL 的第 39 个 nullable 参数与详情读取接回已有 `ops_error_logs.routing_diagnostics`。没有新增 migration、改 ledger、default、backfill、Ent schema 或生成文件。未接入此 producer 的其他流量继续 nil/SQL NULL。

251–254 工作树字节与 MIG-001、固定 A 三方完全一致；SHA-256：

| 文件 | SHA-256 |
| --- | --- |
| `251_add_usage_log_timing.sql` | `abce11c5a8cf8fa993d9a0fb25190af3130722fcc419f1f94b5a5397ff60003b` |
| `252_add_ops_routing_diagnostics.sql` | `b01aa7a1a3fd47681dc135b13997710f253f9a63ab36ea139593200194b395ac` |
| `253_service_status_observation.sql` | `8440c2da7a92dd5b70da653a7c02a485bca4337ae76868ef5b0ccd7996b897c2` |
| `254_service_status.sql` | `3e8deac13622490588c61dc5cc9411653c05631009c19380e83e289e17251b8b` |

数据库批量失败保持显式错误及原有原子性；诊断字段非法时丢对象而真实错误仍入库。恢复只撤销本任务应用提交，保留 MIG-001 和 additive nullable schema；旧应用忽略新列并继续旧字段写入。正式回退前仍由父任务按其 MIG-001 兼容证据和部署环境复核，不手工改列/迁移身份。

## Risks 与父任务接手

- 本候选改变内部 Ops 公共 JSON/persistence/并发快照边界，需独立只读复核；本次不把自审作为独立复核。
- 父任务 S4 要整合 S1/S3/S4 共用的 handler、ops logger、stream snapshot、WS ingress owner。重点保留路由 owner/实际发送归因、当前模型及新 KIN per-turn 分组和利润门；不得通过覆盖旧文件丢失其他任务修改。
- 未验证生产账号池、真实上游服务、真实 JWT 登录全流程、会话绑定/合规/审计、生产账本扣费、浏览器或完整数据库恢复演练。真实 socket 与用量测试仅验证受测边界。
- 未运行远端 PR CI/SecurityScan；没有推送/PR/发布/部署；Personal CI/来源切换属于 UPSTREAM-007。

## Acceptance checklist

- [x] 重新比较目标 KIN 具体 sticky、guardian、previous response、load balance、gwpool owner 并保留其调度行为。
- [x] selection_attempt 只在完整重新评估增长，未 probe/低排序/可等待不误计过滤。
- [x] 实际池、计数一致、partial/unobserved 与 compact stale 恢复合同通过定向测试。
- [x] 白名单、错误链、非法诊断仅丢对象、不泄露原值。
- [x] HTTP/WS turn、第二连接和不可变异步队列副本合同通过 race scope。
- [x] 管理员单记录 JWT 路由边界、普通用户白名单/归属、列表裁剪通过。
- [x] 历史 NULL/no-default、single/batch、失败原子性在真实 PostgreSQL 测试通过，251–254 原字节保留。
- [x] native 后续 turn 换号安全证明、字符串 input、当前模型/映射及分组计费 owner 定向通过。
- [x] backend build 与 diff/范围检查通过。
- [ ] 独立只读复核及 S1/S4 整合后复验。
- [ ] 同一最终集成 SHA 的远端 PR CI/SecurityScan。
- [ ] 生产灰度、真实账本/完整 JWT E2E、部署恢复验收（本次未执行）。
