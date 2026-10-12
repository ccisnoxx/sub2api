# personal 来源、重放与发布记录合同

`main` 管理同步/CI/发布/部署控制工具和长期维护记录；`personal` 保存实际应用。维护文档以固定的 personal 完整 SHA 读取源码、测试、`deploy/personal-source.json` 与 NOTICE，不能用 main 的同名文件认定应用状态。当前源矩阵入口是 [UPSTREAM.md](../../UPSTREAM.md)；操作与授权规则以 [CONTRIBUTING_PERSONAL.md](../../CONTRIBUTING_PERSONAL.md) 和 [CI-000](../ci/CI-000.md) 为准。

## 固定来源与证据 owner

- KIN 来源文件保留 `upstream_repository`、`upstream_tag`、`upstream_sha` 三字段。上游标签须记录 tag object 和 peeled commit 的区别，以完整 SHA 固定；未发布 SHA 不能借旧标签冒充发行来源。Plus 标签提交和固定阅读提交分别登记于 NOTICE，不把参考版本背景当作自动同步基线。
- 应用版本、上游版本、实际发布标签、应用 SHA、控制工具 revision、镜像 digest 是不同字段。`VERSION` 中的 RC 值、成功 dry-run、PR 合并和旧版本的成功 Full 都不表示当前应用已经发布。
- 来源、维护 owner 和原作者归属分别记录；沿用版权头/SPDX/许可证。不因文件被重写、功能同名或相同许可证认定完全自主原创或整模块复制。当前文件/测试只证明入口存在，实际测试结果绑定其执行的代码和环境。
- [来源记录目录](../upstream/records/README.md)保存每次发布与上游迁移记录。main 记录应用 SHA 可避免把应用自己的 SHA 写入自身提交造成循环；记录提交本身不改 personal 来源或发布产物。

## 下一次 KIN 迁移闭环

1. 从实际最新 personal 与 main 控制基准开始，保存完整 SHA 和工作树状态；固定旧来源/旧稳定回退 tag、目标 KIN tag object/commit、merge-base 和目标分叉。命令与字段来自记录目录，不依赖某个本地工作树。
2. 同源后代更新可复用 `.github/personal-sync/sync.py` 的候选准备。来源分叉、标签移动、控制文件变化或冲突时停止自动合并，人工审查后按模块重放；不改脚本跳过祖先/基础检查，不自动 ours/theirs、强推或删除历史。上游 refs 使用独立空间，不覆盖 fork 标签。
3. 先核对迁移身份/checksum、旧 schema/数据与应用回退边界，再按 S1 计时、S2 目录、S3 诊断、S4 状态、S5 展示/导出的依赖分小批次重放。每一模块登记冲突文件、语义取舍、实际 owner/test、变更和必要定向证据；同名 KIN 能力先按来源矩阵核实原生覆盖，再决定删薄包装或保留个人增量。
4. 基线切换采用普通 merge 保留旧 personal、目标 KIN 和已验收重放历史的真实祖先；不可把分叉的旧 KIN 错写为目标 KIN 祖先。连接 tree 和 parents 的核对单独记入迁移记录。原 SQL/迁移账本、来源和旧稳定标签保留。
5. 实际切换后，把固定个人应用的来源字段与其 NOTICE、模块矩阵和新记录核对；main 只搬运必要维护/控制增量，不合入整棵应用树。个人新功能与更新服务等未重放缺口明确登记，不能根据旧 A 的文档推断新 I 已具备。

## 定向验证、交付与发布

普通 push、PR 更新、Ready 和 full-ci 标签不触发应用 CI/安全扫描。开发按变更的实际边界选择必要本地定向检查，输入未变的成功证据复用；需要云端反馈时维护者手动选择 Quick。同步仅准备候选，只有显式手动 CI 演练才调度 Quick，不在每个任务补跑 Full。

正式发布是另一次明确授权的操作：固定最终 personal SHA → 手动 Personal CI Full → main 手动 Personal release → Release → 维护者手动 Docker 部署。发布 guard 仍要求来源/祖先、同一次手动 Full 的全部必要成功 jobs、固定 run ID、tag、可信 main 工具与 digest；普通合并不启动发布，分支 required checks 不代替发布资格。保留禁止强推/删除规则，成功旧 SHA 不能换成当前候选证据。

每次实际发布后在记录目录保存真实个人 tag/SHA、KIN tag/SHA、Plus 标签/阅读 SHA 和借鉴范围、控制 revision、同次 Full/Release run、平台与 digest、Release notes 链接。用固定 personal SHA 的 NOTICE/来源文件及 Release notes 实际源码身份交叉核对。若 Notes 没直接列出 Plus 等字段，记录中注明其来源于对应源码/NOTICE；不冒称模板已经自动输出这些字段，不在普通任务改写历史 Release。

迁移记录和候选记录不分配正式版本，也不创建 tag、运行发布 dry-run、构建镜像或访问生产。历史旧流程、保护要求和自动发布描述以其固定执行日期保存；本合同和 CI-000 定义当前手动流程。只有准备发布/迁移的明确门禁才执行相应完整验证，静态文档检查不替代这些门禁。
