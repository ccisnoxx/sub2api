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
| Chat Completions、Anthropic、Gemini/Antigravity、Grok 转换 | 原转换 owner | 原用量/扣费 | **第一批未接入**；当时 CC→Responses fallback 定向断言 version 0。本轮 CC/Responses/Grok 桥增量见下方 S1.5.1；Anthropic/Gemini 和独立媒体入口仍待后续任务 |

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

## S1.5.1 Chat Completions 与转换链路

2026-10-10 本地候选 `1ae266791d0ab324cd13a63ab344019c79102176`，基于 personal `3d5e1fde82707900a21f2b5538b112c7bab3c04a`。第一批表格仍描述其固定候选；以下是本轮新增覆盖，不代表 S1.5.2–S1.5.5 已完成。

| 实际入口 / owner | 本轮采集与终态 | 用量归属及可观察边界 |
|---|---|---|
| `ForwardAsChatCompletions` → raw CC `streamRawChatCompletions` / `bufferRawChatCompletions` | 沿用 `startTime`；SSE 观察真实增量，JSON 在完整 body 读到时观察内容；按实际请求 n 和各 choice 的 finish_reason 判断 | 观察同一原 parser 接受的 CC usage；原 Token、透传内容、返回错误与结算保持；HTTP upstream fixture 验证 JSON/SSE |
| Responses 入站 → CC 上游 → Responses 输出 `streamChatCompletionsAsResponses` / `bufferChatCompletionsAsResponses` | 观察原始 CC，合成的 Responses completed 不成为新终态事实；有效 reasoning/tool 参数纳入采集 | CC final/partial 随实际接受点记录；EOF/读错/上游失败及取消后 drain 的输出与用量来源分开 |
| CC 入站 → Responses 上游 → CC 输出 `handleChatStreamingResponse` / `handleChatBufferedStreamingResponse` | 使用真实 Responses 事件；非流式只在最终重建的完整内容 ready 时形成首/末时点 | typed terminal 与通用 parser 的原接受顺序保持；nested/top usage、零用量、音频别名及缺失拆分测试 |
| Grok CC ↔ Responses 桥 | 复用上述真实 CC/Responses owner，JSON/SSE 断言 `timing_version=1`；不扩大到独立 Grok Responses/媒体 producer | 已有 Grok 桥回归；Composer 多次上游用量合计时无法证明单一 final 来源，保持 source unknown / audio NULL，原 Token 合计不变 |
| 共享 CC scan / JSON reader 与 `readOpenAICompatBufferedTerminal` | 新 observer 可选；旧 Messages caller 不传入 | CC→Anthropic 与 Responses→Anthropic buffered 仍为 version 0，旧读取/usage/error 合同定向验证 |
| handler → `OpenAIForwardResult` → `RecordUsage` → SQL/DTO → 页面/导出 | 新 result 快照复用既有两次 `UsageTiming.Clone()`；不改下游 owner | SQL、DTO、前端/导出输入与历史证据已核验；本轮没有新增迁移、字段、页面或失败用量行 |

- 非空 text/refusal、reasoning/reasoning_content、function arguments/custom input 可形成严格首/末；role、工具名称、metadata、空 delta、usage 与 `[DONE]` 不形成输出。音频/图片真实内容只提供对应首次输出事实。
- 全部已请求 choice 的 stop/tool_calls/function_call 才判 completed；length/content_filter 为 interrupted；缺失或无法识别终态不猜成功。第一次终态冻结；cancel/write failure 后 drain 可取得 final usage，状态仍为 client_disconnected。
- 音频拆分来自同一个实际接受的 usage 对象；最终对象整体替换部分对象，缺失或非法统计不沿用旧数值。仅能证明纯文本且从未观察到音频/未知输出时填零，其余保持 NULL。
- 同一服务先失败再成功的转发尝试不串状态；仅 usage 不补造首/末。旧 `first_token_ms`、Token、计费、调度、重试与错误返回保持。
- 未接入：原生 Anthropic（S1.5.2）、Gemini/Antigravity 与实际媒体入口（S1.5.3）、Cyber 独立入账、probe/gwpool/Live 及不复用本轮 owner 的独立 Grok 媒体/Responses 路径。原版本或未知值保持。

