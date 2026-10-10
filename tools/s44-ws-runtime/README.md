# S4.4 可控 HTTP/WS 上游与真实客户端

本目录只提供独立 fixture 与驱动，不修改应用、数据库、调度或监控合同。服务端使用项目已安装的 `github.com/coder/websocket v1.8.14`；客户端进行实际 WS handshake/read/write/close。`run_complete` 仅证明客户端预期协议结果，始终包含 `database_verified=false`。

关键不变量及权威 owner：

- 应用的请求/准入逻辑 turn owner 创建 `service_status_observation.observation_key`。fixture 不注入、不生成、不返回此字段，也不向 ctx 或数据库预填监控事实。输入 `s44:<run>:<case>:<index>` 和响应 ID 只供真实源日志对照。
- fixture 的单一 mutex state owner 按输入 marker 保存 attempt，跨上游连接重建不重置。上游 `session/local_turn` 是真实 socket 局部计数，不能代替应用 `logical_turn`。
- client 在收到本轮 `response.created` 后先断开实际下游，再等待 `--cancel-delay`，最后放行上游完成。只有实际 DB `client_disconnected` 终态与 fixture 的完成写出证据共同证明取消后 drain。
- fixture 计划在 run/case 第一次请求前冻结；已开始后修改返回 409。证据上限显式拒绝新请求，若并发写出达到上限，控制证据返回 507 和 `evidence_complete=false`。
- 上游与控制分别监听；两者都按实际 socket 对端允许 loopback/显式私网 CIDR，不信任 `X-Forwarded-For`。公网 CIDR 配置直接失败。控制无需密码，只能部署在 internal 隔离 Docker 网络，主机不发布端口。
- 只接受空认证（直接本地 smoke）或以下固定 fake 上游凭据：`sk-s44-fixture-first`、`sk-s44-fixture-second`、`s44-fixture-oauth`。不会将认证头、原始正文、原始网络错误或任意响应正文写入证据。

## 构建与隔离网络

仓库根运行：

```sh
bash tools/s44-ws-runtime/build.sh amd64
cd tools/s44-ws-runtime
S44_NETWORK_NAME=实际隔离网络名 S44_FIXTURE_CIDR=实际私网CIDR docker compose -f compose.fixture.yaml build
S44_NETWORK_NAME=实际隔离网络名 S44_FIXTURE_CIDR=实际私网CIDR docker compose -f compose.fixture.yaml up -d
```

应先由验证 owner 检查 `docker network inspect` 的 `Internal=true`，再把测试应用与 fixture 接入该网络。上游默认为 `http://s44-ws-fixture:8080`，控制为 `http://s44-ws-fixture:8081`。镜像为无 shell 的 scratch，复制本地交叉构建二进制，build 无网络，不触发项目 CI。客户端可通过 `docker compose run --rm --no-deps s44-ws-fixture drive ...` 在同一网络运行。应用入口也可通过 owner 已建立的 loopback 转发由本地二进制驱动。对不在 loopback 的 fixture 控制端，须由父代理安排私网可达路径。

仅隔离测试实例的配置（真实键名来自 `config.go`）：

```yaml
security:
  url_allowlist:
    enabled: true
    upstream_hosts: [s44-ws-fixture, chatgpt.com]
    allow_private_hosts: true
    allow_insecure_http: true
gateway:
  openai_ws:
    enabled: true
    oauth_enabled: true
    apikey_enabled: true
    force_http: false
    mode_router_v2_enabled: true
    ingress_mode_default: ctx_pool
    responses_websockets_v2: true
    prewarm_generate_enabled: false
    dial_timeout_seconds: 10
    read_timeout_seconds: 30
    write_timeout_seconds: 10
    ingress_inter_turn_idle_timeout_seconds: 120
```

读取超时应大于 cancel delay；不要为测试关闭 TLS 校验。不得把以上允许 HTTP/私网参数复制到生产。

## OAuth 的固定 URL 边界

当前应用 `openai_ws_forwarder_payload.go` 将 OAuth WS 固定到 `wss://chatgpt.com/backend-api/codex/responses`，OAuth `credentials.base_url` 不生效。fixture 同时提供这个路径及 API key 的 `/v1/responses`，HTTP/SSE/WS 均可读。

