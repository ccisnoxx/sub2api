# UPSTREAM-005：服务状态后端重放准备交付

## Task

本报告记录 S4 后端的独立准备实现与本地验证。它不是 UPSTREAM-005 的正式阶段验收；正式累计候选、前置阶段验收、独立复核及远端门禁由父任务接续完成。

| 项目 | 固定身份 |
| --- | --- |
| 新 KIN stable tag | `v0.2.15-klno.2` |
| stable tag object | `0c1ede31ec63f9cfc2b72f07db66fb41f540208d` |
| stable peeled commit | `ca686de5e5ac14834e12701c156831abd5ec6b71` |
| MIG-001 基准 | `c0e7ab68b8479fc9d6f3b888d548eca4f4192974` |
| 准备分支起点，已含 S1 | `2d21a1634228d3d0573ac266dacd726e2908e2de` |
| 独立 S3 原提交 | `a38c26b77f0e5638dfd6d57e2fe1cd1c3b5a5ecb` |
| S3 cherry-pick 后的 S4 实现基准 | `593787383e73879f821b4d6faff6834899936045` |
| 本次 S4 实现 | `3497f3168542a53c4078eb7c5a81fb3ebbf0b544` |
| S4 代码审查范围 | `593787383e73879f821b4d6faff6834899936045..3497f3168542a53c4078eb7c5a81fb3ebbf0b544` |
| 固定 Personal 来源 A | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| S4 首引入 | `35962a802fdc499639c9c861072f25774ccba50b` |

合同来源是固定 A 的 `openspec/changes/adopt-plus-usage-and-diagnostics/service-status-contract.md` 全文，以及本次派发的 UPSTREAM-005/M5 边界。旧提交区间 `afcc7852b36a073dfc5fc0721ae8d24b90b7a335..A` 仅作为 S4 小范围重放起点；新文件在完整读取后引入，共享文件按当前权威代码应用必要增量，没有覆盖旧整文件或合并旧历史。

S3 cherry-pick 自动合并成功，无手工冲突。其原实现和 `docs/upstream/UPSTREAM-004.md` 保留为独立依赖提交；本报告的实现范围不包含该依赖。正式累计分支应只接入本 S4 实现及本报告，避免再次重放 S3。父任务随后补充的 S1 JSON fixture 修正 `ee415b62ea9076b7f682ab73fd94203478ca363a` 和 S3 journal `d06e9cd55b1b121f42427cebf8c6d3218222b3ca` 不在本准备树中；前者不改变运行代码，二者由正式累计基底承接。

交付前通过 GitHub latest release 和 `git ls-remote --tags upstream` 重查，最新非 draft、非 prerelease 稳定版本仍为上述 tag；release 发布时间为 `2026-10-10T07:54:36Z`，tag object 与 peeled commit 均一致。

## Summary

新增独立的匿名服务状态配置、聚合、事件及管理员 API，将真实 HTTP/WS 逻辑 turn 的安全元数据接入既有 usage/Ops 写入路径。默认配置保持关闭，不影响 V1/V2 监控。聚合结果只反映受限来源中可证明的事实；无 terminal、来源缺口、冲突、小样本、未支持入口和历史 NULL 保留 `unknown`，不生成正常状态或零比例。