本轮验证是协议/服务 owner 与 HTTP upstream fixture，未执行真实供应商付费请求、新数据库/JWT/浏览器矩阵、完整回归或新远端 CI。未变下游证据按原边界复用，不能据此宣称所有平台或第二批交付完成。详见[执行证据](implementation-evidence.md#s151-chat-completions-与转换链路)、[验证清单](evidence/s1.5.1-validation.json)及[复核](evidence/s1.5.1-reviews.json)。

## S1.5.4 第二批差异验证与不适用范围

2026-10-10 用户明确不需要 Anthropic、Gemini/Antigravity 和独立媒体入口。S1.5.2、S1.5.3为**不适用**，不表示实现或验证通过；此前S1.5.1的“留待后续”为历史状态，现由本次范围决定取代。本轮只验证应用 `1ae266791d0ab324cd13a63ab344019c79102176`，没有新增源码或扩大入口。

| 范围 | 本轮状态与证据 | 边界 |
|---|---|---|
| raw CC JSON/SSE、Responses→CC→Responses、CC→Responses→CC | S1.5.4验证完成；最终源码/原日志核验，复用协议/终态/usage/audio/attempt测试 | 真实协议事件及实际接受的usage决定新快照；合成结束或HTTP成功不代替真实终态 |
| 复用上述owner的Grok桥 | S1.5.4验证完成；复用实际Forward及桥测试 | 独立Grok Responses/媒体producer未接入；Composer聚合来源unknown、音频NULL |
| handler→异步用量→RecordUsage→SQL/DTO→页面/导出 | owner不变；独立核对新快照与既有链路衔接，复用未变边界证据 | 旧Token/费用/幂等键/用量行创建条件保持；旧SQL/页面证据只证明未变下游 |
| Anthropic直连/透传/转换适配（S1.5.2） | **不适用：用户不需要** | 沿用旧计时或未知；共享reader兼容测试不代表新采集已接入 |
| Gemini/Antigravity及独立媒体入口（S1.5.3） | **不适用：用户不需要** | 沿用旧口径；所选CC协议内可识别的媒体输出/audio拆分仍按S1.5.1合同判断 |
| Cyber独立入账、probe/gwpool/Live及不复用本轮owner的其他入口 | 本轮未支持，保留原覆盖限制 | 不登记为新采集完成；本次范围决定不删除已有功能 |

修正后受影响链复用最终race9顶层/69含子例通过证据；较早124/271、20/115、3/9仅用于未受音频修正影响的边界，计数不相加。本轮没有新跑测试、供应商付费请求、新数据库/JWT/浏览器或完整CI。S1.5.5尚未执行，不宣称第二批交付完成。见[执行证据](implementation-evidence.md#s154-第二批差异验证与独立复核)、[复用清单](evidence/s1.5.4-reused-evidence.json)、[验证](evidence/s1.5.4-validation.json)和[复核](evidence/s1.5.4-reviews.json)。

## S1.5.5 当前所选入口与交付状态

固定应用 `1ae266791d0ab324cd13a63ab344019c79102176`，personal基础 `3d5e1fde82707900a21f2b5538b112c7bab3c04a`。下表汇总累计覆盖及本轮交付边界；旧第一批/S1.5.1/S1.5.4表格保留其当轮事实。当前候选尚未合入personal、未发布或部署，S4.4生产记录不能证明新增CC链路线上生效。

| 实际入口 / owner | 当前采集覆盖 | 验证与交付状态 |
|---|---|---|
| 原生OpenAI Responses HTTP/SSE、WS逻辑turn | 第一批UsageTiming v1；当前候选继承 | 原S1/S4.4有效证据按未变边界复用；生产仅有已登记的原Responses范围，本轮未重查生产 |
| raw CC JSON/SSE：streamRawChatCompletions / bufferRawChatCompletions | UsageTiming v1，真实内容/choice终态/接受usage；JSON为完整内容观察时点 | S1.5.1实现、S1.5.4验证复核完成；本地HTTP upstream fixture，未取得本候选新远端CI或付费供应商验收 |
| Responses→CC→Responses：streamChatCompletionsAsResponses / bufferChatCompletionsAsResponses | 观察原CC事件，合成completed不替代原终态 | S1.5.1/S1.5.4有效协议/取消/drain/用量归属证据复用 |
| CC→Responses→CC：handleChatStreamingResponse / handleChatBufferedStreamingResponse | 观察真实Responses事件；buffer以最终重建完整内容时点采集 | 音频修正后受影响范围race9顶层/69含子例通过证据复用，最终缺拆分保持NULL |
| 复用上述owner的Grok桥 | 跟随所复用owner采集；Composer多上游聚合source unknown、audio NULL | 实际桥测试与独立复核有效；不扩大为独立Grok Responses/媒体producer完成 |
| 所选CC链路内部已可识别的媒体内容及audio拆分 | 按既有首次媒体输出和可信usage合同；未知不填零 | 包含在所选协议测试边界，独立媒体入口未接入 |
| handler→异步Clone→RecordUsage→SQL/DTO→页面/导出 | 复用第一批下游合同和历史行未知语义 | 相关输入不变、原证据复用；本轮无新增CC到SQL/页面完整E2E |
| Anthropic原生/直连/透传/转换（S1.5.2） | **不适用**，保留旧计时或未知 | 用户明确不需要；共享reader兼容测试不表示新采集已接入 |
| Gemini/Antigravity与独立媒体入口（S1.5.3） | **不适用**，保留旧计时或未知 | 用户明确不需要，不记作实现通过 |
| Cyber独立入账、probe/gwpool/Live、不复用本轮owner的其他producer | 本轮未支持，原覆盖限制保持 | 不升级为v1声明，不混入所选文本TPS/真实完成状态的覆盖结论 |

S1.5所选入口本地实现/验证/交付记录完成；远端候选门禁、镜像与生产状态仍独立待取得。新候选没有新增迁移，但现有部署工具对12个新运行路径正反向均按合同拒绝，不能沿用S4.4证明或旧rollback ID。详见[交付与回退](delivery.md#s155-第二批本地交付记录)、[门禁与回退检查](evidence/s1.5.5-gates-and-rollback.json)、[验证](evidence/s1.5.5-validation.json)。

## S1.5 候选远端门禁补充

当前应用仍为 `1ae266791d0ab324cd13a63ab344019c79102176`；[草稿PR #8](https://github.com/ccisnoxx/sub2api/pull/8) 与 [Personal CI 38065981816](https://github.com/ccisnoxx/sub2api/actions/runs/38065981816) 对同一候选、基础 `3d5e1fde82707900a21f2b5538b112c7bab3c04a` 全部11项通过。上方S1.5.5表格中的“未取得新远端CI”是本地记录完成时快照，本节更新其远端状态：raw CC、双向转换及复用Grok桥已取得现行必要候选CI，不扩大已有协议/owner覆盖。原本地fixture、最终音频race、未变下游及S1.5.4独立复核继续有效；没有新增付费供应商、CC→worker→SQL→页面整体E2E或生产验收。

Anthropic、Gemini/Antigravity及独立媒体仍不适用；本PR未合入、未发布、未部署。新运行树的部署/回退审定和既有覆盖限制保持。见[远端门禁证据](evidence/s1.5.5-remote-gates.json)与[复用清单](evidence/s1.5.5-remote-reused-evidence.json)。
