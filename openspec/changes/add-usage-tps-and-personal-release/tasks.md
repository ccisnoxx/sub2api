# 实施任务清单：平均输出 TPS 与个人分支维护

编制日期：2026-10-08（America/Los_Angeles）。依据：[整体方案 plan.md](plan.md)。当前状态：**P0/P1 已完成；P2.1–P2.8、P2.10 已完成，P2.9 的真实升级因历史改写/冲突停止；P3.1–P3.9 已完成并首次发布；P2.11/P4 待实施**。勾选只代表已登记的实际实现与验证，见 [实施证据](implementation-evidence.md)。

本文将方案转换为可以按编号执行的任务。需要查看指标口径、分支职责或风险依据时，回查 plan.md；实现改变这些约定时先更新两份文档。下文标注“新增”表示原计划需要创建的文件；实际存在及验收状态以复选框和实施证据为准。相关命令在文件和依赖就绪后执行。

## 执行顺序与完成规则

| 阶段 | 前置条件 | 交付物 | 当前状态 |
|---|---|---|---|
| P0：保护并固定基线 | 已保存工作区中的已有工作 | 安全的工作流状态、personal、来源记录 | 远端 main/personal 安全定义已生效 |
| P1：实现 TPS | P0 完成 | 组件、双语文案、接入测试和前端验证结果 | 已完成 |
| P2：建立同步 PR | P0/P1 完成 | 同步工作流、定向 CI、候选 PR 和触发验证 | 流程及定时入口已验收；真实升级因历史改写/冲突停止 |
| P3：发布个人镜像 | P1 完成，发布所需检查已建立 | 个人版本标签、amd64 镜像、可追溯发布结果 | 已发布 v0.2.14-klno.3-tps.1；公开匿名完整拉取通过 |
| P4：部署与回滚 | P3 镜像发布成功，部署脚本已验证 | hostdzire 指定版本、部署记录与回滚证据 | 待实施 |

推荐按 P0 → P1 → P2.1–P2.10 → P3 → P4 → P2.11 执行。P2 的流程准备先通过临时仓库和候选 PR 验证，不合入新的生产上游版本；首次 P3/P4 仍发布“原上游基线 + TPS”。P2.11 是首次上线后的一次实际上游升级验收。

只在任务的行为已经实现、相应验证取得证据后勾选。测试失败、外部权限不足或检查尚未运行时保持未勾选，并记录原因与下一步。实际推送、修改仓库设置、发布镜像和生产部署按执行时已经获得的授权范围进行；当前交付是实施清单。

## P0. 保护并固定基线

**主要位置：**Git 分支、`.github/workflows/sync-upstream.yml`、`deploy/personal-source.json`（新增）、本 change 的文档。

