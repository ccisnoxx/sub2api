# Release matrix

The release workflow builds the frontend once, then runs each configured Go target on its own Linux runner. `CGO_ENABLED=0` permits cross-compilation. The target matrix is read from `.goreleaser.yaml`, including its exclusions; simple releases select Linux amd64 only.

Each build uses GoReleaser OSS in snapshot mode with the selected release version and one target. Archive naming, bundled files, Go flags and release templates remain in the existing GoReleaser configurations. Every archive is accompanied by its source commit, target, version and SHA256. The publishing job verifies the complete matrix before building images or publishing. It uses GoReleaser's `extra_files` support to publish existing archives and checksums, with builds disabled. No Pro license is needed.

Go caches are isolated by target and refreshed on each source commit, with fallback to the preceding target cache. Save uses the original restore key, even if a build hook changes `go.sum`. Matrix jobs upload uniquely named artifacts. The publishing job extracts only the regular Linux binary from each verified archive and restores its executable permission before constructing Docker contexts. QEMU remains limited to runtime-image instructions. DockerHub images are omitted when its credentials are absent; GHCR is always retained. Simple mode still publishes only the amd64 GHCR image and the simple release description.

All build jobs use the commit resolved by `prepare`, including a manual release's selected tag. Helper scripts come from the workflow revision and are passed as a run-local artifact, so older application tags do not need to contain the new scripts. The workflow serializes release runs to prevent simultaneous updates to moving image tags.

## 许可与对应源码

前端构建生成 `backend/internal/web/dist/THIRD_PARTY_LICENSES.txt`，缺少实际分发依赖的完整许可时必须失败。二进制构建前运行 `release_matrix.py materials --sha "$RELEASE_SHA" --version "$RELEASE_VERSION"`：核对当前 HEAD，要求所选源码自己的 `LICENSE`、`LICENSE-GPL-3.0.txt`、`NOTICE` 和前端声明非空，生成 `SOURCE.json`，记录完整 SHA、精确源码树/源码归档链接和各许可材料的 SHA256。

tar.gz/zip 的收集和传输校验同时检查这些材料及对应平台二进制，拒绝缺失、重复、非普通文件、来源不匹配及原有项目许可字节变化。容器上下文使用已验证归档中的材料；镜像保存于 `/usr/share/licenses/sub2api/`。Release notes 和 OCI revision 使用完整提交，source 指向本 fork，version 与所选版本一致。

旧应用提交缺少这些材料时，可信工具会明确阻止发布，不从工作流提交借用 NOTICE 或第三方声明。正式发布前必须把 LEGAL-001/002 的所需材料与构建配置纳入 `personal` 应用候选并重新验证；当前历史产物不会因修改 main 自动补齐。直接运行 GoReleaser 也须先构建前端、生成 `SOURCE.json`；源码 Docker 构建须明确传入实际 `--build-arg COMMIT=<完整SHA>` 和 `--build-arg VERSION=<版本>`。

## Validate without publication

From a branch containing this workflow:

```bash
gh workflow run release.yml --ref <branch> \
  -f tag=<branch> -f source_sha=<full-source-SHA> \
  -f dry_run=true -f simple_release=false
```

A dry run builds all selected archives and both runtime images, verifies artifact provenance and produces the final checksum file. It exports images locally as OCI archives instead of pushing them. It skips registry logins, GitHub Release publication, DockerHub description updates, Telegram notifications and VERSION synchronization. Test the simple path separately with `simple_release=true`.

Dry-run artifacts are available in the Actions run, including `release-dry-run-report`. Compare job start/end times, GoReleaser's build duration and cache restore results. Do not present an initial cold-cache run as a warmed-cache benchmark; publishing network time is not measured by dry runs.

Helper checks:

```bash
python -m pip install --require-hashes --only-binary=:all: -r .github/release-tools/requirements-release.txt
python -m unittest discover -s .github/release-tools -p 'test_*.py'
bash -n .github/release-tools/release-images.sh
```

