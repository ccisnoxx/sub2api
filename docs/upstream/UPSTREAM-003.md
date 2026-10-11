# UPSTREAM-003：授权模型与价格目录迁移记录

Task: UPSTREAM-003

Summary:
- 从固定旧 personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 中抽取 S2 范围，适配新 KIN `ca686de5e5ac14834e12701c156831abd5ec6b71`。准备树基线为 MIG-001 `c0e7ab68b8479fc9d6f3b888d548eca4f4192974`；正式分支 `codex/upstream-003-s2-catalog` 以 UPSTREAM-002 固定候选 `ee415b62ea9076b7f682ab73fd94203478ca363a` 为直接父基准，小范围重放 S2，没有合并旧历史。当前已完成整合定向检查；前置与阶段门禁最终结果以对应 PR 固定记录为准。
- 保留精确单个 `view=catalog` 的显式选择；无参数、重复或未知 view 继续返回旧数组。目录使用真实当前用户、活跃组和有效订阅授权 owner，不根据展示可见性授予访问权。
- 授权后只读一次活跃渠道快照，价格解析复用现有计费 owner，基础价格与用户/时段倍率分开。请求或响应依赖、未明确的媒体单位、缺失价格返回带原因的 unknown；明确零价及个人零倍率保留。
- DTO 使用显式白名单，不序列化 Group/Channel、内部模型映射、渠道 ID 或账号信息。报价标识由配置身份生成，不携带凭据。
- 对齐现有 `channel_groups.group_id` 唯一约束：授权活跃分组若绑定不同活跃渠道，目录明确返回 `CATALOG_CHANNEL_BINDING_CONFLICT`，不再按读取顺序覆盖。用户错误不包含内部 ID；原因保留在服务器错误链。未授权或停用配置不干扰当前用户。
- 中英文目录页面支持本地搜索、分组报价、未知原因、倍率、阶梯价格及键盘详情；取消或用户切换后旧响应不能覆盖当前页面。

Files changed:
- `backend/internal/service/{channel_catalog,catalog_pricing}.go` 与对应价格测试：目录快照和真实计费价格解析。
- `backend/internal/service/{billing_context_schedule,billing_service,model_pricing_resolver,pricing_service}.go`：复用现有计费解析结果，保存来源明确零价信息；原计费数字和旧广场参数保持。
- `backend/internal/handler/available_channel_handler.go`、`available_model_catalog.go` 及测试：opt-in 路由、授权、白名单 DTO 和冲突错误；`backend/cmd/server/wire_gen.go` 由当前 Wire 重新生成。
- `backend/internal/repository/available_catalog_contract_integration_test.go`：真实 PostgreSQL/Redis、JWT、订阅、角色、RPM 和 HTTP 目录与账单价格对照。
- `frontend/src/api/channels.ts`、目录工具/组件/页面、双语 dashboard 文案及定向测试。

Tests run:
- `go test -tags=unit ./internal/handler -run '^TestAvailableModelCatalog_DuplicateAuthorizedBindingFailsRegardlessOfOrder$' -count=1`：修复前正序/逆序均因预期500、实际200失败，确认覆盖原缺陷；修复后冲突和无关冲突定向4项通过。
- `go test -tags=unit ./internal/service ./internal/handler -run '^(TestAvailableModelCatalog_|TestCatalogPricing_|TestCatalogRateMultipliers_|TestResolveContextPricingSchedule_|TestPlazaSchedulePreservesCacheWrite1h|TestResolve_|TestConfiguredServiceTierMultiplier|TestChannelOverridePreservesCatalogFastRatioByDefault|TestAnthropicFastUsesDefaultMultiplierWithoutCatalogTier|TestCalculateTokenCostContextTierEnablement|TestCalculateTokenCostCombinesIntervalAndFastMultiplier)' -count=1`：143项通过。
- Colima 环境下设置容器内 `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock` 和 `CI=true`，执行 `go test -tags=integration ./internal/repository -run '^TestAvailableCatalog' -count=1 -timeout=10m`（宿主 socket 由 `docker context inspect colima` 确認）：真实集成11项通过。首次运行仅设置宿主socket导致Ryuk无法挂载；按安装版Testcontainers的Colima文档修正环境后执行，未禁用清理器或修改harness。
- `corepack pnpm@9.15.9 --dir frontend install --frozen-lockfile`：通过，未改依赖或锁文件。
- `corepack pnpm@9.15.9 --dir frontend exec vitest run src/api/__tests__/channels.catalog.spec.ts src/components/channels/__tests__/ModelCatalogCard.spec.ts src/utils/__tests__/modelCatalog.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts`：10项通过。
- `corepack pnpm@9.15.9 --dir frontend typecheck`：通过。
- 等价构建步骤 `corepack pnpm@9.15.9 --dir frontend exec vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts`、`corepack pnpm@9.15.9 --dir frontend exec vue-tsc -b`、`corepack pnpm@9.15.9 --dir frontend exec vite build`：3项文案检查、编译与生产构建通过。原build内裸pnpm误走主机pnpm11并中止，分步执行完全相同脚本解决工具版本问题，未修改项目配置。
- `make -C backend build`：通过。Wire工具依赖在临时modfile解析，生成服务DI成功；仓库go.mod/go.sum保持原样。
- 变更范围 `corepack pnpm@9.15.9 --dir frontend exec eslint`：通过。
- `git diff HEAD --check`：通过。