| 关键不变量 | 权威所有者与行为 |
| --- | --- |
| 请求生命周期与计费幂等独立 | `service_status_observation.go` 在每个真实请求/逻辑 turn 生成服务端随机 observation key；同一 turn 的 attempt 共用 key，WS 后续 turn 使用新 key，不依赖 provider response ID 或 billing request ID。 |
| 首可信终态及取消不能后改 | source owner 的 attempt sequence 拒绝迟到旧 attempt；S1 timing observer 继续持有终态/取消排序锁，冻结终态时间、原因与匿名元数据。异步提交前深拷贝并提交终态。 |
| 重试成功不继承失败终态 | HTTP/WS bridge、pooled ingress 和 passthrough 使用当前 turn/attempt owner；恢复后由可信 terminal 决定归一化结果。缺终态的 attempt 不被当成失败终态。 |
| 不改变计费语义 | metadata 是 usage/Ops 的内部可空字段。原计费、倍率、重试、扣费、幂等和 mandatory fallback owner 保留；不新增失败计费行，非法 metadata 不阻断原 usage 写入。billing 去重丢失新生命周期时显式记录 source gap。 |
| 匿名数据边界 | source metadata 仅内部写入，JSON API 标记 `json:"-"`；聚合事实/API 不保存或输出 observation key、user/key/account、cost/token、原始请求或错误正文。范围仅含 platform/group/requested model 和匿名计数、时间、事件。 |
| 平台清单与平台归属唯一 | whitelist 复用新 KIN `domain.IsConcretePlatform`，包括 `command_code`、`cline`。普通 group 采用 group 定义的平台，composite 采用实际 final account，缺 account 保留未知。 |
| 聚合必须原子提交 | `service_status_repo.go` 使用事务、advisory lock 和配置行锁；每分钟以 45 秒预算重算最近 10 个完整分钟，并额外关联此前 90 分钟的候选 source。超过 50,000 条显式失败，facts/incidents/watermark 一起回滚，watermark 仅成功 commit 后推进。 |
| 恢复必须有新证据 | incident owner 按新的 qualified evidence 时间推进；重复扫描、旧失败窗口自然移出、未知窗口不能完成恢复。disable/re-enable、配置版本/epoch、group 可见性触发恢复计数重置。 |
| 持久化与保留边界 | 31 天 facts/resolved TTL，30 天可查询历史；单事件最多 32 个阶段更新，保留初次 detected；保留所有 open incident，resolved 最多 200 条且先按 platform/range 过滤再限量。 |
| 管理 API 的权限和输入边界 | 继承现有管理员认证/审计/限流，再严格要求 `auth_method=jwt`。PUT 必须完整字段并提供当前 version，冲突 409；body 上限 8192 字节，拒绝未知字段、尾随 JSON、配置 query、重复/非法 snapshot query。 |
| source 丢失不能静默变正常 | 异步 drop/panic、非法元数据、写入失败、billing 去重缺口标记安全 source gap。reporter 合并标记、有界异步写入及关闭 flush；源生产者和队列停止后、DB 关闭前停止 reporter。 |

共享 owner 整合保留 S1 strict 输出/取消语义、raw usage 采纳及包装音频 NULL 保护；保留 S3 诊断裁剪/不可变快照、当前 turn 调度、换号倍率、原失败 turn key/pricingAt/准入 context 和新 KIN per-turn billing helper。新 KIN 的协议、模型、计费和 provider 接线未从旧 A 覆盖。

相对于旧 A，新增以下有实证的缺口修正：

1. 旧 11 平台 switch 无法接受新 KIN 的 `command_code`/`cline`。先确认旧行为失败，再复用 domain owner，并以真实数据库检验普通/composite/missing account 归属及额外关联窗口。
2. WS→HTTP bridge 的真实 transport/non-2xx HTTP 失败缺 source terminal 候选。先以实际 bridge 调用确认失败类型为 unknown，再在发送 attempt 和实际失败 owner 接线；修复重试成功路径也验证为 completed。
3. pooled passthrough 首 turn 的 source 位于 request context，但独立 control context 未继承。真实 WS 回归在第一次定向 race 中暴露 metadata NULL；将 source owner 绑定到保留取消语义的 control context，修复后重跑失败项和受影响范围。
4. 旧 handler 仅验证 admin role，未限制认证方法。按本次显式合同收窄为管理员 JWT；先确认原行为接受其他认证方法，再验证真实 JWT 的签名、撤销、role 和新 API 权限。

## Files changed