Tool versions, platform-specific wheel hashes, runtime metadata and the monthly
manual upgrade review are specified in [CI-002](../../docs/ci/CI-002.md).
Use CPython 3.12.13 for the helper checks above. The hash allowlist covers Linux
x86_64 CI and macOS arm64/x86_64; an unsupported platform fails rather than
building an unpinned source distribution.

## personal 发布

本 fork 的正式入口只接受 `vX.Y.Z-klno.N-tps.N`，使用 simple/linux/amd64。`personal-release.yml` 从默认 main 的可信工具准备版本，仅由维护者在 main 手动触发；日常 push/PR 不运行应用 CI；只有当前 personal 完整 SHA 的成功手动 Full CI，且该次运行的全部必要 jobs（含 personal-ready/full-ci-certified）成功，才可创建标签及显式 dispatch Release。CI 完成和推送标签均不会自动发布；分支保护设置不参与发布授权。完整操作命令见 [CI-000](../../docs/ci/CI-000.md)。开关为仓库变量 `PERSONAL_RELEASE_ENABLED`，默认未设置时关闭。

来源与版本规则集中在 `personal_release.py`，复用 sibling `.github/personal-sync/sync.py` 的来源和祖先合同。Personal release 的准备脚本从 main 保存；正式 Release 的发布 guard 及构建/发布控制 helpers 全部从同一次 main checkout 保存，记录完整 control SHA 并作为同次运行 artifact 传到后续 runner。应用 checkout 与应用自带的 release_matrix、supply_chain、release-images、personal_release 或 sync 脚本均不参与正式控制执行。dry-run 的控制工具取候选工作流 revision，以验证未合并的修改。正式工作流 revision 必须与应用 personal SHA 对应；`source_sha` 输入在 dry run 中也必须等于实际 checkout。个人发布跳过 main VERSION 写回。

个人运行镜像先本地导出带证明的 OCI，再完成 SHA/CI/占用核对后仅 push canonical 版本，独立把 latest 指向核验后的 digest。部分失败重跑保持 tag/SHA，已存在的正确镜像不重新构建或推送；已完成 Release 直接跳过。正式构建前、标签创建/dispatch 前、镜像推送前、latest 更新前和 Release 写入前均检查，远端查询后重新核对 CI 和 tag，拒绝错误标签、错误 SHA、其他仓库或旧 SHA 的 CI、PR/push/Quick CI、跳过或失败的必要检查和来源改变。发布证据直接来自固定 run 的最新 jobs，不跨运行复用。文档变化相对最近成功版本无构建/运行变化时不分配新版本。

prepare 选定的 CI run ID 在发布全过程保持固定，原 run 失效时不能换用同 SHA 的另一成功 run。最终 Release 同时比较当前 guard、publication 记录和 published image 摘要的 digest，版本镜像已删除或替换时失败关闭。

## 从 main 验证并在本机恢复失败发布 job

`release-recovery.yml` 是独立的手动 main 验证入口，沿用 `PERSONAL_RELEASE_ENABLED` 开关，并与正式 Release 共用 publication 并发队列。它只读取原产物和已存在的镜像并上传校验材料；没有 Release API 或 registry 写权限，不执行登录、latest 更新或正式发布。适用范围是原 Release 的 prepare、前端和唯一 linux/amd64 binary job 都成功、仅 release job 失败、版本镜像已经存在的 simple 发布。原 source SHA、tag、CI run ID 和 OCI 主 index digest 必须保持；应用和已成功的 Full 不需要因控制修复而重建或重跑。

验证工具从本次 `github.sha` 的 main checkout 保存到 runner.temp，再以 `python -I` 执行。原 run 的 artifact 只作为数据：核对同仓库 workflow ID/path、personal SHA、手动运行、成功生产 jobs、唯一未过期 artifact、生产时间窗口及 GitHub 的运输 SHA256/size。需要 version-file、release-guard、linux/amd64 归档、原 binary SBOM/provenance 和 binary toolchain 元数据。旧 guard 仅读取 control-sha 和指定文件字节，与其 main 祖先提交比较；不会执行原 artifact 或应用目录中的 Python 模块。

