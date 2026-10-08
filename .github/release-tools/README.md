# Release matrix

The release workflow builds the frontend once, then runs each configured Go target on its own Linux runner. `CGO_ENABLED=0` permits cross-compilation. The target matrix is read from `.goreleaser.yaml`, including its exclusions; simple releases select Linux amd64 only.

Each build uses GoReleaser OSS in snapshot mode with the selected release version and one target. Archive naming, bundled files, Go flags and release templates remain in the existing GoReleaser configurations. Every archive is accompanied by its source commit, target, version and SHA256. The publishing job verifies the complete matrix before building images or publishing. It uses GoReleaser's `extra_files` support to publish existing archives and checksums, with builds disabled. No Pro license is needed.

Go caches are isolated by target and refreshed on each source commit, with fallback to the preceding target cache. Save uses the original restore key, even if a build hook changes `go.sum`. Matrix jobs upload uniquely named artifacts. The publishing job extracts only the regular Linux binary from each verified archive and restores its executable permission before constructing Docker contexts. QEMU remains limited to runtime-image instructions. DockerHub images are omitted when its credentials are absent; GHCR is always retained. Simple mode still publishes only the amd64 GHCR image and the simple release description.

All build jobs use the commit resolved by `prepare`, including a manual release's selected tag. Helper scripts come from the workflow revision and are passed as a run-local artifact, so older application tags do not need to contain the new scripts. The workflow serializes release runs to prevent simultaneous updates to moving image tags.

## Validate without publication

From a branch containing this workflow:

```bash
gh workflow run release.yml --ref <branch> \
  -f tag=<branch> -f dry_run=true -f simple_release=false
```

A dry run builds all selected archives and both runtime images, verifies artifact provenance and produces the final checksum file. It exports images locally as OCI archives instead of pushing them. It skips registry logins, GitHub Release publication, DockerHub description updates, Telegram notifications and VERSION synchronization. Test the simple path separately with `simple_release=true`.

Dry-run artifacts are available in the Actions run, including `release-dry-run-report`. Compare job start/end times, GoReleaser's build duration and cache restore results. Do not present an initial cold-cache run as a warmed-cache benchmark; publishing network time is not measured by dry runs.

Helper checks:

```bash
python -m pip install -r .github/release-tools/requirements-release.txt
python -m unittest discover -s .github/release-tools -p 'test_release_matrix.py'
bash -n .github/release-tools/release-images.sh
```

## personal 发布

本 fork 的正式入口只接受 `vX.Y.Z-klno.N-tps.N`，使用 simple/linux/amd64。`personal-release.yml` 从默认 main 的可信工具准备版本，只有当前 personal 完整 SHA 的成功 Personal CI 和实际 personal-ready 门禁才可创建标签及显式 dispatch Release。开关为仓库变量 `PERSONAL_RELEASE_ENABLED`，默认未设置时关闭。

来源与版本规则集中在 `personal_release.py`，复用 sibling `.github/personal-sync/sync.py` 的来源和祖先合同；两目录均作为本次运行 artifact 传到发布 runner。正式 Release 的工具 revision 必须与应用 personal SHA 对应；`source_sha` 输入在 dry run 中也必须等于实际 checkout。个人发布跳过 main VERSION 写回。

个人运行镜像先本地 buildx --load，再完成最后一次 SHA/CI/占用核对后仅 push canonical 版本，独立把 latest 指向核验后的 digest。部分失败重跑保持 tag/SHA，已存在的正确镜像不重新构建或推送；已完成 Release 直接跳过。正式构建前和写入前均检查，拒绝错误标签、错误 SHA、PR CI、失败检查和来源改变。文档变化相对最近成功版本无构建/运行变化时不分配新版本。

新标签的第一次包默认可能 private；创建后使用包设置将其公开，并验证匿名完整拉取。最终运行记录保存 tag、source/workflow SHA、上游标签/SHA、CI/Release run、平台及 OCI digest。P3 不访问生产服务器。
