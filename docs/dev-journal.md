# 开发日志

## 2026-10-09：KIN 借鉴 Plus S3.3 错误存储与 DTO 贯通完成

- 本轮仅S3.3。personal仍9397eb8af，草稿PR #5仍旧S2.7候选88156f09且未合并；从干净personal来源S3.2完整候选d9b06f4建立独立`codex/plus-routing-storage-s33`，应用`ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`。main维护树只登记文档，没有替代personal应用源码。
- 发送绑定记录不可变诊断与request/turn owner，WS跨owner旧绑定失效；同turn重启/同账号重试保持。上游各事件保存所属尝试，provider最终错误取最后实际失败，routing取本次评估，RequestScoped保持nil。Voice/Realtime两次真实凭据失败绑定补齐，没有伪造最终选择耗尽的第三次发送。
- 严格v1校验及队列自有JSON/字节预算，nullable JSONB迁移252、单条/批量SQL第39参数、严格读回及admin单详情DTO贯通。历史NULL与观察0分开，坏诊断仅丢对象而真实故障保留；普通/include_detail列表裁剪新对象，用户白名单与归属不扩展。
- 最终owner定向14顶层/36 PASS及两包race通过；既有handler63顶层/129 PASS、service26顶层/59 PASS按未变边界复用，admin5次handler读取、用户4顶层、SQL/DTO、真实PG16及固定旧d9b06f4源码往返均通过。复用S3.2未变producer/调度/core race；S2.7 CI只证明旧候选。fresh只读复核首turn继承建连诊断P2关闭，最终无确认残留；复核未独立执行测试，完整JWT服务器/native-passthrough多turn日志E2E/页面未执行。
- [任务证据S3.3](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s33-贯通错误存储与-dto)、[验证](../openspec/changes/adopt-plus-usage-and-diagnostics/evidence/s3.3-validation.json)、[复核](../openspec/changes/adopt-plus-usage-and-diagnostics/evidence/s3.3-reviews.json)及closed/verify通过的[委派摘要](../openspec/changes/adopt-plus-usage-and-diagnostics/evidence/s3.3-subagent-digest.md)已登记。三处各自旧journal全文保留，原S0的17项应用改动及其他现存树HEAD/源码不变。S2.5旧目录核对中已移除，本轮工具未删除/归档；原分支及19份未提交记录按开工哈希保存在本地证据目录。
- 临时容器已清理，Colima恢复停止。S3.3勾选，下一项S3.4「扩展现有错误详情」，未自动开始；S3.5/S3.6不提前完成。没有新完整Go/CI/浏览器/付费上游或生产请求，没有push/PR更新/合并/镜像发布/生产部署。

## 2026-10-09：KIN 借鉴 Plus S3.2 实际决策 producer 接入完成

- 本轮仅S3.2。最新personal仍9397eb8af，草稿PR #5仍S2.7候选88156f09且未合并；从personal来源建立独立`codex/plus-routing-producer-s32`，应用提交`cee1e908261c68880040b740aae8054410e03070`。main维护树只登记，原S0的17项应用改动及全部旧工作树保留。
- 接入OpenAI主调度advanced/legacy及公开LoadAwareness、渠道/Grok/阈值/compact/DB、sticky/子池、proxy第二轮/gwpool/图片fallback真实快照。入口池与过滤按不同ID计一次，保持0/NULL/partial；完整新评估、深副本及结构化错误保持原错误链。HTTP/SSE请求、Responses WS建连/逻辑turn、Voice/Realtime预accept重选owner接入；未接入平台/独立旧入口/TokenCount保持nil。
- 最终服务17顶层/59 PASS、handler17顶层/22 PASS、既有调度回归及core/handler race通过，13文件格式/hash/diff检查通过。fresh只读复核四项问题全部关闭；复核未独立运行测试，native/passthrough后续第三turn仍用共享映射/scope及既有回归，HTTP bridge有新增实际WS反例。夹具早期Grok/WS失败原因与修正记录完整，无依赖变更。
- 已更新[任务证据S3.2](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s32-实际决策-producer-接入)、合同覆盖补充、验证/保留清单及已关闭验证的[委派摘要](../openspec/changes/adopt-plus-usage-and-diagnostics/evidence/s3.2-subagent-digest.md)。旧执行证据与各树journal全文保留；本地应用树同步既有S2.7/S3.1登记，其他树原源码不改。S2.7旧CI只证明旧候选，没有新完整门禁/DB/浏览器或付费上游。
- S3.2勾选；下一项S3.3「贯通错误存储与DTO」，未自动开始。发送/终态和Ops队列绑定、持久化/迁移、DTO/API/页面未实施；S3.5/S3.6未提前勾选。没有push/修改PR、更新personal、合并、镜像发布或生产部署。

