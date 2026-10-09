# KIN 借鉴 Plus 功能执行证据

- 记录日期：2026-10-09（America/Los_Angeles）。
- 本次范围：仅 S0.1；[任务清单](tasks.md)和[计划](plan.md)。
- 维护登记位置：从远端默认 main 准备的 `codex/plus-s01-evidence`，本地文档候选尚未提交或推送；原计划位置同步相同记录。应用检查始终使用 personal 修复工作树。
- 合并、镜像发布、生产部署及 S0.2/S0.3/S1–S5 均未执行。

## S0.1 现有修复最终候选核对

### 基线、工作位置与候选身份

先读取用户指定的 plan.md、tasks.md、docs/dev-journal.md，再核对 Git 工作树、远端分支、来源记录和默认 main 的最新维护日志。远端通过 `git ls-remote origin refs/heads/personal refs/heads/main refs/heads/codex/fix-version-usage-help` 只读查询，未执行 fetch、push、PR 或 Actions dispatch。

| 对象 | 实际值与结论 |
|---|---|
| 最新远端 personal、本地 personal、修复 HEAD | 均为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，提交差异为 0；候选无需更新应用基线 |
| 应用来源 | `deploy/personal-source.json` 固定 `KlN-4096/sub2api`、`v0.2.14-klno.5`、`c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| 应用检查位置 | `/Users/sc/.codex/worktrees/fix-version-usage-help/sub2api-kin`，分支 `codex/fix-version-usage-help` |
| 初始未提交改动 | 17 个应用文件、docs/dev-journal.md，以及未跟踪的本 change 计划目录；原 index 没有 staged 改动 |
| 最新远端 main | `b43a472f4b1bde5983b3bdfa1447cd3622a77540`；维护日志已登记 `.5-tps.1` 的旧版本上线，不是本修复候选的发布证据 |
| 文档登记位置 | `/Users/sc/.codex/worktrees/plus-s01-evidence/sub2api-kin`，从 `origin/main` 上述 SHA 创建 `codex/plus-s01-evidence`；没有搬运 main 的应用源码到 personal |
| 原聊天 checkout | `/Users/sc/PycharmProjects/sub2api-kin`，`codex/ci-validation-scope` / `7949efe62e3854c83f39a21a0c36e03402c05688`，初始干净；本次未修改 |
| 远端修复分支 | 查询没有返回 `refs/heads/codex/fix-version-usage-help`，本修复尚未推送 |

应用候选仍未提交，**`9397eb8af` 是基线 SHA，不是包含修复的最终提交 SHA**。本次以以下身份固定实际检查的内容：

- 应用候选树：`27d96087e53a97386fd86935afa4ccc3dcd991e3`。从基线使用临时 Git index 加入 17 个应用文件生成；不包含本次文档，没有改变原 index、HEAD 或分支。
- 应用补丁 SHA-256：`dc20cf45f0f4c904c11a792dc2b77f3abdb3bd8ea98ba22b3a54d6fa8e1a4639`，对应 `git diff --binary HEAD -- <17 个应用路径>`。
- [候选校验清单](evidence/s0.1-candidate.json)登记完整基线、17 文件 SHA-256、构建/锁文件输入及逐文件 diff 统计；`candidate_commit=null` 明确表示未提交。
- 候选树只用于内容核对，不能作为 Personal CI、合并或发布所需的 commit SHA。S0.2 应先提交已审查内容，再绑定实际完整提交 SHA；本次不提前执行。

### 最终 diff 与行为确认

应用相对最新 personal 的净差异仅为 17 文件、656 行新增、125 行删除；没有新增用量字段、迁移、Ent 生成代码或调整转发、指纹、调度、gwpool、计费和导出。

| 范围 | 最终候选行为 |
|---|---|
| 个人更新通道 | `X.Y.Z-klno.N-tps.N` 使用 `ccisnoxx/sub2api` 的最近 100 条 release，过滤草稿和非法标签，保留 prerelease，按基础版本、KlN 序号、TPS 序号选择最新；普通 KlN release 沿用 KlN latest，Wei-Shaw 只读监测 |
| 缓存与构建能力 | 缓存绑定发布仓库并校验个人标签；当前版本重启后重新比较。API 的 build_type、update_mode、release_repository 分开报告；个人 release 是 container，个人 source 是 manual，普通 KlN release 保留 in_place |
| 二进制操作 | 个人镜像/源码构建在 service 的查询、下载或文件替换之前拒绝更新及回滚；真实 service 接入的个人 HTTP 测试确认四个入口为 `409 / IN_PLACE_UPDATE_NOT_SUPPORTED`；回滚列表 handler 保留结构化错误 |
| 版本提示 | 保留完整 `klno.N-tps.N` 后缀；尚未检查及检查失败不显示“已是最新”。个人镜像显示固定 digest 更新及数据兼容回滚说明；普通 KlN 的更新/重启入口保留 |
| TPS 点击说明 | TPS 标签和数值去掉原生 title；TPS 旁独立圆圈使用 HelpTooltip click 模式。计算仍为 `output_tokens × 1000 / duration_ms`，包含首字等待；颜色、数字格式和不可用范围不变 |
| 旧首字文案 | 首字旁使用自己的说明按钮；中英文明确这是转发开始到记录的首个响应事件或输出，可能包含元数据、推理或工具调用，未采集显示 `-`；没有把旧 first_token_ms 声称为严格首 Token |
| 使用入口 | 管理员和用户页面继续复用同一 UsageTable；两个说明互相关闭，点击/Enter/Space 打开，Escape/点击外部或关闭按钮关闭，窄屏定位沿用已验证的 HelpTooltip |

本轮没有修改上述应用文件。最终 diff 自查未确认需要修正的新增可操作问题；本结论是当前范围的自查，独立复核另见下表。

### 验证结果与证据复用

逐文件校验 `/tmp/sub2api-plus-adoption-preserved-files.json`，**17/17 与计划编制时的最终修复内容一致**。相关配置和锁文件相对 personal HEAD 无变更，已有 node_modules 仍标记 `pnpm@9.15.9`；源码修改时间早于各自成功日志和最终复核。历史日志实际读取，审计包按 manifest 逐个验证 SHA-256，而非只引用任务状态。

| 验证 | 本次处理 | 结果与边界 |
|---|---|---|
| UpdateService/SystemHandler 定向 Go | 复用最终原始日志 `/tmp/sub2api-version-final-go-tests.log` | service 和 handler/admin 两包通过；日志没有保留完整 shell 筛选命令，本次不补造，也没有重跑 Go |
| UsageTps、UsageTable、HelpTooltip | 复用 `/tmp/sub2api-usage-tooltip-tests.log` | 51 + 41 + 5 = 97 个用例通过；对应说明、适用范围、双入口切换和关闭合同 |
| 最终 VersionBadge | 复用 `/tmp/sub2api-version-final-ui-tests.log` | 10 个用例通过；涵盖个人后缀、失败/待检查、个人源码模式、普通 KlN 更新与重启 |
| 版本 store | 本次补充已有测试的原始执行证据 | `/tmp/sub2api-tps-tools/pnpm test:run src/stores/__tests__/app.spec.ts`，28 个用例通过；没有新增测试 |
| 改动前端文件 ESLint | 本次补充原始执行证据 | pnpm 9.15.9 `exec eslint` 检查 13 个改动前端文件，exit 0；未使用 --fix |
| i18n / Vue 类型 / Vite | 复用 `/tmp/sub2api-version-usage-build.log` | pnpm build 全部通过，locale completeness 3 个用例通过；Browserslist 数据过旧、Node shell 选项弃用及 chunk 体积提示是历史构建警告，没有宣称零 warning |
| 说明浏览器交互 | 复用 `usage-help-fix/results.json`、对应 qa.cjs 和截图 | 合成 API；管理员/用户、1440/390、中文/英文、明暗组合共四组通过；首轮两个管理员组合仍有尚未加入版本文案的 i18n warning，不能宣称该首轮零 warning |
| 最终版本浏览器状态 | 复用 `usage-help-fix/version-results.json` | update/current/error 三个状态均 passed，errors/warnings 为空；最终版本文案已补齐。本次未重启预览或重新进行浏览器矩阵 |
| 最终 service 真实网络闭环 | 复用 `/tmp/sub2api-version-live-check.log` | 当时匿名查询 GitHub，`.5-tps.1` 识别为当前最新，release=container；同通道缓存用于 source=manual。该证据只证明当时查询，不代表本次生产验收或未来发布状态 |
| 历史独立复核 | 复用 `20261009T061925Z-fix-version-tps-tooltips-06d9f2b5` | 历史两次 critical_reviewer 只读复核；初轮两项问题已修正，最终记录未确认新增可操作问题。20 个审计产物校验通过，bundle closed/verification passed；本次新增独立复核数为 0 |
| 当前 diff / 保留改动 | 本次检查 | git diff --check 通过，17 文件校验值一致；本次仅增改计划、任务、日志与证据 |

完整本次工具结果、历史日志摘要及 SHA-256、浏览器原始结果和审计身份见[验证登记](evidence/s0.1-validation.json)。浏览器截图和审计包仍在 Git 外原路径；记录中的历史复核次数不计为本次新委派。

补充检查首次使用当前默认 pnpm 11.25.0 时，触发与 pnpm 9 依赖布局不匹配的自动安装准备，并以 `ERR_PNPM_ABORTED_REMOVE_MODULES_DIR_NO_TTY` 退出；测试和 lint 当时尚未启动。这属于工具版本不匹配，不是应用失败。确认 `.modules.yaml` 与既有 pnpm 路径后，使用本机已安装的 9.15.9 重新执行受影响的两项检查并通过，没有重建依赖或改动锁文件。初次失败原始结果保留在验证登记。

### 完成状态、限制与下一项

- **S0.1 已完成**：最新 personal、实际候选、最终 diff、可复用证据与未提交状态均明确；原有 17 个应用文件保持原值。
- 应用候选与本次维护文档均未提交、未推送。没有给未提交应用修复补造 commit SHA，没有使用旧 `9397eb8af` CI 或 `.5-tps.1` 发布/生产结果证明本候选交付。
- 本次未执行完整回归、新的 Personal CI/Release gate、数据库检查或新的浏览器矩阵；本次没有新增对应业务、迁移或交互变化，已有直接证据覆盖 S0.1。真实权限隔离、生产页面及生产版本不属于本次本地核对证据。
- 现有部署工具按固定后端运行树约束兼容；本候选改动 update_service.go，不能沿用 `.5` 旧候选的固定运行树证明。针对真实提交审定部署兼容与回退边界，留待后续交付准备。
- **下一项是 S0.2**：先固定实际应用提交 SHA，按仓库必要门禁取得该候选的 CI 证据，准备合并、个人发布和部署兼容条件。实际合并与发布须在相应授权范围内执行；本会话止于 S0.1。S0.3 与 S1 仍未开始。
