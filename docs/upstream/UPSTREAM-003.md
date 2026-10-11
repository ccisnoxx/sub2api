# UPSTREAM-003：授权模型与价格目录迁移记录

Task: UPSTREAM-003

Summary:
- 从固定旧 personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 中抽取 S2 范围，适配新 KIN `ca686de5e5ac14834e12701c156831abd5ec6b71`。准备树基线为 MIG-001 `c0e7ab68b8479fc9d6f3b888d548eca4f4192974`；正式候选须从验收后的 UPSTREAM-002 候选整合。本节记录准备树的本地验证，不冒充后续正式候选或远端门禁。
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
- `DOCKER_HOST=unix:///Users/sc/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock CI=true go test -tags=integration ./internal/repository -run '^TestAvailableCatalog' -count=1 -timeout=10m`：真实集成11项通过。首次运行仅设置宿主socket导致Ryuk无法挂载；按安装版Testcontainers的Colima文档修正环境后执行，未禁用清理器或修改harness。
- `corepack pnpm@9.15.9 --dir frontend install --frozen-lockfile`：通过，未改依赖或锁文件。
- `corepack pnpm@9.15.9 --dir frontend exec vitest run src/api/__tests__/channels.catalog.spec.ts src/components/channels/__tests__/ModelCatalogCard.spec.ts src/utils/__tests__/modelCatalog.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts`：10项通过。
- `corepack pnpm@9.15.9 --dir frontend typecheck`：通过。
- 等价构建步骤 `corepack pnpm@9.15.9 --dir frontend exec vitest run src/i18n/__tests__/localeKeyCompleteness.spec.ts`、`corepack pnpm@9.15.9 --dir frontend exec vue-tsc -b`、`corepack pnpm@9.15.9 --dir frontend exec vite build`：3项文案检查、编译与生产构建通过。原build内裸pnpm误走主机pnpm11并中止，分步执行完全相同脚本解决工具版本问题，未修改项目配置。
- `make -C backend build`：通过。Wire工具依赖在临时modfile解析，生成服务DI成功；仓库go.mod/go.sum保持原样。
- `git diff HEAD --check`：通过。

Risks:
- 本地准备树尚不构成正式任务验收；前置S1、正式候选复核和同SHA远端门禁分别记录于阶段交付。
- 实际付费上游与线上结算未执行；目录契约及账单对照用真实服务/解析owner和隔离数据库验证，不发起上游请求。
- 冻结KIN的AGENTS引用的出站身份conventions缺失；本任务没有改出站身份。
- 维护者在2026-10-11明确授权UPSTREAM-002—006使用固定候选标准PR CI及Security Scan；现有Personal CI的旧personal祖先/PR base/source绑定保持，来源切换及最终Personal CI留UPSTREAM-007。

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
- [ ] 正式候选已绑定验收的S1并完成独立复核及同SHA阶段远端门禁。
