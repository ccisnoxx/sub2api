# hostdzire 个人镜像部署

本目录是本机部署控制工具。入口从当前仓库读取个人发布标签与来源记录，通过既有 `ssh hostdzire`、`scp` 将已验证的完整 Python 模块及无凭据配置 bundle 上传到私密临时目录，在服务器执行一次完整操作。工具不依赖本机 Docker daemon，不修改 SSH 配置或 host key 检查。

最后离线验证日期：**2026-10-19**。固定 Linux x86_64 现场的恢复验证见 [BASELINE.md](../../BASELINE.md)；REL-001 交付历史候选部署前合同；REL-002 当前源码绑定及实际发布/部署状态以验收记录为准。

## 使用前提与固定现场

本机需要 Python 3.9 或以上、Git、OpenSSH，`origin` 指向 `ccisnoxx/sub2api`，并持有个人发布标签、上游基线标签以及当前运行 revision 的完整 Git 对象。浅克隆需先获取这些对象；工具遇到缺失对象会停止。服务器需要 Python 3.9 或以上、Linux x86_64、Docker Compose 的 JSON 配置与 `--wait`/`--wait-timeout` 能力、curl。已核实的服务器 Python 为 3.11、Compose 为 v5.1.3。

工具固定使用 `/root/sub2api-kin/docker-compose.yml`、项目 `sub2api-kin`，服务集合必须为 `sub2api`、`postgres`、`redis`。应用容器为 `sub2api-kin`，端口为 `127.0.0.1:10088 → 8080`，命名卷 `sub2api-kin_sub2api_data` 挂载到 `/app/data`。现场变化会停止操作，需要重新审查工具的现场合同。

正式部署会短暂重建一个应用容器。选择合适窗口，保留现有数据库、Redis、网络、卷和旧镜像；正在进行的 HTTP 流与 WebSocket 可能断开。健康验证完成后，还需验收登录、使用记录 TPS 与经授权的网关请求。

## 日常追更与上线顺序

CI 与发布统一按 [CI-000](../../docs/ci/CI-000.md) 操作：稳定升级候选只跑一次手动 Full，普通同树 merge 后由 Personal release 自动复用；不要为新 merge SHA 重跑同一棵树，也不要在中间提交反复触发 Full。复用只减少重复的源码验证，不替代新上游的迁移兼容审查和隔离验证。

1. Release 成功后一次性记录正式个人版本、最终 source SHA、固定 OCI digest、CI/Release run ID 和已获取的可信 origin/main 工具 SHA。应用来源与工具来源分开固定。
2. 确认当前生产到目标版本有已审查的兼容合同，并完成适用的隔离验证。未知迁移/运行树继续拒绝，不为新上游自动放行。目标已缓存且需要只读现场诊断时，可用 `--dry-run`；它不拉取目标、不产生备份，不是备份门禁的替代品。
3. 涉及迁移的现有 REL-002 站点先用可信完整 bundle 执行 `--maintenance freeze`，沿用既有站点，不重新 setup 或修改生产证书。
4. 对固定 digest 执行一次正式部署，保留返回的记录 ID。执行器在同一锁内完成预检、数据库/应用数据/配置备份及完整性检查，**备份通过后才启动新应用**；只替换应用，不重建 PG/Redis。无需手工再复制一套同样备份，不能跳过工具备份。
5. 核对部署成功记录、运行身份、回环健康、完整迁移账本和依赖身份后，用同一可信 bundle 的 `--maintenance resume --target-revision <最终发布SHA> --deployment-record <本次成功记录ID>` 恢复流量，并显式提供同一 `--tool-revision`。
6. 验收公开健康、登录、授权网关请求、SSE/计费及适用的聚合观察周期。保留备份和失败现场；单向迁移失败继续前向修复，数据库恢复另行授权。

发布、冻结、部署和恢复仍是明确的手动动作；CI 复用不会自动上线，也不增加 force/ignore/skip-backup 开关。

## 部署与回滚命令

在仓库根目录指定个人版本；可带 `v` 或完整 fork 镜像版本地址。版本先拉取并解析唯一 RepoDigest，再拉取该 digest；启动、验收和记录均使用相同的不可变引用。

```sh
deploy/personal/deploy-hostdzire.sh 0.2.17-klno.1-tps.1 \
  --tool-revision <已经获取的本-fork-origin/main-完整SHA>
```

固定 digest 必须同时给出对应个人版本；revision 从本机个人 Git 标签读取，再核验实际镜像的 OCI 标签。已核实旧生产 `.4` 版本对应的固定镜像只读预检命令为：

```sh
deploy/personal/deploy-hostdzire.sh \
  ghcr.io/ccisnoxx/sub2api@sha256:7e49c8d375ba9ae4ab9bdef4a03c52b634f234d0cebd9ddd5e77240278e94247 \
  --version 0.2.15-klno.4-tps.1 \
  --tool-revision <已经获取的本-fork-origin/main-完整SHA> \
  --dry-run
```

成功输出包含服务器记录 `id`、目标与实际运行 digest、Image ID、OCI version/revision/source、健康状态。仅声明 `image_rollback_compatible=true` 且与当前成功选择相同的**部署记录**可显式回滚。生产 `.4` 和本次 `.17` 合同均为 false，旧429及跨KIN恢复禁令保持；下面只展示既有兼容记录的参数格式，不能用于本次升级恢复：

```sh
deploy/personal/deploy-hostdzire.sh --rollback 20261008T180000Z-123456789abc \
  --tool-revision <已经获取的本-fork-origin/main-完整SHA>
```

上面的 ID 是格式示例，执行时替换为本次部署实际返回的 ID。回滚提交新的记录与选择 ID，原部署 ID 随即不能重放。新部署输入只允许 `ghcr.io/ccisnoxx/sub2api` 的个人版本或 digest，拒绝 `latest`、其他仓库及 shell 语法。旧 KlN 仓库只允许首次已验证记录的固定 digest/revision/version/source，作为恢复来源，不接受新部署输入。镜像必须为 `linux/amd64`；缺失、错误或不一致的 OCI 标签与 RepoDigest 会停止操作。

## ARCH-002 模块、配置与可信运输

`hostdzire.json` 是唯一无凭据现场配置，完整指定 SSH 别名、工具 scratch、部署路径、Compose 项目/服务、容器/卷、端口、健康地址和维护站点/标记路径。它不包含密码、连接串或私密环境值。schema 拒绝缺失/未知字段、错误类型、非法路径与不一致的回环地址；没有覆盖目标或默认值的 CLI 开关。

