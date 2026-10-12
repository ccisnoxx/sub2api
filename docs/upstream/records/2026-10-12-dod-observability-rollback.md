# 第8节续作：OBS整合、运行监控与跨KIN回退

## 实际结论与固定边界

2026-10-12本轮依次普通合入OBS-002 / PR #32、OBS-001 / PR #31、OBS-004 / PR #34（携带既有OBS-003 / PR #33语义依赖），定向验证和独立复核完成。源码整合最终为personal `423f690bec2ee9d0ebccd1672906d6b688c220c3`。实际监控接入未验收：通知渠道/接收目标/已有认证配置路径尚待维护者提供；固定生产镜像没有新聚合日志producer，另建版本的发布授权已询问，未获得答复。跨KIN评估及定向负向验证完成，确认旧写入会丢失新状态；全业务无损条件不成立，禁止旧镜像恢复。第8节当前12项完成、2项未完成，不声明整个计划完成。

开始时main为 `8d540c39c751d8526155ff206715a6f05025cf78`、personal为 `8f26d0ee2064867f293fbeda6744633aa5f07d43`。本轮只推进维护者指定的三项，不升级KIN，不修改已执行迁移身份，不覆盖正式标签/镜像，不重复应用Full或安全扫描。原工作区分支 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca` 和既有 `AM REMEDIATION_PLAN.md` 保留；实施及记录使用隔离工作树。

| 已发布字段 | 固定值与核对结果 |
|---|---|
| KIN基线 | `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71` |
| 正式应用source / tag | `42907bcab278f93db48d7e1b39b28e28f664569e` / `v0.2.15-klno.2-tps.1` |
| 发布Full | [38274106520](https://github.com/ccisnoxx/sub2api/actions/runs/38274106520)，原固定429源码的唯一手动Full |
| Release | [ID410901327](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.15-klno.2-tps.1)，12个既有assets及证明正文保持 |
| OCI index | `sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3`，本轮只读检查正式镜像tag仍解析至该digest |
| amd64 leaf | `sha256:db1819ef0c08eeaa0c0400529aab4b22dad22c037f47b04dc9cc631f5145b8e6`，沿用未变部署证据 |
| 旧稳定来源 / digest | `v0.2.14-klno.5-tps.2` / `3d5e1fde82707900a21f2b5538b112c7bab3c04a` / `sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530` |
| 控制工具 | ARCH-002 `b1f9fb1332db8b705716ebaf0bcb134f5cbb74f5`，CI38304959563 / 95合同及现场只读dry-run证据复用 |

后续文档同步HEAD不是发布源码，新OBS源码也没有旧Full的发布资格。本轮没有创建Release、构建/推送镜像、部署应用或修改生产数据库。生产运行source仍429；已执行308行migration ledger的身份、校验和与applied_at合同沿用REL-002验收，不重新执行迁移。

## 普通整合与验证证据

| PR | 整合head / 普通merge / 最终tree | 实际完成 |
|---|---|---|
| [#32](https://github.com/ccisnoxx/sub2api/pull/32) | head `53c694a1900b77ea443dc8c60adff7e78eb21175`；merge `fe27c725c42de005358b23c964faad81bf3ae4b3`；tree `7bc03792c504ced09a31eaaad2e6058d43c88a98` | 21:59:51Z，基于8f26；当前ARCH004组合、隐私/限流/审计定向验证与fresh复核 |
| [#31](https://github.com/ccisnoxx/sub2api/pull/31) | head `0b26439f46f02d4ec43e85ace020692195709da0`；merge `04937e64879f19a07a2edf0e1c931a8008c0b7c9`；tree `a5a0108ef969de194b9bcd42a5c543701182fde3` | 22:50:09Z，基于fe27；两个组合P2修复、定向验证与fresh最终复核 |
| [#34](https://github.com/ccisnoxx/sub2api/pull/34) | head `f21b92378840f6852b49edab1ad577fdfaa0087c`；merge `423f690bec2ee9d0ebccd1672906d6b688c220c3`；tree `62290b31e11758faf72c5af45ad567fd07069feb` | 23:06:50Z，基于049；真实PG/Source/前端组合验证与fresh复核 |

逐次merge parents及实际tree均核对等于已验收候选；使用普通push/Ready/merge，没有admin绕过、强推或删除分支。PR #34携带原OBS-003语义依赖：PR #33于23:06:53Z自动变为MERGED，不能写成另外执行了一次手动合并。新source/merge精确SHA的Actions查询均为0，本轮没有dispatch新应用Full或扫描。

### OBS-002：当前组合下的隐私合同

原候选 `b8bf844d19805fda490c1d6e039585423f17586f` 的32路径增量中29路径逐字保持。运行时组合点为Ops handler和admin routes；journal冲突保留双方历史。首次定向Race为30顶层成功、4失败，原因是旧 `NotContains("routing_diagnostics")` 字符串断言误判ARCH004合法 `routing_diagnostics_enabled` 布尔metadata。仅3测试文件改为解码实际JSON，直接检查facts缺失、单详情metadata保留、批量facts/metadata均缺失、嵌套事件facts缺失；原正文、权限和隐私canary保留。重验原4失败顶层/22含子例通过，聚合证据覆盖34唯一顶层，不能写成单次34全绿。

fresh最终只读复核覆盖冻结组合及完整32文件，无确认可行动问题。仓储、迁移、privacy投影/types、Go依赖、相关前端代码/测试/锁/Vitest/TS输入未变，原PG、前端和 [历史Full38163070709](https://github.com/ccisnoxx/sub2api/actions/runs/38163070709)证据复用；packageManager只声明原实际pnpm9.15.9。历史Full绑定原b8，不能用于新SHA发布。诊断关闭时非法历史格式按ARCH004显式失败；启用时旧详情隐藏非法optional字段并保留正文，不把两种行为混称。生产429没有这些新增保护，当前勾选只表示源码整合验收。

### OBS-001：配置前置失败与整轮耗时

原ce0增量与fe27组合，OBS-002 Snapshot保持。定向PG13顶层成功、无skip/fail，其中4为默认unit方法、9为真实SQL测试；133890行31天合成负载含采样14ms。初始Monitor/Aggregator Race6通过。独立复核随后确认两项P2，原候选成功不能覆盖新ARCH004组合路径：

1. `runOnce` 在 `GetConfig` 出错、返回nil或45秒预算耗尽后退出，未进入仓储Aggregate defer，导致漏报ERROR、timeout和连续失败。最小修复引入仓储同一monitor的前置失败报告；真实Start→GetConfig→monitor回归RED1→GREEN1，直接Race10通过，包括一次真实45秒deadline。失败计数1→2→3→nil4→Aggregate begin失败5，确认 `awaiting_epoch` 提交后连续计数归零、累计保持5；正常Stop取消为INFO且不增加计数，不对HTTP配置读取错误计数，不触发双报/清理/采样。
2. 成功配置等待不计入Aggregate新起点，41秒前置会漏报slow。改为 `runOnce` 唯一单调 `started` 显式传入Aggregate，业务观察窗口 `at` 独立。真实41秒回归RED `duration_ms=3` → GREEN `41002`、WARN、`aggregation_slow`、一轮一次事件、失败计数0。新单项Race和直接9项Race通过；旧45秒失败/取消路径和SQL输入未变，复用其成功证据，不重复45秒或PG完整集。

仓储/服务vet、handler/routes/server unit消费者及integration消费者编译通过；25个PG测试调用仅适配显式时间参数，SQL fixture/assertions与Snapshot保持。fresh最终只读复核无新增确认问题。COMMIT真实断连、专项容量并发、完整分钟ticker长期分布没有重新动态验证，不把消费者编译称为运行测试。

### OBS-004：语义依赖、真实SQL与前端组合

21文件整合携带PR #33。README冲突保留当前429安装/手动CI流程和新增SLA说明；前端冲突同时保留OBS-002 schema2隐私fixture/null用例及OBS-003双语reason/health断言。API合同按当前schema2/max(5,min)、低样本/互补隐藏、平台NULL和小时证据更新；runbook按ARCH004禁用skip聚合与cleanup、共享45秒预算/整轮起点/一次报告纠正。

实际运行 `TestServiceStatus(SnapshotStateContract|WatchSQLStoppedSourceAndReenable)`：真实PG2顶层/28含子例通过，包含26状态case和维护WATCH_SQL/evaluate的停止、源缺口、禁用/重开；Source固定字段/隐私/非阻塞/停止flush直接Race5通过。前端 `ServiceStatusView.spec.ts` 65 + locale completeness3 = 68 Vitest、`vue-tsc --noEmit`通过。pnpm9.15.9 frozen offline install没有下载新依赖，锁文件未变。Python检查器5项与相同分类语义的原成功证据复用；[历史CI38191718143](https://github.com/ccisnoxx/sub2api/actions/runs/38191718143)已回读为原79 head的completed/success，只作为历史证据。

fresh只读复核无源码阻断；确认文档P2为OBS-004报告仍有泛化旧回退表述。复核树 `5b2f177fc4e0617c49a66f037c4b3449a13704d8` 后仅该报告按方向修正跨KIN禁令/当前CI流程及原CI实际状态，其余20路径逐字保持；最终tree62290与实际merge相同，不为纯文档修正重复源测试。

## 实际运行探测与接入缺口

既有目标为hostdzire的 `/root/sub2api-kin`，应用127.0.0.1:10088，公开入口 `https://api.962850.xyz`。固定原429应用和PG容器身份后只读检查：Ops通知配置存在且alert_enabled=true，但recipients=0，SMTP host/from/password均未配置，无对应外部timer，sendmail/mail不存在。未输出实际凭据/收件人，也未调用会初始化默认settings的GET接口。