- [x] **P0.1 保存已有工作。**检查当前分支、远程与工作树；保留本计划、任务清单和所有无关改动。切换基线前先使这些工作可恢复，不使用 reset/clean 丢弃内容。
- [x] **P0.2 停止旧同步逻辑。**检查继承的同步工作流是否已经启用；若已启用，先暂停。默认分支上的定义必须停止原来的 rebase、强推、自动打 KlN 标签及重建 main 行为；完整同步逻辑在 P2 验证后启用。**远端 main 已通过普通推送替换为暂停定义，再以手动安全入口验收 P2；旧破坏性逻辑已删除。**
- [x] **P0.3 核对实际部署基线。**执行 hostdzire 只读检查，记录运行镜像 digest/revision、架构、Compose 目录/文件/项目名、应用服务名、数据库/Redis 服务与挂载。只保存必要的非敏感信息，更新已发生变化的历史记录。
- [x] **P0.4 固定源码来源。**核对 `v0.2.14-klno.3` 解引用为 `de08df02ae1d81668a22f798b398aa0438ac1276`，并与运行 revision 对照；不一致时先解释差异并确定实际起点。
- [x] **P0.5 建立 personal。**不存在该分支时从已核实的发布标签创建；已存在时检查并复用，不覆盖。main 继续承载默认分支工作流，个人应用从 personal 开发。
- [x] **P0.6 保持两条分支的流程安全。**把暂停旧同步行为的维护改动和本 change 文档带入 personal；在 main/personal 都确认旧逻辑不会被误触发，不用合并整个 main 来搬运少量文件。**远端 main/personal 均已生效；维护定义内容一致，未合并整个 main 的应用源码。**
- [x] **P0.7 写入来源记录。**新增 `deploy/personal-source.json`，记录上游仓库、基础发布标签、完整源码 SHA；定义字段含义，并使后续同步/发布读取同一来源。手动未发布 SHA 需要记录真实来源及版本归属，不能冒充发布标签的提交。
- [x] **P0.8 检查所选基线的实际合同。**编码前读取 UsageTable、UsageLog 类型、媒体/请求类型工具、两个使用记录页面和相关测试；使用 personal 中的维护源码，保留现有 KlN 额外显示字段。
- [x] **P0.9 登记阶段证据。**实施时新增本目录的 `implementation-evidence.md`，记录源码 SHA、工作树状态、基线事实和检查结论。后续阶段追加本次候选对应的结果，不粘贴完整含敏感信息的命令输出。

**完成条件：**运行基线与 personal 起点可对应；main/personal 的旧破坏性同步动作不再运行；文档和无关工作均保留。

只读核对命令，从仓库根目录执行：

```bash
git status --short --untracked-files=all
git remote -v
git branch -a
git rev-parse HEAD 'v0.2.14-klno.3^{commit}'
git diff --shortstat main..v0.2.14-klno.3
```

## P1. 实现平均输出 TPS

**主要位置：**`frontend/src/components/admin/usage/`、其中的 `__tests__/`、`frontend/src/i18n/locales/{zh,en}/dashboard.ts`。

- [x] **P1.1 建立功能分支。**从 personal 创建或复用 `codex/usage-tps`，将 TPS 的组件、接入、测试和文案作为同一功能提交。
- [x] **P1.2 实现 UsageTps.vue。**新增组件，接收现有记录字段，统一按 `output_tokens × 1000 ÷ duration_ms` 计算；不扣首字耗时，不增加后端字段或迁移。
- [x] **P1.3 实现可用性判断。**文本、有效输出和有效总耗时才显示数字；无效值、媒体、compaction/live/probe/gwpool_degraded 显示 `—`。检查 token 计费下的图片输出，图片输入且文本输出的记录仍可计算，未知计费模式不猜测。
- [x] **P1.4 实现格式与说明。**保留两位小数及 `tok/s`，小于 0.01 的正值显示 `<0.01 tok/s`；显示平均速率口径及不可用原因，提示可通过键盘访问。不引入新组件库或依赖。
- [x] **P1.5 接入耗时栏。**在 UsageTable 的 `cell-latency` 添加 TPS 第三行，保留首字、总耗时与健康度色条；不添加服务器排序字段或改动 CSV。
- [x] **P1.6 补齐中文和英文文案。**在两份 dashboard.ts 中增加对应键，避免把 TPS 展示与 Fast 是否实际加速作等同表述。
- [x] **P1.7 增加组件行为测试。**新增 `UsageTps.spec.ts`，覆盖下面的展示合同；若采用 TDD，先确认最小测试因缺失行为失败，再实现。
- [x] **P1.8 保护表格接入。**修改 UsageTable.spec.ts 的 DataTableStub，真实渲染 `cell-latency` 和 UsageTps，验证 TPS 与原有耗时共存；不重复整组计算边界测试。
- [x] **P1.9 完成定向验证。**运行组件/表格及 HelpTooltip 定向测试、改动文件 lint 和 frontend build，将结果绑定本次代码 SHA。构建失败时先定位原因，再按受影响范围重跑。
- [x] **P1.10 完成界面验收。**通过本地构建预览与合成 API 记录检查管理员与用户使用记录中的历史文本、图片输入、不可用记录；验证明暗主题、窄屏布局和提示交互，保留能证明结果的截图。
- [x] **P1.11 检查差异并合入 personal。**确认没有后端、计费、服务档位或无关格式变更；按仓库规则审查和合并，记录最终提交及其实际检查结果。