| 模块 | 输入 | 输出与权威责任 |
|---|---|---|
| `deploy_config.py` | 完整 JSON 配置 | 不可变现场模型，严格 schema |
| `deploy_trust.py` | 本 fork origin/main 完整 SHA、本机工具路径 | 全部维护源码/config 的 SHA-256 清单、导入前校验 |
| `deploy_local.py` / `deploy_git.py` | CLI 目标、实际旧镜像、真实 Git 对象 | 发布 revision、来源/祖先/runtime 的 canonical 兼容证明 |
| `deploy_transport.py` | 已验证 bundle、现场配置、请求 | 原生 SSH/scp、私密 scratch 清理、绑定退出码的 JSON 结果 |
| `deploy_process.py` | 明确 argv、文件流与 sensitive 标识 | 子进程结果；原始诊断只保存在私密操作记录 |
| `deploy_images.py` | runner、现场配置、预期镜像 | 平台/digest/OCI/Image ID 验证；拉取必须显式调用 |
| `deploy_compose.py` | 原 Compose 字节、配置、明确镜像 | 精确 image 行适配及完整 JSON 布局验证 |
| `deploy_database.py` | 数据库容器 ID、账本/操作文件 | 原顺序账本 fingerprint、固定迁移验证、显式备份结果 |
| `deploy_health.py` | 运行镜像、依赖身份、账本期望 | 容器、回环健康、PG/Redis 身份及迁移验收 |
| `deploy_lifecycle.py` | 已绑定请求、配置、runner | 唯一 Deployment owner；锁、记录、备份时序、提交及失败恢复 |

入口 `deploy_hostdzire.py` 只解析参数并编排这些模块。原私有 import 边界保留给已有只读调用/测试；生命周期故障注入的 owner 已移至 `deploy_lifecycle`。不迁移旧记录，不改变成功选择的格式，也不要求重新部署应用。线上旧 REL-002 维护入口及其 open/closed 状态仍可继续使用；更新本目录不会自动替换服务器已安装的旧控制工具。

必须显式提供 `--tool-revision`，且它等于已获取的本 fork `origin/main` 完整 SHA。shell 入口使用 Python `-I -S -B`；在导入业务模块前，先从固定 main Git blob 读取 trust 实现，再检查所有源码/config 的普通文件、字节、文件模式及暂存/工作树漂移。工作树的既有 pyc 不参与加载，额外可导入文件也会被拒绝。目标应用 tag/revision 与工具 main SHA 各自验证，不能用工具 revision 绕过应用来源。

SSH 一次 scp 运送完整清单。远端 isolated Python bootstrap 在导入任何业务模块前检查所有清单文件的 SHA-256、普通文件与独占 scratch 内容，随后请求再次绑定入口 hash、完整 bundle 和同一可信工具 revision。原单文件只校验 entry hash 与 revision 格式；这里补齐整套运输的信任边界。SSH 中断没有自动写入重试。

## 只读 dry-run

当前固定生产镜像已在主机缓存中，可在工具 SHA 已推送到 main 并获取之后执行：

```sh
deploy/personal/deploy-hostdzire.sh \
  ghcr.io/ccisnoxx/sub2api@sha256:7e49c8d375ba9ae4ab9bdef4a03c52b634f234d0cebd9ddd5e77240278e94247 \
  --version 0.2.15-klno.4-tps.1 \
  --tool-revision <已经获取的本-fork-origin/main-完整SHA> \
  --dry-run
```

成功输出 `status=dry-run`、`will_apply=false`。它取得**既有** deployment.lock 的共享锁，执行同一只读 preflight 合同，验证已缓存目标的 OCI/platform/digest、当前选择/成功记录、Compose 定位、PG/Redis 身份、迁移账本、容器及回环健康，并在返回前复核现场未漂移。缺少状态/锁、目标未缓存或任一证据不完整都失败关闭，不创建或修复运行状态。

此路径不调用 apply，不 pull 镜像，不备份数据库或应用数据，不创建操作记录，不改变 selection/Compose，不启动容器或执行迁移。仅工具运输 scratch 与临时 Compose 镜像变量文件可以写入显式固定的 `/tmp`，不采用 `TMPDIR`、`TEMP` 或 `TMP` 选择目录，操作后清理；它们与运行状态目录分开。Python 不读取私密 `.env` 字节；Compose 进程只读解析必要配置，完整 JSON 仅作为 sensitive 内存校验，不打印/落盘。dry-run 因此明确报告 `private_environment_bytes=not_read`，只验证选定 Compose 的可观察运行合同，不表示完整私密环境漂移已验证，也不重算 `.env` hash；正式 apply 仍按原合同读取私密字节、保存备份并执行 hash 防漂移。只读通过不表示新的部署、备份恢复或核心业务验收已完成。

## 状态与兼容性合同

`Deployment` 是唯一远端生命周期 owner，使用非阻塞 `flock` 持有 `/root/sub2api-kin/.personal-deploy/deployment.lock`，锁覆盖预检、备份、拉取、启动、健康、持久提交及失败恢复。并发部署在执行 Docker 命令前退出。其他生产配置维护也应尊重这把锁；工具的 hash 检查会拒绝已观察到的外部漂移。

本机比较实际旧、新完整 revision 的整个 `backend/`、`Dockerfile`、`Dockerfile.goreleaser`、`.dockerignore` 与 `deploy/`，仅排除本部署工具目录及 `deploy/personal-source.json`。旧基线继续只允许零运行差异、下述精确工具链补丁及固定 S0–S4 累计应用；跨基线只允许下述固定 `.3 → .5`、REL-001 固定 `.5-tps.2 → .15-klno.2`、生产 RESP `.15-klno.2-tps.3 → .15-klno.4-tps.1` 和固定生产 `.15-klno.4-tps.1 → .17-klno.1-tps.1` 正向证明。新 `.15-klno.2` 重新部署也必须保留审定锚点与完整运行树。其他后端、迁移、镜像运行资源、部署配置差异或未知对象都会阻止操作，没有 force 或 ignore 兼容开关。首发 `de08df02… → 896de21b…` 在这些路径仅新增来源记录，满足零差异合同。

本轮经用户三次明确授权，升级 Go 1.27.0 → 1.27.2，x/net v0.58.0 → v0.60.0，并按模块最小版本选择同步 crypto0.57、sys0.48、term0.46、text0.42、tools0.50、mod0.41、sync0.23。精确 lint 兼容规则仅匹配五个固定文件中的既有 HTTP/2 弃用接口，其他检查继续启用。例外绑定已审定的 `backend/.golangci.yml`、`backend/go.mod`、`backend/go.sum`、`Dockerfile`、`deploy/Dockerfile` 前后完整文件 SHA-256；应用候选为 `ce682c4901033d09c7e12614b7b73e11bdd17ef4`，旧输入为 tps.1 revision `896de21b4be7f4ec4b4236f4df663b47371665b0`。五个路径必须全部变化、方向一致，双方均为普通 `100644` 文件。任意其他版本、校验行、字节、权限、业务源码、迁移或运行配置变化均拒绝；未来安全补丁需要另行审定，不能按文件路径泛化放行。CI/Release 精确核对所选源码 go.mod 的 Go 声明并继续全部门禁，personal 的声明固定为1.27.2；main 不维护应用树。

兼容证据记录补丁ID、方向和双方文件 SHA-256，远端按同一已审定清单验证，并绑定实际旧/新 revision。证据总 hash 校验本机传输完整性，不是远端重新执行 Git 审计或镜像构建证明。原有零差异证据和成功记录继续有效。精确补丁允许反向镜像故障恢复；普通 deploy 旧版本会在备份/拉取/启动前被拒绝，须使用绑定当前成功记录的 --rollback，或由失败部署自动恢复。恢复仍须满足成功记录、配置、迁移和依赖条件；旧Go及x/net有已知漏洞，因此不能长期停留旧安全版本。

