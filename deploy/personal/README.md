# hostdzire 个人镜像部署

本目录是本机部署控制工具。入口从当前仓库读取个人发布标签与来源记录，通过既有 `ssh hostdzire`、`scp` 将标准库 Python 执行器上传到私密临时目录，在服务器执行一次完整操作。工具不依赖本机 Docker daemon，不修改 SSH 配置或 host key 检查。

## 使用前提与固定现场

本机需要 Python 3.9 或以上、Git、OpenSSH，`origin` 指向 `ccisnoxx/sub2api`，并持有个人发布标签、上游基线标签以及当前运行 revision 的完整 Git 对象。浅克隆需先获取这些对象；工具遇到缺失对象会停止。服务器需要 Python 3.9 或以上、Linux x86_64、Docker Compose 的 JSON 配置与 `--wait`/`--wait-timeout` 能力、curl。已核实的服务器 Python 为 3.11、Compose 为 v5.1.3。

工具固定使用 `/root/sub2api-kin/docker-compose.yml`、项目 `sub2api-kin`，服务集合必须为 `sub2api`、`postgres`、`redis`。应用容器为 `sub2api-kin`，端口为 `127.0.0.1:10088 → 8080`，命名卷 `sub2api-kin_sub2api_data` 挂载到 `/app/data`。现场变化会停止操作，需要重新审查工具的现场合同。

正式部署会短暂重建一个应用容器。选择合适窗口，保留现有数据库、Redis、网络、卷和旧镜像；正在进行的 HTTP 流与 WebSocket 可能断开。健康验证完成后，还需验收登录、使用记录 TPS 与经授权的网关请求。

## 部署与回滚命令

在仓库根目录指定个人版本；可带 `v` 或完整 fork 镜像版本地址。版本先拉取并解析唯一 RepoDigest，再拉取该 digest；启动、验收和记录均使用相同的不可变引用。

```sh
deploy/personal/deploy-hostdzire.sh 0.2.14-klno.3-tps.1
```

固定 digest 必须同时给出对应个人版本；revision 从本机个人 Git 标签读取，再核验实际镜像的 OCI 标签。首发的推荐命令为：

```sh
deploy/personal/deploy-hostdzire.sh \
  ghcr.io/ccisnoxx/sub2api@sha256:f4a979fdeef6c79b982d16d77bc3a6c7b528164bd6ce5b1deb34a8f4981a3d76 \
  --version 0.2.14-klno.3-tps.1
```

成功输出包含服务器记录 `id`、目标与实际运行 digest、Image ID、OCI version/revision/source、健康状态。仅当前成功的**部署记录**可显式回滚：

```sh
deploy/personal/deploy-hostdzire.sh --rollback 20261008T180000Z-123456789abc
```

上面的 ID 是格式示例，执行时替换为本次部署实际返回的 ID。回滚提交新的记录与选择 ID，原部署 ID 随即不能重放。新部署输入只允许 `ghcr.io/ccisnoxx/sub2api` 的个人版本或 digest，拒绝 `latest`、其他仓库及 shell 语法。旧 KlN 仓库只允许首次已验证记录的固定 digest/revision/version/source，作为恢复来源，不接受新部署输入。镜像必须为 `linux/amd64`；缺失、错误或不一致的 OCI 标签与 RepoDigest 会停止操作。

## 状态与兼容性合同

`Deployment` 是唯一远端生命周期 owner，使用非阻塞 `flock` 持有 `/root/sub2api-kin/.personal-deploy/deployment.lock`，锁覆盖预检、备份、拉取、启动、健康、持久提交及失败恢复。并发部署在执行 Docker 命令前退出。其他生产配置维护也应尊重这把锁；工具的 hash 检查会拒绝已观察到的外部漂移。

本机比较实际旧、新完整 revision 的整个 `backend/`、`Dockerfile`、`Dockerfile.goreleaser`、`.dockerignore` 与 `deploy/`，仅排除本部署工具目录及 `deploy/personal-source.json`。同一上游基线只允许零运行差异、下述精确工具链补丁及固定 S0–S4 累计应用；跨基线只允许下述固定 `.3 → .5` 正向升级。其他后端、迁移、镜像运行资源、部署配置差异或未知对象都会阻止操作，没有 force 或 ignore 兼容开关。首发 `de08df02… → 896de21b…` 在这些路径仅新增来源记录，满足零差异合同。