组件测试需要保护的结果：

| 输入或情形 | 期望 |
|---|---|
| 输出 1040、总耗时 24650ms | `42.19 tok/s` |
| 同一记录首字为空，或首字改变 | TPS 保持相同 |
| 输出或耗时缺失、非有限、非正数 | `—` 与对应说明 |
| 图片/视频/其他明确不适用的请求类型 | `—` |
| 图片输入、有效文本输出 | 正常计算 |
| 有效 TPS 小于 0.01 | `<0.01 tok/s` |
| 共用表格真实耗时插槽 | 首字、总耗时、TPS 同时显示 |

**完成条件：**管理员与用户页面显示同一口径；相关测试、lint 和构建通过；视觉结果已检查；没有迁移或接口改动。

验证命令，从仓库根目录执行：

```bash
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend exec vitest run \
  src/components/admin/usage/__tests__/UsageTps.spec.ts \
  src/components/admin/usage/__tests__/UsageTable.spec.ts \
  src/components/common/__tests__/HelpTooltip.spec.ts
pnpm --dir frontend exec eslint \
  src/components/admin/usage/UsageTps.vue \
  src/components/admin/usage/UsageTable.vue \
  src/components/admin/usage/__tests__/UsageTps.spec.ts \
  src/components/admin/usage/__tests__/UsageTable.spec.ts \
  src/i18n/locales/zh/dashboard.ts src/i18n/locales/en/dashboard.ts \
  src/components/common/HelpTooltip.vue \
  src/components/common/__tests__/HelpTooltip.spec.ts
pnpm --dir frontend run build
```

依赖未变化时复用安装结果。当前 build 已包含 i18n 键完整性、Vue 类型检查和 Vite 构建，不额外重复同一候选的独立 typecheck。实际窄屏验收修复了 HelpTooltip 的共享定位问题，因此本轮 lint 和定向测试也覆盖该组件及其测试；不扩大到无关文件。安装和 build 子命令统一使用发布工作流对应的 pnpm 9，保持 lockfile 不变。

## P2. 建立 KlN 同步 PR

**主要位置：**`.github/workflows/sync-upstream.yml`、`personal-ci.yml`（新增）、已有 `backend-ci.yml` 的必要触发入口、来源记录；提取脚本时放在实际维护目录，避免在 YAML 内堆积难以验证的逻辑。