本轮另外审定的跨基线升级只接受 `v0.2.14-klno.3 / de08df02ae1d81668a22f798b398aa0438ac1276` → `v0.2.14-klno.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。旧运行树固定取自 tps.1 `896de21b4be7f4ec4b4236f4df663b47371665b0`，新树取自融合候选 `c00e8982258736a188a09023e901d8d39671814f`。对全部运行条目的路径、Git mode、type 与 object ID 排序并计算 SHA-256，旧树为 `ecf6eff8c2902b29a1a689232dc8a1028e9d10e5615bcb6e293d35d8d278df35`，新树为 `928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d`；同时绑定精确的 151 个运行差异路径。最终 PR 合并 revision 可以不同，但来源与整个运行树必须相同。修改任意已有、未变化或新增运行路径的内容、类型、权限、迁移或来源都需重新审定。远端公共 validator 检查证明类型、hash、所有固定字段、来源、完整差异列表、实际旧新 revision 及镜像版本；不能借零差异或五文件安全补丁绕过跨基线合同。

此正向证明明确记录 `image_rollback_compatible=false`。`.5` 的 `account.extra.openai_gwpool_usage_rounds` 新增 `rounds.active_usage` 与归档的 `active_duration_ms/incomplete`，contacts 新增 `previous`；`.3` 的类型解码与 whole-key `UpdateExtra` 会丢失这些字段。`.5` reporter 在 HTTP 监听与 health 之前启动，cooldown 首次 arm 也可能写入持久化数据。因此迁移清单未变化、启动等待失败或 `/health` 尚未成功都不能证明旧镜像恢复无损。该升级拒绝 `.5 → .3` 普通部署、显式回滚，以及新应用启动后的自动旧镜像恢复。需要另行评估数据兼容的向前修复或经授权的维护恢复；数据库恢复与可能丢失写入不在部署工具的授权中。

S0–S4 的 `klno.5-tps.1-to-s0-s4` 证明固定旧应用 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92 / 0.2.14-klno.5-tps.1`，保留双方 `.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d` 来源。新候选锚点为 `d72a3dafbdcb1edb642fca8dcfb29a803eaa97af`，旧完整运行树为 `928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d`，新树为 `ecbf079cd17f89f7604987d35a2740ea90264d7327178ed5c7ee2d61e73f9747`，精确 128 个差异路径以 `deploy_compatibility.py` 的 `CUMULATIVE_CHANGED_PATHS` 为准。正式个人标签的 merge revision 可以不同，但必须包含审定候选且整个运行树完全相同。本机核对真实 Git 对象，远端核对 canonical 清单、证明 hash、实际旧新 revision 与镜像版本。不能删掉专属证明、伪装成零差异或安全补丁放行这次累计运行差异。

累计证明只允许新增 `251_add_usage_log_timing.sql`、`252_add_ops_routing_diagnostics.sql`、`253_service_status_observation.sql`、`254_service_status.sql`，分别固定 Go runner 使用的 `SHA256(strings.TrimSpace(SQL))`。251 的追加计时/终态字段包含旧写入可使用的默认值和可空列，252/253 追加可空 JSONB，254 创建独立服务状态表并安装分组可见性 trigger；兼容结论绑定这些完整 SQL 和固定旧应用的真实扩展 schema 证据，不推广为“所有追加迁移均安全”。回退只恢复应用镜像，保持扩展 schema 与 ledger，`schema_rollback=retain`。

远端取得锁后再次读取实际旧 digest、revision、version、source、Image ID，与本机兼容证据绑定的旧状态比较。PostgreSQL `SELECT 1`、Redis `PING` 与容器 healthcheck 均须通过；数据库/Redis 容器 ID 在整个应用更新中保持不变。通过 `schema_migrations` 的 `filename/checksum` 数据库排序清单记录行数与 SHA-256，保留数据库实际顺序。通常更新后以及自动恢复前必须完全一致，未知迁移差异阻止恢复。已有成功记录的迁移状态后来发生变化时，同样禁止后续部署或显式回滚。

累计升级要求启动前账本与当前成功记录相同，所有原行逐字不变，健康终态只能新增全部四个固定 filename/checksum；新成功记录保存扩展后的 fingerprint。启动失败或取消时，自动恢复只接受无新增或按 runner 事务顺序提交的固定前缀，并在替换旧应用后再次核对；未知行、旧行变动、错误 checksum、缺口、重复记录、配置/依赖漂移或未绑定的实际运行镜像均明确阻止恢复。固定新增迁移可能在 replacement 停止新应用前完成下一笔事务，恢复后仍只能观察到相同原行和不倒退的固定前缀。

自动恢复保留原 selection，不改写旧成功记录。失败操作若已经提交任何新增迁移，旧镜像虽恢复健康，下一次自动部署仍会因原成功记录与扩展账本不一致返回 `E_MIGRATION`，需要先人工核对失败记录和账本。成功累计部署可使用当前部署 ID 显式切回固定旧 `.5`，通过健康后提交新的回滚选择与完整扩展账本；之后只有这份成功回滚记录所绑定的同一扩展账本可再次升级审定累计应用。普通 `deploy` 不允许反向切回。

REL-002 的 `klno.5-tps.2-to-0.2.15-klno.2` 专属证明只接受固定旧发布 `3d5e1fde82707900a21f2b5538b112c7bab3c04a / 0.2.14-klno.5-tps.2`、digest `sha256:7446ff8ebdddca5e60989f0a5b8dec670ce3a030a9c8472e2dd3c27986e21530`，由旧 `.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d` 切换至新 `v0.2.15-klno.2 / ca686de5e5ac14834e12701c156831abd5ec6b71`。新运行锚点是 `42907bcab278f93db48d7e1b39b28e28f664569e`；旧完整运行指纹是 `ecbf079cd17f89f7604987d35a2740ea90264d7327178ed5c7ee2d61e73f9747`，新指纹是 `90e2851678966e6c27e41f5d7ec6e4d0c1196f2a7a8c0c81a1ba84a34c30e7a6`。精确 292 个排序差异路径的 canonical JSON SHA-256 为 `e22db48c79378e47c4b30d02541d83476aede91a58886afa6dcf794562e21c24`，证明仍保存全部实际路径。正式 merge SHA 可以不同，但必须包含新锚点，完整指纹、精确差异和来源三字段必须一致；任意未知运行树，即使旧新树之间零差异，也不能冒充新基线重新部署。本机生成与远端验证复用同一 canonical 合同。

