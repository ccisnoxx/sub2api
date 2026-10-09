# P0–P4 实施证据

- 实施日期：2026-10-08（America/Los_Angeles）。
- 最新阶段：P5 前端展示候选与本地验收完成，安全门禁阻塞，未合并/发布/部署（第 9 节）。P3.1–P3.9、P4.1–P4.10 已完成；hostdzire 运行固定 tps.1，两个使用记录页面的新/历史 TPS、耗时及真实官方客户端单次 HTTP 烟测和唯一新增记录均通过。下文第 1–7 节是各阶段当时的记录，当前状态及身份验收边界以第 8 节和 tasks.md 为准。P2.9/P2.11 的 .5 升级不属于本轮。
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

现场数据库约 643 MB，应用数据约 43 MB，可用磁盘约 32 GB。本段属于部署前观测；实际备份、生产切换与验收结果如下。


### 工具、合同验证与独立复核

- 工具提交 `2e5da44431efe152b4b6105a80b7a84f233436a6`，从实现分支快进到无保护规则的 main 并普通 push；没有强推、修改保护或把 main 应用源码合入 personal。personal 的严格 App 15368 门禁和 merge-only 规则保持原状。工具 revision 与发布应用 revision 分开记录。
- 新入口 `deploy/personal/deploy-hostdzire.sh`/`deploy_hostdzire.py` 只依赖标准库，使用原生 hostdzire SSH/scp；校验 fork、个人版本/tag/source/revision、digest 与 linux/amd64，拒绝 latest 和未知来源。Deployment 单独拥有生命周期状态，flock 覆盖预检、备份、拉取、仅应用更新、健康、持久提交和失败恢复。
- 原 Compose 仅将唯一应用 image 行参数化为 `"${SUB2API_IMAGE:?必须指定应用镜像}"`。完整 Compose JSON 在内存中对比，仅 image 可以变化；现有 `.env`、端口、卷、PG/Redis、网络和其他配置保留。镜像选择保存为权限 600 的 `.personal-deploy/image.env`，每次操作显式传入既有 `.env` 和操作镜像文件。
- [Personal Deployment Tools Actions](https://github.com/ccisnoxx/sub2api/actions/runs/37828364221) 绑定工具提交，Linux/CPython 3.11.17：shell 语法与全部 28 项测试通过。真实临时 Git/文件、flock/子进程与 SSH/Docker 替身覆盖成功、拉取失败、健康超时/失败自动恢复、显式回滚/防重放、锁竞争、来源/platform/digest 拒绝、迁移/依赖/配置漂移、备份失败、持久选择部分提交、SSH 中断。替身结果不等同现场 Docker 或生产回滚。
- 第一次 fresh critical_reviewer 确认应用 tar 枚举不能保证 gzip footer CRC 已读取，以及 QA 目录缺失不能证明同名 Docker 资源不存在。坏 CRC 回归在原逻辑实际失败（误成功退出），修正为 gzip 分块读至 EOF 后，CRC、原备份失败、成功路径 3 项定向检查通过；CI 再验证完整合同。QA 写入前增加固定容器/卷/default network 与项目标签存在检查，Docker 查询失败停止。
- 第二次 fresh critical_reviewer 确认两项阻断解除，并核对原始 Actions 日志；未确认剩余生产发布阻断。它们没有进行 SSH 或现场部署验证。审计 Bundle `20261008T183220Z-personal-deploy-p4-d413702c` closed/verify passed，3 次执行均验收，2 次独立复核；详细审计和原始日志在仓库外私密目录。
- 没有改动应用、构建产物、锁文件、计费或迁移，复用 P3 固定应用 SHA 的成功检查；没有扩大为无关应用全量测试。

### 真实隔离回滚验收（不是生产回滚）

本机没有运行 Docker daemon，因此在 hostdzire 使用独立目录 `/root/sub2api-p4-rollback-qa`、Compose 项目/容器/卷/网络 `sub2api-p4-rollback-qa`、loopback 10089 和全新空 PostgreSQL/Redis。写入前确认该目录、三个容器名、三个卷名、默认网络和项目标签资源均不存在。测试副本只替换 4 个现场常量及 2 个健康/端口地址；核心 Deployment 备份、兼容、锁和回滚算法与已复核源码相同，原脚本 SHA-256 为 `cc57bd404f0f5936402f1ee8587c6521facad69020a5cff22dea315e449f0382`，原/测试副本 hash 及替换明细留在 QA 私密记录。

- 实际部署记录 `20261008T190058Z-c332cd133f9c` success：旧 KlN 固定 digest → 指定 tps.1 固定 digest，健康/数据库/Redis/迁移核对通过。
- 实际显式回滚记录 `20261008T190119Z-bf9e2d5c1dc3` success：绑定前一成功记录，恢复同一旧 digest 并健康；迁移指纹和 QA 数据库/Redis 容器不变。
- 最初演练外层 exit=1 发生在 finally 的临时栈 stop：工具已参数化 image，收尾遗漏已提交的 image.env。两个核心操作此前均已成功。依据现有成功选择补传 `--env-file` 和显式同一 `SUB2API_IMAGE` 后，stop 成功；没有再次切换、重做演练或删除卷。最终 `sandbox-result.json` 明确标注 `passed_after_cleanup_fix` 和原外层失败，不能把第一次外层退出写成成功。
- 生产应用/PG/Redis 的 ID、镜像和启动时间前后未变，生产健康采样无隔离演练导致的失败。QA 容器已停止，私密证据、配置和卷保留。没有把生产数据导入 QA，没有在生产进行回滚，也没有数据库恢复演练。

### 正式生产部署与备份

正式入口使用用户指定 digest 和 `--version 0.2.14-klno.3-tps.1`，exit=0；部署记录 `20261008T190259Z-2bb5bc1b6d52` 为 success。操作开始 `2026-10-08T19:02:59Z`；备份后执行目标 digest pull，拉取/OCI 核验通过才适配 image 和启动应用。成功选择只在健康核验完成后持久提交。没有移动 Git 标签、覆盖镜像或分配新版本。

| 现场验收 | 实际结果 |
|---|---|
| 部署前镜像 | `ghcr.io/kln-4096/sub2api@sha256:c0ec609deaf0fb6f323de660ed7d43cf2030b4c4083c6fc4bef506d28f08ad8c` |
| 部署后镜像/运行 Image ID | `ghcr.io/ccisnoxx/sub2api@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`；Docker 29 containerd 的实际 Image ID 同该 digest |
| 实际二进制版本/revision | `/app/sub2api -version` 返回 `0.2.14-klno.3-tps.1` / `896de21b4be7f4ec4b4236f4df663b47371665b0`，built=`2026-10-08T18:09:26Z` |
| 应用启动时间/健康 | `2026-10-08T19:03:16.515665323Z`；容器 healthy、`/health={"status":"ok"}` |
| 数据库/Redis | 两个原容器 ID 不变，healthy；PostgreSQL SELECT 1=1、Redis PING=PONG |
| 配置/挂载 | Compose 逐字对比仅应用 image 一行改变；`.env` 与原备份相同；原应用卷挂载 `/app/data`，全部其他配置保留 |
| 迁移 | 303 行，排序 filename/checksum 指纹仍为 `d2765fa67306539751aab2ddc4147ef2f9c340d31745dbed686f3a5f66bb0a5a` |

备份留在服务器 `.personal-deploy/records/20261008T190259Z-2bb5bc1b6d52/`，目录 700，数据/配置/记录和镜像选择 600；没有上传备份、凭据或原始现场配置到 Git。原 Compose、`.env`、旧镜像完整元数据均保存；旧镜像继续保留。`postgres.dump` 为 custom 格式，60,157,925 字节；`app-data.tar.gz` 为 12,619,793 字节，各自 SHA-256 保存在私密记录。应用归档通过 tar 和完整 gzip EOF/CRC/长度校验；数据库先经 `pg_restore --list`，部署后又以 `pg_restore --file=/dev/null` 解码全部内容，exit=0，不连接目标数据库、不恢复或写入数据库。

备份是在运行期间分别取得，未证明数据库与应用数据具有跨文件业务事务一致性；本次旧、新后端/迁移相同，镜像回滚保留现有数据，不依赖数据库恢复。实际完整数据库恢复与备份后写入的数据取舍仍需要单独维护方案，不能据解码检查声称已恢复验证。

### TPS、网关与中断：实际结果及验收边界

- 初次浏览器验收通过原生 hostdzire SSH 转发到本机 `127.0.0.1:18088`，仅到达生产登录页。用户随后明确“已登录”，本轮复用 `https://api.962850.xyz` 的既有管理员会话，实际访问管理员 `/admin/usage` 和我的账户 `/usage`；没有输入或修改凭据，没有把 P1 合成 API 记录用于生产验收。两个页面先查看近24小时的新记录，再选择昨天并应用，确认上线前历史记录也实际渲染 TPS、首字和总耗时；验收后恢复近24小时范围。说明按钮在两个页面均实际打开，Escape 关闭，提示包含整段平均公式、包含首 token 等待、推理 token 及不能据 TPS 证明 Fast 的限制。控制台未捕获 error/warn，使用浏览器既有窄屏卡片布局，没有运行额外浏览器矩阵。
- 页面样本与 hostdzire 数据库只读查询对应如下，查询只投影时间、模型、输出 token 和计时字段，不保存身份、费用、API key 或会话凭据。页面时间为 America/Los_Angeles（UTC−07:00），两个历史样本均早于 `2026-10-08T19:03:16Z` 上线。TPS 使用原始毫秒值计算，不能用已经格式化成两位小数的秒值重新计算后要求逐位相等。

| 页面/记录 | 页面时间 | 输出 token | 总耗时 ms / 页面 | 首字 ms / 页面 | TPS 页面/原始值计算 |
|---|---|---:|---|---|---|
| 管理员历史 | 2026/10/07 23:59:42 | 113 | 7156 / 7.16s | 4482 / 4.48s | 15.79 tok/s |
| 用户页面历史 | 2026/10/07 19:56:13 | 925 | 27053 / 27.05s | 19435 / 19.43s | 34.19 tok/s |
| 管理员上线后记录 | 2026/10/08 12:36:28 | 1143 | 29015 / 29.02s | 10010 / 10.01s | 39.39 tok/s |
| 用户页面上线后记录 | 2026/10/08 12:42:09 | 141 | 3143 / 3.14s | 1945 / 1.95s | 44.86 tok/s |

- 本次用户页面通过管理员的“我的账户”入口访问，覆盖用户页面的实际数据和显示，不等同独立普通用户身份的登录/权限验收。截取的两个历史耗时区域及说明浮层、只读计时投影保存在仓库外私密 `p4` 证据目录；截图不包含身份、API key、费用或请求内容，没有把原始页面快照提交到 Git。追加只读现场检查确认应用仍是固定 tps.1 digest、healthy，启动时间仍为 `2026-10-08T19:03:16.515665323Z`，本轮没有再次部署。
- 首次授权的最小文本请求使用自写 HTTP 客户端，凭据仅留在进程内存，POST `/v1/responses`，model=`gpt-6.1-sol`，文本要求仅回复 OK。开始 `2026-10-08T19:04:18Z`，收到 HTTP 403，没有成功使用记录，当时未重试。先前仅依据 Ops 的 `upstream_status_code=403` 和被隐藏的 `upstream_error / Upstream request failed` 将其称为上游拒绝，这一归因已纠正：后续只读核对确认同条 Ops 记录为 `error_phase=auth`、`error_source=client_request`、`error_owner=client`，原错误是只允许 Codex 官方客户端；对应日志明确 `reject_reason=missing_engine_fingerprint`。固定应用 SHA 的 `openai_gateway_forward.go` 与 `openai_client_restriction_detector.go` 证明检测在转发前返回本地 403。自写客户端虽设置了 Codex User-Agent，仍缺少必要引擎指纹；不是已确认的应用回归或上游拒绝。没有修改 key、IP、账户限制、检测策略或凭据。
- 上线后既有客户端自然产生成功 WebSocket 使用记录：截至 `19:06:50Z` 有 9 条，均有正输出 token 与 duration。只读样本：1789 output / 43174 ms、first-token 13283 ms，整段平均 41.44 tok/s；994 / 21893 ms 为 45.40 tok/s；598 / 10095 ms 为 59.24 tok/s。它们支持上线后实际 WebSocket 转发和新增记录可用，但不替代本次失败的 HTTP 定向烟测；后续页面验收采用实际浏览器结果和上表样本。没有读取/保存请求文本、API key 或会话凭据到证据文档。
- 健康观察从 `18:59:30.959895Z` 到 `19:06:50.878467Z`，共 812 次；请求间 sleep 0.25 秒，加上 SSH 往返约 0.14–0.28 秒，因此实际采样间隔约 0.4–0.55 秒。只有应用切换时 3 次失败：`19:03:16.159964Z` 至 `19:03:16.964032Z`；相邻成功采样为 `19:03:15.612999Z`、`19:03:17.364751Z`。成功采样间跨度 1.752 秒，失败采样跨度 0.804 秒；报告实际中断约 1–2 秒，精度受采样限制。没有监测每个 HTTP/WS 客户端的断连/重连，不能声称全部现有连接保持。

### P4.9 403 诊断后的单次成功验收

用户明确要求继续解决 P4.9 后，本次从干净且已 fetch 的 main/origin/main `0c425fee89636b0c41e61d14020e2d4ca24985aa` 创建 `codex/p49-gateway-acceptance`；personal/origin/personal 和 tps.1 标签仍为指定应用 SHA。只读排查采用该固定应用源码，未将 main 的应用实现当作运行版本。使用 personal-vps-ssh 经原有 hostdzire 别名检查，复用现有活跃管理员 OpenAI key 的授权测试方式，凭据只经 SSH 留在进程内存。本次仅登记 main 文档，不修改应用或生产配置。

- 改用本机安装的真实 Codex CLI `0.159.0`，由客户端自身生成官方 User-Agent 和引擎指纹，不伪造检测信号或降低现有策略。独立空临时工作目录，忽略用户配置，不保留会话并关闭模型重试；禁用 WebSocket，本次只验收 HTTP 流式文本。提示仅要求回复 OK。临时 loopback 中继以标准 Responses 参数清空工具并设置 `tool_choice=none`，保留真实客户端请求头；收到首个模型请求便占用唯一转发额度，任何后续 POST 返回 409。SSH 转发只监听 loopback，并在结束后关闭。
- 上线前的本地接收器检查没有访问生产模型：确认真实客户端头、单次转发和第二次 POST=409。早期候选仍携带工具声明，相关断言未通过；调整此次请求的工具参数后，最终本地验证通过，目标接收器收到零个工具、`tool_choice=none`。接收器故意返回 HTTP 400，因此官方 CLI exit=1 是该模拟路径的预期结果，不是成功生产请求。原候选失败和最终结果均保留在 Git 外，未扩大为应用全量测试。
- 实际生产请求仅一次：开始 `2026-10-08T20:02:41.501845Z`、客户端结束 `20:02:49.938443Z`；HTTP 200，SSE `response.completed`、文本 OK、CLI 最终消息 OK、`turn.completed` 和 exit=0 全部确认。响应产生的 `X-Client-Request-ID` 对应 `usage_logs.request_id=client:<响应 UUID>`；在请求前最大 ID 之后并限定同一现有 key，只读查询恰好一条新记录，排除同时发生的自然客户端请求。原失败请求没有被改写为成功。

| 新增烟测记录 | 实际结果 |
|---|---|
| 使用记录 ID / 类型 / 模型 | `149251` / 流式（request_type=2）/ `gpt-6.1-sol` |
| 创建时间 | 数据库 `2026-10-09T04:02:47.193302+08:00`；页面 `2026/10/08 13:02:47`（America/Los_Angeles） |
| 输出 / 总耗时 / 首字 | 5 token / 1675 ms / 1607 ms |
| 平均输出 TPS | `5 × 1000 ÷ 1675 = 2.99 tok/s`（两位小数） |
| 两个实际页面 | `/admin/usage` 与我的账户 `/usage` 均显示 2.99 tok/s、首字 1.61s、总耗时 1.68s |
| 对应网关日志 | `codex_official_client_match=true`、`reject_reason=official_client_user_agent_matched`，同一客户端请求的 POST `/v1/responses` access status=200 |
| 请求后运行状态 | 固定 tps.1 digest、版本与指定 revision 一致；应用/PG/Redis healthy，SELECT 1=1、Redis PING=PONG；应用启动时间仍为 `2026-10-08T19:03:16.515665323Z`，两个依赖启动时间未变 |

追加现场检查的第一条 Redis 探针附带额外认证环境，没有通过；按现有部署工具的原命令 `docker exec sub2api-kin-redis redis-cli ping` 核对为 PONG。没有修改 Redis 配置或凭据，也没有把第一次探针记为通过。原始请求结果、诊断必要投影、候选及最终本地检查、现场核对和两个页面的耗时区域截图保存在仓库外权限受限的 `p49` 证据目录；不保存 API Key、会话凭据、原始请求/响应、完整私密日志或配置到 Git。浏览器最后恢复我的账户 `/usage` 和近24小时范围。

本次成功记录连同已取得的历史页面/说明交互证据满足 P4.9 的两个页面显示及网关转发/记录合同。页面仍复用管理员会话；已请求用户切换已有普通用户会话，但截至本次登记尚未取得该身份的验收证据。不能将这些结果扩写为独立普通用户身份的登录、权限隔离或跨用户可见性测试。没有再次部署或回滚，没有新增应用中断；实际生产累计两次定向文本请求，第一次本地 403，诊断后一次成功，成功请求未重试。

### 恢复方式与阶段状态

当前成功记录、配置、来源和迁移仍匹配时，可在本仓库执行 `deploy/personal/deploy-hostdzire.sh --rollback 20261008T190259Z-2bb5bc1b6d52`。工具会再校验当前选择、旧 digest/revision/source、Git 兼容性、数据库指纹和依赖，以旧固定 digest 仅重建应用，通过健康后生成新的成功回滚记录；不自动恢复数据库或应用数据。如果这些条件漂移或不兼容，工具明确停止并保留诊断，不自动恢复数据库。本轮生产仍运行 tps.1，没有执行该生产回滚命令。

P4.1–P4.10 完成；P4.9 已取得两个生产页面的新/历史 TPS、原有耗时、说明交互和官方客户端单次 HTTP 转发及唯一新增使用记录证据。独立普通用户身份、实际数据库恢复和每个客户端的重连仍未验证；这些边界不以替身或自查填补。本次只更新四份阶段文档，复用未变工具及应用的既有检查，没有重复运行应用全量测试。P2.9/P2.11 的 KlN .5 升级仍不属于本轮。最终登记只修改 main 文档，personal/标签/应用镜像保持固定。


## 9. P5 TPS 前端展示对齐（2026-10-08）

### 起点与范围

- 起点 main/origin/main=`1c2e51beef1b6ce4697435b56c26810003b0ab27`；fetch origin 后 personal 仍为 `896de21b4be7f4ec4b4236f4df663b47371665b0`，初始工作区干净。读取最新 main 的四份阶段文档、用户给出的规则及 `/Users/sc/.codex/AGENTS.md`；工作树和祖先目录未发现额外 AGENTS.md。
- 从 origin/personal 创建 `codex/tps-display-alignment`；功能提交 `37cf8f69fcf10da766f3595dfb8a2c321f028bd2`，普通 push，建立 [PR #4](https://github.com/ccisnoxx/sub2api/pull/4)，base=personal。没有合入 main 应用树。另从最新 main 创建 codex/tps-display-evidence 登记阶段结果。
- 读回保护：personal strict personal-ready / GitHub Actions App 15368，enforce_admins=true，禁止 force/delete，仓库只允许 merge commit；Rulesets 为空。没有修改保护或授权设置。PERSONAL_RELEASE_ENABLED 和 PERSONAL_SYNC_SCHEDULE_ENABLED 保持 true。
- SSH 重新确认 hostdzire `/root/sub2api-kin`、sub2api-kin 项目、sub2api 服务、x86_64；应用固定 tps.1 digest/revision、启动时间 `2026-10-08T19:03:16.515665323Z`，应用/PG/Redis healthy，/health=ok。首次直接 Compose ps 缺少 SUB2API_IMAGE，被现有必填检查拒绝；按既有 .env + .personal-deploy/image.env 重查通过，没有修改配置。
- Plus 参考固定为 `90da415c62b94c9417d9ce2b72b1507ed22f0303`，读取 UsageTable.vue、usageTiming.ts、zh/dashboard.ts。gread 确认公有仓库后，因其安装合同没有固定 ref 参数，用 GitHub 官方 raw URL 取得指定提交，未把最新分支当成固定参考。

### 实际前端变更

仅六个维护文件：UsageTps.vue、UsageTable.vue、两份既有测试与 zh/en dashboard.ts。

- 第三行标签 TPS；有效数值 `text-cyan-600 / dark:text-cyan-400`，不可用灰色 `-`。所有有效速度使用相同青色，没有速度分档。
- 小于 0.1 两位有效数字，0.1 至小于 100 四舍五入一位小数并去除 .0，100 起取整数；tok/s 保留。样例 42.19→42.2、7.00→7、150.45→150，1/200000ms→0.005 tok/s，低速正值不会显示为零。
- 标签及数值的 title 提供同一份统计口径和原因；唯一常驻说明按钮移至首字旁，复用 HelpTooltip、原生 button 和既有关闭交互。UsageTps 通过 timing 插槽提供同一 explanation，计算/适用范围不复制到表格。窄屏实际截图发现关闭按钮叠到首行文字，在本处提示内容增加 pr-5 间距后复验。
- unavailableReason 函数与原公式未改动；首字不扣除、历史首字缺失仍可计算。原首字/总耗时及色条表达式保留。前端功能提交37cf8f69 的 backend、迁移、接口类型、计费、锁文件和发布/部署工具差异为空；随后工具链安全升级范围另列。
- 未引入 Plus 的严格首字判断或 first_output_ms、last_token_ms、timing_version、is_complete。前端展示对齐候选完成，不代表已具备 Plus 完整计时能力，也不代表已上线。

### 本地验证与页面证据

macOS / Node v24.16.0 / pnpm 9.15.9 / Vitest 2.1.9 / Playwright 1.62.1，复用冻结依赖与已安装 Chromium headless shell 1228。Browser 插件技能未列出（Browser plugin not available）；CUA 浏览器清点另报 Codex auth token is unavailable，线上预核对改用原生 Chrome UI。没有安装浏览器依赖或更改锁文件。

- 更新合同在旧实现上 52 项失败，涉及旧格式、旧占位、缺少新标签、颜色与说明入口；实现后 UsageTps 51、UsageTable 41、HelpTooltip 5 共 97 项通过。首字旁归属断言最初只查直接父级（HelpTooltip 的内部 wrapper）而失败，修正为实际标签容器后通过。提示间距修正后重新跑受影响的 UsageTable 41 项通过，其他未变化测试复用既有成功结果。
- 六个改动文件 ESLint 通过；间距修正后 UsageTable lint 再通过。最终 frontend build 通过，内含 i18n 键检查、vue-tsc -b、Vite；构建耗时 10.99s。仅有既有类别的 Browserslist 陈旧、Node shell deprecation、混合导入和大 chunk 提示，没有为提示改动依赖或分包。git diff --check 通过。
- 实际构建预览 `http://127.0.0.1:4173`，API 拦截为明确合成记录和 QA 身份；覆盖 13 类文本/历史/图片输入/无效耗时/媒体/compaction/live/probe/degraded/零输出/低速/整数/高速记录。未使用生产认证或模型请求作为本地 fixture。

| 页面 / 语言 / 主题 | 视口 | 结果 |
|---|---|---|
| 管理员 / 中文 / 明亮 | 1440×1100 | 通过 |
| 用户 / 英文 / 暗色 | 1440×1100 | 通过 |
| 管理员 / 中文 / 暗色 | 390×844，触屏上下文 | 通过 |
| 用户 / 中文 / 明亮 | 390×844，触屏上下文 | 通过 |

每组确认 URL/title、非空、无框架 overlay、console error/warning 为空；13 类记录的格式、title 和实际计算颜色正确，原有三行及色条可见。提示位于视口内、文字避开关闭按钮；鼠标点击、Enter、Space、Escape、触屏 tap、关闭按钮与外部点击均实际行使。桌面保留原表格横向滚动，各 TPS 可在所属单元完整查看；窄屏卡片无行间重叠。自动化第一次窄屏 outside-click 选中了隐藏 h1 超时，改为可见 heading 后仅重验窄屏通过；不是应用失败。随后对新间距候选重验四组，并针对桌面完整单元边界和横向滚动截图补证，均通过。

脚本、results.json 和实际截图留在 Git 外：`/Users/sc/.codex/visualizations/2026/10/09/01a11e53-7467-7a71-9b08-79e6c0d2714d/tps-alignment/`。路径的归档日期不代替本节 America/Los_Angeles 的实施日期。代表截图为 admin-zh-light-1440-rows.png、admin-zh-dark-390-tooltip.png、user-en-dark-1440-rows.png、user-zh-light-390-tooltip.png；已人工查看管理员桌面和窄屏提示。

### 远端门禁与安全升级

- [候选 Personal CI](https://github.com/ccisnoxx/sub2api/actions/runs/37871626629) 绑定完整候选 37cf8f69 和 personal 基础 896de21b；既有全部 CI/Security Scan 保留。没有把 P3 旧结果当成本轮检查。
- existing-security/backend-security 的 govulncheck 返回 exit 3：当前 Go 1.27.0 被判定命中 12 项可达标准库漏洞，修复版本 Go 1.27.2。既有独立 [Security Scan](https://github.com/ccisnoxx/sub2api/actions/runs/37871622419) 同样失败。候选 backend 和构建输入与 tps.1 相同，失败不是本轮前端修改引入；动态安全数据库的变化使旧成功结果不再满足当前发布门禁。
- [Go 官方发布记录](https://go.dev/doc/devel/release#go1.27) 确认 1.27.2 于 2026-10-08 发布；[GO-2026-6600](https://pkg.go.dev/vuln/GO-2026-6600) 同日公布且影响 1.27.0 至 1.27.2 之前。扫描涉及 GO-2026-6617、6613、6612、6611、6610、6609、6608、6607、6605、6603、6600、6599；另有未可达的导入/依赖漏洞，不能把它们全部写成运行路径已受影响。
- 解阻需要独立的最小 Go 工具链安全升级（backend/go.mod、三个 CI/Release 版本断言、两个构建 Dockerfile），并评估 P4 对完整 backend/运行资源 diff 的严格兼容限制。用户原范围明确“只调整前端”，因此先请求额外范围决定；用户随后明确选择“允许最小安全升级并继续上线”。在授权前没有改工具链、放宽检查、合并 PR、创建新标签、发布镜像或部署。
- 新版本、digest、部署记录和新页面线上验收均尚未产生。原生 Chrome 预核对确认既有管理员会话能打开真实 /admin/usage，仍显示 tps.1 的旧两位小数；此证据不是新版本验收，也不是独立普通用户身份验收。

首次安全门禁时的阶段状态：P5.1–P5.5 完成；P5.6–P5.8 在安全升级授权下继续执行。线上保持 `v0.2.14-klno.3-tps.1` / `sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`。P2.9/P2.11 的 KlN .5 升级与 Plus 完整计时能力保持独立待办。前端自查不充当独立复核；本轮新增 P4 工具链兼容例外安排 fresh critical_reviewer 只读复核，结果随后登记。

### 经授权的最小安全升级候选

- 应用提交 `d0b8be3548538ebf0ead15ed2f720ff5ce28cc79` 仅将 backend/go.mod、两个构建 Dockerfile、backend-ci/security-scan/release 的版本断言从 1.27.0 更新为 1.27.2：六文件、七处版本替换；业务源码、go.sum、require 依赖和运行层镜像未变。普通 push 延续 PR #4，相关 actionlint 与 diff check 通过。
- main 的三个控制 workflow 同步版本断言。P4 比较完整运行路径的合同继续有效，仅授权三个固定路径全部同方向替换指定 Go 版本行；普通100644文件、其他字节相同，记录双方文件SHA-256和方向。零差异旧证据继续接受，精确补丁反向切换仍须满足旧成功记录、配置、迁移与依赖条件。旧Go有已知漏洞，反向恢复仅用于故障恢复。
- P4 定向33项通过，保护真实Git字节比较、额外依赖/源码/Docker指令/版本/权限/部分路径拒绝、proof边界、健康失败和显式恢复。没有削弱扫描、迁移或数据备份检查。独立复核进行中，当前未部署新候选。

- Go1.27.2 新候选的独立 Security Scan `37872430782` 继续返回 exit3：标准库报告消失，但 x/net v0.58.0 命中5项可达漏洞（GO-2026-6617、6612、6611、6610、6603），均由 v0.60.0 修复。官方模块 go.mod 要求 x/crypto v0.57.0、x/sys v0.48.0、x/term v0.46.0、x/text v0.42.0，现有版本须随之最低更新。已请求新的最小依赖补丁范围决定；未将纯工具链授权扩写为自动依赖升级/部署授权，线上仍 tps.1。

- 用户再次明确选择“允许最小依赖安全补丁并继续上线”。在隔离工作树准备并检查精确补丁后，普通push提交 `2b19ca02d950efe9cc3609b2b9830802193f6b6f`，仅go.mod八个x/*模块与go.sum新增16条校验行。官方模块go.mod显示text0.42要求tools0.49、mod0.41、sync0.23，故实际最低解析为八模块。Go1.27.2的go mod verify通过，模块图已保存Git外。新Personal CI `37872758394`绑定该提交。

- 首轮fresh critical_reviewer完成，只读追踪Git证明、远端生命周期、旧证据及恢复；确认一项控制分支直接CI回归（main Go声明旧版而固定断言新版）。通过应用提交 `14afc9891e4eff9eea6d9f21dc48cbf8b0cfd4eb` 及main同步定义改为精确核对实际checkout go.mod声明，setup-go与原安全/发布门禁保留。最终四文件安全hash清单及依赖补丁已安排第二个fresh critical_reviewer；未复用前轮线程，结果随后登记。
- 最终P4候选仅接受已审定四文件完整字节hash，不再保留初步的泛版本行例外。更新后33项定向用例、3份workflow actionlint、shell语法及diffcheck通过；验证额外依赖、校验行、源码、Docker指令、版本、权限、symlink、部分路径、混合方向和未审定有效长度hash全部拒绝。

- 第二轮fresh复核确认并关闭反向例外入口问题：原候选允许普通deploy旧tps.1使用合法reverse proof，绕过显式rollback成功记录/配置绑定。远端validate_proof现拒绝deploy+rollback方向，位于备份、拉取、启动前。新增真实状态回归在旧实现返回0而失败，修复后普通反向拒绝、显式和自动恢复3项通过；最终P4 34项通过。生命周期、迁移、依赖和持久提交原合同继续保留。
- 完整Personal CI `37872892187` 安全/TPS通过，但backend unit及lint因Go1.27.2 V5 export data被旧x/tools拒绝而失败；没有当成flaky重跑或忽略。官方x/tools v0.50.0支持V5，golangci-lint v2.14.0包含该读取器，按安全升级所需工具兼容更新后，失败的TestAuthIdentityFoundationSchemas定向通过（1.941s），go mod verify通过。最终应用候选 `e5acf91d204c6dc56516e88cf2ba906d9fb7c59e`、x/tools0.50、lint2.14.0；sum删除本轮中间0.49两行，最终相对原版本仅新增16条checksum。
- 四文件安全清单刷新为896de→e5acf固定blob；P4 34项及actionlint/diffcheck通过。该最终工具兼容及清单刷新由第三个fresh critical_reviewer窄范围复核，不复用前轮上下文或重复生命周期验证；新candidate全门禁仍须实际通过。

### 最新门禁核对与待授权的 lint 兼容补丁

- 第三轮 fresh critical_reviewer 对最终 e5acf91 的 x/tools v0.50.0、lint v2.14.0 和四文件完整字节清单复核完成，没有确认阻断项；旧门禁失败由 schema V5 导出数据兼容问题导致，定向失败用例已在修复后通过。三轮复核的审计 Bundle `20261009T015946Z-tps-display-alignment-safe-release-8ee15421` 已 closed，audit-verify 通过。复核不代表后续 CI 自动成功。
- main 控制定义提交 `50290816c` 普通快进推送至 origin/main，仅包含三个 workflow 的 Go 声明精确断言和 lint2.14.0，不包含 personal 应用源码。
- e5acf91 的 Personal CI `37873707977` backend-security 已通过，后端 Unit tests 已通过；golangci-lint2.14.0 报告16条 SA1019：x/net0.60 将既有 HTTP/2 Transport、ConfigureTransports、Server 参数和 GoAwayError 标记为弃用。没有据此改动 PING、H2C 或错误分类业务代码；集成和 recording/race 门禁尚待最终结果。
- 已在Git外准备 backend/.golangci.yml 的三条精确配置规则，只匹配五个固定文件内列举接口的弃用告警；官方 lint config verify 通过，逐条对照当前16条告警及四项不同文件/接口/检查反例通过。此前依赖授权明确限定 go.mod/go.sum，已请求新增 lint 配置与 P4 精确字节清单的范围决定，尚未把该候选配置写入应用、推送或合并。临时定向 staticcheck 仍在执行，结果随后登记。

- 临时配置第一次置于 /tmp，lint 默认按配置目录计算路径，精确路径规则未匹配，故仍报告16条；按真实 backend 配置目录验证后，原输出按行去重隐藏的同一行 http2.Server 弃用告警显现。补齐明确列举的 Server 类型后，官方 config verify 与三个受影响包的 staticcheck 为 `0 issues`，退出0；只使用临时候选文件，检查后已删除，维护配置未变。没有把原始失败记作成功，也没有清缓存或迁移业务接口。
- main 的四文件 P4 兼容补丁已保存为 `4629a8ad7` 并普通快进推送，实际工具 SHA-256=`3ccd5856db26a41fe31de85d2c2d5ff170fc57407c931bcf2c5bdd58965b326c`；34项既有及新合同用例通过。临时 lint 补丁尚不在该清单中，后续实施时必须更新精确第五文件及 fresh 复核，不能凭四文件旧复核直接部署新的不同候选。
- 截至本次登记，尚未收到 lint 配置新增范围授权；PR #4 仍 open/head=e5acf91/base=personal/merge blocked。Personal CI37873707977：安全扫描、前端、TPS、单元测试等通过，lint失败，集成及recording/race尚待结束。没有新个人标签、镜像或部署记录。线上仍为 tps.1 及原固定digest；展示对齐候选完成，发布上线未完成。

### lint 兼容补丁获授权及上游版本再核对

- 用户明确允许已准备的精确 lint 兼容补丁并继续发布上线。应用提交 `ce682c4901033d09c7e12614b7b73e11bdd17ef4` 仅新增 backend/.golangci.yml 三条规则，列举五个固定文件内的既有 HTTP/2 弃用接口；其他SA1019、lint与安全扫描继续执行，未迁移业务调用。最终 Personal CI [37875785175](https://github.com/ccisnoxx/sub2api/actions/runs/37875785175) 绑定该候选与896de21b基础；此前e5候选的后端unit、integration、recording/race最终均通过，只有lint及汇总门禁失败。
- P4第五文件的完整SHA256：旧 `ed037798aa38c9e377aafa2fad35c12dc95b1edb29f18ec61f0a08b4bfbd5599`，新 `0ee503a72db01ad77e24cdab2038b53880835ec42d6aa100028e1138c48964f8`。补丁ID改为 `go1.27.2-xnet0.60.0-http2-lint`；五个路径完整同向、100644和所有字节检查继续有效，新增全局禁用SA1019的真实Git反例被拒绝。34项部署合同测试通过（2.381s），shell语法与diff检查通过。
- fresh critical_reviewer仅复核新的lint范围、五文件字节/模式/证明及恢复连接，未重复完整生命周期审查。其独立规则匹配、八个反例、双方全部文件hash、十个proof反例和普通deploy反向拒绝均确认；无确认新阻断。审查输入快照一致，审计 `20261009T024252Z-tps-display-lint-release-final-88197bc2` closed/audit-verify通过。P4控制更新提交 `83b01bda3` 已普通快进main，实际工具SHA256 `c18dbdd782f843f9fed07eb97b4e516bedb64b0fd0709bd17dc0efbfdd13ac29`。
- 重新读取KlN官方最新发布与远端refs：仍为 `v0.2.14-klno.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d`，与此前停止的版本相同。.3/de08仍不是它的祖先；只读merge-tree到当前personal仍120个冲突路径，净差异164文件、6821插入/4161删除，backend/migrations无净差异。官方说明涉及网关池取票/验证/换票续办/计时，不能把无新增迁移当作完整后端兼容证明。仅获取隔离readonly refs和合并预览，不修改来源记录、个人应用或服务器配置。
- 已说明建议先完成当前TPS及安全版本上线，再单独处理.5历史整合；按用户第1项明确的继续上线指示推进原任务。调整为先整合.5的顺序选择已提出，未据此自动授权或实施更大后端迁移。P2.9真实升级及P2.11保持独立未完成。

## 10. KlN .5 整合及统一发布（实施中）

### 10.1 授权、固定来源与实际历史整合

维护者明确允许已准备的精确lint补丁，并选择“先整合 .5，再统一发布上线”。此前先发布 .3 展示候选的建议已被该选择取代，.3 候选未发布。上游最新发布核对仍为 `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`，官方标签只读获取且没有移动既有标签。

- 旧固定上游 .3：`de08df02ae1d81668a22f798b398aa0438ac1276`；旧个人线上：`896de21b4be7f4ec4b4236f4df663b47371665b0`。
- 保留 TPS/安全/lint 的合并前个人候选：`ce682c4901033d09c7e12614b7b73e11bdd17ef4`，完整 Personal CI37875785175 success。
- 逐树比较：.3→.5 官方净变更164路径（6821插入/4161删除）；.3→ce个人34路径，交集为空。普通merge产生120历史冲突，119归官方净变更、1为上游未改的个人表格测试；按实际改动方恢复并逐一核对所有索引路径的blob/mode，零不符。没有借ours/theirs覆盖真实双边改动。
- 普通合并提交 `c00e8982258736a188a09023e901d8d39671814f`，父提交为ce与c7。额外只更新来源JSON为 .5固定tag/SHA；对 .5 的diff仍恰为34个人路径，官方后端逻辑未经手改。没有强推或重写个人历史，自动同步历史改写/冲突门禁保持。
- [PR #4](https://github.com/ccisnoxx/sub2api/pull/4) 已改为 .5/TPS/安全统一交付。新的 [Personal CI37877529750](https://github.com/ccisnoxx/sub2api/actions/runs/37877529750) 已全部success：绑定、TPS/构建、前端关键测试/类型、后端单元/集成/recording竞态、lint、安全扫描及personal-ready。此为候选门禁，最终personal SHA仍须重新检查后才能发布。

### 10.2 新运行状态与恢复边界

fresh deep_auditor 比较固定 .3/.5及线上896对象。数据库迁移、Ent及实例配置无净差异；计费/usage记录 owner 无变化。确认 .5新增 `openai_gwpool_usage_rounds.rounds[].active_usage`、归档活跃时长以及contacts.previous；.3 typed decode后整体回写顶层JSON键会丢弃这些字段，后台idle维护亦可触发。所以同迁移及旧镜像可健康启动不能证明无损回滚。P4只允许经审定的 .3→.5正向升级，跨基线显式/故障自动镜像回退禁止，失败保留备份和诊断供人工评估，不自动恢复数据库。

新reporter可能在HTTP监听和/health前写网关JSON，备份时点必须在启动新应用前。新旧进程共享Redis并行启动会清理旧进程活跃槽位；保持单应用Compose替换顺序和获准短中断，不采用共享Redis蓝绿。

hostdzire只读账号预检确认：当前启用网关池的OpenAI账号为0，池影子/母账号和setup-token来源为0，实际池端点为0。现有配置没有新增成员身份阻断；未据此声称外部池成员/batch协议已验证，后续启用池前需另行检查。当前应用仍是 .3-tps.1原digest且healthy。

两个官方发布对象的AGENTS.md引用的 outbound identity、extra freshness约定及docs/tasks均缺失；已查找而未伪造规则，此为来源覆盖缺口。主仓库既有阶段文档继续维护在main。Plus完整计时能力仍未引入，TPS继续使用整段总耗时公式。

### 10.3 合并后前端页面验证

在最终合并应用树c00重新以frozen lockfile安装依赖，首次构建因新工作树尚无node_modules而停止（vitest不存在）；补齐依赖后i18n/Vue类型/Vite构建通过，Vite11.12s。没有因前一次环境失败反复重跑检查。实际新构建的四组页面（管理员zh浅色1440、用户en暗色1440、管理员zh暗色390、用户zh浅色390）全部pass，覆盖13类有效/不可用/历史记录、实际计算颜色、三行空间、视口内提示、关闭按钮与文字间距、Enter/Space/Escape/click/tap/外部关闭，控制台/网络错误为0。结果与截图留在Git外`tps-alignment-klno5/`，未用旧 .3产物替代最终候选页面。远端最终候选的97项TPS测试/改动文件lint和构建也通过。

### 10.4 P4候选实现及验证边界

main控制候选新增精确正向证据：固定旧/新上游完整tag+SHA，以及896/c00被部署运行树的全部路径、mode、type和Git object。旧3,437条目SHA-256 `ecf6eff8c2902b29a1a689232dc8a1028e9d10e5615bcb6e293d35d8d278df35`；新3,464条目 `928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d`，精确151运行变化路径。最终合并revision可变，运行树必须全等；其他后端/部署/迁移内容或mode/type修改均拒绝。远端公共validator绑定真实旧新revision和镜像基线version，拒绝zero/security伪装、字段/hash/source/list篡改。

证明明确 `image_rollback_compatible=false`；普通反向deploy、本机/远端显式回滚及新进程启动后的自动旧镜像恢复均拒绝。启动/health失败非零，保存原错误、diagnostics/observed_running、`rollback_status=blocked`/`E_COMPATIBILITY`，不覆盖原selection；已替换selection而后续落盘失败继续保留部分提交并停止自动接管。原同基线零差异/精确五文件patch的锁、漂移、备份、自动恢复/显式回滚合同保持。

新增9项合同测试，总43 tests5.801s OK，定向固定Git/反向及树变化测试、shell语法、py_compile、diff-check均通过。首次测试日志包装使用zsh保留变量status产生包装错误，测试本身已通过；修正包装后明确获得最终exit0。未声称替身验证是真实服务器或实际数据库恢复。Linux部署CI增加只读获取 .5官方标签，actionlint通过。fresh critical_reviewer正在复核实际四文件diff和融合候选，未结束前不发布部署。

独立复核已完成且未确认阻断：全部官方/个人联合源码树零不符，固定运行树与151路径重新独立计算匹配；16个证明/回滚拒绝检查和7种纯内存生命周期失败注入通过，确认启动后无旧镜像恢复以及selection部分提交保护。复核四文件输入前后SHA-256不变。当前审计 `20261009T025813Z-klno5-tps-integration-release-8b507569` closed/verify passed（状态调查、实现、fresh独立复核三个阶段）；先前Go/依赖/lint独立复核保留各自已关闭审计，不伪造聚合计数。
