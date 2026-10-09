# KIN 借鉴 Plus 功能执行证据

- 记录日期：2026-10-09（America/Los_Angeles）。
- 当前续接范围：仅 S2.2；[任务清单](tasks.md)和[计划](plan.md)。下方 S0.1、S1、S2.1 保留当时记录，本轮价格解析结果见文末 S2.2。
- 当前维护登记位置：从 `origin/main` 准备的 `codex/plus-pricing-s22-evidence`，只归档本 change 与开发日志；同步原指定计划位置及 personal 应用候选。历史登记位置见各阶段记录，应用始终从 personal 出发。
- 当前完成 S0.1、S1.1–S1.4 本地部分、S2.1 合同及 S2.2 价格服务本地实现与验证；远端 CI、合并、镜像发布、生产部署、S0.2/S0.3、S1.5、S2.3 及后续任务均未执行。

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


## S1.1–S1.4 第一批本地实施与交付准备

### 基线、保存原候选与稳定身份

本次先读取指定 plan/tasks/journal，读取 personal 约定、相关领域/用量/计费、迁移、DTO、页面与导出合同，再核对工作树与候选。只读远端复查在 UTC 2026-10-09 14:19:42 完成：personal 仍为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，main 为 `b43a472f4b1bde5983b3bdfa1447cd3622a77540`。`deploy/personal-source.json` 保持 KlN `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`；Plus 参考仍为固定 `90da415c62b94c9417d9ce2b72b1507ed22f0303`。

| 对象 | 实际状态 |
|---|---|
| 工作位置 | `/Users/sc/.codex/worktrees/plus-usage-s1/sub2api-kin`，`codex/plus-usage-s1`，工具创建并附加的独立 personal 工作树 |
| 原 S0 候选 | 原 tracked diff 与 untracked change 文档按字节复制，再在新树本地提交 `6bf78b18ce759e4f211c62b0518c3bca7418efde`；原17个应用文件未修改 |
| 数据/迁移 | `0e2ea47950564aa05dd6e8eca374874725047057`，包含主代理冻结的 `UsageTiming` 与 spec；Ent 由正规 generator 维护 |
| 页面/导出 | `12499e1a7`，15文件；后续说明修正在下列独立补丁 |
| HTTP/SSE/WS采集 | `f9d9fbbd7ce6048b3b18be46a55f964a8db4b34e`，19文件；后续复核修正 `36ba3360d6c0839af1593d017e729ff9b0fdaba5` |
| 最终验证应用 | `3f04437572e2819f0313ccc2a3f1a618a2afcdf0`，最终音频事实修正后再通过受影响采集与race；相对已保存S0新增/修改54个应用文件 |
| 当前聊天 checkout | `codex/ci-validation-scope` / `7949efe62e3854c83f39a21a0c36e03402c05688` 继续干净，没有借用其 main 应用源码或未合入CI选择候选 |
| 文档职责 | 新应用树保存阶段记录；原指定位置与 main 来源的既有文档候选增量同步。各自既有journal全文保留，未合入远端main |

[候选清单](evidence/s1.1-4-candidate.json)固定最终应用完整 SHA、54文件 SHA-256、构建/依赖输入和来源。该 SHA 是已验证应用提交；文档归档提交会推进本地分支 HEAD，后续 CI 必须重新绑定届时的实际完整候选 HEAD，不能把旧 SHA 的结果改标成新 SHA。

AGENTS 引用的 `docs/conventions/codex-outbound-identity.md` 与 `docs/tasks/` 在当前 personal 与 main 来源树中缺失；已搜索相关本地/历史来源，没有补造规则。本期未修改出站身份、指纹、账号调度、gwpool、自动重试、现有计费解析与结算 owner。`openai_ttft_mode`、旧 `first_token_ms`、旧迁移/聚合均保留。

### 数据与实际入口