此升级只追加 `242_drop_platform_check_constraints.sql`，其 runner checksum 固定为 `a7725f31f637208d200e0f37d5270e9f263743d99fa9e4f1bb9a7387e8be5a61`。远端初始账本必须是 [MIG-001](../../docs/migrations/MIG-001.md) 的固定 307 行，filename/checksum 指纹为 `d96e2af9dccac1570317feebfe5fd0c0443340a8e07a722c4a6b9ff22277ef4a`；旧行的数据库顺序、内容以及 251—254 必须保持。启动前再次核对账本，健康终态只允许原账本加唯一正确 242，记录完整 308 行指纹。再次部署已审定的新树仅接受完整正确扩展账本和当前成功记录一致，不能把缺失、错误、重复的 242 或其他账本变化收编为已完成升级。242 与其账本插入由原 runner 的单文件事务拥有；本工具不修改 SQL、runner、历史 checksum 兼容规则或数据库账本。

REL-001 证明明确保持 `image_rollback_compatible=false` 和 `schema_rollback=retain`。本轮没有证明跨多版本 `account.extra` 等持久业务数据无损回退。[UPSTREAM-007](../../docs/upstream/UPSTREAM-007.md) 与 MIG-001 的合成库旧应用 `/health`、扩展 schema 与备份恢复结果，仅支撑其已记录的具体边界，不能代替全业务数据兼容证据。新应用启动、取消、健康或 selection 提交前失败后，工具保存失败现场、原始错误、实际镜像、原选择与扩展账本，明确阻止自动旧镜像恢复；成功跨基线记录的显式回滚和普通反向部署同样拒绝。新基线重新部署也不获得镜像恢复豁免。需要人工核对失败记录，选择经过验收的前向修复；覆盖数据库或应用数据的维护恢复必须另行授权，说明备份后写入损失并保存现场，不自动删除 242 或恢复被移除的约束。

OBS 的 `429-tps.1-to-obs-tps.2` 专属单向证明从已发布的 `42907bcab278f93db48d7e1b39b28e28f664569e / 0.2.15-klno.2-tps.1 / sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3` 升级至包含测试门禁修复锚点 `b663258dd6c93a794f2dcfe716979e4dd37eaccd` 的 `0.2.15-klno.2-tps.2`。双方来源仍须逐字段等于 `KIN_NEW_SOURCE`。新完整运行树指纹固定为 `3d3a9765382ecc98a2a9ad95d6940377b618d1f3e1e5675a16b734b6cfccb9c0`，精确 32 条运行差异固定在 `OBS_CHANGED_PATHS`。Release 会单独构建并嵌入前端，因此本证明另绑定整个 `frontend/` 的路径、mode、type、object：旧指纹 `ef9884449ee278c275c02b7fd06c34978116b8c3d7c8052694d46d118aa52f4c`，新指纹 `47b21affa698058f84a71bcce078c4e105965017a9f34c5707886d25a3e0a367`，精确 13 条前端差异固定在 `OBS_FRONTEND_CHANGED_PATHS`。历史运行路径与证明常量保持原合同。OBS源码整合锚点仍为423f690；新锚点仅修正测试类型断言及repository执行分片，45秒/40秒行为合同和每进程60秒限制不变。

同一 OBS tps.2 完整运行树及前端可重新部署，实际旧、新完整 SHA 都必须包含 OBS 锚点。文档及 `deploy/personal` 控制后继允许改变最终 SHA；任何新增应用内容、模式、类型、来源字段或同字节缺失锚点的树均拒绝。生成和验证兼容证明只依赖固定应用锚点与真实 Git 对象，Full 中的合同检查不依赖尚未创建的 tps.2 标签或最终文档 SHA；实际部署及维护恢复仍核对正式个人标签、OCI 标签与完整发布 SHA。tps.2 专属 OBS，不能改用旧 `kin_upgrade` 零差异证明来降级放行。

OBS 没有新增 SQL 或 schema 迁移；证明保留 242、251—254 的固定内容和普通文件模式，生命周期要求正确完整 308 行账本与当前成功记录相同，启动前及健康终态均不得变化。仍记录 `image_rollback_compatible=false`、`schema_rollback=retain`；启动或健康失败保留失败记录、现场新镜像和原选择，禁止自动、显式或普通部署切回 429/旧 KIN。需要人工前向修复或另行授权的数据恢复。

RESP 的 `obs-tps.2-to-resp-tps.3` 专属证明只接受实际已发布的 `c08fba12eb4cd30e65c4177f3acbe20567ecd252 / 0.2.15-klno.2-tps.2 / sha256:c4ffab7a920f05564eafe055bdb3171772b145ca5e2ccda690fe8b554bf6fa25`，正向升级至包含 PR50 与门禁修复合并锚点 `8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90` 的 `0.2.15-klno.2-tps.3`。双方来源逐字段等于 `KIN_NEW_SOURCE`；旧运行树为 OBS 固定树，新完整运行指纹为 `9f0490b6a346e885dc1e9ff4d9296b15e4f71161a4d9458072cf4ba18f170a48`，23 条精确运行差异见 `RESP_CHANGED_PATHS`。本机同时核对完整 `frontend/` 指纹 `47b21affa698058f84a71bcce078c4e105965017a9f34c5707886d25a3e0a367`、外部两份 raw legal 文档指纹 `70666fc568db23059802282f36ec4d3fa25a3a2cc0fdc3bbd56c8a24a5808942`，两端全部前端输入相同。后继文档或部署工具提交只有保留锚点及这些完整输入才能重新部署；未知树即使零差异也拒绝。tps.3 仅属于这份 RESP 证明，不能用旧 KIN 或 OBS 证明伪装放行。

RESP 没有 schema 或 SQL 迁移，242、251—254 内容和文件模式继续固定。远端预检要求当前成功记录与正确完整 308 行账本一致，启动前再次核对，健康终态仍须保持同一账本；成功记录保存本次 canonical RESP 证明、健康观察、目标 digest/Image ID 和实际旧身份。证明保持 `image_rollback_compatible=false`、`schema_rollback=retain`。启动、健康、取消或选择提交前失败保留现场和原选择，阻止自动旧镜像恢复；选择已替换而落盘失败时保留失败记录与已提交选择，后续操作拒绝自动接管。反向 C08、429 或跨 KIN 部署/显式回滚均拒绝，本证明不授权数据库、账本或业务数据恢复。离线合同不要求尚未发布的 tps.3 标签；实际部署必须从真实正式标签读取完整源码 SHA，并核对目标 OCI 元数据。

`.15-klno.2 → .4` 的 `klno.2-tps.3-to-klno.4-tps.1` 专属单向证明，只接受当前生产发布 `8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90 / 0.2.15-klno.2-tps.3 / sha256:a216fd8fb138aa1889278df7fbc4a0dd24a09d974886f198cc5102eac6a1b627`，旧来源逐字段固定为 `v0.2.15-klno.2 / ca686de5e5ac14834e12701c156831abd5ec6b71`。新来源固定为 `v0.2.15-klno.4 / c555ad918b08cb24ccb2eb45f4ae44a843e24baf`，已提交应用锚点为 `045cd71115d48111258bb051d275ca385d4078ed`，正式版本只接受 `0.2.15-klno.4-tps.1`。真实正式个人标签必须指向实际 OCI 完整 revision；来源标签必须精确等于记录的上游 SHA，且上游和应用锚点都是发布 revision 的祖先。正式合并以及文档/部署工具后继可以改变最终 SHA，但不能改变应用构建输入。发布版本由发布工具实际分配；若不是上述版本，必须重新审定合同，不猜测最终 merge SHA 或镜像 digest。

