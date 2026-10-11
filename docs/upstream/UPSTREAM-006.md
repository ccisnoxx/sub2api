# UPSTREAM-006 前端重放与正式整合记录

Task: UPSTREAM-006

## Summary

本报告保留前端准备的实现与验证证据，并记录正式累计后端上的整合。正式基底为 S4 `39753447cb4287c1a0aeee68b79b589ff345498a`；只 cherry-pick 006 交付 `a83ef2457dcccc898bde7f5bc3dc647be69606d3`，S2 目录已存在，无重复依赖提交。前置和同 SHA 阶段门禁全部满足后才在计划勾选任务。

- 新 KIN 冻结基线：`ca686de5e5ac14834e12701c156831abd5ec6b71`，版本 `v0.2.15-klno.2`；准备树起点为 MIG-001 提交 `c0e7ab68b8479fc9d6f3b888d548eca4f4192974`。
- 固定重放来源 A：`3d5e1fde82707900a21f2b5538b112c7bab3c04a`；旧 KIN 对照为 `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。
- 先单独准备模型目录前置提交 `ad080df10a789801f64f5710fb6b8ebbbaa4adc2`；以下文件清单及 006 交付差异均以该提交为起点。此提交仅提供目录 UI 前置，不表示正式 003 已完成。
- `UsageTable` 保留新 KIN 平台、路由和数值对齐行为，接入严格首 Token、首次输出、末次 Token、完成状态和来源；版本 0/缺失保留旧口径，版本 1 的严格字段缺失时显示未知，毫秒 0 与布尔 false 不被吞掉。
- 普通文本平均 TPS 统一采用 `(output_tokens - image_output_tokens - audio_output_tokens) * 1000 / duration_ms`，含首 Token 等待；未知媒体拆分不推定为 0，非适用记录不计算，可信部分用量明确标注部分响应。UI 格式与原始导出值分离。
- CSV/XLSX 在原列尾部按固定顺序追加 12 列：平均 TPS、说明、计时版本、严格首 Token、末次 Token、首次输出、输出类型、图片输出 Token、音频输出 Token、完成状态、完整性、来源。NULL 导出为空，0/false 和未经舍入的速率保持。未新增上传 XLSX 读取入口。
- 路由诊断仅接入管理员单记录详情；保留分类和原有详情行为，以冻结标签白名单显示选择层、原因、覆盖与过滤计数，未知机器码不回显。partial 明确表示已知下界，missing/unobserved 与观察到的零值分别解释；晚到详情不覆盖当前记录。
- 独立服务状态页面、导航和 API 接线保留管理员认证与路由约束。停用仍可编辑配置，清除快照；配置版本冲突保留草稿；身份、路由、筛选变化及 401/403/停用响应清理上下文，旧成功响应不能恢复数据。页面显示历史、覆盖缺口、故障与恢复观察、未知与低样本，不将未知比例描绘为健康结论。
- 平台选项复用新 KIN 权威平台目录，包含 `command_code`、`cline`。真实 Axios 请求验证后，仅两个服务状态 GET 端点豁免自动 timezone，其余 GET 继续沿用现有行为。
- 中英文文案完整；HelpTooltip 使用视口坐标、窄屏边距、点击提示滚动与 Esc/关闭后焦点恢复。真实浏览器发现 390px 的部分响应标记挤压 TPS，已限定为窄屏换行，桌面保留同行显示。

## 正式整合与独立复核

- 正式运行增量 `3de27c8095a9f116e5735b14d11ef97fd95b4cf6`。正式 frontend 全树与已测试、已独立复核的 `a83ef2457dcccc898bde7f5bc3dc647be69606d3` 逐字节相同；package、lock、配置和依赖输入没有变化，复用下述成功 Vitest/typecheck/lint/build/Chromium 及实际下载证据。
- 正式 backend 全树与 S4 `39753447cb4287c1a0aeee68b79b589ff345498a` 相同。共同 owner race121顶层PASS、真实 PostgreSQL 23顶层PASS（14状态数据库合同、7纯函数、2目录HTTP JWT/数据库合同）、项目build通过；S4真实 JWT/routes race15已通过。后端严格DTO、13平台清单、nullable比例/数组、range、version409和disabled503与实际前端逐字段核对，前端替身不承担后端权限证明。
- fresh只读 `critical_reviewer` 对固定准备增量独立复核，无确认问题；检查实际CSV/XLSX下载的29/47列顺序、NULL/false/0和未舍入数值，并核对权限、晚响应、未知机器码与页面/提示。正式增量运行源码一致，文档/journal差异不改变行为；独立复核未重新运行测试或生产业务，覆盖限制保留。
- 本阶段以 S4 分支为PR base，只包括006前端与阶段文档。维护者明确授权固定候选标准PR CI + Security Scan，Personal CI及来源切换留UPSTREAM-007；最终门禁及直接前置状态以本任务PR的固定SHA记录为准。

## Files changed

006 应用范围共 41 个前端源码/测试文件，另有本报告；模型目录前置文件不计入本节。

- `frontend/src/api/__tests__/admin.serviceStatus.spec.ts`
- `frontend/src/api/admin/index.ts`
- `frontend/src/api/admin/ops.ts`
- `frontend/src/api/admin/serviceStatus.ts`
- `frontend/src/api/client.ts`
- `frontend/src/components/admin/usage/UsageFirstToken.vue`
- `frontend/src/components/admin/usage/UsageTable.vue`
- `frontend/src/components/admin/usage/UsageTps.vue`
- `frontend/src/components/admin/usage/__tests__/UsageFirstToken.spec.ts`
- `frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts`
- `frontend/src/components/admin/usage/__tests__/UsageTps.spec.ts`
- `frontend/src/components/common/HelpTooltip.vue`
- `frontend/src/components/common/__tests__/HelpTooltip.spec.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/service-status/ServiceStatusMetrics.vue`
- `frontend/src/i18n/__tests__/opsLocaleKeys.spec.ts`
- `frontend/src/i18n/locales/en/admin/index.ts`
- `frontend/src/i18n/locales/en/admin/ops.ts`
- `frontend/src/i18n/locales/en/admin/serviceStatus.ts`
- `frontend/src/i18n/locales/en/common.ts`
- `frontend/src/i18n/locales/en/dashboard.ts`
- `frontend/src/i18n/locales/zh/admin/index.ts`
- `frontend/src/i18n/locales/zh/admin/ops.ts`
- `frontend/src/i18n/locales/zh/admin/serviceStatus.ts`
- `frontend/src/i18n/locales/zh/common.ts`
- `frontend/src/i18n/locales/zh/dashboard.ts`
- `frontend/src/router/__tests__/feature-access.spec.ts`
- `frontend/src/router/index.ts`
- `frontend/src/types/index.ts`
- `frontend/src/utils/__tests__/usageTiming.spec.ts`
- `frontend/src/utils/usageTiming.ts`
- `frontend/src/views/admin/ServiceStatusView.vue`
- `frontend/src/views/admin/UsageView.vue`
- `frontend/src/views/admin/__tests__/ServiceStatusView.spec.ts`
- `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- `frontend/src/views/admin/ops/components/OpsErrorDetailModal.vue`
- `frontend/src/views/admin/ops/components/OpsRoutingDiagnosticsPanel.vue`
- `frontend/src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts`
- `frontend/src/views/admin/ops/components/__tests__/OpsRoutingDiagnosticsPanel.spec.ts`
- `frontend/src/views/user/UsageView.vue`
- `frontend/src/views/user/__tests__/UsageView.spec.ts`
- `docs/upstream/UPSTREAM-006.md`