实现提交修改 66 个后端文件，5069 行新增、171 行删除。职责包括：服务/source owner 与 DTO、usage/Ops schema/SQL 写入接线、HTTP/WS 调用者、事务聚合/事件/API、Ent/Wire 生成结果及必要回归测试。没有 frontend、CI、发布、sourcejson、计划、journal、SQL migration、migration runner、部署、依赖清单或无关静态资源变更。

完整实现文件清单见本节末。`backend/ent/schema/usage_log.go` 是 Ent 的维护源，生成产物来自本项目 `go generate ./ent`；Wire 源变更由当前安装的 Wire v0.7.0 生成，不复制旧生成结果。

## Tests run

以下全部在与实现提交一致的代码状态上完成；受后续修改影响的范围已再次验证，未受影响的成功证据复用。PASS 计数仅为日志顶层测试数量，不含子项，也不跨多次命令累加为覆盖指标。原始 stdout 已完整保留并交给父任务，报告仅记录无本机路径的文件名和内容哈希。

| 检查 | 结果与证据 |
| --- | --- |
| Ent 生成 | `go generate ./ent` 成功，维护 schema 与生成产物一致。 |
| Wire 生成 | 默认命令因冻结 go.sum 未记录 Wire 工具依赖的 checksum 失败；核对安装版本后以 `GOFLAGS=-mod=mod go generate ./cmd/server` 成功。仅生成器临时产生的 go.sum 记录已恢复，go.mod/go.sum 最终无 diff。`upstream005-wire.log`：SHA256 `b717bb2f419d4b7eb4f06d0aa37bcda4060f3ccc966f0b4d11140bd2b4122361`。 |
| 初始 S4 定向 unit，5 包 | exit 0，53 PASS，0 SKIP。仓储/cleanup 范围未再改变；后续 handler/JWT 变更由下述最终检查覆盖。`upstream005-unit.log`：SHA256 `1d67c50783138064cdc6526554dee5fb1f1fa039efc61f2afb98782c10570e35`。 |
| source/聚合/配置/事件真实集成 | exit 0，20 PASS，0 SKIP；其中 13 项为真实 PostgreSQL 集成，7 项为同包的归一化/事件纯函数测试。实际启动 PostgreSQL `18.1-alpine3.23`、Redis `8.4` 与 Ryuk `0.13.0` fixture。`upstream005-integration.log`：SHA256 `6af2809044e130e4bc895e556ef3fecff00b0d417de99b06538b8b25d32624aa`。 |
| 新平台与归属真实集成 | exit 0，1 PASS，0 SKIP。与上一项合计 14 个不同的真实 PostgreSQL 集成测试。`upstream005-platform-integration.log`：SHA256 `6acb6812ca2b8df89d9e455c6927b731edb9198f6fc2548541e795e1f6e4d2e3`。 |
| usage/Ops SQL shape 和安全缓冲 unit | exit 0，18 PASS。`upstream005-sql-unit.log`：SHA256 `5b7e92acc03a360a0926378592a8951251e8e1c42394db862fef2c5e1d795462`。 |
| bridge 实际失败/恢复及 body/query 回归 | exit 0，3 PASS。`upstream005-bridge-green.log`：SHA256 `7a93ed856d5814762c3a9fc1b19d1b9b74325c182423b762db25891176c4d958`。 |
| 第一次定向 race，5 包 | exit 1，147 PASS、1 FAIL、0 SKIP。唯一失败为新增真实 passthrough 首 turn metadata 回归；不是一次成功门禁。其余未变动的 relay/Ops/repository/routes 成功输出保留复用。`upstream005-race-initial.log`：SHA256 `e093cabd3f67946ea79a643e8533b8a38a2cfbc49da0d8c8cfabcd3e58f430b9`。 |
| passthrough 失败项修复后 race | exit 0，3 PASS。`upstream005-ws-green.log`：SHA256 `4f360ea73eb68c8fab2de56803450a9a8d4f8512c06ea65bd34a42bec93b600f`。 |
| 真实 native/passthrough 调度 fixture 的 S4 断言 race | exit 0，3 PASS；既有 S3 调度/计费断言与新增匿名 turn/冻结 metadata 断言共同通过。`upstream005-s35-source-race.log`：SHA256 `9393d01189ed69a2ee62e5168be650e90acd7cf345f242daa4d19040e8ed57ca`。 |
| 严格 JWT handler 与真实 routes race | exit 0，15 PASS，0 SKIP。`upstream005-jwt-green.log`：SHA256 `47630edbed801911ebc202f66f0db66cac2b1558cb785a5aab6b9d0793e8a833`。 |
| 最终 service/handler 受影响范围 race | exit 0，88 PASS，0 SKIP，无 DATA RACE；覆盖 observer、source freeze、异步用量深拷贝、HTTP/WS/bridge、取消、失败、无 terminal、工具/推理/图片/音频、真实交错 turn 与权限输入。`upstream005-final-race.log`：SHA256 `1df844e0ec409d6e79cf129871b9e6a42bfa91516ff619b67ee01014de9eb320`。 |
| 要求的实际 build | `make -C backend build` exit 0。项目命令为 `CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=0.2.14" -trimpath -o bin/server ./cmd/server`，保留冻结基底的版本文件。`upstream005-build.log`：SHA256 `376cdd47b9d520f6fd50baddaa4ab2307bc889988817d325cc4e763d23469f3b`。 |
| 最终静态/范围检查 | `git diff --check` 成功；变更仅在分配的后端范围；go.mod/go.sum 和 migrations 无变更，251–254 与 MIG-001 逐字节相等。 |