本机精确核对新完整运行树 `102aac962476b496dc29ef33610e31adefb9fb6e60e7065c77e803c6849e71a2`、完整前端树 `71c1f6c1b119fa6b4b4f241a7ca8e92be59cf9fbd1c6c38d7d8f121115df27b6` 和外部 raw legal 输入 `70666fc568db23059802282f36ec4d3fa25a3a2cc0fdc3bbd56c8a24a5808942`。220 个完整排序运行差异路径的 canonical JSON SHA-256 为 `7129c5c47d76706d4881ca4a214c4164294f554d6161917e91c93595f47d0c15`，36 个前端差异路径的摘要为 `e65693f08330499c200af40cbf84dadc46792c2fce8fe65ff48f65c109a9909d`；证明保存全部实际运行差异以及 `KLNO4_FRONTEND_CHANGED_PATHS` 的完整精确前端清单。改变未变化文件、Git mode/type、来源三字段、前端或外部 raw 输入均拒绝；未知树零差异、缺失候选祖先、旧 KIN/OBS/RESP 证明伪装、反向旧来源部署及回滚也拒绝。同一正式版本的固定新完整树可以重新部署。

此合同保留全部既有 SQL 字节及文件名，特别是个人 `251_add_usage_log_timing.sql`、`252_add_ops_routing_diagnostics.sql`、`253_service_status_observation.sql` 和 `254_service_status.sql`；同数字前缀不代表同一 filename。只允许追加三项普通 `100644` SQL，runner checksum（`SHA256(strings.TrimSpace(SQL))`）分别为：

| 新迁移 | 固定 checksum |
|---|---|
| `251_drop_gwpool_previous_ledgers.sql` | `7d0bbc6f82ce83b9a873a884908cd6a250f4f0a9b6ec991a41201956fbc9d1d1` |
| `252_drop_retired_turn_state_and_gwpool_keys.sql` | `687f8f1978bd92b75291a56aa2c13c0b2e01b1ad1f4436106211446d65c1512b` |
| `253_move_gwpool_ledgers_to_table.sql` | `0e84fa98c52d09464d644980ba6f514195c319d4554e5244c11885067d7ffb06` |

旧生产账本必须是固定 308 行，filename/checksum 摘要 `a31c942e9e772bef1af9c68f930ee35afaa8c3591b5d1c22609f9bf7d30da882`，同时等于当前成功记录。备份和原账本保存完成后、新应用启动前再次逐字核对。runner 按文件名排序，已存在个人迁移直接校验后跳过，新三项分别由单文件 SQL 与账本插入同一事务提交；生命周期健康终态必须保留旧行及其数据库实际顺序，只追加全部三项正确 checksum，形成 311 行账本。同树重部署必须从完整正确 311 行及对应成功记录开始，部分或伪造终态不接管。

251 删除池账本的 previous 标签及嵌套归档；252 删除旧 turn-state 猎手、自动接管、恢复、手动覆写等配置/运行态、池 metrics/prepare_retries，并移除 `model_rate_limits` 中 `turn_state_hold` 条目；253 将 gateways/contacts/usage_rounds/feedback_outbox 对象账本搬入 `gateway_pool_ledgers`，随后从 `accounts.extra` 删除这些键。无效的非对象值不搬入新表，但原 extra 键仍会被删除。旧 .2 应用读写 extra，不能通过镜像切回恢复已删除字段或保证新表写入无损。生产已核对 gwpool 缺省关闭且未配置池地址或消费凭据，本合同不要求外部池升级；这些事实不取消 SQL 对存量字段的破坏性边界。

`.4` 证明始终为 `image_rollback_compatible=false / schema_rollback=retain`。启动等待、健康、取消、账本检查或选择提交前失败后，保存实际镜像、诊断、原选择与数据现场；尽力只读记录 0—3 项固定事务前缀及观察账本指纹，未知差异记录 `migration_observation_error`，不把前缀视为恢复许可。即使观察到零项提交，也不自动或显式启动旧镜像。选择已替换后的失败保留已提交选择并拒绝自动接管。失败记录与旧成功选择不一致时，需要人工核对并经授权前向修复；恢复数据库必须另行说明备份后的写入损失、隔离恢复验证和切换目标，本工具不删账本、不恢复 previous/turn-state 字段、不从新表反搬数据、不提供 force/ignore。

`.4` 继续使用既有 REL-002 站点、持久标记与历史安装 `source_sha=42907…`，不重新 setup。可信 main 完整 bundle 的显式 resume 按正式 `.4-tps.1` 标签和精确构建输入校验本机目标；远端要求专属 canonical `.4` 成功记录、当前 selection、实际 OCI/digest/Image ID、健康和正确完整 311 行账本一致，才删除关闭标记并追加 accepted 身份。旧无参数入口仍只接受历史 429，不能以任意 running digest 或旧 RESP 证明开放 `.4` 流量。

当前 `VERSION=0.2.15-klno.2-tps.1-rc.1` 仍被版本、镜像元数据及个人发布标签合同拒绝；没有预发布部署、force/ignore 或生产允许开关。旧 .2 合同内的正式发布必须具有真实个人 tag、对应完整 revision、`0.2.15-klno.2-tps.N` 前缀、正确 OCI 元数据和审定完整运行树；更改运行范围内的 VERSION 或其他字节必须重新固定并审查锚点，不能沿用本指纹。正式发布和 REL-002 获得相应授权、完成发布身份与现场前置验收后才执行部署；离线替身和真实Git证据本身不表示真实部署；当前正式tps.2/C08及immutable C4镜像已完成测试→生产，见[OBS验收](../../docs/upstream/records/2026-10-12-obs-release.md)。

首次适配只修改普通 `services/sub2api` 中唯一的直接 `image` 行为 `"${SUB2API_IMAGE:?必须指定应用镜像}"`；不重新序列化 YAML，原注释、CRLF、其他服务和设置逐字保留。特殊缩进、锚点或无法唯一定位的结构会失败，不能用仓库 Compose 模板替换生产文件。适配前后在内存中核对完整 Compose JSON，仅应用 `image` 可变化，完整配置和环境值不会进入日志或输出。

持久镜像选择由 `.personal-deploy/image.env` 单独拥有，包含固定 `SUB2API_IMAGE` 与 `SUB2API_DEPLOYMENT_ID`。操作期间使用私密 `operation-image.env`，显式传入 `.env`、操作镜像文件与同一个进程镜像变量，抵消已有 shell 或 `.env` 的同名镜像变量。通过全部健康验证才原子提交持久选择；不要用普通 Compose 命令或外部环境变量绕过此选择合同。

唯一运行更新命令是：

```text
docker compose --env-file <现有.env> --env-file <私密操作镜像文件> \
  -p sub2api-kin -f docker-compose.yml \
  up -d --no-deps --wait --wait-timeout 120 sub2api
```

数据库和 Redis 仅被检查、读取与备份，不执行其更新或重建。工具不执行 Compose down、镜像 prune 或卷删除。