- 新增 `251_add_usage_log_timing.sql`，只追加9项观测字段及注释；事务内 lock_timeout=5s，历史值为0/NULL/unknown，不回填成功、不清理数据。原SQL文件 SHA-256 为 `abce11c5a8cf8fa993d9a0fb25190af3130722fcc419f1f94b5a5397ff60003b`；runner 对 TrimSpace 后内容计算 ledger checksum，两者不能混用。
- 领域、Ent schema/8个生成文件、6组显式SQL列/参数、扫描、单条/批量/幂等/best-effort fallback、用户与管理员DTO及前端类型贯通。规范化仅处理空状态/source→unknown，不猜未知枚举。go.mod/前端锁文件无变更；go.sum只追加8行正规generator依赖checksum，无版本升级。
- [冻结合同](spec.md)与[覆盖表](coverage.md)分别说明数据和实际owner。第一批原生 Responses 普通/透传 HTTP/SSE、非流式JSON/SSE转JSON、pooled WS/ingress/WS→HTTP桥接、原生WS relay已接线；用原duration起点与权威turn/ID，提交深复制快照。
- 首次终态冻结；取消/完成、failure/DONE、多turn、重试attempt隔离均有定向反例。流式只有done聚合不补造严格时点；非流式是完整内容观察时点。可信正音频统计也保留持续音频事实；权威缺失拆分保持NULL，明确零才为0，拒绝的全零终态保留原部分值。
- 页面与导出使用同一helper。平均TPS使用总耗时及可信文本/媒体拆分，保留原始未舍入值；历史旧首字、新严格首Token与部分状态分开说明。所有新行说明非流式观察边界，避免pooled WS旧DTO没有原请求模式时漏说明；未改变旧request_type。

### 实际验证与复用

完整实际命令、退出、原始日志SHA-256、浏览器结果和环境事件见[验证登记](evidence/s1.1-4-validation.json)。原始输出已持久保存至 `/Users/sc/.codex/validation/sub2api-kin/20261009-s1`，可由JSON清单核验，不只保存在/tmp。原始green日志自身没有记录筛选命令/HEAD，登记中的命令及退出来自本次实际工具调用记录；不把日志单独视为这两项元数据证明。

| 检查 | 结果与范围 |
|---|---|
| Ent generator / repository unit / DTO | 正规生成通过；55个repository顶层定向检查、13个DTO顶层检查通过。新夹具时序调整后只重跑受影响范围 |
| PostgreSQL 16集成 | 7个顶层检查通过，后续仅4个新字段夹具重跑；真实单条/批量/重复写、Ent、历史未知、真实FK引发best-effort失败后single fallback、新字段读回。不是skip或纯mock |
| 旧应用与恢复往返 | 固定6bf78b18c旧源码执行真实ApplyMigrations、SQL/DTO；真实pg_dump/pg_restore隔离库；修checksum夹具后同库S1→S0→S1启动、迁移与新SQL/双DTO读回通过 |
| 最终Responses采集 / 计费快照 | service与WS relay定向集合对最终3f0443757通过；覆盖HTTP六种组合、取消/失败、重试隔离、WS owner、多ID/两turn、legacy/atomic×balance/subscription费用/倍率/扣费次数/原command保持、可空字段深复制 |
| 既有回归 / race | 原失败、lease、多turn、bridge retry与计费幂等通过并复用；36ba修正的定向race两包通过；3f最终正音频事实与WS归属race通过。未运行完整Go suite |
| 前端定向 / lint / build | 7相关测试文件合计162个去重用例通过，含最终首Token组件6项；15个改动文件lint及后续2文件lint通过；最终i18n3项、vue-tsc、Vite通过。既有Node/Browserslist/import/chunk警告仍登记 |
| 页面与真实导出 | 合成API的4组Chromium配置：管理员/用户、zh/en、light/dark、1440/390；说明点击/Enter/Space/Escape、互斥、外部关闭、视口边界；实际四份CSV/Excel共32行解析，原始TPS/空值/bool/旧列语义一致 |
| 最终说明布局 | 更新条件说明后仅重跑四组tooltip流程，无error/warning、无裁切；未改导出helper，先前32行一致性证据复用 |
| S0证据 | 版本service/handler、VersionBadge/store等文件保持原内容，沿用S0.1仍有效证据；变化的TPS/表格由本期验证替换，未把S0成功结果套给新功能 |

