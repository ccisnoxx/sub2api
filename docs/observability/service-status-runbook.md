# 服务状态聚合故障运行手册

服务状态仅代表本部署已观察请求，不是供应商 SLA。聚合每分钟运行，配置前置读取与聚合共用单轮45秒预算，耗时从配置读取前的同一monotonic起点计量；前置失败和业务事务失败由同一个monitor计数且一轮一次。事实、水位和事故在同一事务提交。网关转发和计费不依赖服务状态配置、成功水位或聚合任务。停用本功能不停止应用、数据库、网关或用量队列。状态语义见 [API 合同](../service-status-api.md)，容量口径见 [OBS-001](OBS-001.md)。

## 告警部署与触发时间

在应用进程之外的现有运维调度器中，设置**每分钟一次**执行以下命令；运行目录由已检出的发行源码位置指定。需要 Python 3 和 `psql`，连接复用运维现有 libpq 环境或 `PGSERVICE`/受保护 service 文件，不新增长期 secrets、不在命令中写连接串。只需要既有身份对配置、水位的 SELECT 权限；本任务不自动扩大权限。

```bash
python3 tools/service-status-watch.py
```

检查器使用数据库时间、两张单行表和一致性只读事务，不扫描请求或历史事实；语句预算2秒、锁预算1秒、子进程最多5秒。退出0表示本次检查无告警（可能为 disabled），退出1表示已发现告警，退出2表示检查失败；stdout 只有固定事件/结果/告警码 JSON，psql stderr 不外发。`current` 仅表示水位检查通过，不能解释成供应商正常或事故 resolved。

调度器必须将退出1/2和下表应用日志接入**已有值班通知渠道**；检查器自身超过120秒无执行记录也须报警。采集器按部署身份与 alert_code 去重，故障持续时保留 active，不按单条聚合 success 自动解除 source_error；无需发送请求正文、组/模型/账号、用户身份或关联键。不要只监控应用内部任务，否则应用或聚合完全停止时无法发现。这里提供检测与路由合同；安装调度、通知路由和生产送达演练属于运维执行步骤，不能以本地检查通过代替。

| 信号 | 触发条件与处置 |
| --- | --- |
| `service_status_watch` / `watermark_stale` | 启用区间超过180秒且成功水位距数据库时间超过180秒。每分钟检查时，通常在最后成功水位后3—4分钟发现；任务停下时最后水位已落后0—60秒，因此通常在停止后2—4分钟发现。 |
| `watermark_missing` | 启用区间超过180秒仍无水位；新启用区间从下一个完整分钟开始，首次启用最多约5分钟发现。 |
| `invalid_watermark` / `configuration_changed` | 未来水位立即报警；配置版本持续不一致在启动宽限后报警。检查时钟/配置/任务，不手工推进水位。 |
| `aggregation_error` / `watch_unavailable` | 数据库保存的聚合错误或探测连接/权限/锁/SQL失败，下次检查报警。数据库不可达也不能解释成健康。 |
| `service_status_aggregate` / `aggregation_timeout` | 45秒预算耗尽，立即 ERROR；先检查锁与源扫描。 |
| `service_status_aggregate` / `consecutive_failures >= 3` | 连续三轮失败约2—3分钟内发现；包含 timeout 的失败。不能仅依赖 alert_code 等值匹配，因为 timeout 优先于连续失败码。跳锁不清零，重启清零；独立水位检查覆盖重启后的持续停滞。 |
| `service_status_source` / `source_error` | 源元数据无效、写入/队列缺口立即 ERROR；持久化或停止收尾失败分别为 source_persist/source_stop。无需等聚合失败。 |
| `service_status_watch` / `source_error` | 最近十分钟且在当前启用区间内的持久源缺口，或当前启用区间提交的 source_error gap；即使成功水位新鲜也报警，不被 stale/error 覆盖。 |
| `service_status_cleanup` / `commit_confirmed=false` | 清理未确认；rolled_back 与 commit_unconfirmed 区分处理。后者不能断言未提交，不重复手工删除。 |
| `service_status_capacity` / 非空 alert_code | 容量采样失败或相邻成功采样增长超过64MiB且超过25%；结合每日逻辑行数、索引和死元组判断。 |

显式停用或平台列表为空时水位/源错误业务告警被抑制，检查器仍输出 disabled 心跳；探测自身失败仍报警。重新启用时以当前启用区间提供180秒宽限，不以旧水位误报启动停滞；旧持久聚合错误仍报警，需确认新成功提交清除。应用日志需要 ERROR/WARN，常规运行/清理/容量需要 INFO；不要通过提高日志级别掩盖问题。应用监控事件带 `ops_system_log_skip=true`，从标准日志采集，避免写回故障数据库或递归产生源错误。

## 诊断顺序

1. 在管理员“服务状态”读取完整配置，确认 enabled、平台列表、minimum_samples、阈值与 version。禁用、无平台、无请求和样本不足不是供应商 outage；不要修改网关模型权限或计费配置来排查监控。
2. 查看每分钟 `service_status_aggregate` 的 outcome/stage/duration_ms/consecutive_failures。lock_skipped 是多实例争抢，不表示成功；确认至少一个实例推进共享水位。无日志时先检查进程、任务启动、日志采集与独立调度器心跳。
3. 运行以下只读命令，检查水位/配置版本、固定错误码、锁、长事务和事故阶段。权限不足时 activity_complete=false，仅表示当前身份可见范围；由已有管理员身份核查，不新授权角色。诊断不输出 SQL 正文，避免活动查询带出请求或参数。

