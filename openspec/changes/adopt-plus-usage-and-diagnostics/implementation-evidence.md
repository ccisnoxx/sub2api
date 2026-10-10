# KIN 借鉴 Plus 功能执行证据

- 记录日期：2026-10-09（America/Los_Angeles）。
- 当前续接范围：仅S3.4；历史记录保留，本轮本地页面、归属修复与验证/复核见文末S3.4。
- 当前维护登记位置：从 `origin/main` 准备的 `codex/plus-pricing-s22-evidence`，只归档本 change 与开发日志；同步原指定计划位置及 personal 应用候选。历史登记位置见各阶段记录，应用始终从 personal 出发。
- 当前完成S0.1、S1.1–S1.4本地部分、S2.1–S2.7及S3.1–S3.4；S2旧累计候选真实Personal CI通过，合并、镜像发布、生产部署、S1.5及S3.5以后均未执行。

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

## S2.3 模型目录 DTO 与查询分支

### 基线、候选与保留结果

先读取用户指定 plan/tasks/journal，再核对远端及本地 personal、现有 S0/S1/S2.2 工作树与未提交改动。personal 保持 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，main 保持 `b43a472f4b1bde5983b3bdfa1447cd3622a77540`；应用来源仍为 KlN `v0.2.14-klno.5/c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。

从干净 personal 来源的 S2.2 HEAD `e4083c38fc594ab75ed8c1cedb7adeb9eaf4a1d2` 创建并附加 `codex/plus-catalog-s23`，路径 `/Users/sc/.codex/worktrees/plus-catalog-s23/sub2api-kin`。其中包含已验证价格实现 `e82287300d1b7cc625295c5307ebaa83c707c019`；本轮已验证应用提交为 `49c209a77059d2927f78bbd31c1dd479b509dc6f`，5个源码/测试/正规生成文件。完整身份见[候选清单](evidence/s2.3-candidate.json)。文档归档提交会推进本地 HEAD，后续 CI 仍须绑定真实最终候选 SHA。

复用干净的 main 来源文档工作树 `codex/plus-pricing-s22-evidence` 仅增量登记本 change 和日志，同步指定原计划位置与新应用树；没有使用 main 应用源码替代 personal，也没有合入 main。原 S0 工作树17个未提交应用文件保持原值；S1的54个应用文件及45个原产物、S2.2的6个应用文件及12个原产物校验一致。新应用相对 S0 的6个用量文件差异是 S1 已有演进，不能称本轮改变。原 S1/S2.2 候选和聊天 checkout 的 HEAD、状态均保持原值；三个登记位置各自历史 journal 全文保留。

AGENTS 引用的出站身份约定和 docs/tasks 在可用应用/维护树与本地历史 refs 中仍未找到，本轮继续登记缺失，不补造规则；实现不涉及出站身份。

### 实际行为与责任

- 同一 GET 路由只在 URL 解码后 `view` 恰有一个值且精确为 `catalog` 时返回对象；默认、空、未知、大小写变体及重复参数继续返回旧渠道数组。用户查询参数不切换 JWT 主体，旧数组的模型白名单行为不改。
- 先执行真实 `APIKeyService.GetAvailableGroups`，再读取一次绑定授权组的活跃渠道配置；分组模型使用 `SupportedModels` 有限候选、普通/复合平台筛选及公开请求名 `ModelAllowlist.Allows`。目录按分组保留报价并在平台/模型维度稳定排序/去重，有权空组保留 models=[]。页面卡片聚合留在 S2.4。
- 每个分组的模型枚举与 S2.2 价格 resolver 使用同次克隆渠道配置，避免再读旧展示合成价或共享渠道缓存。只读取一次当前用户倍率，仅嵌入授权组覆盖；成功且无覆盖为 null、明确零为0、读取失败为 unavailable 和参考默认倍率。授权/渠道/价格失败显式返回，不夹带部分目录。
- 显式 DTO 白名单保留上下文/服务档位/分时/effort、UnsupportedComponents 与按次零标签回退规则；缺失和0分开。单价未乘用户/分组倍率，已解析适用倍率单独输出，消费者仅应用一次。offer_key 用分组、渠道、具体平台和小写请求ID派生SHA-256，不输出原始内部渠道ID。
- GET 没有可信媒体/按次请求入口事实，per_request/image/video 配置返回 unsupported_unit，单位unknown；upstream/response_model 依赖真实请求时返回 request_dependent。不改变扣费、结算、调度、数据库或前端。实现字段和算法见[目录合同补充](catalog-contract.md#s23-已实现的目录-dto-与查询分支)。

### 检查、复用与独立复核

| 实际检查 | 结果与证据边界 |
|---|---|
| 新 handler/DTO 与旧接口定向检查 | `go test -tags unit ./internal/handler -run '^(TestAvailableModelCatalog\|TestUserAvailableChannel\|TestFilterUserVisibleGroups\|TestToUserSupportedModels\|TestBuildPlatformSections)' -count=1 -v` 通过；新增13顶层/32含子项PASS，旧helper10顶层/PASS，共23顶层/42PASS。表格中的竖线为Markdown转义，机器命令见JSON；夹具预期的500错误日志保留 |
| 权限与响应验证边界 | 在仓库边界注入夹具，执行真实授权/平台/白名单/价格owner，覆盖选择兼容、公开限制/专属/有效及过期订阅/他人订阅、跨用户、短路、错误、零/未知、参考倍率、单次快照与DTO白名单；不称真实JWT或生产隔离验证 |
| 服务端构造编译 | `go test ./cmd/server -run '^$'` 通过；明确无业务测试运行，保护新增BillingService注入与生成结果可编译 |
| 正规Wire生成 | 固定v0.7.0。首次被工具go.sum缺项阻止；全局GOFLAGS临时modfile又触发loader非module目录错误；按诊断仅给生成器构建传入临时modfile后成功。应用go.mod/go.sum未变化。生成器还重排两项独立构造并省略维护源码中的注释；实际调用与生命周期不变，未手改生成文件 |
| 证据复用 | S2.2同输入的15顶层/22PASS、48组owner对账及既有98顶层/158PASS结果复用；本轮未重跑这些检查。S0/S1仅复用其未变边界，不作为目录权限、真实数据库、浏览器或当前远端CI证据 |
| fresh独立只读复核 | 1次critical_reviewer完成，未确认可操作缺陷；复核实际diff/5文件/原始日志，未重跑测试。派发前后5文件校验一致；其后主代理只修正handler的分支注释，不改生产逻辑，复用行为测试与构造编译 |
| 实际diff与收尾 | 应用5文件、902新增/6删除，未改扣费/调度/前端/迁移/依赖；git diff --check、格式与保留工作树核对通过。三处任务及合同同步、唯一新增S2.3勾选、链接与历史日志保留另见[收尾检查](evidence/s2.3-document-checks.json) |

委派审计 `20261009T155935Z-kin-plus-catalog-s23-f949dc81` 已closed、audit-verify passed、无errors/warnings。验收通过只表示复核交付被接受；模型/effort是Agent TOML配置证据。见[复核记录](evidence/s2.3-reviews.json)、[确定性执行摘要](evidence/s2.3-subagent-digest.md)和[原始日志及复用清单](evidence/s2.3-validation.json)。

本轮未执行真实JWT/后台模式/面板限流运行链、真实数据库/生产报价链、前端构建或浏览器、全量回归、远端CI或上游凭据请求。路由及middleware未改，复杂价格owner输入未变；本轮只实现S2.3，进一步阶段一致性验证与完整页面流程仍由S2.5/S2.6取得实际证据。没有推送、PR、Actions、合并、镜像发布、SSH或生产部署。

**S2.3 已完成本地实现、验证与执行证据。下一项是 S2.4 改造已有可用渠道页面，未自动开始。** S2.5–S2.7及其他阶段保持未执行。


## S2.4 可用渠道页面模型与分组报价目录

本轮仅 S2.4，本地应用 **`b8cf49ca25007cbc23338a9443b71df4c0411155`**；[候选校验](evidence/s2.4-candidate.json)、[检查与原始证据](evidence/s2.4-validation.json)。S2.5、S2.6、S2.7 均保持未执行。

### 应用来源与工作位置

先读取原指定计划、任务和日志，再核对全部6个工作树及远端。personal 仍为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；来源 KlN `.5/c7aacf5d`。S2.3 的 personal 来源候选 `b8db9596977d196f025d79a1f4cf0c3442e5b3d8` 干净且包含 S1/S2.2/S2.3 依赖，从它建立 `/Users/sc/.codex/worktrees/plus-catalog-ui-s24/sub2api-kin` / `codex/plus-catalog-ui-s24`。未用 main 应用源码。

main 来源 `codex/plus-pricing-s22-evidence` 只登记本 change 与日志，并增量同步原指定计划位置。原 S0 17应用文件均匹配；S1、S2.2、S2.3 与聊天维护树的源码、HEAD保持原值。每处原journal全文保留。AGENTS要求的出站约定与docs/tasks仍未找到，沿用前轮缺失记录；本轮不涉及出站身份。

### 用户可见结果与价格边界

- 新客户端 `getCatalog` 显式传单个 `view=catalog`，旧 `getAvailable` 类型、签名和数组调用不变。页面不再并发拼接 `/groups/rates`，仅使用当前目录内嵌倍率。
- 模型按具体平台与大小写无关名称聚合；检索模型/平台/分组/渠道，保留各组报价和空模型组。原生 details/summary 允许键盘展开，详情切分组、服务档，显示来源、订阅/专属、(min,max]整单上下文、缓存单价、参考时点与单位。
- 单价仅乘服务端已解析适用Token倍率一次，并转换USD/token为USD/1M token。服务档策略已在单价中，不另乘Fast/Flex；分时与最终effort单独展示规则且明确未进入表内。没有计算“最低路由价”或完整账单。
- null显示—，明确0显示$0；unsupported_unit/request_dependent/pricing_unavailable展示不同原因。媒体未知单位不猜测；未支持分量明确说明。个人倍率unavailable保留参考默认倍率和刷新提示。
- 刷新先清目录，失败显式错误与重试；用户ID变化同步取消、清筛选及重载，晚到成功/错误需匹配请求序号与主体。卸载取消并使旧响应失效，不延用旧用户报价。

### 实际验证与独立复核

| 检查 | 实际结果与限制 |
|---|---|
| 新合同定向测试 | API/聚合/过滤/一次倍率4通过；组件与页面6通过，共10去重用例。保护零/未知、阶梯/档位、失败重试、晚到成功/错误、用户切换与退出/卸载。首轮4失败为runtime-only测试夹具没有应用JIT define；中间2次模块路径尝试未收集测试；最终只修夹具并重跑受影响2文件 |
| lint/类型/build | 11文件lint通过，夹具变更后2文件lint与最终vue-tsc -b通过；build包含双语完整性3通过、类型与Vite。Node DEP0190、Browserslist过期、混合导入和chunk警告保留；测试自定义message compiler警告仅在夹具，不宣称零warning |
| 浏览器 | `/available-channels`，1440×1100/390×844、中英、明暗4组通过；身份/标题、内容、无overlay、检索、分组/服务档、零与未知、参考倍率失败、空目录、错误与重试，Enter/Space、Escape及焦点恢复。另对2组价格表实际ArrowRight横向滚动与截图，整页无水平溢出 |
| 浏览器证据边界 | Browser plugin not available，复用Playwright 1.62.1/Chromium与合成GET夹具；不称真实JWT/数据库/生产隔离。正常流程零error/warning，每组1次有意HTTP500有独立错误状态与恢复验证。早期原生select键盘提交假设与缺少AppLayout `/keys`夹具的失败保留诊断；最终以selectOption验证报价切换，原生summary键盘另行通过 |
| 未变证据复用 | S2.2的6文件/12产物、S2.3的5文件/14产物均校验一致；复用原价格owner48组对账和原授权/DTO/旧接口成功边界，不重跑Go。新页面证据由本轮获得；S0/S1不扩大为目录权限证明 |
| fresh只读复核 | 1次critical_reviewer未确认可行动缺陷；检查实际源码、DTO、合同与原始日志，未独立执行测试。复核输入校验一致，之后无应用修改；浏览器键盘/双语缺口由主代理实际验收补齐 |
| 最终diff | 仅11前端文件，644新增/100删除；backend/deploy/依赖/锁文件相对S2.3无差异，未改扣费、调度、权限owner或迁移。diff/保留源/文档链接与状态核对见[收尾检查](evidence/s2.4-document-checks.json) |

委派审计 `20261009T164258Z-kin-plus-catalog-ui-s24-61dc32dd` closed、audit-verify passed、无errors/warnings；[确定性执行摘要](evidence/s2.4-subagent-digest.md)。模型/effort是Agent TOML配置证据；验收通过表示复核交付被接受。

**S2.4 已完成本地实现、检查与执行证据。下一项 S2.5 验证权限与报价一致性，未自动开始。** S2.6完整阶段验收仍保持未勾选，本轮页面证据可在输入未变时复用。真实JWT/数据库、全量/浏览器矩阵、远端CI、推送/PR/Actions、合并、镜像发布、SSH和生产部署均未执行。


## S2.5 权限与报价一致性验证

本轮仅 S2.5，验证候选 **`3722c48ccdc0e4468dbc5c6ebb13584624c566d6`**；[候选校验](evidence/s2.5-candidate.json)、[验证与原始证据](evidence/s2.5-validation.json)、[独立复核](evidence/s2.5-reviews.json)、[文档及保留检查](evidence/s2.5-document-checks.json)。S2.6、S2.7和其他尚未开始的阶段保持未执行。

### 基线与工作位置

先读取指定计划、任务和日志，核对远端 personal 仍为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，来源 KlN `.5/c7aacf5d`，main 仍 `b43a472f4`。S2.4工作树开始时干净，HEAD `1696d054c82044fb5aaaf0d4851023f0423846dc`，已验证功能为 `b8cf49ca25007cbc23338a9443b71df4c0411155`。从该完整SHA建立并附加 `/Users/sc/.codex/worktrees/plus-catalog-verify-s25/sub2api-kin`，本地分支 `codex/plus-catalog-verify-s25`；没有使用main应用源码。

原S0的17个应用文件与历史校验值全部匹配，未提交改动保留。S1、S2.2、聊天维护checkout保持原HEAD和源码状态。S2.3/S2.4旧目录在新树准备后已不在磁盘，本会话未调用删除或归档；原Git分支引用及提交完整保留，不声称旧目录仍存在。main来源 `codex/plus-pricing-s22-evidence`仅增量登记文档，并同步指定计划位置及新应用树；各自旧journal全文保留。AGENTS指定的出站约定和docs/tasks仍未找到，本轮不涉及出站身份或账号extra。

### 实际验证与复用

新增一个integration测试文件，复用仓库真实迁移、PG16/Redis harness、生产用户路由注册、JWT鉴权、分组/订阅/渠道/倍率SQL仓库及生产解析器。HTTP请求不注入AuthSubject；本地合成用户使用真实签名JWT，GET变更审计插槽为no-op。测试不调用上游、调度、用量结算或生产数据。

| 边界 | 实际结果 |
|---|---|
| JWT与权限 | A→B→A主体隔离；查询user_id/group_id/admin不切换主体；公开限制、专属组、有效/过期/停用/他人订阅、停用/软删除组、有权空组按真实SQL和授权owner过滤；原始响应不含隐藏渠道名 |
| 旧接口与middleware | 无view、大小写不同、重复view仍是旧数组；两视图均拒绝无效/缺失JWT、密码指纹变化后的旧Token及停用用户；后台模式普通用户403，管理员仍只有自身范围；真实Redis共用按用户桶，旧数组后catalog返回429及Retry-After |
| 报价与计费 | HTTP目录明确3模型和default/priority/flex/ultrafast四档；0/unknown、平台/公开请求名白名单与内部映射边界；个人倍率0.5/0覆盖默认7，仅应用一次；1/100/101/300上下文及普通输入/缓存读取，共64次费用与独立生产ChannelService→ModelPricingResolver→CalculateCostUnified对账一致 |
| HTTP规则 | 精确断言UTC分时、唯一00:00至23:59倍率2及high effort=1.5，费用期望使用响应倍率；不把隐藏规则漏出或缺失误算成完整对账 |
| 新执行结果 | `go test -tags integration ./internal/repository -run '^TestAvailableCatalog(JWTDatabasePermissions\|HTTPBillingParity)$' -count=1 -v`，最终2顶层/11 PASS项；使用DOCKER_HOST、PG16与CI=true禁止harness静默跳过。Go1.27.2；gofmt/diff检查通过 |
| 仍有效证据 | S2.2的8输入/12产物、S2.3的7输入/14产物、S2.4的15输入/25产物均匹配，共30输入/51产物；复用原48组owner对账、定向回归/HTTP/DTO以及页面倍率/类型/build证据，各自保持原覆盖，不称新执行 |
| 失败与修正 | 初次编译夹具误用不存在的SetTokenVersion，尚未运行测试；改用真实密码指纹变化。第二次夹具直接写后台模式绕过进程缓存，预期403得200；改用真正UpdateSettings刷新缓存。生产owner未改。首轮三项测试保护缺口修正后仅重跑原两个定向集成测试，通过 |

实际命令、日志、SHA-256及环境/复用边界见验证清单。Colima开始为停止，本轮启动已有实例；临时PG/Redis/ryuk容器在检查后清理，原三个无关停止容器保持，Colima最终恢复停止。没有安装依赖或扩大为全量验证。

### 独立复核与交付边界

两次fresh只读复核：首轮检查现有S2权限/价格链及本轮测试，未确认生产逻辑缺陷，但确认原始响应泄漏断言、模型/服务档范围断言、HTTP分时/effort字段三处测试保护缺口。主代理修正并定向重跑；第二轮仅复核这些修正及最终日志，三项关闭，未确认新增可操作问题。复核未独立执行测试；验收通过统计指复核交付被接受，不能理解为首轮候选已经无问题。

审计 `20261009T170219Z-kin-plus-catalog-verify-s25-cd7afaef` closed、audit-verify passed；[确定性执行摘要](evidence/s2.5-subagent-digest.md)。最终diff仅新增测试和本阶段文档；生产代码、扣费、调度、迁移及依赖保持S2.4候选内容。

这些是本地真实基础设施的HTTP集成证据，未启动生产服务进程，未使用生产JWT、带会话指纹的绑定变化、上游凭据或实际扣款请求。过期JWT/数据库错误/Redis故障没有新增集成检查；未变错误边界保留既有证据。新集成夹具限定UTC单时段和high effort，非UTC、工作日限制及跨午夜沿用原owner证据；weekdays_only普通bool未单独观察字段省略/null，当前生产DTO明确输出。媒体和依赖真实请求的报价继续unknown，不宣称整张请求账单或实时可用性。S2.6浏览器完整查看流程、S2.7阶段交付、全量/远端CI及发布门禁未运行。本会话没有push/PR/Actions、合并、镜像发布、SSH或生产部署。

**S2.5已完成。下一项S2.6验收模型查看流程，未自动开始。**


## S2.6 模型查看流程验收

本轮仅 S2.6，复用 personal 来源应用 HEAD **`f6e91d55d05ca71332657bd577f223ccb7849cb0`**；功能提交 `b8cf49ca25007cbc23338a9443b71df4c0411155`，S2.5验证提交 `3722c48ccdc0e4468dbc5c6ebb13584624c566d6`。没有新应用源码改动或提交。[候选校验](evidence/s2.6-candidate.json)、[浏览器与复用证据](evidence/s2.6-validation.json)、[独立复核](evidence/s2.6-reviews.json)、[文档及保留检查](evidence/s2.6-document-checks.json)。S2.7和其他尚未开始的阶段保持未执行。

### 基线、工作位置与源码保留

先读取用户指定plan/tasks/journal，核对远端、本地及origin/personal仍为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，来源KlN `v0.2.14-klno.5/c7aacf5d`；main仍为 `b43a472f4`。S2.5应用工作树开始时干净，HEAD `f6e91d55d05ca71332657bd577f223ccb7849cb0`，为S2.5验证后的文档提交，包含S2.4功能/S2.5测试；personal为其祖先。复用 `/Users/sc/.codex/worktrees/plus-catalog-verify-s25/sub2api-kin` 运行应用验收，未使用main应用源码。S2.3/S2.4旧目录已不在磁盘，但原分支/提交仍保留。

原修复工作树的17应用文件与S0校验值一致，未提交应用改动保留；本轮开始/结束核对应用的4380个backend/frontend/来源文件完全一致。main来源 `codex/plus-pricing-s22-evidence`仅登记本change文档及journal，增量同步原指定文档位置和应用树；三处旧journal全文保留。原聊天checkout、S1、S2.2工作树HEAD/文件状态保持原值。AGENTS中指定的出站约定与docs/tasks在相关工作树仍未找到，本轮没有出站身份或账号extra修改。

### 浏览器实际结果

流程为 `/available-channels` → 检索/筛选 → 模型详情 → 分组/档位报价 → 无权限/空目录与恢复。Browser技能未列出，按frontend-testing-debugging使用已有Playwright 1.62.1/Chromium headless shell，无新依赖。地址 `http://127.0.0.1:4186/available-channels`，中文亮色1440×1100、英文暗色390×844两组，各9条流程通过。真实personal前端静态资源配合本地合成API/用户/JWT占位；该浏览器数据不证明后端权限，真实授权及计费证据单独复用S2.5。

