# Sub2API 二开项目修复与治理计划

> 仓库：`ccisnoxx/sub2api`  
> 直接技术上游：`KlN-4096/sub2api`  
> 原始项目链路：`Wei-Shaw/sub2api`  
> 部分代码与设计参考：`LuckyKuang/sub2api-plus`  
> 文档基线日期：2026-10-10  
> 文档用途：供维护者和 Codex 按任务编号逐项实施、验证和交付

> **当前执行策略（2026-10-12，覆盖后文历史流程）：** 普通 push/PR 不运行应用 CI 或安全扫描，不强制每个任务创建 PR。日常按任务批次选择本地定向检查，Quick 仅按需手动执行；准备发布时固定当前 personal 完整 SHA，手动执行一次 Full，再手动发布镜像与候选部署验证。安全扫描改为每月一次；main/personal 只禁止强推和删除。迁移、计费、权限、并发等变化仍做直接覆盖合同的定向检查。旧 Quick/Full 自动触发、required personal-ready 和十任务观察记录仅保留为历史，不再作为当前开发前置。

> **本次授权补充（2026-10-12）：** 维护者撤销此前“暂不发布或部署”限制，按 REL-002 → REL-003 → ARCH-002 顺序执行固定源码 Full、手动镜像发布及已核实测试/生产目标的分阶段部署。历史记录中的限制保留其当时边界；当前进度和验收见 [REL-002 续作](docs/upstream/records/2026-10-12-rel-002-recovery.md)、[REL-003](docs/upstream/records/2026-10-12-rel-003.md) 及 [ARCH-002](docs/upstream/records/2026-10-12-arch-002.md)；三项均已实际验收。随后维护者授权继续第8节：依次整合OBS-002、OBS-001/004与实际监控，最后验证跨KIN回退；保持当前基线和正式产物，未证明无损时保留禁令。当前结果见[第8节续作](docs/upstream/records/2026-10-12-dod-observability-rollback.md)。 维护者随后明确不需要外部通知渠道，真实通知送达按本次部署需求记为不适用；已授权另建新OBS版本并分阶段部署，保留旧正式产物。发布与本机监控验收见[OBS新版本续作](docs/upstream/records/2026-10-12-obs-release.md)。

---

## 1. 文档目标

本计划用于解决当前二开项目在以下方面的主要问题：

1. 项目说明、安装入口、分支职责和镜像来源不够准确；
2. 第三方来源、代码借鉴和许可证归属说明不完整；
3. 安全报告渠道仍指向原上游，审计例外存在占位信息和临近到期项；
4. 当前 `personal` 基线与新的 KIN 上游已经发生历史分叉，不能继续按普通快进同步处理；
5. CI、发布链和构建工具仍有部分可变依赖，供应链可复现性不足；
6. 用量计时、路由诊断、服务状态和部署脚本规模较大，后续维护成本较高；
7. 服务状态与诊断数据需要进一步落实容量、隐私、保留和运行监控。

本计划不是新增产品功能计划。除兼容迁移所必需的调整外，实施期间不主动扩大业务范围。

---

## 2. 已知基线与执行前置检查

截至本计划形成时，已知版本关系如下：

| 项目 | 版本或分支 | 提交 |
|---|---|---|
| 当前 KIN 基线 | `v0.2.14-klno.5` | `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` |
| 重新冻结的 KIN 目标（UPSTREAM-001） | `v0.2.15-klno.2` | `ca686de5e5ac14834e12701c156831abd5ec6b71` |
| Plus 参考基线 | `v0.2.14+custom.002` | `90da415c62b94c9417d9ce2b72b1507ed22f0303` |
| 当前个人迁移 | `251`—`254` | 用量计时、路由诊断、服务状态相关 |

**重要：Codex 开始执行上游迁移任务前，必须重新查询 KIN 的最新稳定标签与提交。**

若最新稳定版本已经变化，不得直接沿用表中目标提交。应先更新本节和 `UPSTREAM-001` 的目标基线，再继续执行。

2026-10-11 重新核对：官方最新稳定 Release 仍为 `v0.2.15-klno.2`，但 annotated tag object `0c1ede31ec63f9cfc2b72f07db66fb41f540208d` 实际解析为上表提交，发布时间为 `2026-10-10T07:54:36Z`。原计划记录 `94db97ee634340ddc74cb548ec92f80c7d3edc65` 与当前官方结果不一致；按 §6.4 停止后，维护者明确确认采用当前官方 tag/SHA 并重新冻结。现有证据不能证明标签曾被改写。

### 2.1 必须保持的安全边界

- 不得通过关闭祖先检查来“修复”上游分叉；
- 不得强推 `main` 或 `personal`；
- 不得直接在受保护分支上实施大规模迁移；
- 不得删除、改写或伪造已经在生产环境执行过的迁移身份；
- 不得为通过 CI 而降低安全扫描等级、删除失败测试或扩大审计例外；
- 不得把哈希后的内部标识当成授权凭证；
- 不得在日志、CI artifact 或 PR 描述中提交凭据、令牌、完整用户请求或敏感数据库内容；
- 发布任务必须继续以精确源码 SHA、同一次成功手动 Full CI 和镜像摘要绑定。

---

## 3. 优先级与任务状态约定

### 3.1 优先级

| 优先级 | 含义 |
|---|---|
| `P0` | 阻塞安全、合规、发布可信度或后续上游迁移，优先完成 |
| `P1` | 明显影响供应链、运行可靠性和可维护性，应在下一稳定发布前完成 |
| `P2` | 中期架构治理和质量改进，可在基线迁移稳定后实施 |

### 3.2 任务状态

- `[ ]` 未开始
- `[x]` 已完成并满足验收标准

任务只有在以下条件全部满足后才能勾选：

1. 代码或文档已经提交；
2. 对应测试已通过；
3. 交付记录说明实际风险和必要回退方法；需要 PR 时同时写入 PR；
4. 验收标准逐项核对完成；
5. 未引入新的占位值、失效链接或未解释例外。

### 3.3 当前状态核对与历史记录边界（2026-10-12）

本次文档核对固定输入为 main `320f9e36cfa0f770d7cb490f9b861c3a004b7255`、personal `00fa2a3d33570e931c8cb7f483589ad7b2db35ce`。第 4、5 节的任务勾选沿用各任务已约定的准备、候选或整合验收边界；第 8 节核对当前控制面、已整合应用及最终发布闭环。候选任务完成不自动等于已合并、已发布或已部署。

下文记录中的“未完成”“尚未合并”“当前”“下一任务”和保护配置均描述记录当时的状态。后续完成或替代记录更新当前结论，但不改写原始失败、取消、跳过、审计异常和未验证边界。

