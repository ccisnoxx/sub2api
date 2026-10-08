# P0/P1 实施证据

- 实施日期：2026-10-08（America/Los_Angeles）。
- 本轮范围：本地 P0 基线准备、P1 平均输出 TPS，以及 hostdzire 只读检查。GitHub 设置修改、远端推送、镜像发布、生产部署均未执行。
- 当前结论：P1 本地完成；P0 本地准备完成，P0.2/P0.6 的远端定义替换仍待执行。不能把本地暂停文件认定为远端已生效。
- 功能代码 SHA：`f74554702e6d55554342134b68fc54ff4a4ff541`，由 `codex/usage-tps` 快进合入本地 `personal`。此后的证据提交只修改文档，复用相同源码的有效验证结果。
- 外部产物：本轮未创建 PR、个人发布标签或镜像；运行镜像仍为 KlN 原版本。

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