| 验收边界 | 新运行结果 |
|---|---|
| 入口与身份 | 正确URL/中英title，有模型内容、目录范围说明；无空白页或框架错误覆盖 |
| 检索/筛选 | 模型大小写、平台、分组、渠道检索；无匹配与有权空组；检索及分组筛选没有额外请求 |
| 详情与报价 | Enter/Space展开，Escape关闭回焦；default/priority与公开/专属offer切换；USD/1M、适用倍率一次、明确$0、分量unknown为—；模型unknown/媒体单位unsupported不伪造单价 |
| 范围收窄 | 刷新等待时旧目录/报价清空；选中专属组移出响应后筛选重置到全部，旧专属报价不可见且不可检索 |
| 倍率失败与拒绝 | 个人倍率失败显示明确默认参考价；403清空现有目录/报价并显示错误；重试恢复当前目录 |
| 空结果及功能关闭 | 无授权组显示空目录，分组列表无残留；功能关闭后入口隐藏，直达页面也只有空目录 |
| 窄屏与视觉 | 390px价格表能用ArrowRight横向滚动，页面本身无横向溢出；12张状态截图，已查看入口、报价与拒绝截图 |
| 控制台 | 每组有意触发1次HTTP403资源错误；正常流程无app error/warning，不把故意403写成零error |

