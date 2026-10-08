# 开发日志

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