本轮经用户三次明确授权，升级 Go 1.27.0 → 1.27.2，x/net v0.58.0 → v0.60.0，并按模块最小版本选择同步 crypto0.57、sys0.48、term0.46、text0.42、tools0.50、mod0.41、sync0.23。精确 lint 兼容规则仅匹配五个固定文件中的既有 HTTP/2 弃用接口，其他检查继续启用。例外绑定已审定的 `backend/.golangci.yml`、`backend/go.mod`、`backend/go.sum`、`Dockerfile`、`deploy/Dockerfile` 前后完整文件 SHA-256；应用候选为 `ce682c4901033d09c7e12614b7b73e11bdd17ef4`，旧输入为 tps.1 revision `896de21b4be7f4ec4b4236f4df663b47371665b0`。五个路径必须全部变化、方向一致，双方均为普通 `100644` 文件。任意其他版本、校验行、字节、权限、业务源码、迁移或运行配置变化均拒绝；未来安全补丁需要另行审定，不能按文件路径泛化放行。CI/Release 精确核对所选源码 go.mod 的 Go 声明并继续全部门禁，personal 的声明固定为1.27.2；main 不维护应用树。

兼容证据记录补丁ID、方向和双方文件 SHA-256，远端按同一已审定清单验证，并绑定实际旧/新 revision。证据总 hash 校验本机传输完整性，不是远端重新执行 Git 审计或镜像构建证明。原有零差异证据和成功记录继续有效。精确补丁允许反向镜像故障恢复；普通 deploy 旧版本会在备份/拉取/启动前被拒绝，须使用绑定当前成功记录的 --rollback，或由失败部署自动恢复。恢复仍须满足成功记录、配置、迁移和依赖条件；旧Go及x/net有已知漏洞，因此不能长期停留旧安全版本。

本轮另外审定的跨基线升级只接受 `v0.2.14-klno.3 / de08df02ae1d81668a22f798b398aa0438ac1276` → `v0.2.14-klno.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d`。旧运行树固定取自 tps.1 `896de21b4be7f4ec4b4236f4df663b47371665b0`，新树取自融合候选 `c00e8982258736a188a09023e901d8d39671814f`。对全部运行条目的路径、Git mode、type 与 object ID 排序并计算 SHA-256，旧树为 `ecf6eff8c2902b29a1a689232dc8a1028e9d10e5615bcb6e293d35d8d278df35`，新树为 `928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d`；同时绑定精确的 151 个运行差异路径。最终 PR 合并 revision 可以不同，但来源与整个运行树必须相同。修改任意已有、未变化或新增运行路径的内容、类型、权限、迁移或来源都需重新审定。远端公共 validator 检查证明类型、hash、所有固定字段、来源、完整差异列表、实际旧新 revision 及镜像版本；不能借零差异或五文件安全补丁绕过跨基线合同。

此正向证明明确记录 `image_rollback_compatible=false`。`.5` 的 `account.extra.openai_gwpool_usage_rounds` 新增 `rounds.active_usage` 与归档的 `active_duration_ms/incomplete`，contacts 新增 `previous`；`.3` 的类型解码与 whole-key `UpdateExtra` 会丢失这些字段。`.5` reporter 在 HTTP 监听与 health 之前启动，cooldown 首次 arm 也可能写入持久化数据。因此迁移清单未变化、启动等待失败或 `/health` 尚未成功都不能证明旧镜像恢复无损。该升级拒绝 `.5 → .3` 普通部署、显式回滚，以及新应用启动后的自动旧镜像恢复。需要另行评估数据兼容的向前修复或经授权的维护恢复；数据库恢复与可能丢失写入不在部署工具的授权中。

S0–S4 的 `klno.5-tps.1-to-s0-s4` 证明固定旧应用 `9397eb8afb621aef483f2ec0bf4b2dd6247c7b92 / 0.2.14-klno.5-tps.1`，保留双方 `.5 / c7aacf5d3ae383d0d5c75f471f66e61690a5701d` 来源。新候选锚点为 `74508aa0e1c2d61e9e269fbe73e3eb8ce515a9da`，旧完整运行树为 `928336853218695778c2aed538952de0af7c3e1413cfd25cce0daf73ec52c81d`，新树为 `5a5391fc71958b62aeae00151480d0dcfeaab3bc42055bfa17fa60d16c43ae24`，精确 128 个差异路径以 `deploy_hostdzire.py` 的 `CUMULATIVE_CHANGED_PATHS` 为准。正式个人标签的 merge revision 可以不同，但必须包含审定候选且整个运行树完全相同。本机核对真实 Git 对象，远端核对 canonical 清单、证明 hash、实际旧新 revision 与镜像版本。不能删掉专属证明、伪装成零差异或安全补丁放行这次累计运行差异。