- [x] **P2.1 固定输入和输出。**定义目标 personal、KlN 上游、指定标签/完整 SHA 的手动输入，以及上游 SHA、基础 personal SHA、候选 SHA、PR URL 等非敏感结果。输入经过校验，用环境变量传入 shell。
- [x] **P2.2 解析 KlN 发布版本。**按数字版本选择 `vX.Y.Z-klno.N`，保留被标记为 prerelease 的 KlN 发布；将上游标签拉入独立引用空间，不覆盖 origin 标签。
- [x] **P2.3 准备候选合并。**从当次 personal HEAD 创建 `codex/sync-klno-<版本或SHA>-<基线短SHA>-<目标短SHA>`，普通 merge 合入固定上游 SHA并更新来源记录；不重写 main/personal 历史。
- [x] **P2.4 处理无更新、重跑与冲突。**无变化直接结束；同一目标/基线复用候选 PR；冲突中止并报告文件；基础分支变化后重新合并检查，不使用旧证据。上游历史被改写时进入人工差异审查。
- [x] **P2.5 实现 personal-ci。**覆盖 personal PR、personal push 和显式 dispatch，运行 P1 相关测试和构建；显式输入候选完整 SHA并核对 checkout，对同一未变化候选复用有效结果。
- [x] **P2.6 确保必要 CI 实际运行。**保留所选 KlN 基线的既有检查，读取实际 Rulesets/分支保护；机器人 push/PR 后显式触发可用入口，处理必要检查待批准情形。最终结果必须对应实际待合并提交及最新基础分支。**候选 9c33a062/base 029cd8fb 的显式及原生检查通过，personal push 检查通过，PR 实际 required personal-ready=pass；严格最新基础/App 15368 保护已读回。**
- [x] **P2.7 验证写入权限。**核对 PR/dispatch 权限，以及上游修改工作流文件时的 push 权限。权限不足明确停止；使用已有授权的本机凭据处理，或另行配置限定仓库的同步令牌，不删除工作流变化来规避限制。
- [x] **P2.8 保留上游合并关系。**PR 使用 merge 方式合入；PR正文列明上游标签/SHA、个人基线、候选 SHA、迁移变动和 CI 结果。默认分支与 personal 同步维护自定义流程定义。
- [ ] **P2.9 验证流程并独立复核。**临时 Git 仓库覆盖保留 TPS、`.9/.10` 排序、无更新、重复运行及冲突停止；再创建一次真实候选，确认机器人 CI 触发和检查归属。候选工作流由独立只读复核检查历史保留、输入和权限边界。**临时仓库、机器人演练 PR 和两次独立复核已有证据；验收期间发布的 v0.2.14-klno.5 非记录来源后代，流程停止待人工历史差异审查，真实升级候选未创建，不能用演练替代。**
  - [x] 临时 Git 仓库及输入/绑定合同：21 个用例通过。
  - [x] 真实 GitHub Actions 创建演练 PR，并显式触发准确候选 SHA 的检查；无更新、重跑、错误输入和旧基础拒绝有实际证据。
  - [x] 两次独立只读复核，确认问题已解决，审计 Bundle 校验通过。
  - [ ] 新 KlN 发布的真实升级候选；v0.2.14-klno.5 已发现，历史改写审查未完成。
- [x] **P2.10 启用安全的定时入口。**完成上述证据后，在默认分支启用每日检查及手动入口。首次 TPS 上线前，候选 PR 可保留待审，不合入另一生产上游版本。**实际 CI/独立复核/保护验收后，于 UTC 16:27:08 设置并读回定时变量 true，每日 UTC 03:17 定义 active；首个真实 schedule 事件尚未发生。**
- [ ] **P2.11 验收实际上游升级，依赖首次 P4 完成。**同步并审查一次新的 KlN 版本，合入 personal 后重验 TPS，按 P3/P4 的已建立流程发布、部署并核对迁移兼容性；记录实际升级结果。

**流程准备完成条件：**P2.1–P2.8、P2.10 以及 P2.9 的临时仓库、实际机器人候选/CI和独立复核有证据；个人功能保留，检查与准确候选及最新基础对应，无更新不会重复发包，冲突/权限失败不改变运行版本。没有新发布或新发布因历史改写而停止时，P2.9 的真实升级候选保持未勾选；首次原基线加 TPS 的 P3 准备仍使用已验收的固定 personal。**完整阶段完成条件：**首次 TPS 部署后补齐真实升级候选及 P2.11，不能将流程演练等同实际升级成功。

## P3. 构建并发布个人镜像

**主要位置：**`.github/workflows/personal-release.yml`（新增）、`release.yml`、`.github/release-tools/`；复用已有 GoReleaser 配置。