## 备份、失败与恢复

每次操作生成 `.personal-deploy/records/<ID>/`。状态目录与记录目录权限为 `700`，记录、镜像选择、配置备份、数据库与应用数据备份、诊断文件权限为 `600`。记录保存工具 Git revision 与入口及完整工具 bundle SHA-256、旧/目标镜像元数据、兼容证据、时间、阶段、配置/.env hash、迁移指纹、备份信息和退出状态。服务器 `.env`、应用配置、数据库、应用日志留在服务器，不输出凭据或上传 Git。

执行顺序是预检、备份、目标拉取与 OCI/平台校验、最小 Compose 适配、仅应用启动、完整健康核对、持久提交成功选择。备份完成在新进程启动之前。更新使用一个应用 Compose replacement 顺序替换；不并行启动共享 Redis 的新旧应用，避免 `CleanupStaleProcessSlots` 清理另一进程的 slot。备份包括原 Compose、`.env`、已有镜像选择、`pg_dump --format=custom` 和 `/app/data` 归档。数据库 dump 以文件流送入 `pg_restore --list`，应用归档先枚举 tar 成员，再分块读取到 gzip EOF，完成 CRC 与长度校验；两份数据备份记录大小及 SHA-256。数据库与应用归档是在运行期间分别取得，恢复时需要人工核对业务一致性；`pg_restore --list` 证明结构可读取，实际完整恢复仍需隔离实例演练。

| 情况 | 状态与恢复 |
|---|---|
| 预检、兼容证据、备份或拉取失败 | 非零退出；旧运行容器、持久镜像选择及原 Compose 保留 |
| 拉取成功后配置校验失败，尚未启动 | hash 核对无漂移时恢复本次 Compose 适配；保留失败记录 |
| 零差异或精确安全补丁应用启动或健康失败，兼容证据有效且数据库/迁移/配置未漂移 | 用旧固定 digest 仅重建应用并验证健康，恢复本次 Compose 适配；原选择保留。`rollback_status=success` 仍是部署失败，退出码 `1` |
| S0–S4 累计升级启动、取消、健康或 selection 写入前失败 | 仅在固定追加兼容证明、原行不变和固定事务前缀、旧新运行镜像、配置与依赖合同均成立时恢复旧 `.5`，保留新 schema/ledger 和原 selection；检查失败记录 `rollback_status=blocked`，实际恢复启动/健康失败记录 `failed` |
| `.3 → .5` 新应用启动或健康失败，持久选择尚未提交 | 保留 `status=failed`、原始错误、诊断与 `observed_running`，记录 `rollback_status=blocked`、`rollback_error=E_COMPATIBILITY`；不启动旧镜像、不替换原 selection，保留当前 Compose 供人工核对 |
| REL-002 新 KIN 启动、健康、取消或 selection 提交前失败 | 保留失败记录、实际新镜像、扩展 schema/ledger 和原 selection；记录 `rollback_status=blocked`、`rollback_error=E_COMPATIBILITY`，禁止旧镜像恢复；人工前向修复或另行授权恢复 |
| `.15-klno.2 → .4` 启动、健康、取消或选择提交前失败 | 保留新应用与数据现场、原选择和失败记录，只读记录可证明的固定事务前缀；`rollback_status=blocked / E_COMPATIBILITY`，禁止旧镜像及数据库自动恢复 |
| 数据库/Redis 容器、迁移或配置发生漂移 | 阻止自动镜像切回，保存失败与当前可观察状态，需人工评估 |
| 同基线显式回滚 | 绑定当前成功部署 ID、旧/目标完整元数据、配置 hash、双方 Git 兼容证据与迁移指纹；通过健康后提交新的回滚记录与旧 digest 选择，保留已完成的 Compose 参数化 |
| `.3 → .5` 成功记录的显式回滚 | 本机证明与远端成功记录绑定都拒绝旧镜像恢复，返回 `E_COMPATIBILITY`；运行与原 selection 保留 |
| REL-002 新 KIN 成功记录的显式回滚 | 本机反向兼容检查和远端成功记录绑定均拒绝，返回 `E_COMPATIBILITY`；不备份、拉取或启动回退目标 |
| 持久选择已替换但后续落盘/成功记录失败 | 不猜测持久提交结果，也不覆盖选择；保留失败记录与 `selection_committed=true`。后续部署/回滚因记录不一致停止，需人工核对 |
| SSH 中断或无有效结果 | 本机明确失败，不自动重新执行写入；先核对服务器记录、运行镜像、配置与选择 |

远端可处理 SIGINT/SIGTERM，按兼容条件保存失败并尝试恢复；SIGKILL、主机重启或磁盘无法写入无法保证收尾。中断阶段、成功记录与选择不一致，或个人镜像运行却缺少已提交的镜像选择时，工具会停止自动接管。修复前应核对私密记录中的 `status`、`observed_running`、迁移指纹与配置 hash；不要仅凭容器运行或 `/health` 认定已完成部署。

工具只恢复应用镜像，不自动恢复数据库、Redis 或应用数据。迁移不兼容、备份恢复及可能丢失备份后写入的情形，需要另行制定经授权的维护与恢复操作。观察期内保留记录、备份、旧 digest 与本地旧镜像。

## .15-klno.4 → .17-klno.1 离线升级合同

本次仅增加离线合同，未发布镜像、升级生产或证明业务回退安全。已核实的旧生产身份为 `2d532db30a94e5d3bff3ef928323186c1d4d93c5 / 0.2.15-klno.4-tps.1`、镜像 `sha256:7e49c8d375ba9ae4ab9bdef4a03c52b634f234d0cebd9ddd5e77240278e94247`，成功选择记录 `20261013T101751Z-4b2b5c9d445b`。旧来源固定为 `v0.2.15-klno.4 / c555ad918b08cb24ccb2eb45f4ae44a843e24baf`，新来源固定为 `v0.2.17-klno.1 / cf0bf0526704bb615745fda76009cb20d85df613`；双方来源三字段各自精确匹配上述固定记录，两个上游标签及候选祖先都必须真实可核验。

`klno17_upgrade` 绑定已提交候选 `d77a512175892111b94e659c3f3ffb1223d1bddc`，正式版本只接受 `0.2.17-klno.1-tps.1`。候选源码内 `VERSION` 仍为 RC，实际发布版本由发布工具分配并通过正式个人标签及 OCI 标签绑定，RC 不能作为部署输入。候选包含 legacy 调度测试的全局设置缓存隔离修复，生产逻辑不变。正式普通 merge 及文档/控制后继可以改变最终 revision，但必须包含该候选且所有构建输入相同；新镜像 digest 不作猜测。

