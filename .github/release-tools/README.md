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

本 fork 的正式入口只接受 `vX.Y.Z-klno.N-tps.N`，使用 simple/linux/amd64。`personal-release.yml` 从默认 main 的可信工具准备版本，只有当前 personal 完整 SHA 的成功 Personal CI 和实际 personal-ready 门禁才可创建标签及显式 dispatch Release。开关为仓库变量 `PERSONAL_RELEASE_ENABLED`，默认未设置时关闭。

来源与版本规则集中在 `personal_release.py`，复用 sibling `.github/personal-sync/sync.py` 的来源和祖先合同。Personal release 的准备脚本从 main 保存；正式 Release 的发布 guard 及构建/发布控制 helpers 全部从同一次 main checkout 保存，记录完整 control SHA 并作为同次运行 artifact 传到后续 runner。应用 checkout 与应用自带的 release_matrix、supply_chain、release-images、personal_release 或 sync 脚本均不参与正式控制执行。dry-run 的控制工具取候选工作流 revision，以验证未合并的修改。正式工作流 revision 必须与应用 personal SHA 对应；`source_sha` 输入在 dry run 中也必须等于实际 checkout。个人发布跳过 main VERSION 写回。

个人运行镜像先本地导出带证明的 OCI，再完成 SHA/CI/占用核对后仅 push canonical 版本，独立把 latest 指向核验后的 digest。部分失败重跑保持 tag/SHA，已存在的正确镜像不重新构建或推送；已完成 Release 直接跳过。正式构建前、标签创建/dispatch 前、镜像推送前、latest 更新前和 Release 写入前均检查，远端查询后重新核对 CI 和 tag，拒绝错误标签、错误 SHA、其他仓库或旧 SHA 的 CI、PR CI、失败检查和来源改变。文档变化相对最近成功版本无构建/运行变化时不分配新版本。

prepare 选定的 CI run ID 在发布全过程保持固定，原 run 失效时不能换用同 SHA 的另一成功 run。最终 Release 同时比较当前 guard、publication 记录和 published image 摘要的 digest，版本镜像已删除或替换时失败关闭。

CI-004 的负向测试与只读 dry-run 边界见 [CI-004](../../docs/ci/CI-004.md)。正式发布需要先将经审查的控制增量合入 main，再按同步规则更新 personal；候选分支不会自动替换 main 的生产 guard。各外部服务间没有原子事务，最后一次检查之后的并发变更仍可能留下部分发布，恢复时必须保留 tag/SHA/digest，不能覆盖既有版本。

新标签的第一次包默认可能 private；创建后使用包设置将其公开，并验证匿名完整拉取。最终运行记录保存 tag、source/workflow SHA、上游标签/SHA、CI/Release run、平台及 OCI digest。P3 不访问生产服务器。

正式 Release 在应用 checkout 后、任何 artifact 下载与输出写入前移除已知生成入口及 symlink，再由可信流程创建上下文与发布材料，避免应用输出路径将写入引向保存的 main guard。

正式 prepare/release 中的 Python 模块安装、内联 JSON 解析和最终 stdin 检查使用 `-I`，排除应用当前目录、PYTHONPATH 与 user site 的隐式模块导入；保存的工具脚本继续使用可信绝对路径。

工作流最小权限、唯一 Git push 凭据例外、PR secrets 隔离、artifact 路径/保留期和正式发布并发规则见 [CI-005](../../docs/ci/CI-005.md)。所有上传均排除环境文件、数据库备份和完整运行日志；dry-run OCI 保留一天，脱敏发布证明保留九十天。正式发布和 dry-run 分组串行，彼此不替换 pending run。
