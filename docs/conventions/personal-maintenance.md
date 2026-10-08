# personal 同步合同

`main` 是默认分支和调度宿主，`personal` 是 KlN 应用及 TPS 的维护分支。两条分支维护相同的 `.github/personal-sync/`、同步/Personal CI/既有 CI/安全扫描定义。搬运维护文件使用普通提交，不合并整个 main 的应用代码。

- 上游固定 `KlN-4096/sub2api` 的 `klno`，自动选择数字最大的 `vX.Y.Z-klno.N` 标签，包括 prerelease。上游标签使用 `refs/personal-sync/tags/`，不覆盖 fork 标签。解析后固定目标 SHA，标签在解析期间变化则停止。
- `deploy/personal-source.json` 沿用 `upstream_repository`、`upstream_tag`、`upstream_sha`。标签记录真实发布来源；显式未发布 SHA 的 `upstream_tag=null`，后续发布必须先明确版本归属，不能借旧标签冒充。
- 从最新远端 personal 以普通 merge 准备独立候选。来源提交、personal 基础和目标必须有正确祖先关系。冲突、上游改写、控制文件变化、同名候选内容变化及权限失败均明确停止，不自动选 ours/theirs、删除工作流差异或强推。
- 同一目标/基础重跑比较候选树并复用现有 SHA/草稿 PR。无更新不创建候选或运行构建；关闭的 PR 尊重人工处理结果。基础前进时创建新候选，旧结果不能复用。
- Personal CI 从指定完整候选 SHA checkout，dispatch ref 必须指向该 SHA，开始/结束核对实际 PR、仓库、候选及最新 personal。通过 reusable workflows 保留既有 shell、Go 单元/集成/race/lint、前端关键检查、发布 helper 和安全扫描。pnpm 9 + frozen lockfile；CI 不持久保存 Git 写入凭据。
- 机器人创建 PR 后显式 dispatch Personal CI；成功复用仅限同一候选、分支和基础。`personal-ready` 汇总所有必需检查并在结束时再次核对基础。远端 personal 的严格状态检查应要求 `personal-ready` 以及分支与最新基础同步，防止结束后基础变化仍合入旧结果。
- 同步 PR 使用 merge commit，保留 TPS 与 KlN 的祖先关系。草稿 PR 明确记录来源、基础、候选 SHA、迁移和 Actions 链接。仓库仅允许 merge commit，禁止 squash/rebase 合入，以维护同步历史。
- `verify_ci=true` 是显式流程演练：只添加候选元数据，草稿 PR 标明“无上游升级、请勿合并”。它不修改来源记录或引入上游代码，不算真实升级。定时入口永远不启用该选项。
- 默认用短期 GITHUB_TOKEN。推送、PR 创建或 dispatch 权限不足时停止；本机已有授权凭据可处理已审查候选。自动推送工作流变化需要实际 Workflows 权限，新增令牌或扩大权限须另获授权。
- 同步不创建个人发布标签、不触发 Release、不部署生产。真实升级 PR 保持待审；P3/P4 独立处理镜像和部署。

手动检查：`gh workflow run sync-upstream.yml --repo ccisnoxx/sub2api --ref main`；指定目标使用 `-f target=vX.Y.Z-klno.N`。基础变动后重新运行同步；合入前核对最新基础、候选 SHA 和 `personal-ready`，使用 `gh pr merge <编号> --repo ccisnoxx/sub2api --merge --match-head-commit <完整候选SHA>`，遵守保护规则。

运行证据、阶段任务勾选和开发日志以默认 main 的最新记录为准；personal 的固定源码保留阶段记录。每日 UTC 03:17 的入口只有仓库变量 `PERSONAL_SYNC_SCHEDULE_ENABLED=true` 才执行，部署维护定义时先保持 false，实际候选 CI、独立复核和严格基础保护验收后启用。