## 2026-10-09：KIN 借鉴 Plus S2.6 模型查看流程验收完成

- 本轮仅S2.6。personal仍`9397eb8af`、KlN `.5/c7aacf5d`；复用干净S2.5 personal来源工作树HEAD `f6e91d55d05ca71332657bd577f223ccb7849cb0`（S2.4功能`b8cf49ca2`、S2.5验证`3722c48cc`），未用main应用源码。原S0的17应用文件及应用4380个源文件校验保持，未改生产代码/扣费/调度/迁移/依赖。
- 本地Chromium/Playwright 1.62.1：中文亮1440、英文暗390两组各9流程通过，检索/详情/分组档位报价/0与unknown、刷新后权限收窄/403清空/空目录/关闭入口/重试及键盘窄屏均通过。每组1次故意403资源错误，其他无error/warning。使用合成API/用户，不把浏览器夹具当真实后端权限证明。
- S2.2–S2.5输入与原产物校验后按原边界复用，包含真实JWT/PG16/Redis权限及64组HTTP/计费对账；S2.4旧构建目录缺失，仅重建Vite静态资源，成功并保留既有警告。首轮工作目录/Playwright命令环境和null选项夹具断言错误均记录修正；生产源码未改。
- 1次fresh只读复核可见范围与价格来源，未确认问题，未独立运行测试；审计 `20261009T190805Z-kin-plus-model-flow-s26-e70c703d` closed/verify passed。已更新[执行证据S2.6](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s26-模型查看流程验收)及任务状态，三个登记位置各自旧journal全文保留。
- S2.6勾选；下一项S2.7未自动开始。未运行全量/浏览器矩阵/远端CI，不推送、不合并、不发布镜像、不部署生产；本轮文档未提交，应用和原候选HEAD保持。

## 2026-10-09：KIN 借鉴 Plus S2.5 权限与报价验证完成

- 本轮仅S2.5。personal仍`9397eb8af`、来源KlN `.5/c7aacf5d`；从干净S2.4 `1696d054c`创建personal来源 `codex/plus-catalog-verify-s25`，验证候选`3722c48ccdc0e4468dbc5c6ebb13584624c566d6`，只新增integration合同测试，未改生产源码/扣费/调度/迁移/依赖。原S0的17应用文件校验一致；旧S2.3/S2.4目录后续不可见但Git分支/提交保留，本会话未执行删除或归档。
- 真实PG16/Redis、JWT/生产路由/SQL仓库验证A→B→A主体、查询参数无提权、订阅/专属/停用/软删除/空组、旧数组、Token撤销、后台模式及共用限流；最终2顶层/11 PASS。HTTP价格同输入64组合与独立生产owner费用一致，明确0/unknown、倍率覆盖一次、完整四档/阶梯和分时/effort规则。测试用本地合成用户，GET审计插槽no-op，无生产凭据或扣款。
- S2.2–S2.4的30输入文件/51原产物匹配，按原边界复用。首次编译及后台缓存夹具失败已诊断修正；两次fresh只读复核，三项测试保护缺口加强断言并定向重跑关闭，未确认生产缺陷；复核未独立运行测试。审计`20261009T170219Z-kin-plus-catalog-verify-s25-cd7afaef` closed/verify passed。
- [执行证据S2.5](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s25-权限与报价一致性验证)和任务状态已更新，三处旧journal全文保留。临时容器已清理、Colima恢复停止。S2.5勾选，下一项S2.6未开始；本轮未跑浏览器阶段/全量/远端CI，不推送、不合并、不发布镜像、不部署生产。

## 2026-10-09：KIN 借鉴 Plus S2.4 可用渠道页面完成

