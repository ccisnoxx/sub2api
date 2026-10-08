# P0/P1 实施证据

- 实施日期：2026-10-08（America/Los_Angeles）。
- 范围：本地 P0 基线准备与 P1 平均输出 TPS；GitHub 设置、推送、镜像发布及生产部署未授权在本轮执行。
- 开始状态：`main`，HEAD `09d17249b2e80fbce6b7cfc052f7815559fe815d`；仅本 change 的 plan.md 与 tasks.md 未跟踪，无其他工作树改动，只有当前一个 worktree。
- 适用规则：用户提供的全局 AGENTS.md 与 `/Users/sc/.codex/AGENTS.md`；仓库及其父项目目录无额外 AGENTS.md。

## P0 基线核对（进行中）

- origin：`https://github.com/ccisnoxx/sub2api.git`；upstream：`https://github.com/KlN-4096/sub2api.git`。
- 本地 `v0.2.14-klno.3^{commit}` 与远端 `git ls-remote upstream` 的解引用均为 `de08df02ae1d81668a22f798b398aa0438ac1276`；标签对象为 `e14d8e4f9ad34dbd9c10918f5408a47500360362`。
- main 与发布标签相差 528 个文件，个人应用不得从 main 源码直接发布。
- GitHub 只读查询：默认分支 main；Actions 仓库级 enabled=true，但 workflow 列表与 run 列表均为 0；因此没有已注册、正在运行的旧同步 workflow。未修改任何 GitHub 设置。
- 本地 sync-upstream.yml 替换为只读、仅手动显示暂停说明的占位；去掉定时、rebase、强推、自动标签、Release dispatch 与重建 main。远端文件未推送，不能认定远端定义已替换。
- hostdzire：现有别名 SSH 连接超时；独立 TCP 2222 检查也超时，路由为当前 en0 出口。尚未读取部署 revision/digest/Compose/挂载，P0.3/P0.4 不得勾选。已请求恢复连接或提供可信部署证据。本轮未修改生产服务。
- 在未取得部署证据前，发布标签只能作为已验证的源码候选基线；不得用原会话服务器信息代替本轮证据。

## 后续登记

P1 实现、定向检查、页面验收、分支和最终提交结果将在实际执行后追加。
