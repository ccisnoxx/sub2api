# P0/P1/P2 实施证据

- 实施日期：2026-10-08（America/Los_Angeles）。
- 前轮范围：本地 P0 基线准备、P1 平均输出 TPS，以及 hostdzire 只读检查。GitHub 设置修改、远端推送、镜像发布、生产部署均未执行。
- 前轮结论：P1 本地完成；P0 本地准备完成，P0.2/P0.6 的远端定义替换当时仍待执行。不能把本地暂停文件认定为远端已生效。
- 功能代码 SHA：`f74554702e6d55554342134b68fc54ff4a4ff541`，由 `codex/usage-tps` 快进合入本地 `personal`。此后的证据提交只修改文档，复用相同源码的有效验证结果。
- 前轮外部产物：未创建 PR、个人发布标签或镜像；运行镜像仍为 KlN 原版本。后续 P2 结果见第 6 节。

## 1. 开始状态与工作保护

本轮开始于 `codex/usage-tps`，HEAD `76ad79b4aaa51b7d65467d2550da0e8b54b556c7`，工作树干净。`personal` 指向相同提交，只有当前一个 worktree，没有未跟踪的计划或无关改动。方案中“main、尚未建立 personal”的状态属于编制时快照，未据此重建现有分支。

此前准备已经保存：

| 本地提交 | 已有内容 |
|---|---|
| main 的 `998e5a312ff5668dbb00a1efd7d5bc487e9d5843` | 计划、任务清单、初始证据及暂停同步定义 |
| personal 的 `1ef44b17b2476ffddb6bda5295a081b1ca25ed2b` | 从 KlN 标签继承同一组维护文件；没有合并整个 main 的应用源码 |
| `76ad79b4aaa51b7d65467d2550da0e8b54b556c7` | `deploy/personal-source.json` |

读取了 plan.md、tasks.md、已有 implementation-evidence.md、仓库根 AGENTS.md 和 `/Users/sc/.codex/AGENTS.md`，并遵守用户给出的全局规则。仓库根 AGENTS.md 是当前存在的本地文件，旧证据中的“仓库无额外 AGENTS.md”已纠正。

根 AGENTS.md 引用的 `docs/conventions/codex-outbound-identity.md`、`docs/dev-journal.md`、`docs/tasks/` 在当前所选基线中缺失；已检查维护树、发布标签的 docs 文件清单及相应路径的 Git 历史，没有把其他项目文件当作替代约定。本轮仅改前端展示，不改出站身份或账号 extra；按非平凡改动回写要求新建 `docs/dev-journal.md`，与证据一起保存。未改写或补造缺失的出站身份约定。

## 2. P0 基线与同步安全状态

### Git 来源

- origin：`https://github.com/ccisnoxx/sub2api.git`。
- upstream：`https://github.com/KlN-4096/sub2api.git`。
- 本地 `v0.2.14-klno.3^{commit}`、`upstream/klno` 和本轮 `git ls-remote upstream 'refs/tags/v0.2.14-klno.3*'` 的解引用均为 `de08df02ae1d81668a22f798b398aa0438ac1276`；标签对象为 `e14d8e4f9ad34dbd9c10918f5408a47500360362`。
- `git merge-base --is-ancestor v0.2.14-klno.3 personal` 成功。复用现有 personal 与功能分支，没有 reset、clean、rebase 或强推。
- 方案编制时 main 与发布标签差异为 528 个文件；开始时 main 已追加四个维护文件，当前差异计数为 532 个文件。应用来源仍是 KlN 标签，不能把 main 的 VERSION 当成应用基线。

来源记录的唯一字段合同：

| 字段 | 含义 | 当前值 |
|---|---|---|
| `upstream_repository` | 功能上游仓库的 owner/repo | `KlN-4096/sub2api` |
| `upstream_tag` | 已合入的 KlN 发布标签；个人发布版本不写在此处 | `v0.2.14-klno.3` |
| `upstream_sha` | 该发布标签解引用的完整源码提交，也是本轮应用祖先 | `de08df02ae1d81668a22f798b398aa0438ac1276` |