验证 OAuth 时由父代理在同一 internal Docker 网络将 fixture 添加别名 `chatgpt.com`，改为 `--listen=0.0.0.0:443 --tls-cert=/tls/fixture.crt --tls-key=/tls/fixture.key`，只读挂载本目录 `tls/`。`bash generate-tls.sh` 创建有效7天、SAN 为 `chatgpt.com/s44-ws-fixture` 的一次性自签证书。应用只读挂载公钥证书并设置 `SSL_CERT_FILE=/实际路径/fixture.crt`，保留域名/TLS 证书验证；不要挂载私钥给应用。fixture 非 root 绑定443需在容器配置 `sysctls: {net.ipv4.ip_unprivileged_port_start: "0"}`，或明确配置监听443所需的最小权限。网络须无外网路由，避免 fake OAuth token 发往真实供应商。应用已有其它代理配置也须由父代理检查。该做法不修改协议 URL owner；证书和 fake 凭据不得进入 Git。

## 实际管理 API 输入

管理员 JWT 由父代理持有；本工具不创建用户或读取 JWT。按真实 API 顺序：

1. `POST /api/v1/admin/groups`：`{"name":"s44-ws-ctx-pool","platform":"openai","rate_multiplier":1,"subscription_type":"standard","is_exclusive":false}`。记录返回组 ID。不同事件验收场景建议独立分组，避免已有失败混入同一个5分钟窗口。
2. `POST /api/v1/admin/accounts` 的 API key owner 输入如下，用实际组 ID 替换 `123`。`ctx_pool` 和 `passthrough` 分别创建独立组/账号，不混合模式。HTTP 429 换号场景至少准备两账号，second 的 priority 改2、fake api_key 改 second。

```json
{"name":"s44-ws-first","platform":"openai","type":"apikey","credentials":{"api_key":"sk-s44-fixture-first","base_url":"http://s44-ws-fixture:8080","model_mapping":{"gpt-5.1":"gpt-5.1"}},"extra":{"openai_apikey_responses_websockets_v2_enabled":true,"openai_apikey_responses_websockets_v2_mode":"ctx_pool"},"concurrency":5,"priority":1,"rate_multiplier":0,"group_ids":[123],"upstream_billing_probe_enabled":false}
```

OAuth 对应输入（expiry 是隔离 fake 时间；不能省略 account ID 后猜测 header）：

```json
{"name":"s44-ws-oauth","platform":"openai","type":"oauth","credentials":{"access_token":"s44-fixture-oauth","chatgpt_account_id":"s44-fixture-account","expires_at":"2099-01-01T00:00:00Z","model_mapping":{"gpt-5.1":"gpt-5.1"}},"extra":{"openai_oauth_responses_websockets_v2_enabled":true,"openai_oauth_responses_websockets_v2_mode":"ctx_pool"},"concurrency":5,"priority":1,"rate_multiplier":0,"group_ids":[123],"upstream_billing_probe_enabled":false}
```

OAuth fake expiry 防止验证中刷新 token；不要配置真实 refresh token。若 fake token 身份解析无法通过，按实际 API 错误检查 `chatgpt_account_id` 的当前合同，不能悄悄换为真实 token。

3. `POST /api/v1/keys`（属于当前 JWT 用户的 API）：`{"name":"s44-ws-key","group_id":123,"quota":0,"rate_limit_5h":0,"rate_limit_1d":0,"rate_limit_7d":0}`。API 返回的 key 只置于 `S44_API_KEY` 环境变量，不打印响应或写入证据。真实网关客户端默认读取此变量；fixture 只看到账号中的固定 fake 上游 token。

测试新请求模型时，driver `--model s44-new-model`，账号 `credentials.model_mapping` 加 `"s44-new-model":"gpt-5.1"`，且当前分组模型白名单允许该请求名。这样 DB 叶子应为 requested_model=`s44-new-model`，fixture 的 `upstream_model` 为 `gpt-5.1`。若需要验证模型真实拒绝则不要添加映射，不能把空库存当用户模型拒绝。分组模型策略、余额/额度和账号健康/冷却均由父代理控制，fixture 不清理它们。

## 驱动与外部合同

```sh
# 子进程已继承 S44_API_KEY，值不写在命令行或终端输出。
tools/s44-ws-runtime/bin/s44-ws-fixture drive --url ws://127.0.0.1:实际端口/v1/responses --scenario ws-multiturn --run ss06ctx
tools/s44-ws-runtime/bin/s44-ws-fixture drive --url http://127.0.0.1:实际端口/v1/responses --scenario http-retry-zero --run ss01http
```

