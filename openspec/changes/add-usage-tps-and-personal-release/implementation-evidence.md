# P0–P4 实施证据

- 实施日期：2026-10-08（America/Los_Angeles）。
- 最新阶段：P3.1–P3.9 已完成；本轮正在实施 P4.1–P4.10。下文第 1–7 节是各阶段当时的记录，当前状态以第 8 节和 tasks.md 为准。P2.9/P2.11 的 .5 升级不属于本轮。
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

## 5. 前轮任务状态与当时进入 P2 的条件

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

### 实际验证与失败分类

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

### 最终基础与远端门禁验收

- 固定 personal `029cd8fb8b04d18a6b3abe4528effe4aeec75859`；main 的相同控制定义为 `2ad9c5be246844024afd67354931283d30e693e5`。两条分支的同步目录、四份工作流和维护约定完全一致。最终状态只回写 main，保持 personal 的检查输入不变。
- [最终机器人演练](https://github.com/ccisnoxx/sub2api/actions/runs/37804458549) 创建 [草稿 PR #2](https://github.com/ccisnoxx/sub2api/pull/2)，候选 `9c33a062c8ed50ea77ca17a05b789d88c94f54bb`、基础为上述 personal。实际 fetch 核对仅增加 candidate.json，来源记录及 TPS 祖先不变；没有构造真实上游升级。
- [显式最终候选 CI](https://github.com/ccisnoxx/sub2api/actions/runs/37804514128) 和 [personal push CI](https://github.com/ccisnoxx/sub2api/actions/runs/37804451356) 全部 success：绑定、TPS/构建、21 个同步合同、既有 shell/Go 单元/集成/race/lint/前端关键检查/发布 helper、安全扫描及最终 personal-ready 均通过。
- [原生候选 CI](https://github.com/ccisnoxx/sub2api/actions/runs/37804516668) attempt 1 为 action_required；批准后的 attempt 2 中，既有 `TestPinnedOpenAIModelsListMixedAccountsShareColdCacheAcrossGroups` 在 openai_models_list_test.go:291 失败，API 调用次数期望 1、实际 2。核对维护源码发现 get(miss) 与 singleflight.DoChan 之间存在时间窗口，前一调用已完成时 refresh 不重查 fresh，可能重复 fetch；涉及代码/测试与 de08 基线完全相同，本轮未改动。相同 SHA 的显式/个人完整测试通过，支持按已定位的并发非确定性进行一次 `--failed` 重跑；attempt 3 只运行失败 backend test job 和最终门禁，其他成功结果复用，最终全部 success。`gh pr checks 2 --required` 实际返回 personal-ready=pass，对应 [最终原生门禁](https://github.com/ccisnoxx/sub2api/actions/runs/37804516668/job/113419994583)。没有跳过测试、降低断言或改动上游应用来取得成功；既有并发窗口仍是应用侧后续事项。
- [错误 SHA 实际 dispatch](https://github.com/ccisnoxx/sub2api/actions/runs/37804800869) 按预期 failure：ref 为测试提交 `7ce3b3a3196e97063066f941a71bb5a033f164fa`，输入 candidate/base 为 personal 029cd8；binding 输出“dispatch ref 的 SHA 与候选 SHA 不一致”，全部后续必要检查 skipped，personal-ready 失败。该负例没有更改正式候选。
- [最终基础的无更新检查](https://github.com/ccisnoxx/sub2api/actions/runs/37805655832) success，输出 state=no_update、base=029cd8fb、upstream_tag=v0.2.14-klno.3、upstream_sha=de08df02；没有新升级 PR、来源变动或额外候选 CI。
- [旧基础完整 CI](https://github.com/ccisnoxx/sub2api/actions/runs/37802988804) 的所有测试和安全检查通过，但结束时 personal-ready 输出“personal 基础分支已变化”并 failure，验证测试期间基础前进后旧证据不能使用。原生旧基础 run 37802990043 同样 failure；PR #1 经确认过时后已关闭，未合并。
- 实际 readback：personal 必须通过 `personal-ready`，App ID=15368，strict=true，enforce_admins=true；allow_force_pushes/allow_deletions=false，required_linear_history=false，允许普通 merge。此前保护为空，本轮新增门禁，没有削弱已有规则；Rulesets 仍为空。仓库 allow_merge_commit=true，allow_squash_merge/allow_rebase_merge=false。
- Actions enabled=true；默认 token 权限保持 read，允许机器人创建 PR。每日 UTC 03:17 定义已推送两条分支并 active。最终完整 CI、PR 实际必要门禁、独立复核和严格保护通过后，于 `2026-10-08T16:27:08Z` 设置并读回仓库变量 `PERSONAL_SYNC_SCHEDULE_ENABLED=true`；尚未观测首个真实 schedule 事件，不把手动 dispatch 当成定时触发证据。
- [启用后的正常手动检查](https://github.com/ccisnoxx/sub2api/actions/runs/37808889397) 运行于 main 6f2bb91、verify_ci=false，按预期因新 .5 非来源后代而 failure，在远端候选写入前停止。PR #2 的正文已登记实际候选/基础和完整成功检查，保持草稿及“请勿合并”；旧 PR #1 已关闭，真实升级 PR 未创建。

### 验收期间的新发布与真实历史停止

UTC 16:02 的无更新结果是真实当时状态。KlN 随后于 `2026-10-08T16:10:39Z` 发布 [v0.2.14-klno.5](https://github.com/KlN-4096/sub2api/releases/tag/v0.2.14-klno.5)，解引用为 `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。[再次自动解析](https://github.com/ccisnoxx/sub2api/actions/runs/37807041368) 在任何候选远端写入前停止，输出“上游目标不是记录来源的后代”。本地仅 fetch 到独立引用，确认 de08 不是新目标祖先，共同祖先为 cdd6e447661349b09316fb75d08a8a096c4a8708；两标签净差异 164 文件、6821 行增加/4161 行删除。workflow、迁移和 UsageTable 相关路径在这次净差异中为空；这不代替完整历史/功能审查。

进一步只读运行 `git merge-tree --write-tree personal c7aacf5d3ae383d0d5c75f471f66e61690a5701d`，退出 1：普通合并产生 120 个冲突文件（24 content、96 add/add），涉及 gateway/gwpool、账号、前端和测试 runner，包含 UsageTable.spec.ts。输出保存在本机 `/tmp/sub2api-klno5-merge-preview.txt`，工作树仍干净；没有选择 ours/theirs、修改源码或创建远端升级候选。可见净差异为空也不代表历史重写后的三方合并没有冲突。该升级需另行完整审查、逐项解决冲突与重新验证。

真实升级候选因历史改写停止，人工差异审查及 P2.9 真实升级部分未完成；没有绕过祖先检查、制造发布或合入演练。P2.11 依赖 P4，P3/P4 未开始。实际机器人修改 workflow 文件的 push 尚未验证；Workflows 权限受限时必须明确停止并用现有授权本机凭据处理，未新增令牌。首次 P3 继续使用原 KlN 基线加 TPS 的已检查 personal。

本轮完成 P0.2/P0.6、P2.1–P2.8、P2.10；P2.9 仅勾选已验证子项。结果登记提交 `6f2bb91ded62c1d76947cefcbb2c1c47d05db435` 已普通推送 main，后续最终登记同样只修改 main 文档；personal 029cd8fb 及其有效检查输入保持不变。下一阶段先实现 P3 的来源/标签/发布门禁及 simple dry run，再按授权发布首次原基线加 TPS 镜像；当前没有个人发布标签、镜像或生产变更。

阶段进度与最终远端验收以默认 main 的最新本文件、tasks.md 和开发日志为准；此阶段提交保存可复用固定源码，后续仅 main 的结果登记不改变 personal 基础或旧检查输入。

## 7. P3 个人镜像发布（2026-10-08）

本轮范围为建立发布流程并实际发布原 KlN 基线 + TPS 镜像。未处理 .5 历史升级、未连接 hostdzire、未修改生产。

- 从 personal 029cd8fb 创建 codex/personal-release，读取 main bd8de131 的最新阶段记录；应用、来源记录、锁文件、迁移保持原基线。发布实现提交 8663821be，可信工具传递修正 ac095448e，最终推送/恢复修正 f439c543dda328f6a9ac385e5c71cbbdd62f85e5。main 对应可信控制提交 b7794d6f2/9a358653a，没有合并 main 应用源码。
- 复用既有前端单次构建、GoReleaser linux/amd64 archive、完整 SHA/版本/目标来源与校验和验证。自动入口只接受本 fork personal 的成功完整 CI；准备和 Release 双重核验来源、最终 SHA、实际 personal-ready App 15368 和必要检查。标签数字分配、同 SHA 复用、冲突停止；GITHUB_TOKEN 创建标签后显式 dispatch Release。
- 正式运行镜像先 buildx --load，远端查询完成后再检查最新 personal，然后仅 push canonical 版本；latest 独立从固定 digest 更新。不存在冗余 -amd64 不可变标签的部分推送窗口。已有正确镜像复用、失败恢复补 Release、已成功版本跳过、仅文档变化不分配版本；个人发布不写回 main VERSION。
- 本地发布 helper 21 个、同步合同 21 个通过；actionlint 1.7.12 三份变动工作流、shell 语法和 diff 检查通过。首次填写错误完整 SHA 的真实 dry run 37816647464 在源码绑定阶段失败、所有构建 skipped；纠正输入后的旧候选 dry run 因候选修正被取消，不算成功。ac095448e simple dry run 37816972232 已完整 success，证明 sibling 控制工具跨 runner 导入、所选 archive/上下文校验及 OCI 构建路径。
- fresh critical_reviewer 完成最终 f439c543d 的独立只读复核，关闭 missing sync 工具、构建/实际推送间基础变化、多标签部分推送覆盖三个确认问题；未确认剩余阻断。新增 shell 合同覆盖构建后门禁失败零推送及已有镜像恢复不构建/推送。审计 Bundle 20261008T172536Z-personal-release-p3-0283d66e 关闭/verify 通过（1 次执行/验收/独立复核，无观测写入）。latest 可是包装为单平台 index 的独立 digest；P4 只使用 canonical 固定 digest。

- [PR #3](https://github.com/ccisnoxx/sub2api/pull/3) 的最终候选 f439c543d [完整 CI](https://github.com/ccisnoxx/sub2api/actions/runs/37817343794) success，实际 required personal-ready=pass；2026-10-08T17:50:10Z 使用普通 merge 合入 personal，最终源码为 `896de21b4be7f4ec4b4236f4df663b47371665b0`，merge 的树与候选一致。来源记录与 TPS 祖先复核不变；严格保护、管理员约束和 merge-only 保持不变，没有使用管理员绕过。

- 最终 personal SHA 的 [simple dry run](https://github.com/ccisnoxx/sub2api/actions/runs/37819646200) 全部 success：prepare、frontend、linux/amd64 archive、发布上下文/来源校验及 OCI 导出；sync-version-file 按预期 skipped。已下载 release-dry-run-report 和 version-file，报告 commit 等于最终 personal，构建目标只有 linux/amd64。分支 dry run 读取继承的 VERSION=0.2.13；正式个人发布另从来源分配的 tag 生成 VERSION，并核验产物及 OCI 标签一致。该 dry run 不写 registry、Release 或 main VERSION。

- 最终 personal merge SHA 的 [全新 Personal CI](https://github.com/ccisnoxx/sub2api/actions/runs/37819628097) 全部 success，event=push、head_branch=personal、title 的 candidate/base 都等于 896de21b 完整 SHA；personal-ready App 15368/check_suite_id 102465683655 对应本次 run。包含原有全部单元/集成/race/lint/前端/安全检查，没有复用 029cd8fb 或 PR 的检查。
- 启用前再次读回 strict=true、App 15368、enforce_admins=true、禁止 force/delete；关闭状态的真实 workflow_run 37821784834 为 skipped。完成最终 CI、dry run 和独立复核后，于 2026-10-08T18:07:47Z 设置并读回 PERSONAL_RELEASE_ENABLED=true，未新增或扩大凭据权限。

- 启用后的 [错误 CI 归属实际运行](https://github.com/ccisnoxx/sub2api/actions/runs/37821899148) 输入最终 personal SHA，却提供 PR 候选 CI run 37817343794；按预期 failure，输出“CI 不是当前仓库、最终 personal SHA 的成功检查；PR 结果不可用于发布”。核对远端仍无标签，没有创建 Release 或镜像。随后从可信 main 9a358653a 以正确 CI 37819628097 启动正式准备。

- [正式准备](https://github.com/ccisnoxx/sub2api/actions/runs/37821989311) success，workflow revision=main `9a358653a8bab6b0679184a8806f2c786d26aba2`；GITHUB_TOKEN 创建 `v0.2.14-klno.3-tps.1`，实际标签指向最终源码 `896de21b4be7f4ec4b4236f4df663b47371665b0`。随后机器人显式触发 [Release](https://github.com/ccisnoxx/sub2api/actions/runs/37822041580)，event=workflow_dispatch、actor/triggering_actor=github-actions[bot]、head_branch=personal、head_sha=最终源码，simple_release=true，发布目标 linux/amd64。

- 正式 Release 的 prepare 门禁成功，下载的 version-file/VERSION=`0.2.14-klno.3-tps.1`，与个人标签去除 v 后一致；与分支 dry run 的继承 VERSION 区分。正式运行的应用和工具 workflow revision 都为最终 personal SHA。

### 实际发布与不可变引用

[正式 Release](https://github.com/ccisnoxx/sub2api/actions/runs/37822041580) 全部 success：prepare、前端、linux/amd64 archive、来源/上下文校验、构建后门禁、GHCR 推送、OCI 核验和 GitHub Release；sync-version-file 按预期 skipped。未运行 DockerHub/Telegram 路径，未写回 main VERSION；发布前后的 main/personal VERSION Git blob 都未变化。

| 发布事实 | 实际结果 |
|---|---|
| 上游来源 | `v0.2.14-klno.3` / `de08df02ae1d81668a22f798b398aa0438ac1276` |
| personal 源码 / Release workflow revision | `896de21b4be7f4ec4b4236f4df663b47371665b0` |
| 默认分支控制 workflow revision | `9a358653a8bab6b0679184a8806f2c786d26aba2` |
| 个人 Git 标签 | `v0.2.14-klno.3-tps.1`，轻量标签 commit 对象为上述 personal SHA |
| 构建 VERSION / OCI version | `0.2.14-klno.3-tps.1` |
| OCI revision | `896de21b4be7f4ec4b4236f4df663b47371665b0` |
| OCI source / 平台 | `https://github.com/ccisnoxx/sub2api` / `linux/amd64` |
| 完整镜像版本地址 | `ghcr.io/ccisnoxx/sub2api:0.2.14-klno.3-tps.1` |
| canonical digest | `sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76` |
| GitHub Release | [个人版本](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.3-tps.1)，published_at=`2026-10-08T18:14:36Z`，draft=false、prerelease=true |
| 包可见性及归属 | [sub2api 包](https://github.com/users/ccisnoxx/packages/container/package/sub2api)，visibility=public，repository=ccisnoxx/sub2api |

simple 配置原本 skip_upload=true，因此 GitHub Release 不上传二进制资产；已校验的 archive 位于 Actions artifact，并用于本次镜像构建。Release API 的 target_commitish 元数据继承为 main；本轮从已存在的个人标签解析构建来源，实际 tag 对象、artifact commit、workflow SHA 和 OCI revision 均已分别核对为上述 personal SHA，未从 main 构建应用。

### 匿名拉取、重跑和剩余范围

- 使用官方 crane v0.22.1，下载的 Darwin arm64 工具 SHA-256 与官方 release asset digest 一致。使用独立 DOCKER_CONFIG，config.json 只有空 auths，没有 registry 登录或凭据，按 canonical digest 完整拉取 linux/amd64 OCI layout 成功。下载 14 个 blob、12 层、总计 88,247,606 字节；逐个计算 SHA-256 与 blob 名相等，并核对 manifest digest、config 和 layer 长度。不是只读取 manifest 或使用已登录缓存。
- 匿名读取版本标签的 digest 为 `sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`；读取 config 的 revision、version、source、os/architecture 均通过断言。personal-publication artifact 的 digest/image/workflow_sha 与独立 registry 查询一致。
- [同 SHA 重跑](https://github.com/ccisnoxx/sub2api/actions/runs/37823645330) success，输出 state=already_published、同一标签/SHA/digest。之后没有新增 Release run，标签仍唯一并指向同一 personal，版本 digest 未变化；没有重建或覆盖已发布版本。文档跳过和失败恢复为 helper/shell 合同与独立代码复核证据，没有把它们写成实际 GitHub 部署演练。
- latest 的根 digest 为 `sha256:5880f2d63111166d022eeb86865730c018d1808bc2c69869da2e2db4c8f6f60d`，是单平台 index；canonical 版本为独立运行 manifest，两者根 digest 不相同。P4 使用下面的固定 canonical 引用，不使用 latest：

```text
ghcr.io/ccisnoxx/sub2api@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76
```

本轮完成 P3.1–P3.9。未修改应用、计费、迁移或锁文件，未新增或扩大持久凭据权限，未访问或更新 hostdzire。P2.9 的真实 .5 升级仍因历史改写及 120 个冲突停止，P2.11/P4 未完成。没有将原 029cd8fb 的成功检查或 PR 检查代替本轮最终 personal SHA 的检查；没有移动已发布标签或覆盖版本镜像。阶段结果登记只在 main，personal 保持本次已发布的固定源码。


## 8. P4 部署与回滚（2026-10-08，本轮记录）

### 起点、职责与现场核对

- 起点 main/origin/main=`59e07a09a`，工作区干净；fetch origin 后一致。personal/origin/personal 和个人标签保持 `896de21b4be7f4ec4b4236f4df663b47371665b0`。从最新 main 创建 `codex/personal-deploy`，部署工具和阶段证据属于仓库控制文件，未合并 main 应用源码进入 personal。未处理 P2.9/P2.11。
- 已读取用户给出的全局 AGENTS.md、`/Users/sc/.codex/AGENTS.md` 和四份指定文档。本次工作树及祖先目录未找到额外 AGENTS.md；第 1 节中的存在状态属于当时观测。
- 实际 SSH 别名 hostdzire；目录 `/root/sub2api-kin`、文件 `docker-compose.yml`、Compose 项目 `sub2api-kin`、服务 `sub2api`、容器 `sub2api-kin`、端口 `127.0.0.1:10088 → 8080` 均重新确认。Compose `v5.1.3` 支持 `--no-deps`、`--wait`、`--wait-timeout`；主机 x86_64。
- 旧运行引用 `ghcr.io/kln-4096/sub2api@sha256:c0ec609deaf0fb6f323de660ed7d43cf2030b4c4083c6fc4bef506d28f08ad8c`；OCI 和实际二进制版本 `0.2.14-klno.3`、revision `de08df02ae1d81668a22f798b398aa0438ac1276`。应用启动时间仍为 `2026-10-08T01:06:33.196657696Z`。
- 应用、PostgreSQL、Redis healthy；Redis PING=PONG，数据库迁移记录 303 条。应用数据仍为 `sub2api-kin_sub2api_data → /app/data`；数据库和 Redis 的既有挂载、网络、端口与依赖未变。配置只检查必要投影，没有输出环境变量或凭据。

### 目标与兼容条件

目标固定为第 7 节 tps.1 canonical digest，平台 linux/amd64。再次从 registry 查询 OCI source、version、revision 与指定发布一致；标签解引用经 GitHub API 确认，未移动标签或覆盖镜像。

旧源码到目标源码的整个 backend、Dockerfile、Dockerfile.goreleaser、deploy/Dockerfile、既有 Compose 和 .env.example 路径差异为空。`backend/migrations` 的 Git tree 两端同为 `97aa2ba12e590cf497ffb50d09b853c17af611ec`。数据库已应用 filename/checksum 排序清单的 SHA-256 为 `d2765fa67306539751aab2ddc4147ef2f9c340d31745dbed686f3a5f66bb0a5a`。本次同基线仅 TPS/构建控制变动，具备镜像回滚条件；此证据不授权未来含后端/迁移变更的版本自动回滚或恢复数据库。

现场数据库约 643 MB，应用数据约 43 MB，可用磁盘约 32 GB。备份、生产切换与验收结果待下文取得实际证据后登记；本段本身不代表已部署。