首次新增首Token组件用例的detached DOM断言失败，修挂载后重跑；浏览器精确文本、窄屏focus滚动和外部点击目标的夹具问题已诊断修正，只重跑失败mobile。新复核反例先红后绿；shell首轮测试直接解引用NULL产生panic，改NotNil断言后证实同一遗漏。共享定向集合还发现纯文本零音频在替换后被清空，调整接受后确认顺序后通过。用户暂停后预览进程结束，继续时连接拒绝；核对端口无进程后重新启动，本次补验通过。这些结果没有被写成首轮全通过，也没有盲重试。

### 独立复核与交付限制

4次fresh独立只读复核的实际候选与逐项关闭见[复核登记](evidence/s1.1-4-reviews.json)：存储确认临时checksum夹具缺陷已以真实往返关闭；采集/导出确认四类P2指标边界已修；进一步确认usage-only正音频事实分支已补先红后绿回归。最后20行按实际3f0443757独立复核，可关闭，未确认残余阻断。独立复核未单独执行测试，不等于独立测试。

本任务审计 `20261009T092527Z-kin-plus-usage-s1-c8034a11` 已closed、audit-verify passed，34产物完整校验，无errors/warnings。[确定性子代理摘要](evidence/s1.1-4-subagent-digest.md)来自当前5个stage的校验结果；模型/effort只表示Agent TOML配置，未冒充运行时遥测。

实际未覆盖边界：HTTP原失败仍返回nil、不新增失败用量行；WS缺少可信turn/ID/start的fallback、opaque binary保留旧版本或未知；Cyber、CC/Anthropic/Gemini/Grok转换及其他媒体API未接入。没有真实provider凭据请求、生产/真实用户授权隔离或跨浏览器矩阵。第一批不能宣布全部平台完成。

[交付准备](delivery.md)已记录增量迁移、备份/恢复、固定旧应用兼容边界及保留扩展schema的回退策略。personal当前必要CI仍是binding、existing-ci、existing-security、tps、sync-contracts及personal-ready；当前聊天的CI范围候选未生效，未借用。远端CI/安全/发布gate未执行，需后续推送/PR授权并绑定实际最终候选；部署工具的真实运行树兼容仍须针对实际生产候选审定。本会话没有推送、PR、Actions dispatch、合并、版本分配、镜像发布、SSH或生产部署，没有把历史tps.1当本候选证据。

**S1.1–S1.4 已完成本地实现、验证与交付准备。** S1.5、S2–S5不自动开始；按计划推荐下一项为 **S2.1 模型价格目录的权限与接口合同**，S1.5第二批采集按实际使用入口另行选择。合并、发布和生产验收保持独立未执行状态。

## S2.1 模型与价格目录权限与接口合同

### 基线与工作位置

- 先读取指定 plan/tasks/journal，再只读查询远端 `personal/main`。远端 personal、本地 personal 与 origin/personal 均为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；远端 main 与 origin/main 均为 `b43a472f4b1bde5983b3bdfa1447cd3622a77540`。未 fetch、push 或修改远端。
- 当前聊天仍为干净的 CI 维护 checkout `7949efe62e3854c83f39a21a0c36e03402c05688`，未用其应用源码。原 `fix-version-usage-help` 基线仍为 `9397eb8af`，17 个未提交应用文件保留。
- 本轮核对应用使用干净的 `/Users/sc/.codex/worktrees/plus-usage-s1/sub2api-kin`，HEAD `e3edb5666a03a582bfbb83a718aedda03ba6ea08`，包含 S1 已验证应用 `3f04437572e2819f0313ccc2a3f1a618a2afcdf0`；personal 是其祖先。与 personal 比较的九个核心渠道/授权/客户端文件无差异，S1 不改变本次旧接口基线。
- 文档沿用 `/Users/sc/.codex/worktrees/plus-s01-evidence/sub2api-kin`，只增量编辑本 change 与 journal，并同步用户指定的原计划目录。两处 plan/tasks/implementation-evidence 编辑前逐字一致，日志各自旧内容保留。S1 源码工作树、HEAD 和历史应用证据不变。

### 合同结果

[目录权限与接口合同](catalog-contract.md)已冻结：