累计证明只允许新增 `251_add_usage_log_timing.sql`、`252_add_ops_routing_diagnostics.sql`、`253_service_status_observation.sql`、`254_service_status.sql`，分别固定 Go runner 使用的 `SHA256(strings.TrimSpace(SQL))`。251 的追加计时/终态字段包含旧写入可使用的默认值和可空列，252/253 追加可空 JSONB，254 创建独立服务状态表并安装分组可见性 trigger；兼容结论绑定这些完整 SQL 和固定旧应用的真实扩展 schema 证据，不推广为“所有追加迁移均安全”。回退只恢复应用镜像，保持扩展 schema 与 ledger，`schema_rollback=retain`。

远端取得锁后再次读取实际旧 digest、revision、version、source、Image ID，与本机兼容证据绑定的旧状态比较。PostgreSQL `SELECT 1`、Redis `PING` 与容器 healthcheck 均须通过；数据库/Redis 容器 ID 在整个应用更新中保持不变。通过 `schema_migrations` 的 `filename/checksum` 数据库排序清单记录行数与 SHA-256，保留数据库实际顺序。通常更新后以及自动恢复前必须完全一致，未知迁移差异阻止恢复。已有成功记录的迁移状态后来发生变化时，同样禁止后续部署或显式回滚。

累计升级要求启动前账本与当前成功记录相同，所有原行逐字不变，健康终态只能新增全部四个固定 filename/checksum；新成功记录保存扩展后的 fingerprint。启动失败或取消时，自动恢复只接受无新增或按 runner 事务顺序提交的固定前缀，并在替换旧应用后再次核对；未知行、旧行变动、错误 checksum、缺口、重复记录、配置/依赖漂移或未绑定的实际运行镜像均明确阻止恢复。固定新增迁移可能在 replacement 停止新应用前完成下一笔事务，恢复后仍只能观察到相同原行和不倒退的固定前缀。

自动恢复保留原 selection，不改写旧成功记录。失败操作若已经提交任何新增迁移，旧镜像虽恢复健康，下一次自动部署仍会因原成功记录与扩展账本不一致返回 `E_MIGRATION`，需要先人工核对失败记录和账本。成功累计部署可使用当前部署 ID 显式切回固定旧 `.5`，通过健康后提交新的回滚选择与完整扩展账本；之后只有这份成功回滚记录所绑定的同一扩展账本可再次升级审定累计应用。普通 `deploy` 不允许反向切回。

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

每次操作生成 `.personal-deploy/records/<ID>/`。状态目录与记录目录权限为 `700`，记录、镜像选择、配置备份、数据库与应用数据备份、诊断文件权限为 `600`。记录保存工具 Git revision 与实际脚本 SHA-256、旧/目标镜像元数据、兼容证据、时间、阶段、配置/.env hash、迁移指纹、备份信息和退出状态。服务器 `.env`、应用配置、数据库、应用日志留在服务器，不输出凭据或上传 Git。

执行顺序是预检、备份、目标拉取与 OCI/平台校验、最小 Compose 适配、仅应用启动、完整健康核对、持久提交成功选择。备份完成在新进程启动之前。更新使用一个应用 Compose replacement 顺序替换；不并行启动共享 Redis 的新旧应用，避免 `CleanupStaleProcessSlots` 清理另一进程的 slot。备份包括原 Compose、`.env`、已有镜像选择、`pg_dump --format=custom` 和 `/app/data` 归档。数据库 dump 以文件流送入 `pg_restore --list`，应用归档先枚举 tar 成员，再分块读取到 gzip EOF，完成 CRC 与长度校验；两份数据备份记录大小及 SHA-256。数据库与应用归档是在运行期间分别取得，恢复时需要人工核对业务一致性；`pg_restore --list` 证明结构可读取，实际完整恢复仍需隔离实例演练。