S2.4旧构建目录已不在磁盘，使用当前候选执行 `pnpm exec vite build`重新生成178个静态文件供浏览器使用；已有i18n/类型/单元/lint证据未变化，未重复运行。Vite成功，保留Node DEP0190、Browserslist过期、混合动态/静态导入及chunk大小警告。初次构建命令在应用根目录没有package.json，构建未开始；改到frontend。项目没有Playwright命令，使用已有bundled runtime。浏览器首轮夹具错误地期待Vue null选项的原生value为空，实际是选项文字；改断言selectedIndex=0验证重置状态，两组完整流程通过，没有改生产源码。

### 有效证据复用与独立复核

S2.2的8输入/12产物、S2.3的7输入/14产物、S2.4的15输入/39产物、S2.5的5输入/33产物引用均校验匹配。计数按清单引用，可能跨阶段引用同一文件，不作为去重用例数。复用原权威owner48组对账/定向回归、HTTP/DTO授权、十个前端合同/类型/lint/build、四组中英/桌面/窄屏浏览器与两组价格表键盘，以及S2.5真实JWT/PG16/Redis权限和64组HTTP/生产计费对账；各自保留原覆盖边界，不称本轮新执行。

1次fresh critical_reviewer只读复核稳定候选：先授权后聚合、真实JWT主体/DTO白名单、KIN resolver与同次配置来源、个人倍率/服务档/单位/零/未知及页面刷新/主体切换隔离，未确认可操作问题；不独立执行测试。复核具体覆盖/限制见复核清单。审计 `20261009T190805Z-kin-plus-model-flow-s26-e70c703d` closed、audit-verify passed；[确定性执行摘要](evidence/s2.6-subagent-digest.md)。复核同时登记一项非阻断覆盖限制：`available_model_catalog_test.go:482`先解码成已知DTO再验证字段，不能独立捕获原始HTTP响应中被解码丢弃的额外字段；本轮逐字段检查生产DTO，未确认泄漏。需要补强时直接用通用JSON检查`w.Body`的嵌套字段白名单。未把旧用例描述为完整原始响应泄漏检测。本轮无需源码修复或新持久化/扣费/调度改动。

