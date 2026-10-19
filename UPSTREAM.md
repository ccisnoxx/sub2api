# 功能来源与长期维护归属

## 2026-10-19：v0.2.17-klno.1 升级候选

当前工作树来源固定为 `KlN-4096/sub2api` 的 `v0.2.17-klno.1`，annotated tag object 为 `f9c7303984ab3d9adfc985c2c99e59cc9df09cab`，peeled commit 为 `cf0bf0526704bb615745fda76009cb20d85df613`。个人候选从 `2d532db30a94e5d3bff3ef928323186c1d4d93c5`（`v0.2.15-klno.4-tps.1`）准备；该来源与 NOTICE、`deploy/personal-source.json` 一致。下文 I/N/C/T 与发布状态均是带日期的历史记录，不能用作本候选已发布或已上线的证明。

上游重写历史，旧来源 `c555ad918b08cb24ccb2eb45f4ae44a843e24baf` 不是新来源的祖先。此次人工三方投影显式使用该旧来源作为逻辑基线，真实 merge 保留旧 personal 和新 KIN 为两个父系；不修改自动同步的祖先门禁。个人计时、目录、诊断、服务状态、部分用量结算及版本更新 owner 保留；Wire/Ent 从维护源生成。源码 VERSION 仅标识 RC，不分配正式发布标签。

旧 SQL 文件保持原名、原字节；新增 `242_group_i18n.sql`、`243_gemini_cached_contents.sql`、`244_gemini_cached_contents_channel_usage.sql`、`245_gemini_cached_contents_mutation_lease.sql`。新增 SQL 是加法 schema；正式部署前仍须绑定实际 311 行旧账本、315 行候选账本、镜像与可信工具证明。保留当前旧镜像和恢复材料，不根据加法 DDL 推断全业务镜像回退安全。

## 历史固定来源与模块证据


核对日期：2026-10-13 UTC。当前维护任务：REL-002/003/ARCH-002 已验收；PREP-003 的旧稳定基线证据保留在本文末尾。main 管理控制工具和维护记录，personal 保存实际应用。应用 owner、测试和来源一律从固定 I 读取，不用 main 的同名应用文件代替。协调 owner 均为 `ccisnoxx`，不代表原作者、版权归属或上游背书。

## 当前固定来源

