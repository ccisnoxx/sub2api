# 开发日志

## 2026-10-11：MIG-001 迁移身份与兼容审核

- 从冻结 KIN `ca686de5e5ac14834e12701c156831abd5ec6b71` 创建隔离任务候选。生产与 S44 测试库只读核对均为固定 personal 的 307 条迁移；保留已执行 251–254 的原文件名和原 SQL 字节，没有修改 runner 或 checksum 例外。
- [完整身份清单](migrations/MIG-001-inventory.json)记录目标 304 个 SQL、旧 KIN 303 个共同 SQL 与四份个人迁移的差集；新 242 约束移除纳入升级、回退和锁影响审核。
- [MIG-001 记录](migrations/MIG-001.md)覆盖空库、旧库、已迁移库、旧应用启动、真实合成库备份恢复、锁失败重试与错误 checksum 拒绝；后续 S1–S4、personal 切换和发布部署尚未实施。
- KIN 树的 AGENTS 引用出站身份 conventions、既有 journal 和 docs/tasks，但本冻结树没有这些文件；仓库其他当前固定候选也未找到该 conventions。已读取可获取的个人历史 journal，未导入无关历史或改变出站行为。本条为该干净树的首个任务日志。

## 2026-10-11：UPSTREAM-002 用量计时迁移

- 在冻结新 KIN 与已验收 MIG-001 之上重放原生 Responses 的严格首输出、末次 Token、完成状态、音频与用量来源；保留旧首字、计费/幂等、重试和异步 owner。
- [阶段报告](upstream/UPSTREAM-002.md)登记 HTTP/SSE/透传、pooled WS 多 turn、WS→HTTP bridge、取消/未知边界，真实 SQL/Ent 集成和观察器/async race 的实际证据。包装 usage 与双 usage 音频缺失保持原 parser 选择和 NULL 语义。
- 没有改迁移 251–254、出站身份、前端或发布控制面。维护者授权阶段标准 PR CI 与 Security Scan；最终 Personal CI/来源切换保留到 UPSTREAM-007。独立复核和远端固定候选结果以该阶段 PR 记录为准。

- 首轮阶段 CI 发现旧用量 API fixture 未包含九个新增观测字段，补齐历史行的 NULL/unknown/version 0 预期；定向 API 合同 12 项通过。独立只读复核运行实现未确认阻断，最终阶段门禁绑定后续提交。

## 2026-10-11：UPSTREAM-003 用户授权模型与价格目录迁移

- [阶段报告](upstream/UPSTREAM-003.md)记录固定 S1 候选之上的目录重放、真实授权/订阅 owner、精确 opt-in、白名单 DTO、未知价格与明确零价，以及遵循现有唯一绑定约束的歧义失败。
- 准备代码独立只读复核无确认阻断；正式整合后 161 项定向 unit、真实 PostgreSQL/Redis 目录合同与项目 build 通过；前端与准备候选一致。
- 不写计费/用量或发起上游请求，迁移/控制面不变；前置及阶段远端固定候选门禁完成前不宣告任务验收。

## 2026-10-11：UPSTREAM-004 结构化路由诊断迁移

- 在冻结新 KIN 与 MIG-001 上重放实际调度评估快照、HTTP/WS turn 隔离、可空持久化和管理员单记录详情边界；保留 sticky、guardian、previous response、等待与 gateway pool owner。
- [阶段报告](upstream/UPSTREAM-004.md)记录候选池/过滤事实、非法诊断不得丢失真实错误、实际 socket/WS 计费 owner、真实 PostgreSQL 和 JWT 权限测试；定向 race 与项目 `make -C backend build` 通过。
- 迁移 251–254 原名原字节、runner 和依赖未变。独立复核与固定候选阶段 PR CI / Security Scan 由父任务完成；Personal CI 与来源切换留到 UPSTREAM-007。

- 阶段 lint v2.14.0 报新增报价平台分支 QF1003，局部等价 switch 修正；报价/倍率定向检查通过，未修改 lint 配置。最终阶段验收使用修正候选。

## 2026-10-11：UPSTREAM-005 服务状态后端迁移

- [阶段报告](upstream/UPSTREAM-005.md)记录真实逻辑 turn 的匿名 source、独立 observation key、首可信终态/取消冻结、配置 version/epoch、事务聚合水位、可见性与新证据恢复、容量/隐私和严格管理员 JWT。
- 正式累计依赖保留已验收 S1/S3 和 S2 目录 owner；按维护源重新生成 Ent/Wire，无生成漂移，Billing 注入与清理顺序保留。真实 PostgreSQL 整合检查和项目 build 通过；其余固定候选结果以阶段 PR 为准。
- SQL251–254 原名原字节、runner、依赖和控制面未改；默认关闭，不在本阶段生产开启。阶段 PR CI + Security Scan 经维护者授权，Personal CI 与来源切换留007。

## 2026-10-11：UPSTREAM-006 前端、导出与管理界面迁移

- [阶段报告](upstream/UPSTREAM-006.md)记录UsageTable严格/旧计时、全时长文本TPS、12追加导出列、目录、管理员诊断和服务状态设置/历史、中英文及键盘/窄屏行为。
- 正式前端与已测试/独立复核增量逐字节一致，复用263个Vitest、最终107个受影响测试、typecheck/lint/build和实际Chromium八环境/导出文件证据。正式后端不变，实际DTO/JWT/数据库与前端合同核对；共同owner的race/PG证据来自S4。
- 本阶段不修改backend、SQL、依赖或控制面；浏览器合成API边界保留。同固定候选PR CI + Security Scan及直接前置全通过后才勾选任务，Personal CI和来源切换留007。