## Tests run

依赖使用 pnpm 9.15.9；执行 `corepack pnpm@9.15.9 install --frozen-lockfile` 成功，未修改依赖或锁文件。下列命令从 `frontend/` 执行。

```bash
corepack pnpm@9.15.9 exec vitest run \
  src/api/__tests__/channels.catalog.spec.ts \
  src/utils/__tests__/modelCatalog.spec.ts \
  src/components/channels/__tests__/ModelCatalogCard.spec.ts \
  src/views/user/__tests__/AvailableChannelsView.spec.ts \
  src/i18n/__tests__/localeKeyCompleteness.spec.ts \
  src/utils/__tests__/usageTiming.spec.ts \
  src/api/__tests__/admin.serviceStatus.spec.ts \
  src/router/__tests__/feature-access.spec.ts \
  src/components/admin/usage/__tests__/UsageFirstToken.spec.ts \
  src/components/admin/usage/__tests__/UsageTps.spec.ts \
  src/components/admin/usage/__tests__/UsageTable.spec.ts \
  src/components/common/__tests__/HelpTooltip.spec.ts \
  src/views/user/__tests__/UsageView.spec.ts \
  src/views/admin/__tests__/UsageView.spec.ts \
  src/views/admin/__tests__/ServiceStatusView.spec.ts \
  src/views/admin/ops/components/__tests__/OpsErrorDetailModal.spec.ts \
  src/views/admin/ops/components/__tests__/OpsRoutingDiagnosticsPanel.spec.ts \
  src/i18n/__tests__/opsLocaleKeys.spec.ts
```