- **CI 流程：** CI-001—005 的 [PR #26](https://github.com/ccisnoxx/sub2api/pull/26)—[PR #30](https://github.com/ccisnoxx/sub2api/pull/30) 已普通合入 main；必要控制文件随后经 CI-000 同步到 personal。CI-000 已按“当前简化验收”记录关闭，旧自动 Quick/Full、strict personal-ready、周扫和十任务观察要求已被替代，不再按旧记录判定任务未完成。此次 API 回读两分支均禁止强推/删除，required checks 与 PR 审查要求为空。
- **已整合应用：** UPSTREAM-007 已随 [PR #25](https://github.com/ccisnoxx/sub2api/pull/25) 完成基线切换；DOC-002、LEGAL-001/002、SEC-002 和 S1—S5 的必要材料以该任务完成记录及实际 personal 文件为准。原阶段 PR #14—#24 仍有开放草稿，不能写成逐个已合并，也不能仅凭 PR 开放状态认定必要材料未整合。UPSTREAM-008 已普通快进合入 personal；仅文档同步至 main。
- **结构治理与候选：** REL-001 的 [PR #36](https://github.com/ccisnoxx/sub2api/pull/36)、ARCH-001 的 [PR #43](https://github.com/ccisnoxx/sub2api/pull/43)—[PR #47](https://github.com/ccisnoxx/sub2api/pull/47)、ARCH-004 的 [PR #48](https://github.com/ccisnoxx/sub2api/pull/48) 已合入 personal；ARCH-005 的 [PR #49](https://github.com/ccisnoxx/sub2api/pull/49) 已合入 main。LEGAL-002 的早期许可材料阻塞及 REL-001 的早期性能失败已由后续候选验收记录替代；原失败和 REL-001 合并轮 digest 校验失败仍保留，不改记为成功。
- **安全例外续作：** 上轮发现的 main 过期条目和占位 owner 已在本轮 SEC-002 定向修复中清理。两分支实际生产审计均为 high 2 / critical 0，仅保留 `xlsx@0.18.5`、完整路径 `. > xlsx@0.18.5` 的两项精确 GHSA 例外，owner 为 `@ccisnoxx`，到期日 `2026-11-09`。main 无对应发现的 lodash、lodash-es、axios、nanoid 例外已删除；严格版本/路径/owner/报告校验与 XLSX 写出门禁已有定向通过证据，两分支 Security Scan 补入已有政策/规则检查入口。候选和边界见 SEC-002 续作记录；不表示 XLSX 包内漏洞已移除或已经发布。
- **OBS 整合及运行验收（2026-10-13 UTC）：** PR #32/#31/#34（含#33）按序普通合并，应用门禁修复锚点b663258，发布源码 `c08fba12eb4cd30e65c4177f3acbe20567ecd252`；定向验证、fresh最终复核及封存审计完成。新tps.2固定Full38335510831全部15 job成功，Release38337243568、独立测试→生产、本机producer和每分钟timer及故障/恢复均实际验收。外部通知按维护者明确需求不适用，未配置或发送。见[OBS验收](docs/upstream/records/2026-10-12-obs-release.md)。
- **发布与回退：** REL-002/003、ARCH-002历史验收保持。当前生产为 `c08fba12eb4cd30e65c4177f3acbe20567ecd252` / `v0.2.15-klno.2-tps.2`，本轮发布与部署控制为 `90aea7ec626aeab41434d72739573aa51d6227ad`，index `sha256:c4ffab7a920f05564eafe055bdb3171772b145ca5e2ccda690fe8b554bf6fa25`；旧429/tps.1/Full38274106520/Release410901327/FA digest及旧KIN基线全部保留。旧JSONB回写丢失 `quality_gateways`，联系人64/300和多协议凭据仍不兼容；禁止自动、显式和普通部署回旧镜像，完整恢复须单独确认快照后写入损失。

---

## 4. 总体任务清单

### 当前优先插入任务

- [x] `CI-000` 个人项目手动 CI、月度安全扫描与轻型分支保护（2026-10-12 按维护者最新方案完成；旧十任务观察要求由新流程取代）

### 里程碑 M0：冻结现状与建立可回滚基线

- [x] `PREP-001` 冻结当前源码、镜像、CI、数据库和部署基线
- [x] `PREP-002` 建立任务分支、提交和 PR 规则
- [x] `PREP-003` 建立功能来源与代码归属清单

### 里程碑 M1：公开文档、归属和安全渠道修复

- [x] `DOC-001` 重写 README 顶部说明、分支职责和安装入口
- [x] `DOC-002` 修正所有部署、克隆、镜像和更新命令
- [x] `LEGAL-001` 新增第三方归属与来源说明
- [x] `LEGAL-002` 审核许可证副本、文件头和发布产物中的许可证材料
- [x] `SEC-001` 重写本 fork 的安全政策与漏洞报告渠道
- [x] `SEC-002` 处理即将到期或不完整的依赖审计例外

### 里程碑 M2：基于新 KIN 的人工重放迁移

- [x] `UPSTREAM-001` 重新确认目标 KIN 并创建干净集成分支
- [x] `MIG-001` 审核迁移编号、迁移账本和生产兼容策略
- [x] `UPSTREAM-002` 重放 S1 用量计时与完成状态
- [x] `UPSTREAM-003` 重放 S2 用户模型与价格目录
- [x] `UPSTREAM-004` 重放 S3 结构化路由诊断
- [x] `UPSTREAM-005` 重放 S4 服务状态观测与聚合
- [x] `UPSTREAM-006` 重放前端、导出、文案和管理界面改动
- [x] `UPSTREAM-007` 完成全量回归并切换 `personal` 基线
- [x] `UPSTREAM-008` 补回个人更新服务与版本徽章（独立迁回缺口；已推送并快进合入 personal，暂不发布或部署）

### 里程碑 M3：CI、发布链和供应链加固

- [x] `CI-001` 将 GitHub Actions 固定到完整提交 SHA
- [x] `CI-002` 固定安全扫描和构建工具版本
- [x] `CI-003` 增加 SBOM、构建来源和镜像证明
- [x] `CI-004` 增加发布控制面与源码 SHA 绑定测试
- [x] `CI-005` 清理工作流权限、凭据持久化和 artifact 保留策略

### 里程碑 M4：功能与代码结构治理

- [x] `ARCH-001` 拆分流式协议解析、计时、完成分类和观测上报职责
- [x] `ARCH-002` 拆分主机部署脚本和环境配置
- [x] `ARCH-003` 修复模型目录中的多渠道绑定歧义
- [x] `ARCH-004` 为个人功能建立独立 feature flag 和降级路径
- [x] `ARCH-005` 建立长期上游来源矩阵与变更记录

### 里程碑 M5：诊断、状态数据和运行治理

- [x] `OBS-001` 增加服务状态事实表容量、清理和聚合耗时监控
- [x] `OBS-002` 增加低样本保护、查询限流和诊断数据隐私控制
- [x] `OBS-003` 固化 `unknown`、`operational`、`degraded`、`outage` 等状态语义
- [x] `OBS-004` 增加服务状态聚合任务故障告警和运行手册

### 里程碑 M6：发布与收尾

- [x] `REL-001` 构建迁移后的候选版本并完成部署前验证
- [x] `REL-002` 在测试环境和生产环境分阶段发布
- [x] `REL-003` 完成发布后验证、文档更新和旧基线归档

---

# 5. 详细任务说明

## PREP-001：冻结当前源码、镜像、CI、数据库和部署基线

**优先级：** P0  
**依赖：** 无  
**建议分支：** `chore/prep-baseline-freeze`

### 目标

在任何大规模修改前，保留可复现、可验证、可回滚的当前状态。

### 实施步骤

- [x] 记录 `main`、`personal` 和当前生产部署的完整 Git SHA；
- [x] 记录当前正式标签、镜像标签、镜像 digest 和 OCI revision；
- [x] 保存最近一次成功的 Personal CI run ID 和 Release run ID；
- [x] 导出当前数据库迁移账本，只保留迁移身份和执行时间，不导出业务数据；
- [x] 记录当前数据库 schema 摘要；
- [x] 记录当前运行配置中的功能开关，但不得提交密钥或完整环境变量；
- [x] 验证当前版本的数据库恢复流程；
- [x] 将以上信息写入受控的发布记录或私有运维记录。

### 验收标准

- 可以从源码 SHA、镜像 digest 和数据库备份恢复到当前稳定状态；
- 发布记录中不存在明文密码、Token、Cookie、私钥或连接串；
- 当前所有已执行的迁移身份均有记录；
- 至少完成一次非生产恢复演练。

### 回滚

本任务仅收集和验证信息，不修改业务逻辑；若发现敏感信息被提交，应立即删除提交、轮换相关凭据，并按安全事件处理。

---

## PREP-002：建立任务分支、提交和 PR 规则

**优先级：** P0  
**依赖：** `PREP-001`  
**建议分支：** `chore/codex-task-policy`

### 目标

保证 Codex 后续按任务编号工作，避免再次形成难以审查的超大 PR。

### 实施步骤

- [x] 新增 `CONTRIBUTING_PERSONAL.md` 或等价维护文档；
- [x] 规定一个 PR 默认只完成一个任务 ID；
- [x] 规定提交标题格式：`<type>(<scope>): <TASK-ID> <summary>`；
- [x] 规定 PR 必须包含：变更摘要、测试、风险、数据库影响、回滚步骤；
- [x] 规定不得跳过失败测试或擅自扩大审计例外；
- [x] 规定数据库任务必须附带升级和回退兼容说明；
- [x] 规定涉及 `main` 控制面的改动必须独立审查。

### 验收标准

- 仓库内存在明确的 Codex 执行规则；
- PR 模板包含任务 ID、依赖和验收清单；
- CODEOWNERS 或审查规则覆盖发布和安全控制文件。

---

## PREP-003：建立功能来源与代码归属清单

**优先级：** P0  
**依赖：** `PREP-001`  
**建议分支：** `docs/source-provenance`

### 目标

明确每项功能来自 KIN、Plus、原始上游还是自主实现，为许可证、维护和重放迁移提供依据。

### 实施步骤

- [x] 新增 `UPSTREAM.md` 或 `docs/upstream-provenance.md`；
- [x] 建立以下字段的来源矩阵：模块、来源仓库、来源 tag、来源 commit、改动方式、当前 owner；
- [x] 至少覆盖用量计时、模型目录、路由诊断、服务状态、部署和发布工具；
- [x] 标记“直接复制”“结构参考”“概念参考”“自主实现”；
- [x] 列出重放到新 KIN 时的关键文件和测试入口；
- [x] 对无法确定来源的代码进行人工复核，不得猜测。

### 验收标准

- 每个个人新增核心模块均能追溯到明确来源；
- Plus 参考版本和提交与 NOTICE 一致；
- 来源矩阵不声称上游维护者为本 fork 提供支持或背书。

---

## DOC-001：重写 README 顶部说明、分支职责和安装入口

**优先级：** P0  
**依赖：** `PREP-003`  
**建议分支：** `docs/readme-fork-identity`

### 目标

让访问仓库的用户能够立即理解这是哪个项目、实际代码在哪个分支、应该安装哪个版本。

### 实施步骤

- [x] 在 README 顶部增加醒目的 fork 关系说明；
- [x] 说明 `main` 是同步、发布和部署控制面；
- [x] 说明 `personal` 是实际个人应用代码分支；
- [x] 说明直接技术上游为 `KlN-4096/sub2api`；
- [x] 说明部分功能借鉴 `LuckyKuang/sub2api-plus`；
- [x] 明确默认推荐安装方式、镜像仓库和支持范围；
- [x] 增加“与上游差异”章节；
- [x] 增加“不代表上游维护者背书”的声明；
- [x] 标注通用上游文档与本 fork 文档的边界。

### 建议文案要点

```text
本仓库是 KlN-4096/sub2api 的个人维护发行版，
并选择性借鉴 LuckyKuang/sub2api-plus 的部分功能。

main：同步、发布和部署控制工具
personal：实际应用代码与个人功能
```

### 验收标准

- 用户不查看其他文件也能确定应使用哪个分支和镜像；
- README 不再暗示这是原始上游的官方仓库；
- README 中所有安装示例均与本 fork 的实际发布流程一致；
- 中英文 README 的核心说明保持一致。

---

## DOC-002：修正所有部署、克隆、镜像和更新命令

**优先级：** P0  
**依赖：** `DOC-001`  
**建议分支：** `docs/fix-install-links`

### 目标

避免用户按文档安装到 `Wei-Shaw/sub2api` 或其他错误来源。

### 实施步骤

- [x] 搜索 README、部署文档和脚本中的 `Wei-Shaw/sub2api`；
- [x] 区分“来源归属链接”和“实际安装命令”；
- [x] 将实际安装、克隆和更新命令指向 `ccisnoxx/sub2api`；
- [x] 明确检出 `personal` 或指定稳定 tag；
- [x] 修正 GHCR 和其他镜像名称；
- [x] 删除未实际维护、未验证或已失效的安装方式；
- [x] 为保留的每种部署方式补充最后验证日期和适用环境；
- [x] 增加文档链接检查。

### 验收标准

- 安装命令不会下载原上游版本；
- 所有镜像名称与 Release 工作流发布目标一致；
- 所有保留的 URL 可访问；
- `grep` 搜索到的原上游链接只出现在归属、对比或引用语境中。

### 建议测试

```bash
rg -n "Wei-Shaw/sub2api|KlN-4096/sub2api|ccisnoxx/sub2api" README* docs deploy .github

docker compose config
```

---

## LEGAL-001：新增第三方归属与来源说明

**优先级：** P0  
**依赖：** `PREP-003`  
**建议分支：** `docs/third-party-notices`

### 目标

公开、准确地说明 fork 链、Plus 借鉴范围和修改关系。

### 实施步骤

- [x] 新增根目录 `NOTICE` 或 `THIRD_PARTY_NOTICES.md`；
- [x] 列出 `Wei-Shaw/sub2api`、`KlN-4096/sub2api` 和 `LuckyKuang/sub2api-plus`；
- [x] 固定记录 Plus 参考 tag 与 commit；
- [x] 列出借鉴模块：用量计时、模型目录、路由诊断、服务状态；
- [x] 明确已经针对 KIN 架构做出实质修改；
- [x] 明确不代表任何上游维护者认可、支持或背书；
- [x] 在 README 中链接 NOTICE；
- [x] 对直接复制或显著改写文件保留原始版权头和 SPDX 信息。

### 验收标准

- 根目录可直接发现第三方归属文件；
- 来源 tag、commit 和仓库名称与 `UPSTREAM.md` 一致；
- 没有把“参考设计”错误写成“完全自主实现”；
- 没有把“使用相同许可证”错误解释成“不需要署名”。

### 完成证据（2026-10-10）

- 前置 `PREP-003` 已随 [PR #11](https://github.com/ccisnoxx/sub2api/pull/11) 合并，合并提交为 `24daf87641750fb78a3ad5818e0eb3eb392d638e`。
- 本任务位于 `codex/legal-001-third-party-notices`，提交为 `dbb4cb099f96218e7cb53040fadcd17467936541`；[草稿 PR #16](https://github.com/ccisnoxx/sub2api/pull/16) 仅修改 `NOTICE`、`UPSTREAM.md` 与 `docs/legal/LEGAL-001.md`，尚未合并、发布或部署。
- 补齐四个 Plus 借鉴模块及 KIN 实质修改关系、署名与维护 owner 边界、无上游认可/支持/背书声明；保留三份 README 已有 NOTICE 入口。Plus 标签实际提交与固定阅读提交分别登记，重新查询远端后仍与 PREP-003 一致。
- 来源字段一致性、README 入口、169 个应用改动文件与 4061 个 Plus 来源文件的版权/SPDX 核对、14 个本地链接、4 个外部 HTTP 200 链接及差异检查均通过；检查范围没有需恢复的来源声明，没有修改业务源码或编造文件头。
- 精确提交的 [CI 38111967529](https://github.com/ccisnoxx/sub2api/actions/runs/38111967529) 与 [Security Scan 38111967508](https://github.com/ccisnoxx/sub2api/actions/runs/38111967508) 全部成功。OpenAI recording race 按现有 main 范围规则明确不适用；同次 push 的重复检查已取消，未将取消或未执行列为通过。
- 验收、风险、数据库影响及回滚记录见 PR 和 `docs/legal/LEGAL-001.md`；完整许可证副本、依赖许可与发行材料仍属于未完成的 `LEGAL-002`。

---

## LEGAL-002：审核许可证副本、文件头和发布产物中的许可证材料

**优先级：** P1  
**依赖：** `LEGAL-001`  
**建议分支：** `chore/license-audit`

### 目标

确保源码仓库、二进制 Release 和容器镜像均能追溯许可证和对应源码。

### 实施步骤

- [x] 确认根目录 `LICENSE` 与实际许可策略一致；
- [x] 检查从 Plus 直接复制或改写的文件头；
- [x] 检查第三方前端依赖许可证生成方式；
- [x] 在 Release 产物中包含 `LICENSE` 和 NOTICE；
- [x] 在容器镜像中增加许可证和版本元数据；
- [x] 在 Release notes 中写明对应源码 SHA；
- [x] 保证使用者可以找到与发布二进制、镜像对应的源代码。

### 验收标准

- 每个正式发布产物都能定位到精确源码提交；
- Release archive 包含许可证和第三方声明；
- 容器 OCI labels 包含 revision、source 和 version；
- 不删除上游已有版权或许可证信息。

### 历史候选记录（当时未完成，2026-10-10；后由 personal 应用候选验收替代）

- 开始前为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，工作树仅 `AM REMEDIATION_PLAN.md`；原暂存内容保留。前置 LEGAL-001 提交 `dbb4cb099f96218e7cb53040fadcd17467936541` 和验收记录已完成，[PR #16](https://github.com/ccisnoxx/sub2api/pull/16) 尚未合并，因此使用其提交作为独立候选基线。
- 候选分支 `codex/legal-002-license-audit`，提交 `2b17e595471b0d01da7a9fb2764e41d11ed7adaf`；[草稿 PR #17](https://github.com/ccisnoxx/sub2api/pull/17) 基于 LEGAL-001 分支，仅包含本任务19个文件。根 LGPL LICENSE 字节未变，补齐官方 GPLv3 与 Vue 精确版本许可副本；实现实际分发依赖材料、归档许可/源码绑定、镜像材料运输和完整 SHA 的 Release notes。
- 真实前端构建因 `@airwallex/components-sdk@1.30.2`（MIT）和 `@airwallex/airtracker@3.2.0`（ISC）缺完整许可及版权文本受阻；没有猜测作者、套通用模板或删除支付功能。完整来源证据与解除条件在 `frontend/licenses/README.md`。
- 发布工具29项通过（原28项成功；ZIP特殊文件收紧后重跑14项matrix，复用其余15项未变证据），前端8项Vite许可测试、vue-tsc、定向ESLint、冻结依赖安装、runtime resources、bash语法及diff检查通过。官方校验的GoReleaser v2.18.3 simple配置通过；full配置严格check因既有archive弃用字段返回非零，未记为通过。
- [Release dry-run 38113665007](https://github.com/ccisnoxx/sub2api/actions/runs/38113665007) 对同一精确提交失败：源码SHA绑定成功，云端build-frontend日志确认上述两项材料缺失并exit 1，二进制/容器/发布步骤跳过。真实archive和OCI验收未通过，不重复同因重试。
- [Security Scan 38113621206](https://github.com/ccisnoxx/sub2api/actions/runs/38113621206) 对同一精确候选提交成功；重复PR安全检查38113651476和push CI38113621144已取消，取消不计通过。保留的 [CI 38113651458](https://github.com/ccisnoxx/sub2api/actions/runs/38113651458) 已于 `2026-10-11 05:17:40 UTC` 完成，最终结果 `success`，绑定候选 SHA `2b17e595471b0d01da7a9fb2764e41d11ed7adaf`；release-helpers、shell、golangci-lint、frontend、test 五个 job 全部成功，后端单元与集成测试通过。OpenAI recording race 检查明确不适用：当前源码没有该模块，未执行 race 测试，未将其列为通过。普通 CI 成功不替代失败的 Release dry-run，LEGAL-002 仍未满足全部验收标准。
- 独立只读复核覆盖最终提交，未确认新增代码缺陷，核实官方许可字节保持原文；没有独立复跑前端/发布测试。执行审计 `20261011T045122Z-legal-002-60db885b` 已关闭并校验通过，前端子任务因真实build未通过而未获验收。
- 此候选不可合并为可发布状态；取得对应版本官方全文后，仍须将LEGAL-001/002必需材料与构建配置纳入personal应用候选，再验证实际产物。未修改数据库/恢复工具、未重复恢复演练，也未合并、正式发布或部署。风险与仅撤销本任务提交的回滚方法见PR及 `docs/legal/LEGAL-002.md`；任务和实施步骤保持未勾选。

### Airwallex 官方来源复核（历史材料缺口，2026-10-11 UTC）

- 重新获取两个精确版本的官方 npm 元数据和 tarball，校验 tarball SHA-512 与 `frontend/pnpm-lock.yaml` 完全一致，并逐一读取各包实际的6个普通文件；两个包均无 LICENSE/COPYING/NOTICE。`@airwallex/components-sdk@1.30.2` 的 [官方 tarball](https://registry.npmjs.org/@airwallex/components-sdk/-/components-sdk-1.30.2.tgz) SHA-256 为 `9c5d6d029422fd71ae7d959664059d936dc9c2ac8aab9676e856eea35dde4de6`；`@airwallex/airtracker@3.2.0` 的 [官方 tarball](https://registry.npmjs.org/@airwallex/airtracker/-/airtracker-3.2.0.tgz) SHA-256 为 `9888ec6fada16995522513f10b8295c1aa7c85f4679d8056b82d960dab2cd912`。许可字段仍分别为 MIT/ISC，作者字段不能替代版权通知；components-sdk 的 CJS 中 Microsoft helper 通知不能作为 Airwallex 包的完整 MIT 文本。
- components-sdk 随包 CHANGELOG 指向 `gitlab.awx.im/airwallex-dev-scale/components-sdk`，但只记录至旧版本 `1.27.2`；不能据此验证 `1.30.2` tag 或许可。[尝试读取该版本 LICENSE](https://gitlab.awx.im/airwallex-dev-scale/components-sdk/-/raw/v1.30.2/LICENSE)、tag tree 和 repository API 均在 TLS 连接阶段报 `UNEXPECTED_EOF_WHILE_READING`，未收到可验证原文，不能推断仓库公开性或文件不存在。Airwallex 官方 GitHub 组织检索未找到上述两个精确版本的源包；airtracker 的官方 npm gitHead `d7cc7ca00bd15a6c67b2de25e73814c475b18778` 在公开 commit 搜索中返回0项，不能据此绑定其他项目的许可。
- [官方 CDN 的 v1.30.2 脚本](https://static.airwallex.com/components/sdk/v1.30.2/index.js) HTTP 200、SHA-256 `10ba0c3ea915932eef145c55c18bd102f3944692729b0920c3ba4fc3d6adf612`，但未含完整许可/版权通知，且无法仅凭 URL 将其源码绑定到 npm 包；同目录探查的 `index.js.LICENSE.txt`、`LICENSE`、`LICENSE.txt`、`NOTICE`、`index.js.map` 均为404。官方集成文档和其他 Airwallex 项目的许可不能替代这两个精确包的完整原文。
- 本轮未取得可核实的两项官方完整许可和版权材料，因此未补充许可文件、未修改 Vite 配置或依赖，候选 HEAD 仍为 `2b17e595471b0d01da7a9fb2764e41d11ed7adaf`。补齐后再重跑最小 Release dry-run 的前置条件尚未满足，本轮未 dispatch 或重试同因失败的38113665007。解除阻塞需要 Airwallex 官方提供完整原文及明确适用于这两个版本的来源/确认；取得后保留原文字节、登记精确版本和固定来源，运行定向许可测试及真实构建，再按新 SHA 执行 Linux amd64 dry-run。LEGAL-002 继续未完成，所有任务勾选不变。

### 授权停用 Airwallex 续作（当时未完成，2026-10-11 UTC；后由 personal 应用候选验收替代）

- 用户明确授权禁用 Airwallex 并移除前端 SDK 与支付入口，保留现有订单及数据库记录。续作前候选分支 `codex/legal-002-license-audit`、HEAD `2b17e595471b0d01da7a9fb2764e41d11ed7adaf`、工作树干净；LEGAL-001 基线及未合并状态不变，原工作区 `AM REMEDIATION_PLAN.md` 的暂存内容保留。
- 已推送候选 `b165664967939b664edcb83d76adc37eb416fc3d` 到同一 [草稿 PR #17](https://github.com/ccisnoxx/sub2api/pull/17)。新增提交只处理授权停用和验收留存的35个文件；整体PR相对LEGAL-001为50个文件。服务层在配置读取及订单事务前拒绝请求，并在实例选择后拒绝真实Airwallex provider；目录及启用类型排除。历史查询、取消、退款、签名与履约、实例配置均保留，无迁移或数据删除。
- 前端删除两个SDK、专属页面/路由、新支付及管理新增入口。独立复核发现历史自定义方法名可能误入Stripe后，补齐认证订单的持久化provider标识（snapshot优先，不猜方法归属、不暴露商户信息）；恢复旧密钥先核实服务商，旧付款窗口和二维码不能启动，合法Stripe支付宝订单仍可继续。历史快照原文字节和匿名查询响应保留。
- 本地后端定向服务36项/provider9项通过，新增拒绝/目录回归先失败后通过；前端实现定向11文件118个不同用例通过。别名修复后直接重跑4文件67项、handler身份/匿名查询和service历史snapshot测试通过，别名回归先失败后通过。冻结pnpm9校验、定向ESLint和最终生产脚本（i18n3项、vue-tsc-b、Vite5.4.21）通过；lock只删除两包15行，其余逐字不变。实际28包许可85857字节、SHA256 `6c2468b5b42456ee43bfd60daa0de8631b073186692779cc57625861d6905800`；133分包/133sourcemap/618来源无两SDK及Airwallex页面，Stripe保留。
- 精确SHA的 [CI 38117316942](https://github.com/ccisnoxx/sub2api/actions/runs/38117316942) 最终success，release-helpers、frontend、shell、golangci-lint、test五个job全部成功，后端单元及集成测试通过（test job于 `2026-10-11 06:24:46 UTC` 完成）。云端发布工具29项及单元runner6项通过；OpenAI recording日志明确当前源码无模块、该检查不适用，未执行race，不计为race通过。[Security Scan 38117314967](https://github.com/ccisnoxx/sub2api/actions/runs/38117314967) 和PR事件38117316939均success，后者在取消前已完成；重复push CI38117314970已取消，不计通过。最终fresh只读复核核对候选并确认旧别名反例解除，无新增代码阻断；读取红/绿原始日志及认证/匿名边界，未独立重跑动态测试。执行审计 `20261011T054745Z-legal-002-disable-airwallex-adf8712d` 已关闭并校验通过。
- 同一SHA的 [最小Linux amd64 Release dry-run 38117402527](https://github.com/ccisnoxx/sub2api/actions/runs/38117402527) 于 `2026-10-11 06:20:43 UTC` 最终success：prepare、build-frontend、build-binaries、release四个job成功，sync-version-file因dry-run跳过。下载实际frontend、binary、OCI及report完成核验，版本 `0.2.14-klno.3`、report commit与候选一致；没有registry推送、正式GitHub Release、版本同步或部署。
- 实际归档 `sub2api_0.2.14-klno.3_linux_amd64.tar.gz` SHA256 `7bf0878af2937c14587f6637f019558957bd71f19f6dbd57fb4939c603b044cb`，含完整 `LICENSE`、`LICENSE-GPL-3.0.txt`、`NOTICE`、`THIRD_PARTY_LICENSES.txt`、`SOURCE.json`；原许可/归属字节与所选源码一致，第三方全文与云端前端和本地最终构建逐字一致，二进制含完整候选SHA。SOURCE.json SHA256 `651eecff14164d836cbfb8dc32de8becb1cae199ce4dd8152c4e028f81a6228b` 绑定全部材料摘要、版本、源码树及下载链接；[对应源码归档](https://github.com/ccisnoxx/sub2api/archive/b165664967939b664edcb83d76adc37eb416fc3d.tar.gz) 匿名HEAD重定向后HTTP200。
- 实际OCI归档 `sub2api-amd64.oci.tar` SHA256 `d9e82aeb7aa7e6deb069a3a3ae31ef6a63200fe9da9e6c78e4453ae37e93beb7`；校验所有读取的descriptor摘要/长度、单一linux/amd64运行manifest及最终层。manifest digest `sha256:202853016085c52711b62d34e24841354acbaa1057ab7120df7047ad2cbd80ff`；`org.opencontainers.image.revision=b165664967939b664edcb83d76adc37eb416fc3d`、`source=https://github.com/ccisnoxx/sub2api`、`version=0.2.14-klno.3`。`/usr/share/licenses/sub2api/` 五份材料与归档逐字一致，实际第三方文本无两SDK。dry-run OCI artifact仅短期留存核验，不是正式发布。
- 两包官方完整许可仍未取得，本次仅因停止分发解除相关阻塞，不宣称补齐。未合并、正式发布、部署或变更恢复工具；必需材料和配置尚未纳入并验证personal应用候选，因此LEGAL-002及全部实施步骤保持未勾选。回滚本次提交会恢复SDK及严格许可构建阻塞，重新启用前须取得精确版本官方材料并重新验收。

### personal 应用候选完成证据（2026-10-11 UTC）

- 续作前原工作区分支 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；原暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。复用干净候选分支 `codex/legal-002-license-audit`、HEAD `b165664967939b664edcb83d76adc37eb416fc3d`。LEGAL-001 已完成但PR #16仍未合并，本轮将其必需材料直接纳入应用候选，不依赖先合并PR。
- 以固定远端 personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 的应用树重放53个任务路径，补充开发日志及新单负载均衡修复，共55个路径。最终候选 `0ea69e62ee0079ee2616273f627ed97cc3f57b5c` 已普通推送到同一 [草稿 PR #17](https://github.com/ccisnoxx/sub2api/pull/17)，base已改为personal；旧候选和personal基础均保留为祖先，没有强推。任务外全树内容/模式一致，原 LICENSE、全部迁移、`deploy/personal-source.json` 和 VERSION 原文保留；KlN来源仍为 `v0.2.14-klno.5` / `c7aacf5d3ae383d0d5c75f471f66e61690a5701d`，既有 VERSION `0.2.13` 仅用于本次dry-run，没有分配正式个人版本。
- 保留授权的 Airwallex 禁用、移除SDK及入口。首轮独立复核确认保留Airwallex实例与有效EasyPay共用 `bank` 时，目录可用但负载均衡会误选Air并403；在新单候选查询中排除Airwallex，保留实际provider后置拒绝和历史实例启用/配置读取。生产构造器、真实SQLite/负载均衡/EasyPay popup的两策略CreateOrder回归先因403失败，修复后有效订单归属和付款地址通过；历史订单查询、取消、退款、签名与履约路径无新禁用条件。
- 本轮本地 Go1.27.2 后端目标43个顶层测试（service30/handler4/provider9，含子测试87条pass）通过；修复后直接相关12个顶层测试（service8/payment4，含子测试57条pass）通过，不将重复测试累加为不同用例。pnpm9.15.9冻结安装、6文件92项Vitest、定向ESLint、生产脚本（i18n3项/vue-tsc-b/Vite5.4.21）、personal发布工具24项、shell语法及runtime resources通过。前端和发布代码未随后端选择修复改变，复用已成功的本轮证据。36个变更源码文件版权/SPDX检查无删除；55路径范围、来源/迁移/原许可和文档本地/固定HTTP200链接检查通过。
- [Personal CI 38119580857](https://github.com/ccisnoxx/sub2api/actions/runs/38119580857) 最终success，绑定上述最终SHA及PR #17/personal基础，全部11个job成功：初始binding、前端、TPS、同步合同、两项安全检查、shell、release-helpers、golangci-lint、后端test及personal-ready。云端 make test-unit、make test-integration均成功；OpenAI recording race实际执行 `go test -race -tags=unit -timeout=60s -run '^Test(Recorder|OpenAIRecording)' ./internal/pkg/upstreamrecord ./internal/service`，两包均ok，不是未运行或不适用。personal-ready于 `2026-10-11T07:10:33Z` 完成，结束时再次核对候选/PR/最新personal绑定成功。重复push CI38119577691已取消，不计通过；push安全检查38119577717在取消前已最终success。CLA Assistant为skipped，不计为验收测试通过。
- 同一SHA的 [最小Linux amd64 Release dry-run 38119582213](https://github.com/ccisnoxx/sub2api/actions/runs/38119582213) 最终success，prepare、build-frontend、build-binaries和release全部成功，sync-version-file因dry-run跳过。report commit为 `0ea69e62ee0079ee2616273f627ed97cc3f57b5c`、version为 `0.2.13`。下载真实前端、二进制、OCI及report后完成核验；云端第三方全文28包85,857字节，与本地生产构建逐字一致，134个JS分包无Airwallex页面且Stripe保留；本地134份sourcemap的634个来源无两个SDK。
- 实际 `sub2api_0.2.13_linux_amd64.tar.gz` SHA256 `1562e794bf66a9d0a1840f8561161bd9fad9000ee7799aa375f19682148137df`，包含普通文件 `LICENSE`、`LICENSE-GPL-3.0.txt`、`NOTICE`、`THIRD_PARTY_LICENSES.txt`、`SOURCE.json`；原许可/归属与源码逐字一致，第三方全文与云端前端逐字一致，二进制含完整最终SHA。SOURCE.json SHA256 `99702fb33889c1816b31e06a83a6115809f256d5ec7490a10da846e6d904af01` 绑定全部材料摘要、版本及 [对应源码归档](https://github.com/ccisnoxx/sub2api/archive/0ea69e62ee0079ee2616273f627ed97cc3f57b5c.tar.gz)，匿名HEAD重定向后HTTP200。原LICENSE SHA256 `a5681bf9b05db14d86776930017c647ad9e6e56ff6bbcfdf21e5848288dfaf1b`，第三方全文SHA256 `6c2468b5b42456ee43bfd60daa0de8631b073186692779cc57625861d6905800`。
- 实际 `sub2api-amd64.oci.tar` SHA256 `15e2159431efb0dce48d02f81ceae17b8d144b0c4262e99f93de2a58a680129a`；全部读取的descriptor摘要/长度及最终层校验通过，单一linux/amd64运行manifest digest `sha256:23293a26f4482bc2b7ed24a4d471216a4d2b7d23a9cfc2fe5454917b7a455fa2`。OCI `revision=0ea69e62ee0079ee2616273f627ed97cc3f57b5c`、`source=https://github.com/ccisnoxx/sub2api`、`version=0.2.13`；`/usr/share/licenses/sub2api/` 五份材料与归档逐字一致。Release日志确认snapshot和skip publish/announce，registry登录、正式发布前授权/占用检查及版本同步因dry-run跳过；没有创建正式标签、发布Release、推镜像或部署。
- 两次fresh critical_reviewer独立只读复核：首轮上述P2已修复，最终复核绑定 `0ea69e62ee0079ee2616273f627ed97cc3f57b5c`，核对全部生产SelectInstance入口、规范provider键约束、候选限额回退、历史配置读取及查询/取消/退款/签名/履约，不再有确认代码阻断。复核读取红/绿原始日志但未独立复跑动态检查。执行审计 `20261011T063727Z-legal-002-personal-integration-a4b4f98c` 已关闭并audit-verify通过；实际云端门禁/产物由主代理核验。
- 两包官方完整许可仍未取得，仅因停止分发解除相关阻塞，不宣称补齐。没有数据库/schema/迁移/恢复工具改动，复用PREP-001既有非生产恢复证据；没有合并PR或修改personal/main。未对真实支付网关执行付款/退款或浏览器E2E，full GoReleaser严格check的既有archive弃用字段仍未迁移，本轮只验证要求的simple/linuxamd64。回滚可在未合并时关闭PR；后续只撤销本任务应用差异并保留LEGAL-001归属，重新启用Airwallex必须先取得精确版本官方材料并重验。同一最终SHA的Personal CI、最小Release dry-run及实际产物核验全部通过，LEGAL-002与七项实施步骤现已勾选；不代表PR已合并或正式发布。

---

## SEC-001：重写本 fork 的安全政策与漏洞报告渠道

**优先级：** P0  
**依赖：** `DOC-001`  
**建议分支：** `docs/security-policy`

### 目标

让本 fork 的漏洞报告进入本仓库维护者可以实际处理的私密渠道。

### 实施步骤

- [x] 修改 `.github/SECURITY.md`；
- [x] 优先启用本仓库 GitHub Private Vulnerability Reporting；
- [x] 删除或替换无法控制的原上游安全邮箱；
- [x] 区分本 fork 特有问题与上游继承问题；
- [x] 明确支持版本和修复策略；
- [x] 明确服务状态、路由诊断、发布工具和部署脚本属于本 fork 安全范围；
- [x] 增加安全响应角色，但不要使用占位邮箱；
- [x] 检查 README 和 issue 模板中的安全报告链接。

### 验收标准

- 用户可以通过私密方式向本仓库报告漏洞；
- 链接实际指向 `ccisnoxx/sub2api`；
- 不再要求用户把本 fork 特有漏洞发送给原上游；
- 安全政策中不存在 `security@your-domain` 等占位值。

---

## SEC-002：处理即将到期或不完整的依赖审计例外

**优先级：** P0  
**依赖：** `PREP-001`  
**建议分支：** `security/audit-exceptions`

### 目标

消除过期、占位和缺少验证证据的安全例外。

### 实施步骤

- [x] 重新运行生产依赖审计；
- [x] 确认 `nanoid` 漏洞的实际依赖路径和调用方式；
- [x] 优先升级、替换或移除受影响依赖；
- [x] 若短期无法修复，记录真实 owner、具体路径、不可利用证据和短期到期日；
- [x] 检查 `xlsx` 两个高危例外是否仍只用于可信写出路径；
- [x] 增加静态检查，阻止引入 `XLSX.read` 或解析用户上传表格；
- [x] 删除所有占位 owner；
- [x] 让过期例外继续导致 CI 失败。

### 验收标准

- 没有已过期例外；
- 每个高危例外都有真实 owner、原因、缓解措施和到期日；
- `pnpm audit --prod` 中所有 high/critical 项均已修复或精确匹配有效例外；
- 例外不能仅按包名宽泛忽略全部未来漏洞。

### 建议测试

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm audit --prod --audit-level=high --json > audit.json || true
cd ..
python tools/check_pnpm_audit_exceptions.py \
  --audit frontend/audit.json \
  --exceptions .github/audit-exceptions.yml
```

### 完成证据（2026-10-10）

- 前置 `PREP-001` 已随 [PR #12](https://github.com/ccisnoxx/sub2api/pull/12) 合并；恢复基线记录提交为 `b29ab3496c8c5f63bbcf4f56649bef8377899a93`。
- 实现位于 `codex/sec-002-audit-exceptions`，最终提交为 `e84f17fee7a100f27d958fd05ea3db4ffec3f793`；[草稿 PR #15](https://github.com/ccisnoxx/sub2api/pull/15) 尚未合并或部署。
- 生产审计结果为 high 2、critical 0；仅保留 `xlsx@0.18.5` 的两个精确 GHSA 例外，owner 为 `@ccisnoxx`，到期日为 `2026-11-09`。`nanoid` 当前安装版本已修复，移除旧例外；未变更依赖版本或 lockfile。
- 冻结依赖安装、有效审计报告例外校验、14 项审计门禁回归、33 项 XLSX 静态规则回归、全部前端源码扫描及 14 项 UsageView 测试均通过。
- 同一最终提交的 [Security Scan 38109232020](https://github.com/ccisnoxx/sub2api/actions/runs/38109232020) 与 [CI 38109232016](https://github.com/ccisnoxx/sub2api/actions/runs/38109232016) 全部通过；独立只读复核确认发现已修正，最终候选无确认阻断。
- 详细验收、风险及回滚记录见本 PR 的 `docs/security/SEC-002.md`；本任务未修改业务代码或数据库。

### main 定向续作验收（2026-10-12）

- 本轮从 main `b4ecd4794e464b334a82cce63dc114ac93dcda75` 与 personal `0dc428e8ad9156302660efcac994d0c986773504` 隔离实施；上轮发现的 main 清理缺口由本记录关闭，原 SEC-002 历史候选/PR #15 状态与审计记录保留。代码候选 main `9fdc2a0c943885466d19e30dcd28af278ba1d395`、personal `b6ac9cee01840c7b06c62d87f5fb6aa652d0b3d1`；[main 续作报告](https://github.com/ccisnoxx/sub2api/blob/9fdc2a0c943885466d19e30dcd28af278ba1d395/docs/security/SEC-002-MAIN.md)保存实际依赖、报告摘要、命令和回退。
- 逐项核对实际生产路径与锁文件差异：main 仍有两项 Airwallex 依赖，其余相关依赖版本与 personal 一致；不合并应用树、不更新依赖或 lockfile。两个有效官方 registry 生产审计均为 high 2 / critical 0，仅为 `xlsx@0.18.5` 的 `GHSA-4r6h-8v6p-xvw6`、`GHSA-5pgg-2g8v-p4x9`，路径 `. > xlsx@0.18.5`。
- main 删除无对应发现的 lodash、lodash-es、axios、nanoid 四例外；两项 XLSX 例外采用真实 owner `@ccisnoxx`、精确版本/完整路径与 personal 同一 `2026-11-09` 期限。main evidence 指向本轮报告，personal 例外配置不变；没有延长期限或按包名忽略新漏洞。
- main 复用 personal 既有严格校验器、XLSX 静态规则及两组既有测试，四份文件逐字相同；原校验器的 14 项回归含子测试出现 19 个失败断言，修复后全部 14 项通过。33 项 XLSX 规则回归、两个分支当前源码扫描、两份实际报告匹配、Security Scan actionlint v1.7.12（未运行 ShellCheck/pyflakes）与 CRLF 感知 diff 检查通过。两分支原 Check audit exceptions 步骤补入既有政策/规则检查命令，触发方式、权限、扫描等级和发布绑定保持。
- personal 首次在未安装环境取得空 finding 路径报告，不计有效验收；冻结安装后新报告与原严格校验器匹配成功，未放宽路径校验。报告的 high 非零导致原 audit 命令退出 1，不冒称没有漏洞；例外校验通过只证明当前限定写出路径和期限。
- 本轮只做上述定向验证，没有全量后端/前端回归、云端 Quick/Full、安全工作流 dispatch、Release dry-run、镜像构建、数据库演练、发布或部署。数据库影响 none；回退使用普通 revert 并保持两分支门禁消费者一致，不能以恢复占位/过期例外冒充安全验收。XLSX 替换仍须在到期前另行处理；第 8 节仅本项由待办更新为完成，OBS 整合、回退兼容和发布项继续待办。

- fresh `critical_reviewer` 对上述固定 main/personal 代码候选独立只读复核，未确认任务内阻断；核对实际例外、报告摘要、消费者/固定 SHA 调用、依赖/应用树和原始定向证据，并用内存反例确认空路径、未知报告、缺失 critical 明细、新 GHSA 与 XLSX 引用逃逸均拒绝。未独立重跑完整 suite，也未审查本轮随后追加的计划文本。审计 `20261012T153408Z-sec-002-main-audit-cleanup-6575bf35` 已闭合、audit-verify 通过，SUBAGENT_EXECUTION_DIGEST 有效；父代理未观察到复核写入，保留覆盖限制。

---

## UPSTREAM-001：重新确认目标 KIN 并创建干净集成分支

**优先级：** P0  
**依赖：** `PREP-001`、`PREP-003`  
**建议分支：** `integration/kin-<tag>-personal`

### 目标

冻结新的 KIN 稳定基线，创建干净集成分支并建立个人功能重放阶段、独立提交和验证规则。

### 维护者确认的任务边界（2026-10-11）

原实施步骤和验收要求 S1–S4 已经重放，而 MIG-001 依赖本任务、S1–S4 又依赖 MIG-001 或前序阶段，形成依赖循环。维护者明确授权本任务调整为准备任务；实际重放提交、编译及测试通过保留在 UPSTREAM-002～005 验收。本轮不执行这些后续任务，也不降低其业务或兼容验收标准。

冻结目标为官方 `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71`，tag object 为 `0c1ede31ec63f9cfc2b72f07db66fb41f540208d`，稳定 Release 发布时间为 `2026-10-10T07:54:36Z`。

### 实施步骤

- [x] 查询 KIN 最新稳定 tag、完整 SHA 和发布时间；
- [x] 验证 tag 与 commit 的对应关系；
- [x] 记录旧基线、新目标和 merge-base；
- [x] 从目标 KIN tag 创建新的集成分支；
- [x] 禁止在此阶段更新 `personal-source.json`；
- [x] 建立四个功能重放阶段：S1、S2、S3、S4；
- [x] 为每个阶段定义独立提交、测试报告及固定候选验证规则；实际提交和报告由 UPSTREAM-002～005 交付；
- [x] 如果目标 KIN 在执行期间又发生历史改写，停止并重新评估。

### 验收标准

- 集成分支的初始提交精确等于目标 KIN tag；
- 没有 merge commit 混入旧 `personal` 的完整历史；
- S1–S4 的边界、依赖、可审查重放方式、独立提交和测试报告归属已建立，`personal-source.json` 保持不变；
- 每个重放阶段的独立编译、定向测试入口及门禁已明确；实际重放提交和测试通过由 UPSTREAM-002～005 验收。

### 禁止事项

- 不得使用 `--force` 更新 `personal`；
- 不得把旧基线错误标记成新 KIN 的祖先；
- 不得通过修改同步脚本跳过分叉检测。

### 准备完成证据（2026-10-11）

- PREP-001 的 [PR #12](https://github.com/ccisnoxx/sub2api/pull/12) 与 PREP-003 的 [PR #11](https://github.com/ccisnoxx/sub2api/pull/11) 均已合并，merge SHA 分别为 `2cca80b396d7f2e21d2e3cd50c0b0e2ae1767738`、`24daf87641750fb78a3ad5818e0eb3eb392d638e`。
- 原工作区仍为 `codex/doc-002-install-sources` / `e0f1643daef7847d8ff2c144de82b0ad758964ca`，只有 `AM REMEDIATION_PLAN.md`，原暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。文档分支 `codex/upstream-001-preparation-record` 从 `origin/main` 的 `c1dae6357b4de7cc11d81547013b87550c4ba586` 创建；[草稿 PR #18](https://github.com/ccisnoxx/sub2api/pull/18) 以 main 为目标，最终候选 `4da7ea33076bb4787c4af1edf73493d08a8152f8` 仅改变 `UPSTREAM.md` 与 `docs/upstream/UPSTREAM-001.md`。
- 独立集成分支 `codex/upstream-001-kin-v0.2.15-klno.2` 已本地创建并附着工作树；初始及本轮 HEAD 精确为冻结的 `ca686de5e5ac14834e12701c156831abd5ec6b71`，增量提交、新增 merge 与 tree diff 均为空，工作树干净。官方 latest stable Release、远端 tag/peeled ref、GitHub tag object API 和本地 `refs/kin-migration/upstream-001/tag` 一致。新目标与旧 KIN `c7aacf5d3ae383d0d5c75f471f66e61690a5701d` / 当前 personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 的 merge-base 均为 `0363b8cdba8cec3e2ba4b2dbd49c4481143fa55d`，两次非祖先检查退出码1为预期分叉证据。
- S1–S4 边界、完整前置依赖、可审查重放方式、独立提交/报告与固定候选编译、定向测试、Personal CI 规则已建立；阶段表与原任务直接前置逐项比对通过。3个本地链接、6个外部HTTP200链接、文档字段/占位及差异检查通过；计划修改只限 §2 与本任务，原暂存内容保留。
- 两次 fresh 只读 reviewer 复核：首轮确认 S4 漏列 UPSTREAM-002，最终候选补回且要求绑定计时/诊断前置 SHA；最终复核无新增阻断。覆盖任务规则、Git reflog/ref/分叉、来源、链接与编译入口，未独立联网核实官方API/CI，也未运行应用或迁移动态测试。执行审计 `20261011T072511Z-upstream-001-preparation-5761b0de` 已关闭并校验通过。
- 纯 KIN 树只复用同 SHA 的 [官方 CI 38034603661](https://github.com/KlN-4096/sub2api/actions/runs/38034603661) 五个 job success；不证明个人功能、迁移或本 fork 发布验收。首轮文档候选 f1adfa6 的 [CI 38121426474](https://github.com/ccisnoxx/sub2api/actions/runs/38121426474) 因被替代而取消，不计通过；[Security Scan 38121426428](https://github.com/ccisnoxx/sub2api/actions/runs/38121426428) 在取消请求前已 success，不能冒用为最终 SHA 门禁。最终文档候选 `4da7ea33076bb4787c4af1edf73493d08a8152f8` 的 [CI 38121768207](https://github.com/ccisnoxx/sub2api/actions/runs/38121768207) 最终 success，shell、golangci-lint、release-helpers、frontend、test 五个 job 全部 success；后端单元 runner6项、单元及集成测试通过，test job 于 `2026-10-11T07:45:34Z` 完成。日志明确 main 源码没有 OpenAI recording 模块，该定向检查不适用，未执行 race，不列为 race 通过。同一最终 SHA 的 [Security Scan 38121768209](https://github.com/ccisnoxx/sub2api/actions/runs/38121768209) success，两安全 job 全部通过；CLA 按既有策略 skipped，不计通过。交付前再次核实官方 tag object/commit 未变化，PR base/head 精确一致，合成 merge 的 tree 与文档候选相同。全部经维护者明确调整的准备验收、独立复核及适用门禁已通过，任务与本阶段实施步骤勾选；后续迁移和重放任务保持未勾选。
- 未改 `personal-source.json`、main/personal 或既有 PR #17 的许可/Airwallex 禁用候选，未开始 MIG-001 或 S1–S4。无 SQL、生产读取、Release dry-run、恢复演练、合并、正式发布或部署；实际迁移和应用切换的验证由后续任务完成。风险及仅撤销本任务文档提交、无后续改动时归档集成工作树的回滚边界见 PR #18 和准备记录。

---

## MIG-001：审核迁移编号、迁移账本和生产兼容策略

**优先级：** P0  
**依赖：** `UPSTREAM-001`  
**建议分支：** 与集成分支一致

### 目标

保证迁移 `251`—`254` 在新 KIN 基线中不冲突，并兼容已经执行过旧迁移的环境。

### 实施步骤

- [x] 列出目标 KIN 的全部迁移身份和编号；
- [x] 检查 `251`—`254` 是否存在编号、文件名或语义冲突；
- [x] 查询生产和测试数据库是否已经执行个人迁移；
- [x] 按以下决策矩阵处理：

| 条件 | 处理方式 |
|---|---|
| 从未在任何受支持环境执行 | 可以在合并前重新编号 |
| 已在测试或生产执行 | 不得直接改名冒充新迁移；保留原身份或增加兼容桥接迁移 |
| 上游出现同编号不同内容 | 必须新增唯一身份并显式检测旧迁移状态 |
| 同语义已被上游实现 | 编写幂等兼容迁移和数据验证，不重复破坏 schema |

- [x] 测试空库升级；
- [x] 测试旧 KIN 数据库升级；
- [x] 测试已经执行个人迁移的数据库升级；
- [x] 测试应用回退到旧版本时的兼容性；
- [x] 建立迁移失败回滚和恢复流程。

### 验收标准

- 所有受支持数据库路径均可升级；
- 不出现同一迁移身份对应不同 SQL 内容；
- 旧应用在保留新增列和表的情况下仍能启动，或有明确阻断说明；
- 数据库备份恢复演练成功；
- 迁移执行时间和锁影响已评估。

### 本轮完成证据（2026-10-11）

- 前置 UPSTREAM-001 已验收，准备候选 `4da7ea33076bb4787c4af1edf73493d08a8152f8` / [PR #18](https://github.com/ccisnoxx/sub2api/pull/18) 尚未合并。执行前与收尾重新查询，最新稳定仍为 `v0.2.15-klno.2`，tag object `0c1ede31ec63f9cfc2b72f07db66fb41f540208d`，peeled commit `ca686de5e5ac14834e12701c156831abd5ec6b71`，与冻结目标一致。
- 原工作区仍为 `codex/doc-002-install-sources` / `e0f1643daef7847d8ff2c144de82b0ad758964ca`，开始时仅有 `AM REMEDIATION_PLAN.md`；原暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。本轮在隔离工作树从目标 KIN 创建 `codex/mig-001-migration-compatibility`；[草稿 PR #19](https://github.com/ccisnoxx/sub2api/pull/19) 以 `codex/upstream-001-kin-v0.2.15-klno.2` 为目标，候选 `c0e7ab68b8479fc9d6f3b888d548eca4f4192974`，候选树干净且与 PR 合并预览树一致。
- [完整身份清单](https://github.com/ccisnoxx/sub2api/blob/c0e7ab68b8479fc9d6f3b888d548eca4f4192974/docs/migrations/MIG-001-inventory.json)覆盖目标 304 份 SQL；旧 KIN 的全部 303 份同名 SQL 与目标和当前 personal 原字节一致。目标未占用 251–254，也无对应扩展 schema 的同名实现；同身份不同内容为零。将当前已部署 personal 的四份 SQL 原名、原字节保留，候选总计 308 个迁移；没有修改 runner 或 checksum 兼容例外。
- 持有现有部署锁的共享锁，只读查询生产及 S44 测试账本、应用镜像和容量元数据：两库均为固定 personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a`，307 条账本，251–254 已执行且 checksum 与原 SQL 一致，账本指纹与 PREP-001 相同；两库均尚无目标新 242。原 applied_at 保留策略已验证，无需重编号或桥接。
- 新增冻结身份/checksum 回归测试和真实 runner 集成测试。本地身份测试通过；完整定向集成通过空库、303 份旧 KIN SQL、307 份已执行个人 SQL 三条升级路径，以及锁失败后同身份恢复、未知 checksum 显式拒绝。增强一万条旧用量实写和 filenode 断言后，仅重跑三条受影响路径并全部通过，旧 Token/费用、unknown/NULL、旧账本及时间、服务状态配置和事实保留。
- 三条路径均在完整扩展 schema 上运行固定旧生产镜像并通过实际 `/health`，账本保留；支持保留新增列、表、函数、触发器和账本回退应用。新应用在旧 schema 上必须先执行迁移；旧应用启动验证不代表支持新平台、完整业务流量或未来 S1–S4 状态语义，这些边界由后续任务重新验收。
- 对升级后的合成库实际执行 `pg_dump -Fc` 和隔离新库 `pg_restore --exit-on-error`，核对全部 308 条 filename/checksum/applied_at、一万条用量、状态配置及事实，恢复库再次启动 runner 成功。PREP-001 固定生产备份的既有隔离全量恢复证据保留；本轮没有覆盖线上数据库，也未将合成演练冒充新的生产恢复。
- 合成库完整升级耗时为空库 811.35 ms、旧 KIN 128.03 ms、已迁移个人库 117.95 ms；用量表 filenode 不变。生产 usage_logs 目录估计 142,173 行，两个新 242 约束所在表估计均为 0 行。251 锁冲突约 5.02 秒后失败，无部分列或账本；解除阻塞后原身份升级成功。DDL 需相关表锁，251/254 自带 5 秒 lock_timeout，252/253/242 无独立该设置，受现有启动上下文约束；实际维护窗口需在生产隔离副本计时并排查长事务，合成耗时不保证线上锁等待。
- 精确候选的 [PR CI](https://github.com/ccisnoxx/sub2api/actions/runs/38124366216)五个作业全部通过，包含完整后端单元、集成、既有录制并发 race、前端 typecheck/critical Vitest、lint、Shell 和发布辅助检查；[PR 安全扫描](https://github.com/ccisnoxx/sub2api/actions/runs/38124366232)通过。重复 push CI `38124362518` 已取消，不计通过；CLA 按既有规则 skipped。文档本地链接、全量源树比较和 `git diff --check` 通过。
- fresh、只读 `critical_reviewer` 对同一候选完成独立复核，无确认问题或阻断项；独立核对三个固定来源全部 SQL、清单及 runner，并读取最终三路径原始日志。未独立重查现场、重跑测试或验证完整业务流量，已在 PR 记录覆盖边界。审计 ID `20261011T080631Z-mig-001-df3f90bc` 已闭合并校验，无异常。
- [审核与失败恢复记录](https://github.com/ccisnoxx/sub2api/blob/c0e7ab68b8479fc9d6f3b888d548eca4f4192974/docs/migrations/MIG-001.md)和 PR 已记录数据库影响、每文件事务及已提交前缀、同身份重试、应用回退与隔离恢复。当前部署证明不覆盖新 KIN / 新 242，实际切换须由后续发布任务更新证明，不能绕过控制面。未合并、发布或部署；仅完成 MIG-001，未重放 S1–S4 或修改其他任务候选。

---

## UPSTREAM-002：重放 S1 用量计时与完成状态

**优先级：** P0  
**依赖：** `UPSTREAM-001`、`MIG-001`  
**建议分支：** `integration/kin-<tag>-s1-usage`

### 目标

将严格首输出、末次 Token、完成状态、音频输出 Token 和用量来源迁移到新 KIN，同时不改变旧计费行为。

### 实施步骤

- [x] 重新定位新 KIN 中 HTTP、SSE、WS 和透传入口；
- [x] 保留旧 `first_token_ms` 语义；
- [x] 新字段仅记录实际观察到的事实；
- [x] 不把缺失值当作零或成功；
- [x] 不新增失败请求计费行；
- [x] 保持已有重试、扣费、幂等和异步记录 owner；
- [x] 对取消、完成、上游错误和中断建立冻结规则；
- [x] 重新适配新 KIN 已修改的 OpenAI/Codex 请求路径；
- [x] 更新迁移和导出字段。

### 验收标准

- 旧用量字段和计费结果与目标 KIN 基线一致；
- 新计时字段在未接入入口保持版本 0、NULL 或 unknown；
- 客户端取消后不会被后续完成事件错误覆盖；
- HTTP、SSE、WS 多 turn、交错 response ID 均有定向测试；
- `go test -race` 覆盖观察器和异步用量复制路径。

### 最低测试范围

- 非流式 JSON；
- 普通 SSE；
- 透传 SSE；
- HTTP→pooled WS；
- WS pooled ingress 多 turn；
- WS→HTTP bridge；
- 客户端取消；
- 上游失败；
- 工具、推理、图片和音频输出；
- 缺失 terminal event。

### 本轮完成证据（2026-10-11）

- 前置UPSTREAM-001/MIG-001已验收；固定新KIN `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71`。从MIG-001创建独立S1分支，没有merge旧personal历史。最终候选 `ee415b62ea9076b7f682ab73fd94203478ca363a` / [草稿PR #20](https://github.com/ccisnoxx/sub2api/pull/20)，报告 `docs/upstream/UPSTREAM-002.md`。
- 保留旧first_token_ms、计费/倍率/扣费/幂等/重试和异步owner；九个新事实保持strict/NULL/unknown/version0，取消与首可信终态冻结；包装音频缺失保持原parser选择与NULL。JSON、普通/透传SSE、HTTP pooled WS、native多turn/交错ID、bridge、取消/失败/工具/推理/媒体/无terminal已有定向证据。observer/async race、真实SQL/Ent PostgreSQL集成、项目make build均通过。
- 首轮CI的旧usage API fixture缺新增字段，修正历史行NULL/unknown/version0预期；12项API合同通过。最终同SHA [PR CI38128573227](https://github.com/ccisnoxx/sub2api/actions/runs/38128573227)五job与实际单元/集成/录制race全部success；[Security Scan38128573323](https://github.com/ccisnoxx/sub2api/actions/runs/38128573323)success。旧失败/取消、重复push和CLA skipped不计成功。
- fresh只读critical_reviewer对相同运行实现复核，无确认阻断；最终额外差异仅历史fixture和报告，父任务核验绑定。未独立重跑测试，原部分成功stdout未存文件但runtime exit已核验；API fixture并非完整生产流量证明。正式累计S4的121项共同owner race与真实PG source/usage合同再次覆盖交叉owner。
- 导出与显示字段按明确阶段归属在006独立PR #24完成；实际CSV/XLSX的末12列、NULL/false/0和未舍入值验证通过。SQL251–254原名原字节与MIG-001一致，没有DDL、历史backfill或失败计费行。回退独立S1应用提交并保留加法schema/账本；完整风险与回滚在PR和报告。
- 维护者明确允许002–006采用固定候选PR CI + Security Scan，Personal CI/来源切换留007，不修改任何祖先/绑定检查。原工作区分支/HEAD和plan原暂存blob保留；未合并、发布、部署或修改生产。

---

## UPSTREAM-003：重放 S2 用户模型与价格目录

**优先级：** P0  
**依赖：** `UPSTREAM-001`、`UPSTREAM-002`  
**建议分支：** `integration/kin-<tag>-s2-catalog`

### 目标

恢复基于用户真实授权组、渠道和计费规则的只读模型目录。

### 实施步骤

- [x] 重新定位目标 KIN 的用户授权、组、渠道和计费 resolver；
- [x] 保持 `view=catalog` 的显式选择机制；
- [x] 只枚举用户实际有权访问的活跃组和活跃渠道；
- [x] 仅展示可解释价格；
- [x] 请求相关、响应相关或媒体单位不明确时返回 unknown，而不是猜测；
- [x] 保持价格倍率与基础单价分离；
- [x] 不序列化内部 Group、Channel 或领域对象；
- [x] 修复多渠道绑定时的歧义，参考 `ARCH-003`；
- [x] 更新前端目录页和国际化文案。

### 验收标准

- 未授权用户无法看到隐藏组、渠道或模型；
- 相同用户在不同授权条件下得到正确目录；
- 目录查询不触发真实上游请求、结算或用量记录；
- 无法精确计算的价格明确标记原因；
- 旧接口在不带 `view=catalog` 时行为不变。

### 本轮完成证据（2026-10-11）

- 直接基底为已验收S1 `ee415b62ea9076b7f682ab73fd94203478ca363a`。最终候选 `43630d16feb9a45fa4cfa92f66e5782dd0213ad7` / [草稿PR #22](https://github.com/ccisnoxx/sub2api/pull/22)，独立S2增量与报告 `docs/upstream/UPSTREAM-003.md`。
- 精确single view=catalog显式选择；复用真实用户活跃授权组、活跃渠道和有效订阅，独立白名单DTO，基础价/个人时段倍率分开，未知原因/明确零价不同。遵守已有group绑定唯一合同，重复活跃渠道明确失败且返回顺序无关，内部ID不泄漏；旧无opt-in接口不变，不发上游、不结算、不写用量。
- 准备定向unit143、真实PG/Redis/JWT11、目录Vitest10、typecheck/i18n/build/变更lint通过；正式S1整合unit161、真实PG HTTP/JWT与账单对照、项目build通过。S2/S1运行文件无冲突。首轮golangci QF1003用局部等价switch修正，报价/倍率22项通过，未更改lint配置。
- 最终同SHA [PR CI38129465410](https://github.com/ccisnoxx/sub2api/actions/runs/38129465410)五job全部success，实际unit/integration/recording race通过；[Security Scan38129465420](https://github.com/ccisnoxx/sub2api/actions/runs/38129465420)success。首轮lint失败/被替代取消、重复push、CLA skipped不冒用为成功。
- fresh只读critical_reviewer复核权限/价格/unknown与zero/DTO/顺序无关冲突，无确认阻断；正式运行实现除等价switch外一致，父任务补核DI/API/billing→RecordUsage共享边界。未独立重跑测试，准备PG原始stdout未保存由runtime核验；正式整合日志保存。未做生产结算，目录合同本身禁止该写入。
- 无SQL/runner/账本/依赖/控制面修改，SQL251–254逐字节保留；撤销独立S2应用/报告提交，保留S1/MIG-001加法schema。风险、数据库影响、回滚及维护者授权阶段门禁边界已在PR记录；原工作区暂存内容保留，未合并/发布/部署。

---

## UPSTREAM-004：重放 S3 结构化路由诊断

**优先级：** P0  
**依赖：** `UPSTREAM-001`、`MIG-001`  
**建议分支：** `integration/kin-<tag>-s3-routing`

### 目标

在不改变调度结果的前提下，记录一次真实选择评估的不可变诊断快照。

### 实施步骤

- [x] 重新定位目标 KIN 的 sticky、guardian、previous response、load balance 和 gateway pool 路径；
- [x] 每次完整重新评估使用独立 `selection_attempt`；
- [x] 只记录实际观察到的候选池和过滤事实；
- [x] 未探查、排序落后或可等待账号不得计为已过滤；
- [x] 只接受白名单层和原因码；
- [x] 保持诊断对象深复制，避免后续异步修改；
- [x] 仅在管理员错误详情中返回；
- [x] 历史记录和未采集记录保持 SQL NULL；
- [x] 不向普通用户错误接口泄露内部账号或过滤信息。

### 验收标准

- 调度、重试、等待计划、扣费和 sticky 行为与目标 KIN 一致；
- `candidate_pool`、`filtered_candidates` 和原因计数保持一致；
- 未完整观察时使用 `partial` 或 `unobserved`；
- 非法诊断对象不会覆盖或丢弃真实故障；
- HTTP 和 WS turn 隔离测试通过。

### 本轮完成证据（2026-10-11）

- 前置 UPSTREAM-001/MIG-001 已验收；以 MIG-001 `c0e7ab68b8479fc9d6f3b888d548eca4f4192974` 为基准，在独立分支 `codex/upstream-004-s3-routing` 重放固定 A 的 S3，没有 merge 旧历史。冻结 KIN tag object/peeled commit 收尾再次核对一致。
- [草稿 PR #21](https://github.com/ccisnoxx/sub2api/pull/21) 最终候选 `d06e9cd55b1b121f42427cebf8c6d3218222b3ca`；运行提交 `a38c26b77f0e5638dfd6d57e2fe1cd1c3b5a5ecb` 仅包含 S3，报告位于 `docs/upstream/UPSTREAM-004.md`。原工作区分支/HEAD及 plan 原暂存 blob 保留。
- 真实调度/HTTP与WS turn、非法诊断保留真实错误、管理员JWT与普通用户/列表裁剪定向检查通过；五包 race、真实 PostgreSQL/Redis 诊断持久化/NULL/批失败与252幂等检查及项目 build 通过。迁移251–254与MIG-001/A三方逐字节一致，无DDL、依赖、CI、source json或前端修改。
- 同固定 SHA 的 [PR CI 38128714451](https://github.com/ccisnoxx/sub2api/actions/runs/38128714451) 五个作业全部 success，实际单元/集成/录制并发检查通过；[Security Scan 38128714460](https://github.com/ccisnoxx/sub2api/actions/runs/38128714460) success。重复 push CI 取消不计通过，CLA skipped不计通过。维护者明确授权阶段 PR CI + Security Scan；Personal CI、来源切换留007，未更改任何绑定/祖先检查。
- fresh `critical_reviewer` 独立只读复核固定候选，实际检查源码/contract/callers、原定向日志与回归红灯，未确认阻断。未重跑测试；临时日志缺完整命令/SHA/exit元数据，构建及候选绑定由父runtime核验。JWT用户仓库替身、WS simple/capture上游不代表线上扣款；跨峰谷后同连接下一turn的完整运行时反例未额外执行，冻结传递/不变KIN刷新owner及已有测试支持。完整覆盖限制已记PR。
- PR已记录风险、数据库影响及按独立提交撤销代码、保留加法schema/账本的回滚方法。该独立阶段验收后，S1/S4正式累计的共同owner race121和真实PG合同已在PR #23验证；本请求最终勾选002–006。未合并、发布、部署或运行生产请求。

---

## UPSTREAM-005：重放 S4 服务状态观测与聚合

**优先级：** P0  
**依赖：** `UPSTREAM-002`、`UPSTREAM-004`、`MIG-001`  
**建议分支：** `integration/kin-<tag>-s4-status`

### 目标

恢复基于真实请求生命周期事实的服务状态聚合，而不是依赖合成探针或推测。

### 实施步骤

- [x] 重新接线 usage 和 ops error 中的 observation metadata；
- [x] 保持匿名聚合与计费数据逻辑独立；
- [x] 保持配置版本、观察起点和水位语义；
- [x] 重新验证 10 分钟重算和 90 分钟关联边界；
- [x] 重新验证分组可见性变化触发器；
- [x] 重新验证 incident 状态转换；
- [x] 聚合失败不得推进成功水位；
- [x] 未知、样本不足和源错误不得输出虚假成功率；
- [x] 增加容量和隐私控制，参考 M5。

### 验收标准

- 聚合任务幂等；
- 同一分钟重复执行不会产生重复事实；
- 关闭和重新启用配置不会沿用旧恢复计数；
- 普通组和 composite 组的平台归属正确；
- 源数据缺失时状态为 unknown，而不是 operational；
- 管理接口权限、请求体大小和参数白名单测试通过。

### 本轮完成证据（2026-10-11）

- 前置S1 PR #20、S3 PR #21、MIG-001已验收；累计依赖基底 `ff5493e1d71910fa3de0f0e338ead91392b4a99a` 保留S1/S2/S3真实owner。最终S4 `39753447cb4287c1a0aeee68b79b589ff345498a` / [草稿PR #23](https://github.com/ccisnoxx/sub2api/pull/23)仅含独立S4增量和阶段记录；完整报告 `docs/upstream/UPSTREAM-005.md`。
- 真实HTTP/WS逻辑turn随机observation key与计费幂等独立；attempt/取消/首可信terminal冻结与异步深复制，usage/Ops安全内部metadata接线，缺失、低样本、冲突、source gap保持unknown。普通/composite组复用当前13平台归属；管理员严格JWT、8192body、完整PUT/version409/query白名单。
- 每分钟45秒事务预算，10完整分钟重算加此前90分钟关联，50000超限明确失败，facts/incidents/watermark一起回滚；成功才推进水位。幂等、无重复事实、可见性触发器、禁用/重新启用重置、仅新证据恢复、31天TTL/30天历史/32更新和过滤后resolved200均有合同证据。
- 准备14个不同真实PG状态集成、SQL shape、严格JWT/routes race15、最终受影响race88和make build通过；首次passthrough metadata NULL红灯已修复，失败项及受影响范围再验证，不计旧失败为成功。正式累计race121顶层PASS、PG23顶层PASS（14真实状态PG+7纯函数+2目录HTTP/JWT/PG）、make build通过；Ent/Wire重生成无diff，保留S2 Billing注入和DB关闭前source清理。
- 同固定SHA [PR CI38130505409](https://github.com/ccisnoxx/sub2api/actions/runs/38130505409)五job全部success、实际unit/integration/recording race通过；[Security Scan38130505358](https://github.com/ccisnoxx/sub2api/actions/runs/38130505358)success。重复push取消和CLA skipped不计通过。维护者授权阶段标准门禁，Personal CI/来源切换留007。
- fresh独立critical_reviewer对最终候选复核，无确认阻断；实际核对source/turn/迟到attempt/取消/计费、SQL唯一约束、事务锁/水位/新证据恢复、匿名/JWT/13平台和前端DTO，并读取最终日志及原红灯。未独立重跑测试、完整真实后端浏览器联调、旧二进制新旧新回退、生产开启/备份恢复/长期容量SLO；DB完全不可达末次gap仍best effort，未知不得输出成功率。
- 无SQL/runner/backfill/依赖/控制面更改，251–254原名原字节保留；默认关闭。回退前按最新version完整PUT关闭配置，再撤销S4应用增量，保留加法schema、账本和历史；重新启用重新累积完整窗口。PR记风险、数据影响及回滚；未合并、发布、部署。

---

## UPSTREAM-006：重放前端、导出、文案和管理界面改动

**优先级：** P1  
**依赖：** `UPSTREAM-002`—`UPSTREAM-005`  
**建议分支：** `integration/kin-<tag>-frontend`

### 目标

将个人功能的前端显示和导出迁移到新 KIN，同时减少与上游 UI 的冲突。

### 实施步骤

- [x] 更新 UsageTable、首输出和 TPS 组件；
- [x] 更新 CSV/XLSX 导出字段；
- [x] 更新模型目录展示；
- [x] 更新路由诊断管理员详情；
- [x] 更新服务状态设置和历史页面；
- [x] 更新中英文文案；
- [x] 检查暗色、亮色、移动端和键盘操作；
- [x] 保持旧记录与新记录的显示区分；
- [x] 不在用户界面暴露内部渠道 ID、账号 ID 或原始过滤细节。

### 验收标准

- TypeScript 类型检查通过；
- 关键 Vitest 测试通过；
- 1440px 和 390px 基本布局可用；
- 导出列顺序、NULL、false 和未舍入数值符合合同；
- 不引入读取用户上传 XLSX 的代码路径。

### 本轮完成证据（2026-10-11）

- 002–005固定候选均完成阶段验收；从S4 `39753447cb4287c1a0aeee68b79b589ff345498a` 接入唯一006增量。最终 `b656fde65419d729dabfad7f7457f17cb7021fd7` / [草稿PR #24](https://github.com/ccisnoxx/sub2api/pull/24)仅有前端/阶段报告和journal，完整报告 `docs/upstream/UPSTREAM-006.md`。没有重复S2依赖或后端变更。
- UsageTable严格/旧计时和全时长文本TPS扣可信媒体，未知拆分不猜0；CSV/XLSX原列尾部按合同追加12列，实际下载29/47列、NULL/false/0和未舍入292.7536231884058/147.82608695652175保持；未新增读取用户上传XLSX。目录、管理员单详情白名单诊断、服务状态设置/历史/unknown/低样本、双语/亮暗/键盘已接线；普通用户无内部渠道/账号/原始过滤细节。
- pnpm9冻结安装、18文件263个Vitest通过；最后窄屏直接影响3文件107个、typecheck/41文件lint、i18n/vue-tsc/Vite等价生产build通过。Chromium真实中英×亮暗×1440/390八环境48验收及11扩展权限/停用/晚响应流程通过；实际请求/DOM/导出可用，无意外console/pageerror/unmatchedAPI。既有Browserslist/chunk提示如实保留，没有无关依赖更新或分包重构。
- 正式frontend全树与测试/独立复核a83ef2457dcccc898bde7f5bc3dc647be69606d3逐字节一致，package/lock/config/deps未改，成功证据复用。正式backend与S4相同，复用真实JWT/routes15、PG23和共同owner race121；实际DTO、13平台、nullable数组/比例、range/version409/disabled503逐字段吻合。浏览器合成API/session不冒用为后端权限或生产业务证明。
- 同固定SHA [PR CI38130636030](https://github.com/ccisnoxx/sub2api/actions/runs/38130636030)五job全部success、实际unit/integration/recording race与前端typecheck/critical Vitest通过；[Security Scan38130636015](https://github.com/ccisnoxx/sub2api/actions/runs/38130636015)success。重复push取消/CLA skipped不计成功。维护者明确授权阶段门禁，Personal CI/来源切换留007。
- fresh独立critical_reviewer对相同前端源码无确认问题；实际解析下载列/类型/原值，并核对权限/晚响应/未知码/页面及提示证据；未重跑测试或生产业务，部分本地typecheck/lint exit由父runtime核验。S4独立复核另覆盖后端合同。执行审计 `20261011T084732Z-upstream-002-006-replay-f2a659c8` 已闭合且校验通过，无异常。
- 无SQL/后端/依赖/控制面修改；回退006独立应用/文档提交恢复S4基底前端，保留S2目录、S1/S3/S4后端和MIG加法schema。风险、无数据库影响与回滚已记PR；原工作区分支/HEAD、原暂存blob保留，未合并、发布或部署。仅本请求002–006勾选，007及后续任务保留未完成。

---

## UPSTREAM-007：完成全量回归并切换 `personal` 基线

**优先级：** P0  
**依赖：** `UPSTREAM-002`—`UPSTREAM-006`  
**建议分支：** 集成分支

### 目标

在所有阶段验证完成后，以受控方式替换旧 `personal` 基线。

### 实施步骤

- [x] 运行后端单元测试；
- [x] 运行集成测试；
- [x] 运行前端测试和类型检查；
- [x] 运行 race 检查；
- [x] 运行安全扫描；
- [x] 运行部署脚本测试；
- [x] 使用真实 PostgreSQL 验证三种数据库升级路径；
- [x] 完成 dry-run Release；
- [x] 审核迁移差异和发布产物；
- [x] 更新 `personal-source.json`；
- [x] 通过 PR 将新集成历史合入或切换到 `personal`；
- [x] 保留旧 `personal` tag，禁止删除旧稳定标签。

### 验收标准

- 所有 required checks 通过；
- `personal-source.json` 与实际新基线一致；
- 发布镜像 revision 与源码 SHA 一致；
- 数据库升级和应用回退验证完成；
- 旧稳定版本仍可通过 tag 获取。

### 本轮完成证据（2026-10-11）

- 前置 UPSTREAM-002—006 均已验收。开始时维护分支 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；使用独立 `codex/upstream-007-baseline-switch`，原分支/HEAD和暂存blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。冻结及结束复查均为 KIN `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71`，annotated tag对象 `0c1ede31ec63f9cfc2b72f07db66fb41f540208d` 未变。
- [PR #25](https://github.com/ccisnoxx/sub2api/pull/25)以普通merge合入受保护personal，实际提交 `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb`；两个父为旧personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 和候选 `560b0459e239e70c189961dec9767d09fdf5d47b`，最终tree与候选逐字一致。旧personal、新KIN、MIG和S1—S5累计历史均为真实祖先；来源三字段指向冻结新KIN。仅适配已审查main控制面及LEGAL-002 / SEC-002 / DOC-002必要材料，完整协议见 `docs/upstream/UPSTREAM-007.md`。
- 候选[Personal CI38134277895](https://github.com/ccisnoxx/sub2api/actions/runs/38134277895)与实际personal[Personal CI38135489900](https://github.com/ccisnoxx/sub2api/actions/runs/38135489900)均11个job全部success，包括真实后端unit/integration/recording race、前端typecheck/critical Vitest/TPS build、严格安全、部署脚本、同步及发布工具合同和末尾personal-ready。候选[Release dry-run38134531480](https://github.com/ccisnoxx/sub2api/actions/runs/38134531480)及实际personal[dry-run38135521737](https://github.com/ccisnoxx/sub2api/actions/runs/38135521737)均success；旧候选被替代的CI cancelled、CLA及版本同步skipped均不计通过。
- 最终真实binary archive与OCI的完整SHA、SOURCE.json、revision/source/version、descriptor/外层artifact摘要、五份许可材料均核对；镜像revision精确为实际personal提交。Linux amd64运行manifest `sha256:b76760d5cfb7e36ccf9822297b7a20e81233df25aa52a231018dd2f1924e18cc`；原LICENSE字节保留，严格前端许可28包摘要 `6c2468b5b42456ee43bfd60daa0de8631b073186692779cc57625861d6905800`，无Airwallex SDK分发。dry-run版本继承源码VERSION `0.2.14`，不分配正式个人tag。
- 使用最终同SHA dry-run不可变OCI在真实PG18.6/Redis8.4演练：空库0→308、旧KIN303→308、已迁移personal307→308，三个子测试本轮全部通过（37.79秒）。每条均真实候选/固定旧应用/再次候选健康、完整停止、旧checksum/applied_at和308账本、10k原始用量/NULL默认、五张状态表version7/facts/watermark保留，并完成真实pg_dump/pg_restore。308SQL/runner/Ent/Wire/Go依赖/Makefile与R零diff，251—254原名原字节保留；没有生产SQL或恢复。候选演练首轮空库的固定旧amd64镜像在ARM64 daemon无启动日志退出139，仅失败路径定向复测通过；底层原因未确认，首次失败/诊断记录保留，不能冒用为首轮全量成功。
- 本地完整前端首轮386文件3226测试成功，1个继承CPR fixture漏项已最小修正并定向5测试通过，其余成功证据复用，未声称整套重跑通过；支付/历史/路由/许可136 Vitest、XLSX33、typecheck/lint/严格build、Go支付定向通过。发布helper29、同步21、personal部署60、审计policy14、unit runner6、Apple生命周期与7项Compose/Caddy/安装检查、完整169本地文档链接均通过。fresh独立critical_reviewer对最终候选无未解阻断，未独立动态重跑CI/数据库；执行审计 `20261011T102932Z-upstream-007-baseline-switch-7c36ae6f` 闭合且校验通过，无异常。
- 维护者明确授权自动发布开关保持false；实际[Personal release38136680180](https://github.com/ccisnoxx/sub2api/actions/runs/38136680180)及prepare均skipped。main与personal保护不变，三个旧稳定tag身份保留；无正式发行tag、registry推送或生产部署。现有主机部署允许清单仍仅批准旧基线，新基线部署证明留REL-001。回退使用新的受保护PR和固定旧digest `sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530`，保留扩展schema/ledger，禁止强推/删tag/在线覆盖数据库。仅勾选本请求UPSTREAM-007。

---

## UPSTREAM-008：补回个人更新服务与版本徽章

**优先级：** P1

**依赖：** `UPSTREAM-007`、`CI-000`

**建议分支：** `codex/upstream-008-personal-update`

### 目标与已确认缺口

旧个人基线 `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 已有个人更新合同；迁回后的 `origin/personal` `bea1db12d5f4853c0b85b44840e15de6a74309b1` 只保留 KIN 更新逻辑。个人 Release 来源、`container/manual` 限制、`tps` 序号比较、来源缓存隔离及徽章状态未完整重放。本条独立记录该遗漏，不把既有 `UPSTREAM-006/007` 的阶段验收改写成已经覆盖它。

### 实施步骤

- [x] 从当前 `origin/personal` 隔离实施，仅迁回更新服务、API 类型、版本缓存、徽章、双语文案及直接相关测试；
- [x] `X.Y.Z-klno.N-tps.N` 个人构建使用 `ccisnoxx/sub2api` Release 列表，包含已发布 prerelease，过滤 draft 与非个人标签，按基础版本、`klno`、`tps` 数值选最新；原上游继续只读监测；
- [x] 后端明确报告 `release_repository` 与 `update_mode`：个人 release 为 `container`，源码构建为 `manual`；更新、备份回退、指定版本回退及在线回退列表在查询、下载和文件替换前拒绝；保留普通 KIN release 的原地更新合同；
- [x] 缓存绑定 Release 来源，拒绝迁回前或其他仓库缓存，重启后依据当前版本重算更新状态；
- [x] 徽章展示个人序号及原上游基础版本，个人镜像只提供 Release 链接与固定 digest 部署提示，源码保留手动更新说明；检查未完成或失败不显示“已是最新版本”；
- [x] 执行更新服务、版本缓存、徽章和必要 API/类型/文案的定向验证，并取得独立只读复核；不触发 Quick/Full、安全扫描、Release dry-run、镜像发布或部署。

### 验收标准

- 个人更新不会混用 KIN 或原版 Release；`tps.2` 到 `tps.10` 按数值判断，基础版本和 `klno` 优先；
- `container/manual` 的服务端更新与回退限制有效，徽章没有可执行的二进制更新或回退入口；
- 旧来源缓存、刷新失败和容器重启不造成假更新或假“最新”；
- 直接受影响的检查通过，交付记录区分本地候选与远端集成状态，并说明未执行发布/部署及真实覆盖边界。

### 风险与回滚

个人发布查询沿用客户端单页最多 100 条的边界；本轮不验证镜像可用性或真实部署。新增响应字段需要前后端配套交付。无数据库、依赖、发布控制面或生产配置变更；回退本任务应用提交即可恢复原行为，但会重新暴露该迁回缺口。已有计划暂存内容保持不变。

### 本地验收证据（2026-10-12；远端集成见下文）

- 从最新 `origin/personal` `bea1db12d5f4853c0b85b44840e15de6a74309b1` 创建独立分支 `codex/upstream-008-personal-update`；应用迁回提交 `acf05d1fdae3928658c17ccc8db20c7ea77e170e`，HTTP 合同修正 `566bba0a86500aa70440b248a4db13382d1dd02b`。隔离工作树交付记录为 `docs/upstream/UPSTREAM-008.md`。首次实施结束时未推送或合入远端 personal，未创建 PR；原主工作区分支/HEAD及暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。
- 恢复个人 Release 列表、draft/标签过滤及 `tps` 数值比较；`container/manual` 拒绝四种二进制操作，来源缓存隔离与重启重算、徽章个人后缀与检查状态已接线。独立复核发现在线回退列表原映射丢失 409/reason，已改用统一错误响应；普通错误仍为 500，其 message 改为 `internal error`，原始错误进入脱敏日志。
- Go 更新服务/版本比较定向 27 项通过；Vitest 徽章/版本缓存/API/文案 4 文件 18 项通过；真实 UpdateService HTTP 定向检查通过，覆盖三类构建×四类操作共 12 个拒绝场景，保留既有 handler 回归。vue-tsc、7 文件 ESLint、gofmt 与实际 diff 检查通过；pnpm 9.15.9 冻结离线安装成功，首次默认 pnpm 11 的锁配置不兼容不计通过，没有更改依赖。HTTP 修正后只跑直接影响的 handler 范围，其余未变成功证据复用。
- 两名 fresh critical_reviewer 分别只读复核原应用候选和最终 HTTP 增量，确认唯一 P2 已修正，无未解发现；没有独立重跑测试。审计 `20261012T144000Z-upstream-008-personal-update-6982198c` 已闭合并 audit-verify 通过，无异常或观察到的复核写入。初始验证由父工具输出确认，handler 原始 stdout 已保存并复核。
- 无 SQL、依赖或发布控制面修改；未运行全量回归、云端 Quick/Full、安全扫描、Release dry-run、镜像推送或部署。HTTP 用内存操作锁和空 GitHub/缓存客户端验证拒绝合同，未验收生产认证、真实幂等数据库、镜像安装/重启或数据兼容；“查询前拒绝”专指更新缓存、Release 查询和二进制操作，POST 仍先取得系统操作锁。

### 远端集成与记录同步（2026-10-12）

- 维护者明确授权推送并合入 personal；复查远端 personal 仍为原基线 `bea1db12d5f4853c0b85b44840e15de6a74309b1`，以普通原子 push 发布任务分支 `codex/upstream-008-personal-update` 并将 personal 快进至候选 `6a7cd4124044e58e329e84a1752db0c47fb95cc0`，远端回读确认。未强推、未改写历史或改变分支保护；按 CI-000 不要求为本次快进创建 PR。
- 代码、依赖、配置及测试与已验收候选一致，复用更新服务 27 项、前端 18 项、系统接口 12 个拒绝场景、类型/lint 及两次独立只读复核证据；旧审计 audit-verify 再核对通过。本轮只补文档、源码树一致性、祖先、实际 diff 和远端状态核对，不重复运行测试。
- 计划与 `docs/upstream/UPSTREAM-008.md` 同步至 personal 和 main 文档面；main 从 `d5d1caa6c4df940b058930c7963fc20b2736860f` 隔离实施，仅提交这两份文档，不合并 personal 应用树。主工作区原分支/HEAD及计划原暂存 blob 保留。
- 未手动触发 Quick/Full、安全扫描、Release dry-run、发布或部署。真实镜像安装、生产认证/幂等数据库和数据兼容仍未验收；未来发布必须在最终 personal SHA 手动完成 Full 并按既有流程执行。

---

## CI-000：个人项目手动 CI、月度安全扫描与轻型分支保护

**优先级：** P0
**依赖：** PREP-002；沿用已复核的 CI-001—005 控制合同
**建议分支：** `codex/ci-000-layered`

### 当前简化验收（2026-10-12）

- 控制工具普通快进更新至 main `ab86413aa27848239d0374adeff9f7aeaa41609e`；同一组必要控制文件普通快进同步至 personal `53079f406d6615566adf829a19842553be834ac0`。各25个文件限定于 `.github`、CI文档、维护规则及README；没有合并main应用树，应用源码、数据库迁移、来源记录和审计例外保持。
- Personal CI仅手动，backend CI仅手动/复用，安全扫描仅手动/复用/月扫（每月1日03:00 UTC）；Personal release/Release仅手动。日常上游同步只准备候选，只有手动选择CI演练才调度Quick；既有五分钟部署工具路径检查和月度工具版本只读复查保留。
- 经维护者明确授权，通过GitHub API调整并回读main/personal保护：禁止强推和删除、管理员遵守；required checks、PR审查和推送限制均为空，分支未锁定，无额外ruleset。personal普通直接推送成功，未强推；远端更新后两新SHA的Actions运行数均为0（本轮查询时）。
- CPython3.12.13/PyYAML6.0.3：发布工具93项通过；同步/策略38项中新增测试首轮因MagicMock返回值无法JSON序列化失败，修正夹具后重跑受影响test_sync.py23项通过，其余15项输入未变的成功证据复用。actionlint v1.7.12（未运行ShellCheck/pyflakes）、发布shell语法、指定控制文件一致性及diff检查通过；没有运行应用全量回归、云端Full、安全扫描或Docker构建。
- fresh critical_reviewer对固定main候选累计25文件diff、完整发布/同步实现、调用、负向测试及保护payload独立只读复核，未确认阻断；没有独立重跑测试或验收生产。审计 `20261012T094715Z-ci-manual-only-ad28e5cc` 已关闭且audit-verify通过，无异常，工作树未观察到复核写入。
- 真实只读 `personal_release.gate` 对新personal SHA按预期拒绝“没有可信的成功手动 Full CI”。发布仍绑定固定源码/来源祖先、同一次成功手动Full的必要jobs、固定run ID、tag、可信main工具和digest，不依赖分支required checks，不拼接其他运行的结果。
- `PERSONAL_RELEASE_ENABLED=false`、既有同步计划开关保持。没有分配版本、创建正式标签、发布镜像、部署或操作生产数据库。未来发布须在最终personal SHA手动Full后进行Docker候选构建和实际冒烟；本轮静态/合同和只读拒绝验收不代替该发布验收。
- 原工作区旧文档分支/HEAD及计划原暂存blob保留。回退时普通revert两组控制提交并同步生产者/消费者，分支保护从变更前快照单独恢复；恢复旧自动发布触发前须保持发布关闭。旧十任务观察不再是本任务完成条件，不补造后续任务或耗时。

### 旧分层方案记录（历史，非当前执行要求）

### 目标

把昂贵门禁放在最终候选、合并和发布边界，日常迭代先本地定向检查与 Quick，避免 push、PR、同步 dispatch 和 post-merge 重复运行同一 Full。

### 实施与验收

- [x] 统计最近二十次 Actions，区分成功、失败、取消、跳过、运行中和已确认的基础设施故障；
- [x] Personal CI 增加 docs/backend/frontend/migrations/dependencies/workflows/deploy/high_risk 分类；删除、重命名和未知运行路径不能绕过 Full；
- [x] backend-ci.yml 保留单一实现，提供 quick/full 模式；PR 更新默认路径相关 Quick，文档不安装应用依赖；
- [x] 最终候选提供 Ready、full-ci 标签和完整 SHA 手动 Full 入口；保留个人分支唯一 required check personal-ready；
- [x] 高风险候选缺 Full 必须阻断；普通应用候选合并后 Full，合并前已有成功 Full 时仅在 candidate/base/tree 全部匹配时复用；
- [x] govulncheck/pnpm audit 按最终 Full、对应依赖变化、周扫和手动安全审计执行，保留固定版本与扫描失败传播；
- [x] 将前端普通检查和 TPS 放到同一 job，只安装一次应用测试依赖；
- [x] 同步专用 PR 只显式 dispatch，取消功能分支 push/PR 双触发，关闭 fork 中无效 CLA 自动入口；
- [x] 发布工具要求最终 personal SHA 的完整检查证据，Quick、取消、旧来源或缺失完整 job 均不能用于发布；
- [x] 更新执行协议，完成候选、本地定向验证和 diff 核对后批量推送；失败先定位具体 job，不盲目重跑全部 Full；
- [x] 控制候选完成定向测试、独立只读复核和 PR 记录，合入 main 后仅同步必要控制增量至 personal，并执行首次高风险 Full；
- [ ] 上线后连续记录至少十个独立任务：文档约三分钟、小任务首次反馈十分钟内、普通任务最多一次 Full、无重复 Full、旧 Quick 快速取消、周扫保留。

### 旧分层方案当时状态（已由当前简化验收替代）

控制面 [PR #37](https://github.com/ccisnoxx/sub2api/pull/37) 已合入 main，合并提交 `cd5afc67fc83c021c72aa056c9f29277949ae4ff`；必要控制同步 [PR #38](https://github.com/ccisnoxx/sub2api/pull/38) 已合入 personal，上线合并 SHA 为 `7ebb60b2f6bf43e5f73c38e37b61de6cb5f063e6`。唯一候选 Full、安全扫描、Ready/标签/dispatch、文档 Quick、旧 Quick 取消、merge 复用及只读发布门禁已完成本轮验收。原工作区/暂存内容保留；完整证据与覆盖边界随 `docs/ci/CI-000-ROLLOUT.md` 交付。十个后续独立任务观察仍待完成，整体 CI-000 保持未勾选；不以本次验收事件充数。发布开关保持 false，未发布或部署。

### 本轮候选证据（2026-10-11）

以下保留 PR #37 交付时的历史状态；已完成的上线结果见下一节。

- [草稿 PR #37](https://github.com/ccisnoxx/sub2api/pull/37) 面向 main，唯一候选 `b267b6ecd5824d74d94d7d6d8d924e94f4c58fae`；从隔离分支实施，并快进保留最新 main `b3b42f13f292851cf11c9780e6694c1bdd25b615` 的既有许可 helper。仅一提交/一次远端 push；应用、数据库、依赖版本、保护规则和审计例外无修改，原计划暂存 blob 保留。
- CPython 3.12.13 / 哈希固定 PyYAML 6.0.3：发布合同90项、同步/分类合同40项成功；actionlint v1.7.12（关闭未安装 ShellCheck/pyflakes）、Bash 语法、变更文档本地链接及 diff 检查成功。晚完成 Full 和同最终 SHA 发布复用用例先确认修复前红灯，再修复通过；真实只读统计 CLI 调用成功。
- 同固定 SHA 的 [Quick 38200259416](https://github.com/ccisnoxx/sub2api/actions/runs/38200259416) success，1分19秒；分类、文档、发布/同步工具和 ci-ready 成功，未选中的应用 job skipped 不计通过。唯一 [backend Full 38200296702](https://github.com/ccisnoxx/sub2api/actions/runs/38200296702) success，12分02秒，8个 jobs 全部成功；单元/集成测试、recording 并发检查、全库 lint、macOS 部署脚本、前端 typecheck/关键 Vitest/build 及发布辅助合同通过。main 无 personal TPS 模块，专项按适用性跳过，仍 build；personal 安全扫描、绑定及发布复用真实上线验收仍待完成。不重复 dispatch/re-run。
- 三轮 fresh critical_reviewer 只读复核；前两轮确认的门禁跳过、敏感所有权漏判、main TPS 适用性、发布根证据生命周期及并发/去重交错问题已修正，最后一轮无确认发现，十个新增内存反例通过。最终冻结文件与提交逐字节一致；审计 `20261012T025134Z-ci-000-layered-20defb5d` 已关闭并校验，无写入观测、活跃残留或异常。静态/桩复核不冒充真实 Actions 上线验收。
- main→personal 同步、首次 personal Full/安全审计、真实标签/Ready/dispatch、晚完成证据和发布失效根拒绝及至少十个后续任务观察尚待完成；整体 CI-000 不勾选。数据库影响 none；未合并、发布或部署。

### 上线推进证据（2026-10-11）

- 按维护者本轮授权普通 merge PR #37，`2026-10-12T03:53:53Z` 合入 main；未强推或放宽保护。后续 personal 同步以 `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 为基础，固定候选 `b07ff16dcf7ed1a1a9773970daef9024c1bc6c70`，面向 personal 的 PR #38。
- 同步必要 `.github` 控制依赖与 CI 文档/协议；Dockerfile 只补齐 main 已复核的 Node/Go 镜像 pins、工具链与版本日志，frontend/package.json 仅声明既有 pnpm@9.15.9。应用源码、锁文件、Makefile、上游来源、许可材料和审计例外保持；没有把 main 应用树合入 personal。
- 本地先确认缺失工具 pins/声明导致合同失败，补齐后按失败检查优先顺序验证工具合同10项、发布合同90项、同步合同40项、actionlint、Bash语法、本地文档链接和diff检查通过。独立 fresh critical_reviewer 对冻结候选无确认阻断，并执行16个聚合失败反例、4个专用触发反例及10个必要 Full job 跳过反例；审计 `20261012T035513Z-ci-000-personal-rollout-34cd115b` 已闭合并校验；另一次 fresh reviewer 对两份上线记录与真实API独立只读复核无确认发现，共两次独立复核，无写入观测或活跃残留。
- 唯一显式 [Personal Full 38202385227](https://github.com/ccisnoxx/sub2api/actions/runs/38202385227) success，18分10秒，15个 jobs 全部成功，包括 Go/前端安全扫描、单元/集成/recording Race、前端关键Vitest/TPS/ESLint/build、全库lint、部署脚本、控制合同、full-ci-certified和personal-ready。真实扫描产物绑定固定候选，实际 Go1.27.2、govulncheck v1.9.0、Python3.12.13。
- [PR opened 门禁 38202375583](https://github.com/ccisnoxx/sub2api/actions/runs/38202375583) 62秒及 [Full期间Ready 38202519258](https://github.com/ccisnoxx/sub2api/actions/runs/38202519258) 66秒均按预期拒绝缺失Full；绑定成功，应用/扫描job跳过，原Full未取消。同步调度器真实API复用同一运行中Full，没有再次dispatch。Full完成后的 [标签门禁 38203673542](https://github.com/ccisnoxx/sub2api/actions/runs/38203673542) success，65秒；check_only Quick引用38202385227，应用/安全/同步job跳过，无第二次Full。
- PR #38 于 `2026-10-12T04:26:45Z` 普通 merge 到 personal；两个父分别为基础 `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 和候选 `b07ff16dcf7ed1a1a9773970daef9024c1bc6c70`，最终tree与候选一致。[merge push 38203810810](https://github.com/ccisnoxx/sub2api/actions/runs/38203810810) success，2分17秒，mode=quick并引用同一Full；未重复运行后端全量测试、全库lint或安全扫描。
- 临时文档 [PR #39](https://github.com/ccisnoxx/sub2api/pull/39) 已关闭、未合并并删除验收分支；[synchronize 38203961302](https://github.com/ccisnoxx/sub2api/actions/runs/38203961302) 2分02秒、[Ready 38204006676](https://github.com/ccisnoxx/sub2api/actions/runs/38204006676) 1分34秒均success，未安装应用依赖/测试/扫描/Full。旧SHA opened38203920822 cancelled，下一synchronize创建后34秒结束；第二SHA的两个独立事件均为成功Quick，不冒称全部重复事件已取消。
- 可信main快照的只读 `personal_release.gate` 接受最终personal SHA的38203810810并实时重验根Full。旧政策历史成功push38135489900被拒绝；最终SHA的[手动Quick38204082448](https://github.com/ccisnoxx/sub2api/actions/runs/38204082448) success（70秒）却不能作为发布证据；新Quick完成后的自动查找仍接受38203810810。适配器只允许GET、拒绝payload；没有调用prepare/authorize。
- `PERSONAL_RELEASE_ENABLED=false`、严格personal保护与personal-ready app15368保持；周安全扫描入口启用。两次自动Personal release38203968842/38204164045均skipped，前后个人tag及最近Release清单一致。没有分配版本、创建标签、发布镜像、部署或运行镜像构建dry-run。
- 上线记录 [PR #40](https://github.com/ccisnoxx/sub2api/pull/40) 已普通merge到main `083eb0e5e4fc105db4a259e48aa55cad91667e43`，文档候选 `53e11a33984d37c6816bfd932974b82d39248b7f`；[文档CI38205286430](https://github.com/ccisnoxx/sub2api/actions/runs/38205286430) success，43秒，changes/docs/ci-ready成功，其他应用/工具job按路径跳过。两份文档通过fresh独立只读复核，主记录不变，未重复Full或安全扫描。
- 十个后续独立任务观察保持待完成；未人为删除/重跑失败远端根Full，精确binding后/certify前完成窗口、失效根拒绝及同最终SHA真实Full复用以本地合同/独立反例覆盖，真实merge复用不冒称这些现场演练。当前实现与主要真实验收子项可勾选，整体任务继续未勾选。

### 旧流程后续独立任务观察（历史；不再作为完成条件）

1. **ARCH-001（2026-10-12，首阶段已合入并验收，整任务未完）**：personal基础 `4cf71d60901e524daf933ba2eebd985d1d378108`，新增首次终态组件及真实返回边界回归；本地定向检查和fresh独立复核通过，固定候选 `0b6f7ea0fdca4b2a87f7e1dddaf0b69ab5850ca3` / [PR43](https://github.com/ccisnoxx/sub2api/pull/43)只一次推送；[Quick38222159383](https://github.com/ccisnoxx/sub2api/actions/runs/38222159383)整轮9分21秒，首次应用反馈8分25秒；应用/文档检查成功，仅完整证据门禁预期拒绝。前端、扫描、全库lint、部署和发布辅助job跳过。Ready唯一[Full38223491602](https://github.com/ccisnoxx/sub2api/actions/runs/38223491602) success，20分23秒，15job全success；PR43普通merge为 `08c3ebb5ef6b87a08955241c62640f1da531b084`，基础/候选父与tree已核对一致。合并后[38226219264](https://github.com/ccisnoxx/sub2api/actions/runs/38226219264) success，9分30秒，真实mode=quick引用同一Full，前端/全库lint/部署/发布辅助/扫描job跳过。认证/ready成功，无第二Full。该Quick仍重跑service包分批单测/vet/后端编译，额外重复包检查成本作为后续优化依据；本轮测试/基础设施失败0，仅缺Full证据预期门禁拒绝1；没有旧Quick需要取消，不冒称完成取消演练。完整任务分阶段推进，同一任务后续PR不另算独立观察。本条已记录首阶段真实云端结论、耗时、失败分类及合并复用；整体ARCH-001仍需后续阶段，十任务观察总项保持未勾选。

数据库影响：none；不发布、不部署、不取消正在运行的其他任务。回滚必须一起撤销 workflow 输入、分类脚本与发布证据合同，不能只放宽发布门禁。

---

## CI-001：将 GitHub Actions 固定到完整提交 SHA

**优先级：** P1  
**依赖：** `PREP-002`  
**建议分支：** `ci/pin-actions`

### 目标

避免 `@v6`、`@v7` 等可变标签在不经仓库审查的情况下改变执行代码。

### 实施步骤

- [x] 列出所有 `uses:` 引用；
- [x] 将第三方 Action 固定到完整 40 位 commit SHA；
- [x] 在同一行注释保留可读版本号；
- [x] 优先处理具有写权限、上传 artifact、登录 registry 和发布权限的 job；
- [x] 建立 Dependabot 或等价流程更新固定 SHA；
- [x] 每次更新 Action SHA 时单独审查 release notes。

### 验收标准

- 正式工作流中不存在未解释的浮动 Action 标签；
- 固定 SHA 对应预期官方 release；
- 工作流语法和 required check 名称不变；
- Dependabot 更新不会自动合并发布权限相关 Action。

### 完成证据（2026-10-11）

- 前置 PREP-002 已由 [PR #13](https://github.com/ccisnoxx/sub2api/pull/13) 合并，提交及本次最新 `origin/main` 基准均为 `c1dae6357b4de7cc11d81547013b87550c4ba586`。执行前为 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；原分支、HEAD 与暂存 blob 保持，只更新本任务状态和证据。
- 在隔离分支 `codex/ci-001-pin-actions` 提交 `1392e4392d3aa039b5510753291ac58d5b35fff4`，创建面向 `main` 的[草稿 PR #26](https://github.com/ccisnoxx/sub2api/pull/26)。12 个文件只处理本任务：8 个工作流的 64 处远程引用完整 SHA/同一行版本注释、Dependabot、固定策略测试、任务记录及其精确 gitignore 放行。2 处本地复用原样保留并解释；完整 66 处引用清单和官方 Release/SHA 映射见 `docs/ci/CI-001.md`。
- 16 个 Action 路径、15 个官方仓库逐项核对非 prerelease Release、tag 解引用后的 SHA 与实际 action.yml，并阅读 release notes；所有固定 SHA 等于执行时原标签解析结果。pnpm/action-setup 的 v6 保持 v6.0.10，没有引入 v6.1.0。工作流其他字节完全不变，名称、触发器、权限、required check 标识与源码/CI 绑定保持。
- Dependabot 仅 github-actions，每周逐个向 main 提出更新 PR；维护者必须单独记录官方 release notes 审查、独立复核后手动合并。只读 API 确认 `allow_auto_merge=false`，无自动合并工作流，明确禁止发布权限相关 Action 自动合并；本次没有改仓库设置或声称真实调度已发生。main 保护查询 404，不把 CODEOWNERS 或名称保持冒充强制保护。
- 新增策略测试先在原工作流上产生 8 个预期失败，替换后与既有 release helpers 共 31 项通过；actionlint v1.7.12 语法/表达式/依赖检查、逐字节不变量对比、Dependabot YAML/目标分支及任务候选 diff 检查通过。误在原工作区对照 main 做 diff 检查时命中台账原有 Markdown 两空格换行，已确认属于开始时未变内容；没有整理无关格式。
- 同固定 SHA 的 [PR CI 38137734599](https://github.com/ccisnoxx/sub2api/actions/runs/38137734599) 五个 job 全部 success，实际 unit/integration、前端 typecheck/critical Vitest、部署 shell 与 release-helpers 通过；[Security Scan 38137734569](https://github.com/ccisnoxx/sub2api/actions/runs/38137734569) success。OpenAI recording race 明确不适用：本次 main 源码没有该模块，未执行 race，未计为通过。重复 push 检查取消、CLA skipped 不算通过。
- fresh critical_reviewer 对精确候选独立只读复核，无确认阻断；检查实际 diff、完整工作流、来源映射、策略正负向/失败传播、更新流程与风险回滚，官方 API 独立抽查 artifact、GoReleaser、Docker 登录与 pnpm annotated tag 均匹配。读取 31 项通过证据并独立比对其余字节，没有独立重跑测试/actionlint、确认云端 CI 或执行发布。源码、HEAD 和工作树保持，无观察写入；审计 `20261011T120110Z-ci-001-pin-actions-8b3f5d9b` 已关闭并校验通过，无异常。
- 风险、无数据库影响和普通 revert 回滚已记 PR/任务文档；工具/容器等传递依赖仍属 CI-002。真实 Dependabot 调度、Release dry-run、数据库恢复演练未执行，不将本次 main 控制面候选视作 personal 已同步或正式发布；后续只搬必要控制增量，并按仓库规则验证同步候选。未合并、发布、部署；仅 CI-001 及其实施步骤勾选，CI-002—005 保持未完成。

---

## CI-002：固定安全扫描和构建工具版本

**优先级：** P1  
**依赖：** `CI-001`  
**建议分支：** `ci/pin-toolchain`

### 目标

提高构建和扫描可复现性。

### 实施步骤

- [x] 将 `govulncheck@latest` 替换为明确版本；
- [x] 固定 GoReleaser 主版本和精确版本策略；
- [x] 确认 pnpm、Node.js、Python 和 Go 版本来源；
- [x] 固定 Python release tools 依赖哈希；
- [x] 记录工具版本到构建元数据；
- [x] 建立周期性工具升级任务，而不是构建时自动取 latest。

### 验收标准

- 相同源码在相同平台使用相同工具版本；
- 安全扫描结果变化可以追溯到工具升级或依赖变化；
- 构建日志明确输出关键工具版本。

### 完成证据（2026-10-11）

- 开始前确认 CI-002 未完成、前置 CI-001 已完成阶段验收；依赖提交为 `1392e4392d3aa039b5510753291ac58d5b35fff4`，其 CI `38137734599` 与 Security Scan `38137734569` 均对该 SHA 成功。[依赖草稿 PR #26](https://github.com/ccisnoxx/sub2api/pull/26) 尚未合并；重新 fetch 的 `origin/main` 为 `c1dae6357b4de7cc11d81547013b87550c4ba586`，是依赖提交的祖先。
- 原工作区为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；保留该分支、HEAD、暂存内容及其他任务状态。本任务在依赖提交的隔离分支 `codex/ci-002-pin-toolchain` 实施，[草稿 PR #27](https://github.com/ccisnoxx/sub2api/pull/27) 以 CI-001 分支为基底，只展示 CI-002 的 17 个文件增量。
- 实现提交 `3c69a4f46bff0b0a71f4306c97f89e5d09c77755`，修正及最终验收提交 `bef03cf59d781d053706d0b4c77b7555c4596711`；代码已提交和推送，未合并、正式发布或部署。版本来源、摘要、月度人工升级、验证边界与回滚记录于 `docs/ci/CI-002.md` 及 PR。
- 固定 govulncheck `v1.9.0`、GoReleaser `2.18.3`、pnpm `9.15.9`、CI Node `20.20.2`、Python `3.12.13`，Go 继续由应用 `backend/go.mod` 精确选择（本候选 `1.27.2`），关闭自动工具链切换；固定 Docker builder、Buildx、BuildKit 和 QEMU 摘要。所有 release-tools PyYAML 安装强制官方 wheel 哈希校验；新增每月只读官方版本复查，升级仍经人工独立 PR，不在构建时安装 latest。
- 本地 CPython `3.12.13` 的 release-tools 41 项、部署工具 60 项、personal-sync 21 项测试通过；实际 Linux wheel 哈希下载、macOS arm64 wheel 哈希安装和篡改哈希拒绝验证通过；固定工具实际版本读取、govulncheck 符号门禁、部署 shell/runtime 检查、release-images 语法、actionlint（未执行 ShellCheck/pyflakes）、工作流合同及 `git diff --check` 通过。
- 最终 SHA 的 [CI 38140571111](https://github.com/ccisnoxx/sub2api/actions/runs/38140571111)、[Security Scan 38140571107](https://github.com/ccisnoxx/sub2api/actions/runs/38140571107) 和 [simple Release dry-run 38140665145](https://github.com/ccisnoxx/sub2api/actions/runs/38140665145) 均 completed/success；CI 五个 job 全部成功，Release 验证 linux/amd64 二进制、镜像构建及 publication 元数据。登录、外部发布授权/验证、DockerHub 更新、Telegram 通知及版本同步均按 dry-run 跳过。取消的重复 CI 不计成功。
- 下载核对安全、前端、二进制和 publication 四份元数据的实际版本、应用/工作流 SHA、run ID 及六项白名单输入摘要；日志明确输出关键工具和同次漏洞数据库更新时间，BuildKit 实际 daemon 为 `v0.34.0`。二进制报告记录原有 `go mod tidy` 后的实际 `go.sum` 摘要；同一 Go 在临时源码目录复现一致，仅删 24 行校验记录，`go.mod` 未变。没有修改该钩子或业务依赖，也不承诺在线漏洞公告内容恒定。
- 初始候选的 macOS Python 不可用、GoReleaser 普通 Action 模式未加入 PATH 两项失败，经真实云端日志和第一次独立审查确认；删除没有 Python 消费者的 macOS 安装步骤，改为固定 GoReleaser install-only 后由同一可执行文件构建及报告，增加安装顺序合同检查。最终候选再次取得全新隔离上下文只读审查，无任务内确认阻断；审计 Bundle `20261011T123445Z-ci-002-pin-toolchain-1ffe4908` 已关闭并校验通过，2 次独立复核、初始拒绝 1 次、最终通过 1 次，0 写入、0 活跃残留。
- 三项验收标准已逐项满足；PR 已记录风险与回滚。未运行五目标完整 Release、真实旧 tag 端到端构建、`deploy/Dockerfile` 完整 builder、macOS x86_64 wheel 安装或月度调度；后者需合入默认分支才生效。本控制增量未同步 personal，依赖 PR 尚未合并；runner/运行镜像/apk 及漏洞数据库仍可变化，不声称逐字节可复现。数据库影响为 none，未改 SQL、迁移、恢复工具或业务数据，未重做恢复演练。未合并时关闭 PR；已合并时普通 revert 本任务两项提交并一起撤销报告消费者，保留 CI-001 Action SHA，禁止 reset/强推，无需数据库回退。

---

## CI-003：增加 SBOM、构建来源和镜像证明

**优先级：** P1  
**依赖：** `CI-002`  
**建议分支：** `ci/sbom-provenance`

### 目标

让每个正式产物能够验证“由什么源码、什么工作流、什么依赖构建”。

### 实施步骤

- [x] 为二进制和镜像生成 SPDX 或 CycloneDX SBOM；
- [x] 保存 SBOM 为 Release artifact；
- [x] 将 SBOM 关联到容器镜像；
- [x] 生成 GitHub artifact attestation 或等价 provenance；
- [x] 在 OCI labels 中写入 source、revision、version、created；
- [x] 发布后验证镜像 digest、revision 和平台矩阵；
- [x] 将验证结果保存为不可变 artifact。

### 验收标准

- 每个正式 tag 都有对应 SBOM；
- 镜像 digest 可以验证其源码 revision；
- 发布说明包含源码 SHA、CI run 和 Release run；
- dry-run 不执行外部发布但仍生成完整验证材料。

### 完成证据（2026-10-11）

- 开始前为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅原有 `AM REMEDIATION_PLAN.md`；原分支、HEAD 和暂存 blob 保留。前置 CI-002 已阶段完成，固定提交 `bef03cf59d781d053706d0b4c77b7555c4596711` 的 CI 38140571111、Security Scan 38140571107、Release dry-run 38140665145 均重新确认 success；[依赖草稿 PR #27](https://github.com/ccisnoxx/sub2api/pull/27) 尚未合并，最新 `origin/main` `c1dae6357b4de7cc11d81547013b87550c4ba586` 是依赖候选祖先。
- 在隔离分支 `codex/ci-003-sbom-provenance` 提交 `82b5a24c729da74e1c297c57cac4d8dc2f3515e2` 和修正 `a49438d0eae8ac963a82df17cccd3b40d0e6cd8f`，[草稿 PR #28](https://github.com/ccisnoxx/sub2api/pull/28) 面向 CI-002 分支，仅 9 个文件处理本任务；没有合并、正式发布、部署或同步 personal。依赖合并后再按普通流程调整 main 目标。原工作区只更新本任务状态和证据，未提交原有暂存内容。
- 每个构建目标生成 SPDX 2.3 二进制 SBOM 与未签名 in-toto Statement v1 / SLSA provenance v1，绑定实际归档/SBOM 哈希、精确源码和工作流 SHA、目标、Release run/attempt、正式成功 CI run 及实际工具/白名单输入摘要。固定官方 Syft 1.54.1、ORAS 1.3.4 的 Linux amd64/macOS arm64 归档 SHA-256，并核对实际版本；BuildKit scanner 固定 index digest。前端 SBOM 登记 lock 中的完整构建依赖，含开发依赖，不宣称全部进入运行时 bundle。
- OCI 原生 SPDX/provenance 保存在完整内容寻址索引中；校验 manifest/config/layer 的实际 digest/size、原生 subject 与运行镜像关联、source/revision/version/created 及完整选定平台。先本地校验，再沿用原 source/CI/版本占用授权门禁；正式发布 ORAS 复制整个索引并按精确远端 digest 拉取再校验，通过后才推进 latest。已有版本不覆盖或重建，保留其原 created，缺证明明确失败。工作流名称、触发器、权限及长期 secrets 保持。
- simple 正式 Release 上传二进制/镜像 SBOM、原生与 pipeline provenance、verification、summary 和 SHA-256 清单，保持无应用二进制附件；说明含 source SHA、成功 CI run 和 Release run。证明附件按 run/attempt 命名，draft 恢复 append 说明；仅重跑失败 Release job 时保留成功 binary job 原生产 attempt，通过逻辑名/公开名/哈希映射运输，不虚构重建。不可覆盖 artifact 按 run/attempt 独立命名，证明保留 90 天；这是写入后不可修改属性，不是永久保存或防删除，正式 Release 附件承担长期分发。
- 最终 SHA 的 [CI 38143726795](https://github.com/ccisnoxx/sub2api/actions/runs/38143726795)、[Security Scan 38143726802](https://github.com/ccisnoxx/sub2api/actions/runs/38143726802) 与 [simple Release dry-run 38143759122](https://github.com/ccisnoxx/sub2api/actions/runs/38143759122) 均 completed/success。CI 五个 job 成功；Release 前端、linux/amd64 二进制与 OCI/publisher job 成功。登录、发布授权、远端发布验证、DockerHub 更新、Telegram 通知及版本同步按 dry-run 跳过。重复 push CI 38143723553 已取消，不计成功；OpenAI recording race 按当前 main 源码明确不适用，不冒称执行。
- 下载并离线核验最终候选的真实二进制、[不可覆盖证明 artifact](https://github.com/ccisnoxx/sub2api/actions/runs/38143759122/artifacts/11703559107)、[OCI tar artifact](https://github.com/ccisnoxx/sub2api/actions/runs/38143759122/artifacts/11703244887) 和 publisher 元数据。二进制 SBOM 1169 packages、镜像 SBOM 184 packages，9 个 Release 证明附件的名称、summary 映射及 checksum 均一致；归档/SBOM subject、source/workflow/run/attempt、OCI blob/label/原生证明/pipeline subject 消费检查全部通过。根索引 digest `sha256:6ef8a131f008ea4d32f750dd2f5ca2315250fc23ed728287cc6b89804eb11f68`，linux/amd64 运行 manifest `sha256:91e45d01df8ad9c9fd1b799b14fbe76acb784f69c92688f45d23684d750d09d1`，UTC created `2026-10-11T13:35:34Z`。dry-run 不需要成功 CI，因此其 summary 的 CI run 为 null；正式 finalize 拒绝缺 CI，正式说明沿用成功 CI 绑定。
- 本地 CPython 3.12.13 / 哈希固定 PyYAML 6.0.3 的 release-tools 50 项测试、actionlint v1.7.12（未执行 ShellCheck/pyflakes）、bash 语法及最终 diff 检查通过。初次真实 Syft 参数错误已修正并用前置真实归档复跑；初始 [dry-run 38142776026](https://github.com/ccisnoxx/sub2api/actions/runs/38142776026) 因 OCI build 未命名导致 BuildKit v0.34.0 空 subject，官方源码确认后增加本地 `--tag`，没有放宽 digest/subject 检查，旧失败不计通过。独立首轮复核确认部分附件上传后重试 HTTP 422 风险，修正 run/attempt 命名与 append；全新上下文最终 critical_reviewer 无未解阻断，独立完成 8 个内存正负向合同检查，未独立重跑云端/本地整套测试。审计 `20261011T131843Z-ci-003-sbom-provenance-e556794b` 已关闭且校验通过，2 次独立复核、初始拒绝 1 次、最终通过 1 次、0 写入、0 活跃残留。
- 四项验收逐项满足发布路径与 dry-run/材料消费证据；未执行真实 GHCR 写入、正式 Release 附件上传、云端部分失败重跑或五目标 full Release，本机无 Docker daemon。未签名证明依赖可信 Actions 控制面、内容寻址和 artifact 运输，不是 GitHub/Sigstore 签名或高等级 SLSA；运行镜像/apk 仍可变化，不承诺逐字节可复现。无数据库影响（none），未改 SQL、迁移、恢复工具或业务数据，未重复恢复演练。风险和回滚已记录 PR/任务文档；未合并时关闭 PR，已合并时普通 revert 本任务两项提交并一起撤销消费者与新工具，保留 CI-001/002 pins 和原授权门禁，禁止 reset/强推或覆盖既有 tag/digest/材料，无需 SQL 回退。只勾选 CI-003，CI-004/005 保持未完成。

---

## CI-004：增加发布控制面与源码 SHA 绑定测试

**优先级：** P1  
**依赖：** `CI-002`  
**建议分支：** `ci/release-guard-tests`

### 目标

防止发布工具、应用源码、成功 CI 和目标标签之间发生错配。

### 实施步骤

- [x] 为 `personal_release.py` 增加负向测试；
- [x] 覆盖错误 SHA、错误 tag、失败 CI、旧 CI、其他仓库 run；
- [x] 覆盖镜像已存在但 revision 不一致；
- [x] 覆盖 release tag 指向移动后的异常；
- [x] 覆盖 dry-run 不需要发布权限；
- [x] 验证发布前和发布中会再次检查源码占用和 CI 状态。

### 验收标准

- 任一绑定关系不一致时发布失败关闭；
- 发布 guard 不依赖应用分支中可修改的脚本；
- 测试不调用真实发布接口或覆盖正式 tag。

### 完成证据（2026-10-11）

- 开始前为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，只有原有 `AM REMEDIATION_PLAN.md`。原分支、HEAD 和暂存内容保留，未提交或覆盖原有台账。CI-004 开始时未完成；前置 CI-002 的固定提交 `bef03cf59d781d053706d0b4c77b7555c4596711`、[草稿 PR #27](https://github.com/ccisnoxx/sub2api/pull/27) 与 [CI 38140571111](https://github.com/ccisnoxx/sub2api/actions/runs/38140571111) 重新确认 success。为保留 CI-003 OCI 运输路径，以已验收提交 `a49438d0eae8ac963a82df17cccd3b40d0e6cd8f` 和 [PR #28](https://github.com/ccisnoxx/sub2api/pull/28) 为基底；最新 main `c1dae6357b4de7cc11d81547013b87550c4ba586` 是其祖先，依赖尚未合并。
- 隔离分支 `codex/ci-004-release-guard-tests` 的最终提交为 `5481d3e7e026c42ea40440aeceb3ae0bd228fbb1`，[草稿 PR #29](https://github.com/ccisnoxx/sub2api/pull/29) 面向 CI-003 分支，仅 10 文件处理 CI-004。五项任务提交范围为 `3b238717e^..5481d3e7e`；原工作区只更新 CI-004 状态和证据。没有合并、正式发布、部署或同步 personal；CI-005 保持未完成。
- 为 personal_release 增加错误/不完整 SHA、错误 tag/上游版本、失败/取消/未完成 CI、旧 SHA 的成功 CI、其他仓库 run、错误 workflow/分支/标题/事件/check suite、错误镜像 revision 和移动 tag 的负向合同。创建 tag、dispatch、远端占用查询后、OCI push/latest 和最终 Release 前重查源码、固定同一 CI run 与 tag；原 R1 失效不能换用同 SHA 的 R2。最终当前 guard、publication 记录和已验证 published-image 摘要必须有同一非空合法 digest，镜像替换/删除停止发布；已存在正确镜像仍按原身份恢复，不覆盖版本。
- 正式 guard、sibling sync 与全部构建/发布控制 helper 从同一次 main checkout 保存、记录 control SHA 并由同次 artifact 运输；应用 helper 替换后，实际运输/执行路径保持主干来源。应用 checkout 后、任何 artifact 下载或 helper 输出写入前清理六个生成入口，消除应用 symlink 将上下文/材料写入引向 main guard 的路径。两个模块安装器、四个内联 JSON 解析器和最终 stdin Python 使用 `-I` 排除应用 CWD、PYTHONPATH 与 user site 的隐式导入。真实 symlink、pip.py/json.py/re.py/pathlib.py 回归先在旧候选实际改写 guard 或执行 payload 后失败，修正后通过。
- 本地 CPython 3.12.13 / 哈希固定 PyYAML 6.0.3 执行 `python -m unittest discover -s .github/release-tools -p 'test_*.py'`：69 项通过。actionlint v1.7.12 的两份发布 workflow 检查（`-shellcheck= -pyflakes=`）、`bash -n .github/release-tools/release-images.sh`、完整任务增量 `git diff --check` 和文档本地链接检查通过；没有执行 ShellCheck/pyflakes。测试阻断未模拟网络，镜像工具为本地桩，pip 只查询版本；没有调用真实发布接口、创建/移动远端正式 tag 或推送镜像。
- 最终 SHA 的 [CI 38149030916](https://github.com/ccisnoxx/sub2api/actions/runs/38149030916)、[Security Scan 38149030952](https://github.com/ccisnoxx/sub2api/actions/runs/38149030952) 和 [simple Release dry-run 38149031098](https://github.com/ccisnoxx/sub2api/actions/runs/38149031098) 均 completed/success。CI 五个 job 成功，含后端单元/集成测试；Release prepare、前端、linux/amd64 binary 和 OCI/publisher job 成功。登录、正式授权、远端发布校验、DockerHub/Telegram 与版本同步按 dry-run 跳过。重复 push CI 38149027918 和 Security 38149027916 已 completed/cancelled，CLA 按既有条件 skipped，均不计通过；初轮旧候选 CI 38145875403 与 dry-run 38145875716 主动取消，不作为最终证据。
- 下载 [证明材料 artifact](https://github.com/ccisnoxx/sub2api/actions/runs/38149031098/artifacts/11705907553) 和 publisher 元数据，离线核对 summary、镜像 pipeline provenance、publication toolchain 的源码与工作流 SHA 均为最终候选；7 项资产哈希与 summary checksum 一致。linux/amd64 OCI root digest `sha256:b2b550389674c6aa4480481ed354f662fc8cbdd97bb6caeef3e98c7242b04ffe`，dry-run 的 CI 字段为 null，正式路径仍要求固定成功 CI。云端可信 helper 完成实际 OCI 内容校验；本次未重复离线解析完整 OCI tar，CI-003 校验器代码未改。
- 五次 fresh 只读 critical_reviewer 复核，前四份候选因五项实证问题拒绝：CI 身份切换、最终 digest 未消费、应用控制 helper 来源、生成目录 symlink 写入、Python 隐式模块导入。逐项修正并增加对应回归后，最终 reviewer 接受固定 SHA，无未解阻断；独立执行 17 项 Personal release 内存测试、8 项最终 stdin digest 正负向内存检查、隔离 pip 版本查询、Bash 语法与增量 diff 检查，通过。未独立重跑 69 项整套测试或 actionlint。审计 `20261011T140216Z-ci-004-release-guard-tests-46b51379` 已关闭并校验通过：5 次独立复核，最终通过 1 次，0 写入、0 活跃残留；历史拒绝均已处理。
- 三项验收逐项满足。正式云端 guard 运输、真实 GHCR/GitHub Release 写入、云端并发/部分失败恢复未执行；dry-run 不替代这些边界。依赖和本任务须先按顺序合入 main，再按规则同步必要控制文件到 personal，候选 PR 不会更新生产 guard；合同依赖可信 workflow/main 审查/artifact 运输，最后检查与跨服务写入间仍有非原子并发窗口。数据库影响 none：没有 SQL、迁移、schema、恢复工具或业务数据变更，未重复恢复演练。风险和回滚已记录 PR/任务文档；未合并时关闭 PR，合并后普通 revert 本任务五项提交并一起撤销 guard 生产者与消费者，保留 CI-001/002 pins 和 CI-003 证明合同；禁止 reset/强推、移动正式 tag 或覆盖既有 digest，无需 SQL 回退。

---

## CI-005：清理工作流权限、凭据持久化和 artifact 保留策略

**优先级：** P1  
**依赖：** `CI-001`  
**建议分支：** `ci/least-privilege`

### 目标

将每个 job 的权限限制到实际需要的最小集合。

### 实施步骤

- [x] 审核所有 workflow 的顶层和 job 级 `permissions`；
- [x] 默认使用 `contents: read`；
- [x] 仅发布 job 获取 `contents: write` 或 `packages: write`；
- [x] 对不需要 Git push 的 checkout 使用 `persist-credentials: false`；
- [x] 检查 fork PR 是否能够接触 secrets；
- [x] 限制敏感 artifact 的保留时间；
- [x] 禁止上传 `.env`、数据库备份或完整运行日志；
- [x] 检查并发组是否可能错误取消正式发布。

### 验收标准

- 每个写权限均有明确用途；
- 普通测试 job 不持有发布权限；
- PR 工作流无法读取生产 secrets；
- artifact 保留时间与用途匹配。


### 完成证据（2026-10-11）

- 开始前为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，工作树仅原有 `AM REMEDIATION_PLAN.md`；原分支、HEAD 和暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。CI-005 当时未完成。前置 CI-001 的固定提交 `1392e4392d3aa039b5510753291ac58d5b35fff4`、[草稿 PR #26](https://github.com/ccisnoxx/sub2api/pull/26) 与 [CI 38137734599](https://github.com/ccisnoxx/sub2api/actions/runs/38137734599) 重新确认 success；依赖已阶段验收，尚未合并。为保留 CI-002—004 控制链，以已验收 CI-004 `5481d3e7e026c42ea40440aeceb3ae0bd228fbb1` 和 [PR #29](https://github.com/ccisnoxx/sub2api/pull/29) 为基准，最新 main `c1dae6357b4de7cc11d81547013b87550c4ba586` 是其祖先。
- 隔离分支 `codex/ci-005-least-privilege` 的任务提交为 `61c62592c8a2f3701e30a85f83bc60a528167d92` 和最终 `bd90291b3e839f75ace44e35cdf2f9e45b692244`，[草稿 PR #30](https://github.com/ccisnoxx/sub2api/pull/30) 面向 CI-004 分支，仅 11 个文件处理 CI-005。原工作区仅更新本任务状态和证据，未提交原台账；未合并、正式发布、部署或同步 personal。任务文档 `docs/ci/CI-005.md` 已提交并逐项记录用途、验证边界与回滚。
- 审核九份 workflow：顶层默认仅 `contents: read`，移除本 fork 跳过的 CLA 写权限和不可达 VERSION 写回 job；Personal CI 的 PR 读权限仅 binding/ready 持有。实际写权限用途限于 Release 发布、Personal release 创建标签/dispatch，以及 main 同步维护的 Git push/草稿 PR/dispatch。普通测试无发布写权限；checkout 全部显式 false，唯一同步 Git push 例外显式 true 并有用途注释。PR 无生产 secrets 引用或 inherit，target PR CLA 在本 fork 跳过且不检出/执行 PR 代码；生产 secrets 仅在非 PR Release publisher 使用，Personal release 保留同仓库 personal 成功 push/手动运行 gate。
- 全部 16 处 artifact 上传保留既有产物路径白名单，关闭 hidden files，排除环境文件、SQL/转储/数据库备份、日志与 .git；runner.temp 使用相同绝对根排除。运输工具/前端/二进制/OCI 1 天、dry-run 报告 3 天、安全摘要 30 天、月度摘要 35 天、发布证明 90 天。Release 正式发布与 dry-run 并发组分离，均 `cancel-in-progress: false`，运行中正式发布不会被并发规则自动取消；同组仍只有一个 pending，新的正式候选可替换旧 pending，不承诺发布过期 personal SHA。
- 本地 CPython 3.12.13 / 哈希固定 PyYAML 6.0.3 执行 `python -m unittest discover -s .github/release-tools -p 'test_*.py'`：78 项通过。新策略首轮在旧工作流产生 6 个预期失败，修正后通过；完整套件初次旧 guard path 单字符串断言失败，改为严格唯一正向可信路径并保留排除规则，先重跑受影响测试再取得全套成功，未删失败测试。actionlint v1.7.12 检查九份 workflow（`-shellcheck= -pyflakes=`）、Bash 语法、任务增量 `git diff --check`、本地文档链接均通过；没有执行 ShellCheck/pyflakes。固定上传 Action 使用的 `@actions/glob` 0.6.1 七类实际目录正负向夹具通过；旧相对排除先真实复现三个临时工具目录的日志/转储遗漏，再改绝对根并新增回归，仅在隔离验证目录使用该库，未增加项目依赖。
- 最终 SHA 的 [CI 38151592962](https://github.com/ccisnoxx/sub2api/actions/runs/38151592962)、[Security Scan 38151590295](https://github.com/ccisnoxx/sub2api/actions/runs/38151590295)、[PR Security Scan 38151592961](https://github.com/ccisnoxx/sub2api/actions/runs/38151592961) 和 [simple Release dry-run 38151626634](https://github.com/ccisnoxx/sub2api/actions/runs/38151626634) 均 completed/success。CI 五个 job 成功，含后端单元/集成、78 项 release-helpers、golangci-lint、shell、前端 typecheck/critical vitest；OpenAI recording 模块不在本源码，该 race 定向检查明确不适用。Release 四个 job 成功，正式登录/授权/发布校验、DockerHub/Telegram 按 dry-run 跳过，GoReleaser 使用 `--skip=publish,announce`。重复 push CI 38151590262 和旧候选 CI/dry-run 已取消，CLA skipped，均不计最终通过。
- 下载实际运输工具核对维护源字节与导入结构，消费 [证明材料 artifact](https://github.com/ccisnoxx/sub2api/actions/runs/38151626634/artifacts/11707335871)：summary、镜像 provenance 和 publisher 元数据的 source/workflow SHA 均为最终候选并绑定本次 run，7 项资产哈希与 summary checksum 一致；OCI root digest `sha256:125324be235f6162708106ada1376a8c2dde5e7a586d87c6bebe83fbbf26d6f1`。dry-run 的 ci_run 为 null，不替代正式授权。云端可信 helper 校验实际 OCI 内容，未重复离线解析完整 OCI tar。API 确认运输/OCI 1 天、报告 3 天、证明 90 天、安全摘要 30 天的实际到期时间；月度 35 天仅配置合同验证。PR 安全产物使用合并测试 SHA，精确最终 SHA 绑定由 push run 的白名单 security-toolchain 验证，不混用两种身份。
- 两次 fresh 只读 critical_reviewer：首轮拒绝旧候选的临时目录相对排除问题，修正并真实复现/回归后，最终 reviewer 接受固定 SHA，无未解阻断；独立执行 12 项定向测试、411 项真实 matcher 内存断言、Bash 语法与增量 diff 通过，没有独立重跑 78 项全套/actionlint/云端发布。审计 `20261011T152202Z-ci-005-least-privilege-23fd0f7f` 已关闭并校验通过：2 次独立复核，最终接受 1 次，0 写入、0 活跃残留；历史拒绝已处理。
- 四项验收均满足，按已提交候选的阶段验收勾选本任务。真实外部 fork PR、正式 GitHub/GHCR 写入、云端并发、月度调度和到期删除未执行；文件名排除不能检测伪装内容或归档内部文件，publisher dry-run job 仍持有正式发布所需写 token，但步骤 gate 禁止发布。依赖须先按序合入 main，再同步必要控制文件到 personal。数据库影响 none：无 SQL、迁移、schema、业务数据、部署或恢复工具变更，未重复恢复演练。风险与回滚已记录 PR；未合并时关闭 PR，合并后普通 revert 本任务两个提交，整体撤销权限/凭据/上传/并发及对应测试文档，保留 CI-001—004 pins/SBOM/main guard 合同，不 reset/强推、移动正式 tag 或覆盖 digest，无需 SQL 回退。

---

## ARCH-001：拆分流式协议解析、计时、完成分类和观测上报职责

**优先级：** P2  
**依赖：** `UPSTREAM-007`  
**建议分支：** `refactor/stream-observation-state-machine`

### 目标

降低 HTTP、SSE 和 WS 入口中计时、状态和观测逻辑交叉导致的回归风险。

### 建议结构

```text
ProtocolParser
OutputTimingTracker
CompletionClassifier
UsageAccumulator
ServiceObservationEmitter
```

### 实施步骤

- [x] 先补齐黑盒回归测试；
- [x] 提取纯状态转换逻辑；
- [x] 将协议事件解析与业务状态更新分离；
- [x] 将完成状态冻结规则集中；
- [x] 将服务状态观测从计费逻辑中分离；
- [x] 保持现有 public DTO 和数据库字段不变；
- [x] 分多个小 PR 完成，不做一次性重写。

### 验收标准

- 行为回归测试与重构前一致；
- 每个状态组件可独立单测；
- 不增加新的 goroutine 泄漏或 data race；
- 取消、完成和错误优先级只有一个权威实现。

### 第一阶段历史推进（2026-10-12；整体状态见第五阶段完成记录）

- 本轮按计划分多个小 PR 推进第一阶段：提取每次 Responses observer 的首次终态分类与冻结，保持 HTTP/SSE/WS 接线、公开 DTO、数据库和计费解析器。协议解析/计时/用量/观测职责进一步拆分及整个任务验收仍待后续阶段，不把一个组件抽取视为整体 ARCH-001 完成。
- 从最新 personal `4cf71d60901e524daf933ba2eebd985d1d378108` 创建隔离分支 `codex/arch-001-stream-observation`；该基础已包含 UPSTREAM-007 与 REL-001 普通合并，继续新的 CI-000 控制。原工作区分支/HEAD/暂存 blob 保留，没有发布或部署。
- 第一批新增可观察回归先在原实现通过：13 顶层/46 pass事件；提取后组件/计时/服务状态34顶层/101 pass事件；目标Race3顶层/15 pass事件；HTTP/WS/计费返回边界10顶层/32 pass事件；handler异步源提交3顶层/5 pass事件，全部无失败。Go1.27.2、GOTOOLCHAIN=local，vet/gofmt/diff通过。不用本地完整应用、安全扫描或发布dry-run代替定向检查。
- 七文件稳定候选 `0b6f7ea0fdca4b2a87f7e1dddaf0b69ab5850ca3` 已提交并只一次推送，[PR #43](https://github.com/ccisnoxx/sub2api/pull/43) 面向personal。fresh独立critical_reviewer对冻结差异无确认新增代码问题；分析/审查审计 `20261012T075115Z-arch-001-stream-observation-first-task-fae8018f` 已关闭并校验，无写入观测、活跃残留或异常。实际[Quick38222159383](https://github.com/ccisnoxx/sub2api/actions/runs/38222159383)应用/文档检查成功，首次应用反馈8分25秒、整轮9分21秒；仅full-ci-certified/personal-ready按预期拒绝缺Full证据，非代码/测试/扫描/基础设施失败。Ready事件唯一[Full38223491602](https://github.com/ccisnoxx/sub2api/actions/runs/38223491602) success，20分23秒，15job全success，真实mode=full且两类扫描启用通过；没有再次dispatch/re-run。PR43于2026-10-12T08:51:55Z普通merge，personal `08c3ebb5ef6b87a08955241c62640f1da531b084`两个父为基础与候选，tree逐字节一致。[合并后38226219264](https://github.com/ccisnoxx/sub2api/actions/runs/38226219264) success，9分30秒；真实mode=quick、evidence_run_id=38223491602，两类扫描关闭，full-ci-certified/personal-ready成功，无第二次Full。此Quick仍重跑整个受影响service包分批单测/vet/后端编译，额外成本保留为后续CI优化依据，不冒称仅绑定冒烟。
- CI-000首项观察记录使用唯一任务ID ARCH-001；后续阶段只补充同一条记录，不重复凑任务数量。

### 第一阶段历史验收（后续职责已由第二至第五阶段完成）

- [x] 第一批边界回归在原实现绿；纯首次终态组件与独立快照单测通过。
- [x] 原mutex/AfterFunc/不同owner/用量/DTO/数据库合同保持，定向入口、异步提交、Race与独立审查通过。
- [x] 精确候选一次Full、普通merge、同tree证据复用及当前personal-ready通过。
- [x] CI-000首项观察记录真实结果和重复Quick包检查成本，未发布或部署。
- [ ] 剩余协议解析、输出计时、用量来源、观测提交职责拆分及整体验收；ARCH-001总体保持未完成。

### 第二阶段候选验收（2026-10-12，新手动流程）

- 下一项可执行任务继续 ARCH-001；REL-002 涉及部署，按维护者本轮要求暂缓，ARCH-002 的部署前置尚未满足。基准为最新 personal `53079f406d6615566adf829a19842553be834ac0`，第一阶段分类器已在其历史中；隔离分支 `codex/arch-001-protocol-events` 从干净工作树实施。
- 实现提交 `478f2ab607ed517e58e05d3b01516a01b0851f28` 抽出无时钟、context、请求 owner 或持久正文的 Responses 本帧事实解析器；原观察器继续在原 mutex 内排序取消、输出计时和首次终态。HTTP/SSE/WS 接线、公开 DTO、数据库字段、accepted usage 及计费 parser 保持。报告/journal 提交 `398db1d77671fbccbda736ccd02273f3bdafd897` 不改变运行实现。
- 新增 3 组黑盒协议边界回归先在原实现通过；抽取后的纯解析、计时、分类器、服务状态源、实际 HTTP/SSE/WS 和计费来源定向检查通过；取消/完成冻结、取消后用量、真实 WS 准入 turn 与迟到重试事实的定向 Race 通过。实际 Go1.27.2，GOTOOLCHAIN=local，默认 vet、gofmt、文档本地链接和精确候选 diff 检查通过。命令、风险、覆盖边界和回退见候选的 `docs/architecture/ARCH-001-02.md` 与 PR。
- fresh critical_reviewer 对固定实现四文件完整差异、原实现和不同入口独立只读复核，未确认 actionable findings 或合入阻断；核对实际命令/exit/stdout但未独立重跑。未穷尽其他 pooled WS/registry/bridge 并发交错，也未重跑未修改的数据库和异步 handler。审计 `20261012T111154Z-arch-001-protocol-events-d3d232bf` 已闭合且 audit-verify 通过，无异常/活跃残留，未观察到复核写入。
- 两个提交只一次推送，建立面向 personal 的[草稿 PR #44](https://github.com/ccisnoxx/sub2api/pull/44)，固定 head 为上述报告提交；交付时查询该 head 的 Actions 运行数为 0，尚未合并。按新流程没有 dispatch Quick/Full，没有全量应用回归、安全扫描、镜像构建或发布 dry-run，没有发布、部署或生产数据库操作。不继续旧十任务观察要求。
- 原工作区 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`、原计划暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留；本节之外的台账保持。数据库影响 none；普通 revert 实现提交可恢复原解析，保留第一阶段分类器/schema/账本，不强推或删除历史。

- [x] 第二阶段：协议事实解析与业务状态更新分离，独立解析单测及基线/入口/Race 回归通过，固定候选已提交并交付审查。
- [x] 第二阶段候选已按维护者后续授权普通合并至 personal。
- [ ] 剩余输出计时、用量来源和观测提交职责拆分及整体 ARCH-001 验收。

### 第二阶段普通合并（2026-10-12）

- 维护者明确授权普通合并 PR44；再次核对 ready 前 head `398db1d77671fbccbda736ccd02273f3bdafd897`、personal base `53079f406d6615566adf829a19842553be834ac0`、候选工作树干净、六文件精确 diff 和第二阶段闭合审计。没有新增实现，复用原定向回归/Race/独立复核证据。
- PR44 ready 后以固定 head 执行普通 merge，没有 admin/force/squash/rebase 或删除分支。实际合并为 `0ff914f35274d6132825a038cc0757b33243f0fc`，parents 依序为上述 base/head，tree `79bb3738139c0e6fbd29ae4a18241b5f6b0d04e0` 与候选完全一致；GitHub 回读 state=MERGED、personal 指向合并 SHA。
- 合并后查询该 SHA 的 Actions 运行数为 0，没有手动 dispatch Quick/Full、发布或部署，没有数据库操作；原工作区分支/HEAD 和计划暂存 blob 保留。第三阶段从这个合并基准继续抽取独立输出计时组件。
- 若撤销这次合并，另建任务分支以第一 parent 为主线普通 revert 合并提交并运行直接相关检查；禁止强推、删除历史或回滚 schema/迁移账本。


### 第三阶段候选验收（2026-10-12，输出计时）

- 从第二阶段实际合并 personal `0ff914f35274d6132825a038cc0757b33243f0fc` 的干净工作树创建 `codex/arch-001-output-timing`，复用已附着工作树；原文档工作区分支/HEAD/计划暂存 blob 保留。
- 实现提交 `7ef9a2fee02b5c261c18daeb0bdd1441f4759c15` 抽出独立输出计时状态，唯一持有既有转发起点及 FirstOutputMs/Kind、StrictFirstTokenMs、LastTokenMs；更新与快照仍在原 observer mutex 内，nullable 字段通过既有 Clone 复制。原毫秒算法、按观察顺序推进、媒体/Token 差别、audioSeen、accepted usage、首次终态/取消及服务状态 owner 保持。公开 DTO、SQL/schema/迁移、计费用量 parser、依赖和控制面均无改动。
- 新增快照隔离/观察顺序黑盒回归在抽取前通过；抽取后的独立组件、计时/分类器、实际 HTTP/SSE/WS、计费与直接相关服务状态源定向检查通过；快照、终态/取消及实际 WS 透传准入 turn 的定向 Race 通过。实际 Go1.27.2/GOTOOLCHAIN=local，默认 vet、正确 gofmt、文档本地文件链接和精确候选 diff 检查通过。首次格式路径前缀错误已纠正；文档按既有忽略规则精确 force-add，没有改忽略配置或纳入无关文件。
- fresh critical_reviewer 对固定实现五文件完整 diff、原实现、Clone/协议/分类器合同、不同入口与 handler 异步计费边界只读复核，未确认 actionable findings 或合入阻断；核对父代理命令/exit/stdout并执行候选 diff --check，未重复测试。Race 未穷尽其他 WS/bridge 并发交错，handler/DTO 只静态追踪，真实数据库本轮未动态执行。审计 `20261012T113245Z-arch-001-output-timing-11e7f19f` 已闭合且 audit-verify 通过，无异常/活跃残留，未观察到复核写入。
- 报告/journal/第二阶段合并记录提交 `bfc2696a96a2ae62fe24cfa79cb923b83575fa42` 仅修改文档，运行证据复用。两提交只一次推送，[草稿 PR #45](https://github.com/ccisnoxx/sub2api/pull/45) 面向 personal、固定 head 为上述文档提交；交付时该 head 的 Actions 运行数为 0，第三阶段暂不合并。完整命令/风险/回退见候选 `docs/architecture/ARCH-001-03.md`。
- 没有手动 dispatch Quick/Full、全量应用回归、安全扫描、前端构建、数据库恢复或发布 dry-run，没有发布、部署或生产操作。数据库影响 none；普通 revert 实现提交可恢复原输出状态，保留已合并的协议解析器、终态分类器和数据库账本，禁止强推或回滚 schema。

- [x] 第三阶段：独立输出计时组件和 nullable 快照，基线/组件/实际入口/Race 回归及独立复核通过，候选已提交并交付审查。
- [x] 第三阶段候选已按维护者后续授权普通合并至 personal。
- [ ] 剩余用量来源、观测提交职责拆分和整体 ARCH-001 验收。

### 第三阶段普通合并（2026-10-12）

- 维护者明确授权普通合并 PR45；再次核对 head `bfc2696a96a2ae62fe24cfa79cb923b83575fa42`、personal base `0ff914f35274d6132825a038cc0757b33243f0fc`、候选干净、八文件精确 diff 和第三阶段闭合审计，复用成功定向/Race/独立复核证据，无新增运行实现。
- ready 后以固定 head 普通 merge，未使用 admin/force/squash/rebase 或删除分支。实际合并为 `96b12ef8a3d0b34607d758da7611d49481e75338`，parents 依序为上述 base/head，tree `ef1e4428762cb40520bc17c94ba77c7388c8a9d2` 与候选一致；GitHub state=MERGED、personal 指向该合并。
- 合并后该 SHA 的 Actions 运行数为 0，没有手动 dispatch Quick/Full、发布、部署或数据库操作；原工作区分支/HEAD 和计划暂存 blob 保留。第四阶段从该基准拆分已接受用量的来源与音频观测状态。
- 若另需撤销合并，以第一 parent 为主线普通 revert 合并提交并执行直接相关检查，禁止强推、删除历史或回滚 schema/账本。


### 第四阶段候选验收（2026-10-12，已接受用量观测）

- 从第三阶段实际合并 personal `96b12ef8a3d0b34607d758da7611d49481e75338` 的干净工作树创建 `codex/arch-001-usage-observation`，复用已附着工作树；原文档工作区分支/HEAD/计划暂存 blob 保留。
- 实现提交 `13395d3651c02b8786f106dec1d13f04d97a4fc8` 提取已接受 raw 用量的纯事实解析、无音频证据检查和独立 UsageSource/AudioOutputTokens/已见音频状态组件。原 HTTP/relay 继续各自决定实际接受来源与全零终态拒绝替换，部分/终态音频拆分、明确零/未知、取消后 drain、原 observer mutex 和 nullable 快照保持。公开 DTO、计费 parser/总量/扣费、SQL/schema/迁移、依赖和控制面不变。
- 新增 accepted raw/部分保留/非对象拒绝/快照隔离/明确零值回归先在原实现通过；抽取后纯解析/状态组件、计时、实际 HTTP/SSE/WS、计费及直接相关服务状态源定向检查通过；已接受用量快照、冻结后 drain 与实际 WS 透传准入 turn 的定向 Race 通过。实际 Go1.27.2/GOTOOLCHAIN=local，默认 vet、gofmt、文档本地文件链接和精确候选 diff 检查通过。
- fresh critical_reviewer 对固定实现六文件完整 diff、原实现、来源/终态/快照/生命周期合同和不同入口独立只读复核，未确认 actionable findings 或合入阻断；核对主代理命令/exit/stdout并执行候选 diff --check，没有重跑测试。Race 未穷尽其他 WS/ingress/bridge 并发交错，handler/持久化只静态追踪，本轮没有重跑 handler/relay 子包或真实数据库持久化。审计 `20261012T115310Z-arch-001-usage-observation-6181f88d` 已闭合且 audit-verify 通过，无异常/活跃残留，未观察到复核写入。
- 报告/journal/第三阶段合并记录提交 `e0b90e900fd1672f8d142607bd9b1e7f86190b4a` 只有文档，运行成功证据复用；两提交只一次推送，[草稿 PR #46](https://github.com/ccisnoxx/sub2api/pull/46) 面向 personal、固定 head 为上述文档提交，交付时该 head 的 Actions 运行数为 0，第四阶段暂不合并。完整命令/风险/回退见候选 `docs/architecture/ARCH-001-04.md`。
- 没有手动 dispatch Quick/Full、全量应用回归、安全扫描、前端构建、数据库恢复或发布 dry-run，没有发布、部署或生产操作。数据库影响 none；普通 revert 实现提交恢复原来源/音频状态，保留已合并的协议解析、输出计时、终态分类器及数据库账本，禁止强推或回滚 schema。

- [x] 第四阶段：已接受用量观测与音频状态组件，基线/组件/实际入口/计费/Race 回归及独立复核通过，候选已提交并交付审查。
- [x] 第四阶段候选已按维护者后续授权普通合并至 personal。
- [x] 第五阶段完整候选整合至 personal 后关闭整体 ARCH-001 台账。 已于本轮完成。

### 第四阶段普通合并（2026-10-12）

- 维护者明确授权普通合并 PR46；ready 前再次核对 head `e0b90e900fd1672f8d142607bd9b1e7f86190b4a`、personal base `96b12ef8a3d0b34607d758da7611d49481e75338`、候选干净、精确 diff 与第四阶段闭合审计，复用成功定向/Race/独立复核证据，没有重复测试。
- ready 后以固定 head 普通 merge，未使用 admin/force/squash/rebase 或删除分支。实际合并 `fd6324f387a3cd204a1ec4d50c033dc349abb5f9`，合并时间 `2026-10-12T12:09:05Z`；parents 依序为上述 base/head，tree `be4d54e4d1a8ee574760bd4ec9933c49e12212eb` 与候选一致。GitHub state=MERGED、personal 指向该合并。
- 合并后该 SHA 的 Actions 运行数为 0，没有手动 dispatch Quick/Full、发布、部署或数据库操作；原工作区分支/HEAD 与计划原暂存 blob 保留。第五阶段从这个固定基准继续分离服务状态观测上报。
- 若另需撤销合并，以第一 parent 为主线普通 revert 合并提交并执行直接相关检查；禁止强推、删除历史或回滚 schema/账本。

### 第五阶段候选验收（2026-10-12，观测上报与结构闭合）

- 从第四阶段实际合并 personal `fd6324f387a3cd204a1ec4d50c033dc349abb5f9` 的干净工作树创建 `codex/arch-001-observation-emitter`，复用已附着工作树。原文档工作区 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca` 与计划暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留，未纳入任务 PR。
- 实现 `08841d75ebaf6171ca1090f586e8ce811174c8ea` 提取独立 responsesServiceObservationEmitter，唯一持有 source、attempt 和观测。首次终态仍只由分类器冻结，组件只接收成功冻结事实；unknown/terminal_missing 不冻结分类器，后续真实终态可以提升，重复快照时点和 Clone 隔离保持。请求最终 owner、迟到 attempt 过滤、原锁序/取消/stop、handler 异步 Commit/队列交付均保持。计时观察器从四组件独立副本组合完整 DTO，不再保存可变 UsageTiming；版本 0/1、nullable 和 JSON 隐藏合同保持。
- 五职责——协议事实、输出时点、首次终态、已接受用量/音频、观测候选——已有单一 owner 和独立单测入口，前四组件未修改，复用各阶段成功证据。观察器保留原生命周期、锁和计费用量接受编排。公开 DTO、计费 parser/总量/扣费、SQL/schema/迁移身份、依赖、权限、配置和控制面无改动。
- 新增缺失事实→真实终态/快照隔离回归在原实现通过。首轮仅因夹具 Local/UTC Location 的反射比较失败，规范夹具 UTC 并保留严格断言后原实现通过；没有修改业务实现规避失败。抽取后独立组件、观察器、实际 HTTP/SSE/WS、费用与服务状态源检查通过；service 的快照/终态/旧 attempt/实际 WS turn 定向 Race 和 handler 的真实 HTTP 重试/异步交付定向 Race 通过。实际 Go1.27.2/GOTOOLCHAIN=local，默认 vet、gofmt、14 个本地文档链接及最终精确候选 diff 检查通过，完整命令/exit/耗时见候选 `docs/architecture/ARCH-001-05.md`。
- fresh critical_reviewer 以隔离上下文对固定实现四文件完整 diff、原实现、前四组件、不同 HTTP/SSE/WS 接线和 handler Clone→Commit→队列边界独立只读复核，未确认 actionable findings 或合入阻断；确认五职责完整候选、两个冻结 owner、unknown 提升、旧 attempt、锁序/取消/stop 和全部 DTO 字段/版本。核对主代理命令/exit/stdout并独立执行 diff --check，没有重复测试。审计 `20261012T122119Z-arch-001-observation-emitter-8e6782fc` 已闭合且 audit-verify 通过，1 次执行已验收、1 次独立复核，无异常或活跃残留，未观察到复核写入。
- 后续三份文档提交 `f03ef4712690ee76f637facb8116aff66c6da342` 不改变已验证运行代码，成功证据复用。两提交只一次推送，[草稿 PR #47](https://github.com/ccisnoxx/sub2api/pull/47) 面向 personal；GitHub 回读 OPEN/draft、精确 base/head 一致，交付时该 head 的 Actions 运行数为 0。第五阶段未合并，整体 ARCH-001 等待此完整候选整合后关闭，实施步骤按已完成结构及候选验收勾选，不声明总任务或生产验收完成。
- Race 不穷尽所有 WS/取消/重试交错；真实数据库持久化和生产运行未验证。没有手动 dispatch Quick/Full、应用全量、安全扫描、前端构建、数据库恢复或发布 dry-run，没有发布/部署。数据库影响 none；普通 revert 实现提交恢复原观测状态，保留前四组件和数据库账本，文档可独立回退；禁止强推或回滚 schema。

- [x] 第五阶段：独立观测上报与快照，基线/组件/实际入口/费用和 service/handler 定向 Race 通过，固定实现独立复核及审计闭合通过。
- [x] 五职责结构与必要定向验收已形成完整可审查候选，报告和草稿 PR 已交付。
- [x] 第五阶段候选普通整合至 personal 后关闭 ARCH-001 总台账。 已于本轮完成。

### 整体完成与第五阶段普通合并（2026-10-12）

- 维护者明确授权普通合并 PR47 并关闭 ARCH-001。ready 前核对精确 head `f03ef4712690ee76f637facb8116aff66c6da342`、base `fd6324f387a3cd204a1ec4d50c033dc349abb5f9`、干净候选、diff --check 与第五阶段闭合审计校验通过；复用未改变运行输入的基线/组件/实际入口/费用、service/handler Race 和独立复核成功证据。
- PR47 于 `2026-10-12T12:44:04Z` 普通 merge 为 `2867ad10d46d7b774a2e8c2b3406f9e6ad75f0e0`，parents 依序为上述 base/head，tree `93a0aef7ee09814ec17f239be2551edbc0db8040` 与候选一致。GitHub 回读 MERGED，personal 指向该 SHA；未使用 admin/force/squash/rebase 或删除分支，合并 SHA 的 Actions 运行数为 0。
- 四项整体验收满足：新增黑盒回归在抽取前通过、抽取后行为一致；五个状态/事实组件各有独立单测；没有新增 goroutine 或锁，原生命周期和定向 Race/独立复核覆盖相关风险；首次终态冻结集中在分类器，请求最终提交仍有原 owner。候选已依序通过五个小 PR 整合，ARCH-001 实施、候选验收和整合完成，关闭总台账。历史阶段的待后续状态由本记录替代，保留历史不删。
- 本轮合并不重跑全量/Quick/Full/扫描，不发布或部署。Race 不穷尽所有交错，真实数据库/生产验收不属于本次结构治理完成结论。数据库影响 none；需撤销本次合并时以第一 parent 普通 revert 并执行直接相关检查，保留历史/schema/迁移账本。原工作区分支/HEAD/计划暂存 blob 保留。
- 下一可执行未完成任务是 ARCH-004（依赖 UPSTREAM-007 已满足）；ARCH-002 等待 REL-002，REL-002/003 涉及本轮明确排除的发布部署，ARCH-003 已完成。不扩大到 ARCH-005。

---

## ARCH-002：拆分主机部署脚本和环境配置

**优先级：** P2  
**依赖：** `REL-002` 之后  
**建议分支：** `refactor/deploy-host-modules`

### 目标

降低 `deploy_hostdzire.py` 同时承担连接、迁移、镜像、备份、回滚和验证带来的维护风险。

### 实施步骤

- [x] 提取配置模型并进行 schema 校验；
- [x] 提取远程命令执行模块；
- [x] 提取镜像验证模块；
- [x] 提取数据库备份与迁移模块；
- [x] 提取部署后健康检查模块；
- [x] 保持 dry-run 和失败关闭；
- [x] 将主机特定值移出源码；
- [x] 增加无真实服务器依赖的单元测试。

### 验收标准

- 主入口只负责流程编排；
- 每个模块具有明确输入输出；
- 配置缺失或未知字段会失败；
- 日志不会输出密钥或连接串；
- 现有部署测试全部通过。

### 完成证据（2026-10-12）

- REL-002/003 实际完成后按顺序实施；main控制代码 `b1f9fb1332db8b705716ebaf0bcb134f5cbb74f5` 普通快进，personal控制文件同步 `3ab7181195cd49ac4e15ee4df7c7b8c0ef82138f`，保留正式应用429/tag/Full/digest和原工作区改动。
- 入口106行只解析参数、导入前信任及编排，13责任模块与完整无凭据hostdzire.json模型明确输入输出；严格字段/type/path/现场关系失败，Host专用值移入配置；唯一Deployment锁/record/selection/部分失败owner保持。完整bundle绑定当前已获取origin/main完整SHA，所有模块/config导入前检查；原schemas和兼容证明不迁移。
- 原81+新14合同成功：本地95唯一成功由定向及未变成功证据组合，不声称单次95全绿；唯一必要Linux [工具CI38304959563](https://github.com/ccisnoxx/sub2api/actions/runs/38304959563) 绑定最终代码SHA，单次Ran95/17.589s/OK，job与cleanup全成功。没有新应用Full/扫描/构建/发布。
- 两次fresh只读critical复核：首次确认唯一TMPDIR持久scratch P2，最小dir=/tmp修复后真实瞬时观察回归红1→绿1、直接10项通过，新fresh复核确认闭合无新阻断。暂存新文件完整检查追加清理9文件空白，完整AST相同，最终cached diff检查通过，既有行为证据复用。
- 实际生产只读dry-run绑定b1f9fb、固定FA镜像，exit0/status=dry-run/will_apply=false。独立前后快照相等：3容器ID/Image/启动/健康，Compose/.env/selection/Nginx及全部部署/维护记录，308账本完整身份+applied_at均保持；/tmp无残留。Python自身不读取.env原始字节，独立快照只记摘要。
- 源码/迁移/许可证/NOTICE/来源与正式应用429无变化；线上83维护工具/open状态不替换，旧镜像回退禁止及真实备份恢复边界保持。工具新线上失败恢复/断电未动态触发，95合同与REL-002既有现场证据承担对应验证。第8节3项全计划缺口保持待办。
- [完整验收与回滚记录](docs/upstream/records/2026-10-12-arch-002.md)已同步两分支。全任务审计因历史config快照变化无法生成单一已验证digest，保留历史与缺口；不伪造计数、不改用户配置。

---

## ARCH-003：修复模型目录中的多渠道绑定歧义

**优先级：** P1  
**依赖：** `UPSTREAM-003`  
**建议分支：** `fix/catalog-channel-ambiguity`

### 目标

避免同一授权组关联多个活跃目录渠道时，map 覆盖顺序决定展示结果。

### 实施步骤

- [x] 明确业务合同：一组一个目录渠道，或允许多个 offer；
- [x] 若合同要求唯一，检测重复绑定并返回可诊断错误；
- 不适用：若允许多个渠道，调整 DTO 和 offer key，完整展示多个来源；本轮沿用既有一组一个渠道合同，不实施多 offer 方案。
- [x] 增加数据库或服务层约束；
- [x] 增加顺序无关测试；
- [x] 不以“最后一次写入胜出”作为隐式规则。

### 验收标准

- 相同数据无论查询顺序如何结果一致；
- 异常配置可被管理员发现；
- 不会把错误渠道价格展示给用户。


### 完成证据（2026-10-11）

- 本任务此前未勾选，但技术实现与顺序无关回归已在 UPSTREAM-003 完成，本轮跳过重复实现，仅核验三项标准并补齐台账。采用既有“一组最多一个渠道”合同；多 offer 条件分支不适用，不调整 DTO、offer key 或计费规则。
- 开始前为 `codex/doc-002-install-sources`，HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，工作树仅原有 `AM REMEDIATION_PLAN.md`；原分支、HEAD 和暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。重新 fetch 后以实际 personal `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 建立隔离验证分支 `codex/arch-003-catalog-ambiguity`，该工作树相对基准无文件变更、无新提交。
- 前置 UPSTREAM-003 候选 `43630d16feb9a45fa4cfa92f66e5782dd0213ad7` / [PR #22](https://github.com/ccisnoxx/sub2api/pull/22) 与同 SHA [CI 38129465410](https://github.com/ccisnoxx/sub2api/actions/runs/38129465410) 重新确认 success。其目录实现已随 [普通合并 PR #25](https://github.com/ccisnoxx/sub2api/pull/25) 进入当前 personal；当前目录快照、目录 builder、目录测试和 081 迁移四个相关文件与 #22 固定候选逐字一致。候选提交与实际 personal 的 Git 身份分别记录，不将 #22 的完整提交误称为当前 personal 的祖先。实现与风险/回滚已有提交和前置 PR 记录，不新建只有重复实现的任务 PR。
- `081_create_channels.sql` 的 `idx_channel_groups_group_id` 唯一索引限制每组最多一个渠道，后续迁移未撤销该约束。`ChannelService.Create/Update` 通过 `checkGroupConflicts` 拒绝其他渠道已占用的组，管理员 API 返回 HTTP 409 / `GROUP_ALREADY_IN_CHANNEL`；沿用现有服务和数据库约束，未新增或重写迁移。
- `ListCatalogChannels` 在授权活跃组与活跃渠道快照中发现不同渠道重复绑定即返回 HTTP 500 / `CATALOG_CHANNEL_BINDING_CONFLICT`。`listCatalog` 在读取个人倍率和构建 DTO 前停止，无部分目录或错误渠道价格；未授权/停用配置不影响当前用户。builder 的 group map 只消费已通过唯一绑定检查的快照，重复配置不采用最后一次写入胜出。
- `ApplicationError.WithCause` 保留冲突分组和两渠道身份，`response.ErrorFrom` 将其写入服务器脱敏错误日志；用户 JSON 仅保留安全 reason/message，不携带 cause 或内部 ID。本轮测试日志实际确认正序、逆序及共享渠道冲突身份均可供管理员定位；既有 HTTP 断言确认用户响应没有内部渠道标识、部分 data，且冲突时不读取个人倍率。
- Go 1.27.2 执行 `go test -count=1 -tags=unit -json -run '^(TestAvailableModelCatalog_.*|Test(Create|Update)_GroupConflict)$' ./internal/handler ./internal/service`：退出 0，18 个顶层测试、含子用例 39 个通过事件，0 失败、0 跳过。覆盖重复绑定正逆序、每个授权绑定、无关/停用冲突、授权隔离、无部分报价、旧接口及创建/更新冲突。没有新增重复测试。初次附加日志解析误用 fixture 渠道 ID 77，按真实 fixture 731 修正后核验通过；Go 测试本身始终通过，未重复执行。
- 当前精确 personal SHA 的 [Personal CI 38135489900](https://github.com/ccisnoxx/sub2api/actions/runs/38135489900) completed/success，运行代码、测试、依赖及配置未变，复用已成功的全量门禁证据；本轮没有重跑完整后端/前端、安全扫描、Release dry-run 或数据库恢复演练。既有独立复核见 UPSTREAM-003 / UPSTREAM-007 的 PR 记录；本轮没有行为或高风险边界变更，不冒用为新增独立复核。
- 三项验收均满足：重复绑定的正逆查询返回同一安全错误，异常配置可由管理员写入错误/服务器日志发现，冲突目录不会输出任一渠道价格。未在生产制造重复绑定、读取生产数据库或执行付费上游请求。数据库影响 none：无 SQL、schema、迁移身份、业务数据、部署或代码变更。回滚本轮仅撤销 ARCH-003 的台账状态与证据，保留已验收目录实现、唯一约束及其他任务记录；应用功能回退属于原 UPSTREAM-003 的独立回滚范围。

---

## ARCH-004：为个人功能建立独立 feature flag 和降级路径

**优先级：** P2  
**依赖：** `UPSTREAM-007`  
**建议分支：** `feat/personal-feature-flags`

### 目标

允许在不回退整个应用的情况下关闭目录、诊断或服务状态等个人功能。

### 实施步骤

- [x] 为模型目录、路由诊断、服务状态分别定义开关；
- [x] 明确默认值；
- [x] 开关关闭时不运行后台聚合任务；
- [x] 开关关闭时接口返回明确状态，不伪装成无数据；
- [x] 增加配置变更审计；
- [x] 保持数据库 schema 可兼容，不做动态删表。

### 验收标准

- 每项功能可独立关闭；
- 关闭后不影响网关核心转发和计费；
- 开关状态可被运维检查；
- 重新启用不会错误复用旧确认计数。

### 候选完成证据（2026-10-12，新手动流程）

- 下一可执行任务选择 ARCH-004：依赖 UPSTREAM-007 已满足；ARCH-002 等待 REL-002，本轮明确暂不发布部署，ARCH-003 已完成，ARCH-005 不扩展。本轮此前已普通合并 PR47、关闭 ARCH-001，合并基准 `2867ad10d46d7b774a2e8c2b3406f9e6ad75f0e0`，创建干净隔离 `codex/arch-004-personal-feature-flags`，复用已附着工作树。
- 固定运行实现 `3303ff1bc27ffa69d292a281ea820cfabc189412`：目录复用 available_channels_enabled（默认 false），认证后 catalog 明确返回 MODEL_CATALOG_DISABLED / MODEL_CATALOG_UNAVAILABLE，旧数组合同保持，禁用不读授权组/渠道/价格/倍率，重开使用当前价格。服务状态复用独立 enabled（默认 false），原45秒预算内每轮GetConfig，关闭/无平台/读失败跳过Aggregate，保留minute ticker发现重开；关闭期间不执行周期TTL清理，重开首轮补做，原取消和Stop保持。
- 路由诊断新增既有settings key routing_diagnostics_enabled，缺少默认true，非法值显式错误，冷加载实际失败只关闭诊断；复用OpsService原atomic不可变快照、发布/刷新锁和约30秒+jitter跨实例刷新。成功持久化后才发布，失败保留原有效值，其它Ops更新保留诊断字段；enabled为持久配置、effective_enabled为本实例采集快照，独立于Ops监控配置。
- 新管理员GET/PUT /api/v1/admin/ops/routing-diagnostics/config 继承原认证/限流/审计，关闭诊断或Ops监控后仍可修复/重开。新请求不建诊断owner/builder，WS/owner/transport/终检/nil builder 保留真实选择、错误sentinel、等待方案和槽位释放。最终RecordError/Batch重检后裁剪关闭前队列的顶层/嵌套诊断，保留普通错误和服务状态observation。管理单记录详情副本用同次状态裁剪历史、明确状态，列表/用户白名单隐藏配置元数据；关闭后的历史裁剪格式错误显式失败，启用时保留原normalization兼容。
- 四项验收有对应证据：三个功能可独立关闭且有明确默认/状态；真实advanced/legacy选择、transport/reject/槽位和普通错误/服务状态事实保持；管理配置接口与既有设置/服务状态入口可供运维检查并记录变更审计；原UpdateConfig事务保持version递增、count立即清零、事件awaiting_data冻结、新epoch与coverage间隙，归一化/事件继续要求新evidence，不复用旧确认计数。本轮没有修改这些持久化owner、SQL/schema/迁移身份/checksum、计费或来源规则。
- 必要定向检查通过：目录全组/追加当前价格回归和错误cause短路单项；服务状态单轮关开/失败/取消预算及handler配置/固定错误；确认计数和coverage三项仓储纯状态；路由诊断/真实选择/Ops错误/上下文与handler/管理详情unit；service/handler/admin定向Race；正式注册+真实JWT+审计队列的新路由检查（签名/撤销/普通用户拒绝、关开、保存失败与四条PUT身份/状态）。实际Go1.27.2/GOTOOLCHAIN=local、默认vet、gofmt、精确diff检查及15个本地文档链接通过。完整实际命令/exit/耗时和输入未变证据复用见候选 docs/architecture/ARCH-004.md。
- 失败均先诊断并按失败检查优先处理：并行未完成接口的仓储编译时点错误待候选稳定后通过；新JWT夹具误将诊断effective与Ops监控AND，按独立开关合同修正后单项通过；既有admin回归发现新增详情metadata进入批量列表，原裁剪点补nil后直接相关范围通过；构造参数/JSON隐藏字段错误测试断言已修正，没有删失败检查或新增业务fallback。
- fresh critical_reviewer 对固定16文件完整diff、新测试、原台账和直接调用/公开/权限/并发/持久化合同独立只读复核，未确认 actionable findings 或交付阻断，四项验收均有支持；独立diff --check通过，没有重复成功测试。原stdout/exit仅保存在线程工具返回，采用主代理/实施者工具证据，不冒称独立导出日志。审计 `20261012T124459Z-arch-004-personal-feature-flags-b0495878` 已闭合且audit-verify通过：3计划/3执行验收通过、1独立复核，无异常、未执行ready task或活跃残留，8个实施写路径符合所有权，分析/复核未观察到写入。
- 文档提交 `f7aafd4dcda39ee6cfa99ad16c71baaffd44fd7c` 只补三份报告/journal，运行输入保持。两提交只一次推送，[草稿 PR #48](https://github.com/ccisnoxx/sub2api/pull/48) 面向personal，回读OPEN/draft、精确base/head一致、mergeState=CLEAN，最终head的Actions运行数为0。按已提交/验证/审查/交付的源码候选验收勾选ARCH-004；尚未合并，不声明上线或生产验收。原工作区分支 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca` 与计划暂存blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留，未提交原计划。
- 真实PG/Redis关开、锁等待/事务提交和进程重启恢复本轮未动态验证：默认与已配置Colima两个Docker sockets均不存在，未把集成harness的exit0跳过计为通过。原仓储事务未修改，纯状态/调度与静态owner复核支持源码候选验收。跨实例最终收敛、旧请求/WS入口快照、已越过最终检查的数据库写入不被取消，以及Race未穷尽所有交错均已记录；前端仅静态核对原错误显示，无浏览器矩阵或完整计费端到端。
- 数据库schema/迁移影响none，仅既有settings新增配置key，保留历史/schema/迁移账本，不动态删表，没有生产操作。没有dispatch Quick/Full、应用全量、安全扫描、前端build、数据库恢复或发布dry-run，没有发布/部署。普通 `git revert 3303ff1bc27ffa69d292a281ea820cfabc189412` 并运行直接相关检查恢复旧行为，保留ARCH-001五组件和历史；新key可保留、旧版本忽略它，文档单独回退，禁止强推/删历史/回滚schema。

### 普通合并记录（2026-10-12）

- 维护者本轮明确授权普通合并 PR48；ready 前确认固定 head `f7aafd4dcda39ee6cfa99ad16c71baaffd44fd7c`、personal base `2867ad10d46d7b774a2e8c2b3406f9e6ad75f0e0`、候选工作树干净、精确diff和闭合审计校验通过。没有新增运行实现，复用输入未变的定向unit/真实选路/JWT/审计/Race与独立复核成功证据，原真实DB覆盖缺口保持。
- PR48 于 `2026-10-12T13:35:08Z` 普通 merge 为 `bea1db12d5f4853c0b85b44840e15de6a74309b1`；parents 依序为上述 base/head，tree `a9e068591e1a7037378d9e9cc558a398b27c0e72` 与候选完全一致。GitHub回读MERGED，personal指向该SHA，未使用admin/force/squash/rebase或删除分支。
- 合并SHA的Actions运行数为0，未dispatch Quick/Full、扫描、发布或部署。原工作区分支/HEAD/计划暂存blob保留。若撤销，以第一parent普通revert本合并并执行直接相关检查，保留历史/schema/迁移账本；不得强推或删除历史。
- 下一可执行未完成任务为ARCH-005（PREP-003/UPSTREAM-007已满足）；维护文档从最新main `ab86413aa27848239d0374adeff9f7aeaa41609e` 创建隔离 `codex/arch-005-upstream-maintenance`，固定实际应用以本次personal合并SHA核对，不用main同名应用树冒充。ARCH-002仍等待REL-002，REL-002/003暂不执行。

---

## ARCH-005：建立长期上游来源矩阵与变更记录

**优先级：** P1  
**依赖：** `PREP-003`、`UPSTREAM-007`  
**建议分支：** `docs/upstream-maintenance`

### 目标

降低后续跟进 KIN 或 Plus 时的人工判断成本。

### 实施步骤

- [x] 维护 `UPSTREAM.md`；
- [x] 每次发布记录 KIN tag、KIN SHA、个人 SHA 和 Plus 参考版本；
- [x] 每个个人模块记录 owner 文件和测试文件；
- [x] 每次上游迁移记录冲突文件、语义变化和处理方式；
- [x] 明确哪些 Plus 功能已停止同步，仅保留当前实现；
- [x] 对已被 KIN 原生吸收的功能记录删除或收敛计划。

### 验收标准

- 新维护者可以根据文档完成下一次重放迁移；
- 不需要依赖某个本地工作树或个人记忆理解来源；
- 文档与 `personal-source.json`、NOTICE 和 Release notes 一致。

### 完成记录（2026-10-12）

- PR #48 已按新流程普通合并后，执行下一可执行 ARCH-005；PREP-003/UPSTREAM-007 已满足，ARCH-002 仍等 REL-002，REL-002/003 暂缓。维护分支 `codex/arch-005-upstream-maintenance` 从 main `ab86413aa27848239d0374adeff9f7aeaa41609e` 开始；应用事实读取固定 personal `bea1db12d5f4853c0b85b44840e15de6a74309b1`，不以 main 同名旧应用源码代替。
- 实质文档 `c54b9497d4bee04215384deb494c5467b917a7db` 更新 UPSTREAM/NOTICE/维护合同与长期记录：I/N/A/K/P/C 来源、S1–S5/ARCH-001/004 真实 owner/test、Plus 冻结停止追随、KIN 已有能力复用；保留 PREP-003 历史正文。三次历史 Release 记录各自应用 SHA/KIN/Plus 适用范围/Notes digest/run；新 KIN 迁移保存分叉、文件语义交点、真实 journal 文本冲突、A/R 双亲连接与回退/下次重放步骤。历史 Notes 未列出的来源注明同 SHA 源码重建，旧 Release 未被外部改写。
- 定向核对通过 32 本地链接、24 固定 Git 链接、121 owner/test/迁移/控制路径、三次 Release 字段与来源三字段、tag/read SHA、祖先/parent/tree 关系，以及 diffcheck/历史保留/八文件范围检查。初检 A 没有 UPSTREAM.md、AccountTableFilters 路径漏 admin 已按实际对象纠正。仅核对测试入口存在，未运行应用/工具 unit/race/integration/build/E2E、Quick/Full、安全扫描、dry-run 或 registry 材料。
- fresh critical_reviewer 对固定候选完成发布资格/迁移兼容/许可来源与固定 C 实施的一致性复核，无确认阻断；未独立执行动态/数据库/产物/生产验收。普通 reviewer 只返回 ROLE_MISMATCH，未计验收通过并转交。审计 `20261012T133859Z-arch-005-upstream-maintenance-42e7f252` closed/audit-verify passed（20产物），尝试3/验收2，保留角色异常；无未执行 ready、残留活跃或观察到的子代理写入。
- 证据追加 `4d67e6b52b6d81c748536c0f6ca3a319135a5949` 仅报告/journal，实质来源/流程与应用输入未变，复用成功证据。两提交一次普通 push，[草稿 PR #49](https://github.com/ccisnoxx/sub2api/pull/49) 面向 main；回读 OPEN/draft、base/head 精确一致、mergeState=CLEAN、head Actions 0 次。按已提交/验证/独立复核/交付的文档候选关闭 ARCH-005，PR 尚未合并；完整证据为候选 docs/architecture/ARCH-005.md。
- 真实后续应用缺口已登记：I 更新 service/版本徽章及测试与 N 同 blob，旧 A 的 personal Release/container/manual 拒绝原地更新和五段 personal 解析未迁回，本任务不声称已修复。当前 RC `0.2.15-klno.2-tps.1-rc.1` 未正式发布；旧 A 能力或旧 Full/dry-run 不作 I 当前资格，没有证据删除 S1–S4/schema。未新增 tag/digest、发布、部署或生产操作。
- 仅八份文档/NOTICE，应用、workflow、来源 manifest、SQL/数据库/迁移、依赖和部署工具未改。原根分支 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`、暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留；本节外台账逐字保留，原计划未暂存或提交。

### 普通合并记录（2026-10-12）

- 维护者本轮明确授权 PR #49 普通合并；固定 head `4d67e6b52b6d81c748536c0f6ca3a319135a5949`、main base `ab86413aa27848239d0374adeff9f7aeaa41609e` 和八文件文档/NOTICE 差异未变。既有定向来源/链接/路径证据与 fresh critical_reviewer 结论复用；审计 `20261012T133859Z-arch-005-upstream-maintenance-42e7f252` 本轮 audit-verify passed，原普通 reviewer 角色转交异常继续保留。
- PR Ready 后以精确 match-head 普通 merge，无 admin、强推或 squash/rebase。GitHub 回读 MERGED，合并时间 `2026-10-12T14:31:34Z`，实际 main `d5d1caa6c4df940b058930c7963fc20b2736860f`；双亲顺序为上述 base/head，tree `c47d90fba70025ff694bb63812e87ea1e32cc0d6` 与候选相同。personal 仍为 `bea1db12d5f4853c0b85b44840e15de6a74309b1`，未合入 main 应用树。
- 本轮仅补合并前 diffcheck、实际 merge parent/tree、分支/工作树和 Actions 状态核对；合并 SHA 查询 Actions 0 次。没有重跑输入未变的文档计数或应用/工具测试，未执行 Quick/Full、安全扫描、dry-run、发布、部署或生产操作。
- 复核总体未完成任务及完整依赖：仅剩 ARCH-002、REL-002、REL-003。REL-002 首步部署测试环境并继续生产发布，与本轮“暂不发布或部署”冲突；ARCH-002 明确依赖 REL-002 之后，REL-003 同样依赖 REL-002。因此当前约束下没有下一项可执行的台账任务，三项保持未完成，不越过依赖或以发布准备冒充任务完成。个人更新 service/版本徽章的迁回缺口继续保留为后续修复方向，本轮未自行新增应用任务。
- 原根分支 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca` 与计划暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 均保留；只追加本节证据，本节外台账逐字不变，原计划未暂存/提交，附着候选工作树仍干净。

---

## OBS-001：增加服务状态事实表容量、清理和聚合耗时监控

**优先级：** P1  
**依赖：** `UPSTREAM-005`  
**建议分支：** `ops/service-status-capacity`

### 目标

验证“分钟 × 平台 × 组 × 模型”的数据基数在实际环境中可控。

### 实施步骤

- [x] 记录事实表每日新增行数；
- [x] 记录主表和索引大小；
- [x] 记录聚合任务耗时、扫描行数和失败次数；
- [x] 记录清理任务删除行数和耗时；
- [x] 监控 autovacuum、死元组和长事务；
- [x] 为聚合超时、连续失败和表增长异常设置告警；
- [x] 使用真实数据评估 31 天保留期；
- [x] 评估分区、降采样或保留期配置需求；当前实测基数及 31 天负载支持继续使用原保留策略。

### 验收标准

- 聚合任务通常在一个统计周期内完成；
- 表增长和索引增长有可观测指标；
- 清理失败不会静默发生；
- 保留策略有实际容量依据。

### 完成证据（2026-10-11）

- 开始时为 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；原分支、HEAD 与暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。前置 UPSTREAM-005 候选 `39753447cb4287c1a0aeee68b79b589ff345498a` 为已合并 personal 的祖先；本次从最新 `origin/personal` `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 创建隔离 `codex/obs-001-status-capacity`。
- 两个本任务提交 `92dc67c483ab5ad103111bdbac52396bc584e92d`、最终 `ce0b04999764133580afc6b029a818db0550a15f`；[草稿 PR #31](https://github.com/ccisnoxx/sub2api/pull/31) 面向 personal，仅七个实现/测试/报告文件。完整说明 `docs/observability/OBS-001.md`；每轮聚合耗时/阶段/源扫描返回行数/失败计数、确认提交后的清理删除数和耗时、每小时有界只读容量/索引/维护采样、超时/连续失败/增长告警码均已接线。跳锁不报告成功，COMMIT 错误保留最终结果未确认语义，容量采样失败不改变业务事务。
- 维护者明确授权通过 `hostdzire` 使用真实环境；2026-10-11 16:11:59 UTC 在真实 PostgreSQL 18.6 只读采样得到 916 条事实、主表 180224 字节、索引 73728 字节、总计 253952 字节；两段部分 UTC 日分别149/767行，平均 tuple112.92字节，每分钟峰值3范围，死元组估计59、autovacuum65次、过期事实与长事务均0。相同源 SQL 的只读 EXPLAIN 返回14行/2.045ms。没有读取请求内容或凭据、修改配置、执行生产清理或部署。
- 真实样本约27小时55分钟，不冒用为31天长期负载；按已观察三范围/分钟持续全天外推31天133920行、约35.41MiB，隔离真实PG生成133890行后最终完整聚合含首轮采样15ms，主表15704064/索引10510336字节。这为当前规模的31天保留提供初始实际依据；没有证据要求新增分区、降采样或保留期配置。
- 初始定向unit71项、原仓储/normalization/监控真实PG集成21顶层及负载单项通过；权限修正后监控集成9项（真实NOINHERIT角色、锁超时、删除后失败回滚、重算日行数和31天负载）、监控/调度race5项、go vet与make build通过。最终固定SHA [Personal CI38155206774](https://github.com/ccisnoxx/sub2api/actions/runs/38155206774)11个作业全部success，实际完整unit/integration/recording race、lint、安全、前端/严格build、部署脚本、发布工具、绑定及personal-ready通过。重复push CI38155203416/Security Scan38155203486 cancelled，CLA skipped，均不计通过。
- 首轮只读reviewer发现MEMBER不能代表未继承统计角色当前可用权限；改USAGE并用真实角色验证activity_complete=false。fresh最终reviewer覆盖固定候选七文件、直接合同、提交/回滚/采样/隐私及保存动态证据，无确认新增问题；未独立重跑测试或连接生产。COMMIT真实断连与专项并发采样未动态重现；新监控未生产运行、长期p95/p99及外部告警投递未验证。执行审计 `20261011T161338Z-obs-001-status-capacity-06c93d0c` 已关闭、校验通过，无异常。
- 无DDL、迁移、账本、依赖、前端、API或发布控制面变更；原45秒事务预算和31天TTL保留。PR记录短样本/日志级别/受限统计/小时重试风险、数据库影响及撤销两个任务提交并保留schema/历史/配置/ledger的回滚方法；未合并、发布或生产部署。固定告警码提供应用信号，外部通知路由和水位/source-error运行手册留OBS-004。仅本任务勾选，其他任务状态不变。

### 当前整合验收与运行边界（2026-10-12）

PR #31已普通合并为 `04937e64879f19a07a2edf0e1c931a8008c0b7c9`。当前ARCH004组合暴露的配置前置失败漏报和成功配置等待漏计整轮耗时两项P2已修复；真实Start路径RED→GREEN、一次真实45秒deadline、41秒慢配置及直接Race、定向PG/消费者编译/vet与fresh最终复核通过，输入未变SQL证据复用。本次生产只读容量样本为1490事实/总360448字节，约58小时15分，不称31天长期实测。新应用监控信号尚未进入固定429生产镜像，外部调度/真实投递仍未验收。详见[第8节续作](docs/upstream/records/2026-10-12-dod-observability-rollback.md)。

---

## OBS-002：增加低样本保护、查询限流和诊断数据隐私控制

**优先级：** P1  
**依赖：** `UPSTREAM-004`、`UPSTREAM-005`  
**建议分支：** `security/diagnostics-privacy`

### 目标

降低管理员通过低流量时间、组和模型组合推断具体用户请求的风险。

### 实施步骤

- [x] 设置最小样本量；
- [x] 对低样本叶子隐藏比例和精细时间；
- [x] 限制可查询时间范围；
- [x] 为管理员接口增加合理速率限制；
- [x] 限制导出能力；
- [x] 记录配置和导出审计事件；
- [x] 不返回内部账号、凭据域、API Key 或原始请求；
- [x] 评估 group/model 组合在小部署中的可识别性。

### 验收标准

- 样本不足时不输出误导比例；
- 普通用户无法访问内部诊断；
- 管理员批量枚举受到限制；
- 导出内容符合最小必要原则。


### 完成证据（2026-10-11）

- 开始时为 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；原分支、HEAD 与暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。前置 UPSTREAM-004 已验收候选的相关路由诊断文件与当前 personal 一致，UPSTREAM-005 `39753447cb4287c1a0aeee68b79b589ff345498a` 为实际基准祖先；实现已随 #25 纳入 personal，不把阶段 PR 的开放状态当作源码未整合。
- 从最新 `origin/personal` `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 创建隔离候选；最终分支 `codex/verify-personal-ci-obs-002-privacy` / HEAD `b8bf844d19805fda490c1d6e039585423f17586f`，五个本任务提交、32 个实现/直接调用/测试/报告文件。[草稿 PR #32](https://github.com/ccisnoxx/sub2api/pull/32) 面向 personal；[完整任务报告](https://github.com/ccisnoxx/sub2api/blob/b8bf844d19805fda490c1d6e039585423f17586f/docs/observability/OBS-002.md)说明合同、测试、风险、数据库影响和回滚。
- 展示门槛 `max(5, minimum_samples)`，隐藏低样本叶子、历史及关联事件，受影响平台计数/比例为 null，证据精确到 UTC 小时；固定24h/7d/30d历史、独立近30天单记录白名单诊断、严格管理员 JWT 与跨接口/记录/实例共享20次/分钟额度。Redis失败503、超额429，不返回内部账号、凭据域、API Key、用户身份或原始请求；无原始/批量导出，配置、读取和拒绝导出使用安全审计。既有日志认证/故障正文保留，诊断失败不清空旧详情。
- 初始159项定向unit、修正后47项权限/限流/隐私race、历史可选事件三入口6个race通过事件、真实PG仓储13项通过。远端集成暴露当前小时恢复漏读后恢复原分钟读取；独立复核发现隐藏恢复抢占200名额，真实201事件先确认可见200→199失败，再以同事务独立完整小时候选（最多200条）修正，14项仓储集成通过、200个展示ID稳定且raw合同保留。足量24桶生命周期回归先确认提示true→false，再限定提示为样本/合计裁剪，最终4项隐私race通过。
- 前端4文件39项Vitest、最终详情11项（404/429/503与晚响应隔离）、实际Axios无timezone、typecheck、定向ESLint、生产build含i18n通过；go vet、make build、差异/本地链接检查通过。最终固定SHA [Personal CI 38163070709](https://github.com/ccisnoxx/sub2api/actions/runs/38163070709) 11个作业全部success，完整unit/integration/OpenAI recording concurrency、lint、前端/严格build、双端安全、脚本/发布工具、绑定及personal-ready均通过；旧失败已修正，取消候选不计通过。
- 六次fresh只读独立复核覆盖初始候选及后续实际增量，确认问题均修正；最后提示增量无新增阻断，核读原始RED但未独立重跑测试。不把主代理测试冒充独立执行。汇总审计 `20261011T175621Z-obs-002-final-incident-review-6aadfc4f` 已关闭、校验通过，无异常；复核交付验收6次不表示旧候选从未有问题。
- 门槛不等同严格匿名：小部署同一主体多次请求、既有日志权限、滚动窗口差分与完整历史候选限额仍有组合关联风险；无生产运行、长期流量或混用旧前端验证。无DDL/迁移/账本、事实写入、保留期、计费、依赖或发布控制面改动；回滚通过受保护PR撤销五个任务提交并一起交付原前后端，保留schema/事实/事件/配置，无SQL回退或清库。未合并、发布或部署；只勾选本任务，原暂存与其他任务记录保留。

### 当前整合验收（2026-10-12）

PR #32 已普通合并至personal `fe27c725c42de005358b23c964faad81bf3ae4b3`，与冻结整合树一致。当前ARCH004组合新增3文件精确JSON测试修复，34唯一顶层定向Race成功证据和fresh独立复核通过；仓储/相关前端输入未变证据复用，不重跑Full。正式应用仍为42907bc，生产新增保护尚未部署。测试、旧失败、审计边界与普通revert方法见[第8节续作](docs/upstream/records/2026-10-12-dod-observability-rollback.md)。

---

## OBS-003：固化服务状态语义

**优先级：** P1  
**依赖：** `UPSTREAM-005`  
**建议分支：** `test/service-status-contract`

### 目标

避免“没有数据”“功能关闭”和“服务健康”被混为一谈。

### 需要固定的状态

| 状态 | 含义 |
|---|---|
| `unknown` | 无请求、样本不足、终态未知、覆盖缺口或源错误 |
| `operational` | 有足够样本且错误率低于 warning 阈值 |
| `degraded` | 有足够样本且错误率达到 warning 阈值 |
| `outage` | 有足够样本且错误率达到 outage 阈值 |
| `recovering` | 事故后的恢复窗口尚未满足 |
| `disabled` | 功能配置关闭，应与 unknown 区分 |

### 实施步骤

- [x] 为每个状态和 reason code 建立表驱动测试；
- [x] 覆盖零请求、全 unknown、样本不足、source error；
- [x] 覆盖告警、故障、恢复和 awaiting data；
- [x] 检查前端文案和颜色不会把 unknown 显示为绿色；
- [x] API 文档说明状态不是供应商全局 SLA。

### 验收标准

- 所有边界状态有自动化测试；
- 未知状态不计算成功率；
- 管理界面和 API 使用相同合同；
- 对外说明明确“仅代表本部署已观察请求”。

### 完成证据（2026-10-11）

- 开始时为 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；原分支、HEAD 和暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。前置 UPSTREAM-005 `39753447cb4287c1a0aeee68b79b589ff345498a` 为最新已获取 `origin/personal` `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 的祖先，S4 已随 #25 纳入应用树。
- 隔离分支 `codex/obs-003-status-contract`，固定提交 `da408a067dc97c635655c8a0c7453fe19a0e71b2`；[草稿 PR #33](https://github.com/ccisnoxx/sub2api/pull/33) 面向 personal，仅12个测试、直接文案和文档文件。完整报告 `docs/observability/OBS-003.md`，公开合同 `docs/service-status-api.md`。
- 固化五个快照健康状态及 disabled 配置/503 合同；最低样本、warning/outage 前值与等值边界、零请求、全 unknown、排除分母、覆盖/源/水位缺口、摘要优先级、恢复中和 awaiting_data 均有表驱动证据。19个快照 reason 和17个明确请求原因分类、unclassified/缺失/未来码得到覆盖；unknown 叶子、平台和不可判定历史的 JSON 比例为 null。
- 双语页面64项、实际 Axios5项、i18n3项共72项通过，亮/暗颜色类保证未知灰显、恢复观察琥珀色；每种语言补齐9条缺失原因文案。API notice、界面和中英文 README 说明仅代表本部署已观察请求，不代表供应商全局可用性或 SLA。
- 本地定向 metrics/summary unit、service/handler race 通过；repository 未来码新测试按严格 schema 修正为 unknown/linkage_unavailable 后单包 race 通过，复用未变包证据；go vet 通过。真实 PostgreSQL 18.1 的26状态场景和既有10个顶层仓储测试通过，最终新增事故/比例断言后26场景再次通过。冻结 pnpm9 安装、typecheck、定向ESLint、真实生产 build、文档链接/原因表及差异检查通过；原 RED、测试编译和运行环境修正详见报告，未将失败或跳过列为成功。
- 同固定候选 [Personal CI 38185468718](https://github.com/ccisnoxx/sub2api/actions/runs/38185468718) 11个作业全部 success，实际完整 unit/integration/OpenAI recording concurrency、lint、前端/严格build、双端安全、脚本/发布工具、绑定及 personal-ready 通过。重复 push CI38185416387 cancelled，CLA skipped，不计通过；重复 Security Scan38185416351 在取消生效前 success，不冒作额外独立门禁。
- 状态计算、事故恢复计数、API结构、权限、事实聚合事务、DDL/迁移/账本、保留期、依赖和发布控制面未变；没有触发高风险独立代理复核要求，未将自审冒充独立审查。没有生产或真实浏览器像素验证，颜色以渲染类及生产构建证明；源日志缺失、过滤/丢弃、范围外迟到及未支持入口仍是实际观测边界。
- PR 记录风险、数据库影响 none、通过受保护 PR 撤销本任务提交并保留 schema/事实/事故/配置/迁移账本的回滚方式；未合并、发布或部署。只勾选 OBS-003，保留其他任务和原暂存记录。

---

## OBS-004：增加服务状态聚合任务故障告警和运行手册

**优先级：** P1  
**依赖：** `OBS-001`、`OBS-003`  
**建议分支：** `ops/service-status-runbook`

### 目标

让聚合任务失败、超时、停滞或源数据缺失能够被运维发现和处理。

### 实施步骤

- [x] 为 watermark 停滞设置告警；
- [x] 为连续聚合失败设置告警；
- [x] 为 source error 设置告警；
- [x] 新增运行手册：检查配置、任务状态、数据库锁、表增长和恢复步骤；
- [x] 定义禁用功能的紧急操作；
- [x] 定义恢复后如何验证水位和事故状态；
- [x] 避免在告警中包含敏感请求内容。

### 验收标准

- 可以在聚合停止后的合理时间内发现问题；
- 运行手册包含诊断命令和安全回退步骤；
- 停用服务状态不会影响核心网关转发。

### 完成证据（2026-10-11）

- 开始工作区为 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，仅 `AM REMEDIATION_PLAN.md`；原分支、HEAD和暂存blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。当前personal基准 `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb`；前置OBS-001 `ce0b04999764133580afc6b029a818db0550a15f` / PR #31、OBS-003 `da408a067dc97c635655c8a0c7453fe19a0e71b2` / PR #33已验收但未合并，隔离候选普通merge纳入两者，唯一journal冲突保留两份原记录。累计前置基准 `dc77a3e51f8998df4fa370be773ab0a860a591dc`。
- OBS-004独立提交 `79d6db57d73de0beaeceac3ebd4aea12be55932e` / 分支 `codex/obs-004-service-status-runbook`；[草稿PR #34](https://github.com/ccisnoxx/sub2api/pull/34)面向personal，OBS-004增量仅11个文件，不重做已验收前置。运行手册 `docs/observability/service-status-runbook.md`，提交时的候选报告 `docs/observability/OBS-004.md`；最终范围确认和门禁状态以本记录及PR为准。
- 独立只读检查器采用数据库时间、一致性单行表读取、2秒语句/1秒锁预算和5秒子进程上限；水位超过180秒、缺失/未来、版本不一致、聚合错误、源缺口和探测不可用均有固定JSON/退出码。每分钟外部调度时通常最后成功水位后3—4分钟发现停止；启动/重新启用保留宽限，disabled/空平台抑制业务告警。源错误/持久化/收尾失败为结构化ERROR且跳过Ops数据库回写；复用OBS-001连续三轮失败/timeout/容量/清理规则，未修改原事务、水位推进或状态算法。
- 维护者在本轮明确确认“只交付仓库实现与手册，不部署”；外部调度安装、通知路由和真实送达是后续运维步骤，不作为本轮门禁，也未冒用本地检测宣称已生产接入。手册含配置/任务/锁/容量诊断、最新version完整PUT紧急停用、409/数据库不可达处理，以及水位、完整覆盖、事故新证据恢复验证；不停止网关、数据库或计费队列。
- Python检查器5项与backend runner6项通过；source/aggregator/monitor三包race含真实HTTP重试200通过；真实完整迁移PostgreSQL18.1/Redis8.4集成含维护源码SQL/Python判定、源缺口、锁超时/回滚/容量、disabled/重新启用、事故新证据及源错误不阻断用量通过。实际psql独立七场景与诊断/容量SQL、go vet、make build、手册Bash语法、本地链接及diff检查通过。最初目录误用导致的格式/脚本命令失败已修正，不计失败为成功。
- 同固定候选 [Personal CI38191718143](https://github.com/ccisnoxx/sub2api/actions/runs/38191718143)11个作业全部success，实际完整unit/integration/OpenAI recording concurrency、lint、前端/严格build、双端安全、脚本/发布工具、绑定及personal-ready通过。重复push CI38191655043 cancelled、CLA38191717872 skipped，均不计通过；重复push Security Scan38191654966 success不另计独立门禁。最终PR头、personal基准和隔离工作树再次核对一致。
- 无DDL、迁移身份/账本、依赖、前端、公开API/权限、事实/事故/保留期或发布控制面变更；独立诊断仅SELECT，源错误沿用原持久化owner。未触发高风险独立代理审查要求，没有把自审称为独立审查。PR已记录外部调度前提、源观测/恢复边界、无生产/长期送达证明、数据库影响none及撤销OBS-004单独提交并移除对应调度/规则、保留前置与schema/历史/配置/ledger的回滚方法。未合并、发布、部署、生产读写或发送通知；只勾选本任务。


### 当前整合验收与运行边界（2026-10-12）

PR #34已普通合并为 `423f690bec2ee9d0ebccd1672906d6b688c220c3`，携带既有OBS-003/PR #33语义依赖；实际merge tree等于最终候选。定向真实PG2顶层/28含子例、Source Race5、前端68用例和typecheck通过，fresh独立复核无源码阻断；其文档P2已按复核方向纠正跨KIN回退禁令。生产只读同SQL探测成功，未安装外部timer、发送通知或部署新producer；现有收件人/SMTP缺口已询问维护者。历史“只交付手册”边界不冒充当前接入验收。详见[第8节续作](docs/upstream/records/2026-10-12-dod-observability-rollback.md)。

---

## REL-001：构建迁移后的候选版本并完成部署前验证

**优先级：** P0  
**依赖：** M1、M2、M3 的 P0/P1 必需任务  
**建议分支：** 新 `personal` 候选

### 实施步骤

- [x] 创建预发布 tag；
- [x] 运行完整 Personal CI；
- [x] 运行 Release dry-run；
- [x] 验证所有 archive、checksums、SBOM 和 metadata；
- [x] 验证镜像构建上下文只包含目标 SHA；
- [x] 使用脱敏数据库副本完成升级测试；
- [x] 完成基本性能比较；
- [x] 审核新增告警和功能开关默认值。

### 验收标准

- dry-run 不发布外部产物；
- 所有发布材料对应同一源码 SHA；
- 数据库升级和启动成功；
- 核心网关吞吐和错误率没有不可接受回归。

### 历史前置检查（当时受阻，2026-10-11；后由最终原生验收替代）

- REL-001 尚未完成。开始时分支 `codex/doc-002-install-sources`、HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca`，工作树仅 `AM REMEDIATION_PLAN.md`；原分支、HEAD、原有台账内容和暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留，本轮只增加本节记录。
- 重新 fetch 后，`origin/main` 为 `c1dae6357b4de7cc11d81547013b87550c4ba586`，`origin/personal` 为 `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb`。KIN 标签对象 `0c1ede31ec63f9cfc2b72f07db66fb41f540208d` 和 peeled commit `ca686de5e5ac14834e12701c156831abd5ec6b71` 与冻结记录一致；personal 来源三字段仍指向该基线。自动正式发布开关 `PERSONAL_RELEASE_ENABLED=false`。
- M3 的 CI-001—005 已完成各自候选阶段验收，但 [PR #26](https://github.com/ccisnoxx/sub2api/pull/26)、[PR #27](https://github.com/ccisnoxx/sub2api/pull/27)、[PR #28](https://github.com/ccisnoxx/sub2api/pull/28)、[PR #29](https://github.com/ccisnoxx/sub2api/pull/29)、[PR #30](https://github.com/ccisnoxx/sub2api/pull/30) 均仍 open/draft、未合并。五个最终验收提交均不是当前 main 或 personal 的祖先，两条分支均缺少 `.github/release-tools/supply_chain.py`。CI-004/005 交付记录明确要求依序合入 main 后再同步必要控制文件；REL-001 的实际发布控制面前置尚未落实，不能将阶段勾选视为已投入当前候选。
- 已通过 GitHub API 核对当前 personal 的 [Personal CI 38135489900](https://github.com/ccisnoxx/sub2api/actions/runs/38135489900) 与 [Release dry-run 38135521737](https://github.com/ccisnoxx/sub2api/actions/runs/38135521737) 均 completed/success、head SHA 与上述 personal 一致。它们是 UPSTREAM-007 的历史证据，没有包含尚未同步的 M3 工具，不代替本任务的完整材料、同 SHA、升级及性能验收。
- 按本次仅处理 REL-001 的范围及 §6.4 跨里程碑停止条件，未合并前置 PR、未提前同步控制文件、未创建预发布 tag 或新候选、未 dispatch Personal CI/Release dry-run，也未执行本任务的产物核验、数据库升级/恢复演练或性能比较；未发布、部署或读写生产数据库。安全下一步是完成已验收 M3 控制面依赖的普通合并，再同步必要控制文件到新 personal 候选并重新执行 REL-001。所有实施步骤及任务总勾选保持未完成。

### 前置依赖解除与候选准备（2026-10-11）

- 维护者补充授权按依赖顺序审查并普通合并 CI-001—005。对固定五个 PR head 做 fresh 独立只读发布控制面复核，未确认合并阻断；完整 SHA 的历史成功 CI/Security/dry-run 均通过 GitHub API 重核，不以 cancelled 运行计通过。
- PR #26—#30 均已依序普通 merge 到 main，未使用 admin/force 或删除分支。实际 merge SHA 分别为 `f1d069469d67db505d5d79848702abff11bc0f5c`、`0e7d491a8532f8f48680e9da6e0e0ef4567f100b`、`5c52ceb42439933fc23951682860ae0f85f5703c`、`2db688a0e78109cf0761688a19829e35d218c25f`、`8a0e9e30a4cc17c7232a27960c541b11b20ce13a`；逐次核对 source head、parents 与实际 main tree 等于已验收源 tree。personal 基准 `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb` 未变，保护与 `personal-ready` 合同保持。
- 新隔离候选 `codex/rel-001-candidate` 从该 personal 基准开始；控制同步提交 `776b4887daf883776b03f6951ca25e9b277b849d` 只纳入必要 M3 控制文件、工具固定版本/对应文档，并保留 personal 原有许可、安装来源和 XLSX 安全检查。预发布 VERSION 为 `0.2.15-klno.2-tps.1-rc.1`，hash-only PyYAML 安装与 81 项发布工具测试通过；完整本轮 CI/dry-run 尚待固定最终源码后执行，不代替最终验收。
- 维护者确认使用仓库现有合成数据库副本作为本轮输入，不接触真实生产数据。REL-001 继续验证实际镜像、三条升级/恢复路径、旧/新真实网关基本性能及告警/开关边界；未创建正式 Release、未推送 registry 或部署，任务仍未勾选。

### 历史验证结果（当时未通过，2026-10-11；后由最终原生验收替代）

- M3 的必要 LEGAL/SOURCE 控制集成修复 [PR #35](https://github.com/ccisnoxx/sub2api/pull/35)，固定源 `a90f15759c82a397741f56abd328975682461f78`，fresh 只读复核及 [CI 38195883738](https://github.com/ccisnoxx/sub2api/actions/runs/38195883738)、[Security Scan 38195883622](https://github.com/ccisnoxx/sub2api/actions/runs/38195883622) 均通过，普通 merge 为 `b3b42f13f292851cf11c9780e6694c1bdd25b615`，parents/tree 核对成功；正式 helpers/guard 继续来自同一次可信 main checkout。
- 隔离 personal 候选 [草稿 PR #36](https://github.com/ccisnoxx/sub2api/pull/36)，base `fb3be6334d97cc1c5b3a1418ecfcf9f560530cfb`，固定 HEAD `036ade78ee5732d0dc860c3293aa70807ec1ddf1`。源码 RC tag `rel-001-rc/v0.2.15-klno.2-tps.1-rc.1` 的 annotated 对象 `3ac07674f41eb8c012bd10bbb093c3007533807b` peeled 到该 HEAD，名称避开正式发布触发器，没有改写 tag。候选未合并或部署。
- 完整 [Personal CI 38196666775](https://github.com/ccisnoxx/sub2api/actions/runs/38196666775) 11/11 jobs 成功，首尾 candidate/base/PR 绑定成功；同一 SHA 的[完整 Release dry-run 38196678157](https://github.com/ccisnoxx/sub2api/actions/runs/38196678157) prepare、frontend、五个二进制目标与 release 全部成功。registry 登录、正式授权和正式镜像验证步骤 skipped，GoReleaser snapshot skip publish/announce。现有三个 Release 和三个正式格式标签列表保持，三个 Release 实际均为 prerelease，releases/latest 返回 404，未将该查询记为成功。
- 实际下载核验五归档、27 条最终 checksum、21 份 SBOM/证明、7 份工具 metadata、GoReleaser metadata 与五个内嵌 Go1.27.2/Commit/VCS/平台信息；同一 SHA/版本/run/attempt/摘要绑定通过。两平台 OCI 所有 descriptor/blob 与 9 个应用/资源/许可文件逐字匹配从该 SHA 归档重建的最小上下文，并拒绝额外应用文件。amd64 index/Image ID `sha256:1d90265792a83cb65b780b3d068e83d21fbc6d0d63ce771ecd5c998b890df807`，arm64 index `sha256:f4a3eb41c389bed8c2c05444916b307fb8ec97be5aa17aa5cb3dab445d35d076`。材料未签名，不宣称签名/SLSA 认证。
- 五个 binary metadata 的 go.mod/go.sum 派生摘要已依据既有 `go mod tidy -C backend` before hook 在隔离 git archive 副本精确复现：仅将既有 testcontainers 改列直接依赖并删除未使用旧 checksum，无新增或升级依赖。五目标派生摘要一致，vcs.modified=true 与已声明步骤一致；未改候选源码或放宽未知摘要检查。
- 用户确认的合成副本使用真实 amd64 Image ID 执行 `go test -v -tags=integration,upstream007 -run '^TestUPSTREAM007UpgradePaths$' -count=1 -timeout=30m ./internal/repository`：空库、旧 KIN 303、个人 307 三条路径升级到 308，新/旧/新启动及真实 pg_dump/pg_restore 到独立库通过，旧 filename/checksum/applied_at、10,000 条用量及服务状态数据保持，测试主体 34.94s。首次在测试主体前因 Ryuk 挂载 macOS socket 失败；已按安装的 Testcontainers v0.40 接口仅为本进程设 `TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`，保留 Ryuk、`CI=true` 禁止跳过，修正环境后重跑成功。
- 本地发布工具 81 项、部署合同 75 项、性能工具合同 3 项通过；fresh 部署只读复核执行 39 个真实 Git/账本/证明拒绝反例及真实 CLI 的 RC 拒绝，无阻断。复核未独立重跑 75 项 suite 或核对原始 stdout，主代理当时核对原始工具返回。当前任务 SUBAGENT_EXECUTION_DIGEST 已生成/校验，audit_id `20261012T015058Z-rel-001-candidate-0c5e904d`。
- 五轮正式性能执行失败（99.72s）：固定旧 digest 和本次新 Image ID，在同一空闲 ARM/aarch64、2 CPU/4 GiB Colima 主机，独立 PostgreSQL18.6/Redis8.4/mock 上游按交替顺序执行。前九个样本完成 3,600 次 JSON/SSE 测量，非预期请求错误为 0；第五轮新镜像在就绪前 fatal OOM，申请 `239935617499136` 字节，当时仅 `28573696` 字节 in use。堆栈为 SchedulerSnapshotService 初始快照重建 → account_repo.go:3584 tempUnschedulablePredicate → ent Selector.Where/Predicate.Append → runtime.growslice。完整 comparison 未生成、rel001_acceptance=false，不能以四个新样本、中间结果、smoke 或零请求错误代替五轮阈值验收。
- 失败处于 amd64 仿真环境，但不能可靠区分仿真、Go runtime 或应用原因。按 §6.4“测试失败原因无法解释”停止继续验收，未盲重试、减少轮次/负载、放宽阈值、禁用功能或修改应用来掩盖失败。安全下一步是在原生 amd64 隔离环境对同一 OCI/源码 SHA 定向复测启动及完整五轮对比，定位原因后再决定最小修复与受影响验证。
- 默认值审核完成：migration254 enabled=false、openai、min5、warning0.05、outage0.9、异常2/恢复3窗口，已有配置保留。候选没有 OBS PR #31—#34 的外部告警治理，不声称生产送达，开启前需另行集成及验证。跨 KIN account.extra 全业务回退未获证明，部署适配继续禁止自动与显式旧镜像恢复，schema/ledger 保留。
- personal 保护/required personal-ready/app15368/strict/admin enforcement/禁止 force-delete 与仓库变量保持，自动正式发布仍 false；main 原 protected=false 未修改。没有 Release、registry push、latest 更新、部署或生产数据库操作。失败诊断与脱敏结果位于候选工作区外的 rel001-evidence，专属 Docker 容器/网络和合成凭据文件已清理。
- 更新时发现并行新增 CI-000 台账与验证规则，逐字保留本节外当前内容；本轮不执行、不勾选 CI-000。REL-001 全部实施步骤和总任务保持未完成，成功门禁不代替性能验收。原工作区分支/HEAD/暂存内容本轮不修改。


### 最终原生验收完成（2026-10-12）

- 维护者继续授权原生 amd64 隔离复测，按新 CI-000 分层规则固定最终 [PR #36](https://github.com/ccisnoxx/sub2api/pull/36) head `3ded82bcde22dad28ff6c08939369c44d57ff0b1`、personal base `7ebb60b2f6bf43e5f73c38e37b61de6cb5f063e6`。候选 `codex/rel-001-candidate` 干净，原工作区仍为 `codex/doc-002-install-sources` / HEAD `e0f1643daef7847d8ff2c144de82b0ad758964ca` / `AM REMEDIATION_PLAN.md`，暂存 blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留。KIN tag 对象 `0c1ede31ec63f9cfc2b72f07db66fb41f540208d` / peeled `ca686de5e5ac14834e12701c156831abd5ec6b71` 远端未变、仍为候选祖先。既有 CI-001—005 的依序审查/普通合并记录保留。
- 新 CI 控制集成的必要修复 [PR #41](https://github.com/ccisnoxx/sub2api/pull/41)恢复 eligibility stdin Python `-I` 与 Full 既有 install source/token 入口；[PR #42](https://github.com/ccisnoxx/sub2api/pull/42)恢复五目标归档前按 prepare SHA/VERSION 生成 SOURCE/许可材料。均 fresh 独立只读复核、适用 Quick 通过，普通 merge 分别 `51dc69aff06ca795e914e4c0951e7346064ae541`、`12615d2eaaea78d8d63484915af73956d4bf93ff`，parents/tree 核对后同步必要控制到候选。四个修复文件与可信 main 逐字一致；最终 release.yml 也与原036相同。相对036，仅23个控制/文档文件变动，应用/迁移/依赖/部署/性能工具不变；没有无关重构或放宽权限/阈值。ff6c79c7 的 dry-run `38209816504` 缺 SOURCE.json 失败保留，未计通过。
- 最终安全 annotated tag `rel-001-rc/ci000-3ded82bcde22dad28ff6c08939369c44d57ff0b1` 对象 `9ed2d944fd2d5a7e7fca8136b75ff50154022e97` peeled 到最终 SHA，VERSION 保持 `0.2.15-klno.2-tps.1-rc.1`；原RC及中间安全tag均未改写。最终高风险候选仅一次显式 mode=full：[Personal CI 38211479466](https://github.com/ccisnoxx/sub2api/actions/runs/38211479466) 15/15 jobs success，实际 Full unit/integration/recording Race/lint/前端关键+TPS+build/双端安全/部署与发布工具/同步合同以及 App15368 的 full-ci-certified/personal-ready 全通过，首尾 source/base/PR 绑定成功。自动 [Quick 38211477866](https://github.com/ccisnoxx/sub2api/actions/runs/38211477866) attempt1 路径检查成功而聚合因 Full 尚未完成明确失败；Full 成功后仅 rerun failed 两个聚合job，attempt2 success，成功应用检查/安全扫描复用。实际 PR required personal-ready 为 1 passed / 0 failed，没有重复 Full。
- 同最终 SHA 的[完整 Release dry-run 38210844700](https://github.com/ccisnoxx/sub2api/actions/runs/38210844700) 8/8 jobs success，dry_run=true/simple_release=false。实际下载24 artifacts后核验五归档、27最终checksum、21 SBOM/provenance资产、7工具metadata、GoReleaser metadata和五目标内嵌Go1.27.2/Commit/VCS/平台；全部同source/workflow/run/attempt/版本及摘要。两平台OCI所有descriptor/blob及9个应用/资源/许可文件逐字匹配固定源码归档重建的最小上下文，拒绝额外应用文件；before hook派生Go输入摘要与已精确复现记录一致。一次在下载完成前缺metadata的校验保留、不计通过，输入完整后才完成上述核验。材料未签名，不宣称签名/SLSA认证。
- 最终 amd64 artifact `11729451134`，ZIP digest `sha256:1922376221cbb1a0d9d042b1c4cc4065d89eb601aaf39588f577ea6d8c82b19d`、tar SHA-256 `c25369794d147d89bc8966a9f3e68e8cea76cbe8dcea98b28f3fe23692553fa3`；index/实际加载 Image ID `sha256:9397e21caf01ea1080c84a93809d5a586a9d6864e6923872a58589817b1850e3`，manifest `sha256:9cd36d4fa42c6c0b890449f71125652917e406006afbd3618b787d98a42babb1`。一次性验证控制 `d75b6103ed560559fe67233e953143393013ca6c` 的[原生 amd64 38212668631](https://github.com/ccisnoxx/sub2api/actions/runs/38212668631) success：x86_64宿主及daemon、4CPU/16GiB、Docker28.0.4，直接加载同OCI，revision=最终SHA，未重建应用。既有性能工具字节SHA-256 `2a6078d692933f323bd5bea2ae4f3710985f159fb413859f4a624406a79238f5` 未变。
- 完整五轮旧/新交替共10个独立样本、4,000测量请求，每样本每格式200/并发4/预热10；JSON/SSE完整响应/usage/terminal、受控400与后续恢复通过。JSON吞吐中位数72.9773→73.1771 rps（+0.2739%），p95中位数60.1302→59.7489 ms（-0.6341%）；SSE吞吐190.1098→192.7245（+1.3754%），p95 25.4829→25.6746（+0.7523%），全部意外错误0。独立复算同脱敏JSON一致，原吞吐下降≤15%、p95增加≤20%、错误0阈值未变。
- 同一已加载OCI在性能结束后执行原生Go1.27.2既有 `go test -json -tags=integration,upstream007 -run '^TestUPSTREAM007UpgradePaths$' -count=1 -timeout=30m ./internal/repository`：root/empty/old-kin/already-personal全PASS，主体45.25s。空库、旧KIN303、personal307升级到308；旧filename/checksum/applied_at、10,000完整用量及服务状态数据保持；各路径新→固定旧→新health和真实pg_dump/pg_restore到独立库通过。仅用户确认的合成副本，无生产数据。测试结果不允许fail/skip，原始输出不上传；性能、合成凭据及Go Testcontainers容器/网络清理门禁通过，无残留。
- 原036同OCI的[原生38207446049](https://github.com/ccisnoxx/sub2api/actions/runs/38207446049)先完成完整五轮通过。旧Colima为QEMU7.0用户态仿真；64KiB MADV_DONTNEED探针返回成功却保留全部脏字节，原生arm64清零；Go1.27.2回收/分配探针在同仿真默认环境崩溃、原生与仅对探针madvdontneed=0通过。实际Go runtime对回收页依赖清零，该违约与发SQL前的新Predicate单回调append约218TiB异常申请有机制支持；未取得原崩溃堆转储，不能声称逐指令回放或证明所有应用并发路径。参见[Go最低要求](https://go.dev/wiki/MinimumRequirements)与[Go#77572](https://github.com/golang/go/issues/77572)。实际应用未加锁/内存或绕过；最终原生未观察到kernel/runtime OOM，采样峰值约96.2MiB，采样不覆盖全部瞬时状态。首次原生OCI加载失败仅针对一次性runner启用containerd存储后修复，旧失败保留。
- 本地控制入口10项、release_matrix19项、Python编译/actionlint/diff通过；原生工具4项通过；部署75项/性能3项及独立部署39个真实拒绝反例复用相关字节未变的成功证据。审计Bundle `20261012T050618Z-rel-001-native-continuation-b56b910c` 已生成/验证 SUBAGENT_EXECUTION_DIGEST 并关闭/verify通过，本轮6次调查/审查交付验收、5次独立只读复核。独立复核未冒充完整suite重跑；原生失败超时下Go子进程完整生命周期未另做真实Docker故障演练，成功路径清理已有实际证据。
- 默认值/告警边界沿用已审查结果：migration254 enabled=false、openai/min5/warning0.05/outage0.9/异常2恢复3、已有配置保留；OBS PR #31—#34未合入，外部告警/通知送达未部署，开启前需另行集成验证。候选无新增/修改DDL或迁移身份，跨KIN account.extra所有写入的无损回退未获证明；部署适配继续禁止自动和显式旧镜像恢复，SQL/schema/ledger不回退。候选未部署，撤销REL-001专用增量可恢复personal运行树并保留安全控制/固定tag；真实备份恢复点与写入损失评估留待REL-002。
- personal完整保护对象/严格personal-ready/App15368/admin/禁止force-delete、merge-only及两仓库变量与此前快照一致，自动正式发布false，main原protected=false未改。dry-run两平台导出OCI、GoReleaser snapshot skip publish/announce，正式registry登录/授权/发布后验证/通知 skipped；3个既有Release及正式格式tag列表未变，latest仍404（均prerelease，不计成功查询）。没有正式发布、registry push、latest更新、部署或生产操作。最终PR已回写实际证据、风险、数据库影响和回退，仍draft/open；四项验收全部满足后仅勾选REL-001八个步骤及总任务，本节之外的并行CI-000/其他台账、原分支/HEAD/暂存内容保留。

### 普通合并与 Full 复用完成（2026-10-12）

- 维护者授权固定最终 [PR #36](https://github.com/ccisnoxx/sub2api/pull/36) 的普通合并。再次核对 head `3ded82bcde22dad28ff6c08939369c44d57ff0b1`、最新 personal base `7ebb60b2f6bf43e5f73c38e37b61de6cb5f063e6`、候选干净及原工作区分支/HEAD/暂存blob。Ready 自动 [CI 38215537156](https://github.com/ccisnoxx/sub2api/actions/runs/38215537156) success，实际 Quick/require_full=true 引用根 Full `38211479466`，required personal-ready 成功。独立 critical_reviewer 审查14个实际差异文件及相关部署/账本/发布/CI合同、原始证据并复算五轮性能，未确认合并阻断；没有重跑完整或历史部署suite，不表示GitHub review approval。
- 未使用admin/force/squash/rebase/删除分支，普通merge为 `4cf71d60901e524daf933ba2eebd985d1d378108`。实际parents依序为上述base、head；base是head祖先，merge tree `26c6e86b657715ed358dbdc75e2bae6f1de6ea90` 与候选tree完全一致。实际 `ci_policy.merge_candidate/merge_evidence` 返回固定head/base及根Full，候选/base/合并tree满足新分层规则，未另dispatch Full。
- 自动 [personal push CI 38216819185](https://github.com/ccisnoxx/sub2api/actions/runs/38216819185) completed/success，10个job成功、3个按规则skipped（不计通过）；实际plan source_sha=merge、diff_base=原base、mode=quick、evidence_run_id=38211479466。merge SHA上的App15368 full-ci-certified/personal-ready都success。工作流标题的push base显示merge SHA，以实际plan diff_base和parent/tree绑定为准。既有最终dry-run/同OCI原生启动、合成数据库升级及完整五轮性能证据复用，不冒称本轮重跑。
- 自动 [Personal release 38217170700](https://github.com/ccisnoxx/sub2api/actions/runs/38217170700) 的title绑定该merge、可信控制head=main `12615d2eaaea78d8d63484915af73956d4bf93ff`；prepare/run skipped，正式发布开关仍false。personal完整保护对象、strict required personal-ready/App15368/admin enforcement/禁止force-delete、merge-only和仓库变量保持，main原protected=false未改；3个既有Release/正式格式tag、上游KIN冻结对象及peeled身份不变。没有正式发布、registry push、部署或生产数据库操作。现有OCI revision仍绑定3ded，CI tree复用不允许将旧材料声明为新merge SHA的正式发布材料。
- 本轮只合并既有候选并更新PR报告/本节，没有新增实现、SQL、依赖或阈值改动。原工作区分支/HEAD及暂存blob `25c8e20e0d99304fee304b5ef1c998d66e47951a` 保留；候选仍为固定3ded且干净。本节外的并行台账逐字保留，REL-001既有验收勾选保持，REL-002未执行；如需撤销合并，另开受保护PR以第一parent为基准revert并执行对应CI，禁止强推、删除tag或回滚schema/ledger。
- 独立审查交付与执行记录保存于审计Bundle `20261012T065556Z-rel-001-personal-merge-d0ffbb79`；派发后本机Codex config.toml摘要由 `2be4036b7592ad6daecf0698347d25f784c9c7b2835439f7f219fe32512f761f` 变为 `a78e60abe9e9d7e14bfa9d234a85775d261fb524bc557cf55f5e34b391673afd`，summary/digest校验明确失败。未修改历史计划来迎合当前配置，本轮未生成有效SUBAGENT_EXECUTION_DIGEST、Bundle保持open，不宣称审计校验/验收计数通过；该限制已在PR披露，远端候选/Full/merge及required实际证据独立核验成功。


---

## REL-002：在测试环境和生产环境分阶段发布

**优先级：** P0  
**依赖：** `REL-001`  
**建议分支：** 无代码改动，按发布流程执行

### 实施步骤

- [x] 先部署测试环境；
- [x] 验证登录、授权、模型目录、转发、计费、用量、诊断和服务状态；
- [x] 观察至少一个完整聚合周期；
- [x] 备份生产数据库；
- [x] 在生产执行迁移前确认锁影响；
- [x] 分阶段切换流量；
- [x] 验证错误率、延迟、扣费和后台任务；
- [x] 保留旧镜像和明确回滚命令；
- [x] 发生迁移异常时停止继续发布，不执行临时手工改表。

### 本次执行记录（2026-10-12）

固定 personal `42907bcab278f93db48d7e1b39b28e28f664569e` 的唯一手动 [Full 38274106520](https://github.com/ccisnoxx/sub2api/actions/runs/38274106520) 15 个必要 job 成功。修复 OCI 根选择、可信旧 artifact 恢复运输及持久维护标记，经定向验证和独立复核后，复用原成功构建、tag/source/Full/digest，完成 [恢复验证 38291786472](https://github.com/ccisnoxx/sub2api/actions/runs/38291786472) 和实际 [Release](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.15-klno.2-tps.1)；原失败 Release run 保留。固定 index `sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3` 已在独立测试及生产部署。生产同 MVCC 快照备份完整隔离恢复退出 0；锁探测回滚后部署 `20261012T191628Z-3be259442583` 成功，307 旧行/applied_at 全部保持，只追加固定 242 到 308 行。公开请求和实际扣费、完整聚合周期、6 个真实合格成功事实、错误率 0 与任务心跳通过。维护入口 open 保留可再次 freeze，旧 tag/digest 和同次恢复点保留；跨 KIN 镜像单独回退仍禁止，维护恢复可能丢失快照后写入。REL-002 已验收；恢复详情及证据边界见 [续作记录](docs/upstream/records/2026-10-12-rel-002-recovery.md)，首次失败见 [原记录](docs/upstream/records/2026-10-12-rel-002.md)。

### 验收标准

- 测试环境验证通过；
- 生产迁移完成且账本正确；
- 镜像 revision 与发布 SHA 一致；
- 回滚路径已准备但未被破坏；
- 发布后核心指标稳定。

---

## REL-003：完成发布后验证、文档更新和旧基线归档

**优先级：** P1  
**依赖：** `REL-002`  
**建议分支：** `docs/post-release-<version>`

### 实施步骤

- [x] 更新 README 支持版本；
- [x] 更新 `UPSTREAM.md`；
- [x] 更新 NOTICE 中的参考基线；
- [x] 记录 Release、CI、镜像 digest 和数据库迁移结果；
- [x] 标记旧基线为归档而非删除；
- [x] 关闭已完成任务并保留验收证据；
- [x] 建立下一次上游检查日期。

### 本次验收（2026-10-12）

三语 README 固定本次实际预发行 tag/source/digest，支持政策和 UPSTREAM.md 更新；NOTICE 的 KIN/Plus 来源字段与固定发布源码相同，main 的分支导航补充导致字节不同，摘要分别明确记录，没有改写正式镜像许可材料。Release notes 已追加真实 Full/恢复/来源/迁移/部署及回退边界，原证明正文、12 个 assets、tag/source/prerelease 标志回读保持。BASELINE.md 只增加归档前言，旧 tag/digest/恢复点保留，下一次人工来源检查为 2026-10-26。main/personal 普通快进、选择性控制/文档同步，应用运行树不变；详见 [REL-003 记录](docs/upstream/records/2026-10-12-rel-003.md)。第 8 节逐项核对 11 项完成、3 项计划外限制；不声称整个计划已经完成。

### 验收标准

- 仓库文档反映实际生产版本；
- 用户可以获取旧稳定 tag；
- 新版本的源码、镜像、SBOM 和发布记录相互一致；
- 任务清单中已完成项均有 PR 或发布证据。

---

# 6. Codex 执行协议

Codex 在执行任一任务时，应遵循以下顺序。

## 6.1 开始任务前

1. 读取本文件对应任务完整内容；
2. 检查依赖任务是否已经完成；
3. 输出当前分支、HEAD SHA 和工作树状态；
4. 确认不会直接修改受保护分支；
5. 列出计划修改的文件；
6. 对涉及上游版本、依赖版本或 GitHub Actions 的任务重新验证当前状态；
7. 对数据库任务确认迁移是否已在任何受支持环境执行。

建议命令：

```bash
git status --short
git branch --show-current
git rev-parse HEAD
git remote -v
```

## 6.2 实施原则

- 一个提交或任务批次保持清晰范围；PR 按需要使用，不要求每个小任务创建 PR；
- 优先做最小、可验证的改动；
- 不顺手重构无关文件；
- 不通过删除测试解决失败；
- 不修改审计例外来掩盖新漏洞；
- 不擅自扩大接口权限；
- 不把开发机路径、个人邮箱或本地工作树写入正式文档；
- 仅对新增、改变或确有覆盖缺口的可观察合同增加自动化测试；
- 对迁移变化增加空库、旧库和已迁移库测试；
- 对发布控制面增加负向测试。

### 本地预检与批量推送

- 除任务明确要求，不在中间状态反复推送；第一次远端推送前完成全部任务内修改、路径相关的本地格式/编译/测试，记录实际命令与结果，并核对工作树和 diff。
- 普通 push/PR 不触发应用 CI 或安全扫描；相关任务批次完成后选择本地定向检查，需要云端反馈时手动 Quick。完整应用检查、安全扫描与镜像构建集中在准备发布的固定 SHA；源码与环境未变时复用成功证据。
- CI 失败先分析失败 job，区分代码/测试、扫描、网络/基础设施和旧运行取消；在本地复现精确命令或确认已改变的环境原因后再重试，不直接 Re-run all jobs。
- 基础、源码 tree 或相关环境变化后只重跑受影响检查；不得为每个修正提交重复触发完整应用回归、安全扫描和发布门禁。

## 6.3 Codex 完成任务时必须输出

```text
Task: <TASK-ID>
Summary:
- ...

Files changed:
- ...

Tests run:
- command: result

Risks:
- ...

Database impact:
- none / details

Rollback:
- ...

Acceptance checklist:
- [x] ...
- [x] ...
```

## 6.4 必须停止并报告的条件

- 目标 KIN tag 或提交已经变化；
- 发现迁移编号相同但内容不同；
- 发现生产已执行迁移与计划不一致；
- 需要强推或绕过分支保护才能继续；
- 测试失败原因无法解释；
- 需要扩大权限或使用新的长期 secrets；
- 发现凭据或用户数据已经进入 Git 历史；
- 任务范围需要跨越多个里程碑，且无法保持可独立验证和回退的任务批次。

停止时应记录事实、影响和安全的下一步，不得自行绕过约束。

---

# 7. 按变更选择的验证入口

以下命令是按行为和风险选择的替代验证入口，不是每个任务的累积清单。日常先按相关任务批次进行本地定向验证，Quick 按需手动执行；完整应用回归、安全扫描和发布门禁集中在准备发布的固定 SHA。优先复用输入未变的成功证据，缺少风险依据时不升级到全量。

## 7.1 后端

```bash
cd backend
python3 ../tools/test_backend_unit_runner_test.py
make test-unit
make test-integration
go test -race -tags=unit -timeout=60s \
  -run '^Test(Recorder|OpenAIRecording)' \
  ./internal/pkg/upstreamrecord ./internal/service
```

按任务补充定向测试，不应只依赖全量测试。

## 7.2 前端

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run type-check
pnpm test -- --run
pnpm run build
```

若项目现有 Makefile 定义不同，以仓库现有命令为准，并在 PR 中写明实际执行命令。

## 7.3 安全与静态检查

```bash
cd backend
golangci-lint run --timeout=30m
govulncheck ./...

cd ../frontend
pnpm audit --prod --audit-level=high --json > audit.json || true
cd ..
python tools/check_pnpm_audit_exceptions.py \
  --audit frontend/audit.json \
  --exceptions .github/audit-exceptions.yml
```

## 7.4 部署与发布

```bash
/bin/bash -n deploy/apple-container.sh
/bin/bash deploy/tests/apple-container-test.sh
/bin/sh deploy/tests/docker-compose-security-test.sh
/bin/sh deploy/tests/docker-compose-gateway-env-test.sh
/bin/sh deploy/tests/docker-runtime-resources-test.sh
/bin/sh deploy/test-caddyfile-cache.sh
```

只有变更实际影响构建、发布来源、产物或运行环境时才执行对应 Release dry-run；只有迁移、升级/回退兼容、备份恢复或部署任务明确要求时才执行数据库恢复演练。文档和普通局部修复不默认重复这些门禁。

---

# 8. 总体验收标准（Definition of Done）

只有满足以下全部条件，整个修复计划才可视为完成：

2026-10-13 UTC 第8节当前核对：**13项完成、1项未满足**。OBS源码整合、门禁修复、成功新Full、tps.2、独立测试→生产及本机监控均实际验收，记录同步至两分支。发布源码固定 `c08fba12eb4cd30e65c4177f3acbe20567ecd252`，Full38335510831，Release38337243568，OCI index `sha256:c4ffab7a920f05564eafe055bdb3171772b145ca5e2ccda690fe8b554bf6fa25`；后继文档HEAD不冒充发布SHA。旧失败Full38333250481、旧产物及原工作区改动保留。外部通知不需要，按本次环境不适用。唯一未满足为跨KIN全业务无损回退：定向验证已确认信息丢失，禁令保持，不能勾选或宣称整个修复计划完成。

- [x] README 明确说明 fork 身份、分支职责和正确安装入口；DOC-001、DOC-002 及当前两分支 README 已提供 personal、GHCR 和固定版本安装入口。
- [x] 根目录存在完整 NOTICE 或第三方来源说明；LEGAL-001/002、ARCH-005 与当前两分支 NOTICE 保留 KIN/原始上游/Plus 来源和归属边界。
- [x] 本 fork 拥有可用的私密安全报告渠道；SEC-001 已合入，当前安全政策指向本仓库，API 回读 Private Vulnerability Reporting 为 `enabled=true`。
- [x] 不存在过期安全例外或占位 owner；SEC-002 续作已按两个分支的实际依赖审计清理 main，保留的两项精确 XLSX 例外均有真实 owner 和 `2026-11-09` 到期日，两份有效报告由严格校验器匹配通过；XLSX 漏洞仍受短期例外管理。
- [x] 新 `personal` 基于重新确认的 KIN 稳定版本人工重放完成；UPSTREAM-007/008 已整合，来源为 `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71`。
- [x] 未绕过历史分叉、祖先检查或分支保护；UPSTREAM-007 保存旧 personal 与新 KIN 的真实历史，后续普通合并/快进记录及当前祖先核对一致；CI-000 保护调整有维护者授权，禁止强推/删除保持。
- [ ] 所有数据库升级路径和回退兼容路径完成验证；MIG-001、UPSTREAM-007、REL-001 已验证空库/旧 KIN/已迁移 personal 三路径和合成库恢复，REL-002 同次生产备份已完整隔离恢复、写入损失边界已说明，跨KIN定向验证已确认旧休整状态读写丢失 `quality_gateways`，静态审计另确认联系人64/300上限和多协议凭据不兼容；全业务无损条件不成立，现有旧镜像恢复禁令保持。详见[第8节续作](docs/upstream/records/2026-10-12-dod-observability-rollback.md)。
- [x] GitHub Actions 和关键工具版本可复现；CI-001—005 已整合必要控制文件，两分支各 91 处远程 Action 引用均为完整 SHA，关键工具版本/摘要已固定。沿用既有验收边界，不宣称 runner、apk 或漏洞数据库逐字节恒定。
- [x] 正式产物包含许可证、NOTICE、SBOM 和源码 revision；当前tps.2的源码 `c08fba12eb4cd30e65c4177f3acbe20567ecd252`、成功Full38335510831、Release411195117/14个assets、OCI index/amd64 leaf及测试/生产镜像均绑定，材料未签名，不声称加密签名证明；旧产物与原失败保留。
- [x] 模型目录不会因多渠道遍历顺序产生不确定结果；UPSTREAM-003/ARCH-003 已整合“一组最多一个渠道”约束及冲突显式拒绝，正逆顺序回归已有通过证据。
- [x] 路由诊断和服务状态具有低样本与隐私保护；OBS-002已在当前personal整合并随tps.2进入生产，定向Race/真实PG/前端及独立复核证据复用，当前Full成功。实际测试JWT/403/401、schema2投影与低样本NULL/unknown保持；6个成功事实不被误称为可确认健康。见[OBS验收](docs/upstream/records/2026-10-12-obs-release.md)。
- [x] 服务状态容量、清理、聚合失败和水位停滞均可监控；PR #31/#34及前置失败/整轮耗时修复已整合并随tps.2运行。测试和生产实际观察aggregate/cleanup/capacity结构化事件；隔离测试停应用产生watermark_stale、恢复后两次current，错误数据库目标返回watch_unavailable，真实source_error保留并自然闭合。生产本机timer已启用，安装完成后的两次自动调度间隔59.870731秒，完整聚合及账本核对通过。外部通知按维护者需求不适用。见[OBS验收](docs/upstream/records/2026-10-12-obs-release.md)。
- [x] 正式发布通过分阶段部署并保留旧稳定 tag；原REL-002验收保持，新tps.2完成独立测试、全新生产备份完整隔离恢复、冻结→单应用部署→回环→显式恢复→公开业务及完整聚合/本机timer验收。旧tps.1与v0.2.14-klno.5-tps.2 tag/digest、历史恢复材料保持。
- [x] `UPSTREAM.md`、NOTICE、`personal-source.json` 和 Release notes 相互一致；原REL-003归档保持，当前tps.2固定源码/KIN/Plus、Full/Release/digest/工具/迁移/部署及本机监控已逐项绑定。NOTICE字段一致与字节差异沿用原边界，后继文档HEAD不冒充C08发布SHA。

---

# 9. 建议的实施顺序

建议严格按以下顺序推进，避免在不稳定基线上进行大规模架构重构：

1. `PREP-001`—`PREP-003`
2. `DOC-001`、`DOC-002`
3. `LEGAL-001`、`SEC-001`、`SEC-002`
4. `UPSTREAM-001`、`MIG-001`
5. `UPSTREAM-002`—`UPSTREAM-006`
6. `UPSTREAM-007`
7. 优先完成新 `CI-000` 的控制面落地和验证，再继续余下任务（`CI-001`—`CI-005` 既有成果保留）
8. `UPSTREAM-008` 独立补回个人更新服务和版本徽章；只做受影响路径的本地定向验证
9. `ARCH-003`、`OBS-001`—`OBS-004`
10. `REL-001`—`REL-003`（按当前授权及实际验收记录执行）
11. `ARCH-001`、`ARCH-002`、`ARCH-004`、`ARCH-005`

其中，`ARCH-001` 和 `ARCH-002` 不应阻塞新的 KIN 基线迁移；先稳定行为，再做结构拆分。

---

# 10. 单任务模板

后续新增任务时可复制以下模板：

````markdown
## TASK-ID：任务标题

**优先级：** P0 / P1 / P2  
**依赖：** TASK-ID  
**建议分支：** `type/task-name`

### 目标

...

### 实施步骤

- [ ] ...

### 验收标准

- ...

### 测试

```bash
...
```

### 风险

- ...

### 回滚

- ...
````

---

## 11. 维护说明

本文件应作为可执行任务台账维护，而不是一次性分析报告。

每完成一个任务：

1. 勾选对应任务；
2. 在任务下方补充 PR、commit 或 Release 证据；
3. 若任务范围发生变化，先更新本文档；
4. 若新的 KIN 基线发布，先更新基线信息和迁移风险；
5. 不删除历史决策，使用“已替代”说明保留审计链。
