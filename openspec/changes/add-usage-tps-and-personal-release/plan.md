# Sub2API 平均输出 TPS 与个人分支维护方案

- 编制日期：2026-10-08（America/Los_Angeles）。
- 状态：P5 展示候选、本地页面验收及最小 Go/依赖安全补丁完成；安全扫描、后端单元测试通过，完整候选 CI 被现有 HTTP/2 接口弃用告警阻塞。精确 lint 配置兼容补丁已准备，等待新增范围授权；尚未合并、发布或部署。P0–P4 既有范围已完成，hostdzire 仍为固定 tps.1；P2.9 真实 KlN .5 历史升级与 P2.11 仍待处理。实际结果及身份验收边界见 [实施证据](implementation-evidence.md)。
- 执行入口：[实施任务清单 tasks.md](tasks.md)，按 P0–P4 编号逐项实施并登记验证结果。
- 讨论来源：[排查 Codex fast 开关配置](codex://threads/01a11a74-fc24-7473-a94f-e61b253867ac)，重点承接该会话最后关于 TPS、自有分支、镜像构建与 hostdzire 更新的讨论。
- 目标仓库：`ccisnoxx/sub2api`；功能上游：`KlN-4096/sub2api` 的 `klno` 发布线。
- 本文位于现有 `openspec/changes/` 中，作为整合方案；不代表已经完成 OpenSpec 规格、代码、工作流或部署。

## 1. 目标与推荐路径

推荐从当前部署对应的 `v0.2.14-klno.3` 建立 `personal` 维护分支，以独立的小提交增加平均输出 TPS。后续通过普通 merge 和同步 PR 跟随 KlN 发布版本，复用现有 Release 工具构建 `linux/amd64` GHCR 镜像，由本机 SSH 脚本更新 hostdzire。

完成后，日常操作应为：查看同步 PR、处理必要冲突、合并、等待镜像成功发布、执行一次部署命令。检测上游、准备候选更新和镜像构建可以自动完成；生产更新由明确选择版本的部署动作启动。

第一版范围：

- 管理员与用户的使用记录，在现有耗时栏中增加“平均输出 TPS”。
- 固定维护分支、上游基线记录、同步 PR、TPS 定向验证和镜像发布入口。
- 本机到 hostdzire 的部署入口、健康检查、部署记录和适用条件明确的回滚。

本轮方案不扩大到 Fast 策略修改、服务档位计费调整、Codex 桌面端开关、WireGuard/Nginx/防火墙优化。TPS 第一版也不引入后端计时字段、数据库迁移、音频速率、全站 TPS 排序或统计、CSV 导出新列、公开 Key 用量页面改造及零中断发布。

## 2. 方案编制时的基线与证据边界

| 项目 | 核实结果 | 对实施的影响 |
|---|---|---|
| 本地目录 | `/Users/sc/PycharmProjects/sub2api-kin` | 本文在该仓库中保存 |
| 当前本地分支 | `main` | 尚未建立 `personal` |
| 本地 HEAD | `09d17249b2e80fbce6b7cfc052f7815559fe815d` | 不能仅按 VERSION 文件认定它包含 KlN 功能 |
| Git 远程 | `origin` 为 ccisnoxx，`upstream` 为 KlN-4096 | 可以复用已有远程命名 |
| 本地发布标签 | `v0.2.14-klno.3` 解引用为 `de08df02ae1d81668a22f798b398aa0438ac1276` | 推荐的初始功能基线 |
| 本地远程跟踪引用 | `upstream/klno` 也指向 `de08df02ae1d81668a22f798b398aa0438ac1276` | 这是本次读取的本地引用，不代表远端永远不变 |
| 两条线的差异 | `main..v0.2.14-klno.3` 涉及 528 个文件 | 将 TPS 直接加到 main 再部署，可能丢失服务器原有 KlN 功能 |
| 共用使用记录组件 | `frontend/src/components/admin/usage/UsageTable.vue` | 管理员与用户页面可通过一次接入同时获得 TPS |
| 已有字段 | `output_tokens`、`duration_ms`、`first_token_ms`、图片 token 字段 | 普通文本平均 TPS 不需要新接口或迁移 |
| 已有耗时布局 | `cell-latency` 展示首字、总耗时及健康度色条 | 在同一栏增加第三行 |
| 已有发布能力 | `release.yml` 支持 `simple_release` 和 `dry_run` | 复用现有构建和产物校验 |
| 文档存放规则 | `.gitignore` 默认忽略新增 `docs/*`，本计划路径未被忽略 | 使用现有 OpenSpec 目录，避免计划被漏提交 |

原会话已经检查过的服务器信息：hostdzire 为 x86_64，部署目录 `/root/sub2api-kin`，Compose 服务名 `sub2api`，容器名 `sub2api-kin`，当时镜像为 `ghcr.io/kln-4096/sub2api:0.2.14-klno.3`，应用数据挂载到 `/app/data`。原会话还确认当时的 Compose 支持 `--wait` 和 `--wait-timeout`。

**这一节记录方案编制时的证据边界；2026-10-08 实施时已重新 SSH 核对，实际结果见 implementation-evidence.md。原会话事实本身不代替当前运行证据。** 首次实施时需要核对运行镜像 revision、实际 Compose 文件、项目名、挂载和数据库服务；若已发生变化，先更新基线再部署。原会话的个人账号、请求原文与认证信息不进入本方案。

方案编制时已检查 `main` 源码，并对初始发布标签中的 UsageTable、使用记录类型、测试插槽和 CI 配置作了针对性核对。发布与同步工具在两条基线中一致，`backend-ci.yml` 有差异，实施时采用所选 `klno` 基线的实际版本。

## 3. TPS 展示合同

### 3.1 指标含义

第一版统一采用整条请求的平均输出速率：

```text
平均输出 TPS = output_tokens × 1000 ÷ duration_ms
```

适用于可识别的普通文本输出记录，流式、非流式及 WebSocket 文本请求使用同一口径。分母沿用接口记录的总耗时，不减去 `first_token_ms`。例如 `output_tokens=1040`、`duration_ms=24650` 时紧凑显示 `42.2 tok/s`。

提示文案说明：该值包含首 token 等待时间，输出统计可能包含推理 token；反映记录中的平均输出速率，不能单独证明 Fast 生效或代表屏幕可见文字的纯生成速度。保留首字与总耗时，便于结合请求档位、模型和输出长度比较。

[Plus 固定提交的实现](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/frontend/src/utils/usageTiming.ts)可作为口径参考。该实现还依赖音频 token、计时版本、首输出类型和完成状态等字段；本项目第一版按现有字段定义自己的可用范围，不直接覆盖其表格或复制整套计时状态判断。

### 3.2 可用范围与缺失值

| 记录条件 | 当前显示（P5） |
|---|---|
| 普通文本记录，输出 token 为有限正数，总耗时为有限正数 | 紧凑显示，例如 `42.2 tok/s` |
| 历史记录满足上述条件，缺少首字耗时 | 正常计算，首字耗时不参与公式 |
| 输出为 0、缺少输出、输出为负数或非有限数 | `-`，提示无有效输出统计 |
| 总耗时缺失、为 0、为负数或非有限数 | `-`，提示缺少有效总耗时 |
| 图片生成、图片输出 token 大于 0、明确的视频/其他媒体记录 | `-`，提示当前记录不适用 |
| 仅有图片输入、输出仍为普通文本 | 可以计算 |
| 原生 compaction、live、probe 或 gwpool_degraded 记录 | `-`，不作为普通文本生成速率 |
| 有效结果小于 0.1 tok/s | 两位有效数字，例如 `0.0081 tok/s`，保留低速正值 |

沿用 `billingMode.ts`、`imageUsage.ts` 和请求类型工具的既有识别合同。`billing_mode=token` 或缺省的历史文本记录可参与计算；明确的 image、video、per_request 记录暂不参与。未知模式不猜测为文本。媒体识别还应检查已返回的图片输出字段，不能只依赖 `isImageUsage()`，因为该函数允许 token 计费的图片请求通过。

当前前端类型没有 Plus 的 `audio_output_tokens`、`timing_version`、`first_output_kind`、`is_complete`。不能宣称已经排除了所有无法识别的音频混合输出或确认所有记录完整。若真实用量发现这些记录，应先限定适用入口；新增数据字段属于后续单独设计，不为第一版填造这些字段。

### 3.3 最小改动组织

| 文件或模块 | 计划改动 |
|---|---|
| `frontend/src/components/admin/usage/UsageTps.vue`，新增 | 接受已有记录字段，负责可用性判断、计算、格式化和可访问的说明提示 |
| `frontend/src/components/admin/usage/UsageTable.vue` | 只增加组件导入及 `cell-latency` 的第三行接入，保留现有色条和耗时行 |
| `frontend/src/i18n/locales/zh/dashboard.ts` | 新增平均 TPS、口径说明、不可用原因等文案 |
| `frontend/src/i18n/locales/en/dashboard.ts` | 同步增加对应英文键 |
| `frontend/src/components/admin/usage/__tests__/UsageTps.spec.ts`，新增 | 保护计算、格式化及不可用记录的展示合同 |
| `frontend/src/components/admin/usage/__tests__/UsageTable.spec.ts` | 渲染 `cell-latency`，验证真实表格接入及既有耗时保留 |
| `frontend/src/components/common/HelpTooltip.vue` 及其测试 | 窄屏验收暴露 fixed 坐标叠加滚动量和边缘越界；在既有定位 owner 修正视口坐标与边距，保护已有 hover/click 交互 |

计算和不可用原因仍由 UsageTps 组件唯一拥有；`timing` 插槽将同一口径说明交给 UsageTable 的首字旁 HelpTooltip，避免复制判断。尽量不改 `UsageView.vue` 的列配置，因为第一版继续使用现有耗时栏，不新增排序字段。提示复用现有组件或项目交互方式，兼顾键盘焦点，不引入新依赖。

TPS 代码、测试和文案作为一个功能提交；同步、发布、部署分别独立提交。以后上游提供相同指标时，核对计算口径和媒体处理后撤除自定义组件及接入点。

## 4. 分支、来源记录与同步 PR

### 4.1 分支职责

| 分支 | 职责 | 更新方式 |
|---|---|---|
| `main` | 暂保留为默认分支，承载定时工作流和计划等仓库控制文件 | 普通提交或 PR；停止继承工作流对 main 的重建 |
| `personal` | KlN 功能、自定义 TPS 和可发布的应用源码 | 功能 PR及上游同步 PR，保留历史 |
| `codex/usage-tps` 等 | 独立功能开发 | 从 personal 创建，完成后合入 |
| `codex/sync-klno-<版本或SHA>-<基线短SHA>-<目标短SHA>` | 某次上游更新候选 | 从当次 personal HEAD 创建，合并指定 KlN 版本 |

初始 `personal` 从已核实的 `v0.2.14-klno.3` 创建。不要从 `main` 创建后仅修改 VERSION，也不要在应用分支使用 GitHub 的同步 fork 按钮替代本文的 KlN 同步流程。首次发布先保持该上游基础不变，让 TPS 与升级问题可以分别判断。

在 `deploy/personal-source.json` 新增最小来源记录：上游仓库、已合入的上游标签及完整 SHA。同步候选成功合并后更新该记录。自有发布标签、镜像 digest 与部署时间记录在发布结果和服务器部署记录中；不靠 `git describe` 从混杂的上游/个人标签中猜测来源。

### 4.2 同步流程

```mermaid
flowchart LR
    A[检测 KlN 发布标签] --> B[从 personal 创建候选]
    B --> C[合并固定上游 SHA]
    C --> D[检查差异与运行相关 CI]
    D --> E[同步 PR 人工合并]
    E --> F[发布固定提交的自有镜像]
    F --> G[本机 SSH 更新 hostdzire]
```

`.github/workflows/sync-upstream.yml` 每日 UTC 03:17 检查，并保留手动指定标签或完整 SHA 的入口。部署定义时保持仓库变量 `PERSONAL_SYNC_SCHEDULE_ENABLED=false`，实际候选 CI、独立复核与严格基础保护验收后才设为 true；不创建 Codex 定时任务。显式 `verify_ci=true` 只生成标明“无上游升级、请勿合并”的演练草稿，不修改来源记录，定时入口不启用该选项。

执行合同：

1. 上游明确为 `KlN-4096/sub2api`，自动选择其 `vX.Y.Z-klno.N` 发布标签；手动 SHA 是显式例外。
2. 从上游单独解析标签与 SHA，固定本轮目标。KlN 标签可能被 GitHub 标为 prerelease，不能简单过滤 `prerelease=false`，也不能沿用仅匹配 `vX.Y.Z` 的旧规则。排序应识别数字版本，不能按字符串把 `.10` 排到 `.9` 前面。
3. 上游标签使用独立引用空间保存，不强制覆盖本仓库的标签。目标已在来源记录中、且没有新内容时直接结束，不产生空 PR 或重复构建。
4. 从最新 personal 创建候选，使用普通 merge 合入固定目标，保留个人功能提交。不 rebase 或强制推送 personal/main，不自动选择 ours/theirs。
5. 冲突时中止该候选合并，列出文件与目标 SHA，保留现有部署和维护分支。若 KlN 改写发布线历史，先检查祖先关系与实际差异，再人工处理；不能假设每次都可无冲突快进。
6. 更新来源记录并创建同步 PR。记录基础 personal SHA、上游 SHA、迁移变动和检查结果。personal 已变化时重新准备候选，不复用旧基线的成功证据。
7. 同一候选重跑应复用对应 PR；候选内容变化需要新的 SHA 和检查。PR 合并采用保留上游祖先关系的 merge，避免把同步内容 squash 成丢失历史关系的大提交。

继承工作流当前从 `Wei-Shaw/sub2api` rebase `klno`，随后强制推送、打标签、发布，并重建 main。这些行为必须在 fork 中改造后再开启定时同步。KlN 的 codex 身份漂移维护责任仍由 KlN 处理；不照搬本仓库旧的固定 codex 版本检测来阻塞每次个人同步，必要时检查所同步 KlN 版本的发布说明和已有测试结果。

定时工作流需要存在于默认分支，因此同一套自定义同步逻辑应同时出现在 main 与 personal 中，防止后续上游 merge 恢复旧逻辑。所有 shell 输入经环境变量传入并正确引用；标签/SHA先验证，不把 dispatch 输入直接拼入 shell 脚本。

同步控制目录和同步、Personal CI、既有 CI、安全扫描定义的上游差异需要人工审查，自动流程明确停止。来源记录的 SHA 必须是候选祖先，即使候选树相同也不能接受丢失历史的 squash。已发布标签以固定的仓库标签为准；显式未发布 SHA 还需核对当前 klno 祖先关系，并记录 `upstream_tag=null`。

### 4.3 检查触发与权限

新增 `.github/workflows/personal-ci.yml`，提供 personal PR、personal push 以及 `workflow_dispatch` 的定向检查入口。同步工作流用 `contents: write`、`pull-requests: write`、`actions: write` 完成所需操作，其他任务维持只读权限。

上游变更包含 `.github/workflows/` 时，还必须核对同步令牌对工作流文件的写入权限；`contents: write` 不等于拥有该权限，继承的同步脚本已包含这种推送失败的处理。首次可使用 `GITHUB_TOKEN`，遇到权限不足时明确停止，并由本机已授权的 Git 凭据推送已审查候选；不能丢弃上游工作流改动或绕过分支规则。若希望这种更新也全自动，再单独配置仅限本仓库的 GitHub App 或 `UPSTREAM_SYNC_TOKEN`，授予实际需要的 Contents/Workflows 写入权限；令牌只保存在 GitHub Secrets，PR 创建和 dispatch 可继续使用短期 `GITHUB_TOKEN`。[GitHub 对工作流文件修改权限的说明](https://docs.github.com/en/rest/repos/contents#create-or-update-file-contents)

以 `GITHUB_TOKEN` 推送的分支不能作为后续 push CI 自动启动的可靠依据。创建候选后显式 dispatch 定向 CI，传入并核对候选完整 SHA，结果挂在该提交上；不要把“PR 已创建”当成“检查已运行”。当前 GitHub 还可能让机器人创建的 PR 检查进入待批准状态，实施时验证仓库实际行为。[GitHub 的工作流触发规则](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow)

保留所选 KlN 基线的已有 CI。显式 dispatch 的定向检查不能代替仓库实际要求的其他检查；若已有 CI 待批准，完成批准，或为其增加等价的显式入口。首次设置时读取实际分支保护和 Rulesets，确定必要检查名称及 merge 提交验证方式，不假定当前已经配置了保护规则。最终门禁绑定最终候选提交，personal 基础分支前进后刷新合并结果。

实际 Personal CI 通过同提交的 reusable workflows 保留上述完整检查，`personal-ready` 开始/结束核对候选、来源及最新 personal。远端 personal 已配置 strict 的 `personal-ready`（GitHub Actions App 15368），管理员也受约束，禁止强推/删除；仓库只允许 merge commit。阶段任务、执行证据与开发日志以默认 main 的最新记录为准，personal 保存固定源码的阶段记录，结果登记不反复改变已验收的 personal SHA。

## 5. 自有镜像与发布合同

复用 `.github/workflows/release.yml`、`.github/release-tools/` 与 `.goreleaser.simple.yaml`，目标镜像仓库为 `ghcr.io/ccisnoxx/sub2api`，使用 `simple_release=true`，只选择 `linux/amd64`。保留现有 frontend 一次构建、固定源码 SHA、产物来源和校验和验证，不另起一套服务器编译流程。

版本示例：

```text
上游标签：v0.2.14-klno.3
个人标签：v0.2.14-klno.3-tps.1
镜像标签：ghcr.io/ccisnoxx/sub2api:0.2.14-klno.3-tps.1
部署引用：ghcr.io/ccisnoxx/sub2api@sha256:<实际发布的digest>
```

现有 `release_matrix.py` 的版本格式允许该个人标签。正式发布需要已存在的 `v` 前缀版本标签，且源码 checkout 必须与标签提交一致；分支名仅可用于 dry run。

新增 `.github/workflows/personal-release.yml`，在 personal 的定向检查成功后准备标签并显式 dispatch Release，也提供手动恢复入口。自动入口可使用 `workflow_run`，但必须同时核对成功结论、仓库、`head_branch=personal`、待发布完整 SHA 和实际必要检查；PR候选通过检查不能触发生产发布。只从可信 personal 提交取代码，不读取或执行不可信 PR 产物。

发布动作要求：

- 从 `personal-source.json` 得到基础标签，在该基础下分配 `tps.N`，固定待发布 SHA；同一 SHA 重跑复用标签，不另增版本。
- 发布准备串行执行。标签冲突必须比较已有目标 SHA，不能移动标签；镜像版本发布成功后不覆盖。失败重跑不得使用另一提交覆盖同一版本。
- 机器人推送标签后显式调用 `release.yml`，不依赖 tag push 再触发。fork 的自动 tag 入口只接收个人标签，避免把上游原始标签误发布成自己的版本。
- Release 的 workflow revision 与所选 source tag 分别记录，dispatch 的 ref 选包含自定义流程的 personal；工具版本和应用版本都可追溯。
- 修改 `sync-version-file`，个人发布跳过向默认 main 写回 VERSION。现有 prepare 已生成构建用 VERSION，不需要额外污染分支或引起下一轮同步冲突。
- 只改文档而应用、构建、运行资源均未变化的提交不必重新发布；已有标签的失败发布可显式恢复。
- 成功报告列出上游标签/SHA、个人标签/SHA、镜像 digest、构建 run 链接与平台。workflow 已提交或正在运行不能表示发布成功。

首次准备 GitHub Actions 的 fork 启用状态、允许 Actions 创建 PR 的设置、工作流写入权限和 GHCR 包可见性。公开镜像可由服务器匿名拉取；若选择私有镜像，另配置服务器拉取权限。默认方案沿用原会话的公开 GHCR，不把 SSH 私钥放入 Actions。[GHCR 权限与公开拉取说明](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)

## 6. hostdzire 部署与回滚

本机入口拟新增 `deploy/personal/deploy-hostdzire.sh`；服务器单次执行逻辑可放在同目录的独立脚本中。使用现有 SSH 别名和 OpenSSH 认证，不把私钥、数据库口令、API Key 或服务器 `.env` 放进 Git。

服务器 Compose 初次适配只把应用 image 改成 `${SUB2API_IMAGE:?必须指定应用镜像}`。保留 `/root/sub2api-kin` 的实际 Compose 项目名、容器设置、数据卷、端口和其他服务，不用仓库模板覆盖生产 Compose。镜像变量存放在独立的受限文件，由脚本显式传入，避免被已有 shell 环境或另一 `.env` 的同名变量覆盖。

部署入口接收明确的版本或 digest；版本先解析为 digest，再拉取同一引用。执行顺序：

1. 核对服务器架构、Compose 能力、应用服务、数据库/Redis 状态和当前镜像。创建部署锁，阻止两次部署并发覆盖镜像状态。
2. 校验镜像来源为预期仓库，读取已发布 digest、源码 revision 和平台，拒绝把 `latest` 作为生产目标。
3. 保存旧镜像 digest、实际运行 revision、镜像变量文件和必要的 Compose 备份；备份不上传仓库。根据上游差异检查迁移、启动配置和运行资源变化。
4. 首次切换或存在迁移时，准备可恢复的数据库与持久数据备份，并明确旧版本兼容性。备份失败时停止。
5. 先拉取新 digest。拉取失败不修改当前运行容器或持久镜像选择。
6. 临时使用新镜像选择执行配置校验，只核对必要字段，避免将完整含敏感信息的 Compose 配置输出到日志。
7. 更新单个应用服务并等待健康，通过后再持久保存新镜像选择和部署记录；超时或失败保留失败状态及诊断，不宣称完成。
8. 核对容器实际镜像、健康状态、`/health` 和版本；登录使用记录页面检查历史 TPS、首字和总耗时。网关烟测使用经授权的测试请求，不能将健康检查当成网关和前端均可用的证明。

核心 Compose 动作如下；完整脚本还需完成上述镜像选择、记录与失败处理：

```bash
cd /root/sub2api-kin
docker compose pull sub2api
docker compose up -d --no-deps --wait --wait-timeout 120 sub2api
```

实际执行时，两个命令必须使用同一 Compose 文件、项目名和目标镜像变量。Docker 文档说明更新镜像会重建容器并保留挂载卷，`--wait` 等待服务运行或健康；没有 healthcheck 时不能据此认定应用健康。[Docker Compose up](https://docs.docker.com/reference/cli/docker/compose/up/)

单容器更新会造成短暂中断，正在进行的 HTTP 流和 WebSocket 可能断开。首次部署应选择合适窗口；本方案不承诺无中断。

回滚分两种情况：

- **相同上游基线且仅增加 TPS/构建流程，确认没有不兼容迁移或配置变化：**恢复旧 digest 和镜像选择，重建应用、等待健康，并记录回滚结果。部署脚本可以支持显式 `--rollback`。
- **上游升级涉及迁移或旧程序不兼容：**停止自动切回镜像。按迁移兼容性和已验证备份制定恢复动作；恢复数据库可能丢失备份后的写入，需要维护窗口及相应操作授权。

保留旧 digest 和可用旧镜像到观察期结束，不在部署过程中清理卷或旧镜像。一次性部署不得执行 `docker compose down -v`。

## 7. 实施阶段与完成条件

| 顺序 | 阶段 | 交付物 | 进入下一阶段的条件 |
|---|---|---|---|
| P0 | 保护并对齐基线 | 修正默认分支旧同步逻辑；personal 分支；来源记录 | 确认起点包含当前部署 KlN 功能，旧重建/强推流程不再执行 |
| P1 | 实现 TPS | 小组件、表格接入、双语文案、定向测试 | 计算与不可用展示正确，管理员/用户页面可见，前端构建通过 |
| P2 | 建立同步 PR | 改造 sync-upstream、定向 CI 和必要检查入口 | 无更新、正常更新、冲突和机器人检查触发均有实际验证 |
| P3 | 建立镜像发布 | personal-release、Release 调整、发布结果记录 | simple dry run 通过，首次镜像成功发布且版本/SHA/digest 一致 |
| P4 | 建立生产更新入口 | 本机/远端脚本、最小 Compose 镜像配置、部署记录 | 演练失败和回滚后，授权部署指定版本并完成健康与界面验收 |

P0 的流程文件需要出现在默认分支，不能只在 personal 上修正。本文当前保存于 main 的工作区，实施时也要将计划和来源记录带入 personal，避免创建基线分支后遗失文档。

首次建议先完成 P0/P1，发布“原上游基线 + TPS”的版本；再验证 P2 的一次上游同步。不要让首次 TPS 上线同时包含大量未核对的上游功能变动。

每阶段结束检查实际 diff 和工作树。任何未完成的检查、外部权限或环境限制写入实施记录，不把计划中的步骤打勾为已完成。

## 8. 验证范围与验收标准

### 8.1 TPS 的定向验证

`UsageTable.spec.ts` 的 `DataTableStub` 已在 P1 补齐 `cell-latency`，测试继续真实渲染 UsageTps；P5 增加标签、颜色及首字旁唯一说明入口的合同断言。

最小行为用例：示例 `1040/24650 → 42.2`；首字为空仍可计算；无效耗时/输出显示 `-`；图片、视频及 compaction/live/probe/gwpool_degraded 显示不可用；图片输入+文本输出可用；小于 0.01 的正值正确显示；表格同时保留首字、总耗时和 TPS。计算用例放在组件测试，表格测试只保护接入合同，避免重复同一组数学边界。

实施后从仓库根目录运行：

```bash
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

当前 `build` 包含 i18n 键完整性检查、`vue-tsc -b` 和 Vite 构建，因此同一未变化的候选无需再单独重复这些检查。用实际前端预览检查管理员和用户页面、明暗主题及窄屏耗时布局；保存用于证明显示结果的截图即可，第一版不要求浏览器矩阵或全 E2E。

### 8.2 同步、发布与部署验证

| 风险或合同 | 最小可靠证据 |
|---|---|
| 同步覆盖自定义提交 | 临时 Git 仓库模拟 personal 功能提交和上游更新，确认合并后两者保留且 main 不被重建 |
| 标签选错、重复运行、上游历史改写 | 固定标签/引用样例测试与一次实际候选运行，覆盖 klno 标签数字排序、已同步目标、冲突中止 |
| 机器人 PR 没有运行检查 | 实际 Actions run，确认指定候选 SHA 的检查产生结果；覆盖必要检查待批准情形 |
| 错误提交或重复发布 | 检查标签指向、镜像 OCI revision、digest 与发布结果；同一 SHA 重跑不能另发版本或改标签 |
| 发布流程适配失败 | 相关 helper 单测、shell 语法检查，以及一次 `simple_release=true` 的 dry run；不要求全平台 dry run |
| 部署脚本误改数据库/卷 | 对 SSH/Docker 的替身演练拉取失败、应用不健康、成功、显式回滚、迁移禁止自动回滚；核对命令仅更新应用服务 |
| 实际服务不可用 | 首次授权部署后核对容器、健康、版本、历史 TPS 与最小网关烟测 |

继承的 CI 在远端保留其实际要求的后端单元、集成和 lint 等检查，上游同步可能涉及后端变更，不能用 TPS 组件测试替代。仅 TPS 的本地开发验证不额外跑全部后端或全平台构建；新增同步差异按其实际影响补充检查。

同步和发布修改触及历史保留、权限及发布行为，部署脚本触及生产状态与回滚。取得候选实现后，应安排独立只读复核，具体检查强推是否移除、最终 SHA 的门禁、凭据边界、镜像一致性及迁移后的回滚限制；此要求在实施阶段执行，本次计划编制没有完成独立代码审查。

### 8.3 最终可观察验收

- 管理员与用户能在已有历史文本记录中看到相同口径的 TPS，原有耗时与计费信息保留。
- 同步 PR 清楚列出版本和 SHA，自定义 TPS 在同步后仍显示；冲突时现有分支及生产服务保持原状态。
- 自有镜像来源于已检查的固定 personal 提交，只包含所需平台，可由 hostdzire 拉取。
- 部署入口明确报告成功或失败，当前运行 digest 可追溯；相同基线的 TPS 发布可按已演练步骤回滚。
- 未来上游原生提供 TPS 时，可以通过撤除小组件与少量接入代码结束自定义维护。

## 9. 首次实施前的待核对项

这些项目影响实施与生产操作，不妨碍本方案成立；执行相应阶段前核对：

- hostdzire 是否仍运行原会话中的 revision，以及实际 Compose/数据库服务和挂载是否变化。
- GitHub fork 是否已启用 Actions，允许创建 PR，包权限和分支 Rulesets 的真实设置。
- 发布时是否沿用公开 GHCR；若改用私有镜像，补齐服务器读取权限。
- 首次上游目标是否仍采用本文固定基线；如决定先升级，独立列出迁移、配置和功能差异。
- 生产维护窗口、测试调用授权、备份位置与恢复能力。文档与本地脚本准备不等于授权执行生产部署。

## 10. 当前实施状态

P0.2/P0.6 的安全定义已通过普通推送在远端 main/personal 生效。P1 功能提交 `f74554702e6d55554342134b68fc54ff4a4ff541` 保留为 personal 祖先；应用来源仍是 `v0.2.14-klno.3` / `de08df02ae1d81668a22f798b398aa0438ac1276`，本轮未改变前端/后端应用及锁文件。

P2 的同步实现、21 个合同用例、workflow actionlint 和两次独立只读复核完成。P2 验收时的 personal 为 `029cd8fb8b04d18a6b3abe4528effe4aeec75859`，机器人演练 PR #2 候选为 `9c33a062c8ed50ea77ca17a05b789d88c94f54bb`；显式、原生候选与 personal push 的完整 CI 均通过，PR 必要 personal-ready=pass。定时变量已设为 true 并读回，每日 UTC 03:17 定义 active，首个真实 schedule 尚未发生。错误 SHA 的实际 dispatch 已被拒绝，旧候选即使全部测试通过也因基础变化被最终门禁拒绝。已配置严格 personal 分支保护及 merge-only 设置。

验收期间 KlN 发布 v0.2.14-klno.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d；该目标不是当前来源 de08 的后代，实际流程在远端候选写入前停止。只读普通合并预览有 120 个冲突文件，164 个文件的净差异及改写历史待人工审查。P2.9 的真实升级候选及 P2.11 保持未完成；演练不算升级，也不会合入。P2 的证据及剩余条件见 implementation-evidence.md 第 6 节。

P3.1–P3.9 已完成。[PR #3](https://github.com/ccisnoxx/sub2api/pull/3) 普通 merge 后的最终 personal 为 `896de21b4be7f4ec4b4236f4df663b47371665b0`；该 SHA 的全新完整 CI、simple dry run 和独立复核通过后启用发布入口。机器人从来源记录分配 `v0.2.14-klno.3-tps.1` 并显式 dispatch simple Release，构建、发布、OCI revision/version/linux/amd64 校验、公开匿名完整拉取及同 SHA 不重复发布均有实际证据。P3 的发布结果只回写 main，未改变已发布 personal 的源码；main VERSION 未写回，P3 阶段没有访问 hostdzire。

P4 固定部署输入为 `ghcr.io/ccisnoxx/sub2api@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`。P4 工具属于 main 部署控制线，已通过 Python 3.11/Linux 的 28 个合同用例及两次 fresh 独立只读复核。真实隔离栈完成旧镜像→tps.1→旧镜像，生产仅切换一次应用服务；正式部署记录 `20261008T190259Z-2bb5bc1b6d52` 为 success，实际 digest、版本、revision、健康、数据库/Redis 和迁移状态均核对通过。配置/数据备份及旧镜像保留，生产没有回滚或恢复数据库。

P4.1–P4.10 完成。管理员和我的账户使用记录页面的新/历史 TPS、首字与总耗时已实际验收，原始毫秒值与页面计算一致；两个页面复用同一管理员会话，独立普通用户身份未验证。首次自写 HTTP 烟测的 403 已确认来自本地官方客户端限制检测，原因是缺少引擎指纹，并非已确认的上游拒绝。用户要求继续解决 P4.9 后，使用真实 Codex CLI 0.159.0 执行一次关闭工具调用、禁止重试的 HTTP 文本烟测，HTTP 200、完整响应 OK、唯一新增使用记录及两个页面展示均通过：5 output / 1675 ms = 2.99 tok/s，首字 1607 ms。本次没有再次部署、修改客户端限制或改变固定版本。P2.9 真实升级部分与 P2.11 保持未完成；发布证据见第 7 节，部署及验收边界见 implementation-evidence.md 第 8 节。


## 11. P5：TPS 前端展示对齐（2026-10-08）

本轮从 personal 应用维护分支建立 `codex/tps-display-alignment`，以 Plus 固定提交 `90da415c62b94c9417d9ce2b72b1507ed22f0303` 的 UsageTable、usageTiming 和中文 dashboard 为参考。只对齐展示，继续采用 `output_tokens × 1000 ÷ duration_ms`；不扣除 first_token_ms，保持原可用范围、原因判断、首字/总耗时及左侧耗时色条合同。

- 第三行统一为 TPS；有效数值为 `text-cyan-600 / dark:text-cyan-400`，不可用为灰色 `-`，青色不表示速度等级。
- 小于 0.1：两位有效数字；0.1 至小于 100：一位小数并去掉末尾 .0；100 起：整数。保留 tok/s，例如 42.19 → 42.2、7.00 → 7、150.45 → 150。低速正值不改成 0 或固定阈值占位。
- TPS 标签和数值使用同一口径提示；唯一常驻按钮位于首字旁，复用 HelpTooltip 和原生按钮。说明只包括公式、首 token 等待、可能的推理 token及现有不可用原因；为关闭按钮保留文字间距。
- 不引入 Plus 的严格首字判断、first_output_ms、last_token_ms、timing_version、is_complete、后端、迁移或计费变更；前端展示对齐不代表已具备 Plus 完整计时能力。
- 本地采用定向组件/表格/提示测试、改动文件 lint、i18n/类型/构建及实际双页面、明暗主题、窄屏和提示交互验证。远端保留完整 Personal CI、personal-ready 与 Release 门禁。
- 应用通过保护规则 PR 合入 personal，既有流程分配下一 tps.N；不移动/覆盖标签及版本镜像。部署继续由 main 的 P4 工具完成兼容性与备份检查，以固定 digest 仅切换应用。

实际结果与发布部署状态以 tasks.md 的 P5 和 implementation-evidence.md 第 9 节为准。阶段结果回写 main，不合入 main 的应用树，不反复改变已发布 personal SHA；P2.9/P2.11 的 .5 历史升级保留为独立事项。

### P5 安全门禁所需的授权补丁

原前端范围完成后，动态 govulncheck 报告既有 Go1.27.0标准库漏洞，升级1.27.2后又确认x/net五项可达漏洞。用户两次明确授权最小安全升级继续上线；因此增加独立工具链与依赖提交，业务源码、数据库、计费和TPS计算不改动。最低解析为Go1.27.2及x/crypto0.57、x/net0.60、x/sys0.48、x/term0.46、x/text0.42、x/tools0.50、x/mod0.41、x/sync0.23。不得绕过扫描或门禁。

P4对安全补丁仅接受审定前后backend/go.mod、backend/go.sum、两份Dockerfile的固定完整字节hash及普通文件模式；四路径必须全部同方向变化，其他运行差异仍拒绝，新增proof经fresh独立复核后使用。反向故障恢复仍须满足记录、配置、迁移、依赖条件；旧Go有已知漏洞，不应长期停留旧镜像。

最新门禁补充：x/net0.60 和 Go1.27.2 使既有 HTTP/2 接口产生 SA1019 弃用告警。Git 外已准备三个按固定文件与明确接口匹配的 lint 兼容规则，五个文件、16个原始报告行（另一个同一行 Server 告警原被去重），配置校验及三个受影响包的 staticcheck 为0 issues。尚未修改维护的 backend/.golangci.yml；新增该文件与 P4 字节清单超出此前 go.mod/go.sum 授权，须先取得范围决定、独立复核及最终完整门禁。