| driver 场景 | 实际上游序列 | 父代理应核对的 DB/聚合合同 |
|---|---|---|
| `http-retry-zero` | 首次429，应用隐藏 retry/换号后同 marker 完成，Token=0 | SS01：同 observation key 中 attempt 与 completed；success=1/failure=0，费用=0，只有一条计费生命周期 |
| `ws-multiturn` | turn1 的 attempt1 在任何输出前断连；同 marker attempt2 返回500 `response.failed`；同下游连接 turn2 返回零Token完成 | SS06：fixture session/local_turn 重置与应用 logical_turn 区分；turn1 与 turn2 两独立 observation key；同 turn 重建保持键，turn2 不覆盖失败 |
| `ws-retry` / `ws-retry-429` | 首次断连/429，第二次同 marker 完成 | 同轮 transport retry/账号切换对照；429 至少两账号，ctx_pool 有 retry，passthrough 是否允许隐藏 retry 以实际合同为准 |
| `ws-cancel --control http://fixture:8081` | 收到created后客户端中断，等待750ms，控制放行completed | SS02：终态应 client_disconnected/excluded；fixture 最后 completed written=true 证明上游 drain 真发生；client run_complete 本身不证明 drain |
| `ws-success --count 5` | 同连接5次独立 completed 后1000正常关闭 | SS02 completed→close 保持success；SS13 每个恢复周期使用新 run，形成新 observed_at |
| `ws-failure --count 5` | 同连接5个500 provider终态 | SS03：5个 key/生命周期；正常关闭不新增失败；同批重扫不推进事件 |
| `ws-auth` / `ws-quota` | 401/402 provider终态 | 供应商原因反例；用户同状态拒绝由父代理走本部署鉴权/额度入口 |
| `http-success --count 5` | 非流式零Token HTTP200/completed | SS21：固定旧应用在新增可空字段schema上真实读写；旧应用行元数据NULL，新应用行有效元数据；计费ID/DTO保持 |

`--url` 使用实际 Responses 路由，当前应用同时注册 `/v1/responses` 与 `/responses`。`--pace` 只延迟请求，不推进聚合。`--timeout` 是单turn期限；默认20秒。driver 对 WS terminal 的类型及固定 fixture response ID作断言，不输出 raw error/message/header。

SS12 必须由父代理等待旧失败退出5分钟窗口/调整隔离配置，再重复聚合而不发送新请求，确认 awaiting_data且不恢复。SS13 必须由父代理分别在三个成功聚合周期前发送独立 `ws-success --run recover1/2/3`，每周期包含足够正常样本且窗口不含异常/unknown；周期中重扫不能当新证据。旧失败还在窗口时发送5个成功不保证窗口正常，需等旧失败移出或按当前阈值提供足够样本。fixture 不加速时钟、不触发聚合、不复制数据库日志。

## 控制及证据

仅在隔离网络中：

- `POST /control/plan {"run":"example","case":"provider_failure","behavior":"success_zero"}`：请求开始前更改该 run/case；支持所有表中行为。不能更改已经开始的 run/case，409是明确失败。
- `POST /control/release {"run":"example","case":"cancel_drain","index":"1"}`：放行取消 gate；客户端驱动自动调用，先看到created再关闭。未创建 gate 返回404。
- `GET /control/evidence?run=example`：返回带 UTC 时间、attempt、socket session/local_turn、写出结果的结构化证据。控制与上游端口分开，不能请求上游8080来访问控制。

客户端 stdout 为 JSON Lines；保存时使用本目录忽略的 `evidence/`。源记录在 HTTP/SSE/WS终态尚未入库前可能异步写入，DB owner 须等待实际队列处理后核对。控制 `frame_written=true` 仅说明 socket 写出成功，是否被 gateway 解析并冻结终态仍须 DB/日志证明。

失败与恢复路径：意外断连/超时使 driver 非零退出并输出固定 error code；不会把未知 terminal当成功。取消 gate 30秒未放行会显式记录 `gate_timeout` 并断开。fixture 重启丢弃内存计划/证据/attempt，必须换新 run，不能用重启后的 attempt 当旧轮重试。SS21旧应用schema兼容与最终 DB结论、本部署权限/收费事实、真实模式是否隐藏 retry均由父代理验证；本工具没有数据库权限。