- [x] **P3.1 准备 fork 发布设置。**核对 Actions、PR 创建、包写入权限及 GHCR 可见性。默认公开 `ghcr.io/ccisnoxx/sub2api`；私有部署方案需要单独具备拉取权限。
- [x] **P3.2 绑定发布来源与检查。**自动入口只接受可信 personal 的成功检查；记录待发布 SHA，确认实际必要检查通过。PR候选、其他仓库和失败/取消结果不产生发布。
- [x] **P3.3 实现版本分配。**从来源记录生成 `v<上游基础版本>-tps.N`；同一源码 SHA 重跑复用标签；串行分配，标签已指向其他提交时失败，不移动标签。
- [x] **P3.4 调整 Release 入口。**个人标签显式 dispatch `release.yml`，设 `simple_release=true`；限制 fork 自动 tag 入口只接收个人标签，保留固定 source SHA 与产物验证。
- [x] **P3.5 停止版本写回 main。**个人发布跳过 `sync-version-file`；检查构建用 VERSION 与版本标签一致，避免发布后自动提交引起下一轮同步冲突。
- [x] **P3.6 处理重跑与文档改动。**已经成功的版本不覆盖；失败恢复保持同一 source SHA/tag；只有文档变更而应用、构建及运行资源未变化时不重新发布。
- [x] **P3.7 完成定向验证与复核。**运行受影响的发布 helper 测试、脚本语法检查与 simple dry run；验证错误 SHA、重复标签、检查不满足时被拒绝。独立只读复核检查来源信任、门禁与发布权限。
- [x] **P3.8 发布首次个人版本。**明确选择原上游基线加 TPS 的 personal 提交，实际发布 amd64 镜像；核对标签提交、OCI revision、版本、平台和 digest，验证镜像可拉取。
- [x] **P3.9 保存发布结果。**记录上游标签/SHA、个人标签/SHA、workflow revision、镜像 digest、平台和 Actions 链接。构建尚在运行或发布失败时不得进入 P4 的生产部署。

**完成条件：**simple dry run 和相关检查通过；实际镜像成功发布；版本、源码 SHA、digest 能对应；不存在重复或错误来源发布。**已取得证据：**PR #3 普通 merge 后的 personal `896de21b4be7f4ec4b4236f4df663b47371665b0` 全新完整 CI、simple dry run、独立复核、正式 Release、同 SHA 重跑均通过；`v0.2.14-klno.3-tps.1` 对应公开 linux/amd64 镜像。空凭据目录完成全部层拉取和 SHA-256 校验，供 P4 使用的固定引用为 `ghcr.io/ccisnoxx/sub2api@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`。详情见 implementation-evidence.md 第 7 节；P4 尚未执行。

发布工具验证命令，从仓库根目录执行，在包含 PyYAML 的独立 Python 环境中运行：

```bash
python -m pip install -r .github/release-tools/requirements-release.txt
python -m unittest discover -s .github/release-tools -p 'test_*.py'
bash -n .github/release-tools/release-images.sh
```

simple dry run 的示例命令，需要工作流已经更新并推送到可运行的 personal：

```bash
gh workflow run release.yml --repo ccisnoxx/sub2api --ref personal \
  -f tag=personal -f source_sha=<当前personal完整SHA> \
  -f simple_release=true -f dry_run=true
```

dispatch 只代表开始运行；跟踪对应 run，检查固定源码 SHA及全部所选产物验证结果后才能完成 P3.7。正式发布使用已存在的个人版本标签作为 `tag` 输入，不使用分支名代替标签。

## P4. 建立部署入口并完成首次更新

**主要位置：**`deploy/personal/deploy-hostdzire.sh`（新增）、必要的远端执行脚本和测试；hostdzire 的既有 Compose 与独立镜像选择文件。