安装的 Ent v0.14.5、Wire v0.7.0 和 testcontainers-go v0.40.0 的相关 API/生成命令由本项目依赖或本机已安装官方文档核对。没有新增第三方库。

可复现命令如下。除 build 外在 `backend` 目录执行。集成测试设置 `CI=true`，不允许 Docker 不可用时静默跳过。实际 `DOCKER_HOST` 来自 `docker context inspect colima` 的 `Endpoints.docker.Host`；下文使用环境变量表达该已确认的宿主 socket，容器内挂载 override 为 `/var/run/docker.sock`，不将开发机路径写入仓库。

```sh
go generate ./ent
GOFLAGS=-mod=mod go generate ./cmd/server

go test -tags=unit -count=1 -v ./internal/service ./internal/handler ./internal/repository ./internal/server/routes ./cmd/server \
  -run 'TestServiceStatus|TestProvideCleanup|TestUsageLogInsert|TestUsageLog.*RequestType|TestUsageLog.*Safety|TestUsageLog.*Session|TestOpsInsertErrorLogArgs'

CI=true DOCKER_HOST="$DOCKER_HOST" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
  go test -tags=integration -count=1 -v -timeout=240s ./internal/repository -run '^TestServiceStatus'

CI=true DOCKER_HOST="$DOCKER_HOST" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
  go test -tags=integration -count=1 -v -timeout=120s ./internal/repository \
  -run '^TestServiceStatusRepositoryGroupPlatformOwnerAndNewConcretePlatforms$'

go test -tags=unit -count=1 -v ./internal/repository \
  -run '^TestPrepareUsageLogInsert|^TestUsageLogInsert|^TestUsageLog.*RequestType|^TestUsageLog.*Safety|^TestUsageLog.*Session|^TestOpsInsert|^TestOpsNullable'

go test -tags=unit -count=1 -v ./internal/service ./internal/handler \
  -run '^TestServiceStatusSourceHTTPBridge|^TestServiceStatusConfigRejectsQueryAndOversizedBodyBeforeRead'

# 第一次 race 返回 exit 1，唯一失败已由下述失败项和最终受影响范围验证修复。
go test -race -tags=unit -count=1 -v -timeout=240s ./internal/service ./internal/service/openai_ws_v2 ./internal/handler ./internal/repository ./internal/server/routes \
  -run 'TestServiceStatus|TestResponsesOutputTiming|TestResponsesUsageTiming|TestRelay.*Timing|TestRelayObserved|TestSubmitUsageRecordTask|TestOpenAISubmitUsageRecordTask|TestOpenAIGatewayHandlerSubmitOpenAIUsageRecordTask|TestOpsRoutingDiagnostics|TestRoutingDiagnostics|TestOpenAIWSRouting|TestOpsUpstreamErrorEvent|TestBeginOpsStream|TestMarkOpsStreamError|TestLogOpsStreamError|TestOpsErrorLog|TestEnqueueOpsErrorLog|TestOpsErrorLoggerMiddleware|TestProxyOpenAIWSHTTPBridgeTurn|TestS35WSNativeLaterTurnFailoverReplaysCurrentTurn'

go test -race -tags=unit -count=1 -v ./internal/service \
  -run '^TestResponsesUsageTiming_WSPassthroughAdmittedTurns$|^TestServiceStatusSourcePassthrough'

go test -race -tags=unit -count=1 -v -timeout=120s ./internal/handler -run '^TestS35WS'

go test -race -tags=unit -count=1 -v ./internal/handler ./internal/server/routes -run '^TestServiceStatus'

go test -race -tags=unit -count=1 -v -timeout=180s ./internal/service ./internal/handler \
  -run 'TestServiceStatus|TestResponsesOutputTiming|TestResponsesUsageTiming|TestSubmitUsageRecordTask|TestOpenAISubmitUsageRecordTask|TestOpenAIGatewayHandlerSubmitOpenAIUsageRecordTask|TestProxyOpenAIWSHTTPBridgeTurn|TestS35WS'

# 从仓库根目录执行。
make -C backend build
git diff --check
```

