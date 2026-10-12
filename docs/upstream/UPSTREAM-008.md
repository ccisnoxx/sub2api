# UPSTREAM-008：个人更新服务与版本徽章迁回

## 范围与来源

- 应用基线：`origin/personal` `bea1db12d5f4853c0b85b44840e15de6a74309b1`；任务分支 `codex/upstream-008-personal-update`。
- 个人合同来源：旧个人基线 `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 的更新服务、徽章及测试。当前 KIN 更新实现和系统操作锁、幂等、HTTP 下载期限保持。
- `REMEDIATION_PLAN.md` 新增独立 `UPSTREAM-008`，不将既有迁回验收写成已经覆盖本缺口。

## 恢复行为

个人 `X.Y.Z-klno.N-tps.N` 构建使用 `ccisnoxx/sub2api` 的最近 100 条 Release，允许已发布 prerelease，排除 draft 与非个人标签，依次按基础版本、`klno`、`tps` 数值比较。原版上游仍仅只读提示，普通 KIN release 保留原地更新合同。

后端显式返回 `release_repository` 与 `update_mode`。个人 release 为 `container`，源码为 `manual`；更新、本地备份回退、指定版本回退及在线回退列表在网络查询或文件操作前返回 `IN_PLACE_UPDATE_NOT_SUPPORTED`。个人容器只展示 Release 链接和固定 digest 部署提示；源码使用手动更新说明。

缓存保存来源仓库，拒绝旧缓存及其他来源缓存，升级重启后按当前版本重算。版本徽章显示基础版本和个人后缀，检查未完成、服务端 warning 或请求失败均不会显示“已是最新版本”或确认的更新指示。保留普通 KIN 更新成功后的重启入口。

## 定向验证（2026-10-12）

- `go test -tags=unit ./internal/service -run '^Test(UpdateService|CompareVersions)' -count=1 -timeout=60s`：27 项通过。
- pnpm 9.15.9 冻结锁文件、离线安装成功；首次默认 pnpm 11.25.0 因 overrides 配置不兼容退出，未修改依赖或锁文件。
- Vitest：`VersionBadge.spec.ts`、`app.version.spec.ts`、`admin.system.rollback.spec.ts`、`localeKeyCompleteness.spec.ts`，4 文件 18 项通过。
- `go test -tags=unit ./internal/handler/admin -run '^TestSystemHandler' -count=1 -timeout=60s`：通过，包含真实 UpdateService 的 3 类构建 × 4 类操作共 12 个 HTTP 拒绝场景；同时验证既有 handler 合同。
- `vue-tsc --noEmit` 与 7 个受影响前端文件的定向 ESLint 通过；`git diff --check` 通过。
- 既有 Browserslist 数据过期提示保留，本任务没有更新依赖。

## 独立复核与修正

首次独立只读 critical_reviewer 对应用候选 `acf05d1fdae3928658c17ccc8db20c7ea77e170e` 未确认发布阻断，确认 P2：在线回退列表把安装能力拒绝序列化为 500，丢失结构化 reason。修正为 `response.ErrorFrom`，与更新/回退一致返回 409 和 `IN_PLACE_UPDATE_NOT_SUPPORTED`；新增真实服务 HTTP 断言，空客户端确保在网络查询前拒绝。未重复运行未受影响的更新服务/前端成功检查。首次复核未纳入该修正，最终增量另行只读复核。

## 边界与回滚

没有数据库、锁文件、发布工作流或部署配置变更。没有触发云端 Quick/Full、安全扫描、Release dry-run、发布标签、镜像推送或部署。测试中的 GitHub 与版本响应使用确定性夹具，没有验收真实镜像拉取、安装、登录或生产数据兼容。按新流程，发布前仍需对最终 personal SHA 手动运行 Full 并完成发布与部署验收。

回退本任务应用提交可恢复迁回前行为，但会重新暴露错误来源、缺失安装能力限制与个人序号比较的缺口。新增响应字段须前后端配套交付；旧缓存会失效并重新查询。Release 查询沿用现有客户端单页 100 条的上限。