Risks:
- fresh `critical_reviewer` 对准备候选 `898afb11105cf84246b6b8bac36602c188db9ff0` 完成独立只读权限、价格、DTO 和冲突复核，无确认阻断；未重跑测试。S1/S2 运行代码没有同文件交叉修改，正式整合后补核共享 billing/resolver → RecordUsage、DI、API、实际目录报价对照；独立结论可复用相同运行代码。前置 S1 和本阶段同 SHA 远端门禁仍须由 PR 固定记录。
- 实际付费上游与线上结算未执行；目录契约及账单对照用真实服务/解析owner和隔离数据库验证，不发起上游请求。
- 冻结KIN的AGENTS引用的出站身份conventions缺失；本任务没有改出站身份。
- 维护者在2026-10-11明确授权UPSTREAM-002—006使用固定候选标准PR CI及Security Scan；现有Personal CI的旧personal祖先/PR base/source绑定保持，来源切换及最终Personal CI留UPSTREAM-007。

正式整合后附加检查（代码配置环境固定）：
- 首轮 PR CI `38129024988` 的 golangci-lint v2.14.0 在新增报价分支触发 `QF1003`（按同一平台的 if/else 应使用 tagged switch）；原 CI 不计通过。仅将该二分支改为等价 switch，保留档位和回退语义，不改 lint 配置。修正后 `go test -tags=unit ./internal/service -run '^(TestCatalogPricing_|TestCatalogRateMultipliers_)' -count=1` 通过；最终固定门禁以 PR #22 为准。
- 上述 S2 service/handler 定向命令同时加入 `./internal/server`、`TestOpenAIUsageTiming_` 与 `TestAPIContracts$`：161 项通过，覆盖新 KIN 计费 owner、S1 观测隔离与旧接口 fixture。
- 同一真实 PostgreSQL/Redis 集成命令 `CI=true go test -tags=integration ./internal/repository -run '^TestAvailableCatalog' -count=1 -timeout=10m`：退出 0；不存在 Docker 跳过，原 11 个目录合同用例执行。
- 正式整合树 `make -C backend build`：退出 0，验证当前 Wire 注入和 S1/S2 共享依赖。
- 目录前端、依赖/锁与准备候选逐字节一致，复用原 Vitest/typecheck/build 证据；SQL、runner、CI 与来源控制面无修改。

Database impact:
- none。未改SQL、迁移runner或251–254身份；只读目录不写用量、余额或上游数据。多渠道冲突检测遵守已有唯一绑定约束。

Rollback:
- 应用回退本任务独立提交，保留迁移基线和已执行账本；不执行DROP或删除251–254，不重写personal/main。目录前端和后端作为同一任务提交回退。

Acceptance checklist:
- [x] 本地目录授权隔离、授权变化和隐藏模型检查通过。
- [x] 目录调用无真实上游请求、结算或用量写入。
- [x] 不可确定价格明确返回原因，零价与缺失分别保留。
- [x] 无view=catalog的旧接口行为保持。
- [x] 多渠道绑定冲突不依赖顺序，未授权/停用配置不泄漏。
- [x] 正式候选接入固定 S1，共享 owner、Wire、目录 HTTP/账单价格对照补核通过；相同运行代码独立复核无确认阻断。
- [ ] 前置 S1 及本阶段同 SHA 的标准 PR CI + Security Scan 全部成功，结果绑定阶段 PR 后才勾选任务。
