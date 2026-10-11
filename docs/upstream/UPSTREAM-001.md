# UPSTREAM-001：KIN 目标冻结与重放准备

核对日期：2026-10-11。本任务只冻结目标、创建干净集成分支和定义后续阶段的提交及验证规则。没有重放个人功能、执行数据库迁移、切换 `personal`、合并 PR 或发布。

## 任务边界与前置证据

- PREP-001 已随 [PR #12](https://github.com/ccisnoxx/sub2api/pull/12) 合并，合并提交为 `2cca80b396d7f2e21d2e3cd50c0b0e2ae1767738`；恢复材料及隔离演练见 [BASELINE.md](../../BASELINE.md)。
- PREP-003 已随 [PR #11](https://github.com/ccisnoxx/sub2api/pull/11) 合并，合并提交为 `24daf87641750fb78a3ad5818e0eb3eb392d638e`；四个模块的来源、合同和旧应用测试入口见 [UPSTREAM.md](../../UPSTREAM.md)。
- 执行前维护工作区分支为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，只有 `AM REMEDIATION_PLAN.md`；原暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。
- 文档分支 `codex/upstream-001-preparation-record` 从核实的 `origin/main` 创建，初始 HEAD 为 `c1dae6357b4de7cc11d81547013b87550c4ba586`，工作树为空；本任务文档 PR 以 `main` 为目标。
- 维护者已确认采用下表的官方 tag/SHA。原计划 `94db97ee634340ddc74cb548ec92f80c7d3edc65` 与当前 tag 解析结果不一致；这里只确认不一致，不推断标签曾被改写。
- 原任务要求 S1–S4 已重放，而 MIG-001 依赖 UPSTREAM-001、S1–S4 又依赖 MIG-001 或前序阶段，形成循环。维护者明确授权将 UPSTREAM-001 改为准备任务，实际重放提交、编译及测试验收保留在 UPSTREAM-002～005。后续任务的业务验收要求没有降低。

## 冻结目标与历史关系

| 项目 | 核实值 |
| --- | --- |
| 官方最新稳定 Release | [v0.2.15-klno.2](https://github.com/KlN-4096/sub2api/releases/tag/v0.2.15-klno.2)，非 draft、非 prerelease |
| 官方发布时间 | `2026-10-10T07:54:36Z` |
| annotated tag object | `0c1ede31ec63f9cfc2b72f07db66fb41f540208d` |
| tag 实际 commit | [ca686de5e5ac14834e12701c156831abd5ec6b71](https://github.com/KlN-4096/sub2api/commit/ca686de5e5ac14834e12701c156831abd5ec6b71) |
| 旧 KIN 基线 | `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| 当前 `origin/personal` | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| 新目标与旧 KIN 的 merge-base | `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d` |
| 新目标与当前 personal 的 merge-base | `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d` |
| 集成分支 | `codex/upstream-001-kin-v0.2.15-klno.2`，本轮仅本地创建并附着为独立工作树 |
| 集成分支初始及本轮最终 HEAD | `ca686de5e5ac14834e12701c156831abd5ec6b71` |

Git 远端 tag ref、peeled ref 与 GitHub tag object API 三者一致；`release.target_commitish=main` 没有被用作提交证明。冻结的 tag object 另保存在本地 `refs/kin-migration/upstream-001/tag`，不覆盖现有全局 tags。

`git merge-base --is-ancestor` 对旧 KIN、新目标以及当前 personal、新目标两组均返回 **1**，表示不满足祖先关系；这是已解释的分叉证据，不是待忽略的失败。集成分支从新目标创建，没有新增提交、tree diff 或 merge commit，因此没有混入旧 personal 的完整历史。保留上游自身已有的 merge 历史。

本阶段不新增或修改 `deploy/personal-source.json`。该文件在当前 personal 中仍固定旧 KIN tag/SHA，在新目标中不存在；不能在功能重放和切换门禁完成前把新目标写成当前应用来源。

## 四个独立重放阶段

每个阶段从已验收的直接前置候选创建任务分支，只用可审查的 cherry-pick、手工适配或小范围 patch 引入本阶段变更。禁止 merge 旧 personal 的整段历史，禁止空提交冒充功能重放，禁止强推 personal 或改同步脚本绕过分叉检查。

| 阶段 / 后续任务 | 直接前置 | 本阶段边界 | 独立提交及报告 | 必须覆盖的定向验证 |
| --- | --- | --- | --- | --- |
| S1 / UPSTREAM-002 | UPSTREAM-001、MIG-001 | 用量计时、终态与音频 Token；保留旧首字、计费及异步 owner | 提交标题包含 `UPSTREAM-002`；报告保存该阶段完整基准/候选 SHA、差异、测试与回滚 | JSON、SSE/透传、HTTP↔WS、多 turn/交错 response ID、取消/失败/缺终态；观察器及异步复制 `go test -race` |
| S2 / UPSTREAM-003 | UPSTREAM-001、UPSTREAM-002 | 授权模型及可解释价格目录；保持 opt-in 和旧接口行为 | 提交标题包含 `UPSTREAM-003`；报告单独绑定候选及前置 S1 SHA | 授权组/渠道、不可计算价格 unknown、无上游请求/结算副作用、前端目录及旧接口 |
| S3 / UPSTREAM-004 | UPSTREAM-001、MIG-001 | 真实选择的不可变诊断、存储及管理员展示；保持调度/重试/计费 owner | 提交标题包含 `UPSTREAM-004`；报告单独绑定候选，接入累计分支时复查与 S1/S2 的交集 | 候选池/过滤原因、异步深复制、管理员与普通用户边界、历史 NULL、native WS 重试/恢复 |
| S4 / UPSTREAM-005 | UPSTREAM-001、MIG-001、UPSTREAM-004 | 真实请求服务状态、聚合与事故；独立开关，保留 V1/V2 | 提交标题包含 `UPSTREAM-005`；报告单独绑定候选及前置诊断 SHA | 终态与未知、时间窗口/水位、幂等/缺口、管理员权限、数据库集成与状态展示 |

以上测试入口以 [来源清单](../../UPSTREAM.md#模块证据与重放入口)及各任务完整验收为起点。它们属于固定旧应用的入口；新 KIN 适配时必须重新定位实际 owner、测试路径和调用边界，不能将旧报告直接当作新候选通过。S3 可以在迁移审计后独立准备，但接入累计分支必须基于当时固定的已验收 HEAD，不能覆盖其他阶段。

每个阶段都必须独立满足以下规则，方可接入累计候选：

1. 固定源目标、前置候选、该阶段提交范围及最终完整 SHA；新增迁移须遵守 MIG-001 对已执行身份/checksum 的兼容结论。
2. 在该固定候选执行 `make -C backend build`；有前端变化时执行冻结依赖安装与 `pnpm --dir frontend run build`。按上表和对应任务运行实际定向单元、集成、race 或用户流程验证，并在报告中写明真实命令及未覆盖范围。
3. 实际应用候选执行适用的 Personal CI；报告绑定同一候选的成功门禁。每个阶段可以单独编译、测试和复核，不借用后续阶段结果补自己的验收。
4. 原有 LEGAL-001/002 材料、构建许可门禁与 Airwallex 禁用候选 [PR #17](https://github.com/ccisnoxx/sub2api/pull/17) / `0ea69e62ee0079ee2616273f627ed97cc3f57b5c` 保留。新目标仍含 Airwallex SDK；这份纯 KIN 分支只是迁移起点，不能当作已具备个人功能或已通过本 fork 许可验收的应用候选。后续整合必须保留禁用和必需许可材料后，才可通过切换、发布门禁。
5. 每次开始及交付前复核官方 tag object/commit。与本冻结目标不一致时停止并重新评估；不追随移动的分支、不自动覆盖冻结 ref、不放宽祖先检查。

本轮没有生成 S1–S4 功能提交或冒充其测试报告；这些后续任务均保持未完成。

## 本轮验证与证据边界

- 重新查询官方 latest stable Release、`git ls-remote` 和 tag API，核实完整 tag object、commit、发布时间及稳定属性；本地 namespaced ref 与官方结果一致。
- 检查集成分支 `rev-parse`、`rev-list TARGET..BRANCH`、`rev-list --min-parents=2 TARGET..BRANCH`、`diff TARGET BRANCH` 及工作树：精确等于新目标、增量为空、没有新增 merge、工作树为空。
- 核实两组 merge-base、预期非祖先退出码，以及 main/personal 精确 SHA、personal-source 原字节与原工作区暂存 blob 保留。
- 原样 KIN 树可复用其同 SHA 的[官方 CI 38034603661](https://github.com/KlN-4096/sub2api/actions/runs/38034603661)：shell、test、frontend、golangci-lint、release-helpers 五个 job 全部 success。这只是上游基线已有证据，本轮没有重跑其全量测试，不能证明尚未实现的个人功能、MIG-001 升级路径或本 fork 的发布验收。
- 文档字段、本地链接、任务范围和差异检查在本轮执行；独立只读复核、文档候选 SHA 及适用的 PR CI 记录于本任务 PR。
- 没有执行 Release dry-run、数据库恢复或生产读取：本任务没有改变应用、数据库或发布工具；这些检查分别由实际迁移、应用切换和发布任务完成。

## 风险、数据库影响与回滚

当前两条历史已经分叉。新 KIN 的迁移身份及生产兼容结论尚未审计；下一任务为 MIG-001。本轮没有查询新的生产账本，没有确认编号冲突，也没有运行 SQL。数据库影响为 **none**。

`main`、`personal`、个人来源文件及 PR #17 均未更新。准备完成不代表新应用可发布；S1–S4、后续前端整合、迁移兼容、许可与最终 Personal CI 门禁仍须各自验收。

回滚本任务文档使用其独立提交的 `git revert <commit>`，保留既有恢复点、个人候选和冻结来源。需要撤销本地集成准备时，先确认没有后续提交或未提交改动，再归档附着的集成工作树；不能使用强制删除或改写 main/personal。冻结 ref 与初始分支也可保留作审计证据，无需改动运行环境。

## 准备阶段验收

- [x] 集成分支初始提交精确等于经维护者重新确认的官方 KIN tag commit。
- [x] 相对该目标没有新增 merge commit 或旧 personal 整段历史。
- [x] S1–S4 的边界、依赖、可审查重放方式、独立提交和报告归属已建立，个人来源文件保持不变。
- [x] 每阶段独立编译和定向测试的入口及门禁规则已明确；实际重放提交和测试通过由 UPSTREAM-002～005 验收。
