# UPSTREAM-002 后端 S1 用量计时重放

## Task

- 工作分支：`codex/upstream-002-s1-usage`。
- 验收基准（MIG-001 已验收）：`c0e7ab68b8479fc9d6f3b888d548eca4f4192974`。
- 实现提交：`b41aff259f1fb9534d3ecfba7c80be1ba9ad9d05`（父提交即上述基准）；音频来源边界修正提交：`df33452e7f74161e1fda20351d92b0834bb474a9`。最终运行实现范围为 `c0e7ab68b8479fc9d6f3b888d548eca4f4192974..df33452e7f74161e1fda20351d92b0834bb474a9`，不含 merge 提交。
- 新 KIN：`ca686de5e5ac14834e12701c156831abd5ec6b71`；固定旧 KIN：`c7aacf5d3ae383d0d5c75f471f66e61690a5701d`；固定重放来源旧 personal：`3d5e1fde82707900a21f2b5538b112c7bab3c04a`。只移入 S1 小范围 patch，没有合并旧历史。
- 交付前于 `2026-10-11T09:14:03Z` 通过 GitHub `releases/latest` 和 `git ls-remote upstream` 复核：最新稳定仍为 `v0.2.15-klno.2`，tag object `0c1ede31ec63f9cfc2b72f07db66fb41f540208d`，peeled commit 与新 KIN 相同。
- 本报告作为实现之后的独立 docs 提交，不改变已验证运行候选。完整交付范围以交付消息提供的报告提交 SHA 固定。

## Summary

在原生 OpenAI Responses HTTP JSON/SSE、HTTP 透传、HTTP→pooled WS、pooled WS ingress、WS 原生透传与 WS→HTTP bridge 上新增九个 S1 字段。`responsesOutputTiming` 是单次尝试或 turn 的事实 owner；已有转发 owner 仍决定起点、取消、drain、重试和结果是否进入用量提交。`UsageTiming` 快照在转发结果、异步提交边界及 `RecordUsage` 深复制可空字段，repository 只持久化这些事实。

保留旧 `first_token_ms` 的 semantic/visible 设置、原 token 总量及图片拆分、模型与 tier 计费、扣费、请求指纹、幂等键、部分失败和异步任务提交策略。没有创建新的失败计费行，也没有接入 S2、S3 或 S4。新 KIN 的 hosted image tool usage、图片输入 token、返回模型观察和已有失败结果处理均保留。

旧 S1 观察器未覆盖新 KIN 的 `data.usage` / `data.response.usage` 来源。这里让观察器与既有计费解析器共用同一来源选择循环，并返回实际接受的 raw usage；解析优先级、总量和 hosted image 回填算法不变。兼容包装中非法音频拆分也保持 NULL，不能因纯文本输出将其改成零。

关键不变量及权威 owner：

| 不变量 | 权威 owner / 实际行为 |
| --- | --- |
| 旧首字与计费结果保持 | 既有 HTTP/WS 首字设置、用量 parser、`RecordUsage` / billing repository；新观察器不修改这些字段或指令 |
| 严格首/末仅记录有效输出 | `responsesOutputTiming`；非空文本、推理、真实工具 delta/added 推进；metadata、空 delta、done 汇总不补造流式 token；媒体只建立首次输出 |
| 非流式观察边界 | 完整 JSON / 缓冲 SSE 实际读到时刻；pooled WS 的同一终态帧使用同一个观察时点 |
| 完成与取消冻结 | observer 的 mutex 和首次终态冻结；取消先被观测则保持 `client_disconnected`，后续 final 只可更新接受用量来源；已完成后关闭不改为取消；failed 后 DONE 不翻成功 |
| 缺失事实保持未知 | 未接入为版本 0 / NULL / unknown；已采集但无协议事实为 unknown；有协议事实却无 terminal 为 interrupted；缺失或非法音频不猜零 |
| 音频与来源跟随实际保留用量 | 原生 extractor 与 relay 原有 parser；全零 terminal 若被原 parser 拒绝替换，来源仍 partial；接受 final 总量后，缺失音频拆分不沿用 partial 数值 |
| WS turn 隔离 | pooled ingress 的当前 ID 过滤；原生透传使用 relay 权威 `ResponseID` / `StartedAt`，注册表同步排序客户端断连与上游事件 |
| 异步值无指针别名 | observer snapshot、handler 提交边界、`RecordUsage` 各自深复制可空字段；原 worker pool/drop/mandatory fallback owner 不变 |

## Files changed

