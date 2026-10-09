# 开发日志

## 2026-10-08：KlN .5、TPS 展示对齐及安全补丁统一上线完成

- 按维护者“先整合 .5，再统一发布上线”的授权完成，未发布此前 .3展示候选。普通合并c00保留个人与固定官方c7双方历史，120历史冲突按实际净改动归属处理并逐一核对blob/mode；官方164路径与个人34路径无重叠。[PR #4](https://github.com/ccisnoxx/sub2api/pull/4) 按strict personal-ready/App15368、merge-only保护合入personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，无管理员绕过/强推/main应用树混入。
- TPS标签、有效青色/不可用灰色短横线、三段紧凑数字/tok/s及首字旁唯一说明入口完成；公式仍为output_tokens×1000÷duration_ms，不扣首字，不改变原有适用范围/原因、耗时或色条。获准安全补丁采用Go1.27.2、x/net0.60等最小依赖及精确HTTP/2 lint规则，没有改变业务代码来压制弃用告警。
- 最终 [Personal CI37879317375](https://github.com/ccisnoxx/sub2api/actions/runs/37879317375) 完整success；97项TPS/表格/提示与静态构建、.5额外121项前端合同、最终构建四组1440/390明暗及触屏/键盘页面预览通过。P4本地与[Linux CI37879304327](https://github.com/ccisnoxx/sub2api/actions/runs/37879304327) 43项合同通过，状态调查/实现/fresh独立复核审计closed/verify passed，确认问题已关闭。
- 既有流程分配并发布 [v0.2.14-klno.5-tps.1](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.1)，[Release37880741133](https://github.com/ccisnoxx/sub2api/actions/runs/37880741133)成功。固定amd64镜像digest `sha256:a6a2a017622158b8813c9355db816d9cca0b37654553c5bacf8aa304b51f06e6` 与9397/source/version一致，公开匿名完整导出及二进制Go/依赖核验通过；原标签、原digest及main VERSION未改变。
- main77的P4工具在备份后仅切换hostdzire应用，记录 `20261009T035515Z-193d1b92933e` success，UTC03:55:46完成。PG备份60,739,634字节、应用数据12,936,600字节，记录目录700/文件600；应用healthy、/health=ok，配置、PG/Redis容器及启动时间、303条迁移指纹均不变。仅应用顺序替换，中断未精确计时。状态复核确认 .3会丢弃 .5新网关JSON，工具仅允许固定 .3→.5向前升级，跨基线自动/显式旧镜像回退禁止，不恢复数据库。
- 实际管理员/我的账户两页面的新及上线前历史记录、颜色、说明入口/完整提示和click/Enter/Escape通过。一次官方Codex CLI0.159.0请求HTTP200、完整OK、exit0；唯一记录150264为5 output/1819ms、首字1751ms，用户页显示2.7 tok/s、1.75s/1.82s，不扣首字。两页面筛选已恢复近24小时；未伪造官方指纹或重试模型请求。
- P2.9/P2.11与P5.1–P5.8完成。独立普通用户身份、实际数据库恢复、所有客户端重连及外部池成员协议（现场启用池为0）仍未验证；启用池前需专门验收。**前端展示对齐完成，Plus完整计时能力尚未引入**，没有新增首输出/末token/完成状态等后端字段。最终四份文档只回写main，详情见[实施证据第10节](../openspec/changes/add-usage-tps-and-personal-release/implementation-evidence.md#10-kln-5-整合及统一发布已完成)。

## 2026-10-08：TPS 展示候选完成，等待精确 lint 兼容补丁范围决定

- 从最新 main `1c2e51bee` 读取阶段记录，从 personal `896de21b` 建立 codex/tps-display-alignment；[PR #4](https://github.com/ccisnoxx/sub2api/pull/4) 当前候选 `e5acf91d204c6dc56516e88cf2ba906d9fb7c59e`，未绕过 strict personal-ready / App15368、管理员约束和 merge-only。控制工具和证据继续留在 main，未合入 main 应用树。
- Plus 固定参考90da415c：TPS标签、有效青色、不可用灰色 -、三段紧凑数字和tok/s；唯一说明按钮移至首字旁，复用 HelpTooltip。保留整段平均公式、首字不扣除、原适用范围/不可用原因、首字/总耗时/色条。未引入 Plus 完整计时字段或严格首字判定，业务源码、数据库和计费不变。
- 97项定向组件/表格/提示测试、六文件lint、i18n/类型/Vite构建通过。实际构建预览管理员/用户、1440px/390px、明暗主题、13类合成记录和鼠标/键盘/触屏提示交互通过；修复窄屏关闭按钮与文字重叠，截图和结果留在Git外。
- 动态安全扫描先报告既有Go1.27.0的12项可达漏洞，再报告x/net五项。用户分别明确授权最小Go1.27.2和依赖安全升级；最低八个x/*模块、16条新增校验行，无业务代码修改。x/tools0.50和lint2.14.0修复Go导出数据V5兼容，原失败schema用例已通过。
- 三轮fresh独立只读复核修复 main 版本断言及普通deploy反向恢复漏洞，最终无确认阻断；审计 `20261009T015946Z-tps-display-alignment-safe-release-8ee15421` closed/verify通过。main `50290816c` 保存workflow更新，`4629a8ad7` 保存四文件完整字节清单及34项部署合同测试。
- 最终 [Personal CI37873707977](https://github.com/ccisnoxx/sub2api/actions/runs/37873707977) 安全扫描与单元测试已通过，lint因现有HTTP/2接口弃用SA1019失败，集成/recording仍待结束。精确三规则、五文件的配置补丁在Git外准备，官方配置校验与受影响三包staticcheck为0 issues；维护配置未改。其新增 lint 文件超出此前go.mod/go.sum限定，已请求范围决定，尚未收到授权。
- P5.1–P5.5完成；P5.6–P5.8未完成，尚未合并、分配版本、发布GHCR或部署，hostdzire仍是tps.1固定digest。线上预核对覆盖现有管理员会话的两个页面，仍为旧格式，不算新版本验收或独立普通用户身份验收。KlN .5升级和Plus完整计时能力保持独立事项。完整事实见[实施证据第9节](../openspec/changes/add-usage-tps-and-personal-release/implementation-evidence.md)。

## 2026-10-08：P4 部署工具、hostdzire tps.1 上线与 P4.9 验收完成

- 从最新 main `59e07a09a` 创建 `codex/personal-deploy`，工具与阶段结果留在 main 控制线；提交 `2e5da44431efe152b4b6105a80b7a84f233436a6` 快进 main 后普通 push，没有绕过 personal 保护。personal 和 `v0.2.14-klno.3-tps.1` 仍为固定应用 SHA `896de21b4be7f4ec4b4236f4df663b47371665b0`，未合入 main 应用树、移动标签或重发镜像，未处理 KlN .5。
- 实现固定来源/digest/platform 输入、生命周期 flock、现场预检、私密配置/数据备份、仅应用 Compose 更新、健康后原子选择/成功记录、失败诊断与明确退出、相同完整后端/迁移基线的自动和显式镜像回滚。工具没有 force 兼容开关，不恢复数据库、不更新 PG/Redis 或删除卷。
- [Linux/Python 3.11 Actions](https://github.com/ccisnoxx/sub2api/actions/runs/37828364221) 通过 28 项合同测试及 shell 语法。两次 fresh 只读复核确认并关闭 gzip footer 完整性和 QA 同名资源检查问题，坏 CRC 回归先红后绿；审计 `20261008T183220Z-personal-deploy-p4-d413702c` closed/verify passed。
- hostdzire 独立空数据库/卷/10089 临时栈完成真实旧镜像→tps.1→旧镜像回滚；两个核心记录 success。最初临时 stop 漏传 image.env 导致外层退出1，按成功选择补传后 stop 成功，没有重复切换，保留 QA 卷与私密证据；生产三容器身份/启动未变。
- 正式部署记录 `20261008T190259Z-2bb5bc1b6d52` success，生产应用一次切换：旧 KlN digest `sha256:c0ec609deaf0fb6f323de660ed7d43cf2030b4c4083c6fc4bef506d28f08ad8c` → 固定 tps.1 digest `sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`。实际版本、revision、容器健康、/health、SELECT 1/PONG 均通过；仅 Compose 应用 image 一行变化，原 .env、挂载、PG/Redis、网络保留，303 条迁移指纹相同。
- 私密备份在正式记录目录，PG custom dump 60,157,925 字节、应用归档 12,619,793 字节；tar/gzip CRC 和 pg_restore 全部解码通过，没有实际数据库恢复演练。记录/数据文件600、目录700，旧镜像和备份保留。兼容条件仍成立时使用 `deploy/personal/deploy-hostdzire.sh --rollback 20261008T190259Z-2bb5bc1b6d52`；生产未执行回滚。
- 健康采样中断约1–2秒，切换区间 UTC19:03:16，未跟踪所有客户端断连。首次自写 HTTP 烟测收到403，无成功记录，当时未重试；上线后既有 WebSocket 客户端实际产生9条正输出/耗时记录。后续经 Ops 的 auth/client_request/client 字段、固定源码与日志确认403来自本地官方客户端限制检测，`missing_engine_fingerprint`，先前“上游403”的归因已纠正。
- 用户随后完成登录，复用生产域名的既有管理员会话，实际检查管理员 `/admin/usage` 和我的账户 `/usage` 的新记录及上线前历史记录。两个页面 TPS、首字/总耗时和说明按钮交互通过，恢复原近24小时筛选；独立普通用户身份未验证。只读原始计时核对：历史 113/7156ms=15.79 tok/s、925/27053ms=34.19；上线后 1143/29015ms=39.39、141/3143ms=44.86，与实际页面相同。截图只包含耗时/提示区域，保存在 Git 外私密目录，控制台未捕获 error/warn。此前“已登录”的页面验收没有发起第二次文本请求或再次部署，固定镜像仍 healthy、启动时间未变。
- 用户要求继续解决 P4.9 后，从干净 main/origin/main `0c425fee89636b0c41e61d14020e2d4ca24985aa` 创建 `codex/p49-gateway-acceptance`。使用真实 Codex CLI 0.159.0，自然生成官方客户端指纹；本次文本请求禁用工具、WebSocket 和重试，临时 loopback 中继最多转发一次。最终本地接收器验证通过，第二次 POST=409，故意400导致 CLI exit=1 属于模拟预期；此前工具声明未清空的候选断言失败如实保留，未向生产发送模型请求。
- 修正后的唯一生产请求 UTC `20:02:41–20:02:49`，HTTP200、SSE完成、CLI exit=0且仅回复 OK；响应客户端请求 ID 关联唯一使用记录 `149251`，5 output / 1675 ms = 2.99 tok/s，首字1607ms。管理员与我的账户页面都实际显示同条记录：2.99 tok/s、首字1.61s、总耗时1.68s，与原始值一致。对应日志确认官方客户端匹配和请求200。现场应用仍为指定 tps.1 digest/version/revision且启动时间未变，PG/Redis healthy、SELECT 1/PONG通过；没有改变检测策略、凭据或生产配置，没有再次部署或增加应用中断。
- P4.1–P4.10 完成，P4.9 已勾选；当前页面证据仍使用管理员会话，独立普通用户身份未验证，不等同权限隔离验收。实际数据库恢复及逐客户端重连也未验证。P2.9 真实 .5 升级和 P2.11 未处理；本次只更新 main 的四份阶段文档，未移动个人源码/标签/镜像。完整方法、实际/模拟边界、回滚条件、中断和剩余事项见 [实施证据第8节](../openspec/changes/add-usage-tps-and-personal-release/implementation-evidence.md)。

## 2026-10-08：个人镜像发布，P3 已完成

- 从已验收 personal 029cd8fb 建立发布分支，保留原 KlN v0.2.14-klno.3/de08df02 和 TPS；[PR #3](https://github.com/ccisnoxx/sub2api/pull/3) 通过 required personal-ready 后普通 merge，最终 personal/构建源码为 `896de21b4be7f4ec4b4236f4df663b47371665b0`。没有合并 main 应用源码，没有修改应用、锁文件或迁移。
- personal-release 与 Release 复用现有 matrix、前端单次构建、archive 来源及校验；绑定可信 personal 完整 CI、实际 App 15368 门禁和最新 SHA，串行分配 tps.N，机器人创建标签后显式 dispatch simple/linux/amd64 Release。构建后实际推送前再核验；同 SHA 复用，已成功版本跳过，部分失败复用镜像，文档变化跳过，个人 VERSION 不写回 main。
- 21 个发布 helper、21 个同步合同、三份变动 workflow actionlint 和 shell 语法检查通过；fresh critical_reviewer 关闭确认问题，最终无确认阻断，审计 Bundle 20261008T172536Z-personal-release-p3-0283d66e 关闭/verify 通过。错误 SHA dry run 和错误 PR CI 归属真实运行均按预期拒绝。
- 最终 personal SHA 的 [全新完整 CI](https://github.com/ccisnoxx/sub2api/actions/runs/37819628097) 与 [simple dry run](https://github.com/ccisnoxx/sub2api/actions/runs/37819646200) 全部成功；严格保护和 merge-only 读回不变后启用 PERSONAL_RELEASE_ENABLED=true。[正式准备](https://github.com/ccisnoxx/sub2api/actions/runs/37821989311) 和 [Release](https://github.com/ccisnoxx/sub2api/actions/runs/37822041580) success。
- 已发布 `v0.2.14-klno.3-tps.1` / `ghcr.io/ccisnoxx/sub2api:0.2.14-klno.3-tps.1`，OCI revision 和 VERSION 一致，平台 linux/amd64，包为 public。空凭据目录完成 88,247,606 字节、14 个 blob 的匿名完整拉取及 SHA-256 校验。固定 digest：`sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`。
- [同 SHA 重跑](https://github.com/ccisnoxx/sub2api/actions/runs/37823645330) 返回 already_published，没有新标签、再次 dispatch 或覆盖镜像。结果只回写 main，personal 固定源码与 main VERSION 保持不变。P3.1–P3.9 完成；P2.9 真实 .5 升级、P2.11/P4 保持未完成，未访问或部署 hostdzire。
- 完整来源、workflow revision、CI/Release 链接、复核和 P4 canonical 镜像引用见 [实施证据第 7 节](../openspec/changes/add-usage-tps-and-personal-release/implementation-evidence.md)。P4 使用 `ghcr.io/ccisnoxx/sub2api@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76`。

## 2026-10-08：personal 同步 PR 与候选检查，P2 实施

- 已获本轮 fork 普通推送、PR、Actions 和必要设置授权；远端 main/personal 先让暂停定义生效，移除旧 rebase/强推/打 KlN 标签/重建 main。
- 实现 `.github/personal-sync/` 与 Personal CI：固定 KlN 标签/SHA、独立引用、普通 merge、来源记录、冲突/历史改写/权限/基础变化停止及同候选复用；保留完整既有 CI/Security Scan，pnpm 9 frozen lockfile。
- 实际 KlN klno 已重写，已发布 v0.2.14-klno.3 仍是当前来源；修正发布标签与当前分支祖先限制。没有制造新版本，显式演练只增加候选元数据。
- 21 个定向合同用例、四份 workflow actionlint 通过；人工同树 squash 的 CI 来源反例先红后绿。两次 fresh 独立只读复核关闭确认问题，审计 Bundle 20261008T154650Z-personal-sync-p2-final-2fcdbc65 校验通过。
- 真实无更新与输入拒绝通过；GitHub Actions 创建演练草稿 PR #1，显式 CI 绑定 ad84de37 候选及 67cb02daa 基础；原生机器人检查待批准已处理，同基础重跑复用 PR/SHA/CI。
- 最终 personal `029cd8fb8b04d18a6b3abe4528effe4aeec75859` 与 main `2ad9c5be246844024afd67354931283d30e693e5` 的控制定义相同；机器人演练 PR #2 绑定候选 9c33a062/base 029cd8fb，显式/原生/personal push 完整 CI 通过，PR 实际必要 personal-ready=pass。原生曾触发既有模型缓存并发用例失败，定位时间窗口后一次仅失败 job 重跑通过，未改应用或删检查。旧候选全部测试通过后因基础变化被 personal-ready 拒绝，PR #1 已关闭；错误 SHA dispatch 也按预期停止。
- UTC 16:10:39 上游发布 v0.2.14-klno.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d，新的解析运行发现它后按历史改写规则停止；de08 非其祖先，共同祖先 cdd6e447，两标签净差异 164 文件。只读 merge-tree 进一步确认普通合并有 120 个冲突文件（24 content、96 add/add），没有修改候选或选择 ours/theirs。真实升级候选、逐项冲突处理和人工差异审查仍待执行，原基线/TPS 保持不变。
- 已新增 strict personal-ready/App 15368 保护，管理员受约束，禁止强推/删除；仓库只允许 merge commit。最终 CI/独立复核/保护验收后于 UTC 16:27:08 启用并读回定时变量 true，每日 UTC 03:17 定义 active，首个真实 schedule 尚未发生。P0.2/P0.6、P2.1–P2.8、P2.10 完成，P2.9 真实升级部分仍未勾选。阶段进度与最终结果登记以默认 main 的最新日志/证据为准，personal 保存固定源码的阶段记录。完整证据见 [实施证据](../openspec/changes/add-usage-tps-and-personal-release/implementation-evidence.md) 第 6 节；未发布镜像、未 SSH 或部署生产，P3/P4 留待后续。

## 2026-10-08：平均输出 TPS，P0/P1 本地实施

- 固定来源：KlN `v0.2.14-klno.3` / `de08df02ae1d81668a22f798b398aa0438ac1276`。hostdzire 只读核对的 OCI revision 与该提交一致，应用、Postgres、Redis 健康，服务保持原启动时间与镜像。
- 复用已有 personal 与 codex/usage-tps；保留先前已提交的计划和暂停同步定义。main/personal 本地定义一致，远端 main 仍含旧同步脚本；P0.2/P0.6 远端部分未完成，未推送或修改 GitHub 设置。
- 功能提交 `f74554702e6d55554342134b68fc54ff4a4ff541`：UsageTps 在共用耗时栏显示整条请求的平均输出速率，首字不扣除；补齐双语文案、缺失及媒体判断、定向组件/表格测试。
- 窄屏验收发现并修复 HelpTooltip 的 fixed 坐标与视口越界；回归用例先红后绿，保留原交互。
- 验证：83 个定向用例、8 个改动文件 lint、前端构建通过；管理员/用户、明暗主题、1440px/390px 页面和键盘提示交互通过，使用本地合成 API 记录。
- 实际范围、来源字段、命令、截图、环境问题及未执行项见 [P0/P1 实施证据](../openspec/changes/add-usage-tps-and-personal-release/implementation-evidence.md)。没有后端、计费、迁移或生产变更。
- 本日志在所选基线缺失，按 AGENTS.md 的非平凡改动回写要求新建；没有补造旧条目。P2 的远端同步与权限验证、P3 发布、P4 部署/回滚留待对应阶段。