- 本轮仅S2.4。只读核对personal仍`9397eb8af`、来源KlN `.5/c7aacf5d`；从干净S2.3 personal来源候选`b8db95969`建立`codex/plus-catalog-ui-s24`，已验证应用`b8cf49ca25007cbc23338a9443b71df4c0411155`。main来源文档树仅登记本change与journal，没有替代应用源码；原S0的17文件及各原候选/聊天树保留。
- 既有可用渠道入口显式读取catalog，按平台/模型聚合但保留各组报价；增加模型/分组/渠道检索、分组与服务档详情、阶梯/缓存/单位/参考时点和分时/effort规则。服务端适用倍率仅应用一次，零与未知分开，个人倍率失败标参考及重试，不承诺实时健康或最低路由价。刷新/主体切换/卸载取消并拒绝晚到响应，旧数组客户端保持。
- 新前端合同10去重用例、11文件lint、最终类型与build通过（含双语完整性3用例）；测试i18n夹具失败已诊断修正，只重跑受影响文件。保留测试compiler及既有build警告。Chromium1440/390、中英/明暗4组检索、详情、报价切换、summary Enter/Space、Escape焦点、错误重试/空目录通过；额外2组价格表键盘滚动与截图通过。使用合成GET，无生产JWT/数据库/权限验收；正常流程零error/warning，每组1次有意500单独验证恢复。
- 1次fresh只读复核未确认可行动缺陷，未独立运行测试；审计`20261009T164258Z-kin-plus-catalog-ui-s24-61dc32dd` closed/verify passed。S2.2的6文件/12产物与S2.3的5文件/14产物一致，原owner/DTO检查按未变边界复用；后端、计费、调度、迁移及依赖未改。
- [任务状态与执行证据](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s24-可用渠道页面模型与分组报价目录)及候选/检查清单已更新。S2.4已勾选，下一项S2.5未开始，S2.6/S2.7保持未执行。本轮不推送、不触发远端CI、不合并、不发布镜像、不部署生产；三处登记增量保留各自旧journal。

## 2026-10-09：KIN 借鉴 Plus S2.3 模型目录 DTO 与查询分支完成

