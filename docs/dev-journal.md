# 开发日志

## 2026-10-11：MIG-001 迁移身份与兼容审核

- 从冻结 KIN `ca686de5e5ac14834e12701c156831abd5ec6b71` 创建隔离任务候选。生产与 S44 测试库只读核对均为固定 personal 的 307 条迁移；保留已执行 251–254 的原文件名和原 SQL 字节，没有修改 runner 或 checksum 例外。
- [完整身份清单](migrations/MIG-001-inventory.json)记录目标 304 个 SQL、旧 KIN 303 个共同 SQL 与四份个人迁移的差集；新 242 约束移除纳入升级、回退和锁影响审核。
- [MIG-001 记录](migrations/MIG-001.md)覆盖空库、旧库、已迁移库、旧应用启动、真实合成库备份恢复、锁失败重试与错误 checksum 拒绝；后续 S1–S4、personal 切换和发布部署尚未实施。
- KIN 树的 AGENTS 引用出站身份 conventions、既有 journal 和 docs/tasks，但本冻结树没有这些文件；仓库其他当前固定候选也未找到该 conventions。已读取可获取的个人历史 journal，未导入无关历史或改变出站行为。本条为该干净树的首个任务日志。