P2 同步与 P3 发布尚未实现，后续必须共同读取此文件；本轮没有声称已有消费者、个人版本或部署选择文件。

### hostdzire 只读结果

此前初始证据的 SSH 超时在本轮已恢复。使用现有 `hostdzire` 别名、BatchMode 和连接超时执行只读命令；没有修改 SSH 配置、认证、Compose、容器、数据库或 Redis。

| 项目 | 本轮观测 |
|---|---|
| 架构 / 镜像平台 | `x86_64` / `linux/amd64` |
| Compose | `Docker Compose version v5.1.3` |
| 目录 / 文件 / 项目 | `/root/sub2api-kin` / `/root/sub2api-kin/docker-compose.yml` / `sub2api-kin` |
| 应用服务 / 容器 | `sub2api` / `sub2api-kin` |
| 运行镜像 | `ghcr.io/kln-4096/sub2api:0.2.14-klno.3` |
| RepoDigest | `ghcr.io/kln-4096/sub2api@sha256:c0ec609deaf0fb6f323de660ed7d43cf2030b4c4083c6fc4bef506d28f08ad8c` |
| OCI revision | `de08df02ae1d81668a22f798b398aa0438ac1276`，与固定标签一致 |
| OCI version | `0.2.14-klno.3` |
| 应用启动时间 | `2026-10-08T01:06:33.196657696Z`；开始与结束核对一致 |
| 端口 | `127.0.0.1:10088` → 容器 `8080/tcp` |
| 应用数据 | 命名卷 `sub2api-kin_sub2api_data` → `/app/data` |
| 数据库 | 服务 `postgres`，容器 `sub2api-kin-postgres`，`postgres:18-alpine`；命名卷 `sub2api-kin_postgres_data` → `/var/lib/postgresql/data`，运行时另有 `/var/lib/postgresql` 匿名挂载 |
| Redis | 服务 `redis`，容器 `sub2api-kin-redis`，`redis:8-alpine`；命名卷 `sub2api-kin_redis_data` → `/data` |
| 状态 | 应用、Postgres、Redis 均 running / healthy；应用依赖两项 service_healthy |
| `/health` | 容器内检查以及主机 `http://127.0.0.1:10088/health` 返回 `{"status":"ok"}` |

首次按旧假定访问主机 8080 得到连接失败；随后检查实际端口映射，使用 10088 验证通过。这是检查入口假定错误，没有调整服务端口。Compose 只输出上述 image、name、mount、port、depends_on 投影，没有输出环境变量或完整配置。

### GitHub 与工作流边界

本轮只读执行 `gh api` 查询仓库 Actions 权限、workflow/run 列表及远端 main 的同步文件：

- 仓库级 Actions `enabled=true`，`allowed_actions=all`。
- workflow 列表为空，run `total_count=0`，没有观测到已注册或正在执行的旧同步任务。
- 远端 main 的文件仍含 schedule、rebase、`git push --force-with-lease`、自动标签、Release dispatch 和重建 main。
- 本地 main/personal 的暂停定义相同，仅 `workflow_dispatch` 和 `contents: read`，手动输出暂停说明，不执行上述破坏性动作。
- 两条本地分支中的维护文件已保存；本轮不推送、不修改 GitHub 设置。远端“目前无运行记录”不能等同“安全定义已替换”。P0.2/P0.6 因此保持未勾选。

## 3. P1 实现与审查范围

`f74554702e6d55554342134b68fc54ff4a4ff541` 包含 8 个前端维护文件，277 行增加、9 行删除：