新增平台、bridge 失败和严格 JWT 回归先在缺失合同的原行为上确认失败；原始 red 输出也已保留。测试覆盖事务回滚/超限、水位、配置冲突、source 单批/缓冲/回退、invalid metadata、不新增失败计费、事件新证据/幂等、可见性、保留过滤、未知窗口、实际 JWT 路由，以及实际 WS 多 turn、重试/换号与异步复制。

## Risks

1. 本地实现准备完成，但正式 002/004 依赖验收、正式累计 005 候选、fresh 独立只读复核、标准 PR CI 和 Security Scan 尚由父任务执行。维护者已授权 002–006 使用标准 PR CI + Security Scan；旧 Personal CI 的旧祖先/PR base 条件留到 007，本次没有修改该控制。
2. source 覆盖是可观测日志的下界。完全未进入 usage/Ops 的请求无法回建；未接入口、历史 NULL、不合法或缺 terminal 数据保持 unknown。匿名计数和 `bounded_observation_notice` 不代表全站请求总量。
3. source reporter 非阻塞、合并且有界重试；DB 完全不可用时不能保证持久化最后一个 source gap，进程内标记、失败/过期水位和 safe unknown 仍保留。它不承诺日志零丢失。
4. 管理员 JWT 是相对固定 A 的有意权限收窄，遵循本次派发。管理 API Key 或其他 admin 认证方式不能读取本新 API；既有其他管理 API 的权限行为未改。
5. 没有生产部署/开启 smoke、浏览器或前端验收，没有真实生产长期容量/SLO、运维告警或备份恢复演练。45 秒预算、50,000 上限与 TTL 的代码/数据库行为通过定向验证，不等于生产吞吐承诺；M5 后续 OBS 工作仍待执行。
6. 冻结 KIN 中部分历史惯例资料没有随源码存在；未据此创建第二套出站身份/调度规范。当前真实 owner、合同和用到的依赖 API 已读取核对。独立复核应重点看终态与取消一致性、跨 turn/attempt 隔离、聚合事务/恢复证据、匿名边界和管理员 JWT。

