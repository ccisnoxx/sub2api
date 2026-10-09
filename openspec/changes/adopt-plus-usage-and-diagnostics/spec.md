# S1 第一批冻结合同

本合同适用于 `codex/plus-usage-s1`，应用从 personal `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92` 出发。固定 Plus 参考为 `90da415c62b94c9417d9ce2b72b1507ed22f0303`，按 KIN 现有计费、旧首字与状态归属作本地适配。第一批只声明原生 OpenAI Responses HTTP/SSE 与 WS；转换入口和其他平台属于 S1.5。

## 数据与历史

| 字段 | 类型 | 历史/未采集 | 合同 |
|---|---|---|---|
| first_token_ms | 既有可空整数 | 沿现状 | 旧首字与 semantic/visible 设置不变 |
| timing_version | int16 / SMALLINT | 0 | 1 表示本行由第一批观察器采集，不是全局开关 |
| strict_first_token_ms | 可空整数 | NULL | 首个有效文本/推理/工具输出距既有转发起点的毫秒数 |
| last_token_ms | 可空整数 | NULL | 同一起点下最后一个有效 Token 类输出的时点 |
| first_output_ms | 可空整数 | NULL | 首个有效输出，包括可识别媒体 |
| first_output_kind | 可空字符串 | NULL | text / reasoning / tool / image / audio / compaction |
| audio_output_tokens | 可空整数 | NULL | 只记录可信拆分；确认无音频才为 0 |
| completion_status | 字符串 | unknown | completed / client_disconnected / upstream_error / interrupted / unknown |
| is_complete | 可空布尔 | NULL | 已观察终态派生；unknown 不等于 true |
| usage_source | 字符串 | unknown | upstream_final / upstream_partial / unknown；描述既有计費 Token 来源，不估算 |

领域快照为 `UsageTiming` 值，提交用量前深复制可空字段；异步 worker 只消费快照。只有空字符串规范为 unknown，不用默认值修复未知枚举。新迁移身份为 `251_add_usage_log_timing.sql`，仅加字段/约束；不修改旧迁移、不删除设置/聚合、不回填历史时点。

## 事件与生命周期

- 复用当前 `duration_ms` 的转发/turn 起点，不加入排队或鉴权时间。HTTP 重试创建新的观察器；WS 以已准入 turn 与 response ID 隔离，不复用上一轮的可变状态。
- 非空文本、推理与真实工具输出推进严格首/末时点；metadata、心跳、usage、空 delta、签名、结束标记不推进。媒体输出只影响首次输出详情；媒体之后有真实文本时可建立严格时点。
- 非流式的时点是完整响应被观察的时点，不表示上游第一个生成 Token。流式终态聚合内容不能补造首 Token。
- owner 在协议事件与客户端输出生命周期中冻结终态；完成后关闭不抹掉 completed。取消早于完成时保留 client_disconnected；协议错误不被随后的结束标记改为成功；缺失可信终态的 EOF/传输中断为 interrupted。
- 没有既有用量行的失败不新增计费或成功用量行；已有独立 Cyber/探测/gwpool/Live 入口保持原版本，不能由本期采集覆盖声明推断其已接入。

可测试序列：metadata→空 delta→text→completed；image→reasoning→completed；text→failed；text→EOF；text→cancel→completed；text→completed→close；retry attempt A 失败→attempt B 成功；同 WS 连接 turn A/B 与不同 response ID 的交错输出。结果应分别保护时点、首次类型、部分终态、完成冻结和归属隔离。

## TPS、页面与导出

平均 TPS = `(output_tokens - image_output_tokens - audio_output_tokens) * 1000 / duration_ms`。不扣首字、不用首末间隔替换分母；非有限/负数、媒体拆分大于总输出、非正耗时或媒体统计未知时给出不可用原因。有效部分响应可显示 TPS，并显示部分说明。历史明确普通文本继续既有适用范围；新记录未知音频不当作已验证的零。Live、probe、gwpool、纯媒体与无文本依据的 compaction 不显示文本 TPS。

新行显示“首 Token”，旧行继续“首字（旧口径）”；TPS 与首字分别使用点击圆圈按钮，保留 Enter/Space、Escape/外部关闭和弹层互斥。数值及标签没有原生 title。管理员与用户页面、Excel/CSV 共用 `usageTiming.ts`；保留旧首字列语义，新增列导出原始毫秒与未舍入 TPS，NULL 保持空值、状态未知保留 unknown。

## 交付边界

本次不合并、不推送、不分配版本、不发布镜像、不部署生产。候选本地通过与远端 Personal CI、镜像发布及线上验收分别登记。执行证据包含真实 SQL 读写、旧应用扩展 schema 兼容检查、合成协议事件、定向计费/幂等与前端/浏览器/导出检查，以及 fresh 独立只读复核。

参考：[Plus timing 合同](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/docs/USAGE_TIMING.md)、[Plus helper](https://github.com/LuckyKuang/sub2api-plus/blob/90da415c62b94c9417d9ce2b72b1507ed22f0303/frontend/src/utils/usageTiming.ts)。KIN 保留旧字段/设置、历史未知和原结算 owner，不复制 Plus 的历史清理及全平台采集声明。