实现提交修改 42 个文件：schema 及项目命令生成的 ent 产物 9 个，handler/DTO 6 个，repository 4 个，service 与定向测试 21 个，固定合同 2 个。完整文件清单：

```text
backend/ent/migrate/schema.go
backend/ent/mutation.go
backend/ent/runtime/runtime.go
backend/ent/schema/usage_log.go
backend/ent/usagelog.go
backend/ent/usagelog/usagelog.go
backend/ent/usagelog/where.go
backend/ent/usagelog_create.go
backend/ent/usagelog_update.go
backend/internal/handler/dto/mappers.go
backend/internal/handler/dto/mappers_usage_test.go
backend/internal/handler/dto/types.go
backend/internal/handler/dto/usage_timing_replay_test.go
backend/internal/handler/openai_gateway_handler.go
backend/internal/handler/usage_record_submit_task_test.go
backend/internal/repository/usage_log_repo_insert.go
backend/internal/repository/usage_log_repo_query.go
backend/internal/repository/usage_log_repo_request_type_test.go
backend/internal/repository/usage_log_timing_integration_test.go
backend/internal/service/openai_gateway_forward.go
backend/internal/service/openai_gateway_passthrough.go
backend/internal/service/openai_gateway_passthrough_function_args_test.go
backend/internal/service/openai_gateway_response_handling.go
backend/internal/service/openai_gateway_service.go
backend/internal/service/openai_gateway_usage.go
backend/internal/service/openai_usage_timing_billing_test.go
backend/internal/service/openai_usage_timing_test.go
backend/internal/service/openai_ws_forwarder_client_cancel_test.go
backend/internal/service/openai_ws_forwarder_ingress.go
backend/internal/service/openai_ws_forwarder_ingress_session_test.go
backend/internal/service/openai_ws_forwarder_success_test.go
backend/internal/service/openai_ws_forwarder_v2.go
backend/internal/service/openai_ws_http_bridge.go
backend/internal/service/openai_ws_v2/passthrough_relay.go
backend/internal/service/openai_ws_v2/usage_timing_callback_test.go
backend/internal/service/openai_ws_v2_passthrough_adapter.go
backend/internal/service/stream_output_timing.go
backend/internal/service/stream_output_timing_test.go
backend/internal/service/usage_log.go
backend/internal/service/usage_timing.go
openspec/changes/adopt-plus-usage-and-diagnostics/spec.md
openspec/changes/adopt-plus-usage-and-diagnostics/coverage.md
```

父任务另更新 `backend/internal/server/api_contract_test.go` 的既有 API fixture。

本 docs 提交另增加 `docs/upstream/UPSTREAM-002.md`。两个 `openspec` 合同与固定旧 personal 的对应 blob 逐字节一致；其中旧前端和旧执行覆盖记录仅作为固定来源保存，本次新候选的执行证据以本报告为准。

## Tests run

下列测试命令均实际运行。除 `make` 在仓库根目录运行外，Go 命令在 `backend` 执行；本地工具链实际为 `go1.27.2 darwin/arm64`。Docker 宿主连接值已与 `docker context inspect colima --format '{{.Endpoints.docker.Host}}'` 实际输出核对一致；下文用 `task_docker_host` 代指该宿主 socket 值，避免在仓库报告写入开发机绝对路径。容器内 socket override 保留原值。