- UsageTps.vue：按 `output_tokens × 1000 ÷ duration_ms` 计算，首字不参与；两位小数与 `<0.01 tok/s`；无效输出、无效耗时、媒体/未知计费模式、原生 compaction、live、probe、gwpool_degraded 返回 `—` 并解释原因。
- 图片 token 计费仍检查图片输出与生成数量；仅图片输入且文本输出可用。历史缺少首字、计费模式或请求类型时沿用已读工具合同。
- UsageTable 的真实 `cell-latency` 增加第三行，保留首字、总耗时和健康度色条；管理员和用户均使用同一组件。
- 中文、英文增加对应 TPS 键和统计口径说明。说明包含首 token 等待和可能的推理 token，不能据此单独证明 Fast 或可见文字纯生成速度。
- 原生 button 复用 HelpTooltip 的 click 交互，可由键盘打开并关闭，无新依赖。
- 窄屏验收暴露既有 HelpTooltip 的 fixed 坐标叠加 scrollX/scrollY，以及右侧溢出。在原定位 owner 去掉滚动量、约束水平中心与顶部边距、限制视口宽度；没有另建提示系统。新增回归用例在修复前因坐标错误失败，修复后通过，原 hover/click/外部关闭测试仍通过。

自查实际 staged diff、合入后的代码差异及工作树：无后端、迁移、接口、计费、服务档位、CSV、排序或锁文件改动；无无关格式化、生成文件、凭据或生产数据。未进行独立只读审查；P1 是前端显示变更，不以自查充当独立审查。P2/P3/P4 的候选仍需文档要求的独立复核。

## 4. 实际验证

环境：macOS、本机 Node `v24.16.0`；pnpm `9.15.9` 与发布工作流 major 9 一致；Vitest `2.1.9`、Vue `3.5.43`、Vite `5.4.21`。验证发生在未提交候选，其 8 个文件随后原样保存为上述功能 SHA；合入 personal 为快进，没有改变源码。

本机默认 pnpm `11.25.0` 安装时忽略 package.json 的 overrides，frozen install 报 `ERR_PNPM_LOCKFILE_CONFIG_MISMATCH`。切换至 pnpm 9 后 `--frozen-lockfile` 成功；build 的嵌套 pnpm 调用也通过任务专用 PATH 固定到 9。没有使用 `--no-frozen-lockfile` 或更改 package.json/lockfile。

```bash
corepack pnpm@9.15.9 --dir frontend install --frozen-lockfile
# 本轮 /tmp/sub2api-tps-tools/pnpm 指向 Corepack 缓存的 pnpm 9.15.9。
export PATH=/tmp/sub2api-tps-tools:$PATH
pnpm --dir frontend exec vitest run \
  src/components/admin/usage/__tests__/UsageTps.spec.ts \
  src/components/admin/usage/__tests__/UsageTable.spec.ts \
  src/components/common/__tests__/HelpTooltip.spec.ts
pnpm --dir frontend exec eslint \
  src/components/admin/usage/UsageTps.vue \
  src/components/admin/usage/UsageTable.vue \
  src/components/admin/usage/__tests__/UsageTps.spec.ts \
  src/components/admin/usage/__tests__/UsageTable.spec.ts \
  src/components/common/HelpTooltip.vue \
  src/components/common/__tests__/HelpTooltip.spec.ts \
  src/i18n/locales/zh/dashboard.ts src/i18n/locales/en/dashboard.ts
pnpm --dir frontend run build
git diff --check
```

| 验证 | 结果 |
|---|---|
| UsageTps | 37 个用例通过，覆盖公式、sync/stream/WS、历史首字变化、输出/耗时缺失及非有限/非正值、媒体和未知模式、图片输入、低速边界、双语原因及提示 |
| UsageTable | 41 个用例通过；新插槽断言真实渲染 TPS，账号计费显示开/关均保留原耗时和色条，其余 KlN 展示回归通过 |
| HelpTooltip | 5 个用例通过，含 2 个视口坐标回归及已有交互；本轮合计 83 个定向用例通过 |
| 改动文件 eslint | 8 个文件通过，无错误或警告 |
| frontend build | 通过；内含 3 个 i18n 键完整性用例、Vue 类型检查、Vite 构建；最终 Vite `1081 modules transformed`、`built in 10.56s` |
| diff 检查 | 通过；功能分支与合入 personal 的 frontend 差异为空 |

首次组件测试因为 Vitest 的 i18n 运行时不含字符串编译器而返回键名；测试改用真实文案的消息函数。Teleport stub 在交互后需要重新查询 DOM，旧 DOMWrapper 的可见状态不能作为关闭后的状态；修正断言后通过。真实键盘交互由下述浏览器验证直接覆盖。