1. 同一路由只有单个、大小写精确的 `view=catalog` 选择新对象；缺失、空、未知、重复值仍返回旧数组。成功封装、原客户端类型、认证/后台模式/限流不变。
2. 分组授权复用 GetAvailableGroups；公开限制、专属授权、有效订阅独立处理，订阅组必须有 active 且未过期的本人订阅。用户参数和管理员身份不能扩大目录范围。
3. 先授权分组，再关联 active 绑定渠道，再做普通/复合平台隔离与模型筛选。有权无模型组保留 models=[]；全局价格不扩增模型。新目录额外复用 KIN 现有 Group.ModelAllowlist.Allows；旧数组本轮不修改。
4. 冻结目录外层、分组/offer 基本字段、稳定身份、白名单与空值；个人倍率读取失败与授权失败分开。旧展示价格包含仅供展示的全局合成，不当作实际扣费报价。
5. S2.2 冻结并接入 KIN 权威价格解析，S2.3 实现目录 DTO 和查询分支。S2.1 未提前新增 resolver、API、前端、迁移或测试代码。

### 本轮检查与未验证项

源码核对覆盖路由/JWT/后台模式、设置读取、分组授权/订阅 SQL、渠道状态/平台/模型枚举、分组模型白名单、旧 DTO 与客户端；关键源文件的实际 SHA-256、Git 基线和检查结果见[检查清单](evidence/s2.1-contract-checks.json)。既有权限、平台、白名单与错误传播用例只作覆盖定位，没有宣称本轮运行通过。

文档链接、任务编号、唯一勾选变化、空白/代码块、两处同步与历史日志保留均检查；四个工作树的 HEAD、既有17个应用改动和关键源文件校验值不变，S1 应用树与当前 CI checkout 保持干净。使用临时快照核验保留内容，持久证据只登记实际结果与必要校验值，不保存无关日志或凭据。

依计划的文档阶段规则未运行应用构建、Go/前端测试、数据库、浏览器或完整门禁；S1 成功证据没有扩大为目录验证。本轮没有新委派或独立代码复核。当前接口未实现 catalog；新分支、跨用户权限、倍率失败状态与权威报价的一致性由 S2.3/S2.5 的实际候选验证。

**S2.1 已完成。下一项为 S2.2 接入 KIN 权威价格解析；未自动开始。** 合并、镜像发布与生产部署均未执行。


## S2.2 KIN 权威价格解析

### 基线、工作位置与保留结果

先读取用户指定 plan/tasks/journal，核对当前 personal、S0/S1 候选与未提交内容。远端、本地及 origin/personal 均为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；main 为 `b43a472f4b1bde5983b3bdfa1447cd3622a77540`。应用从干净 S1 候选 `e3edb5666a03a582bfbb83a718aedda03ba6ea08` 创建 `codex/plus-pricing-s22`，位于 `/Users/sc/.codex/worktrees/plus-pricing-s22/sub2api-kin`。S1 已验证应用 `3f04437572e2819f0313ccc2a3f1a618a2afcdf0` 是其祖先，来源继续为 KlN `.5/c7aacf5d`；没有使用 main 应用源码。

原 S0 工作树的17个未提交应用文件逐一匹配原 manifest；S1 的54个应用文件、45个原始验证产物校验值均匹配。原 S0 应用文件及 HEAD、S1 工作树与 HEAD、聊天 CI checkout `7949efe62` 均保留。此前 `plus-s01-evidence` 在本轮文档写入前已不在磁盘，未由本会话删除；指定原计划目录保存最新 S2.1。因此新建 main 来源 `codex/plus-pricing-s22-evidence` 仅登记文档，各位置旧 journal 原文保留。

最终已验证应用提交 `e82287300d1b7cc625295c5307ebaa83c707c019`，6个服务源码/测试文件；[候选清单](evidence/s2.2-candidate.json)登记真实 commit、tree、来源及文件/依赖校验值。其后的文档归档提交不替代这个应用身份。完整行为与 S2.3 接线要求见[价格合同](pricing-contract.md)。

### 实现与计费责任