```sh
go generate ./ent
make -C backend build

go test -tags=unit ./internal/repository -run '^(TestUsageLogStaticInsertShape|TestPrepareUsageLogInsert|TestUsageLogRepository|TestScanUsageLog|TestUsageLog.*RequestType|TestBuildUsageLog|TestExecUsageLog)' -count=1

go test -tags=unit ./internal/repository -run '^TestUsageLogInsertQueries_IncludeSessionID$' -count=1

DOCKER_HOST="$task_docker_host" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock go test -tags=integration ./internal/repository -run '^(TestUsageLogTiming_|TestMigration251_UsageTiming|TestUsageBilling)' -count=1 -timeout=180s -v

go test ./internal/service -run '^(TestResponsesUsageTiming_HTTPWrappedUsageKeepsNewKINAccountingSource|TestExtractOpenAIUsage|TestParseSSEUsage_SelectiveParsing|TestParseSSEUsage_NonTerminalUsageMergesNonZeroFields|TestParseSSEUsage_Terminal|TestParseSSEUsageEffectiveTerminalRules|TestParseSSEUsageBytes_ResponseCompletedWithImageGen|TestOpenAIGatewayServiceRecordUsage_|TestOpenAIUsageTiming_)' -count=1

go test -race ./internal/service ./internal/service/openai_ws_v2 ./internal/handler ./internal/handler/dto -run '^(TestResponsesOutputTiming|TestResponsesUsageTiming|TestOpenAIUsageTiming|TestRelayUsageTimingCallback|TestUsageTimingReplay|TestUsageLogFromService|TestOpenAIGatewayHandlerSubmitOpenAIUsageRecordTask|TestOpenAIGatewayHandlerSubmitUsageRecordTask|TestOpenAIGatewayHandlerSubmitMandatoryUsageRecordTask|TestForwardResponsesChatCompletionsFallbackKeepsFunctionArgumentsSingle|TestForwardOpenAIWSV2_ClientCancellation|TestOpenAIGatewayService_Forward_WSv2_(SuccessAndBindSticky|ResponseFailedIsNotSchedulingSuccess)|TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_KeepLeaseAcrossTurns|TestRelay_ImageUsageMatchesHTTPResponsesAccounting|TestOpenAIWSPassthroughTurnResultCarriesImageUsage)' -count=1 -timeout=120s

go test -race ./internal/service -run '^TestResponsesUsageTiming_WSHTTPBridgeSeparateTurnsAndIDs$' -count=1 -timeout=120s

go test -race ./internal/service ./internal/service/openai_ws_v2 -run '^(TestResponsesOutputTiming|TestResponsesUsageTiming|TestRelayUsageTimingCallback|TestForwardOpenAIWSV2_ClientCancellation|TestOpenAIGatewayService_Forward_WSv2_(SuccessAndBindSticky|ResponseFailedIsNotSchedulingSuccess)|TestOpenAIGatewayService_ProxyResponsesWebSocketFromClient_KeepLeaseAcrossTurns|TestRelay_ImageUsageMatchesHTTPResponsesAccounting|TestOpenAIWSPassthroughTurnResultCarriesImageUsage)' -count=1 -timeout=120s

git diff --check
```

- ent 生成成功，未手改产物；生成过程新增的 CLI 依赖校验和已清除，`go.mod` / `go.sum` 最终没有 diff。最终项目 build 成功。
- repository unit 命令分别通过 49 项和 1 项，覆盖新增列宽、placeholder、请求类型、scan 及原有尾部字段顺序。
- PostgreSQL 定向 integration 真正执行并通过 14 项，包括单条/批量 SQL、冲突幂等、best-effort / FK 失败后的单条回退、ent 可空历史、新版 schema 与旧 writer、251 重复应用及既有 billing 测试。
- parser / `RecordUsage` 定向命令通过 108 项，覆盖新 KIN 包装来源、hosted image 回填、token fallback、计费与幂等/扣费 owner。
- 四包定向 race 通过 106 项，覆盖 observer 取消排序、实际 async worker 快照、普通与透传 HTTP JSON/SSE、SSE→JSON、HTTP→pooled WS、同连接 pooled ingress 多 turn / foreign response ID、WS 原生透传、WS→HTTP bridge、失败、取消、缺 terminal、工具/推理/图片/音频及旧 DTO。之后加强 bridge 的 foreign ID 媒体案例，对该测试单独跑 race 通过。最后发现并修正 native 与 relay 两类 parser 对双 usage 的优先级不同所暴露的音频推零问题；最终实现提交针对全部受影响的 observer / HTTP / WS 路径再跑两包定向 race 通过 75 项，并再次通过项目 build。此前 async、DTO、repository 和 billing 成功证据对应的实现没有改变。
- 回归红灯已实际确认：新增 DTO 测试在未重放基准上缺 `timing_version`；兼容包装测试在初始重放候选上出现计费已接受却 `usage_source=unknown`；包装内非法音频测试在修复来源选择后仍被错误改为零。另已确认真实 WS 透传双 usage 案例在来源优先级不一致时会把非法音频改为零，修正后最终 race 通过。对应定向测试均已通过。
- 首次未指定 Docker host 的 integration 被 harness 跳过，不计通过；仅指定 Colima host 的一次执行因 Ryuk 把宿主 socket 路径当作 VM 挂载路径失败。按安装版 testcontainers-go v0.40.0 官方本地文档增加 socket override 后，上述 14 项实际执行通过，没有修改仓库 harness。
- 非平凡第三方 API 已对安装版本核对：ent v0.14.5、gjson v1.18.0、coder/websocket v1.8.14、testcontainers-go v0.40.0；读过实际 API doc / 安装版 Colima 指南。

