# UPSTREAM-007：personal 基线切换与验收协议

本任务仅在已验收 UPSTREAM-002—006 之上完成全量回归、来源绑定和受保护分支切换。本文保存固定输入、验收入口与回滚合同；实际完成结果由任务 PR、绑定源码的 CI / dry-run 产物和 REMEDIATION_PLAN.md 完成记录提供。本文本身不代替这些执行证据。

## 固定输入与真实历史

| 输入 | 完整提交 / 标签 |
| --- | --- |
| 旧 personal A / 固定回退 tag | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` / `v0.2.14-klno.5-tps.2` |
| 冻结新 KIN N | `ca686de5e5ac14834e12701c156831abd5ec6b71` / `v0.2.15-klno.2` |
| 累计重放 R（UPSTREAM-006） | `b656fde65419d729dabfad7f7457f17cb7021fd7` |
| 真实历史连接 | `70cace8dbedcf4c3919282303fe5fa1b908dcee7` |
| main 控制面快照 | `c1dae6357b4de7cc11d81547013b87550c4ba586` |
| LEGAL-002 许可 / Airwallex 停用 | `0ea69e62ee0079ee2616273f627ed97cc3f57b5c` |
| SEC-002 严格审计 | `e84f17fee7a100f27d958fd05ea3db4ffec3f793` |
| DOC-002 安装来源修正 | `e0f1643daef7847d8ff2c144de82b0ad758964ca` |

开始时原维护分支为 `codex/doc-002-install-sources`，HEAD 为 DOC-002 上表提交，工作树仅有维护者保留的 `AM REMEDIATION_PLAN.md`。实施使用从 A 创建的独立 `codex/upstream-007-baseline-switch`。迁移前重新查询最新 KIN 稳定 Release/tag，仍与 N 一致；结束前再次查询，变化时按计划停止。

历史连接的两个实际父为 A 与 R，连接后的应用 tree 与 R 完全相同。随后按上表来源适配必要控制/许可/安全/安装文件；不导入 main 的应用树，不把旧 KIN 误作 N 的祖先。来源文件为 [deploy/personal-source.json](../../deploy/personal-source.json)，三字段指向 N。PR 必须采用普通 merge，保持 A、N 和各阶段累计历史。

## 发布与保护边界

- personal 分支保护继续要求 strict `personal-ready`，强推与删除仍禁止，管理员不绕过。Personal CI 对固定候选 SHA 执行 binding、共享 CI、共享 Security、TPS、sync-contracts，并在 personal-ready 前重新核对当前 base 与 PR head。
- 维护者明确授权阶段 002—006 使用同 SHA 标准 PR CI + Security Scan；007 必须恢复实际 Personal CI / personal-ready，不继续沿用阶段例外。
- 仓库原 `PERSONAL_RELEASE_ENABLED=true` 会在合并后的成功 push Personal CI 自动进入正式发布。维护者已明确授权本任务设为 `false` 并保持关闭，后续正式发布任务再决定开启；本任务不创建正式发行 tag、不推镜像、不部署生产。
- Release dry-run 应使用候选分支上的已验收许可工作流，应用选择为固定完整源码 SHA：提交后用 `git rev-parse HEAD` 取得 `UPSTREAM007_SOURCE_SHA`，再执行 `gh workflow run release.yml --repo ccisnoxx/sub2api --ref codex/upstream-007-baseline-switch -f tag="$UPSTREAM007_SOURCE_SHA" -f source_sha="$UPSTREAM007_SOURCE_SHA" -f simple_release=true -f dry_run=true`。平台与现有个人发布一致，为 Linux amd64。
- dry-run 版本来自该源码的 `backend/cmd/server/VERSION`，目前继承值是 `0.2.14`；它不表示 KIN 来源 tag，也不分配 `-tps.N` 正式版本。正式版本分配仍由后续发布门禁负责。
- 保存实际 run/attempt、workflow revision、应用 SHA、artifact ID/digest/expiry。校验真实 binary archive、SOURCE.json、全 SHA、OCI descriptor、revision/source/version 和镜像五份许可材料。候选 C 与最终 merge M 为不同 SHA 时，C 的 OCI 不冒充 M；最终 M 另作 dry-run 来源身份验收。

## 数据库与应用回退

SQL 251—254 原文件名与原字节继续保留，全部 308 个 SQL 和 migration runner 与 R 相同。本任务不改 SQL/Ent/schema，不重置迁移账本。迁移差异审核延续 [MIG-001](../migrations/MIG-001.md)：空库 0→308、旧 KIN 303→308、已迁移 personal 307→308，后一条只增加 KIN 242 约束移除；其 checksum 和失败原子性合同保持。

真实应用生命周期入口见 `backend/internal/repository/migrations_upstream007_integration_test.go`；按 `integration,upstream007` 明确选入，缺候选镜像或 SHA 明确失败。候选镜像须从同 SHA dry-run 产物取得并验证 OCI revision。每条路径核对旧 checksum/applied_at、308 条账本、合成 usage 数据与个人状态 facts/version，再依次验证新候选健康、固定旧应用健康、再次新候选健康和真实 dump/restore。服务状态在演练库禁用，避免异步聚合改写合成 fixture；不更改生产开关。

固定旧镜像为 `ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530`。应用回退保留扩展 schema/ledger，不以删除 SQL 列或回退数据库证明兼容。dump/restore 使用隔离合成库；不复用生产备份证明新应用行为。生产恢复仍需维护窗口和当时恢复点，见 [旧稳定基线](../../BASELINE.md)。现有主机部署工具的运行树/来源允许清单只批准旧基线；本任务保留其失败关闭合同，未将新 KIN 增量纳入可部署证明。新基线的主机部署适配和部署前验证由后续 REL-001 处理。

## 回归与审查

最终固定候选执行后端 unit/integration、OpenAI recording race、前端 Vitest/typecheck/build、严格安全扫描、部署脚本/同步与发布工具合同。按现有 Makefile 和 workflow 实际命令，不删除失败测试或放宽断言、安全级别、例外版本/路径/期限及祖先检查。全量前端回归发现继承的 AccountTableFilters 测试漏列现有 CPR 类型；仅补入明确选项期望，继续通过真实点击验证全部选项的合并 filter/change 事件，不修改运行代码。

已验收 LEGAL-002 应用补丁仅禁用 Airwallex 新单/新实例/SDK 路由，保留历史查询/取消/退款/callback；旧 SDK 专用生命周期测试按停用合同适配，Stripe 与新 KIN 平台配置保留。SEC-002 的 XLSX 严格边界和 policy 测试从固定 S15 保留。来源、NOTICE、授权与许可全文保持可追溯，CRLF 审计文件保留来源字节，diff whitespace 检查明确识别合法 CRLF。

最终候选须经 fresh 只读独立审查，覆盖真实合并历史、精确来源、required checks、禁止自动正式发布、许可/安全组合、支付历史恢复与数据库验收入口。审查结果与未独立执行的验证边界记录于 PR；自身检查不冒充独立 review。

## 切换与回滚

合并前确认 PR head/source、最新 personal base、最新 KIN 与旧稳定 tag，所有 required checks 成功且独立审查无未解阻断，自动发布开关为 false。通过受保护 PR 普通 merge 后核对实际 personal SHA、A/N 祖先、来源字段、保护与旧 tags，再取得实际 personal SHA 的 Personal CI / personal-ready 和 dry-run 身份证据。

源码回滚通过新的受保护 PR 撤销本任务应用增量或建立明确的旧应用 tree；禁止强推、删除旧 tag、伪造来源或绕过保护。运行回退选择上面固定旧 digest 并保留扩展 schema；本任务不执行生产回退或恢复。若重新启用发布，需另按正式发布任务核对精确 SHA 的成功 CI、材料和部署恢复点。
