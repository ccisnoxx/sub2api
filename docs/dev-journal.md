# 开发日志

## 2026-10-11：MIG-001 迁移身份与兼容审核

- 从冻结 KIN `ca686de5e5ac14834e12701c156831abd5ec6b71` 创建隔离任务候选。生产与 S44 测试库只读核对均为固定 personal 的 307 条迁移；保留已执行 251–254 的原文件名和原 SQL 字节，没有修改 runner 或 checksum 例外。
- [完整身份清单](migrations/MIG-001-inventory.json)记录目标 304 个 SQL、旧 KIN 303 个共同 SQL 与四份个人迁移的差集；新 242 约束移除纳入升级、回退和锁影响审核。
- [MIG-001 记录](migrations/MIG-001.md)覆盖空库、旧库、已迁移库、旧应用启动、真实合成库备份恢复、锁失败重试与错误 checksum 拒绝；后续 S1–S4、personal 切换和发布部署尚未实施。
- KIN 树的 AGENTS 引用出站身份 conventions、既有 journal 和 docs/tasks，但本冻结树没有这些文件；仓库其他当前固定候选也未找到该 conventions。已读取可获取的个人历史 journal，未导入无关历史或改变出站行为。本条为该干净树的首个任务日志。

## 2026-10-11：UPSTREAM-002 用量计时迁移

- 在冻结新 KIN 与已验收 MIG-001 之上重放原生 Responses 的严格首输出、末次 Token、完成状态、音频与用量来源；保留旧首字、计费/幂等、重试和异步 owner。
- [阶段报告](upstream/UPSTREAM-002.md)登记 HTTP/SSE/透传、pooled WS 多 turn、WS→HTTP bridge、取消/未知边界，真实 SQL/Ent 集成和观察器/async race 的实际证据。包装 usage 与双 usage 音频缺失保持原 parser 选择和 NULL 语义。
- 没有改迁移 251–254、出站身份、前端或发布控制面。维护者授权阶段标准 PR CI 与 Security Scan；最终 Personal CI/来源切换保留到 UPSTREAM-007。独立复核和远端固定候选结果以该阶段 PR 记录为准。

- 首轮阶段 CI 发现旧用量 API fixture 未包含九个新增观测字段，补齐历史行的 NULL/unknown/version 0 预期；定向 API 合同 12 项通过。独立只读复核运行实现未确认阻断，最终阶段门禁绑定后续提交。
