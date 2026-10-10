# 功能来源与维护归属

核对日期：2026-10-10。任务：PREP-003。本文记录当前已交付代码的来源和重放入口；不是新 KIN 迁移方案，也不声明上游为本 fork 提供支持或背书。

`main` 管理同步、发布、部署工具和维护记录；`personal` 保存实际应用。下面的应用文件均按固定应用提交核对，不能用 `main` 中的同名文件代替。当前 owner 表示本 fork 的维护协调责任，不是原作者或版权归属认定。

## 固定来源

| 代号 | 来源仓库 | 标签或用途 | 完整提交 |
| --- | --- | --- | --- |
| K | [KlN-4096/sub2api](https://github.com/KlN-4096/sub2api) | 应用基线 `v0.2.14-klno.5` | `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| P | [LuckyKuang/sub2api-plus](https://github.com/LuckyKuang/sub2api-plus) | 固定阅读与适配参考，版本背景为 `v0.2.14+custom.002` | `90da415c62b94c9417d9ce2b72b1507ed22f0303` |
| A | [ccisnoxx/sub2api](https://github.com/ccisnoxx/sub2api) | 已发布应用 `v0.2.14-klno.5-tps.2`，位于 `personal` | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| F | [ccisnoxx/sub2api](https://github.com/ccisnoxx/sub2api) | 本次核对的 `main` 控制工具快照，无独立工具发行标签 | `f5c4c3304106040a01a32c5e6d4ff18f3ec49930` |

K 与 [A 的来源记录](https://github.com/ccisnoxx/sub2api/blob/3d5e1fde82707900a21f2b5538b112c7bab3c04a/deploy/personal-source.json)一致；K 是 A 的祖先。原始上游为 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api)。本文对继承代码以可核实的 K 快照定位，不把 K 的每一文件都推断成原始上游独创。

### Plus 标签与阅读提交的区别

远端 `v0.2.14+custom.002` 是 annotated tag，tag object 为 `4d8a25878fa7d4f99ec5bf2f8576dccb023409eb`，实际指向 `a7749f5826ec0a5c6493c47fde9474dc31525f14`。P 是随后合入的发布来源文档提交，其第一父提交就是该标签提交。二者的完整 tree diff 只有 `UPSTREAM.md`，下面引用的应用源码与功能文档相同。

因此保留已有适配记录的固定阅读提交 P，但不再声称 P 就是标签解析结果。发行标签提交与阅读提交同时登记于 [NOTICE](NOTICE)；未来复核使用完整 SHA，不追随移动的 `main`，也不据此推断标签曾被改写。历史 [适配计划](openspec/changes/adopt-plus-usage-and-diagnostics/plan.md)中旧的“标签 / 提交”简写须按本节理解。

## 来源矩阵

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

## 模块证据与重放入口

以下代码路径是对应固定树中的真实文件。应用链接可从 [A 源码树](https://github.com/ccisnoxx/sub2api/tree/3d5e1fde82707900a21f2b5538b112c7bab3c04a)进入，控制工具从 [F 源码树](https://github.com/ccisnoxx/sub2api/tree/f5c4c3304106040a01a32c5e6d4ff18f3ec49930)进入。实施和验证历史见 [执行证据](openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md)；本文不将历史测试通过当作新 KIN 已验证。

### 用量计时与展示

- Plus 参考：[指标合同](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/USAGE_TIMING.md)、[helper](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/frontend/src/utils/usageTiming.ts)、[schema](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/backend/ent/schema/usage_log.go)。本地 [冻结合同](openspec/changes/adopt-plus-usage-and-diagnostics/spec.md)与[覆盖表](openspec/changes/adopt-plus-usage-and-diagnostics/coverage.md)明确只声明原生 Responses HTTP/SSE、WS 第一批。
- 本地引入：数据合同 `0e2ea47950564aa05dd6e8eca374874725047057`，采集/展示 `12499e1a78fd3fee5d2afd61559d73bd8e770136`。
- 重放文件：`backend/internal/service/usage_timing.go`、`backend/internal/service/stream_output_timing.go`、`backend/internal/service/openai_gateway_usage.go`、`backend/internal/service/openai_gateway_response_handling.go`、`backend/internal/repository/usage_log_repo.go`、`backend/ent/schema/usage_log.go`、`backend/internal/handler/dto/mappers.go`、`frontend/src/utils/usageTiming.ts`、`frontend/src/components/admin/usage/UsageTable.vue`、`frontend/src/views/admin/UsageView.vue`、`frontend/src/views/user/UsageView.vue`。
- 迁移：`backend/migrations/251_add_usage_log_timing.sql`。WS 接线和未接入范围按覆盖表逐条核对，不把同名迁移直接复制到新基线。
- 测试入口：`backend/internal/service/stream_output_timing_test.go`、`backend/internal/service/openai_usage_timing_test.go`、`backend/internal/service/openai_usage_timing_billing_test.go`、`backend/internal/service/openai_ws_v2/usage_timing_callback_test.go`、`frontend/src/utils/__tests__/usageTiming.spec.ts`、`frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts`。

### 模型与价格目录

- Plus 参考：[AVAILABLE_CHANNELS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/AVAILABLE_CHANNELS.md)。本地[目录合同](openspec/changes/adopt-plus-usage-and-diagnostics/catalog-contract.md)与[价格合同](openspec/changes/adopt-plus-usage-and-diagnostics/pricing-contract.md)指定 KIN 权限和计费 owner。
- 本地引入：价格 `e82287300d1b7cc625295c5307ebaa83c707c019`、DTO `49c209a77059d2927f78bbd31c1dd479b509dc6f`、页面 `b8cf49ca25007cbc23338a9443b71df4c0411155`。
- 重放文件：`backend/internal/service/catalog_pricing.go`、`backend/internal/service/channel_catalog.go`、`backend/internal/service/model_pricing_resolver.go`、`backend/internal/service/billing_context_schedule.go`、`backend/internal/handler/available_model_catalog.go`、`backend/internal/handler/available_channel_handler.go`、`frontend/src/api/channels.ts`、`frontend/src/utils/modelCatalog.ts`、`frontend/src/components/channels/ModelCatalogCard.vue`、`frontend/src/views/user/AvailableChannelsView.vue`。
- 测试入口：`backend/internal/service/catalog_pricing_test.go`、`backend/internal/handler/available_model_catalog_test.go`、`frontend/src/api/__tests__/channels.catalog.spec.ts`、`frontend/src/utils/__tests__/modelCatalog.spec.ts`、`frontend/src/views/user/__tests__/AvailableChannelsView.spec.ts`。

### 路由诊断

- Plus 参考：[ERROR_REQUEST_DIAGNOSTICS.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/ERROR_REQUEST_DIAGNOSTICS.md)。本地[诊断合同](openspec/changes/adopt-plus-usage-and-diagnostics/routing-diagnostics-contract.md)保留未知与实测空池的区别。
- 本地引入：producer `cee1e908261c68880040b740aae8054410e03070`、存储/DTO `ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`、详情 `d23474171035320eda06a35d76d455d7f8d7aae4`；后续重放须保留 `da581c6846d1bf89926ca9730abf2290ea8b65ea` 的 native WS 安全重放修复。
- 重放文件：`backend/internal/service/routing_diagnostics.go`、`backend/internal/service/routing_diagnostics_validation.go`、`backend/internal/service/openai_account_scheduler.go`、`backend/internal/handler/openai_ws_routing_diagnostics.go`、`backend/internal/handler/ops_error_logger.go`、`backend/internal/repository/ops_repo.go`、`frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`。
- 迁移：`backend/migrations/252_add_ops_routing_diagnostics.sql`。
- 测试入口：`backend/internal/service/routing_diagnostics_test.go`、`backend/internal/service/openai_routing_diagnostics_test.go`、`backend/internal/service/ops_routing_diagnostics_dto_test.go`、`backend/internal/handler/admin/ops_routing_diagnostics_handler_test.go`、`backend/internal/service/openai_ws_native_resume_test.go`、`frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts`。

### 服务状态

- Plus 参考：[CHANNEL_MONITOR.md](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/CHANNEL_MONITOR.md)。本地[独立合同](openspec/changes/adopt-plus-usage-and-diagnostics/service-status-contract.md)要求管理员可见、独立监控键、未知与样本不足，不替换原 V1/V2。
- 本地引入：`35962a802fdc499639c9c861072f25774ccba50b`；当前实现以 A 为准，包含后续完整分钟和源日志缺口修复。
- 重放文件：`backend/internal/service/service_status.go`、`backend/internal/service/service_status_aggregator.go`、`backend/internal/service/service_status_observation.go`、`backend/internal/service/service_status_source_reporter.go`、`backend/internal/repository/service_status_repo.go`、`backend/internal/repository/service_status_normalize.go`、`backend/internal/repository/service_status_incidents.go`、`backend/internal/handler/service_status_handler.go`、`backend/internal/server/routes/admin.go`、`frontend/src/api/admin/serviceStatus.ts`、`frontend/src/views/admin/ServiceStatusView.vue`、`frontend/src/router/index.ts`。
- 迁移：`backend/migrations/253_service_status_observation.sql`、`backend/migrations/254_service_status.sql`。
- 测试入口：`backend/internal/service/service_status_test.go`、`backend/internal/service/service_status_observation_test.go`、`backend/internal/repository/service_status_source_integration_test.go`、`backend/internal/repository/service_status_repo_integration_test.go`、`backend/internal/handler/service_status_handler_test.go`、`backend/internal/server/routes/service_status_routes_test.go`、`frontend/src/views/admin/__tests__/ServiceStatusView.spec.ts`。

### 个人版本、同步、发布与部署

- 应用适配：`backend/internal/service/update_service.go`、`frontend/src/components/common/VersionBadge.vue`；测试入口为 `backend/internal/service/update_service_test.go`、`frontend/src/components/common/__tests__/VersionBadge.spec.ts`。`backend/scripts/resolve-version.sh` 与 K 相同。
- F 同步工具：`.github/personal-sync/sync.py`、`.github/personal-sync/check_binding.py`、`.github/workflows/sync-upstream.yml`、`.github/workflows/personal-ci.yml`；测试入口 `.github/personal-sync/test_sync.py`。本地引入可追溯至 `624f3d6a2564cf250dd1cf5ce6733c45b2693952`。
- F 个人发布工具：`.github/release-tools/personal_release.py`、`.github/release-tools/release-images.sh`、`.github/workflows/personal-release.yml`、`.github/workflows/release.yml`；测试入口 `.github/release-tools/test_personal_release.py`、`.github/release-tools/test_release_matrix.py`。个人 helper 的本地引入为 `b7794d6f25f25e9d28f82b7cda1b82325ba5e96d`，后续门禁修正见 `9a358653a8bab6b0679184a8806f2c786d26aba2`。基础 `.github/release-tools/release_matrix.py` 在 K、A、F 字节相同，其共享历史引入提交为 `0892ef3a9a320dad79ff87930dcd5ae56a9a140a`，不因出现在上游就抹去已有贡献历史。
- F 部署工具：`deploy/personal/deploy-hostdzire.sh`、`deploy/personal/deploy_hostdzire.py`、`.github/workflows/personal-deploy-ci.yml`；测试入口 `deploy/personal/test_deploy_hostdzire.py`。本地引入 `2e5da44431efe152b4b6105a80b7a84f233436a6`，当前绑定修正截至 `e3b3e8e284e2d9b0ff8f972650807c4d40863c64`；详见[部署说明](deploy/personal/README.md)。本次不执行主机操作。

## 人工复核结论与边界

- 已核对 P 的四份功能文档、计时 helper/schema、本地合同与引入提交，以及 A 的实际文件/测试入口。计时 helper 不是原文件直接搬运：P 使用同名 `first_token_ms`，A 保留旧字段并读取 `strict_first_token_ms`，返回类型、适用范围、空值与导出接口均有本地适配；不能宣称 Plus 的全平台采集已在本 fork 实现。
- K 中已有的 `backend/internal/pkg/upstreamrecord/recorder.go`、`frontend/src/utils/codexCatalogConfig.ts` 和 `backend/scripts/resolve-version.sh` 与 A 字节相同；它们不属于个人新增核心模块。公开原始上游关系保留，逐文件的更早作者归属不作猜测。
- 本文没有发现足以把 S1–S4 整模块标为“直接复制”的记录；结构/概念参考明确保留 Plus 归属，不据文件不同或相同许可证宣称全部自主原创。局部代码版权头、许可证副本与发布产物材料留给 LEGAL-001/LEGAL-002 审核，不在此给出完成结论。
- PREP-001 的数据库迁移账本、schema 冻结和非生产恢复演练尚未完成。本文的 Git 来源核对不能替代恢复基线；PREP-003 任务状态保持未勾选。
- 重放到新 KIN 时逐模块核对上述 owner、接口、权限和迁移身份。应用测试需在实际新候选中按测试文件/名称定向执行；本文仅核对入口存在，不声称已重放、已编译或已通过这些测试。