远端首轮 PR CI `38127927159` 在既有 `TestAPIContracts/GET_/api/v1/usage_(paginated)` 失败：旧 inline JSON fixture 漏新增九列，并非运行行为与 S1 合同不一致。父任务补齐版本 0、NULL、unknown 的精确响应预期；`go test -tags=unit -run '^TestAPIContracts$' ./internal/server` 实际通过 12 项。lint、frontend、shell、release-helpers 和首轮安全扫描通过；失败候选不计阶段 CI 验收。最终候选及门禁结果以 PR #20 固定记录为准。

## Risks

- fresh `critical_reviewer` 独立只读复核运行候选 `2d21a1634228d3d0573ac266dacd726e2908e2de`，覆盖取消/终态 mutex 顺序、两类 parser 来源、SQL 82 列、异步复制与 response ID 隔离，未确认阻断代码问题。审查未重跑测试；成功 race/PG 原始 stdout 未保存，由父任务原 runtime 输出核验。
- 原生 WS 未结算 turn、全零 terminal fallback、无法确认 ID/起点的 opaque binary 等路径维持既有记录 owner；不为补齐 S1 事实制造用量行。Chat Completions 转换、其他平台、独立图片/音频等媒体入口及 S1.5 未声明采集；CC→Responses fallback 已动态断言零值 `UsageTiming`，DTO 显式编码未知。
- HTTP 错误返回没有既有可提交结果时，内部 timing 事实不会自动成为日志行；这正是保持“不新增失败计费行”的边界。
- 本分工未修改/测试前端展示、CSV/Excel 或 S2–S4；固定 coverage 文档中旧前端验收不能视为本候选已验证。
- 冻结基准缺少 AGENTS.md 引用的 `docs/conventions/codex-outbound-identity.md`，沿用 MIG-001 已登记缺口；本次没有修改出站身份、请求形态或路由策略。
- 父任务已获授权按同一固定候选执行标准 PR CI 与 Security Scan；本子任务未推送、未触发远端门禁。旧 Personal CI 的祖先/PR base 要求留在 UPSTREAM-007，未修改 control checks 或 source json。

## Database impact

没有新增、修改或重命名迁移。`251_add_usage_log_timing.sql` 以及 252–254 保持 MIG-001 的原始字节；runner、hash 白名单和迁移顺序均未改。此次 maintained schema 和 ent 产物仅描述已存在的九个 251 列；生产启用本候选以前须已有 MIG-001 schema。

历史行/旧 writer 默认 `timing_version=0`、六个可空事实为 NULL、状态/来源为 unknown。新 writer 在原有 SQL 写入和冲突路径传递九个事实列，不增加回填或独立写入事务。部分失败仍由原 batch / best-effort / single fallback 返回错误或写入成功行；不会覆盖既存幂等行，也不改变费用扣除与 billing command。

## Rollback

代码回滚按后进先出顺序 revert 修正提交 `df33452e7f74161e1fda20351d92b0834bb474a9` 和实现提交 `b41aff259f1fb9534d3ecfba7c80be1ba9ad9d05`，保留 MIG-001 加法迁移和已采集数据。旧应用仍可以向带新列的表写入，由数据库默认值补齐未知事实；不要 drop 这些列或回滚迁移历史。单独撤销本报告提交不改变运行行为。父任务整合时应保留本实现提交与报告提交的对应关系。

## Acceptance checklist

- [x] 固定新 KIN、旧 KIN/旧 personal 来源和验收基准，未合并旧历史；交付前稳定 tag 复核一致。
- [x] 仅重放 S1 后端；旧首字、费用、重试、扣费、幂等、异步 owner 及新 KIN 模型/协议/图片计费行为保留。
- [x] 新事实有版本、NULL 和 unknown 边界；音频缺失/非法不猜零；未采集的转换入口保持旧版本。
- [x] 取消后冻结、完成后关闭、failed 后 DONE、缺 terminal、retry 和多 turn / response ID 隔离有定向动态证据。
- [x] 最低 HTTP/JSON/SSE/透传/pooled WS/bridge/工具/推理/图片/音频路径有定向覆盖。
- [x] observer 和异步用量复制实际跑 race；真实 SQL/ent/迁移兼容定向 integration 与项目 build 通过。
- [x] 迁移、runner/白名单、依赖文件、前端、发布/部署/审计配置均无 diff；固定合同逐字节移入。
- [x] 父任务 fresh 独立只读复核运行实现，无确认阻断；测试执行由父任务 runtime 证据核验。
- [ ] 父任务同一固定候选的标准 PR CI + Security Scan；其他阶段验收按父任务授权范围执行。

此 checklist 只记录本报告状态，没有修改 `REMEDIATION_PLAN.md` 的任务勾选。
