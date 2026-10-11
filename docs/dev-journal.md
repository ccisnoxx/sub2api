# 开发日志

## 2026-10-11：MIG-001 迁移身份与兼容审核

- 从冻结 KIN `ca686de5e5ac14834e12701c156831abd5ec6b71` 创建隔离任务候选。生产与 S44 测试库只读核对均为固定 personal 的 307 条迁移；保留已执行 251–254 的原文件名和原 SQL 字节，没有修改 runner 或 checksum 例外。
- [完整身份清单](migrations/MIG-001-inventory.json)记录目标 304 个 SQL、旧 KIN 303 个共同 SQL 与四份个人迁移的差集；新 242 约束移除纳入升级、回退和锁影响审核。
- [MIG-001 记录](migrations/MIG-001.md)覆盖空库、旧库、已迁移库、旧应用启动、真实合成库备份恢复、锁失败重试与错误 checksum 拒绝；后续 S1–S4、personal 切换和发布部署尚未实施。
- KIN 树的 AGENTS 引用出站身份 conventions、既有 journal 和 docs/tasks，但本冻结树没有这些文件；仓库其他当前固定候选也未找到该 conventions。已读取可获取的个人历史 journal，未导入无关历史或改变出站行为。本条为该干净树的首个任务日志。

## 2026-10-11：UPSTREAM-004 结构化路由诊断迁移

- 在冻结新 KIN 与 MIG-001 上重放实际调度评估快照、HTTP/WS turn 隔离、可空持久化和管理员单记录详情边界；保留 sticky、guardian、previous response、等待与 gateway pool owner。
- [阶段报告](upstream/UPSTREAM-004.md)记录候选池/过滤事实、非法诊断不得丢失真实错误、实际 socket/WS 计费 owner、真实 PostgreSQL 和 JWT 权限测试；定向 race 与项目 `make -C backend build` 通过。
- 迁移 251–254 原名原字节、runner 和依赖未变。独立复核与固定候选阶段 PR CI / Security Scan 由父任务完成；Personal CI 与来源切换留到 UPSTREAM-007。