结果：18 个测试文件、263 个测试全部通过。覆盖目录前置回归、严格/旧计时和媒体扣除、导出列/值、真实客户端参数、管理员路由、晚响应、未知码、双语及键盘提示。最后的窄屏样式调整后，复用未受影响的成功证据，并重跑以下直接影响范围：

```bash
corepack pnpm@9.15.9 exec vitest run \
  src/components/admin/usage/__tests__/UsageTps.spec.ts \
  src/components/admin/usage/__tests__/UsageTable.spec.ts \
  src/components/common/__tests__/HelpTooltip.spec.ts
corepack pnpm@9.15.9 exec vue-tsc -b
```

结果：3 个文件、107 个测试通过；最终 `vue-tsc -b` 退出 0。单独的最终 HelpTooltip 检查也为 6/6 通过。

- `corepack pnpm@9.15.9 exec eslint`：参数为本节列出的 41 个变更 `.ts`/`.vue` 文件（去掉 `frontend/` 前缀），最终退出 0，无 lint 警告。
- 构建按 package build 的三个阶段执行：上述 i18n 测试、`vue-tsc -b`、`corepack pnpm@9.15.9 exec vite build`。Vite 显式使用系统临时输出目录，避免写入默认后端静态产物；最终 1099 个模块构建成功，退出 0。未使用会调用全局 pnpm 11 的嵌套裸 pnpm 脚本。
- `git diff --check`：通过；实际差异检查确认排除更新器、CSP、其他系统设置、backend、CI、SQL、平台源 JSON、依赖/锁及计划改动。

真实浏览器验证使用本地 Vite、Playwright 1.62.1 与已安装的 Chromium；当前会话没有 Browser 插件接口，未宣称通过该插件执行。API 与登录状态均为合成 fixtures，边界限定为前端运行、实际客户端请求、DOM、下载文件及交互，不能代表真实后端 JWT 或持久化权限已经验收。