构建输出 Browserslist 数据陈旧、Node 子进程 shell deprecation、动态/静态混合导入与大 chunk 提示，退出码为 0；没有为这些构建提示改动依赖或分包，也未额外构建旧基线来比较告警集合。本轮未运行后端全套、全前端测试或浏览器矩阵；显示行为及共享提示定位由相应定向测试和页面验收覆盖。

### 本地管理员/用户页面验收

使用 `http://127.0.0.1:4173` 的实际构建预览，检查 `/admin/usage` 和 `/usage`，由本机 Playwright `1.62.1` 驱动已安装 Chromium headless shell revision `1228`。Browser 插件在本会话不可用，记录为 `Browser plugin not available`；没有新增浏览器依赖。默认 Playwright 查找 revision 1234 失败后明确选择已安装可执行文件。

API 全部拦截为合成记录，登录身份为本地 QA fixture；初始化状态、版本、空图表/筛选辅助数据也返回明确 fixture。无关支付 SDK 静态脚本在浏览器路由中返回空脚本，不验证支付功能。没有使用生产认证、读取真实使用明细、向服务器写入记录或发起付费网关请求。

| 页面/语言/主题 | 视口 | 结果 |
|---|---|---|
| 管理员 / 中文 / 明亮 | 1440 × 1100 | 通过 |
| 用户 / 英文 / 深色 | 1440 × 1100 | 通过 |
| 管理员 / 中文 / 深色 | 390 × 844 | 通过 |
| 用户 / 中文 / 明亮 | 390 × 844 | 通过 |

每组实际渲染并核对 11 类合成记录：历史文本、图片输入、无效耗时、token 计费图片输出、视频、低速、compaction、live、probe、gwpool_degraded、零输出。页面身份/标题正确、内容非空、无框架 overlay、无 console error/warning。焦点按钮通过 Enter/Space 打开提示、Escape 关闭；缺失耗时与媒体提示显示对应原因；提示四边位于视口内，原有耗时/色条和窄屏卡片可见。截图已人工查看。

本地脚本、results.json 和截图保存在：

`/Users/sc/.codex/visualizations/2026/10/08/01a11bf9-6de4-7391-a1e7-a440d2ff0b45/usage-tps/`

运行命令：

```bash
pnpm --dir frontend exec vite preview --host 127.0.0.1 --port 4173 --strictPort
node /Users/sc/.codex/visualizations/2026/10/08/01a11bf9-6de4-7391-a1e7-a440d2ff0b45/usage-tps/qa.cjs
```

代表性截图：

- [管理员中文桌面明亮](</Users/sc/.codex/visualizations/2026/10/08/01a11bf9-6de4-7391-a1e7-a440d2ff0b45/usage-tps/admin-zh-light-1440-rows.png>)
- [用户英文桌面深色](</Users/sc/.codex/visualizations/2026/10/08/01a11bf9-6de4-7391-a1e7-a440d2ff0b45/usage-tps/user-en-dark-1440-rows.png>)
- [管理员中文窄屏深色提示](</Users/sc/.codex/visualizations/2026/10/08/01a11bf9-6de4-7391-a1e7-a440d2ff0b45/usage-tps/admin-zh-dark-390-tooltip.png>)
- [用户中文窄屏明亮提示](</Users/sc/.codex/visualizations/2026/10/08/01a11bf9-6de4-7391-a1e7-a440d2ff0b45/usage-tps/user-zh-light-390-tooltip.png>)

截图/脚本属于本机证据，未作为生成资源提交到仓库；生产页面验收属于 P4，未执行。

## 5. 任务状态与进入 P2 的条件