新增 CatalogPricingResolver 消费已授权活跃分组和绑定渠道快照，复用 KIN 平台索引、精确/通配/模型归一化、requested/channel_mapped 及 Group → Channel → LiteLLM → 内置来源链。构造检查不代替权限；模型枚举、白名单、聚合与 DTO 仍属于 S2.3。upstream/response_model 与无显式价卡的 Composite 不伪造最终模型报价。

Token 与按请求次数价格从真实计费探针求值，固定 ReferenceAt，输出 (min,max] 有效上下文段、服务档位、FreeFast、独立分时与最终 effort 规则。个人倍率覆盖分组默认值（含0），订阅高峰及独立图片/视频倍率复用实际 owner；各因素仅应用一次。未知与明确零分别保留 nil/0，来源存在性随同次价格读取传递，不因价格刷新混用版本。

按次合法重叠区间按真实首次命中拆段；零标签表示随请求上下文继续求价，FallsBackToContext=true、Price=nil，引用有效上下文与默认价。只有明确 UsageKind=request 才输出 USD/request；媒体按张/秒/分钟/字符和图片 Token 单价没有可靠完整入口，明确 unsupported_unit/UnsupportedComponents。resolved 的 reason 为空，后续 DTO 映射 null；所有规则为参考，不能承诺未来完整账单。

旧 ModelPricingResolver 构造、旧模型广场和计费公式/结算/调度保持原行为。新增元数据不序列化；未新增 HTTP、DTO、前端或迁移代码，不直接序列化 Group/Channel 等领域对象。

### 实际验证与独立复核

- backend 的实际 Go toolchain 为 go1.27.2 darwin/arm64。首次编译发现包内 int max 和原测试访问 concrete channelService；分别改用 math.Max、保留原字段并增加私有快照来源，编译错误消除。失败日志保留，不记为测试通过。
- 最终 `go test -tags unit ./internal/service -run '^TestCatalog' -count=1 -v` 通过：15个顶层、22个含子用例 PASS 项；其中48组服务档位/上下文/缓存输入与生产 ChannelService 和独立 ModelPricingResolver 逐项对账。
- 受影响既有 resolver、阶梯、Token/按次、缓存零价、动态价格解析及旧模型广场定向回归通过：98个顶层、158个含子用例 PASS 项。来源元数据修正后重新执行；最后仅目录零标签表示改动，其 owner 输入未变，复用最终通过结果。已有夹具 warning 如实保留。
- 2次 fresh critical_reviewer 只读实际 diff 与日志；4项 P2 已修正：重叠按次区间、刷新时零值存在性、成功 reason、零标签跨上下文。四项都有针对合同的先红后绿证据，零标签覆盖默认0/0.25和0/1/100/101/300 Token。第二轮明确认可回退标记方向；最后小修由主代理按该方向实施并对账，没有第三次 fresh 全文复核。复核代理未独立运行测试。
- 委派审计 `20261009T151715Z-kin-plus-pricing-s22-ef4e4438` 已 closed、audit-verify passed；计划/执行记录均校验，源码写入观测为零。见[复核与关闭记录](evidence/s2.2-reviews.json)、[生成执行摘要](evidence/s2.2-subagent-digest.md)。验收通过计数表示复核交付被接受，不表示含问题的早期候选被接受。
- S0/S1 输入和原验证产物校验成功，仅复用它们原有边界；不扩大为新目录权限验证。详见[验证记录与日志校验](evidence/s2.2-validation.json)。文档链接、唯一勾选变化、三处计划同步、旧日志保留、最终 diff 与工作树检查登记在[文档收尾检查](evidence/s2.2-document-checks.json)。

没有运行新 HTTP/权限/DTO、前端/浏览器、数据库/迁移、完整回归、远端 CI 或真实上游请求：本轮只改只读价格服务，对应新目录行为由 S2.3–S2.6 的实际候选取得证据。未执行推送、PR、合并、镜像发布或生产部署。

**S2.2 已完成。下一项是 S2.3 实现模型聚合 DTO 与 opt-in 查询接线，未自动开始。** 当前接口仍返回旧渠道数组，目录功能尚未对用户启用。