```bash
psql -X -w -v ON_ERROR_STOP=1 -f tools/service-status-diagnostics.sql
psql -X -w -v ON_ERROR_STOP=1 -f tools/service-status-capacity.sql
```

4. 对 aggregation_timeout 先检查阻塞会话，再查 source_rows_loaded、scan_completed、阶段和表/索引增长。未完成扫描时零行不能证明零请求；十分钟重算产生重复 INSERT，不能用 n_tup_ins 当每日新增。只读命令失败保留为故障证据，不循环执行重查询；不自动终止会话、清库、VACUUM FULL 或扩大扫描范围。
5. 对 source_error 检查源持久化、队列丢弃、非法元数据及 source_persist/source_stop；只共享固定码和计数。正常推进水位不能证明源完整。修复源 owner 后让新完整窗口自然积累，不清除 source_error_at、gap_code、覆盖区间或事故记录制造恢复。

## 紧急停用与安全回退

优先在现有管理员界面关闭**服务状态独立开关**。命令操作须使用既有管理员 JWT：`ADMIN_CURL_CONFIG` 指向权限0600、含已有认证 header 的 curl 配置文件，`SUB2API_URL` 使用实际部署地址。不要开 shell tracing、打印认证文件或将 JWT 放进 curl 参数、工单或提交；管理 API Key 不支持本接口。

以下 Bash 命令读取最新配置，保留全部字段与 version，只改 enabled。先在测试部署核对地址；不要使用历史配置覆盖其他管理员更改。

```bash
set -euo pipefail
: "${SUB2API_URL:?设置本部署地址}"
: "${ADMIN_CURL_CONFIG:?指定既有管理员认证的私密 curl 配置文件}"
umask 077
status_work=$(mktemp -d)
trap 'rm -rf "$status_work"' EXIT
curl --silent --show-error --fail-with-body --config "$ADMIN_CURL_CONFIG" \
  "$SUB2API_URL/api/v1/admin/service-status/config" \
  | jq -e '.data | select(.version >= 1 and (.enabled | type == "boolean"))' > "$status_work/config.json"
jq '.enabled = false' "$status_work/config.json" > "$status_work/disable.json"
curl --silent --show-error --fail-with-body --config "$ADMIN_CURL_CONFIG" \
  -X PUT -H 'Content-Type: application/json' --data-binary @"$status_work/disable.json" \
  "$SUB2API_URL/api/v1/admin/service-status/config" > "$status_work/updated.json"
jq -e '.data.enabled == false' "$status_work/updated.json"
http_status=$(curl --silent --show-error --config "$ADMIN_CURL_CONFIG" \
  -o "$status_work/snapshot.json" -w '%{http_code}' \
  "$SUB2API_URL/api/v1/admin/service-status/snapshot")
test "$http_status" = 503
jq -e '.reason == "SERVICE_STATUS_DISABLED"' "$status_work/snapshot.json"
```

409表示 version 冲突，停止并重新 GET 后只改 enabled；PUT 失败、超时或503不得声称停用成功，重新读取确认。禁用不停止源日志或计费队列；按当前ARCH-004合同，任务入口跳过 Aggregate 和保留期清理，ticker继续读取开关以发现重新启用，不推进业务水位；快照503/disabled符合合同。用已授权的测试网关请求确认转发仍完成、计费/用量正常，勿为验证监控而发起未经授权的生产扣款。

数据库不可达时不能保证开关写入；不要停止整个应用或直接改迁移/账本作为“停用”。先恢复既有数据库通路；应用版本处理须遵守部署工具的兼容判定；当前跨 KIN 旧镜像恢复仍被禁止，不能据本手册绕过 image_rollback_compatible=false。优先冻结写入并向前修复；完整备份恢复须另行确认备份快照后的写入损失，保留原恢复材料，不直接覆盖生产数据库或做 SQL 回退。撤销 OBS-004 时同时撤销其检查器调度/规则，保留 OBS-001/003；不强推受保护分支。

## 恢复验证与告警解除

修复锁、连接、源 owner 或容量原因后，按最新 version 通过管理员完整 PUT 重新启用。新增启用区间从下一个完整分钟开始，停用期间不回填。等待至少两个相邻一分钟检查：水位应前进、年龄不超过180秒、watermark 配置版本一致、last_error_code为空；独立检查器应无告警，应用聚合确认提交，连续失败计数回到0。

检查 source_error_at 和 gap_code、完整覆盖和新样本。源时点离开十分钟重算范围仍需生成新的完整当前窗口，旧历史缺口保持 unknown；没有源错误日志不能证明所有请求均采样。不要仅凭一个 success 日志解除 source_error 或 watch_unavailable，先取得对应外部检查的新证据。

事故恢复必须有最后异常之后的新、足量成功请求，并达到配置 recovery_windows；重复扫描、时间流逝、无请求和旧成功样本不会恢复。awaiting_data/unknown 保持比例 null，recovering 保持观察态，只有 resolved 及 resolved_at 已写入才确认事故恢复。诊断脚本只输出事故阶段计数；详细管理员页面遵循现有隐私控制，不在告警或工单分享范围细节。

在隔离测试部署演练“停止聚合但保留外部调度器→3—4分钟水位告警→恢复后两次水位前进”“三轮失败”“源报告持久化失败”“禁用后网关继续转发”。再用现有通知渠道的测试功能验证真实送达、去重与解除；这一步不得向未授权收件人发送消息。生产送达与长期运行未经演练时须记为未验证。