- 本轮只执行 S2.3。personal 仍为 `9397eb8af`、来源 KlN `.5/c7aacf5d`；从干净 S2.2 候选 `e4083c38f` 建立 `codex/plus-catalog-s23`，已验证应用 `49c209a77059d2927f78bbd31c1dd479b509dc6f`。复用 main 来源文档工作树仅登记文档，没有替代应用源码。
- 同一用户 GET 仅单个精确 `view=catalog` 返回目录，默认/其他/重复值兼容旧数组。真实 GetAvailableGroups 先授权，active绑定与平台/公开请求名白名单先于聚合；保留分组报价、空组、稳定offer_key及显式DTO。报价来自S2.2同次配置快照，倍率只读当前用户一次、覆盖含0、失败标参考；未知单位不猜测，不改扣费/调度。
- 新HTTP/DTO合同13顶层/32PASS及旧helper10用例通过，共23顶层/42PASS；服务端构造编译通过（无业务测试运行）。固定Wire v0.7.0用生成器临时modfile完成，应用go.mod/go.sum原值保留；两次生成环境失败保留诊断。S2.2同输入owner对账和定向回归复用。
- 1次fresh只读复核未确认可操作缺陷；未独立重跑测试。审计 `20261009T155935Z-kin-plus-catalog-s23-f949dc81` closed/verify passed。复核后仅修正分支注释；S0原17文件、S1的54文件/45原产物、S2.2的6文件/12原产物均匹配，原候选与聊天checkout保留。三处计划及各自旧journal增量保留。
- [任务状态与执行证据](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s23-模型目录-dto-与查询分支)已更新；[DTO合同](../openspec/changes/adopt-plus-usage-and-diagnostics/catalog-contract.md#s23-已实现的目录-dto-与查询分支)记录单位、倍率与白名单。S2.3已勾选，下一项S2.4页面未开始；真实JWT/数据库/浏览器、S2.5/S2.6、全量与远端门禁未执行；不推送、不合并、不发布镜像、不部署生产。

## 2026-10-09：KIN 借鉴 Plus S2.2 权威价格解析完成

- 本轮只执行 S2.2。核对 personal 仍为 `9397eb8af`，来源 KlN `.5/c7aacf5d`；从干净 personal 来源 S1 `e3edb5666` 建立 `codex/plus-pricing-s22`，已验证应用 `e82287300d1b7cc625295c5307ebaa83c707c019`。main 来源新文档树 `codex/plus-pricing-s22-evidence` 仅登记文档，没有替代应用源码。
- [权威价格服务](../openspec/changes/adopt-plus-usage-and-diagnostics/pricing-contract.md)复用 KIN 真实 resolver、计费探针、上下文/服务档位、FreeFast、分时/effort 与个人/分组/高峰/媒体倍率；明确零价和未知分开，实际依赖请求或未支持单位保留规则/unknown。零标签继续按真实上下文求价，不展示成固定价。
- 新增15顶层/22 PASS项及48组生产 owner 对账通过；既有98顶层/158 PASS项定向回归通过。2次fresh只读复核确认4项问题均以反例先红后绿关闭；最终标签小修遵循第二轮认可方向并完成对账，没有第三次fresh全文复核。审计 `20261009T151715Z-kin-plus-pricing-s22-ef4e4438` closed/verify passed。
- 原17应用改动、S1候选和聊天checkout保持原值；S0/S1的17/54文件及45个原产物校验匹配，复用原边界成功证据。三处本计划及各自历史日志增量保留。详见[执行证据 S2.2](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md#s22-kin-权威价格解析)。
- S2.2已勾选；下一项S2.3模型聚合DTO与查询分支未开始。目录API/权限聚合、页面及媒体单价未接入；本轮不运行对应浏览器/数据库或全量门禁，不推送、不合并、不发布镜像、不部署生产。

## 2026-10-09：KIN 借鉴 Plus S1.1–S1.4 第一批完成

- 本次只执行S1.1–S1.4。开始/结束只读核对personal仍为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`；来源保持KlN `.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。新应用工作树 `codex/plus-usage-s1` 从personal创建，完整保存原S0候选到 `6bf78b18ce759e4f211c62b0518c3bca7418efde`，原17应用文件未修改；原聊天CI维护checkout保持干净，没有使用main应用源码。
- 最终已验证应用 `3f04437572e2819f0313ccc2a3f1a618a2afcdf0`。冻结9项新计时/终态字段与历史0/NULL/unknown；新增251迁移，正规Ent生成，显式SQL/单条/批量/幂等/best-effort fallback/DTO/前端类型贯通。保留旧first_token_ms/openai_ttft_mode、Token/费用/调度/重试与原用量owner。
- 第一批原生Responses普通/透传HTTP/SSE、JSON/SSE转JSON、pooled/ingress/桥接/原生WS采集接线；沿用既有duration/turn起点、ID归属、首次终态冻结与异步深复制。页面与CSV/Excel共用TPS helper，分清旧首字/严格首Token、媒体与部分状态，统一说明非流式完整内容观察边界。
- 验证：真实PG16单条/批量/重复写/历史未知/失败fallback/Ent，55个repository与13个DTO顶层检查通过；真实备份恢复后固定旧源码与新源码S1→S0→S1启动/迁移/SQL/DTO往返通过。最终定向采集、计费快照、WS与race通过；前端162去重用例、改动lint、i18n/类型/Vite通过；四组本地Chromium页面/键盘/窄屏/双语及实际32行CSV/Excel解析一致，说明修正后四组弹层重新验收。仍有效的S0和未改动证据复用，构建既有warning如实保留。
- 4次fresh只读复核完成，确认的checksum临时夹具、权威音频拆分/同源用量/shell item/JSON来源及usage-only音频事实问题均关闭；新增反例先红后绿。最终20行按实际3f0443757复核可关闭；审计 `20261009T092527Z-kin-plus-usage-s1-c8034a11` closed/verify passed，34产物完整，无异常。复核是代码/日志检查，不称独立执行测试。
- [任务与证据](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md)、[实际覆盖](../openspec/changes/adopt-plus-usage-and-diagnostics/coverage.md)、[交付准备](../openspec/changes/adopt-plus-usage-and-diagnostics/delivery.md)已更新并按维护职责增量同步main来源文档候选及原指定位置，各自旧journal全文保留。文档提交会推进本地HEAD，后续CI绑定真实最终候选；远端必要CI尚未执行，部署运行树兼容仍需针对真实生产候选审定。
- HTTP原失败不新增用量行；无可信turn/ID、opaque frame、Cyber和转换/其他平台仍旧版本或未知，不能称全平台完成。没有provider凭据请求、生产权限隔离或浏览器矩阵；没有推送/PR/Actions、合并、镜像发布、SSH或生产部署。S1.1–S1.4已勾选，S0.2/S0.3、S1.5与后续保持未执行；下一项推荐S2.1权限与接口合同，本会话到此暂停，不自动推进。

## 2026-10-09：KIN 借鉴 Plus S0.1 最终候选核对完成

- 本次仅执行 S0.1。远端 personal、本地 personal 与 `codex/fix-version-usage-help` HEAD 均为 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，应用仍从该 personal 工作树核对；来源保持 KlN `.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。没有使用 main 应用源码替代候选。
- 原有 17 个应用文件与计划编制时 SHA-256 清单全部一致；最终 diff 确认个人 release 通道/缓存、镜像与源码更新能力、版本检查状态、TPS/首字独立点击说明及旧首字文案。应用候选仍未提交，以应用树 `27d96087e53a97386fd86935afa4ccc3dcd991e3`、基线 SHA 与补丁/文件校验值固定内容；该树不是可用于发布的提交 SHA。
- 复用原始 Go 两包、UsageTps/UsageTable/HelpTooltip 97 用例、最终 VersionBadge 10 用例、i18n/类型/Vite build、合成 API 浏览器交互及真实匿名 GitHub 查询证据。历史独立复核审计包的 20 个产物校验通过；本次没有新委派。补充版本 store 28 用例及 13 个改动前端文件 ESLint，通过。
- 补充检查先被默认 pnpm 11 与现有 pnpm 9 依赖布局不匹配阻止，实际测试未启动；改用已有 pnpm 9.15.9 后两项检查通过，没有安装依赖或改动锁文件。浏览器首轮遗留版本文案 warning、最终版本三状态零 error/warning，以及历史 build 警告分别登记，不混称零 warning。
- 按维护职责从最新 origin/main `b43a472f4b1bde5983b3bdfa1447cd3622a77540` 创建 `codex/plus-s01-evidence` 文档工作树，登记[执行证据](../openspec/changes/adopt-plus-usage-and-diagnostics/implementation-evidence.md)，同步原计划位置的任务状态和证据；两处原有日志均保留，原聊天 CI 维护 checkout 未修改。
- S0.1 已勾选；下一项为 S0.2 的实际候选提交 SHA、必要 CI 与交付条件准备，未自动开始。应用与本次文档均未提交/推送，合并、镜像发布、生产部署和后续阶段均未执行；旧 `.5-tps.1` 发布/生产结果不能代表本修复，固定后端运行树兼容证明仍需针对真实候选审定。

## 2026-10-09：KIN 借鉴 Plus 功能计划与任务清单

- 按用户要求仅编写计划文档和任务清单，保存于 `openspec/changes/adopt-plus-usage-and-diagnostics/`，未实施新增应用功能。
- 固定当前 KIN `.5` 来源和 Plus `90da415c` 参考；主线为现有修复交付、用量计时/完成状态、模型价格、错误诊断，服务状态与用户协助按实际规模另行排期。
- 计划保留现有指纹、调度、gwpool、计费与旧首字配置；建议新增严格首字字段，避免替换旧字段而影响现有统计。新增迁移、请求/WS 快照、权限与报价一致性均写入对应验收任务。
- 文档完成不代表代码、发布或线上验收完成。现有版本/说明修复候选保留；本轮不推送、不发布、不连接生产。
- 文档检查覆盖本地链接、唯一任务编号、勾选状态和 diff 空白；既有 17 个应用改动文件的内容校验值保持一致。本轮未运行应用测试或构建。

## 2026-10-08：个人版本提示、TPS 与首字说明修复

- 当前聊天 checkout 为 CI 维护分支，未将该树的应用源码用于修复。复用已部署应用来源 `personal` / `9397eb8af`，在 managed worktree 的 `codex/fix-version-usage-help` 准备候选，保留 KlN `.5` 与个人 TPS 补丁。
- 版本 owner 识别个人 `X.Y.Z-klno.N-tps.N` 通道，按基础版本、KlN 序号、TPS 序号比较；个人发布查询 `ccisnoxx/sub2api`，`Wei-Shaw/sub2api` 继续只读监测。缓存绑定仓库，旧 KlN/原版缓存不混用。API 报告真实构建类型、发布仓库和更新能力；带个人标签的源码构建保持手动更新；个人镜像与源码构建在 service 提前拒绝二进制更新及回滚，个人 HTTP 路由返回 `409 / IN_PLACE_UPDATE_NOT_SUPPORTED`。
- 个人徽标保留完整后缀，固定 digest 更新说明遵守现有部署流程与回滚数据兼容/部署记录合同；检查失败及尚未检查不宣称“最新”。普通 KlN release 升级路径保留。
- TPS 移除标签和数值的原生悬停说明，使用独立圆圈点击入口；首字点击入口仅说明首次响应事件/输出耗时，明确平台与设置可能记录响应元数据、推理或工具调用，未采集显示 `-`。两个说明可独立关闭，点击另一个入口会关闭此前弹层，Enter/Space/Escape 交互保留。
- 已读取 LuckyKuang/sub2api-plus 的当前 `UsageTable.vue` 和 `usageTiming.ts`：兼容的平均速率公式、颜色、数字格式沿用；完整首字详情依赖当前 KlN 接口不存在的 `timing_version`、`first_output_kind`、`first_output_ms`、`last_token_ms` 等字段，本轮未扩展后端用量合同或伪造该数据。
- 验证：UsageTps/UsageTable/HelpTooltip 97 个用例通过；版本 service/handler 的定向 Go 测试两包通过；VersionBadge/app/locale completeness 共 41 个用例通过（最终 VersionBadge 10 个，其余检查沿用未变代码的成功证据）；改动前端文件 ESLint 与 diff 检查通过；前端 build（i18n、Vue 类型检查、Vite）通过。浏览器在 `http://127.0.0.1:4173` 验证管理员/用户、1440/390、中文/英文、明暗主题的说明交互及三种个人版本状态。使用合成 API，无生产认证或数据。
- Browser 插件未列出，使用已有 Playwright/Chromium；版本稳定候选的三个状态无控制台错误或警告。首轮 TPS 浏览器验收遇到实现中的版本文案尚未添加，后续版本验收确认已补齐。未运行全套回归或发布 gate，未推送、创建 PR、发布镜像、SSH 或部署。
- 真实 GitHub 元数据补充：个人两个已发布版本均 `draft=false / prerelease=true`，`releases/latest` 实际返回 404。已将个人查询改为读取最近 100 条发布，过滤草稿和非法标签、保留预发布，按五段版本数字选最新项；普通 KlN latest 查询与回滚筛选保留。新增回归在修正前分别复现预发布查询失败及源码误判为容器，修正后通过。
- 真实网络闭环：使用最终 `UpdateService` 和仓库 GitHub 客户端匿名查询个人发布，`.5-tps.1` 正确识别自身为最新、无 warning，release 模式为 container；同通道缓存用于 source 构建时模式为 manual。无生产接口或更新操作。
- 两次独立只读复核完成：首轮发现预发布 latest 查询及个人源码构建误判，两项已修正；最终复核未确认新增可操作问题。子代理审计 Bundle 已校验并归档（`20261009T061925Z-fix-version-tps-tooltips-06d9f2b5`）。未把源码修复或本地验证记录成线上已生效。部署工具的固定运行树兼容证明需要针对本候选重新审定，不能沿用 `.5` 上线时的旧证明。

## 2026-10-08：个人镜像发布，P3 实现候选

- 从已验收 personal 029cd8fb 创建 codex/personal-release；读取默认 main 最新阶段记录，未合并 main 应用源码。来源仍为 v0.2.14-klno.3/de08df02，TPS 保留，未处理 .5 历史升级或连接生产。
- 复用 Release matrix、前端单次构建、archive 来源及校验；增加个人来源/最终 SHA/完整 CI 门禁、串行 tps.N 分配、显式 Release dispatch 与镜像占用校验。已完成版本跳过，部分发布复用镜像，文档变化不分配版本，个人 VERSION 不写回 main。
- 当前为实现候选，PERSONAL_RELEASE_ENABLED 尚未开启；实际 CI、simple dry run、独立复核、首次 GHCR 发布与匿名拉取仍需取得远端证据。完成结果继续只登记 main，不改变已发布源码 SHA。

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