### 边界与下一项

本轮浏览器由合成响应控制权限收窄、403、空目录、功能关闭及个人倍率失败；不是启动真实后端并接数据库/JWT的浏览器E2E，也没有生产用户、凭据、上游调用或实际扣款。真实后端权限和价格链依靠S2.5未变集成证据及本轮独立复核。未测试其他浏览器，不声明完整浏览器矩阵；媒体或实际路由依赖的报价继续unknown，不承诺整张请求账单、实时健康或最低价路由。

**S2.6已完成。下一项S2.7登记阶段交付，未自动开始。** 未执行全量/远端CI、push/PR/Actions、合并、镜像发布、SSH或生产部署；本轮登记文档保留为未提交改动，固定应用HEAD与原候选引用保持。


## S2.7 阶段交付登记

本轮仅S2.7，最终累计personal候选 **`88156f09fcf980a771a8aab570f0dbaec5de25fb`**；[候选与来源](evidence/s2.7-candidate.json)、[真实门禁与复用](evidence/s2.7-validation.json)、[文档保留检查](evidence/s2.7-document-checks.json)、[交付与回退](delivery.md#s2-阶段交付s27)。[草稿PR #5](https://github.com/ccisnoxx/sub2api/pull/5)保持草稿且未合并，下一项S3.1未自动开始。

### 候选与实际差异

开始/结束personal均`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，KlN来源`.5/c7aacf5d`、main仍`b43a472f4`。从完整S2.5 HEAD `f6e91d55d05ca71332657bd577f223ccb7849cb0`创建并附加 `/Users/sc/.codex/worktrees/verify-personal-ci-plus-catalog-s27/sub2api-kin`，分支`codex/verify-personal-ci-plus-catalog-s27`；只复制已验证的9个S2.6文档，初始提交`aee00ae7716218418ed3cb772e37c1763873fde7`，随后完成下述3文件门禁修正，形成最终SHA。没有使用main应用源码或未合入CI选择方案；原S0的17应用改动及全部旧工作树保留。

候选包含S0版本/TPS说明修复、S1 Responses HTTP/SSE/WS计时与迁移251、S2模型目录，并非只含S2补丁。S2功能增量从S1文档候选`e3edb5666`核对23个应用/测试文件：目录GET/DTO/页面只读；BillingService只增价格来源存在性元数据，实际计算不消费该元数据；上下文探针提取保持旧参数；resolver的渠道快照仅由目录注入，真实计费保留原ChannelService路径。S2未改余额/订阅写入、Token计算、调度、gwpool、重试或新增迁移，Wire仅接入目录所需BillingService。最终相对S2.6的4380文件快照有4377文件完全一致，另外3处是门禁修正，不声称这些文件hash未变化。

### 门禁失败与最小修正

首轮[Personal CI `37981138211`](https://github.com/ccisnoxx/sub2api/actions/runs/37981138211)实际failure：后端`TestAPIContracts/GET_/api/v1/usage_(paginated)`的旧JSON预期遗漏S1已冻结的9字段；生产DTO正确保留历史`timing_version=0`、可空字段null及unknown。只补测试wantJSON，既有费用/Token/首字仍原值。首轮单元失败后integration/recording race步骤skipped，不登记为首轮通过。

golangci-lint另报告S1 WS测试`CloseNow`未显式处理返回值和S2目录平台分支QF1003。只将测试兜底清理写成显式忽略已关闭连接返回值，及平台if/else改等价switch；正常关闭仍require.NoError，不添加lint豁免、不改检查规则。共3文件13新增/3删除，生产改动仅等价平台分支。实际diff/gofmt核对通过；定向API合同1顶层/12 PASS项和目录价格、倍率及WS测试16顶层/23 PASS项全部通过，原48组价格owner对账在该目录测试中重跑。未安装本地golangci-lint，由必需远端门禁验证；没有重复本地完整回归或浏览器。

### 最终真实门禁与绑定

当前personal保护为strict/App 15368 `personal-ready`，管理员也受约束。现行Personal CI需要binding、existing-ci、existing-security、tps、sync-contracts及末尾personal-ready，未合入CI选择优化不生效。首轮明确失败诊断后，定向通过才推送修正的最终SHA，由PR synchronize原生事件触发[最终run `37982040213`](https://github.com/ccisnoxx/sub2api/actions/runs/37982040213)。总计2次普通候选分支push、2次不同SHA原生门禁，0次额外dispatch/同SHA rerun/本地完整gate；既有`codex/verify-personal-ci-*`排除规则避免push事件再起重复CI/Security Scan，CLA Assistant按原政策skipped。

最终所有必要job success：shell、Go单元/集成/recording race、前端、golangci-lint、release helpers、两类安全检查、TPS、同步合同及personal-ready。开始/结束绑定同一候选`88156f09fcf980a771a8aab570f0dbaec5de25fb`与最新personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，API读回要求App 15368 check的head SHA及结论。实际job、初始失败/定向修正和最终完整日志均存清单，不拿旧SHA结果代替最终门禁。

PR仍draft，personal/main未更新；没有候选对应个人发布、Release或镜像digest。发布自动化仅接受personal成功push/dispatch，本轮为独立候选pull_request；原自动化变量和workflow保持。正式合并后的personal完整SHA仍必须获得它自身的成功personal-ready，PR门禁不替代正式发布资格。

### 证据复用与交付边界

初始S2.2–S2.6所有输入和原产物校验匹配；最终catalog_pricing.go因等价风格修正hash变化，其目录价格合同在本轮重跑，未变输入与产物继续按原范围复用。S2.5真实JWT/PG16/Redis/64组HTTP价格owner对账、S2.4/S2.6合成浏览器仍各有证据边界。各阶段独立复核按原行为复用，不冒充本轮fresh复核，本轮无新委派。原始HTTP额外字段白名单的非阻断测试限制、媒体/请求依赖unknown、S1第二批未接入及生产审计/会话绑定/实际上游与扣款未验证继续登记。

S2无新增迁移，但累计候选含S1迁移251；原固定旧源码/扩展schema兼容证据和备份恢复要求继续适用，不能据无S2迁移宣称累计候选无数据变化。未分配版本、发布镜像或连接生产；未来部署需真实备份、固定digest及工具运行树审定。

S2.7结果按main来源维护职责登记，并增量同步原指定计划位置和旧S2.5应用树，各自旧journal全文保留；最终门禁候选保持干净固定HEAD，本轮交付记录在三处登记树未提交，避免登记结果后改变已验证SHA或再次触发完整gate。

**S2.7已完成。下一项S3.1冻结结构化诊断合同，未自动开始。合并、镜像发布和生产部署均未执行。**


## S3.1：结构化诊断合同冻结

本轮仅 S3.1。最新远端/本地 personal 仍为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；应用分析仅使用干净 personal 累计候选 `88156f09fcf980a771a8aab570f0dbaec5de25fb`。[草稿 PR #5](https://github.com/ccisnoxx/sub2api/pull/5) 仍 OPEN/draft、head/base 与 S2.7 一致且未合并；来源 KlN `.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` 未变。main 来源 `plus-pricing-s22-evidence` 只维护文档，增量同步原计划树与 S2.5 记录树；没有替换应用源码或移动任何已有 HEAD。

### 冻结结果与来源检查

[结构化诊断合同](routing-diagnostics-contract.md)明确可选 v1 对象、选择层/原因机器码、入口池观察点、过滤原因白名单和完整/部分/未观察统计。已观察空列表为 `0`，预检查拒绝/列表报错/未观察池为 `NULL`；池是实际分组/平台查询范围内返回的账号行，无法推算全库。Grok/阈值前置过滤及 compact 最终拒绝保留真实入口观察；旧 CandidateCount、TopK、子池、summary 文案和模型可用性补查均不能作为新统计。

按 26 个 personal owner 文件和 6 个既有测试边界对照，确认 legacy/advanced、channel pricing、Grok、compact、gwpool、handler failover、日志归因/恢复 telemetry、队列和用户白名单。冻结每次完整选择评估的新序号、同账号发送的绑定快照、WS turn 清理及不可变副本；传输/身份 producer 不能改写选择事实。诊断仅进入已有管理员详情，不向普通用户列表/详情、导出或对客错误帧开放；不含凭据、原始 body/header、账号列表、成本/利润或内部会话标识。

参考固定 Plus `90da415c62b94c9417d9ce2b72b1507ed22f0303` 的 ERROR_REQUEST_DIAGNOSTICS.md，只借鉴完整 owner 快照与观察/未知原则，按 KIN 现有 owner 适配；来源文件与符号校验值见 [候选证据](evidence/s3.1-candidate.json)。没有修改 producer、持久化/迁移、DTO、页面、调度/重试/扣费或依赖。

### 实际验证与复用边界

- 开工快照覆盖 7 处相关工作树的文件、HEAD/branch、已有未提交改动与三处各自文档；结尾按增量白名单比较，原 S0 的 17 项应用改动、全部既有源文件和其他工作树改动保留。三处 journal 分别保留原文，旧执行证据全文保留，任务只新增 S3.1 完成。
- 3 个 JSON 合同样例核对已观察 0、预检查 NULL 和 WS 部分过滤；字段/整数/机器码、原因加总和范围关系核对；源码符号定位、文档链接和最终 diff 检查通过。结果见 [文档及保留检查](evidence/s3.1-document-checks.json)。这属于文档检查，不是 producer 运行测试。
- S2.7 的 24 个原始证据产物校验值一致，已成功的 [Personal CI 37982040213](https://github.com/ccisnoxx/sub2api/actions/runs/37982040213) 仅按同一 88156f09 候选及原行为边界复用；不证明新诊断实现、NULL 持久化或 WS 快照传递。
- 本轮未运行应用测试/构建、数据库/浏览器或完整门禁，未触发新 CI；未取得新独立代码复核。现有 S3 合同由主代理源码对照及文档自查，实际实现的归属/权限/异步快照独立复核留到 S3 候选与 S3.6。后续 producer 覆盖与运行验收清单在合同中明确，不勾选后续任务。

证据根目录：`/Users/sc/.codex/validation/sub2api-kin/20261009-s31`；保留 baseline.json、reused-evidence.json、PR 元数据、固定 Plus 原文及最终检查产物。本轮文档未提交，应用候选保持干净；未 push/修改 PR、更新 personal、合并、发布镜像、生成 digest 或部署生产。

**S3.1 已完成；下一项 S3.2「接入实际决策 producer」，本轮停止，不自动进入。**


## S3.2 实际决策 producer 接入

本轮仅S3.2。开工与收尾只读确认personal仍`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`、main仍`b43a472f4b1bde5983b3bdfa1447cd3622a77540`；[草稿PR #5](https://github.com/ccisnoxx/sub2api/pull/5)仍OPEN/draft、head `88156f09fcf980a771a8aab570f0dbaec5de25fb`、base personal且未合并。KlN `.5/c7aacf5d`来源保持。核对7处旧工作树31283个已有文件及未提交改动后，从干净S2.7 personal候选创建独立`codex/plus-routing-producer-s32`，应用位于`/Users/sc/.codex/worktrees/plus-routing-producer-s32/sub2api-kin`，本地应用提交`cee1e908261c68880040b740aae8054410e03070`，13个应用/测试文件。main来源维护树仅承担文档登记；原S0的17项应用改动、旧S2.7门禁候选及其他工作树源码全部保留。

### 实际行为与范围

新增v1 `RoutingDiagnostics`及请求/turn内选择owner，由真实分支形成选择层、原因、池、过滤数量/理由及coverage。列表成功返回后、阈值/Grok后置过滤前观察入口；池与过滤均按不同账号ID计一次。实际空列表为`0/0/{}/complete`；预检查拒绝、列表失败及只查单号的sticky未观察池时为NULL，不解析错误summary、不额外查询或重跑有副作用的限制。TopK、子池、未probe、排序及WaitPlan不算过滤；compact stale恢复不算最终排除；信息不全保留partial。

覆盖OpenAI主调度advanced/legacy、公开OpenAI `SelectAccountWithLoadAwareness` legacy、previous/guardian/session/weighted sticky与load balance、订阅/普通子池、渠道定价/阈值/Grok额度/compact fresh与DB复核、proxy第二评估、gwpool重选/终检及图片native→basic。每次真实新评估生成新attempt及全新字段；成功结果、决策、结构化错误和当前请求状态分别深复制，保留`Error()`与`errors.Is/As`，结果及决策字段为`json:"-"`。

实际HTTP/SSE入口复用`setOpsRequestContext`的请求owner。Responses WS在业务ctx、当前Request及gwpool wait写回之间显式绑定同一owner；建连重选保留Turn=NULL，Proxy开始后以连接逻辑turn建新owner。Proxy局部turn重启、同账号重试和后续429换号不能使新turn借用旧快照或序号；已有Ops/计费/安全钩子的局部编号保持。Grok Voice和Realtime预accept选路循环补齐请求owner；Realtime后续音频帧没有新turn producer。未重新选路的新WS turn没有当前诊断，不能冒充观察过池。

其他平台的generic GatewayService、独立兼容或直接返回账号的旧入口、TokenCount尚未接入，保持nil；本轮不宣称全部平台覆盖。provider发送/终态绑定、Ops失败快照/队列预算、持久化/迁移、DTO/API及管理员页面尚未实现，用户错误白名单没有扩展。范围详见[合同补充](routing-diagnostics-contract.md#8-s32-实际-producer-覆盖与验证)。

### 验证、复核与保留

最终候选13文件SHA-256与测试/复核固定输入一致，gofmt和实际diff检查通过。Go `1.27.2 darwin/arm64`在应用树backend内完成：

| 定向范围 | 最终实际结果 | 原始记录 |
|---|---|---|
| producer与既有派生值合同 | 17顶层、59 PASS项、0 FAIL、package pass、exit 0 | `service-final.jsonl` |
| HTTP请求/WS/音频owner、既有归因与native/passthrough回归 | 17顶层、22 PASS项、0 FAIL、package pass、exit 0 | `handler-final.jsonl` |
| 既有advanced/legacy/DB/compact/子池/gwpool/利润调度 | pass、exit 0 | `service-regression-final.log` |
| 核心不可变副本/并发owner race | pass、exit 0 | `core-race-final.log` |
| WS逻辑turn/实际failover/音频请求归属 race | pass、exit 0 | `handler-race-final.log` |

服务合同包括提前NULL/真空池0、Grok原位全过滤、同ID去重及全排除、compact恢复、TopK/子池/WaitPlan部分观察、重复DB复核、新评估/网关池/图片fallback及TokenCount未知。实际本地httptest WS覆盖BeforeRequest前凭据重选与第二turn真实429换号后第三turn清空；Voice/Realtime实际handler三次凭据失败保持attempt=3且无上游发送。使用合成账号与本地假上游，无付费/生产请求。

1次fresh独立只读复核完整diff、新文件、合同和日志，确认并修正4项：重复ID池统计；WS初始及gwpool wait写回丢owner；Proxy重启导致逻辑turn复用；Grok音频入口未保存请求owner。每次修正先检查受影响失败，再执行上述最终范围；最终无确认仍未修复的P0/P1/P2。复核未独立运行Go测试，结论来自原始日志与稳定源码。native/passthrough重试后的第三turn未新增同等E2E诊断断言，当前证据为共享映射源码、scope测试及既有WS回归；日志队列/终态和权限边界留S3.3。委派[执行摘要](evidence/s3.2-subagent-digest.md)已由审计工具close/verify通过。

初次Grok夹具用APIKey代替仅OAuth的team gate并缺UsageLogRepository，修正后定向及最终通过；WS反例最初以第二turn 402触发换号，与既有仅后续429换号合同不符，改用429及正确input后通过。临时zaptest日志辅助在测试前要求间接依赖，改回已安装zap，没有go.mod/go.sum变化；没有盲重试或借测试改变调度。

S3.1合同按原语义复用；S2.7 Personal CI `37982040213`/personal-ready仅继续证明旧88156f09，不作为本次应用门禁。未运行无关完整测试/CI、数据库或浏览器；无新存储、迁移、依赖、页面或workflow改动。AGENTS引用的outbound identity/account extra约定在相关personal文件及Git tree中未找到，本轮不触及对应owner，未借main替代或猜测规则。原始记录根为`/Users/sc/.codex/validation/sub2api-kin/20261009-s32`，完整命令/哈希/早期失败与复核边界见[验证清单](evidence/s3.2-validation.json)。

完成时[文档与保留检查](evidence/s3.2-document-checks.json)核对旧7处工作树、原17项应用改动及旧证据；三处登记树分别追加文档并保留各自旧journal全文。独立应用树同步S2.7/S3.1既有登记与本轮S3.2文档，各自旧历史不覆盖；旧门禁候选保持干净原HEAD。应用与文档仅本地固定，维护/原计划/S2.5登记树的既有未提交文档保持未提交，没有push/修改PR、新CI、更新personal、合并、镜像发布或生产部署。

**S3.2已完成；下一项S3.3「贯通错误存储与DTO」。本轮停止，不自动开始S3.3，S3.5/S3.6也未提前勾选。**

## S3.3 贯通错误存储与 DTO

本轮仅执行S3.3。开工及收尾核对personal仍`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`、KlN来源`v0.2.14-klno.5/c7aacf5d`；草稿PR #5仍OPEN/draft，head `88156f09fcf980a771a8aab570f0dbaec5de25fb`、base personal，未合并。从干净S3.2完整应用/文档候选`d9b06f4fe78a8588282958783d91e9966ff05231`建立独立`codex/plus-routing-storage-s33`，应用位于`/Users/sc/.codex/worktrees/plus-routing-storage-s33/sub2api-kin`。最终应用提交`ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`；main来源维护树只登记文档，未替代personal应用源码。

### 实际贯通范围

- 选定账号后绑定不可变选择快照，发送绑定同时记录request/turn owner。同账号重试保留原选择，真实重选绑定新快照；owner不一致时旧连接绑定无效。同逻辑WS turn的Proxy重启保留新绑定，新turn未选号时为NULL。Voice/Realtime预accept两条独立选路循环也在真实选定账号后绑定；Realtime后续音频帧仍不提供新turn producer。
- 上游失败事件各持所属发送快照，带内错误保存完整自有副本；后续请求修改、重选及WS turn不覆盖旧值。顶层provider/凭据故障取既有归因的最后失败尝试，routing本地终态取所属本次评估；RequestScoped不继承诊断，恢复provider错误仍为原telemetry。没有伪造额外上游事件来保存选择过程，phase/type/owner/source、状态与SLA/skip规则保持。
- 队列前及服务准备边界验证完整v1：版本、9个必需字段、NULL、白名单、精确整数、层/原因与数量/coverage关系。未知码、版本、漏字段、非整数或矛盾关系只丢弃该诊断，并留下不含原始值的固定内部异常信号，真实故障继续记录。typed对象在进入队列前深复制并序列化，队列不持有原map；顶层JSON计入job bytes，事件对象随原JSON序列化计入256事件/512 KiB界限，最近16正文窗口和较早尝试丢弃标记继续沿用。
- 新增`252_add_ops_routing_diagnostics.sql`，只扩展`ops_error_logs.routing_diagnostics` nullable JSONB，无默认、无回填、未改旧迁移/ledger。Ops表由手写SQL/迁移维护，无对应Ent schema，未新增虚假schema或手改生成文件。`OpsInsertErrorLogInput → 队列 → OpsService → 单条/批量SQL第39参数 → OpsErrorLogDetail`贯通；repository只保存经owner准备的JSON，不从typed或当前账号状态重建历史。
- 详情严格解码白名单typed对象。历史SQL NULL及整体JSON null保持nil，已观察池0、过滤0与`{}`保持原值。坏历史对象省略诊断并发出固定`routing_diagnostics_invalid`信号，错误ID/正文仍可读。新对象仅管理员单记录`/ops/errors/:id`与`/ops/request-errors/:id`详情可见；普通列表及`upstream-errors?include_detail=1`列表裁剪顶层和事件内诊断。用户列表/详情白名单不扩展，直接user_id归属检查和admin认证/审计/合规/监控开关不变。

### 验证与兼容

全部选定最终检查退出码0，原始记录位于`/Users/sc/.codex/validation/sub2api-kin/20261009-s33`。具体命令、最终源文件及产物校验值见[验证清单](evidence/s3.3-validation.json)；下表按边界列出，不把交叉运行重复计为总覆盖数量。

| 边界 | 实际结果 | 原始日志 |
|---|---|---|
| 新日志/队列/DTO、请求与WS owner、音频真实凭据失败绑定 | 14顶层/36 PASS项，全部通过 | `ws-owner-fix-final.jsonl` |
| 既有错误归因、RequestScoped/recovered/skip/SLA、HTTP/WS | handler63顶层/129 PASS项通过；新音频断言另行先红后绿 | `handler-regression.jsonl`、`audio-binding-green.jsonl` |
| 服务事件/脱敏/历史规范化及队列/记录回归 | 26顶层/59 PASS项通过 | `service-regression.jsonl` |
| 管理员实际handler单记录/普通列表/include_detail列表及用户归属 | 管理员1顶层5次HTTP读取、用户4顶层通过；不是完整JWT服务器E2E | `admin-detail.jsonl`、`user-ownership.jsonl` |
| 实际单条/批量SQL参数与DTO白名单 | 参数3顶层/6 PASS、DTO2顶层/9 PASS项通过 | `storage/validate-owner-and-args.log`、`storage/dto-unit.log` |
| PG16真实迁移/写读/历史NULL/坏对象/批量原子失败 | 5顶层/12 PASS项通过，PG16.15 | `storage/pg16-integration.log` |
| 新→固定旧d9b06f4→新源码/扩展schema | 真实migration runner检查、旧详情/列表读取及单条/批量写入通过，恢复新源码原诊断保持、3条旧写入SQL NULL，4条故障与252 ledger未变化 | `storage/compat-new-before.log`、`storage/compat-old.log`、`storage/compat-new-after.log` |
| 新深副本及受影响WS/音频owner race | 服务与handler通过；真实WS/音频受影响范围最终通过 | `ws-owner-fix-race.log` |

回退证据只证明上述固定旧源码的migration runner及Ops repository/DTO边界，不等于完整服务器启动、实际生产运行树或未知候选兼容。回退仅回退应用，保留新增列与ledger；部署前仍需按实际环境取得运行树证明、备份及恢复条件。测试容器已移除，Colima恢复停止，原三个停止容器及Docker default context保持；数据备份/原始日志留在本地证据目录，未提交数据库内容。

### 复核、失败诊断与证据复用

1次fresh独立只读复核覆盖全部17个应用/测试文件、主代理及storage实现、冻结合同和原始验证产物。确认初候选首WS turn借用建连诊断的P2，最终owner身份修正及先红后绿/受影响race证据足以关闭；最终未确认剩余P0/P1/P2。候选manifest曾残留初始SHA及两个旧哈希，复核指出后已统一为最终候选并逐项匹配17文件；无需因此重跑测试。复核未修改文件、未独立运行Go测试。未新增native/passthrough完整多turn贯穿队列/真实数据库/管理员HTTP的诊断E2E，真实认证/审计/开关HTTP和页面仍未验收。见[复核记录](evidence/s3.3-reviews.json)；[委派摘要](evidence/s3.3-subagent-digest.md)已由审计工具closed/verify通过。

新增Voice/Realtime断言首轮把3次选择误当3次上游失败：实际两次凭据失败后第三次选择耗尽，修正夹具断言为2条真实事件。随后反例确认这两个独立入口未绑定发送诊断，补齐后每事件attempt 1/2正确、最终selection attempt 3，未增加事件或上游调用。WS建连反例先红确认beginProxy提前更换owner会借旧绑定；发送绑定owner校验修正后通过，覆盖同turn重启与新turn无选号。早期失败和最终成功日志分别保留，不盲重试、不借测试改变调度。

S3.2原13项输入及5份最终产物逐项校验；未变producer/scheduler/builder及计费边界复用原服务producer、调度回归与core race成功证据，改动后的日志/stream/WS/音频边界使用本轮验证。S2.7 Personal CI `37982040213`/personal-ready继续只证明旧88156f09，不能当作本次应用门禁。没有新依赖或生成漂移，计费/调度/重试决策/身份/冷却owner未修改；gofmt、实际diff与文件归属检查通过。

保留检查核对8处开工工作树。7处仍存在的旧树HEAD、应用文件及原未提交工作均保持；S2.5旧目录在核对过程中已移除，本轮工具未执行删除/归档，其分支仍指向原f6e91d55，4567项内容与原Git提交匹配、18份未提交文档有原hash副本，journal已按原SHA-256精确重建。19份原未提交记录另保存于本地`preserved-s25-uncommitted`；没有擅自重建或改写该旧工作树。维护树、原指定计划位置与本轮应用树增量更新任务/证据及各自journal，原旧历史全文保留。

本轮新增文档链接/锚点及JSON清单检查通过。全量文档链接检查发现5处既有S3.1链接与原冒号标题的锚点不匹配，已按开工文档对照分类为历史缺口；未改写旧历史。非文档文件/原HEAD保留检查通过，最终17文件哈希保持。

没有完整Go/CI门禁、浏览器/页面或付费上游/生产请求；没有push/PR更新/workflow dispatch、更新personal、合并、镜像发布或生产部署。HTTP管理员检查为真实handler与存根repository，数据库/SQL链另有真实PG证据；未宣称真实JWT完整服务器或native/passthrough重试后第三turn的新增日志E2E。其他平台、独立旧选择入口、TokenCount及无新选择的连接复用仍保持原未知边界；S3.5/S3.6综合验收与交付未提前勾选。

**S3.3已完成；下一项S3.4「扩展现有错误详情」。本轮停止，未进入页面及下一阶段。**


## S3.4 扩展现有错误详情

本轮仅S3.4。先读取指定plan/tasks/journal，再核对8个现存树的HEAD/分支/未提交文件及来源、远端personal/main、草稿PR #5。personal仍`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，KlN `.5/c7aacf5d`，PR仍旧S2.7 `88156f09`、OPEN/draft/未合并。复用已附加且干净的S3.3 personal工作树，从完整`032db7982970847179f77ec68d33b2f1ea964521`新建`codex/plus-routing-details-s34`；原S3.3分支固定不动，未采用main维护应用源码。应用候选`d23474171035320eda06a35d76d455d7f8d7aae4`，8个前端/测试文件；文档固定后本地HEAD另记。原S0的17项应用改动、其他树既有未提交内容和各自journal保留。

共享`OpsErrorDetailModal`接入`OpsRoutingDiagnosticsPanel`，只读取当前管理员单详情的可选v1快照：真实0保留，NULL/缺失为未知，partial明确已知下界；空原因map与未观察不同，未知schema说明不支持。6个选择层、11个原因、35个过滤码使用双语白名单标签，不回显额外属性或未知原始码；不推导剩余可用池。展示选择评估序号及已知WS逻辑turn，明确不等于发送/切换次数。保留当前详情phase/error_owner/error_source、原根因与诊断载荷；未新增敏感请求或认证字段、普通用户/列表字段、后端迁移或权限分支。Ops上游单详情别名沿用同一个管理员GetErrorLogByID owner，并非关联列表include_detail可见性扩展。

旧详情/关联请求晚到可串记录的反例在改前明确失败；实现按show/errorId/errorType的generation隔离详情及关联列表成功、错误和finally，关闭/切换/卸载使旧结果失效。最终8文件hash固定，候选实现经过fresh只读`critical_reviewer`复核，无确认阻断；复核未重跑测试或验收浏览器。动态测试没有直接断言同ID切类型且旧请求pending、卸载后晚到rejection、旧finally先结束三种组合，源码归属保护已核对，未确认缺陷；不伪称全部生命周期E2E完成。

| 检查 | 实际结果与边界 |
|---|---|
| 定向前端/国际化 | 详情6、面板12、列表6、图表3、管理员Usage15、Ops keys28、全部locale编译5/完整性3，共78项通过；旧响应反例先红后绿 |
| 静态/构建 | 8文件lint、vue-tsc及Vite生产构建退出0；构建产物在仓库外，依赖/lock和受管静态资源未改；既有Node/Browserslist/import/chunk警告登记 |
| 实际页面 | 本地127.0.0.1:4194生产前端；中文亮色1440×1100、英文暗色390×844，各自Usage错误列表四种诊断、Escape关闭、关闭pending后新详情与旧响应晚到、Ops上游详情返回列表通过；身份/非空/无overlay/console健康、首屏及面板截图检查通过 |
| 后端证据复用 | S3.3的17后端输入逐项一致，只按原范围复用发送/终态、队列预算、SQL/PG16/固定旧应用往返、用户白名单及race；旧CI不证明本次前端 |
| 保留和复核 | 三个登记位置各自旧journal/evidence/冻结合同正文保留，最终源hash/工作树与文档差异核对见机器清单；审计closed/verify passed |

浏览器插件本会话未提供，使用已有Playwright 1.62.1/Chromium，无新依赖。浏览器是实际前端配合合成管理员/GET API，不能作为完整JWT服务器或生产权限证明。初始PNPM symlink安全拒绝、runtime-only i18n测试编译器、遗漏后台API与引导层键、返回离场过渡和截图入场动画均按诊断修正验证环境后重跑受影响部分，日志保留；未为此改生产源码。额外观察到既有`/admin/ops?open_error_details=1&error_type=upstream`首次打开列表未发GET；两个owner文件相对S3.3未改，正常卡片入口可加载并完成返回。此范围外问题已登记，未扩展修改或宣称深链接通过。

[候选清单](evidence/s3.4-candidate.json)、[验证与原日志索引](evidence/s3.4-validation.json)、[复核及限制](evidence/s3.4-reviews.json)、[保留/文档检查](evidence/s3.4-document-checks.json)及[委派摘要](evidence/s3.4-subagent-digest.md)。外部原证据目录`/Users/sc/.codex/validation/sub2api-kin/20261009-s34`；应用、维护文档树及用户原指定位置同步登记，main只改文档。

S3.4勾选，下一项**S3.5「验证归属与行为保持」**未开始。S3.5综合账号选择/冷却/切换/扣费及S3.6阶段交付未提前完成；完整JWT后端浏览器E2E、native/passthrough多turn日志E2E、新远端门禁继续未覆盖。本轮无push/PR更新/合并、镜像发布或生产部署。


## S3.5 验证归属与行为保持（部分完成，既有 P1 阻断）

本轮仅S3.5。先读取用户指定plan/tasks/journal，核对8个现存树HEAD、分支、未提交内容及来源；personal仍`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，KIN `.5/c7aacf5d`，草稿PR #5仍旧S2.7 `88156f09`、OPEN/draft/未合并。复用已附加且干净的S3.4 personal应用树，从完整`e3140a68d245d6bb4bddbff509b30395ecaeeb19`新建`codex/plus-routing-behavior-s35`，原S3.4/S3.3分支固定。测试候选`3101cb41636a658dbac47e882ebdc37f34e84641`仅新增HTTP/WS两份测试，不改生产调度、冷却、切换、计费、存储、权限或前端；未使用main维护应用源码。原S0的17项源码改动、其他树未提交内容及各自旧journal保留检查见机器清单。

HTTP驱动真实Grok 402/429、选号/转发/冷却与Ops中间件：首失败属账号801/评估1，恢复成功属802/评估2；后续请求只发802、序号重置且已排队诊断不变。恢复行经过真实flush→RecordErrorBatch→仓库替身，以StatusCode=200留在失败SLA之外；权威查询按status_code≥400计失败，本轮未执行聚合SQL。该夹具是simple模式且usage repository为nil，不声称实际用量落库或数据库扣费。

WS使用实际客户端及上游socket，native/dedicated与passthrough各两连接、每连接三turn：首turn隐藏429换号后客户端可见失败、第二turn成功、第三turn再失败。验证真实stream error→Ops中间件→队列的账号、消息、turn/评估与不可变JSON；第三turn未选号保持NULL。首连接队列保留至次连接完成后才消费。native另覆盖首turn下游尚未输出时同账号socket重建，发送序列、查询次数及6条用量/响应ID归属保持；用量经过真实AfterTurn/RecordUsage/Create。两种夹具均simple模式，标准扣费由独立service owner对照验证。Ops在连接退出才排队；帧内隐藏429未产生upstream error event，不能宣称WS恢复telemetry贯通，HTTP恢复证据单列。

| 检查 | 实际结果与边界 |
|---|---|
| 新HTTP真实流程 | 1顶层/3 PASS，最终退出0；失败发送诊断不被恢复选择或后续请求覆盖，冷却与选号次数符合原行为 |
| 新WS真实流程及race | 2顶层/4 PASS，最终unit-04/race-02退出0，无DATA RACE；通过范围为首turn换号/同账号恢复后多turn和延后队列消费 |
| 旧/新行为对照 | 固定pre-S3 personal `88156f09`与当前使用16个相同测试oracle，各25 PASS；legacy/advanced选择、空池/模型筛尽、sticky、429限次/混合状态、402冷却、取消后停止切换、标准余额/订阅/atomic/legacy费用和去重一致；费用/切换owner及依赖哈希核实 |
| 原证据复用 | 17个S3.3输入、6个producer/调度输入及12份原日志一致；复用S3.2选择前拒绝NULL/观察空池0/三路径筛尽/partial及S3.3发送、队列、SQL/真实PG16/旧应用兼容、admin单记录/用户白名单/race原边界；S3.4前端未变，不重复页面/build |
| 外部后续turn探针 | probe-02自然退出1，确认native第2turn换号误发第1turn载荷；与通过候选分开，完整WS后续换号目标未通过 |
| 独立复核 | fresh只读reviewer确认两份新增测试通过边界有效、无新增候选缺陷，并确认既有P1；未独立重跑测试，统计元数据及外部夹具问题关闭 |

**既有P1：native后续turn换号误重放首包。**第1turn完成后，第2turn在下游尚未输出时收到429；实际发送`93501/turn-1成功→93501/turn-2 429→93502/turn-1误重放`。客户端期望`resp_session-1-turn-2`却收到`resp_session-1-turn-1`；真实失败snapshot/排队行属逻辑turn2、账号93502、诊断turn2/评估1，消息却来自旧turn1请求。native forwarder `openai_ws_forwarder_ingress.go:1918`返回finalErr未携带当前payload，handler `openai_gateway_handler.go:3260`获取不到包装而落到`:4039`首包fallback。pre-S3整个forwarder文件及三个关键分支与当前逐字相同，故按源码证据归类既有；仅当前运行时复现，旧WS运行时未执行。修复方向为native失败出口携带安全重放的当前turn载荷、沿用HTTP bridge安全判断；无法安全重放时显式终止。生产修复另行执行，本轮保存最小overlay/原日志和开放风险。

HTTP首轮失败为手动队列读取不扣原子计数，WS早期失败为握手/turn及gwpool查询夹具期望；只修自有测试后重跑。probe-01同类收尾阻塞已终止，仅外部overlay改有界消费后probe-02得到上述真实失败。differential顶层统计误用Package斜杠已按Test字段修正，两边原日志不变、未重跑。没有无依据重试或全量门禁。

passthrough后续turn换号、后续turn同账号重建、新完整JWT服务器/后台数据库消费/本次SLA聚合、旧native动态复现仍未覆盖。其他平台、独立旧入口、TokenCount和无新选择连接复用保持原未知；既有Ops深链接首次列表问题不在本轮修改范围。没有全量Go/新CI、付费上游/生产请求、push/PR更新/workflow dispatch、合并、镜像发布或生产部署。临时socket随检查退出，未启动常驻服务或改变容器/VPS；固定旧源码导出及复现夹具保留于外部证据目录。

[候选](evidence/s3.5-candidate.json)、[验证与原日志](evidence/s3.5-validation.json)、[复核/开放P1](evidence/s3.5-reviews.json)、[文档/保留检查](evidence/s3.5-document-checks.json)、[委派摘要](evidence/s3.5-subagent-digest.md)。外部原证据目录`/Users/sc/.codex/validation/sub2api-kin/20261009-s35`；两处现存登记位置各自旧正文保留。子任务交付验收与审计closed/verify通过不代表S3.5整体通过。

**S3.5保持未勾选。下一步先修复既有native后续turn换号payload错误并复验S3.5，再进入S3.6；本轮停止，不自动开始生产修复或下一阶段。**

收尾发现开工时的五个历史工作树已移出磁盘和Git worktree列表；本轮没有删除/归档调用，移除来源未由本轮确认。相关分支仍在，按Codex snapshot对照原文件哈希，五树共22527个文件全部一致，包含原维护树未提交文档；未重建目录或改写snapshot。原维护文档位置已不存在，本轮只同步现存应用树和用户原指定目录。