- 完成并勾选：P0.1、P0.3–P0.5、P0.7–P0.9；P1.1–P1.11。
- 部分完成并保持未勾选：P0.2、P0.6。维护文件在两条本地分支已准备好，远端 main 尚未推送安全定义，也没有修改远端设置。
- 待执行：P2 全部（包括首次 P4 后的 P2.11）、P3 全部、P4 全部。
- 进入 P2 的本地设计/实现已具备固定 KlN 来源及已验收 TPS；启用实际远端流程前，需要后续阶段的推送/设置授权，先让 main/personal 的安全定义在远端生效，读取实际 Rulesets/分支保护与工作流写入权限。
- P2 候选需实现普通 merge、版本解析、无更新/重跑/冲突停止及指定候选 SHA 的显式 CI，并完成临时仓库演练、一次真实候选和独立只读复核，之后再启用定时同步。首次 TPS 发布仍保持当前 KlN 基线。
- 当前类型没有音频输出计数、计时版本、首输出类型或完整性字段，不能声称排除了所有无法识别的混合音频记录或确认记录完整。真实生产数据及新镜像体验留在 P4；本轮健康检查不代替网关验收。


## 6. P0 远端生效与 P2 同步流程（2026-10-08）

本轮得到普通推送、PR、相关 Actions 和必要仓库设置授权，范围止于同步；没有创建发布标签、发布镜像、SSH 或修改生产。本地起点为 personal `f12e870b7ede984aa7ada9ded767b766bfa575b1`，工作树干净；远端只有 main `09d17249b2e80fbce6b7cfc052f7815559fe815d`。先完整读取计划、任务、前轮证据及日志，再重新查询远端状态，没有依据旧计划快照重建分支。

### 实现、权限与历史

- 先以普通推送让 main `3ca17211a` 和 personal `f12e870b7` 的暂停定义生效；旧 rebase、强推、KlN 标签/Release dispatch 和重建 main 已删除。Actions 原来已 enabled，推送后注册工作流；无保护、rulesets/rules 列表为空。
- 本机已有凭据具备 repo/workflow，无新增令牌。仓库默认 workflow 权限保持 read，只启用 Actions 创建 PR 的设置。同步 job 仅授予 Contents/PR/Actions write；候选 CI 只读且 checkout 不持久保存 Git 凭据。GITHUB_TOKEN 的 job 权限列表没有 Workflows；未来实际工作流文件推送若受限，明确停止，由已有授权本机凭据处理，不能丢弃上游差异。
- 初始实现 `ca847c875`，解析/历史/门禁修正 `67cb02daae48cc331cd22776ebdeeb1584b0ce8b`，CI 来源祖先修正 `773594445e9397ab118350ce67cd11749084295e`。固定来源仍为 `v0.2.14-klno.3` / `de08df02ae1d81668a22f798b398aa0438ac1276`；TPS 提交 `f74554702e6d55554342134b68fc54ff4a4ff541` 保留为祖先，前端/锁文件/后端应用代码未修改。
- 维护逻辑位于 `.github/personal-sync/`；来源字段不改名。标签按四段数字排序，独立 refs/personal-sync/tags 引用；显式未发布 SHA 记录 upstream_tag=null，后续发布必须明确版本归属。候选普通 merge，记录基础/来源/候选 SHA、迁移与 PR；关闭 PR、不一致树、缺失上游祖先、冲突、改写历史、控制文件差异、基础变化和权限失败均停止。两条维护分支不被脚本推送。
- Personal CI 显式输入/ref/实际 checkout 对应完整候选 SHA，开始和结束核对 PR、当前 personal 与来源祖先；聚合 TPS 和原 CI/Security Scan 的全部检查。依赖为 pnpm 9、frozen lockfile。相同 SHA/基础的已运行检查可复用；personal 前进必须重新准备候选。
- main 搬运维护目录时曾误提交 Python 缓存，后续普通提交 `680bc0fe0` 删除并添加目录级忽略，personal 对应 `e0dca83a4`；最终树不含缓存。没有重写已推送历史。

### 实際验证与失败分类

