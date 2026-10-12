# ARCH-005：长期上游矩阵与变更记录

## 范围与固定输入

工作分支 `codex/arch-005-upstream-maintenance` 从 main `ab86413aa27848239d0374adeff9f7aeaa41609e` 创建。依赖 PREP-003 与 UPSTREAM-007 已交付；ARCH-002 依赖尚未执行的 REL-002，本任务不启动发布/部署来满足它。应用来源固定为 PR #48 普通合并后的 personal `bea1db12d5f4853c0b85b44840e15de6a74309b1`，KIN `v0.2.15-klno.2` / `ca686de5e5ac14834e12701c156831abd5ec6b71`。

## 已实现记录

- [UPSTREAM.md](../../UPSTREAM.md)建立 I/N/A/K/P/C 固定来源、实际 S1–S5/ARCH-001/ARCH-004 owner 与直接测试、Plus 四模块冻结/停止追随，以及 KIN 已有能力复用方向。保留 PREP-003 原历史，不将旧 A 的更新能力宣称为 I 当前能力。
- [发布/迁移记录目录](../upstream/records/README.md)定义可核对字段，保存三次已发布个人版本及新 KIN 人工重放的固定历史、文件交点、语义决策、真正的 journal 文本冲突和回退边界。main 记录应用 SHA，不往 personal 自身写其自身 SHA。
- [维护合同](../conventions/personal-maintenance.md)更新下一次重放步骤和当前 CI-000 手动流程；NOTICE 以实际 personal 来源登记新 N 和原 Plus 署名，不改许可证条件。历史发布 Notes 未列出的 KIN/Plus 来源明确从同 SHA 源码重建，不冒称自动模板已输出。

## 来源核对发现与未完成应用行为

固定 I 的更新服务、测试、版本徽章和测试四个 blob 与 N 完全相同，旧 A 的个人 Release/container/manual 拒绝原地更新、五段 personal 版本比较没有迁回。已在矩阵/迁移记录登记后续 service、DTO、UI 与必要定向验证方向；ARCH-005 不修应用，也不把此缺口写成已解决。I 的 RC VERSION 没有本次查询确认的正式 Release 或镜像 digest。

N 原有 recording、Codex CatalogConfig、旧首字/耗时、渠道授权和真实计费 resolver 可复用，但没有 S1–S4 整模块已被 N 原生吸收的证据。当前不删除个人增量、测试或 schema；未来新目标有等价合同后才审查收敛。

## 定向验证与限制

只读核对固定来源对象、标签 commit 与阅读 SHA、祖先/连接 parent/tree、版本、三字段来源及历史 Release notes；检查本次 Markdown 本地链接和固定 Git 链接目标，以及 owner/test 路径存在。文档链接初检发现 A 没有 UPSTREAM.md 和 AccountTableFilters 测试目录漏 admin，已按实际 Git 路径纠正。复核纠正后，定向脚本通过 32 个本地链接、24 个固定 Git 源链接、121 个 owner/test/迁移/控制文件路径、三次 Release 字段及来源/历史关系核对，错误为 0；入口存在不表示重跑应用测试。

未执行应用 unit/race/integration、前端构建/E2E、全量 CI、安全扫描、发布 dry-run、registry 拉取或生产操作。历史动态证据只绑定当时固定运行对象，不冒用为当前 I 的发布/部署资格。当前 main 不存在 personal-source.json，不新增伪造副本；来源字段始终读取固定 personal。旧 GitHub Release 不被外部修改。

## 数据影响与回退

仅文档/NOTICE 修改；应用、工作流、来源 manifest、SQL/迁移、依赖、产物与部署工具无改动，数据库影响 none。撤销本任务文档提交可恢复记录，保留旧来源与发布历史；不改 main/personal 历史、发行 tag 或镜像，也不触发应用/数据库回退。

## 固定候选独立复核与审计

实质文档候选 `c54b9497d4bee04215384deb494c5467b917a7db` 经 fresh critical_reviewer 只读复核，未确认阻断问题。复核交叉读取固定 C 的 sync/binding、CI-000、Personal CI/Personal release/Release 与 guard，以及固定 I 的来源、MIG-001/007、NOTICE 和实际历史 Releases API；发布/迁移/许可合同与现有实施一致。未独立执行动态测试、迁移/生命周期、归档/OCI/registry 或生产验收。

首个普通 reviewer 仅确认八文件 diff 与角色边界后返回 ROLE_MISMATCH，没有完成一致性审查；按角色转交 fresh critical_reviewer，不把前者记成验收通过。审计 `20261012T133859Z-arch-005-upstream-maintenance-42e7f252` closed、audit-verify passed（20 artifacts），保留该已转交角色异常；三阶段实际尝试3、验收2，未执行 ready task/残留活跃 worker 为0，未观察到子代理写入。Agent TOML 配置为 gpt-6.1-sol/xhigh，不冒充运行时模型确认。

本次追加仅登记复核/审计结果，未改变固定候选中的实质来源、流程或应用输入；沿用成功定向证据。交付使用一次普通 push、main 草稿 PR，尚未合并，不启动发布/部署。
