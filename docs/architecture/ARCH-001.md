# ARCH-001：流式观测职责拆分

本任务按修复计划分多个小 PR 交付。第一阶段只提取 Responses 首次终态分类与冻结；协议解析、输出计时、用量来源及观测提交的后续拆分仍待完成。整体 ARCH-001 保持未完成，不把第一阶段称为一次性重写或完整五组件落地。

## 第一阶段的合同

`responsesCompletionClassifier` 是每次转发尝试或已准入 WS turn 的首次终态 owner。它不依赖 context、时钟、协议 JSON、计费或服务状态提交，零值表示尚无终态。协议或生命周期 owner 确认终态后调用 `freeze`：第一次生效，后续完成、取消或错误不重排该生命周期。`snapshot` 输出现有 completion_status/is_complete 口径；unknown 的可空值为 nil，每次返回独立 bool。

`responsesOutputTiming` 保留 mutex、AfterFunc、HTTP/SSE/WS 接线与现有 attempt/response-ID 约束；在同一锁内更新和读取 classifier。DTO 的完成字段仅在快照边界生成，不另存可变副本。协议事件分类函数整体移入组件文件，原映射语义保持；JSON、网络错误与请求最终 owner 的不同作用域没有被合并成一个无条件优先级。

终态冻结后仍处理实际接受的 drain usage。客户端取消可以与 upstream_final 用量来源同时成立；冻结不能阻断已有计费解析或猜测音频拆分。服务状态候选继续使用原始终态/时刻/原因，重试 attempt 的候选与请求最终 terminal 仍分别由既有 owner 处理。

## 验证与边界

先在重构前执行扩大后的既有边界回归：完成后错误、中断后错误、取消/完成顺序、drain 最终用量与快照隔离通过；随后提取组件并验证同一回归。新增独立组件测试覆盖 unknown/nil、首次终态、快照隔离及协议状态分类。复用既有服务状态与入口测试保护 attempt/WS turn/response-ID、失败归因和最终提交。

执行命令按变更定向选择，使用应用 go.mod 指定 Go 1.27.2、GOTOOLCHAIN=local：

```bash
# 提取前的边界回归
go test -count=1 -tags=unit -run '^TestResponsesOutputTiming_' ./internal/service

# 提取后的组件、计时和服务状态合同
go test -count=1 -tags=unit -run '^Test(ResponsesOutputTiming_|ResponsesCompletionClassifier_|ServiceStatusSource)' ./internal/service

# 取消回调、终态和快照的并发边界
go test -race -count=1 -tags=unit -run '^Test(ResponsesOutputTiming_(TerminalOrdering|TerminalFreezePreservesAcceptedUsage)|ServiceStatusSourceCancellationOrderMatchesS1)$' ./internal/service

go vet -tags=unit ./internal/service
```

以上从 backend 目录执行。实际结果、精确候选、独立审查及云端运行证据记录于本阶段 PR 和维护者修复计划。普通 PR 先运行路径相关 Quick；网关生命周期路径按 CI-000 要求在稳定候选上取得一次 Full。中间状态不反复 push 或 dispatch Full。本阶段作为 CI-000 后续观察的第一个任务记录，同一 ARCH-001 后续阶段不重复增加任务计数。

## 剩余阶段、数据库和回滚

后续分别拆分协议事实解析与业务更新、输出计时与用量来源、观测提交与计费接线；每个小 PR 按实际边界建立独立验收，不提前勾选整体任务。HTTP、SSE、WS 不同终态 owner 的语义必须保留，不能把重试失败提前提交成请求最终结果。

公开 DTO、数据库字段、迁移、用量总量解析器、认证权限、依赖和发布/部署控制不变。数据库影响 none。没有新增 goroutine；仍由原 observer 的 mutex 串行化取消回调和快照，Race 验证不等同证明所有可能交错或生产行为。

回滚通过普通 revert 本阶段提交，整体恢复原 observer 的状态所有权和相关回归；保留既有 schema、迁移账本及上游历史。不强推、不变更正式标签、不运行发布或部署。
