# 发布与上游迁移记录

记录由 main 维护，完整应用身份从固定 personal 源码读取。[来源矩阵](../../../UPSTREAM.md)定义当前 owner/测试和 Plus 冻结范围，[维护合同](../../conventions/personal-maintenance.md)定义执行顺序。记录不生成正式版本，不修改来源文件，也不作为发布或部署授权。

## 已保存记录

- [个人历史发布](published-personal.md)：GitHub API 查询到的三次已发布个人版本、各自 KIN 来源和四模块参考的实际引入边界。
- [KIN v0.2.15-klno.2 迁移](2026-10-11-kin-v0.2.15-klno.2.md)：分叉、S1–S5 处理、真实历史连接、来源/回退和迁移后维护快照；原迁移时未正式发布；本次实际发布见下列 REL-002/003。

- [REL-002 原失败](2026-10-12-rel-002.md)及[恢复/部署验收](2026-10-12-rel-002-recovery.md)：固定 Full、原 artifacts、真实 Release/digest、独立测试及生产备份/迁移/聚合。
- [REL-003 发布后记录](2026-10-12-rel-003.md)：当前支持版、来源、归档及第 8 节核对。
- [ARCH-002 控制工具验收](2026-10-12-arch-002.md)：模块/配置、完整可信 bundle、只读 dry-run、95合同与真实现场无变化。

## 每次发布的字段与交叉核对

| 字段 | 权威输入与核对方式 |
|---|---|
| 状态/时间 | GitHub Release 的 published_at、draft/prerelease；候选或 dry-run 用自身状态，不填成 published |
| 个人 tag / 完整 SHA | 实际远端 tag 的 commit；与 Release notes 源码 SHA、应用归档 SOURCE.json.sha、镜像 revision 相等，不能用 target_commitish 的分支名猜 SHA |
| KIN 仓库 / tag / SHA | 同一个个人 SHA 的 deploy/personal-source.json；远端 tag object/peeled commit 和个人祖先关系核对 |
| Plus 版本 / 标签 commit / 阅读 SHA / 范围 | 同一个应用 SHA 的 NOTICE/来源矩阵及模块引入记录；不存在本表四模块参考时注明不适用，无法核实则写原因并保留缺口，不补造 SHA |
| 控制工具 revision | 实际运行捕获的 main 控制 SHA；不以应用同名脚本或旧工作流代替 |
| CI / Release | 固定同一次成功手动 Full run 与其必要 jobs、实际 Release run/attempt；历史旧流程按当时事实记，不冒用为当前发布资格 |
| 平台 / image digest | 已发布镜像核验材料；记录 immutable digest 与平台，不用 latest 指代固定版本 |
| Notes / 产物 / 回退 | 实际 Release notes、源码/材料链接，迁移影响与回退限制；源码记录与发布结果各自绑定 SHA |

记录采用上面字段表和实际证据创建新的 Markdown 文件，不在这里预填未发生发布的 tag/digest/run。发布或源码更新后补一条记录，旧记录不覆盖；纠正来源用带日期的新说明保留旧证据。Release notes 中可链接固定 main 记录，同时保留固定应用源码/NOTICE 链接；目前发布模板仅自动写源码 SHA/链接等字段，完整 KIN/Plus 来源在此与 NOTICE 交叉核对。

## 每次迁移的最小记录

记录旧来源和稳定应用、目标标签/commit、merge-base/分叉或祖先结果、模块 owner/测试路径，按实际冲突文件及语义决策登记。说明未复用的历史基线代码、KIN 已有能力的收敛方向、仍需重放的个人增量、停止自动跟踪的 Plus 范围、迁移身份/回退和已完成/未验证边界。报告的 source/merge/dry-run/发布 SHA 各自写明，不把候选的产物冒充最终 merge 或生产。

只读检查可使用 `git show <完整SHA>:deploy/personal-source.json`、`git show <完整SHA>:NOTICE`、`git ls-tree -r <完整SHA>`、`git merge-base --is-ancestor` 和 `git ls-remote --tags`。这些命令的 SHA 从记录真实字段取值；main 文件不存在或与 personal 不同不会成为使用另一份应用源码的理由。未来动态检查仍按当时变更和明确门禁执行，文件/链接检查不说明测试通过。