## 2026-10-11：UPSTREAM-007 受控 personal 基线切换

- 在已验收 S1–S5 的累计新 KIN 应用树之上，用真实 merge 连接旧 personal 历史；只适配固定 main 控制面和已验收许可/安全/安装补丁，来源切换为冻结新 KIN。旧稳定 tag、迁移身份和严格祖先/Personal CI 保护保留。
- [切换验收说明](upstream/UPSTREAM-007.md)登记源提交、真实三路径与新/旧/新应用演练入口、精确源码 Release dry-run、产物审查、普通 PR 合并及回滚边界。实际结果以固定候选 PR checks 与任务完成记录为准。
- 维护者明确授权关闭自动正式发布；本任务保持开关 false，只执行 dry-run，不创建正式发行 tag、不部署生产。

## 2026-10-11：REL-001 发布候选与部署前验证

- 按维护者授权依序独立审查并普通合入 main 的 CI-001—005；新 personal 候选仅同步必要控制文件，保留应用/许可/安装/安全边界，预发布 tag 避开正式发布触发器。
- [候选验证合同](release/REL-001.md)定义同 SHA 的完整 Personal CI、全矩阵 Release dry-run/材料验证、合成数据库三路径与真实恢复、固定旧/新网关性能比较，以及告警/默认开关覆盖。最终固定候选结果在 REL-001 PR 与任务台账，不把此合同当作已执行验收。
- 不新增 SQL、不改迁移身份/checksum、不正式发布或部署。主机前置证明精确绑定固定新 KIN 运行树；跨基线旧镜像恢复继续失败关闭。

## 2026-10-12：REL-001 原生复测与 CI-000 控制续接

- 同原候选 OCI/SHA 的原生 amd64 完整五轮通过；旧 QEMU 7.0.0 的 MADV_DONTNEED 错误成功和堆回收反例支持环境归因。实际门禁不设置诊断 workaround，应用代码和 SQL 不变，详见 [候选验证合同](release/REL-001.md)。
- 普通 merge 纳入当前 personal 的已验收 CI-000 控制历史；六处独立同步冲突均采用最新 personal 的控制字节。保留原 RC 标签、运行树证明和迁移身份，最终候选按分层规则取得一次新 Full/同 SHA dry-run 材料；结果由任务台账和 PR 记录，全部验收完成前不勾选。

### REL-001：最终控制修复同步（2026-10-12）

- 仅同步 PR #41 的两份 workflow 和 release_control 回归文件：发布资格模块隔离、Full 安装来源/token 必检。没有应用、迁移、VERSION、依赖、阈值、Quick 分类或权限变更。绑定最新 personal 7ebb60b2 后，最终稳定候选必须取得新的 Full/同 SHA 全材料 dry-run，并在原生 amd64 对实际 OCI 重做三路径升级恢复和完整五轮性能。原 036ade78 的原生成功用于定位 OOM，不代替最终新 SHA 验收。

### REL-001：归档来源绑定必要同步（2026-10-12）

- ff6c79c7 同 SHA dry-run 五归档 SOURCE.json 缺失根因明确：恢复 CI-000 同步遗漏的原四行材料入口及 workflow 回归，来源 metadata 动态绑定 prepare 应用 SHA/VERSION。仅同步可信 main 控制补丁 5f6dcba7 的 workflow/test 字节及记录，运行代码、SQL、VERSION 和门禁阈值不变；独立复核和主线 Quick 普通合并后再推最终新候选，旧成功/失败均保留，不冒用为新 SHA 通过。

## 2026-10-12：ARCH-001 第二阶段协议事实解析

- 基于最新 personal 的新手动 CI 流程，从观察器抽出无时钟/请求 owner/持久正文的 Responses 协议事实解析；计时状态更新继续在原 mutex 内排序，第一阶段首次终态分类器保持。
- [阶段报告](architecture/ARCH-001-02.md)记录实现候选、基线黑盒回归、纯组件及实际 HTTP/SSE/WS/计费入口检查、取消/终态定向 Race 和回退边界。输出顺序、起点前音频、终态汇总与 compaction 的既有语义不变。
- 本阶段不改 DTO、数据库、计费 parser、依赖或控制面；暂不发布或部署。整体 ARCH-001 仍待计时、用量来源、观测提交职责的后续小批次，不以本阶段组件抽取勾选整体任务。

## 2026-10-12：ARCH-001 第二阶段合并与第三阶段输出计时

- PR44 普通 merge 到 personal `0ff914f35274d6132825a038cc0757b33243f0fc`，parents 和 tree 与固定基准/候选核对一致，复用成功定向验证和独立复核；没有重新触发应用 CI。
- [第三阶段报告](architecture/ARCH-001-03.md)记录从该基准提取输出计时状态、四个可空输出字段的独立快照、抽取前黑盒回归、组件/实际入口/计费检查和定向 Race。取消与终态排序、音频事实和原用量/观测 owner 保持。
- 没有 SQL、DTO、依赖、权限、发布控制或生产操作。整体 ARCH-001 仍待用量来源、观测提交职责及最终验收；本轮不发布或部署。