使用维护检查器同一PROBE_SQL、数据库时间、RR READ ONLY、statement_timeout=2s/lock_timeout=1s：真实结果enabled/has_platforms=true，启用区间209294.724秒、水位年龄74.724秒，aggregation_error/source_error/configuration_changed均false，exit0。这是现场兼容探测，没有安装timer或发送通知，不能证明持续调度、探测不可用通知、去重、恢复通知或真实送达。

2026-10-12T22:24:52.759301Z同一固定PG执行维护容量SQL，exit0：1490事实、主表253952/索引106496/总360448字节，死元组估计20、autovacuum83次，逻辑UTC日行149/850/491、每分钟范围峰值4、过期事实0、长事务0、activity_complete=true。覆盖约58小时15分，不称31天真实长期实测；没有清理、调整权限或数据库写入。

当前429应用不包含新的聚合耗时/扫描/清理确认/失败计数producer。外部SQL可探测水位和容量，不能补造这些应用指标。本轮已向维护者询问现有通知渠道、接收目标及认证配置路径/secret名称（不请求secret内容），并在完整候选可审查后询问是否授权另建 `v0.2.15-klno.2-tps.2`，对最终personal SHA执行一次发布必要Full并按既有独立测试→生产流程部署。原REL-002授权限定原tag/SHA/Full/digest恢复，未据此发布新增OBS源码。尚无答复，相关步骤停止并保留证据；不会用日志文件代替真实投递或用原Full冒充新SHA资格。