- [ ] **P4.1 明确部署输入。**脚本接受指定版本或 digest，验证预期仓库与平台，拒绝 latest；版本解析成固定 digest后，同一引用贯穿拉取、启动、验收和记录。
- [ ] **P4.2 实现预检与部署互斥。**使用现有 SSH 别名只读核对实际部署目录、Compose 项目和服务、数据库/Redis 健康、旧镜像与挂载；实际更新开始时建立部署锁防止并发覆盖。
- [ ] **P4.3 实现镜像配置方式。**基于实际生产 Compose 参数化应用 image，用独立受限文件保存选择；显式控制变量来源。保留现有卷、端口和其他服务，不用模板替换生产配置。
- [ ] **P4.4 实现更新顺序。**先保存旧 digest/revision和必要配置备份，检查迁移差异，再拉取新镜像；临时选择新 digest，只更新应用服务并等待健康，成功后持久保存选择和部署结果。
- [ ] **P4.5 实现显式失败与回滚。**拉取失败保持原服务；健康失败报告诊断及当前实际状态；相同基线且无不兼容变更时支持显式 `--rollback`，涉及迁移时停止自动切回镜像。
- [ ] **P4.6 演练脚本并独立复核。**用 SSH/Docker 替身覆盖成功、拉取失败、健康超时、回滚、部署锁和迁移限制；检查未更新数据库/Redis、未删除卷、未输出凭据。独立只读复核生产更新及失败状态处理。
- [ ] **P4.7 准备实际更新条件。**核对新旧版本差异、维护窗口、部署与测试调用的现有授权；首次切换或有迁移时准备可恢复备份，确认可用旧镜像和恢复步骤。
- [ ] **P4.8 执行首次更新。**以 P3 成功发布的固定 digest部署，保留 Compose 项目与挂载；等待应用健康，核对运行 digest、revision、版本和 `/health`。
- [ ] **P4.9 验收使用体验。**登录管理员与用户页面，核对历史 TPS和原有耗时；执行经授权的最小网关请求，验证转发与记录。单容器更新导致的连接中断按实际情况登记。
- [ ] **P4.10 完成可恢复性验收。**保存首次部署结果和适用的回滚证据；以临时实例或适当窗口完成回滚演练，不为了验证而随意重复切换生产。观察期内保留旧镜像与备份。

**完成条件：**脚本演练通过，实际运行镜像与发布结果一致，页面与网关烟测有证据，回滚范围及恢复步骤明确。未执行生产更新时只能标注“脚本准备完成”，不能勾选 P4.8–P4.10。

部署入口正式用法在 P4.1 确定接口后写入脚本帮助和此文档；当前不提供假定已经实现的部署命令。

## 阶段交付与证据登记

建议提交按“基线保护 / TPS / 同步与 CI / 发布 / 部署”拆分。每个提交只包含该阶段负责的改动，保留其他人的工作；上游更新提交与个人功能提交可分别追踪。

在 `implementation-evidence.md` 每次登记以下内容即可：

| 字段 | 要求 |
|---|---|
| 任务编号与状态 | 对应本文编号；说明完成、失败、待执行或不适用的依据 |
| 候选来源 | 分支、源码完整 SHA、上游标签/SHA；必要时写实际合并 SHA |
| 实际验证 | 命令或 Actions 链接、结果、运行环境及关键输出摘要 |
| 外部产物 | 已创建 PR、真实镜像 digest、服务器实际运行 revision；尚未创建时留空 |
| 失败与限制 | 区分代码失败、环境缺失、权限限制、未执行检查与已确认的无关失败 |
| 复核与下一步 | 独立复核结果、剩余任务、必要授权或条件；不将自查记为独立审查 |

实施结束前检查实际 diff与工作树，确认没有覆盖无关工作、引入隐藏兜底、泄露凭据或改动生成文件。只有材料风险取得对应证据后完成相应任务；不为增加测试数量重复已经有效的验证。

**当前实施状态：**P0/P1、P2.1–P2.8、P2.10 已完成，main/personal 控制定义一致且远端生效，最终候选与 personal 的完整 CI、PR 必要门禁、严格保护及定时开关已验收。P2.9 的临时仓库、实际机器人演练和独立复核完成；真实 v0.2.14-klno.5 升级因历史改写停止，普通合并预览有 120 文件冲突。P3.1–P3.9 已完成，tps.1 已公开发布并匿名完整拉取验证；固定源码与digest见实施证据第7节。P4本轮正在实施，生产尚未切换。P2.9真实升级及P2.11仍待另行处理。