## Database impact

本阶段没有新增或修改 SQL migration。253/254 已由 MIG-001 原字节纳入，本次只补维护 schema 和应用读写接线；不做历史 backfill，不修改 usage/Ops 原始 retention、billing 去重键或扣费流程。

source 元数据分别写入既有 usage/Ops 的可空 JSONB 列，非法元数据只丢弃该内部字段并标记缺口；事实表、配置/epoch 和事件维护使用 MIG-001 已创建的结构。source key 仅在规范化过程中用于内存关联，不落到匿名事实/事件/API。

聚合超限、取消、数据库错误会回滚当轮 facts/incidents/watermark；固定安全错误记录是独立的 best effort 写入，不将失败轮标成成功覆盖。配置更新采用 version 冲突检测；disable/re-enable/可见性变化重置恢复证据，重新等待完整启用窗口。

251–254 与 MIG-001 的字节校验如下：

| 文件 | SHA256 |
| --- | --- |
| `backend/migrations/251_add_usage_log_timing.sql` | `abce11c5a8cf8fa993d9a0fb25190af3130722fcc419f1f94b5a5397ff60003b` |
| `backend/migrations/252_add_ops_routing_diagnostics.sql` | `b01aa7a1a3fd47681dc135b13997710f253f9a63ab36ea139593200194b395ac` |
| `backend/migrations/253_service_status_observation.sql` | `8440c2da7a92dd5b70da653a7c02a485bca4337ae76868ef5b0ccd7996b897c2` |
| `backend/migrations/254_service_status.sql` | `3e8deac13622490588c61dc5cc9411653c05631009c19380e83e289e17251b8b` |

## Rollback

上线时默认关闭。若已经开启，先经管理员 JWT 读取最新配置，以完整 PUT 和当前 version 将 enabled 设为 false，再切回此前已验收程序；保留 MIG-001 的 SQL/schema 和已有 source/facts/incident 历史，不删除 253/254 或回滚其结构。旧程序忽略本阶段额外可空字段。恢复本阶段程序后，重新启用产生新版本/epoch 并从完整分钟窗口重新累积恢复证据；没有新终态或仍有 source gap 时保持 unknown。

正式累计基底若已包含其他阶段的 schema/DI 变更，接入本增量时必须从该基底的维护源按本项目命令重新生成 Ent/Wire，保留所有当前 providers；不能将本准备树的整份生成文件覆盖新累计 owner。父任务已说明正式依赖树含 S1+S2+S3，需合并 S2 的 AvailableChannelHandler Billing 注入并重新生成 Wire；本 S4 delta 仍仅为上述 `593787383e73879f821b4d6faff6834899936045..3497f3168542a53c4078eb7c5a81fb3ebbf0b544`。

## Acceptance checklist

- [x] 从固定 S1 起点独立 cherry-pick S3，S4 实现与依赖提交范围分开。
- [x] 来源合同完整读取；小范围重放并保留新 KIN/S1/S3 真实权威 owner。
- [x] 服务端随机 observation key、逻辑 turn 隔离、首可信 terminal/取消冻结及匿名字段接线。
- [x] no terminal/source gap/小样本为 unknown，恢复依赖新证据，平台取当前 domain owner。
- [x] 每分钟 45 秒预算、10 完整分钟 + 额外 90 分钟关联、50,000 上限失败回滚、成功 commit 水位。
- [x] 配置版本/epoch、可见性恢复重置、事件幂等、32 更新/31 天 TTL/30 天历史/open 保留/resolved 200 先过滤后限量。
- [x] 严格管理员 JWT、8192 body、完整 PUT/version409、query/字段 whitelist；API 匿名边界。
- [x] 本地必要 unit、真实 PostgreSQL source/aggregate/incident/配置集成、observer/异步/真实 WS race 和要求的 build 通过；失败项已诊断修复并直接重验。
- [x] SQL251–254、runner/白名单、frontend/CI/发布/部署/依赖清单无变更。
- [ ] 002/004 正式依赖验收与正式累计 005 候选确认，由父任务接续。
- [ ] fresh 独立只读复核、正式候选的标准 PR CI + Security Scan，由父任务接续。
- [ ] 生产开启 smoke、长期容量/运维/恢复验证及 006 前端集成，不在本准备树中宣称完成。