新完整运行树指纹为 `16b775db55e3855b86b310d16e17131100502147fe3833baa980fe7fababe33b`，整个前端指纹为 `eca3f8a22cf25f2ddbd07498fb3ac70d121e998da926bdf085fbc26b70bb2704`，两份外部 raw legal 输入仍为 `70666fc568db23059802282f36ec4d3fa25a3a2cc0fdc3bbd56c8a24a5808942`。304 条排序运行差异的 canonical JSON SHA-256 为 `2ab0b9d6206f512bd09fbb06ebd722049328c67aa470af2dc9673e0147a019b9`，65 条前端差异的摘要为 `a21eaa87d4dfc4076208cf8bd1ac212db04ed0470307232be8a2e334b5dcb135`。证明保留全部运行差异路径和 `KLNO17_FRONTEND_CHANGED_PATHS` 完整列表；内容、mode、type、来源、祖先、未知零差异树和重新 hash 后伪装均不能放行。

全部旧 311 份 SQL 文件逐字保留，只追加以下普通 `100644` 文件；checksum 使用 runner 的 `SHA256(strings.TrimSpace(SQL))`：

| 新迁移 | 固定 checksum |
|---|---|
| `242_group_i18n.sql` | `e81c6818cc806bc54365c993f4cec1634f16a311451faf5cb1a875a1358774cd` |
| `243_gemini_cached_contents.sql` | `2d345cff5c64b01e705f04ec972e5d5ec3d618fe71dc7d45bc80fdd38c6a89d5` |
| `244_gemini_cached_contents_channel_usage.sql` | `9072cb37752a23c3a29cc0330a69837f09c5102375ee053d7bccf14634d3da29` |
| `245_gemini_cached_contents_mutation_lease.sql` | `75434a7beeb8aad09b3339708ed520604d153e99c7fe680a376e8c958aadb1e8` |

启动前账本必须等于当前成功记录及固定生产 311 行摘要 `d59f2470d37390ba808e4d16e7a74ee531b431f4b74f2eb82e9b3236402a3c31`。原行及数据库实际排序逐字保留，只允许四项固定迁移按 runner 顺序形成 0—4 提交前缀，健康终态必须完成 315 行；重部署只能从完整正确终态开始。未知/重复/缺失迁移、旧行或 checksum 改变、备份后账本漂移均拒绝。工具不执行 SQL、不改历史账本；备份仍须在新应用启动前完成，部署锁、依赖身份、app-only、配置漂移检测及 selection 原子提交保持既有合同。

新 `cachedContents` mutation 租约/恢复和新增 `account.extra` 业务字段尚未证明对旧 `.4` 镜像全业务无损，故始终记录 `image_rollback_compatible=false / schema_rollback=retain`。新应用一旦尝试启动，等待、健康、取消、账本或 selection 提交前失败只尽力记录实际账本、0—4 前缀、运行镜像和原选择，不启动旧镜像；零项提交也不豁免。已替换 selection 后失败保留已提交选择并拒绝接管。成功记录的显式 rollback、普通反向 deploy 及同树 redeploy 的旧镜像自动恢复同样禁止。恢复路径为核对私密失败记录后经授权前向修复；恢复数据库、清理租约或外部缓存必须另行评估和授权，本工具没有 force、ignore 或数据回滚入口。

`.17` 复用现有 REL-002 站点/标记/维护记录，不重新 setup。可信 main 完整 bundle 的 `--maintenance resume` 必须显式绑定真实正式个人标签与 OCI 的最终完整 revision、当前成功部署 ID、专属 canonical 证明、digest/Image ID、容器及回环健康，以及正确完整 315 行账本，才在同一锁内开放流量；历史安装身份 `source_sha=42907…` 保持不变，追加本次 accepted 身份。旧无参数入口和旧 `.4`/RESP 证明不能开放 `.17` 流量。上面的新版本命令须在独立复核、正式构建和发布门禁通过后使用；固定旧生产命令仅供升级前只读预检，不能用于 `.17 → .4` 反向部署。

`test_klno17_deployment.py` 使用真实 Git 对象与隔离临时普通 merge 核验候选/来源/完整构建输入、四份新增 SQL 和全部旧 SQL 字节；账本及生命周期替身覆盖固定 311→315、全部前缀、篡改/伪装、启动前备份、账本漂移、失败不回退、选择提交两侧与维护失败关闭。测试不访问生产、不执行真实迁移或外部 Gemini mutation，也不替代发布镜像、数据库和现场业务验收。

## 定向验证与 CI

以下命令不访问 hostdzire、不使用真实 Docker、不安装依赖；CI checkout 应使用 `fetch-depth: 0` 并保留个人及上游标签，发布绑定用例需要已发布固定对象。

```sh
python3 -m unittest discover -s deploy/personal -p 'test_*.py' -v
sh -n deploy/personal/deploy-hostdzire.sh
python3 -m py_compile deploy/personal/*.py
git diff --check
```

替身覆盖成功仅更新应用、版本解析后固定 digest、拉取/备份失败无运行变更、健康超时及自动恢复、显式回滚与过期拒绝、真实 `flock` 竞争、运行 revision 漂移、OCI 标签/source/platform/RepoDigest 拒绝、迁移禁止部署与回滚、配置漂移保留、选择落盘失败及部分提交。真实固定 Git 对象与临时 Git 保护完整运行路径 diff、来源/tag 绑定、`.3 → .5` 正向及反向拒绝、运行树内容/模式/类型/迁移/来源漂移；远端替身覆盖正向证明字段与 hash 篡改、成功 app-only 生命周期、启动前备份、启动/健康失败阻止旧镜像与保留原 selection、单向记录显式回滚拒绝及部分提交。累计合同另保护固定四迁移、原账本字节、部分事务与取消恢复、未知差异阻止恢复、恢复后重试拒绝、显式回滚后再次升级、selection 部分提交、证明伪装及最终 merge 祖先/完整运行树检查。真实本机子进程验证 dump 文件流和私密错误边界；OpenSSH 替身验证固定别名、scp 和通过 stdin 传输 JSON。正式前还需独立只读审查以及现场健康、界面与网关验收。

新 KIN 合同使用真实固定 307 份旧 SQL 构造离线账本，验证仅追加 242 和正确完整账本重新部署；保护错误/缺失 242、旧行变化、251 缺失、重复/未知行、canonical 证明篡改与伪装、固定旧 digest/版本、新来源正式版本前缀、预发布拒绝、未知树零差异、锚点祖先及运行内容/权限/类型/来源漂移。启动、健康、取消与 selection 提交前失败验证均明确阻止旧镜像；这些替身不执行真实业务数据回退。

`test_resp_deployment.py` 用真实 Git 对象核对 C08 → RESP 精确运行差异、全树/来源/锚点、前端与 raw 输入不变；隔离临时 Git 标签验证维护恢复的正式发布绑定。生命周期替身直接覆盖成功/同树重部署、只读预检、重算 hash 后篡改或伪装证明拒绝、旧 digest 绑定、失败不恢复 C08、选择部分提交和完整 308 行账本。维护替身覆盖 canonical RESP 成功记录、当前选择、OCI/Image ID、原站点/标记、伪造成功账本及无显式目标拒绝，真实主机验收仍由发布维护者执行。