## 跨KIN持久化合同评估与实际负向验证

固定旧应用A `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 与新正式I `42907bcab278f93db48d7e1b39b28e28f664569e`。相关路径分别与旧/新冻结KIN源相同；当前OBS423到I的migration、account schema/repository/service/usage、来源字段与NOTICE等8组业务输入未变。评估没有升级KIN或改变任何writer。

| 不兼容合同 | 实际writer和影响 | 证据范围 |
|---|---|---|
| 休整状态 `quality_gateways` | I的 `GatewayPoolRestState` 保存质量网关集合；A旧结构没有该字段，正常后台resume读后整体重写 `openai_gwpool_rest_state`，删除集合并可能延后恢复时刻。仓储root JSONB浅合并不能保留被整键替换的嵌套字段 | 真实源码类型/decoder/JSON序列化及原仓储JSONB表达式隔离执行确认信息丢失；后台调用/质量OR-count规则为静态合同核对 |
| 联系人轮次上限 | I允许300、A仅64，TTL都为7天；65条仍有效记录中更新已有近期轮次，A读/prune/serialize/write保留64，丢失尚未过期记录 | 固定源码writer、上限、调用和原测试静态反例；未额外运行该路径 |
| 多协议凭据 `protocols` | I规则保留 `responses`/`chat_completions` 列表；A普通credentials编辑走旧Normalize，只序列化pattern/protocol再整体SetCredentials，丢失多协议列表并改变路由 | 固定源码类型/Normalize、admin更新入口、redaction merge、repository整体写入和原多协议fixture静态反例；未额外运行该路径 |

主反例直接摘取未改的A `openai_gwpool_resume.go` 16—53行与I 16—56行真实类型/`readGatewayPoolRest`，适配同一Extra map并用实际类型JSON写回。合成active状态含 `quality_gateways=["good"]` 和独立unrelated键；Go1.27.2 `go run .`退出0，结果旧patch没有quality、新patch保留quality、新类型重读旧patch不能重建，`lossless=false`。保存完整来源SHA、片段SHA、fixture、patch和结果，不是另写一套模拟decoder。

随后使用本地缓存PostgreSQL18.1隔离容器，network=none、无发布端口/显式mount，执行A `account_repo.go:2545` 的原表达式 `COALESCE(extra, '{}'::jsonb) || $4::jsonb || jsonb_build_object($5::text, NOW())`，仅绑定合成CTE值、READ ONLY/2秒语句/1秒锁：exit0，unrelated仍保留，新quality为 `["good"]`，旧active仍在但旧quality缺失。该负向证据排除了“root浅合并会自动保留嵌套集合”的解释。专属容器已按ownership标记核对后清理；没有加载生产数据、运行应用迁移或写入生产。

这不是整套旧应用/后台调度/跨实例动态证明，也未覆盖全部 `command_code`/cline/new-platform配额writer、旧resume读失败/缓存以及整Account.Update的并发快照回写。通用map持久化不能证明各typed owner无损；已确认反例足以否定全业务无损要求，不继续重复Full或批量测试。旧镜像自动/显式恢复仍被部署工具 `image_rollback_compatible=false` 拒绝；95合同/既有真实拒绝与现场dry-run输入未变证据复用，本轮没有调用可能写入的live rollback。

源码定位：[旧休整类型/reader](https://github.com/ccisnoxx/sub2api/blob/3d5e1fde82707900a21f2b5538b112c7bab3c04a/backend/internal/service/openai_gwpool_resume.go#L22)、[新quality状态](https://github.com/ccisnoxx/sub2api/blob/42907bcab278f93db48d7e1b39b28e28f664569e/backend/internal/service/openai_gwpool_resume.go#L30)、[原JSONB writer](https://github.com/ccisnoxx/sub2api/blob/3d5e1fde82707900a21f2b5538b112c7bab3c04a/backend/internal/repository/account_repo.go#L2545)、[旧联系人上限](https://github.com/ccisnoxx/sub2api/blob/3d5e1fde82707900a21f2b5538b112c7bab3c04a/backend/internal/service/openai_gwpool_contacts.go#L20)、[旧协议Normalize](https://github.com/ccisnoxx/sub2api/blob/3d5e1fde82707900a21f2b5538b112c7bab3c04a/backend/internal/service/opencode_go.go#L229)。

## 恢复方法与未完成验收

当前生产故障恢复优先冻结写入、保留现场并向前修复。既有维护helper可重新freeze，入口和命令按[REL-002恢复记录](2026-10-12-rel-002-recovery.md)及[运行手册](https://github.com/ccisnoxx/sub2api/blob/423f690bec2ee9d0ebccd1672906d6b688c220c3/docs/observability/service-status-runbook.md)执行。跨KIN不能仅切旧镜像；完整备份恢复需要维护者明确确认快照后写入损失、在隔离库完整恢复核对后再执行受控切换，禁止在线覆盖当前库。当前恢复点为2026-10-12T19:10:31.237543Z同MVCC快照，沿用REL-002真实完整隔离恢复证据；备份保留不等于回退无损。

源码撤销使用普通新提交/PR，按第一parent评估merge revert并执行受影响验证，不强推/删除产物/回滚schema或ledger。完整revert PR #34的merge也会撤销其携带的PR #33增量；只撤销OBS-004时应选择性撤销原独立增量并保留OBS-001/002/003及后续修复，不能盲用整merge revert。

第8节两项继续未勾选：完整运行监控需要明确通知目标及认证路径、解决新producer发布边界、实际调度与故障/恢复/送达验收；跨KIN全业务无损回退已经被实际反例否定，需独立兼容设计和完整writer证据才能考虑解除禁令。本轮不擅自扩大为新的兼容改造。

## 独立复核与执行审计边界

当前Bundle `20261012T214308Z-dod-observability-and-kin-rollback-39cf8265` 已关闭、校验通过，9次执行尝试中8次交付验收通过，6次标记独立复核，剩余活跃Worker=0。代理只读复核不等于独立重跑测试；源码验证由主代理/写入代理实际执行，证据按各自命令记录区分。

保留两条异常：第一轮OBS-002 review误用一次 `git write-tree`，源文件摘要保持，但Git对象/cache-tree零写入无法确认，因此该执行严格只读验收rejected、写入边界unknown；fresh替代复核已完成并验收，不改记第一轮为成功。OBS-001两轮P2及原RED、OBS-004文档P2、原取消/失败运行也保持历史事实。

原始命令、stdout/stderr/exit、固定树、来源片段、通知配置布尔投影和负向验证结果保存在权限受限的本地验收目录；正式记录仅写非敏感汇总。临时测试容器已清理，原8个本地容器身份和Docker context default保持，既有工作区暂存/未暂存摘要保持。本记录、REMEDIATION_PLAN.md、UPSTREAM.md和记录索引按分支职责同步；文档同步不改变运行源码、KIN基线、正式产物或数据库。