### 实现文件清单

以下仅列 `3497f3168542a53c4078eb7c5a81fb3ebbf0b544` 的 S4 实现文件；本报告是另一次文档提交。

```text
backend/cmd/server/wire.go
backend/cmd/server/wire_gen.go
backend/cmd/server/wire_gen_test.go
backend/ent/migrate/schema.go
backend/ent/mutation.go
backend/ent/runtime/runtime.go
backend/ent/schema/usage_log.go
backend/ent/usagelog.go
backend/ent/usagelog/usagelog.go
backend/ent/usagelog/where.go
backend/ent/usagelog_create.go
backend/ent/usagelog_update.go
backend/internal/handler/handler.go
backend/internal/handler/openai_gateway_handler.go
backend/internal/handler/openai_ws_routing_diagnostics.go
backend/internal/handler/ops_error_logger.go
backend/internal/handler/s35_ws_routing_behavior_test.go
backend/internal/handler/service_status_handler.go
backend/internal/handler/service_status_handler_test.go
backend/internal/handler/service_status_observation_test.go
backend/internal/handler/usage_record_submit_task_test.go
backend/internal/handler/wire.go
backend/internal/repository/ops_repo.go
backend/internal/repository/ops_repo_args_test.go
backend/internal/repository/service_status_incidents.go
backend/internal/repository/service_status_normalize.go
backend/internal/repository/service_status_normalize_test.go
backend/internal/repository/service_status_repo.go
backend/internal/repository/service_status_repo_integration_test.go
backend/internal/repository/service_status_source_integration_test.go
backend/internal/repository/usage_log_repo_insert.go
backend/internal/repository/usage_log_repo_insert_shape_unit_test.go
backend/internal/repository/usage_log_repo_query.go
backend/internal/repository/usage_log_repo_request_type_test.go
backend/internal/repository/usage_log_repo_safety_buffering_unit_test.go
backend/internal/repository/usage_log_session_id_unit_test.go
backend/internal/repository/wire.go
backend/internal/server/middleware/middleware.go
backend/internal/server/routes/admin.go
backend/internal/server/routes/service_status_routes_jwt_test.go
backend/internal/server/routes/service_status_routes_test.go
backend/internal/service/openai_gateway_forward.go
backend/internal/service/openai_gateway_passthrough.go
backend/internal/service/openai_gateway_usage.go
backend/internal/service/openai_usage_timing_test.go
backend/internal/service/openai_ws_forwarder_ingress.go
backend/internal/service/openai_ws_http_bridge.go
backend/internal/service/openai_ws_v2_passthrough_adapter.go
backend/internal/service/ops_models.go
backend/internal/service/ops_port.go
backend/internal/service/ops_service.go
backend/internal/service/ops_upstream_context.go
backend/internal/service/service_status.go
backend/internal/service/service_status_aggregator.go
backend/internal/service/service_status_aggregator_test.go
backend/internal/service/service_status_observation.go
backend/internal/service/service_status_observation_test.go
backend/internal/service/service_status_source_integrity.go
backend/internal/service/service_status_source_integrity_test.go
backend/internal/service/service_status_source_reporter.go
backend/internal/service/service_status_source_reporter_test.go
backend/internal/service/service_status_test.go
backend/internal/service/service_status_types.go
backend/internal/service/stream_output_timing.go
backend/internal/service/usage_timing.go
backend/internal/service/wire.go
```