二进制证明继续声明原 run、原 workflow SHA 和精确 producer attempt。验证控制 SHA、真实 verification run/attempt、原控制 SHA、原成功 job 身份及 artifact digest 写入新的 release-summary。pipeline provenance 使用真实 release-recovery.yml 身份；summary 的 `release_run` 为 null，`verification_run` 指向真实验证运行，`publication.performed=false` 明确表示尚未发布。原证明字节不改写，公开文件名带 verification run/attempt。验证不构建前端、二进制或镜像，不调度另一个 Full，不删除或覆盖原 artifact、版本镜像或 tag。

```bash
gh workflow run release-recovery.yml --repo ccisnoxx/sub2api --ref main \
  -f original_run_id=<失败的原-Release-run-ID> \
  -f source_sha=<原发布且当前-personal-完整-SHA> \
  -f tag=<原个人版本标签> \
  -f ci_run_id=<原发布绑定的成功-Full-run-ID> \
  -f expected_digest=sha256:<已推送主-index-的64位摘要>
```

OCI 回读使用固定 digest 完整复制。主 index 和 ORAS 登记的 leaf/attestation 并列时仍选择预期主 index；没有预期 digest 时只接受唯一可证的主 index。重复、无匹配、额外主根及内容漂移均失败，继续验证所有引用 blob 的 digest/size、平台、labels、SPDX、provenance 和 subject。单根 dry-run tar 仍适用。只有验证 job 和该次运行全部成功，且同次唯一 `release-recovery-supply-chain-<run>-<attempt>` 未过期，才能进入本机发布。

本机发布显式使用既有 gh 维护者认证，不自动改用 GITHUB_TOKEN 或其他环境 token，不创建 secret。使用固定 CPython/PyYAML 环境和已验证的 ORAS 1.3.4，从原 source SHA checkout 的目录调用**验证运行使用的同一已提交 main 工具**：

```bash
<固定Python> -I <已审查main工作树>/.github/release-tools/release_recovery.py publish-local \
  --verification-run-id <成功verification-run-ID> \
  --original-run-id <失败的原-Release-run-ID> \
  --sha <原发布且当前personal完整SHA> --tag <原个人版本标签> \
  --ci-run-id <原发布绑定的成功Full-run-ID> \
  --expected-digest sha256:<已推送主index的64位摘要> \
  --auth gh-maintainer --output <全新的本地私有输出目录>
```

本机入口必须满足：工具工作树 HEAD、工具字节及远端 main 都等于 verification 的控制 SHA；相关控制路径没有未提交或未跟踪增量；当前目录 HEAD 等于原 source SHA。入口重新核对 verification 的仓库、workflow ID/path、手动 main、完整 control SHA、成功 attempt/job 和 artifact 运输 digest，重新下载验证材料与原数据，沿用同一原数据、许可、binary SBOM/provenance 和工具材料校验。验证证明字节必须等于原 artifact，summary/pipeline 必须精确绑定真实 verification 和原 producer。原 source/Full/tag/digest、生产者或运输身份漂移时均停止。

身份确认后，先从维护者可见的分页 List releases 按 `tag_name` 精确查找恢复对象；同 tag 多个对象直接拒绝，唯一对象按 ID 回读并核对 source 和恢复绑定。tag 查询只返回已发布 Release，404 不能证明没有 partial draft。未知 draft、同名不同 digest/size 或重复资产均在 registry 登录、latest 更新和任何 Release 写入前失败。完整材料和所有绑定再次核对后才把 latest 指向原固定 digest，并回读确认。既有 gh 维护者 token 只通过 stdin 交给 ORAS，使用临时私有 registry config，退出时删除，不覆盖操作者的全局认证配置。

