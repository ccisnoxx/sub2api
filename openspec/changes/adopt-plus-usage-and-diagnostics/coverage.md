# S1.1–S1.4 第一批实际覆盖

固定应用候选：`3f04437572e2819f0313ccc2a3f1a618a2afcdf0`；合同见 [spec.md](spec.md)。该表描述已接线且定向验证的入口，不将数据库有列或页面有组件视为全部平台已采集。

| 实际入口 / owner | 起点与归属 | 终态 / 用量提交 | 第一批范围与证据 |
|---|---|---|---|
| `handleStreamingResponseWithReasoning` | 既有 `startTime`；每次处理新 observer | 原 Responses 协议事件；`Forward` → `OpenAIForwardResult` → `RecordUsage` | 普通 HTTP/SSE；有效 delta/added、媒体首输出、终态冻结、缺失终态；HTTP wire 定向测试 |
| `handleStreamingResponsePassthrough` | 既有 `startTime`；每次处理新 observer | 同上；保留透传、drain、错误返回合同 | HTTP 透传 SSE；保持旧 `FirstTokenMs`、Token 与错误处理 |
| `handleNonStreamingResponse` / `handleNonStreamingResponsePassthrough` | 既有转发起点；完整 body 读到时观察内容 | 完整 Responses JSON 与既有 parser 接受的 usage | 普通/透传 JSON；时点表示完整内容观察时刻，不是上游生成首 Token |
| `handleSSEToJSON` / `handlePassthroughSSEToJSON` | 既有转发起点；缓冲 body 的完整内容观察时刻 | 原最终结果 / 用量 owner | SSE→JSON；无新重试，不提前声称流式首 Token |
| `forwardOpenAIWSV2`（HTTP→pooled WS） | 既有 `startTime`、当前 response ID | 原 WS 结果与 `RecordUsage` | 流式实际事件；非流式完整结果；取消、成功/失败 owner 保持 |
| `sendAndRelay`（WS pooled ingress） | 既有 `turnStart`；每个已准入 turn 独立 | 原 hooks / 用量提交 | 同连接多 turn、媒体/工具与完成后关闭；既有 lease 不变 |
| `proxyOpenAIWSHTTPBridgeTurn`（WS→HTTP） | 既有 `turnStart`；当前 response ID | 原 SSE/HTTP 结果与 hooks | 排除其他 ID；既有 retry 成功及 rejection 检查复用 |
| `proxyResponsesWebSocketV2Passthrough` / `openai_ws_v2.Relay` | relay 权威 admitted turn / response ID / startAt | 同步只读 `OnUpstreamEvent` → 每 ID observer → `OnTurnComplete` 快照 | 真实本地 WS 两 turn、交错 ID、完成/取消 race；callback 带原 parser 接受的 usage 原文，不另选计费来源 |
| `OpenAIGatewayService.RecordUsage` | 只消费结果快照，不再计时 | `UsageTiming.Clone()` → 原异步 worker / repository | 深复制所有可空字段；legacy/atomic × balance/subscription 费用、倍率、扣费次数、原 atomic command 保持 |
| Cyber 独立入账、probe / gwpool / Live | 原 owner，不具备本期可验证的 `ForwardResult` 关系 | 原提交合同 | **未接入**：version 0 / NULL / unknown，页面不将其当新文本 TPS |
| Chat Completions、Anthropic、Gemini/Antigravity、Grok 转换 | 原转换 owner | 原用量/扣费 | **S1.5 未执行**；CC→Responses fallback 定向断言 version 0。媒体 API 独立入口不在本期声明内 |

## 输出与终态事实

- 非空 text/reasoning、function/custom/MCP 工具参数、code interpreter 代码、shell command/stdout/stderr delta/added 和原生 `shell_call.action.commands` 的非空内容 可推进严格首/末；工具名称、metadata、usage、心跳、空 delta、签名及结束标记不推进。
- 流式只有 text/tool `.done` 或 terminal aggregate，没有有效 delta/added 时，严格首/末保持 NULL；媒体/compaction 的真实完整输出可以形成首次输出。compaction 完整 JSON 回显的历史 message/tool 不算本次新输出。
- 音频拆分只接受可信非负整数；已接受的正音频统计也属于本轮持续音频事实，最终缺失拆分不保留部分数值也不猜零；无音频须由可确认的最终输出证明，音频 transcript/done 已出现时不能因最终空 output 猜零。
- 第一次已观察终态冻结：cancel→completed 为 client_disconnected；completed→close 保留 completed；failed→DONE 保留 upstream_error。有协议事实但无终态为 interrupted，缺少事实为 unknown。
- HTTP 普通/透传 Forward 的失败原本丢弃 partial result，本期保持该返回合同；内部失败事实已测试，但**没有新增失败用量行**。
- 原生 WS 未结算 turn、零终态 fallback、opaque binary frame 无法确认 turn/ID/起点时，保留旧版本或未知。本期没有扩展 relay lifecycle 以补造用量记录。

## 页面与导出

管理员与用户复用 `UsageTable`、`UsageFirstToken`、`UsageTps` 和 `usageTiming.ts`。新行显示首 Token、首次输出/类型、末次 Token、完成状态与来源，旧行保留旧首字；部分结果仍可计算平均 TPS 并注明部分响应。CSV/Excel 保留所有旧列，尾部统一增加 12 列；原始时点、未舍入 TPS、NULL、bool false 和 unknown 已核对。

本地 Chromium 合成 API 验收覆盖管理员/用户、中文/英文、明暗主题、1440/390px、点击和 Enter/Space/Escape、说明互斥与外部关闭；实际下载四份导出共 32 行并解析核对。该证据验证页面流程，不证明真实用户授权隔离或真实供应商行为。没有执行生产请求或跨浏览器矩阵。

非流式经 pooled WS 的旧 DTO 仍规范为 ws_v2/stream=true，当前合同不保存观察模式。所有新计时详情统一说明非流式完整内容观察边界，不改变旧请求类型；ws_v2 说明回归与四组更新弹层边界已验证。JSON 的 completed 或成功 compaction 才提供 final 来源证据，未完成状态为 partial，状态缺失/无法识别为 unknown。