`test_klno4_deployment.py` 使用真实固定 Git 对象验证220条运行差异、36条前端差异、来源/tag/锚点和仅追加三项 SQL；隔离临时标签保护正式维护恢复绑定。账本与生命周期替身覆盖固定原308行、三项0—3提交前缀与311完整终态、原行/顺序/checksum/缺口/重复/未知行拒绝、重算hash伪装、只读升级预检、备份先于启动、取消/等待/健康失败禁止旧镜像、部分提交后拒绝接管、选择部分提交及维护失败关闭。离线合同不执行真实SQL或业务数据恢复，真实新SQL迁移效果、生产启动及核心业务验收由发布维护者在隔离测试和现场验证。

## REL-002 单站点流量维护

`maintenance_hostdzire.py` 是服务器端的单次发布维护入口，与部署执行器共用同一把锁。只接受 `setup`、`freeze`、`resume`、`remove`；固定 `api.962850.xyz` 两个 server 的已核实原始配置摘要，备份原字节和权限，拒绝未知配置、标记或记录漂移。新版本维护文件依赖 `deploy_config.py`、`deploy_common.py` 和 `hostdzire.json`，须按 `deploy_trust.py` 的 `BUNDLE_FILES` 清单，以同一可信 main revision 运输完整模块/config bundle，不能单文件上传。现有已验收的 `83c250` 控制器继续有效；更新仓库文件不会自动替换它或迁移现有维护记录。只有正式发布与测试环境验收完成后才在生产使用。

`setup` 安装单站点条件但保持流量开放。生产备份及迁移锁检查完成后，`freeze` 创建 `/var/lib/sub2api-rel002-maintenance/closed`，关闭新公开请求。该专用目录为执行者所有的普通 `0755` 目录，标记为 `0644` 且不含凭据；创建和写入执行 fsync。不能把标记放回重启清空的 `/run`，也不能放在 Nginx worker 无法遍历的私密部署记录目录。

本地及内部回环验收完成后，`resume` 重新验证固定应用源码、当前成功部署记录、持久选择、运行镜像、健康及完整 308 行账本，才删除关闭标记。公开观察期间仍可再次 `freeze`；`opening` 中断也可重新冻结。完成核心业务与聚合周期验收后，`remove` 恢复同一站点的原始字节和权限。未知状态失败关闭，不把 `/health` 单独当作完成依据；该入口不恢复数据库或旧镜像。

OBS 和 RESP 发布继续使用既有 REL-002 站点、持久标记内容和维护记录目录，不重做 `setup`，不迁移或替换已安装的 `83c250` 控制器。新可信 main 完整 bundle 可执行本次冻结和恢复：

```sh
deploy/personal/deploy-hostdzire.sh --maintenance freeze \
  --tool-revision <已经获取的本-fork-origin/main-完整SHA>

deploy/personal/deploy-hostdzire.sh --maintenance resume \
  --target-revision <tps.2或tps.3-正式个人标签与OCI所绑定的完整发布SHA> \
  --deployment-record <本次成功部署实际返回的记录ID> \
  --tool-revision <同一可信main完整SHA>
```

恢复前，本机按精确运行树识别 OBS 或 RESP，验证对应的正式 tps.2 或 tps.3 标签、审定祖先、完整后端/运行资源、前端和外部 raw 输入。远端导入前验证完整工具 bundle，再在同一部署锁内检查：显式记录 ID 等于当前持久选择、成功记录的 id/action/schema/exit/健康终态、对应专属 canonical 证明、目标完整 SHA、不可变 digest/Image ID、OCI revision/version/source、Compose 项目和服务、容器与回环健康、正确完整 308 行账本。RESP 升级成功记录还必须绑定固定 C08 旧 revision/版本/来源/digest。选择和记录必须保持普通 `0600` 文件，维护标记的内容、`0644` 模式、所有权与目录合同须保持。全部通过才删除标记并 fsync 标记目录，在原维护记录追加 `accepted_source_sha` 与 `accepted_deployment_id`；历史 `source_sha=42907…` 继续表示同一站点安装身份。任一目标、记录、选择、站点、标记或健康/账本漂移保持关闭。没有默认 OBS/RESP 目标、任意 running digest 放行或失败后的静默旧版本恢复。旧服务器入口的无参数 `resume` 仍只接受固定 429；OBS 和 RESP 必须用上述显式可信入口。

六项隔离文件系统合同检查通过，旧 `/run` 实现在重启回归中因标记丢失失败。既有主机的隔离 Nginx 实例验证了 ext4 标记在易失目录清空及 Nginx 新进程启动后仍返回 503，并验证开放与再次关闭；没有重启共享生产主机。实际生产站点的完整操作与验收以 REL-002 发布记录为准。

稳定测试边界为 `Deployment(base=临时目录, runner=Docker替身)`；生产项目、容器名、持久卷和健康 URL 明确固定在受信任的 hostdzire.json，没有临时栈或更换生产目标的 CLI 开关。隔离真实镜像恢复演练由维护者在完全独立的容器/卷/端口中组织，不能据替身结果宣称生产已更新或数据库已恢复验证。

### OBS 的外部前端构建输入

OBS 专属证明还固定 `docs/legal/admin-compliance.en.md` 和 `docs/legal/admin-compliance.zh.md`：两份文档通过前端 `?raw` 导入并嵌入发布产物，不能按普通文档后继放行。路径、Git mode/type/object 与精确摘要一并绑定；改变任一文件的内容、权限或类型，升级和重部署均拒绝。历史运行树/证明常量及跨KIN回退禁令保持。

## 历史运行与本机监控记录（2026-10-13 UTC）

该历史记录的发布源码为 `8015ce827fe5de2c9e0962f3c98bb1bd2ea0ee90` / tps.3，实际发布控制和部署工具为 `5e24b02d7aeb32f440af7a3aafbb21394fa14dd8`，生产部署记录 `20261013T075506Z-bf72a8e35e62`；Full、Release、固定digest、备份、SSE/计费及完整聚合见[Responses验收](../../docs/upstream/records/2026-10-13-responses-release.md)。后继文档main HEAD不冒充当次工具revision；未来操作仍获取并显式提供当前可信origin/main。308完整账本保持，原83c250站点控制器及记录source429为历史安装身份，accepted_source/部署ID已绑定本次发布。原C08/tps.2记录保留于[OBS验收](../../docs/upstream/records/2026-10-12-obs-release.md)。

生产 `sub2api-kin-service-status-watch.timer` 每分钟、Persistent执行独立只读检查；源码位于personal的 `tools/service-status-watch.py`，安装目录 `/root/sub2api-kin/.personal-deploy/monitor/obs-tps2`，PG凭据只在服务器内存读取。安装完成后的两次自动current、固定文件hash/运行digest/依赖/完整账本及隔离测试的水位停滞/恢复/unavailable均已验收。查看本机固定事件可使用 `journalctl -u sub2api-kin-service-status-watch.service --output cat`；没有外部通知渠道或送达功能。测试专用unit与4282转发已按唯一所有权清理，原测试栈、正式证据及生产timer保留。

当前证明仍是单向并记录image_rollback_compatible=false/schema_rollback=retain；禁止旧429及跨KIN旧镜像恢复。故障先用可信main完整bundle freeze保留现场，再前向修复；数据库恢复必须先明确该快照后的写入损失、隔离恢复与切换目标，未经另行确认不能覆盖在线数据。