- `python3 -m unittest discover -s .github/personal-sync -p 'test_*.py' -v`：21 个合同用例通过。真实临时 Git 仓库覆盖 TPS/上游/main 历史、数字排序、annotated tag/独立引用、无更新、重跑同 SHA、同树 squash、冲突 abort、历史改写、基础变化、推送拒绝、控制/必要 CI 文件变化和未发布 SHA 记录；PR/dispatch 边界使用明确替身。
- CI 来源历史反例先在修正前因 `SyncError not raised` 失败，复用 source_at() 后通过；不是仅断言配置值。新增标签 resolver 合同使用真实本地 remote，验证当前 klno 改写后合法发布标签仍能解析，fork 标签不被覆盖。
- 官方 actionlint `1.7.12`，下载 SHA-256 与 GitHub release asset digest 一致；四份变动工作流检查通过。差异检查通过。P1 应用输入未变化，复用前轮 83 个组件/表格/提示测试、lint、build 和界面验收；实际候选中仍会运行 TPS 定向检查。
- [首轮同步失败](https://github.com/ccisnoxx/sub2api/actions/runs/37802495138) 是发布标签被当前 klno 祖先限制误拒。重新 fetch 发现 klno 从 de08 改写为 c52b2e3d4ecf89404dd9ec61b0356484e79358bd；与已发布标签有 162 文件差异，共同祖先 cdd6e447661349b09316fb75d08a8a096c4a8708。修正以仓库的发布标签为固定来源；未来非来源后代的目标仍停止供人工审查。
- [自动选择无更新](https://github.com/ccisnoxx/sub2api/actions/runs/37802893857) 和 [显式标签无更新](https://github.com/ccisnoxx/sub2api/actions/runs/37803320223) 成功，输出固定标签/SHA、base 67cb02daa，不生成升级 PR 或额外 CI。
- [机器人候选创建](https://github.com/ccisnoxx/sub2api/actions/runs/37802900732) 成功产生 [草稿 PR #1](https://github.com/ccisnoxx/sub2api/pull/1)，候选 `ad84de3780c8e0bea882049de7be4a601602aee2`，base `67cb02daae48cc331cd22776ebdeeb1584b0ce8b`。作者为 GitHub Actions，仅增加 candidate.json，来源和 TPS 不变，标题明确“无上游升级，请勿合并”。
- [显式 Personal CI](https://github.com/ccisnoxx/sub2api/actions/runs/37802988804) 的 event=workflow_dispatch、head_sha=候选、run title 同时含候选/基础 SHA；check-runs 全部归属候选，App ID 15368。机器人原生 PR run [37802990043](https://github.com/ccisnoxx/sub2api/actions/runs/37802990043) 曾 action_required，已通过 API 批准，既有必要 CI 由 reusable 调用完整运行。此记录编写时后端测试仍运行，尚不把整体 CI 记为通过。
- [同基础重跑](https://github.com/ccisnoxx/sub2api/actions/runs/37803749271) 复用同一 PR/候选和已有 CI，没有再次 dispatch。[非法输入](https://github.com/ccisnoxx/sub2api/actions/runs/37803333056) 在任何候选写入前明确失败。连续排队的演练 37803326195 被 GitHub concurrency 单 pending 语义取消，未执行，随后单独派发验证；不是成功结果。旧 SHA 的过时 CI 已取消，不复用旧基础结果。

### 独立复核与剩余验收

两位 fresh critical_reviewer 只读审查，确认并关闭：同树候选丢失上游历史、必要 reusable gate 未保护、合法发布标签误拒，以及在 CI 边界补齐人工修改候选后的来源祖先校验。最终复核未确认新的阻断。审计 Bundle `20261008T154650Z-personal-sync-p2-final-2fcdbc65` 聚合 2 次执行/2 次验收/2 次独立复核，关闭及 verify 通过，未观测仓库写入。

严格 personal-ready/App 15368/最新基础保护及定时入口仍待首轮完整 CI 后生效，随后需要按最终 personal SHA 刷新演练并验证最终门禁。当前没有新 KlN 发布，因此真实升级候选未产生；P2.9 真实升级部分保持未勾选，P2.11 依赖 P4，P3/P4 未开始。演练 PR 不能合并或作为实际升级成功。

阶段进度与最终远端验收以默认 main 的最新本文件、tasks.md 和开发日志为准；此阶段提交保存可复用固定源码，后续仅 main 的结果登记不改变 personal 基础或旧检查输入。
