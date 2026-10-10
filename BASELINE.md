# PREP-001：当前生产基线与隔离恢复记录

本记录冻结 2026-10-10 的已发布应用及恢复点，只处理 PREP-001。生产数据库在运行中取得一致的只读快照；“冻结”指保存恢复点，没有暂停线上写入、重启应用或执行生产恢复。

## 源码、发行与运行绑定

| 项目 | 核实值 |
| --- | --- |
| `main` 控制面快照 | `f5c4c3304106040a01a32c5e6d4ff18f3ec49930` |
| `personal` 与生产应用 SHA / OCI revision | `3d5e1fde82707900a21f2b5538b112c7bab3c04a` |
| 本次核实的生产部署工具 SHA | `e3b3e8e284e2d9b0ff8f972650807c4d40863c64` |
| 实际 Git 标签 | [v0.2.14-klno.5-tps.2](https://github.com/ccisnoxx/sub2api/releases/tag/v0.2.14-klno.5-tps.2) |
| Release 状态 | GitHub 标记为 prerelease；这是当前实际运行基线，不称为稳定 Release |
| 应用镜像标签 | `ghcr.io/ccisnoxx/sub2api:0.2.14-klno.5-tps.2` |
| 应用固定镜像 | `ghcr.io/ccisnoxx/sub2api@sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530` |
| OCI source / 平台 | `https://github.com/ccisnoxx/sub2api` / `linux/amd64` |
| 生产成功部署记录 | `20261010T120908Z-b497b80ee8a6` |
| PostgreSQL 18.6 固定镜像 | `postgres@sha256:6c538e7206ea40ff740ef27883529390a690b6ead6ba96b44c67a9f7c638e8fd` |
| Redis 固定镜像 | `redis@sha256:bd999b5cfee25fb24b8320a31fddbd69f462df44c8138c66e369582937beebc0` |

生产应用、PostgreSQL 和 Redis 均健康；应用只发布 `127.0.0.1:10088 → 8080`。运行镜像、持久镜像选择和 OCI revision 一致。Git 标签实际解析到上述 personal SHA，匿名 GHCR 拉取元数据得到相同 digest、revision 和唯一 linux/amd64 平台。

## CI 与 Release

- 最近一次成功的 Personal CI：[run 38065981816](https://github.com/ccisnoxx/sub2api/actions/runs/38065981816)，SHA `1ae266791d0ab324cd13a63ab344019c79102176`，分支 `codex/verify-personal-ci-plus-chat-s155`，事件 `pull_request`。这是后续候选的成功运行，不是当前生产镜像的来源。
- 当前生产 SHA 自身的成功 Personal CI：[run 38049101726](https://github.com/ccisnoxx/sub2api/actions/runs/38049101726)。
- 最近一次成功的 Release：[run 38050231176](https://github.com/ccisnoxx/sub2api/actions/runs/38050231176)，与当前生产应用 SHA 一致。
- `main` 的继承应用树与实际 personal 应用不同。已有文档候选的 CI / Security Scan 38077678935 / 38077678932 失败：前者缺少 `tools/test_backend_unit_runner_test.py`，后者报告 12 个 Go 标准库可达漏洞。它们不等于当前生产 personal 的发布门禁失败；本任务不修复这些既有问题，也不将其列为通过。

## 私有恢复材料与数据库摘要

- 记录 ID：`PREP-001-20261010T190826Z`。私有运维位置：`hostdzire:/root/sub2api-kin/.personal-deploy/baselines/PREP-001-20261010T190826Z`。
- 开始：`2026-10-10T19:08:26.912806+00:00`；备份完成：`2026-10-10T19:08:42.564880+00:00`；最终恢复验收：`2026-10-10T19:16:48.790250+00:00`。
- `record.json` 保存来源/CI/镜像、备份校验、各次验证结果和恢复边界；目录权限 700、文件权限 600。数据库归档、配置及应用数据只保留在该私有位置，没有上传 Git。
- `migration-ledger.json` 仅含全部已执行迁移的 filename、checksum 和 UTC applied_at；checksum 是迁移身份的一部分，不含业务行。
- 迁移总数：**307**。与固定 personal 源码的全部 307 个 SQL 文件逐一核对：没有缺失、未应用项或 checksum 差异；包含已执行的 251–254。
- filename/checksum 账本指纹：`d96e2af9dccac1570317feebfe5fd0c0443340a8e07a722c4a6b9ff22277ef4a`。完整身份/执行时间 JSON 的 SHA-256：`79f0d0708ee9c6995a8e604a14fcf7dd0c7ed1ef100bc32ae76759c644551743`。
- 非系统 schema：**105 张表、1496 个列**。`schema-summary.json` 只含 schema/表名/列数，SHA-256 `a4e3bd17213526cc576149e3ce01a0a45fce816db968e3ccebc534e089e897c8`；完整 DDL 保存在私有 `schema.sql`。

| 备份 | 字节数 | SHA-256 |
| --- | ---: | --- |
| `app-data.tar.gz` | 9246363 | `5b3b9ad261d8075a7b8ed112189bc3383ea153aa581400a9cdc8b23a3947f3da` |
| `postgres.dump` | 62506867 | `ff9872946427bdf664b15742129f6e2ec83ad526b76db59be6aec60351aae38f` |

原成功部署目录中的数据库备份取得于应用升级前，不能替代这个包含 251–254 的当前恢复点。本次重新保存 `postgres.dump`、`app-data.tar.gz`、Compose、环境文件和持久镜像选择。`pg_restore --list` 通过，应用归档完成 gzip EOF/CRC 校验和隔离解压，归档中的应用配置与当前生产配置逐字一致。

## 运行功能开关

运行模式为 `standard`。完整的显式数据库布尔开关（60 项）、容器布尔环境开关及独立服务状态配置保存在私有 `feature-flags.json`，SHA-256 `8a44e3a129b91219c3fdab0458e947425269ed981ed32508586d9bb258d5fea4`。缺失配置不推断为默认值；没有导出完整环境变量、密钥或其他 settings 内容。

| 个人功能或显式运行配置 | 本次观测 |
| --- | --- |
| `available_channels_enabled` | `false` |
| `channel_monitor_enabled` | `true` |
| `ops_monitoring_enabled` | `true` |
| 独立服务状态 | version `2`，enabled `true`，platforms `openai` |
| 状态阈值 | minimum_samples `5`，warning `0.05`，outage `0.9`，abnormal_windows `2`，recovery_windows `3` |
| `AUTO_SETUP` | `true` |
| `ENABLE_SERVER_TIMING` | `false` |
| `GATEWAY_IMAGE_CONCURRENCY_ENABLED` | `false` |
| `GATEWAY_OPENAI_HTTP2_ENABLED` | `true` |
| `GATEWAY_SCHEDULING_DB_FALLBACK_ENABLED` | `true` |
| `GATEWAY_SCHEDULING_LOAD_BATCH_ENABLED` | `true` |
| `REDIS_ENABLE_TLS` | `false` |
| `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS` | `true` |
| `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP` | `true` |
| `SECURITY_URL_ALLOWLIST_ENABLED` | `false` |

上述值是当前生产观测，不是新安装建议。本任务只记录现状，没有调整任何安全或功能开关。

## 非生产恢复演练与复用步骤

演练使用独立 PostgreSQL 18.6 容器和新临时卷，固定为生产所用的 PostgreSQL 镜像；`--network none`、无宿主机端口、无生产卷挂载，并限制为 1 CPU / 768 MiB。数据库认证仅供这个无网络实例初始化，未更换任何生产凭据。

1. 持有现有部署维护锁，核对运行 digest、成功选择、三项服务身份、健康、迁移指纹和配置；不调用部署或回滚入口。
2. 通过 `pg_export_snapshot()` 保持只读 repeatable-read 事务；迁移身份/执行时间、schema、各表行数和 `pg_dump --format=custom --snapshot` 使用同一快照。备份输出写入私有文件，业务内容不进入报告。
3. 在独立容器中初始化隔离库。确认 PID 1 为正式 `postgres` 进程后再检查 `pg_isready`，避免命中初始化临时服务器。使用该容器自身环境中的用户/库名，从私有归档输入执行 `pg_restore --exit-on-error`，保留原 owner、ACL 和完整数据，不使用忽略错误选项。
4. 核对全部 307 条迁移的身份、checksum、执行时间，以及 105 张表逐表行数；行数按表名比较，不依赖未排序 SQL 的输出顺序。
5. 重新导出完整 schema。四处文本重解译差异已单独验证：两个 Unix epoch 时间戳的 `+08` / `+00` 表示经 PostgreSQL 比较相等；两个平台 CHECK 的数组转换经临时表原生解析得到相同约束。其余完整 DDL 逐行一致，不跳过其他差异。
6. 核对恢复库的全部显式数据库布尔开关和服务状态配置；在事务中进行临时表写读、真实 usage_logs.completion_status 原值更新和读取，然后 ROLLBACK。
7. 验证生产容器身份、Compose、环境、镜像选择、应用配置和迁移账本未变，`/health` 通过；删除本轮临时容器、卷及解压目录，保留私有备份和诊断。

最终完整 `pg_restore` 退出码为 0；所有上述对比与读写回滚均通过。验证历次问题和修正保留于私有 record/diagnostics：初始化就绪误判、未排序行数比较、schema 等价表示，以及探针误用不存在的 status 列。最终使用核实过的 completion_status。没有把此前失败或仅可读归档当作恢复成功。

## 风险与回滚边界

- 本轮证明的是非生产数据库完整恢复及恢复材料可用性，没有执行生产切换、真实应用镜像回滚或恢复后完整用户流程。
- 恢复点固定于备份快照；生产此后继续写入。实际灾难恢复会丢失备份后写入，必须另行授权维护窗口并保留当时现场；不能覆盖在线计费数据库。
- 数据库与应用文件分别备份，不宣称跨资源原子快照；配置逐字一致已检查。Redis 当前镜像已固定，但没有把临时缓存备份或跨主机灾备列为已验证。
- 当前材料保留在生产主机私有目录；不宣称已获得异地主机灾备能力。后续迁移、镜像或配置变化后应创建新的恢复点，不复用这份记录证明新状态。
- 当前 `.5-tps.2` 基线不能任意退回 `.3`。应用回退合同仍按[现有部署说明](deploy/personal/README.md)执行并保留扩展 schema/ledger；本任务没有执行该回退。
- 本 PR 只新增脱敏记录；回退文档提交不改动生产。私有恢复材料继续保留，不自动删除备份、回退镜像或恢复数据库。

## 验收

- [x] 已保存可绑定源码 SHA、固定应用/依赖镜像与当前数据库备份的恢复材料，并完成隔离恢复。
- [x] 公开记录和私有 record JSON 不含明文凭据、Cookie、私钥或连接串；敏感配置/业务数据只在权限受限的私有备份中。
- [x] 当前全部 307 个已执行迁移身份及执行时间已有私有记录，与源码、成功部署账本及恢复库一致。
- [x] 至少一次非生产完整恢复及读写回滚验证通过；临时资源清理，生产现场未变。
