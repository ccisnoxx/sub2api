# KIN 借鉴 Plus候选交付与回退

本文件保留S1–S4历史准备、门禁与交付记录；当前第二批候选/门禁/回退状态见文末S1.5.5。旧段落的“本会话/当前/下一项”仅指当时记录轮，不代替最新候选状态。

本文件准备后续交付操作；本会话不推送、不合并、不发布镜像、不连接生产。已验证应用提交为 `3f04437572e2819f0313ccc2a3f1a618a2afcdf0`；实际覆盖、检查与独立复核结果登记在 [执行证据](implementation-evidence.md)。文档归档会推进本地HEAD；后续远端门禁必须绑定届时的真实完整候选，不能将本地结果称作远端通过。

## 候选与门禁

应用分支 `codex/plus-usage-s1` 基于 personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`，既有 S0 修复单独保留于 `6bf78b18ce759e4f211c62b0518c3bca7418efde`。交付前再次核对最新远端 personal、来源 JSON、候选 diff 与 SHA；基础变化时先评估并更新候选，不能继续复用失效的绑定。

应用不包含 main/CI 维护候选的应用源码。当前聊天 `codex/ci-validation-scope` 的未合入 CI 选择变更不作为 personal 已生效门禁。按 personal 当前定义，后续必需 Personal CI 包含 binding、existing-ci、existing-security、tps、sync-contracts，最后由 personal-ready 在开始/结束核对相同 SHA 与最新基础。不得使用其他 SHA、旧个人镜像或 S0 的检查冒充 S1 门禁。远端完整 CI 尚未运行，本次只完成本地与交付准备；获得后续推送/PR授权后对稳定候选执行当前必需门禁，不重复中间全套 gate。

数据、采集与页面必须作为同一兼容候选交付，不单独部署要求新字段但迁移尚不存在的消费者。发布仍沿个人 simple/linux/amd64、来源 SHA、固定 digest 和部署工具合同；本期没有新的发布机制。

## 增量迁移

`backend/migrations/251_add_usage_log_timing.sql` 仅扩展 usage_logs；旧迁移和 schema_migrations 的历史 checksum 不变。启动时仍由现有 ApplyMigrations 加锁并应用未执行迁移。历史时点/音频/完成布尔保留 NULL，timing_version=0，status/source=unknown；无需回填或清空统计。

数据库真实验证与旧应用兼容范围见执行证据。兼容证明针对本次固定旧源码及扩展 schema 的迁移启动、用量 SQL 和 DTO；不代表未知的未来候选、其他迁移或未经核对的部署工具自动获得兼容授权。

## 备份、恢复与应用回退

后续生产更新前，由实际部署 owner 核对 PostgreSQL 版本、数据库名、卷及可用空间，准备维护窗口、当前固定镜像 digest 和 revision。备份必须先完成并确认可读；文件限制访问，避免将数据库内容提交到仓库。示例命令由部署环境中的真实变量决定，不能复制测试库参数到生产：

```sh
# 在已经授权的部署主机执行，按实际 compose/容器名称适配。
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > usage-s1-before.dump
chmod 600 usage-s1-before.dump
pg_restore --list usage-s1-before.dump > usage-s1-before.list
```

恢复演练应使用隔离数据库，检查 schema_migrations 和用量样本读写，然后再按已授权窗口实施。恢复到备份时间点会丢失其后写入，因此仅回退应用优先保留扩展 schema 和 migration ledger；不得为回退镜像删除新列、清理 ledger、修改旧 SQL 文件或回填历史成功状态。只有执行证据确认的旧应用在扩展 schema 可运行时，才可声明该固定旧应用的数据兼容。部署工具仍需针对真实部署候选审定运行树兼容信息；S0 的无迁移证明不足以覆盖 S1。

若后续必须恢复数据库，先停止用量写入和相关 worker，保存当前库以保留恢复期间的数据证据；按实际批准的恢复点恢复到隔离库验证，再决定切换。不能在在线计费期间覆盖数据库。最终生产 revision、镜像版本/digest、迁移结果与线上验收必须另行记录；本文件不代表这些动作已完成。

## 下一项

第一批验收完成后暂停。按主线推荐下一项 S2.1 模型价格目录权限与接口合同；S1.5 的第二批入口按实际使用需要另选；本会话不自动进入两者。S0.2/S0.3、S1发布/生产验收继续保持独立状态。


## S2 阶段交付（S2.7）

- 最终累计候选：`88156f09fcf980a771a8aab570f0dbaec5de25fb`，KlN来源`v0.2.14-klno.5/c7aacf5d`，personal基础`9397eb8afb621aef483f2ec0bf4b2dd6247c7b92`。工作树`codex/verify-personal-ci-plus-catalog-s27`，旧候选及未提交改动保留。
- [草稿PR #5](https://github.com/ccisnoxx/sub2api/pull/5)及[Personal CI `37982040213`](https://github.com/ccisnoxx/sub2api/actions/runs/37982040213)绑定相同候选/基础，全部必要job及App 15368 personal-ready通过。首轮失败按证据修正3处，定向通过后才触发最终SHA门禁，总计2个不同SHA原生run，无额外dispatch或本地完整gate，未采用未合入CI选择优化。
- 范围包含S0修复、S1第一批用量和S2目录。S2只读GET保留默认数组/JWT/分组权限；价格同源，个人倍率一次，0与unknown可区分。未改变实际Token计算、余额/订阅结算、调度、gwpool或重试。
- 数据边界：S2无新增迁移；累计候选继承S1迁移251。原固定旧源码/扩展schema兼容与上述备份恢复要求继续适用，不删除列或migration ledger，不宣称整个候选无数据变化。
- 状态：PR保持草稿，personal未更新，未合并/发布/生成镜像或digest/部署生产。合并后的最终personal完整SHA仍需自身成功personal-ready；S0.2/S0.3的合并发布/线上验收继续独立待执行。
- 未覆盖范围：S1第二批平台、生产审计/会话绑定/实际上游与扣款、真实后端浏览器E2E、媒体/请求依赖报价及原始HTTP字段白名单测试限制保持原边界；本轮无新fresh独立复核。

[本阶段门禁、修正及来源清单](evidence/s2.7-validation.json)。下一项S3.1未自动开始，本轮到此结束。

## S3.3 本地存储候选与回退边界

本轮仅S3.3。应用`ab4a3f5ce058b28cc3139e5e60297ab4c264ffb1`位于personal来源`codex/plus-routing-storage-s33`；文档固定可能推进本地HEAD，后续门禁须绑定届时真实完整候选。现有草稿PR #5/Personal CI只证明旧S2.7候选，不代表本轮通过远端门禁。本轮未push/更新PR/触发CI、合并、发布镜像或部署生产。

迁移252只给`ops_error_logs`新增无默认/回填的nullable JSONB；历史行保持NULL，新增诊断按管理员单记录白名单读回。真实PG16证明迁移、单批写读、坏诊断保留真实故障及批量失败原子回滚。新Ops repository→固定旧d9b06f4→新Ops repository读写往返和migration runner检查通过，新增ledger未修改、原诊断保留、旧写入NULL；这不是完整服务器/生产运行树证明。回退应用时保留扩展schema和ledger，不删除历史或修改旧迁移；实际交付前仍须取得运行树/备份/恢复条件及对应授权。

本地临时容器已清理，Colima恢复停止；原三个停止容器保留。固定来源、验证/复核限制见[执行证据S3.3](implementation-evidence.md#s33-贯通错误存储与-dto)。下一项仅S3.4「扩展现有错误详情」，未开始；S3.5/S3.6综合验收和交付仍未勾选。


## S3.4 本地页面候选与回退边界

本轮仅S3.4，应用`d23474171035320eda06a35d76d455d7f8d7aae4`在干净S3.3完整032db7982上续接，分支`codex/plus-routing-details-s34`复用已附加的personal工作树；原`codex/plus-routing-storage-s33`仍固定032db7982。8文件仅前端与测试，共用管理员单详情展示诊断并修复晚到响应归属；没有新增迁移、依赖、调度或扣费改变。候选依然继承S1迁移251和S3.3迁移252，上述数据备份/固定旧源码兼容及保留扩展schema的回退要求继续适用；不将前端回退描述为整个累计候选无数据变化。

78项定向、lint/类型/build、两组实际前端/合成API流程及fresh只读源码复核通过；S3.3后端原证据经17输入校验后复用。浏览器夹具不作为真实JWT或生产证明；既有Ops深链接列表加载限制、完整多turn日志及综合调度/扣费边界见[验证](evidence/s3.4-validation.json)。旧草稿PR #5/Personal CI仅绑定S2.7候选，本轮无push/PR更新/新CI、合并、镜像发布或生产部署。后续门禁必须绑定届时真实完整候选，不能称本地构建为远端通过。下一项S3.5未开始，本会话到此停止。


## S3 阶段交付（S3.6）

最终累计候选`afcc7852b36a073dfc5fc0721ae8d24b90b7a335`位于personal来源`codex/plus-routing-delivery-s36`，应用修复`da581c6846d1bf89926ca9730abf2290ea8b65ea`。personal基础9397eb8af、KlN .5/c7aacf5d保持；原草稿PR #5及固定S3.5分支保留。新增[草稿PR #6](https://github.com/ccisnoxx/sub2api/pull/6)的[Personal CI `38020112510`](https://github.com/ccisnoxx/sub2api/actions/runs/38020112510)全部必要job与App15368 personal-ready通过，绑定相同最终候选/最新基础；fresh只读阶段复核无确认阻断。实际检查、复用边界、门禁触发次数与未覆盖项见[执行证据](implementation-evidence.md#s36-独立复核与阶段交付)和[验证清单](evidence/s3.6-validation.json)。

候选累计包含S0修复、S1第一批、S2目录及S3诊断/native安全修复。v1对象只含白名单事实，管理员单详情可见；用户/列表及公开错误帧范围保持。S3.5的native当前轮次安全重放/当前模型选号和请求价/逻辑turn去重为明确行为修复，其他调度与扣费owner证据按原范围复用。本轮只追加5文件等价lint修正（22新增/24删除），定向35顶层/114 PASS与本地同版本S3四包不限输出lint 0 issues通过；S3.4页面与S3.3数据边界未变，旧证据按未改owner边界复用。

迁移251/252及前述备份/回退合同继续适用：历史未知不回填，应用回退保留扩展schema和migration ledger。固定旧源码的repository/migration runner兼容不等于完整生产运行树证明，不能沿用S0无迁移声明；实际部署前准备并验证可读备份、隔离恢复及真实运行树兼容，按另行授权执行。

状态：草稿待审；未合并/更新personal、未发布镜像/执行Release dry run、未生成新镜像digest、未SSH或部署生产。PR门禁只证明本候选，正式发布要求合并后最终personal SHA自身门禁。完整JWT后端浏览器、生产扣款/运行树、付费上游以及已登记的WS运行覆盖限制仍未验证；既有Ops深链接问题不在本轮扩展范围。

S3阶段结束，下一顺序项S4.1仅确认启用需求，S4/S5未排期；S1.5和S0.2/S0.3按实际需求与独立授权另选。本会话到此停止，不进入下一阶段。


## S4.3 本地实现登记

独立personal分支`codex/plus-service-status-s43`的应用候选`35962a802fdc499639c9c861072f25774ccba50b`完成S4.3，默认关闭；新迁移253/254追加nullable内部字段和独立表/分组可见性触发器，不回填旧数据，不修改旧ledger。原S3.6 CI/草稿PR只绑定afcc候选，没有更新为S4结果。最终定向检查和独立复核见[执行证据](implementation-evidence.md#s43-独立聚合与展示实现)。

S4.4真实JWT/数据库/浏览器贯通、固定旧应用扩展schema兼容及实际终态覆盖未执行，不能直接宣布应用镜像回退兼容。回退保留新列/表/ledger与触发器，关闭S4；强制退出前未持久源缺口不保证重建，最终写入失败会明确返回。生产操作前仍按实际授权准备备份、必要CI与固定digest，不自动触发发布/部署或开启配置。


## S4.4 发布与生产验收准备（历史快照，74508aa）

固定候选`74508aa0e1c2d61e9e269fbe73e3eb8ce515a9da`来自personal基础9397；主应用仍在personal候选树，main只维护发布/部署工具及运行记录。[PR #7](https://github.com/ccisnoxx/sub2api/pull/7)的[必要CI38042366636](https://github.com/ccisnoxx/sub2api/actions/runs/38042366636)进行中，53 fixture/binding独立复核无确认阻断；745 clock/binding复核进行中，main部署锚点e52的[工具CI](https://github.com/ccisnoxx/sub2api/actions/runs/38042363913)进行中。最终personal合入SHA必须取得自身完整CI才能发布；旧候选及PR结果不能代替正式发布门禁。

固定旧9397已在真实PG18扩展schema完成读写、实际JWT/API/HTTP及仓储批量/幂等/fallback，再切回新应用。新版默认关闭开关不改变源终态采集；正式部署先核对真实revision/version/digest、健康/基础功能及源记录，再按授权启用S4并观察真实水位、样本和未知原因。隔离环境的故障/恢复与可控上游反例按[运行验证](evidence/s4.4-runtime-validation.json)复用，不在生产注入故障。部署终态/固定前缀/应用镜像恢复合同以60项工具测试为证据；没有真实生产回滚或数据库恢复演练。

当前生产仍固定9397/0.2.14-klno.5-tps.1；正式新版本、digest、最终personal SHA CI及部署验收尚未发生。累计回退只切回绑定的旧应用，保持扩展schema/ledger、配置及PG/Redis，不恢复数据库、不覆盖私密备份。实际成功记录生成后补充其专属rollback入口；不能预填示例ID为可用恢复命令。S1.5/S5未开始。


## S4.4 当前累计正式交付

累计候选`d72a3dafbdcb1edb642fca8dcfb29a803eaa97af`经PR #7按merge commit合入personal，最终`3d5e1fde82707900a21f2b5538b112c7bab3c04a`的11项必要CI及personal-ready成功；版本`0.2.14-klno.5-tps.2`、实际Git标签、唯一linux/amd64及digest`sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530`绑定一致，正式Release38050231176成功。S0修复/S1第一批/S2目录/S3诊断/S4独立状态包含在该personal应用树，main应用源码不参与替换。实际旧应用9397扩展PG18读写兼容、真实多轮WS/重试/取消/失败与恢复及最终正式镜像的JWT/数据库/浏览器证据见[运行验证](evidence/s4.4-runtime-validation.json)。

可信main工具e3b3e8e完成hostdzire原生部署记录`20261010T120908Z-b497b80ee8a6`，仅替换应用；备份可读校验、完整旧/新revision与版本/digest外层兼容证明及账本303→307/仅251–254通过。PG18.6与Redis原容器身份保持。先以S4默认关闭核对真实8条WS终态及基础页面，再开启v2且保留默认阈值；连续水位推进与完整窗口DB/UI一致，无聚合错误。实际有请求分组显示已观察样本正常，空请求分组与平台保持未知。

生产备份及专属记录保留在`/root/sub2api-kin/.personal-deploy/records/20261010T120908Z-b497b80ee8a6`。应用回退仅使用该成功记录绑定的旧9397/tps.1镜像并保留扩展schema/ledger及PG/Redis；专属工具入口为`bash deploy/personal/deploy-hostdzire.sh --rollback 20261010T120908Z-b497b80ee8a6`。此处登记入口，不执行回退；不会恢复数据库、覆盖私密配置或删除监控表。实际生产回滚/数据库恢复未演练，隔离旧应用兼容及60项部署工具合同不改称实机回滚。

S4.4、S0.2/S0.3完成；S1.5.1为下一项，未自动开始。PR #5/#6保持原状态。先前中断worker审计终态缺失、完整执行Digest无法闭合，失败明示并保留原始日志；有效独立复核与两项P2修复红绿、最终CI另行记录。

## S1.5.5 第二批本地交付记录

### 固定候选与实际覆盖

应用仍为 `1ae266791d0ab324cd13a63ab344019c79102176`，分支 `codex/plus-chat-timing-s151`，基础personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a`；KlN来源 `v0.2.14-klno.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。main仅增量登记文档，不替代personal应用源码；本轮不追加应用提交。相对personal仅12个backend service源码/测试路径变化，原Token/费用/倍率/幂等/异步行创建/调度合同保持。

所选raw CC JSON/SSE、CC ↔ Responses双向转换及复用Grok桥已在S1.5.1实现、S1.5.4验证复核；原生Responses HTTP/SSE/WS继承第一批覆盖。Anthropic（S1.5.2）、Gemini/Antigravity及独立媒体（S1.5.3）不适用；其他未支持producer保持旧口径/未知。实际owner、协议与限制以[当前覆盖表](coverage.md#s155-当前所选入口与交付状态)为准。

### 候选门禁与状态

| 门禁 | 当前状态 | 后续绑定要求 |
|---|---|---|
| 本地协议/用量/终态/音频与独立复核 | 已通过，S1.5.4有效证据按未变输入复用 | 音频修正后race9/69；较早124/271、20/115、3/9仅用于未受影响边界，不算最终完整重跑 |
| personal来源与祖先、当前实际diff | 本轮只读核对通过，12源输入及依赖不变 | 基础或候选变化时重新评估，不能复用旧绑定 |
| 当前候选PR / Personal CI / personal-ready | **未取得**：只读查询无该分支PR、无该SHA运行；check-runs HTTP422 No commit found | 稳定完整候选、dispatch ref、实际PR和最新personal基础一致；开始/结束重核对 |
| 合并后最终personal SHA门禁 | **未执行** | 最终merge SHA自身完整Personal CI及可信App15368 personal-ready，不以PR/旧基线结果替代 |
| 镜像发布与生产更新 | **未执行**；没有新版本、digest或部署记录 | 沿用既有版本分配、simple/linux/amd64、OCI/source/revision/固定digest和实际部署记录合同 |
| 新运行树部署/回退兼容证明 | **未审定，现有owner拒绝** | main部署owner另行审定实际完整运行树、数据与配置保证，不改成force/ignore放行 |

当前personal的 `.github/workflows/personal-ci.yml` 仍要求binding、existing-ci（backend-ci）、existing-security（security-scan）、tps、sync-contracts，最后personal-ready汇总全部成功并重核对绑定；pnpm9/Node20/frozen lockfile及现行检查保持。没有采用当前cwd未合入的CI选择优化。非personal候选必须有实际同仓库open且base=personal的PR，head/base完整SHA与检查一致。当前源SHA未在远端可解析，因此“无检查”不是通过。

本轮只完成交付记录，不创建PR、不push、不dispatch完整CI。按已有文档的后续交付授权边界，取得对应远端操作授权后才对稳定候选执行必要门禁；同候选/分支/基础的成功结果有效时复用，不重复中间全套gate。若日后把文档另行提交到应用分支，实际HEAD变化也必须重新绑定真实完整SHA。发布前必须使用合入后的personal SHA自身门禁。

### 数据与回退边界

本批相对personal没有新增迁移、Ent/schema、依赖、镜像配置或部署资源变化，继承251–254及原扩展schema。回退源码的方向是撤回本批应用提交的12路径变化，恢复到基础应用行为；需要在独立候选中处理并重新验证，保留已有未提交改动，不在当前工作树reset/覆盖文件。旧行、已写新计时、音频NULL与未知完成状态不回填或清除；回退后这些CC入口恢复旧计时/未知，不能继续宣称新采集覆盖。

本机部署GitEvidence使用的完整运行树指纹：基础 `ecbf079cd17f89f7604987d35a2740ea90264d7327178ed5c7ee2d61e73f9747`，当前候选 `425f3f552915e414959213925b3819bab2a93852c3eec49e93dab3af83f5fdf8`。新树与固定S0–S4树不同，真实12路径不属于已审定的五文件安全补丁；对正反方向调用本机兼容owner均返回预期 `E_COMPATIBILITY`。这只是只读Git/owner检查，没有SSH、Docker、镜像构建或部署调用，也没有新增兼容放行。

即使schema相同，也不能宣布现有部署工具已支持该候选。后续部署/故障恢复必须先审定真实旧/新完整revision、运行树、已发布固定digest及实际部署记录；只有证明和现场配置/依赖/账本保持条件成立时，才可仅回退应用。保留扩展schema、migration ledger、旧/新用量、服务状态表、配置和PG/Redis，不删列/表/触发器，不用旧schema覆盖数据库。备份须可读；数据库恢复及可能丢失备份后写入另行授权并在隔离环境验证。

最近已登记生产基线是personal `3d5e1fde...` / `0.2.14-klno.5-tps.2` / digest `sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530`；本轮未连接生产重新核验。历史记录 `20261010T120908Z-b497b80ee8a6` 只绑定当时tps.2→tps.1回退，不是本候选的新rollback ID，不得用于声称新候选可恢复。本轮没有新部署记录，也没有可执行的新候选回退命令。

### 证据、限制与停止位置

14份S1.5.1/S1.5.4证据、12最终源输入、11原日志、6第一批产物及未变下游/DTO/前端输入核验后复用。没有新跑行为测试、构建、fresh复核或完整CI；相同候选既有复核有效，本轮文档不修改门禁/部署工具行为。真实供应商付费、新增CC→worker→SQL→页面整体执行、无typed usage零终态/data包装的行为覆盖及生产回滚/数据库恢复限制仍如原证据登记。

[候选](evidence/s1.5.5-candidate.json)、[门禁与回退](evidence/s1.5.5-gates-and-rollback.json)、[复用清单](evidence/s1.5.5-reused-evidence.json)、[验证](evidence/s1.5.5-validation.json)、[执行证据](implementation-evidence.md#s155-第二批交付记录)。S1.5.5勾选表示**本地交付记录完成**；合并/发布/部署独立未执行。可选下一项S5.1用户支持需求确认仍未排期，本会话到此停止，不自动进入。

## S1.5 候选远端门禁

按本轮明确授权，将相同完整候选 `1ae266791d0ab324cd13a63ab344019c79102176` 非强制推送到 `codex/verify-personal-ci-plus-chat-s155`，创建并保持 [草稿PR #8](https://github.com/ccisnoxx/sub2api/pull/8)（open，base=personal）。该分支名是现行backend/security workflow排除重复push检查的预留范围；没有改workflow或使用cwd未合入的CI选择优化。应用本地分支/HEAD与脏文档保持，维护证据另行本地提交，不推进PR源码SHA。

| 门禁/交付项 | 本轮结果 | 适用边界 |
|---|---|---|
| PR候选、来源与最新personal基础 | 开始/结束binding成功；head `1ae266791d0ab324cd13a63ab344019c79102176`，base `3d5e1fde82707900a21f2b5538b112c7bab3c04a`，同仓库open草稿PR | 基础或候选变化必须重新绑定 |
| 必要Personal CI | [38065981816](https://github.com/ccisnoxx/sub2api/actions/runs/38065981816) 原生pull_request事件，run attempt 1，全部11项success；App15368 personal-ready成功 | 含现行backend、frontend、security、TPS、sync及脚本/辅助工具合同；无dispatch/重试/重复push全套CI |
| 合并后personal SHA门禁 | 未执行 | PR检查不能替代未来merge SHA自身的门禁 |
| 合并、版本/镜像发布及生产更新 | 未执行 | 无新版本、digest或部署/rollback记录 |
| 新运行树部署/回退兼容 | 未审定 | S1.5.5既有E_COMPATIBILITY证据与数据保留条件继续适用；CI成功不等于部署owner放行 |

当前remote personal和main均未前进；候选未改变schema/迁移、依赖、镜像或部署配置，继承251–254。原数据/账本、PG/Redis、备份/恢复与撤回12路径的说明保持；历史tps.2及旧rollback ID不用于本候选。门禁证据见[远端结果](evidence/s1.5.5-remote-gates.json)，复用边界见[复用清单](evidence/s1.5.5-remote-reused-evidence.json)。到此停止，可选S5.1未排期，不自动进入。
