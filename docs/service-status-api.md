# 服务状态 API 合同

服务状态**仅代表本部署已观察请求**，不代表供应商全局可用性或 SLA。它基于已有 usage/Ops 终态证据，没有主动探测供应商；未经过本部署的请求不会进入结论。管理员界面使用本合同和后端摘要，不根据平台合计成功率重新判定全站健康。

## 入口与停用

三个入口均要求管理员 JWT：

| 方法与路径 | 作用 |
| --- | --- |
| `GET /api/v1/admin/service-status/config` | 读取独立配置；停用后仍可读取 |
| `PUT /api/v1/admin/service-status/config` | 提交完整配置及 version；冲突返回 `409 / SERVICE_STATUS_CONFIG_CONFLICT` |
| `GET /api/v1/admin/service-status/snapshot` | 当前窗口、平台、分组/请求模型、历史及事件；查询参数仅支持单个 `range=24h\|7d\|30d` 和可选 `platform` |

默认 range 为 `24h`。响应沿用项目的 `{code, message, data}` 包装；下述字段属于 `data`。

`disabled` 是配置状态：`enabled=false` 时，快照入口返回 **`503 / SERVICE_STATUS_DISABLED`**，没有健康快照。前端保留配置、清空已展示快照并明确显示“监控已停用”。它与启用后返回 `health=unknown` 的 `200` 响应不同；不新增 `health=disabled` 的成功响应。

数据库读取失败返回 `503 / SERVICE_STATUS_UNAVAILABLE`，前端显示读取失败并清空当前健康结论。已持久化的源错误、聚合错误或水位缺口则可以返回有明确 reason 的 unknown 快照。503 不是供应商 outage。

## 状态、计数与比例

当前叶子窗口为成功聚合水位之前的五分钟；历史按所选范围分桶。`success`、`failure`、`excluded`、`unknown` 保留不同结果口径，`qualified = success + failure`。

| 状态 | 叶子含义 | 管理界面 |
| --- | --- | --- |
| `unknown` | 无请求、无合格请求、样本不足、终态未知、覆盖缺口、源错误，或未结束事故等待新证据 | 灰色；比例不可用 |
| `operational` | 无未知/缺口、合格样本达到 minimum_samples、失败率严格低于 warning_error_rate，且没有未确认恢复的事故 | 绿色“已观察样本正常” |
| `degraded` | 足够样本、失败率达到 warning_error_rate 且低于 outage_error_rate | 琥珀色“服务异常” |
| `outage` | 足够样本、失败率达到 outage_error_rate | 红色“服务中断” |
| `recovering` | 当前足量样本正常，但事故的新证据恢复确认次数尚未达到 recovery_windows | 琥珀色“恢复观察”；事件仍未结束 |
| `disabled` | 独立功能关闭；通过配置和 503 表达 | 明确停用提示；无快照 |

有效比例为 `error_rate = failure / qualified`、`success_rate = 1 - error_rate`。排除样本不计入分母，也不补足最低样本；任何 unknown 样本或缺口均优先于正常比例。unknown 叶子、不可判定的平台合计及不可判定的历史桶，其两个比例均为 **JSON `null`**，不能补成 0% 或 100%。恢复观察可以显示实际已观察比例，但不能据此宣称事故已恢复。

平台和全局 `health` 是叶子摘要：全部叶子 outage 才为 outage；任一叶子 degraded/outage 则为 degraded；其余有未知或等待证据则为 unknown；其余有恢复观察则为 recovering；其余为 operational。空范围为 unknown。全局水位/源错误等缺口覆盖摘要结论，因此不能用高流量正常叶子抵消低流量未知范围。

## unknown_reason 与 gaps

`unknown_reason` 描述当前不可判定原因；全局 `gaps` 列出数据完整性提示。叶子具体原因与摘要原因可以不同：当某叶子缺少请求或终态证据时，摘要通常使用 `awaiting_data`；空叶子集合使用 `no_recent_requests`。前端为以下所有码提供中英文说明，未来未知码使用通用未知提示并灰显。

| reason code | 含义 |
| --- | --- |
| `no_recent_requests` | 当前窗口零请求 |
| `no_qualified_requests` | 仅有 excluded 样本，qualified 为零 |
| `insufficient_samples` | 合格样本少于 minimum_samples |
| `terminal_unknown` | 包含未知终态样本 |
| `watermark_missing` | 尚无成功聚合水位 |
| `invalid_watermark` | 水位在未来，无法使用 |
| `watermark_stale` | 水位距当前时间超过 180 秒 |
| `configuration_changed` | 水位配置版本与当前配置不一致 |
| `aggregation_error` | 最近一次聚合留下错误码 |
| `source_error` | 当前窗口涉及源记录写入/读取或完整性错误 |
| `scope_unavailable` | 请求无法归属有效平台、active 分组或请求模型；当前范围的此类事实形成全局缺口 |
| `coverage_gap` | 该窗口未完整覆盖 |
| `linkage_unavailable` | 源记录缺少有效 observation 关联元数据 |
| `terminal_missing` | 没有可信终态；attempt 不能证明最终结果 |
| `terminal_conflict` | 关联终态、turn、归属或完成标志冲突 |
| `unsupported_entry` | 当前入口没有支持的终态证据 |
| `unclassified` | 终态原因无法分类 |
| `awaiting_data` | 摘要仍有未知范围，或事故尚无可确认恢复的新证据 |
| `no_enabled_platforms` | 功能已启用，但配置或平台筛选的监控范围为空 |

全局水位检查按 missing → future → stale → configuration_changed → aggregation_error → source_error 的顺序选用原因；事实/覆盖缺口与计数解释遵循现有仓储和 `ServiceStatusEvaluate`。历史桶使用各桶自身的事实与覆盖，当前瞬时错误不改写已观察的历史比例。

## 事故与请求原因分类

事故阶段为 `detected`、`ongoing`、`recovering`、`awaiting_data`、`resolved`。达到 abnormal_windows 个连续、带新合格请求证据的异常观察后检测事故；重复扫描不增加确认次数。只有达到 recovery_windows 个连续、带事故最后异常之后的新成功请求证据的正常观察，才进入 resolved。无请求、unknown、低样本或覆盖缺口会转为 awaiting_data 并重置恢复确认；时间流逝和旧成功样本不会证明恢复。停用/重新启用和配置区间变化不能沿用旧恢复计数。

源 observation 的 `reason_code` 与上述快照 reason 不同。终态为 upstream_error/interrupted 时，其分类如下：

| 结果 | reason_code |
| --- | --- |
| `excluded` | `client_cancelled`、`user_invalid_request`、`user_context_limit`、`user_model_unsupported`、`user_group_access`、`user_authentication`、`user_quota`、`user_rate_limit`、`content_policy` |
| `failure` | `provider_authentication`、`provider_quota`、`provider_capacity`、`provider_5xx`、`account_pool_unavailable`、`service_timeout`、`service_transport`、`service_internal` |
| `unknown` | 缺失或 `unclassified` 产生 unclassified；schema 不接受的未来码使关联元数据不可用，产生 linkage_unavailable；不能默认为成功 |

## 观测边界

成功水位仅证明指定范围聚合完成，不证明所有请求都写入源日志。无源记录、过滤或丢弃的错误日志、超出十分钟重算/额外九十分钟关联边界的迟到记录无法重建。快照的 `bounded_observation_notice` 与中英文界面都明确这一范围。配置不改变模型授权、路由或计费，停用期间不回填；这些限制必须随 API 数据一起解释。