- 主流程：中英文 × 亮/暗主题 × 1440/390px，共 8 组环境、48 条验收记录通过。每组检查 URL/标题/内容、无 Vite 错误遮罩、页面横向溢出、目录折叠 summary 键盘、价格档位/滚动/过滤、服务状态历史/恢复/低样本/配置开关/筛选、严格时点 0/未知/部分、提示边界、Esc/焦点、管理员诊断未知码与滚动。窄屏另检查 TPS 值不越过行容器。
- 扩展流程：11 条验收记录通过。普通用户用量及错误列表/详情未渲染注入的内部渠道、账号和过滤原因；管理员诊断不存在于普通用户详情；访客和普通用户访问管理员状态页均重定向且不请求状态 API；关闭配置不请求快照；筛选/离开路由后的晚响应无法恢复旧数据；503 清快照保留配置，403 清快照和配置。
- 用户真实 CSV 下载为 29 列，管理员真实 XLSX 下载为 47 列；两者末尾 12 列顺序相同。验证实际文件中平均值仍为 `292.7536231884058`/`147.82608695652175`，0、false、空字段保持，而 UI 显示 `293 tok/s`/`148 tok/s`。
- 已保存并人工查看桌面/窄屏截图，包含目录、服务状态、时点提示、管理员路由详情与普通用户详情。截图在动画完成后采集；本机脚本、JSON、日志、截图和导出样本随父任务证据交付，不写入仓库或正式报告中的开发机路径。
- 主流程控制台零 error/pageerror、零未匹配 API；扩展流程仅出现故意合成 403/503 的资源错误，无其他运行时错误、pageerror 或未匹配 API。

## Risks

- 正式后端DTO/JWT/持久化由实际后端合同和真实数据库测试验收；Chromium使用合成登录/API，验证范围是页面、请求、下载与交互，不代表生产完整收费/状态开启旅程。
- 权限与晚响应边界由组件/浏览器和独立只读审查覆盖；后端source/配置/事务等高风险所有权另由S4独立只读复核承担。
- 固定候选标准PR CI与Security Scan由阶段PR执行；未完成的结果不列为通过。Personal CI和来源切换属于007。
- 构建存在既有 stale Browserslist、动态/静态导入分包、超过 500kB 的 chunk 提示，以及 Node 子进程 shell 选项弃用提示。构建和测试均通过，未扩展为依赖更新或分包重构。
- 冻结 KIN AGENTS 引用的 conventions 在树与本地完整历史中缺失，父任务已记录；未发明身份规则或修改相关文件。

## Database impact

none。本交付仅修改前端与任务报告；不含数据库、SQL 或迁移，未运行数据库升级/回退检查。

## Rollback

撤销本006应用与文档独立提交即可恢复到S4正式基底 `39753447cb4287c1a0aeee68b79b589ff345498a` 的前端，保留S2目录及S1/S3/S4后端。无数据库回退动作。

## Acceptance checklist

- [x] 准备树行为与 diff 局限于明确授权的 006 前端范围；目录依赖单独提交。
- [x] 严格/旧计时区分，0/NULL/false、完成/部分/未知及来源解释有实际测试。
- [x] 文本平均 TPS 使用全时长并扣除可信媒体用量，未知拆分不猜值。
- [x] CSV/XLSX 固定追加列，UI 舍入不影响实际导出原始数值，NULL 与 false 保持。
- [x] 未新增读取用户上传 XLSX 路径。
- [x] 管理员单详情显示白名单诊断，未知码不回显；普通用户浏览器流程未泄漏内部信息。
- [x] 服务状态独立配置开关、管理员前端门禁、筛选/身份/路由晚响应、拒绝/停用清理通过定向验证。
- [x] 历史、故障、恢复观察、覆盖缺口、unknown 与低样本展示有组件及浏览器证据。
- [x] TypeScript、关键 Vitest、变更范围 lint、等价生产 build 通过。
- [x] 中英文、亮暗、1440/390px 和 summary/Esc/焦点/滚动通过实际 Chromium 验证。
- [x] 正式累计依赖树整合完成；真实API/JWT/持久化合同核对，共同owner检查通过。
- [x] 最终运行增量与独立只读复核/前端验证候选逐字节一致。
- [ ] 固定候选标准PR CI与Security Scan阶段验收，以阶段PR最终记录为准。
- [ ] 正式 UPSTREAM-006 任务勾选由父任务统一处理。