| 情况 | 状态与恢复 |
|---|---|
| 预检、兼容证据、备份或拉取失败 | 非零退出；旧运行容器、持久镜像选择及原 Compose 保留 |
| 拉取成功后配置校验失败，尚未启动 | hash 核对无漂移时恢复本次 Compose 适配；保留失败记录 |
| 零差异或精确安全补丁应用启动或健康失败，兼容证据有效且数据库/迁移/配置未漂移 | 用旧固定 digest 仅重建应用并验证健康，恢复本次 Compose 适配；原选择保留。`rollback_status=success` 仍是部署失败，退出码 `1` |
| S0–S4 累计升级启动、取消、健康或 selection 写入前失败 | 仅在固定追加兼容证明、原行不变和固定事务前缀、旧新运行镜像、配置与依赖合同均成立时恢复旧 `.5`，保留新 schema/ledger 和原 selection；检查失败记录 `rollback_status=blocked`，实际恢复启动/健康失败记录 `failed` |
| `.3 → .5` 新应用启动或健康失败，持久选择尚未提交 | 保留 `status=failed`、原始错误、诊断与 `observed_running`，记录 `rollback_status=blocked`、`rollback_error=E_COMPATIBILITY`；不启动旧镜像、不替换原 selection，保留当前 Compose 供人工核对 |
| 数据库/Redis 容器、迁移或配置发生漂移 | 阻止自动镜像切回，保存失败与当前可观察状态，需人工评估 |
| 同基线显式回滚 | 绑定当前成功部署 ID、旧/目标完整元数据、配置 hash、双方 Git 兼容证据与迁移指纹；通过健康后提交新的回滚记录与旧 digest 选择，保留已完成的 Compose 参数化 |
| `.3 → .5` 成功记录的显式回滚 | 本机证明与远端成功记录绑定都拒绝旧镜像恢复，返回 `E_COMPATIBILITY`；运行与原 selection 保留 |
| 持久选择已替换但后续落盘/成功记录失败 | 不猜测持久提交结果，也不覆盖选择；保留失败记录与 `selection_committed=true`。后续部署/回滚因记录不一致停止，需人工核对 |
| SSH 中断或无有效结果 | 本机明确失败，不自动重新执行写入；先核对服务器记录、运行镜像、配置与选择 |

远端可处理 SIGINT/SIGTERM，按兼容条件保存失败并尝试恢复；SIGKILL、主机重启或磁盘无法写入无法保证收尾。中断阶段、成功记录与选择不一致，或个人镜像运行却缺少已提交的镜像选择时，工具会停止自动接管。修复前应核对私密记录中的 `status`、`observed_running`、迁移指纹与配置 hash；不要仅凭容器运行或 `/health` 认定已完成部署。

工具只恢复应用镜像，不自动恢复数据库、Redis 或应用数据。迁移不兼容、备份恢复及可能丢失备份后写入的情形，需要另行制定经授权的维护与恢复操作。观察期内保留记录、备份、旧 digest 与本地旧镜像。

## 定向验证与 CI

以下命令不访问 hostdzire、不使用真实 Docker、不安装依赖；CI checkout 应使用 `fetch-depth: 0` 并保留个人及上游标签，发布绑定用例需要已发布固定对象。

```sh
python3 -m unittest discover -s deploy/personal -p 'test_*.py' -v
sh -n deploy/personal/deploy-hostdzire.sh
python3 -m py_compile deploy/personal/deploy_hostdzire.py deploy/personal/test_deploy_hostdzire.py
git diff --check
```

替身覆盖成功仅更新应用、版本解析后固定 digest、拉取/备份失败无运行变更、健康超时及自动恢复、显式回滚与过期拒绝、真实 `flock` 竞争、运行 revision 漂移、OCI 标签/source/platform/RepoDigest 拒绝、迁移禁止部署与回滚、配置漂移保留、选择落盘失败及部分提交。真实固定 Git 对象与临时 Git 保护完整运行路径 diff、来源/tag 绑定、`.3 → .5` 正向及反向拒绝、运行树内容/模式/类型/迁移/来源漂移；远端替身覆盖正向证明字段与 hash 篡改、成功 app-only 生命周期、启动前备份、启动/健康失败阻止旧镜像与保留原 selection、单向记录显式回滚拒绝及部分提交。累计合同另保护固定四迁移、原账本字节、部分事务与取消恢复、未知差异阻止恢复、恢复后重试拒绝、显式回滚后再次升级、selection 部分提交、证明伪装及最终 merge 祖先/完整运行树检查。真实本机子进程验证 dump 文件流和私密错误边界；OpenSSH 替身验证固定别名、scp 和通过 stdin 传输 JSON。正式前还需独立只读审查以及现场健康、界面与网关验收。

稳定测试边界为 `Deployment(base=临时目录, runner=Docker替身)`；生产项目、容器名、持久卷和健康 URL 固定在本模块，没有临时栈或更换生产目标的 CLI 开关。隔离真实镜像恢复演练由维护者在完全独立的容器/卷/端口中组织，不能据替身结果宣称生产已更新或数据库已恢复验证。