GitHub 发布使用受控 draft，部分上传续传保留原 ID，并在每次上传和完成发布前按 ID 重新核对绑定；不会因 tag 端点 404 而 POST 另一个 draft。同名资产只有 digest/size 全相同才复用，冲突显式拒绝，不覆盖任何既有资产。说明和额外 publisher JSON 显式记录 `local-gh-maintainer-token`、真实维护者、control SHA、verification 运输及本机 publication ID；不伪造 Actions publisher run。新 publisher 文件使用 verification run/attempt 加独立本机 ID，原验证材料保持原字节。

本机进程不占用 Actions concurrency 队列。每次写入前还检查同 source SHA 的 Release，以及不按 head SHA 筛选的全部 Personal release 都已结束；存在 queued/in-progress/waiting 等运行时停止，不自动取消。旧 main 调度的 Personal release 仍可能 checkout 当前 main 和当前 personal；API 不提供 dispatch 输入，不能用旧 head SHA 判定其与本次发布无关，也不能猜测同 source 的 Release 是 dry-run 而忽略它。

中断留下的受控 draft 保持关闭，下一次发布继续保留旧资产并追加本次 publisher 记录。未知 draft 缺少精确恢复绑定时停止；已完成 Release 在核对原运行和发布授权后幂等退出，不重建。仅重跑原失败 job 仍下载旧 prepare 工具，不能加载 main 修复；不得删除或覆盖旧 release-tooling。原运行被重跑、producer/artifact 重名、运输过期、main/personal/CI/tag/digest 漂移时，需要重新明确绑定。各外部服务之间仍没有原子事务，最后检查之后仍可能出现新的外部并发写入并留下部分状态；写前复查、不可覆盖资产和受控 draft 限定了恢复范围。

GitHub Release API 对涉及 workflow 文件的目标提交可能要求 Workflows:write，而 GITHUB_TOKEN 无法授予该权限；因此云端入口仅验证，正式发布使用显式的既有维护者认证。`target_commitish` 保留原 source SHA；已存在标签时不用于创建标签，不以 main 冒充应用来源。403/404 等服务失败明确保留，不静默换 token 或回退来源。API 合同见 [GitHub Release REST API](https://docs.github.com/en/rest/releases/releases?apiVersion=2026-03-10#create-a-release)。

CI-004 的负向测试与只读 dry-run 边界见 [CI-004](../../docs/ci/CI-004.md)。正式发布需要先将经审查的控制增量合入 main，再按同步规则更新 personal；候选分支不会自动替换 main 的生产 guard。各外部服务间没有原子事务，最后一次检查之后的并发变更仍可能留下部分发布，恢复时必须保留 tag/SHA/digest，不能覆盖既有版本。

新标签的第一次包默认可能 private；创建后使用包设置将其公开，并验证匿名完整拉取。最终运行记录保存 tag、source/workflow SHA、上游标签/SHA、CI/Release run、平台及 OCI digest。P3 不访问生产服务器。

正式 Release 在应用 checkout 后、任何 artifact 下载与输出写入前移除已知生成入口及 symlink，再由可信流程创建上下文与发布材料，避免应用输出路径将写入引向保存的 main guard。

正式 prepare/release 中的 Python 模块安装、内联 JSON 解析和最终 stdin 检查使用 `-I`，排除应用当前目录、PYTHONPATH 与 user site 的隐式模块导入；保存的工具脚本继续使用可信绝对路径。

工作流最小权限、唯一 Git push 凭据例外、PR secrets 隔离、artifact 路径/保留期和正式发布并发规则见 [CI-005](../../docs/ci/CI-005.md)。所有上传均排除环境文件、数据库备份和完整运行日志；dry-run OCI 保留一天，脱敏发布证明保留九十天。正式发布和 dry-run 分组串行，彼此不替换 pending run。
