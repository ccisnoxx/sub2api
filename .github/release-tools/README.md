# Release matrix

The release workflow builds the frontend once, then runs each configured Go target on its own Linux runner. `CGO_ENABLED=0` permits cross-compilation. The target matrix is read from `.goreleaser.yaml`, including its exclusions; simple releases select Linux amd64 only.

Each build uses GoReleaser OSS in snapshot mode with the selected release version and one target. Archive naming, bundled files, Go flags and release templates remain in the existing GoReleaser configurations. Every archive is accompanied by its source commit, target, version and SHA256. The publishing job verifies the complete matrix before building images or publishing. It uses GoReleaser's `extra_files` support to publish existing archives and checksums, with builds disabled. No Pro license is needed.

Go caches are isolated by target and refreshed on each source commit, with fallback to the preceding target cache. Save uses the original restore key, even if a build hook changes `go.sum`. Matrix jobs upload uniquely named artifacts. The publishing job extracts only the regular Linux binary from each verified archive and restores its executable permission before constructing Docker contexts. QEMU remains limited to runtime-image instructions. DockerHub images are omitted when its credentials are absent; GHCR is always retained. Simple mode still publishes only the amd64 GHCR image and the simple release description.

All build jobs use the commit resolved by `prepare`, including a manual release's selected tag. Helper scripts come from the workflow revision and are passed as a run-local artifact, so older application tags do not need to contain the new scripts. The workflow serializes release runs to prevent simultaneous updates to moving image tags.

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
python -m pip install -r .github/release-tools/requirements-release.txt
python -m unittest discover -s .github/release-tools -p 'test_*.py'
bash -n .github/release-tools/release-images.sh
```

## personal 发布

本 fork 的正式入口只接受 `vX.Y.Z-klno.N-tps.N`，使用 simple/linux/amd64。`personal-release.yml` 从默认 main 的可信工具准备版本，只有当前 personal 完整 SHA 的成功 Personal CI、可信检查证据和实际 personal-ready 门禁才可创建标签及显式 dispatch Release。开关为仓库变量 `PERSONAL_RELEASE_ENABLED`，默认未设置时关闭。

来源与版本规则集中在 `personal_release.py`，复用 sibling `.github/personal-sync/sync.py` 的来源和祖先合同；两目录均作为本次运行 artifact 传到发布 runner。正式 Release 的工具 revision 必须与应用 personal SHA 对应；`source_sha` 输入在 dry run 中也必须等于实际 checkout。个人发布跳过 main VERSION 写回。

个人运行镜像先本地 buildx --load，再完成最后一次 SHA/CI/占用核对后仅 push canonical 版本，独立把 latest 指向核验后的 digest。部分失败重跑保持 tag/SHA，已存在的正确镜像不重新构建或推送；已完成 Release 直接跳过。正式构建前和写入前均检查，拒绝错误标签、错误 SHA、PR CI、失败检查和来源改变。文档变化相对最近成功版本无构建/运行变化时不分配新版本。

新标签的第一次包默认可能 private；创建后使用包设置将其公开，并验证匿名完整拉取。最终运行记录保存 tag、source/workflow SHA、上游标签/SHA、CI/Release run、平台及 OCI digest。P3 不访问生产服务器。

## 按变更选择候选检查

候选门禁只有 `personal-ci.yml` 一个入口。`backend-ci.yml` 仅接受 `workflow_call`；`security-scan.yml` 只接受调用和周度维护扫描，特性分支 push 不再额外启动另一套完整检查。草稿 PR 的自动门禁跳过；稳定候选可以通过 ready-for-review 或显式 dispatch 验收。原生 PR 和 dispatch 共用同一分支并发组。

`ci_policy.py` 对完整 base→candidate 差异和上游来源选择检查，删除及改名两端都参与选择。纯前端改动运行前端 lint、关键测试、构建和相关 TPS 合同，不运行后端测试及额外安全扫描；后端变化选择后端检查；依赖变化选择对应扫描；部署及发布控制变化选择相关合同检查；上游来源变化选择完整检查。无法归类的构建输入保守选择前后端检查，文档目录中的 Markdown 与后端运行资源中的 Markdown 不混同。

前端构建包含 i18n 和类型检查，候选只构建一次。后端单元、集成和 recording/race 使用独立 jobs，没有串行依赖；各 job 复用 Go 缓存。并行可能缩短关键路径，总 runner 用量和冷编译成本仍需实际运行测量。

`personal-ready` 继续是 required check。它根据经过校验的选择计划，要求所有已选择检查成功；只有未选择或已经可信复用的检查可以 skipped。失败或取消不能被当作跳过。

## 合并后的检查证据复用

每次成功候选以 `personal-ci-evidence` artifact 保存 `ci-evidence.json`，保留 30 天。证据绑定候选、base、源码树、检查定义及配置、依赖锁文件、声明的工具链选择器、检查策略和实际运行身份。选择器描述配置输入，不代表单独证明了 Node/pnpm 等工具在 runner 上解析出的精确补丁版本。

合并后的 personal 提交仍执行来源、当前分支、父提交及保护规则核对，并产生自己的成功 `personal-ready`。只有普通双父 merge、候选树与最终树相同、base 及输入匹配、同仓库候选的真实成功检查和 artifact 均有效时，才能复用功能测试。PR 结果本身不能直接用于发布。

自动 PR 的临时 merge workflow revision 缺少顶层 API provenance；这类复用额外要求 base、候选及最终树中的 CI 控制定义保持一致。改动控制定义的 PR 不走该复用路径；显式 dispatch 的执行 revision 可由事件和 head SHA 建立。扫描证据有 24 小时时效，扫描器固定为 `golang.org/x/vuln/cmd/govulncheck@v1.8.0`；漏洞数据库仍随时间变化，周度扫描继续运行。

找不到有效候选证据时，明确记录原因并执行按变更选择的检查。权限、网络和解析失败直接失败。发布再次校验证据及复用来源、最终 SHA 的必要检查和 check-suite；含新策略的提交缺少证据不能退回旧门禁。旧版源码的发布恢复继续采用原来的最终 SHA 完整门禁。

## 应用维护流程

控制定义需审查后同步到 main 和 personal，保持两分支的应用树分工。维护时只把本次 `.github` 控制文件的补丁带入 personal，不合并 main 的旧应用源码。先在包含新定义的 personal 候选中验收；更新 CI 控制定义的第一轮可能不满足 PR 证据复用条件，后续普通应用候选才使用该路径。首次远端验收应检查纯前端选择、上游同步选择、同树 merge 复用、最终轻量门禁及实际保护状态。

本地定向合同与 actionlint 能验证选择和拒绝条件；没有实际 Actions 运行时，不声称平台执行、artifact 权限或等待时间已经验收。此改动不需要发布应用版本或访问生产服务器。