| 代号 | 来源 / 用途 | tag / 完整 SHA |
|---|---|---|
| N | [KIN](https://github.com/KlN-4096/sub2api)，当前应用基线 | `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71` |
| I | [personal 应用快照](https://github.com/ccisnoxx/sub2api/tree/c08fba12eb4cd30e65c4177f3acbe20567ecd252)，OBS #32/#31/#34 与门禁修复后的正式发布源码 | `c08fba12eb4cd30e65c4177f3acbe20567ecd252`；正式标签 `v0.2.15-klno.2-tps.2`；生产已部署，GitHub 为 prerelease |
| O | 归档的前一 KIN .2 个人发布，保留源码/Full/Release/digest | `v0.2.15-klno.2-tps.1` / `42907bcab278f93db48d7e1b39b28e28f664569e`；原 Full38274106520、Release410901327不作为新版本资格 |
| A | 归档 personal 基线 / 维护恢复来源 | `v0.2.14-klno.5-tps.2` / `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| K | A 的旧 KIN 来源，与 N 分叉 | `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| P | [Plus](https://github.com/LuckyKuang/sub2api-plus)，固定借鉴参考 | 版本背景 `v0.2.14+custom.002`；标签 commit `a7749f5826ec0a5c6493c47fde9474dc31525f14`；阅读 SHA `90da415c62b94c9417d9ce2b72b1507ed22f0303` |
| C | 本次实际发布/部署控制 revision；原83c250站点控制器保留 | `90aea7ec626aeab41434d72739573aa51d6227ad` |
| T | ARCH-002结构及OBS部署输入/维护绑定修复后的实际工具 revision，无独立工具发行 tag | `90aea7ec626aeab41434d72739573aa51d6227ad` |

N 的 annotated tag object 是 `0c1ede31ec63f9cfc2b72f07db66fb41f540208d`，peeled commit 为 N。P 的 annotated tag object 是 `4d8a25878fa7d4f99ec5bf2f8576dccb023409eb`；标签 commit 与阅读 P 的完整 tree diff 仅 `UPSTREAM.md`，引用的四份功能文档/代码相同，不把阅读 SHA 冒充标签解析结果。

[I 的三字段来源文件](https://github.com/ccisnoxx/sub2api/blob/c08fba12eb4cd30e65c4177f3acbe20567ecd252/deploy/personal-source.json)指定仓库 `KlN-4096/sub2api`、N tag 与 N SHA，与 [I 的 NOTICE](https://github.com/ccisnoxx/sub2api/blob/c08fba12eb4cd30e65c4177f3acbe20567ecd252/NOTICE)及当前 [NOTICE](NOTICE)一致。A、N、累计重放 R 均为 I 的真实祖先；K 不是 N 的祖先。原始上游仍是 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)。固定源码、正式 Release、OCI digest 与生产 308 行账本已绑定，见 [当前OBS发布验收](docs/upstream/records/2026-10-12-obs-release.md)；原 [REL-002](docs/upstream/records/2026-10-12-rel-002.md) 和 [REL-003](docs/upstream/records/2026-10-12-rel-003.md)保持归档。旧 A 保留用于维护恢复，不能仅切回镜像。

发布与迁移事实见 [记录目录](docs/upstream/records/README.md)、[个人发布归档](docs/upstream/records/published-personal.md)、[本次 KIN 迁移](https://github.com/ccisnoxx/sub2api/blob/c54b9497d4bee04215384deb494c5467b917a7db/docs/upstream/records/2026-10-11-kin-v0.2.15-klno.2.md)。下一次操作按 [维护合同](https://github.com/ccisnoxx/sub2api/blob/ded64ff9ac48a1817a8f26af57f69eb382fb4fb0/docs/conventions/personal-maintenance.md)和 [CI-000](docs/ci/CI-000.md)进行；历史自动 CI/发布规则不作为当前流程。

## 第8节OBS整合续作的来源边界

PR #32/#31/#34（含#33语义依赖）已普通整合，最终发布I为 `c08fba12eb4cd30e65c4177f3acbe20567ecd252`。成功[Full38335510831](https://github.com/ccisnoxx/sub2api/actions/runs/38335510831)、[Release38337243568](https://github.com/ccisnoxx/sub2api/actions/runs/38337243568)、新tps.2及immutable digest经独立测试→生产实际验收；旧429产物保留。容量、cleanup和aggregate结构化producer已运行，生产独立本机timer每分钟只读检查、真实相邻调度和测试故障/恢复均已验收；维护者不需要外部通知，没有配置或宣称送达。跨KIN旧JSONB写回已证明丢失 `quality_gateways`，联系人/多协议凭据还有反例，全业务无损条件不成立，旧镜像禁令保持。详情见[OBS验收](docs/upstream/records/2026-10-12-obs-release.md)和[第8节续作](docs/upstream/records/2026-10-12-dod-observability-rollback.md)。

## 当前个人模块 owner 与测试矩阵

以下所有应用文件/测试属于固定 I。测试列确认源码入口存在；本次发布Full对I全部必要job成功，具体定向复用和实际运行证据见OBS记录，不能把文件清单当作逐项本地重跑。S1/S2/S3/S4 分别为计时、目录、诊断、服务状态；S5 为展示/导出。Plus 改动方式沿用结构/概念参考，不能把本地适配写成整模块直接复制或完全自主原创。

| 模块 / 来源方式 | 状态和合同 owner 文件 | 关键接线 / 持久化 | 直接测试入口 |
|---|---|---|---|
| S1：N 基底 + P 结构参考，保留个人严格时点/用量事实 | `backend/internal/service/stream_output_timing.go` 仅负责锁、生命周期和组合快照；五职责见下表；`usage_timing.go` / `usage_log.go` 定义值合同 | HTTP/SSE/透传 `openai_gateway_response_handling.go` / `openai_gateway_passthrough.go`；WS `openai_ws_forwarder_v2.go` / `openai_ws_forwarder_ingress.go` / `openai_ws_http_bridge.go` / `openai_ws_v2_passthrough_adapter.go`（均在 backend/internal/service）；异步 Clone/Commit 仍在 `backend/internal/handler/openai_gateway_handler.go`；SQL insert/query 为 `backend/internal/repository/usage_log_repo_insert.go` / `usage_log_repo_query.go` | `backend/internal/service/stream_output_timing_test.go`、`openai_usage_timing_test.go`、`openai_usage_timing_billing_test.go`（同 service）；`backend/internal/handler/openai_gateway_usage_context_test.go`；`backend/internal/repository/usage_log_timing_integration_test.go` |
| S2：N 授权/计费 + P 目录组织概念参考 | `backend/internal/handler/available_channel_handler.go` / `available_model_catalog.go`；`backend/internal/service/channel_catalog.go` / `catalog_pricing.go` | `backend/internal/service/model_pricing_resolver.go` / `billing_context_schedule.go` 继续拥有真实报价规则；`backend/internal/server/routes/user.go`；`backend/cmd/server/wire_gen.go` 为生成 DI，按维护源重新生成 | `backend/internal/handler/available_model_catalog_test.go`；`backend/internal/service/catalog_pricing_test.go`；`backend/internal/repository/available_catalog_contract_integration_test.go` |
| S3：N 实际调度 + P 诊断结构参考 | `backend/internal/service/routing_diagnostics.go` / `routing_diagnostics_validation.go`；真实选择 `openai_account_scheduler.go`（同 service） | WS turn `backend/internal/handler/openai_ws_routing_diagnostics.go`；采集入口 `ops_error_logger.go`（同 handler）；写入终检/详情 `backend/internal/service/ops_service.go`；SQL `backend/internal/repository/ops_repo.go` | `backend/internal/service/routing_diagnostics_test.go` / `openai_routing_diagnostics_test.go`；`backend/internal/handler/admin/ops_routing_diagnostics_handler_test.go`；`backend/internal/repository/ops_repo_routing_diagnostics_integration_test.go` |
| S4：P 终态/水位/事件概念参考，KIN 下独立管理员实现 | `backend/internal/service/service_status_observation.go` 拥有请求最终冻结与 Commit；`service_status.go` / `service_status_aggregator.go`（同 service）；归一化 `backend/internal/repository/service_status_normalize.go` | 事务/config/epoch/watermark：`backend/internal/repository/service_status_repo.go`；事件 `service_status_incidents.go`（同 repository）；JWT `backend/internal/handler/service_status_handler.go`；路由 `backend/internal/server/routes/admin.go` | `backend/internal/service/service_status_observation_test.go` / `service_status_aggregator_test.go`；`backend/internal/repository/service_status_repo_integration_test.go`；`backend/internal/server/routes/service_status_routes_jwt_test.go` |
| S5：个人展示/导出，P helper 结构参考并按 KIN 值合同适配 | `frontend/src/utils/usageTiming.ts`；`frontend/src/components/admin/usage/UsageTable.vue`；导出 `frontend/src/views/admin/UsageView.vue` / `frontend/src/views/user/UsageView.vue` | 目录 `frontend/src/api/channels.ts` / `frontend/src/views/user/AvailableChannelsView.vue`；诊断 `frontend/src/views/admin/ops/components/OpsRoutingDiagnosticsPanel.vue`；状态 `frontend/src/api/admin/serviceStatus.ts` / `frontend/src/views/admin/ServiceStatusView.vue` / `frontend/src/router/index.ts` | `frontend/src/utils/__tests__/usageTiming.spec.ts`；`frontend/src/views/admin/__tests__/UsageView.spec.ts` / `ServiceStatusView.spec.ts`；`frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`；`frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts` |
| 个人更新识别：UPSTREAM-008 已迁回 | `backend/internal/service/update_service.go` / `frontend/src/components/common/VersionBadge.vue` | 个人 Release 查询指向本 fork；container/manual 拒绝原地更新及回退，个人五段版本数值比较；原REL-002管理员页面验证过tps.1 Release链接，本轮复用未变UI证据 | `backend/internal/service/update_service_test.go` / `frontend/src/components/common/__tests__/VersionBadge.spec.ts`；真实 handler 409 拒绝合同见 [UPSTREAM-008](docs/upstream/UPSTREAM-008.md) |
| OBS运行监控：本地适配，结构化事件不进入Ops系统日志 | `backend/internal/repository/service_status_monitor.go`；前置失败/整轮起点为 `backend/internal/service/service_status_aggregator.go` | `tools/service-status-watch.py` 独立只读水位/来源/配置检查；`tools/service-status-capacity.sql`；部署的本机timer不含通知路由 | `backend/internal/repository/service_status_aggregator_monitor_test.go` / `service_status_monitor_integration_test.go`；`tools/test_service_status_watch.py`；真实timer、故障与恢复见OBS记录 |

### ARCH-001 后 S1 的五个责任 owner

路径均在 I 的 `backend/internal/service/`；每行实现和对应测试独立维护。emitter 只上报 attempt 候选，请求最终冻结、计费、队列/存储仍属于原 owner；首次终态冻结后可继续接受真实 drain 用量。

| 职责 | 实现文件 / 符号 | 测试文件 |
|---|---|---|
| 协议事实 | `responses_protocol_parser.go` / `parseResponsesEvent`、`parseResponsesJSON` | `responses_protocol_parser_test.go` |
| 输出时点 | `responses_output_timing_tracker.go` / `observe`、`snapshot` | `responses_output_timing_tracker_test.go` |
| 首次终态 | `responses_completion_classifier.go` / `freeze`、`snapshot` | `responses_completion_classifier_test.go` |
| 已接受用量 / 音频事实 | `responses_usage_parser.go` / `parseResponsesAcceptedUsage`；`responses_usage_accumulator.go` / `observeAccepted` | `responses_usage_parser_test.go` / `responses_usage_accumulator_test.go` |
| 服务状态 attempt 候选 | `responses_service_observation_emitter.go` / `emitFrozenTerminal` | `responses_service_observation_emitter_test.go` |

### ARCH-004 独立开关

| 功能 | I 的配置 owner / 默认 | 关闭、失败及持久化边界 | 测试入口 |
|---|---|---|---|
| 模型目录 | 继承 `available_channels_enabled`，`backend/internal/service/setting_parse.go` / `setting_public.go`；默认 false | 认证优先；catalog 配置失败/关闭分别为 `MODEL_CATALOG_UNAVAILABLE` / `MODEL_CATALOG_DISABLED`、503，在授权/渠道/报价读取前短路；旧数组成功空值合同保留 | `backend/internal/handler/available_model_catalog_test.go` |
| 路由诊断 | `backend/internal/service/ops_routing_feature.go`，缺 key 默认 true，非法值失败；`ops_service.go` 持有快照 | 冷启动配置失败只关诊断；持久化成功后发布；关闭时不建新 builder，写入终检/历史详情裁剪；不撤销已开始 DB 写入或删除历史 | `backend/internal/service/ops_routing_feature_test.go`；`backend/internal/server/routes/personal_feature_flags_routes_test.go` |
| 服务状态 | `service_status_config.enabled`，`backend/migrations/254_service_status.sql` 默认 false；仓储 config/epoch owner | 每轮 disabled/无平台/配置读取失败跳过 Aggregate，ticker 保留以发现重开；Snapshot disabled；配置 API 可访问，重开重置连续计数/epoch，不删历史/schema | `backend/internal/service/service_status_aggregator_test.go`；`backend/internal/repository/service_status_repo_integration_test.go` |

`251_add_usage_log_timing.sql`、`252_add_ops_routing_diagnostics.sql`、`253_service_status_observation.sql`、`254_service_status.sql` 在 I 的 backend/migrations 保持原身份；关闭开关、删薄包装或文档收敛均不授权改名、删列、重写 checksum/账本。

## 当前控制工具 owner 与测试

以下发布文件属于固定 C、部署模块属于固定 T 的 main，不属于 I 应用版本；当前执行语义以 CONTRIBUTING_PERSONAL/CI-000 为准，无独立工具发行 tag。main 增量只按需要同步控制文件到 personal，不合入整棵应用树。

| 责任 | main owner 文件 | 直接测试入口 |
|---|---|---|
| 上游候选/来源绑定 | `.github/personal-sync/sync.py` / `check_binding.py`；`.github/workflows/sync-upstream.yml` / `personal-ci.yml` | `.github/personal-sync/test_sync.py`；`.github/release-tools/test_ci_concurrency.py` |
| 手动发布/来源材料 | `.github/release-tools/personal_release.py` / `release_matrix.py` / `release-images.sh`；`.github/workflows/personal-release.yml` / `release.yml` | `.github/release-tools/test_personal_release.py` / `test_release_matrix.py` / `test_release_control.py` |
| 主机部署/兼容回退 | `deploy/personal/deploy_hostdzire.py` / `deploy-hostdzire.sh`；`deploy_lifecycle.py`（唯一状态owner）、`deploy_compatibility.py`、`deploy_database.py`、`deploy_images.py`、`deploy_health.py`；`.github/workflows/personal-deploy-ci.yml` | `deploy/personal/test_deploy_hostdzire.py` |
| 部署配置/可信运输/只读预检 | `deploy/personal/hostdzire.json` / `deploy_config.py`；`deploy_trust.py` / `deploy_transport.py` / `deploy_local.py` | `deploy/personal/test_arch002.py`；95合同和真实dry-run见[ARCH-002](docs/upstream/records/2026-10-12-arch-002.md) |

## Plus 停止跟踪与原生收敛决策

四模块均固定 P 并停止自动追随 Plus。新借鉴必须单独确认版本、完整源 SHA、权限/计费/终态/数据合同与必要证据；此策略不声称执行过外部订阅或 workflow 设置变更。冻结的是借鉴范围，不是承诺永不修复本地缺陷。

| Plus 参考模块 | 固定参考 | 当前保留 / 不采用范围 |
|---|---|---|
| 计时/完成 | [USAGE_TIMING.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/USAGE_TIMING.md) | 保留旧 first_token 与独立 strict/NULL/终态；只声明原生 Responses HTTP/SSE/WS 已接入口；不复制全平台、历史清理、转换入口、probe/gwpool/Live 采集 |
| 模型/价格目录 | [AVAILABLE_CHANNELS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/AVAILABLE_CHANNELS.md) | 授权模型来自 KIN，价格复用 billing；不扩张 provider discovery/模型集合/媒体单位；ARCH-004 的 disabled503 不沿用 Plus 成功空目录 |
| 路由诊断 | [ERROR_REQUEST_DIAGNOSTICS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/ERROR_REQUEST_DIAGNOSTICS.md) | 保留当前 turn/attempt、nil/0/coverage、错误链与管理员白名单；不扩展身份、历史纠正或额外 telemetry |
| 服务状态 | [CHANNEL_MONITOR.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/CHANNEL_MONITOR.md) | 独立管理员 JWT/表/配置，健康由终态/缺口/样本/错误率决定；不采用全登录用户可见或 TTFT/TPS 健康阈值，不替代 V1/V2 |

当前采集合同/覆盖见 [I spec](https://github.com/ccisnoxx/sub2api/blob/c08fba12eb4cd30e65c4177f3acbe20567ecd252/openspec/changes/adopt-plus-usage-and-diagnostics/spec.md)、[I coverage](https://github.com/ccisnoxx/sub2api/blob/c08fba12eb4cd30e65c4177f3acbe20567ecd252/openspec/changes/adopt-plus-usage-and-diagnostics/coverage.md)。目录/价格/诊断/状态冻结合同仍在固定 [A 合同目录](https://github.com/ccisnoxx/sub2api/tree/3d5e1fde82707900a21f2b5538b112c7bab3c04a/openspec/changes/adopt-plus-usage-and-diagnostics)，不制造 I 中不存在的本地链接。

| N 已具备能力 | 实际 owner / 收敛方向 | 仍需保留的个人增量 |
|---|---|---|
| recording | `backend/internal/pkg/upstreamrecord/recorder.go` / `backend/internal/service/openai_upstream_recording.go`；沿 native opt-in/recorder 接线，避免另建副本；`recorder_test.go` 在同 pkg | 原生录制不是个人 S1–S4 新增或吸收成果，不能用它证明严格时点/终态采集 |
| Codex CatalogConfig | `frontend/src/utils/codexCatalogConfig.ts` / `frontend/src/components/keys/UseKeyModal.vue`；复用解析 helper；测试 `frontend/src/utils/__tests__/codexCatalogConfig.spec.ts` | 密钥使用 manifest 不是 S2 授权报价目录，保留新 DTO/未知价格与授权合同 |
| 基础耗时/旧首字 | `backend/ent/schema/usage_log.go` / `backend/internal/service/openai_gateway_usage.go`；S1 复用既有 Duration 起点与旧字段 | strict 首末、媒体 NULL、首次终态/来源独立保留；不折叠回旧首字 |
| 原渠道授权/计费 resolver | `backend/internal/handler/available_channel_handler.go` / `backend/internal/service/model_pricing_resolver.go`；S2 复用 native billing/request snapshot seam，不复制第二套计费 | 不删个人 view=catalog、白名单 DTO 与不确定报价语义 |
| 调度指标 / ChannelMonitor V1/V2 | `backend/internal/service/openai_account_scheduler.go` / `backend/internal/server/routes/admin.go` / `user.go`；保留 native owner，个人 observer 不反向接管它们 | 不是 S3 可空实测 pool/attempt，也不是 S4 真实终态/独立状态 |

N 无当前 timing_version/strict/media/终态来源字段、routing_diagnostics、service_status_observation 和 CatalogPricingResolver 对应实现。没有证据称 S1–S4 整模块已被 N 原生吸收；上表若干能力在 K/A/N 已继承，不能改写成个人增量后来被吸收。后续目标 KIN 确有等价合同后，先收敛共享逻辑、移除重复适配和重复测试，再按独立迁移方案处理 schema，不能据同名功能删数据。

### 个人更新行为已迁回

UPSTREAM-008 已普通快进整合到固定 I。个人更新服务、版本徽章、来源缓存及 HTTP 409 能力拒绝沿用其定向验证和独立复核；原 REL-002 生产页面确认过 `v0.2.15-klno.2-tps.1`、本 fork Release 链接和固定 digest 提示；本轮tps.2由固定源码、OCI与公开API验收绑定，没有重跑浏览器版本页。没有启用容器内二进制替换。原 ARCH-005 登记的待迁回状态属于历史，不代表当前 I。

下一次 KIN 来源检查日期：**2026-10-26**。届时按 CI-000 人工确认候选；本次没有升级新 KIN，也没有创建自动同步或发布任务。

## PREP-003 旧稳定基线历史（2026-10-10）

以下完整保留当时 A/K/F 的来源、引入和验证边界。其“当前”只指 2026-10-10 固定快照；新应用和当前手动流程以上文 I/N/C 与 CI-000 为准，尤其旧 A 的个人更新能力未在 I 保留。

### 固定来源

| 代号 | 来源仓库 | 标签或用途 | 完整提交 |
| --- | --- | --- | --- |
| K | [KlN-4096/sub2api](https://github.com/KlN-4096/sub2api) | 应用基线 `v0.2.14-klno.5` | `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| P | [LuckyKuang/sub2api-plus](https://github.com/LuckyKuang/sub2api-plus) | 固定阅读与适配参考，版本背景为 `v0.2.14+custom.002` | `90da415c62b94c9417d9ce2b72b1507ed22f0303` |
| A | [ccisnoxx/sub2api](https://github.com/ccisnoxx/sub2api) | 已发布应用 `v0.2.14-klno.5-tps.2`，位于 `personal` | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| F | [ccisnoxx/sub2api](https://github.com/ccisnoxx/sub2api) | 本次核对的 `main` 控制工具快照，无独立工具发行标签 | `f5c4c3304106040a01a32c5e6d4ff18f3ec49930` |

K 与 [A 的来源记录](https://github.com/ccisnoxx/sub2api/blob/3d5e1fde82707900a21f2b5538b112c7bab3c04a/deploy/personal-source.json)一致；K 是 A 的祖先。原始上游为 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)。本文对继承代码以可核实的 K 快照定位，不把 K 的每一文件都推断成原始上游独创。

#### Plus 标签与阅读提交的区别

远端 `v0.2.14+custom.002` 是 annotated tag，tag object 为 `4d8a25878fa7d4f99ec5bf2f8576dccb023409eb`，实际指向 `a7749f5826ec0a5c6493c47fde9474dc31525f14`。P 是随后合入的发布来源文档提交，其第一父提交就是该标签提交。二者的完整 tree diff 只有 `UPSTREAM.md`，下面引用的应用源码与功能文档相同。

因此保留已有适配记录的固定阅读提交 P，但不再声称 P 就是标签解析结果。发行标签提交与阅读提交同时登记于 [NOTICE](NOTICE)；未来复核使用完整 SHA，不追随移动的 `main`，也不据此推断标签曾被改写。历史 [适配计划](https://github.com/ccisnoxx/sub2api/blob/f5c4c3304106040a01a32c5e6d4ff18f3ec49930/openspec/changes/adopt-plus-usage-and-diagnostics/plan.md)中旧的“标签 / 提交”简写须按本节理解。

### 来源矩阵

来源仓库、tag、commit 用上表代号绑定；“无独立标签”表示真实的维护提交，不借用应用版本冒充工具发行版。所有行的当前维护协调 owner 均为本仓库维护者 `ccisnoxx`。

改动方式含义：**直接复制**需要确认搬运来源与相同内容；**结构参考**采用字段、接口或计算组织并做本地适配；**概念参考**采用需求与语义，按 KIN 合同实现；**自主实现**指有本 fork 引入记录的新增实现，不把其依赖的继承代码算作自主部分。直接继承的 KIN 文件另行标明，不冒充个人新增模块。该分类不替代逐文件版权审核。

| 模块 | 来源仓库 | 来源 tag | 来源 commit | 改动方式 | 当前 owner |
| --- | --- | --- | --- | --- | --- |
| 用量计时、完成状态与 TPS/导出 | K、P、A | K、P 的版本背景、A | K、P；当前实现 A | Plus 指标、helper 与 schema 的结构参考；保留 KIN 旧首字，新增独立严格首字、可空音频与终态快照，按原转发及计费 owner 适配 | `ccisnoxx` |
| 授权模型与价格目录 | K、P、A | K、P 的版本背景、A | K、P；当前实现 A | Plus opt-in 目录组织的概念参考；使用 KIN 分组授权、映射与真实计费探针，本地实现目录 DTO 和页面 | `ccisnoxx` |
| 结构化路由诊断 | K、P、A | K、P 的版本背景、A | K、P；当前实现 A | Plus 决策快照字段的结构参考；接入 KIN 实际选择分支、请求/逻辑 turn、Ops 队列及管理员白名单 | `ccisnoxx` |
| 管理员真实请求服务状态 | K、P、A | K、P 的版本背景、A | K、P；当前实现 A | Plus V3 终态、分钟事实、水位与故障事件的概念参考；本地 observation key、独立表/开关与管理员权限，不复制 Plus 全登录用户可见或 TTFT 健康判定 | `ccisnoxx` |
| 个人版本识别与更新入口 | K、A | K、A | K；当前实现 A | KIN 更新服务的本地适配；个人版本使用 fork Release、容器/manual 模式，拒绝二进制原地更新；版本解析脚本直接继承 K | `ccisnoxx` |
| 同步与个人发布门禁 | F；应用侧 A | 控制工具无独立标签；应用 A | F、A | 本 fork 新增同步、来源/祖先、同 SHA CI 和版本占用门禁；个人发布 helper 为自主实现，基础发布矩阵直接继承共享历史 | `ccisnoxx` |
| 主机部署与兼容回滚 | F | 无独立工具发行标签 | F | 本 fork 自主实现的固定 digest、应用 revision、迁移兼容和部署生命周期工具；不是通用安装器 | `ccisnoxx` |
| 已有录制器、Codex 目录配置与基础发布矩阵 | K；F 的发布矩阵 | K；F 无独立标签 | K、A、F | 继承边界：A 的录制器、目录配置与版本解析脚本以及 A/F 的发布矩阵与 K 相同；不把这些现有能力列为 S1–S4 新增原创功能 | `ccisnoxx` |

### 模块证据与重放入口

以下代码路径是对应固定树中的真实文件。应用链接可从 [A 源码树](https://github.com/ccisnoxx/sub2api/tree/3d5e1fde82707900a21f2b5538b112c7bab3c04a)进入，控制工具从 [F 源码树](https://github.com/ccisnoxx/sub2api/tree/f5c4c3304106040a01a32c5e6d4ff18f3ec49930)进入。实施和验证历史见 [执行证据](https://github.com/ccisnoxx/sub2api/blob/f5c4c3304106040a01a32c5e6d4ff18f3ec49930/openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md)；本文不将历史测试通过当作新 KIN 已验证。

#### 用量计时与展示

- Plus 参考：[指标合同](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/USAGE_TIMING.md)、[helper](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/frontend/src/utils/usageTiming.ts)、[schema](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/backend/ent/schema/usage_log.go)。本地 [冻结合同](openspec/changes/adopt-plus-usage-and-diagnostics/spec.md)与[覆盖表](openspec/changes/adopt-plus-usage-and-diagnostics/coverage.md)明确只声明原生 Responses HTTP/SSE、WS 第一批。
- 本地引入：数据合同 `0e2ea47950564aa05dd6e8eca374874725047057`，采集/展示 `12499e1a78fd3fee5d2afd61559d73bd8e770136`。
- 重放文件：`backend/internal/service/usage_timing.go`、`backend/internal/service/stream_output_timing.go`、`backend/internal/service/openai_gateway_usage.go`、`backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/repository/usage_log_repo.go`、`backend/ent/schema/usage_log.go`、`backend/internal/handler/dto/mappers.go`、`frontend/src/utils/usageTiming.ts`、`frontend/src/components/admin/usage/UsageTable.vue`、`frontend/src/views/admin/UsageView.vue`、`frontend/src/views/user/UsageView.vue`。
- 迁移：`backend/migrations/251_add_usage_log_timing.sql`。WS 接线和未接入范围按覆盖表逐条核对，不把同名迁移直接复制到新基线。
- 测试入口：`backend/internal/service/stream_output_timing_test.go`、`backend/internal/service/openai_usage_timing_test.go`、`backend/internal/service/openai_usage_timing_billing_test.go`、`backend/internal/service/openai_ws_v2/usage_timing_callback_test.go`、`frontend/src/utils/__tests__/usageTiming.spec.ts`、`frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts`。

#### 模型与价格目录

- Plus 参考：[AVAILABLE_CHANNELS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/AVAILABLE_CHANNELS.md)。本地[目录合同](https://github.com/ccisnoxx/sub2api/blob/f5c4c3304106040a01a32c5e6d4ff18f3ec49930/openspec/changes/adopt-plus-usage-and-diagnostics/catalog-contract.md)与[价格合同](https://github.com/ccisnoxx/sub2api/blob/f5c4c3304106040a01a32c5e6d4ff18f3ec49930/openspec/changes/adopt-plus-usage-and-diagnostics/pricing-contract.md)指定 KIN 权限和计费 owner。
- 本地引入：价格 `e82287300d1b7cc625295c5307ebaa83c707c019`、DTO `49c209a77059d2927f78bbd31c1dd479b509dc6f`、页面 `b8cf49ca25007cbc23338a9443b71df4c0411155`。
- 重放文件：`backend/internal/service/catalog_pricing.go`、`backend/internal/service/channel_catalog.go`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/billing_context_schedule.go`、`backend/internal/handler/available_model_catalog.go`、`backend/internal/handler/available_channel_handler.go`、`frontend/src/api/channels.ts`、`frontend/src/utils/modelCatalog.ts`、`frontend/src/components/channels/ModelCatalogCard.vue`、`frontend/src/views/user/AvailableChannelsView.vue`。
- 测试入口：`backend/internal/service/catalog_pricing_test.go`、`backend/internal/handler/available_model_catalog_test.go`、`frontend/src/api/__tests__/channels.catalog.spec.ts`、`frontend/src/utils/__tests__/modelCatalog.spec.ts`、`frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`。

#### 路由诊断

- Plus 参考：[ERROR_REQUEST_DIAGNOSTICS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/ERROR_REQUEST_DIAGNOSTICS.md)。本地[诊断合同](https://github.com/ccisnoxx/sub2api/blob/f5c4c3304106040a01a32c5e6d4ff18f3ec49930/openspec/changes/adopt-plus-usage-and-diagnostics/routing-diagnostics-contract.md)保留未知与实测空池的区别。
- 本地引入：producer `cee1e908261c68880040b740aae8054410e03070`、存储/DTO `ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`、详情 `d23474171035320eda06a35d76d455d7f8d7aae4`；后续重放须保留 `da581c6846d1bf89926ca9730abf2290ea8b65ea` 的 native WS 安全重放修复。
- 重放文件：`backend/internal/service/routing_diagnostics.go`、`backend/internal/service/routing_diagnostics_validation.go`、`backend/internal/service/openai_account_scheduler.go`、`backend/internal/handler/openai_ws_routing_diagnostics.go`、`backend/internal/handler/ops_error_logger.go`、`backend/internal/repository/ops_repo.go`、`frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`。
- 迁移：`backend/migrations/252_add_ops_routing_diagnostics.sql`。
- 测试入口：`backend/internal/service/routing_diagnostics_test.go`、`backend/internal/service/openai_routing_diagnostics_test.go`、`backend/internal/service/ops_routing_diagnostics_dto_test.go`、`backend/internal/handler/admin/ops_routing_diagnostics_handler_test.go`、`backend/internal/service/openai_ws_native_resume_test.go`、`frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts`。

#### 服务状态

- Plus 参考：[CHANNEL_MONITOR.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/CHANNEL_MONITOR.md)。本地[独立合同](https://github.com/ccisnoxx/sub2api/blob/f5c4c3304106040a01a32c5e6d4ff18f3ec49930/openspec/changes/adopt-plus-usage-and-diagnostics/service-status-contract.md)要求管理员可见、独立监控键、未知与样本不足，不替换原 V1/V2。
- 本地引入：`35962a802fdc499639c9c861072f25774ccba50b`；当前实现以 A 为准，包含后续完整分钟和源日志缺口修复。
- 重放文件：`backend/internal/service/service_status.go`、`backend/internal/service/service_status_aggregator.go`、`backend/internal/service/service_status_observation.go`、`backend/internal/service/service_status_source_reporter.go`、`backend/internal/repository/service_status_repo.go`、`backend/internal/repository/service_status_normalize.go`、`backend/internal/repository/service_status_incidents.go`、`backend/internal/handler/service_status_handler.go`、`backend/internal/server/routes/admin.go`、`frontend/src/api/admin/serviceStatus.ts`、`frontend/src/views/admin/ServiceStatusView.vue`、`frontend/src/router/index.ts`。
- 迁移：`backend/migrations/253_service_status_observation.sql`、`backend/migrations/254_service_status.sql`。
- 测试入口：`backend/internal/service/service_status_test.go`、`backend/internal/service/service_status_observation_test.go`、`backend/internal/repository/service_status_source_integration_test.go`、`backend/internal/repository/service_status_repo_integration_test.go`、`backend/internal/handler/service_status_handler_test.go`、`backend/internal/server/routes/service_status_routes_test.go`、`frontend/src/views/admin/__tests__/ServiceStatusView.spec.ts`。

#### 个人版本、同步、发布与部署

- 应用适配：`backend/internal/service/update_service.go`、`frontend/src/components/common/VersionBadge.vue`；测试入口为 `backend/internal/service/update_service_test.go`、`frontend/src/components/common/__tests__/VersionBadge.spec.ts`。`backend/scripts/resolve-version.sh` 与 K 相同。
- F 同步工具：`.github/personal-sync/sync.py`、`.github/personal-sync/check_binding.py`、`.github/workflows/sync-upstream.yml`、`.github/workflows/personal-ci.yml`；测试入口 `.github/personal-sync/test_sync.py`。本地引入可追溯至 `624f3d6a2564cf250dd1cf5ce6733c45b2693952`。
- F 个人发布工具：`.github/release-tools/personal_release.py`、`.github/release-tools/release-images.sh`、`.github/workflows/personal-release.yml`、`.github/workflows/release.yml`；测试入口 `.github/release-tools/test_personal_release.py`、`.github/release-tools/test_release_matrix.py`。个人 helper 的本地引入为 `b7794d6f25f25e9d28f82b7cda1b82325ba5e96d`，后续门禁修正见 `9a358653a8bab6b0679184a8806f2c786d26aba2`。基础 `.github/release-tools/release_matrix.py` 在 K、A、F 字节相同，其共享历史引入提交为 `0892ef3a9a320dad79ff87930dcd5ae56a9a140a`，不因出现在上游就抹去已有贡献历史。
- F 部署工具：`deploy/personal/deploy-hostdzire.sh`、`deploy/personal/deploy_hostdzire.py`、`.github/workflows/personal-deploy-ci.yml`；测试入口 `deploy/personal/test_deploy_hostdzire.py`。本地引入 `2e5da44431efe152b4b6105a80b7a84f233436a6`，当前绑定修正截至 `e3b3e8e284e2d9b0ff8f972650807c4d40863c64`；详见[部署说明](deploy/personal/README.md)。本次不执行主机操作。

### 人工复核结论与边界

- 已核对 P 的四份功能文档、计时 helper/schema、本地合同与引入提交，以及 A 的实际文件/测试入口。计时 helper 不是原文件直接搬运：P 使用同名 `first_token_ms`，A 保留旧字段并读取 `strict_first_token_ms`，返回类型、适用范围、空值与导出接口均有本地适配；不能宣称 Plus 的全平台采集已在本 fork 实现。
- K 中已有的 `backend/internal/pkg/upstreamrecord/recorder.go`、`frontend/src/utils/codexCatalogConfig.ts` 和 `backend/scripts/resolve-version.sh` 与 A 字节相同；它们不属于个人新增核心模块。公开原始上游关系保留，逐文件的更早作者归属不作猜测。
- 本文没有发现足以把 S1–S4 整模块标为“直接复制”的记录；结构/概念参考明确保留 Plus 归属，不据文件不同或相同许可证宣称全部自主原创。局部代码版权头、许可证副本与发布产物材料留给 LEGAL-001/LEGAL-002 审核，不在此给出完成结论。
- PREP-001 已保存当前 307 条迁移身份/执行时间、schema、固定源码/镜像及私有备份，并完成非生产完整恢复与读写回滚；见[已验证基线](BASELINE.md)。本文的来源核对与恢复基线分别提供代码来源和恢复证据，不将它们当作新 KIN 已完成验证。
- 重放到新 KIN 时逐模块核对上述 owner、接口、权限和迁移身份。应用测试需在实际新候选中按测试文件/名称定向执行；本文仅核对入口存在，不声称已重放、已编译或已通过这些测试。
