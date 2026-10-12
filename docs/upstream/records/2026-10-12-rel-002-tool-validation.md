# REL-002 发布恢复工具：本线程实际验证证据

本文按主代理要求整理 `/root/release_recovery_implementation` 当前线程已经执行的命令及可见工具返回。不重跑检查，不修改候选实现，不将交付自报补写为原始日志。

## 证据来源与限制

本线程使用 `functions.exec` 调用 `tools.exec_command`，部分调用完整输出了返回对象，部分只输出了返回对象的 `.output`。后者的可见历史没有 `exit_code`，本文不补填。`functions.exec` 的“Script completed”表示 JavaScript 编排结束，不能充当嵌套 shell 命令的退出码。

工具提供的是 `exec_command.output` 命令输出文本。本线程没有分离保存 stdout/stderr；下方引用为工具输出原文短节选，不是独立 stdout 日志文件。尤其 unittest 的输出不能据此重新标注为只来自 stdout。

| 类别 | 当前可用事实 |
|---|---|
| 本线程验证原始日志已落盘 | 没有。下述检查没有通过 tee、重定向或日志文件参数保存原始输出。 |
| 仅 runtime 工具输出 | 聚合 unittest、recovery 定向 unittest、红→绿反例、actionlint/bash 检查调用及本文列出的短节选。 |
| 交付自报 | 先前交付中的“95 项”和“最终 recovery 22 项”为成功证据组合计数；不存在本线程单次 `Ran 95 tests` 或 `Ran 22 tests` 的原始输出。 |
| 外部提供的落盘输入 | 原 OCI 布局、既有 artifact 目录及诊断 JSON 是主代理提供的只读输入，不是本线程创建的检查日志。 |
| 主代理通知的最终独立复核 | 主代理通知“14 个纯内存反例无新阻断”。本线程未执行或获取该复核的原始命令、输出、退出码或日志文件，因此本文不把它计入本线程验证。 |

没有检查的 UTC 开始/结束时间元数据可在本线程对应可见输出中确认，本文不补造时间戳。文档日期取任务指定的记录日期。

## 候选与输入绑定

工作区：`/Users/sc/.codex/worktrees/rel-002-delivery/sub2api-kin`。

本线程最后一次 `git rev-parse HEAD` 的实际输出：

```text
468139e8c3382e79d1298c326f5a191ca5fc0c3d
```

这是历史检查执行时的工作区基准 HEAD，不是候选 diff 的整体内容摘要，也不是归档时的当前 HEAD。本子代理没有执行 commit/push。

证据整理期间主代理通知：受复核的控制/维护/CI-000 文件已提交为短 SHA `83c250d`，正在普通 push main，`.github` 工具候选字节没有再改。这是主代理消息，不是本线程新执行的 Git/API 核验；没有在本线程工具输出中获取该新提交的完整 SHA 或 push 完成状态，因此不补填，也不将它作为上述历史检查的执行 SHA。验证记录将稍后随 REL-003 归档，verification/publish 期间不为本文更新 main。

本线程最后一次执行的候选哈希命令为：

```bash
shasum -a 256 .github/release-tools/release_recovery.py .github/release-tools/test_release_recovery.py
```

实际工具输出为：

```text
d98318fa4358d22d8ee77418f32f47dece635dd51d6e5780083f7612f0afb8dc  .github/release-tools/release_recovery.py
24ddafc2e4c09fc3c44dc2f4cd4606c77d92d6e8c3d036885b2172557c31ffff  .github/release-tools/test_release_recovery.py
```

该调用只输出 `.output`，可见历史没有 shell 退出码。上述两个摘要是实际输出，不能扩大成完整工作树或全部依赖的哈希证明。该哈希调用之后本线程没有修改 `.github` 候选；本次整理只新增本文。

验证使用的固定 Python：

```text
/Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python
```

任务提供的环境为 CPython 3.12.13、PyYAML 6.0.3。本线程检查命令实际使用上述解释器路径，但没有在本文对应运行中另行输出它的版本；不得把环境说明改写为这次运行的版本日志。

原发布绑定输入在真实 OCI/原 artifact 只读检查中使用：

| 输入 | 值 |
|---|---|
| 原 source/workflow SHA | `42907bcab278f93db48d7e1b39b28e28f664569e` |
| 原 Release run | `38278730044`，attempt `1` |
| 原 Full CI run | `38274106520` |
| 原 tag | `v0.2.15-klno.2-tps.1` |
| 固定 OCI 主 digest | `sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3` |
| 固定 linux/amd64 leaf | `sha256:db1819ef0c08eeaa0c0400529aab4b22dad22c037f47b04dc9cc631f5145b8e6` |

## 聚合 94 项与“95 项”覆盖计数

实际执行的 unittest 子命令为：

```bash
PYTHONPATH=.github/release-tools /Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python -m unittest test_supply_chain test_personal_release test_release_matrix test_release_control test_release_recovery test_workflow_security test_action_pins
```

该次 shell 命令在上述子命令之前还运行了一个写入 release-tools README 的 Python 脚本。完整 `exec_command` 返回对象实际提供 `chunk_id=0b6f51`、`exit_code=0`；退出码属于整个 shell 命令，不是另行保存的 unittest 进程元数据。

实际输出中的 unittest 尾部：

```text
----------------------------------------------------------------------
Ran 94 tests in 5.763s

OK
```

输出随后还包含 fixture OCI digest 打印。原始完整输出没有落盘。

这次聚合包含 recovery 的 17 项及其他 6 个模块的 77 项。之后增加了本机发布拒绝活动 Actions writer 的一项 recovery 合同，并修改 `local_recheck` 对应实现。实际只重跑直接受影响的 recovery 模块：

```bash
/Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python -m unittest discover -s .github/release-tools -p test_release_recovery.py && git diff --check
```

该调用只输出 `.output`，可见历史未提供 `exit_code`。实际 unittest 尾部：

```text
----------------------------------------------------------------------
Ran 18 tests in 0.682s

OK
```

因此先前“95 项通过”的准确含义是：未变的其他 77 项成功证据复用，加上当时最新 recovery 18 项成功证据，覆盖计数为 95。**没有单次聚合 95 项的执行记录或日志。**不能引用为 `Ran 95 tests / OK`。

之后独立复核发现的两个 P1 说明，当时 recovery mock 和输入反例存在覆盖缺口；95 的计数不能证明这些 P1 已受保护。后续修正如下。

其他 77 项的相关实现、配置、依赖和测试输入在后续 P1 修复中没有被本线程修改；后续改动仅为 `release_recovery.py`、`test_release_recovery.py` 和 release-tools README。其成功证据按该范围复用，不重新运行整套检查。未对聚合 94 时的每个输入文件记录内容哈希，因此这里是基于实际编辑范围的覆盖绑定，不是完整历史输入指纹。

## 四个修复前红→修复后绿的反例

下列命令在 P1 修复前、纠正了 API 桩并加入反例后实际执行：

```bash
PYTHONPATH=.github/release-tools /Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python -m unittest test_release_recovery.ReleaseRecoveryTest.test_tag_404_but_list_contains_partial_draft_resumes_original_id_and_assets test_release_recovery.ReleaseRecoveryTest.test_multiple_same_tag_releases_fail_before_any_remote_write test_release_recovery.ReleaseRecoveryTest.test_old_main_personal_release_can_checkout_current_main_and_must_block_publisher test_release_recovery.ReleaseRecoveryTest.test_existing_asset_conflict_and_unbound_draft_are_never_overwritten
```

该调用只输出 `.output`，可见历史没有退出码。修复前实际尾部：

```text
----------------------------------------------------------------------
Ran 4 tests in 0.215s

FAILED (failures=5, errors=1)
```

四个 testcase 中包含 subTest，因此 failure 数不能当作 testcase 数。原输出中确实出现以下错误/失败，不是本文追加的推断：

```text
sync.SyncError: 模拟真实服务：同 tag draft 已存在，重复 POST 为 422
AssertionError: SyncError not raised
AssertionError: False is not true
```

这些是依据真实 API 合同构造的内存服务反例，不是实际创建远端 draft 的日志。API 桩不再通过 tag 端点返回 draft：tag 查询为 404/None，维护者分页 List 能发现 partial draft；重复 POST 用 422 反例拒绝。

修复后，同一四项命令再次实际执行。可见历史仍没有退出码，实际 unittest 尾部：

```text
----------------------------------------------------------------------
Ran 4 tests in 0.251s

OK
```

对应可观察合同：

1. tag 404、List 可见 partial draft 时沿用原 ID 与已有资产，不再 POST 新 draft。
2. 同 tag 多个 Release 拒绝，在 registry 登录、latest 或 Release 写入前停止。
3. 旧 main O 的 queued/in-progress Personal release，即使当前 main 为 C，也阻断本机发布；不能按 current control SHA 将它过滤掉。
4. 未知 draft 或同名不同 digest/size 资产拒绝，零 registry 登录/tag 与零 Release 写入。

正式实现的发现依据是维护者可见的分页 List releases，匹配后按 ID 回读。官方 tag API 只查询已发布 Release；参见 [GitHub Release REST 合同](https://docs.github.com/en/rest/releases/releases?apiVersion=2026-03-10#get-a-release-by-tag-name)和 [gh draft 查询实现](https://github.com/cli/cli/blob/trunk/pkg/cmd/release/shared/fetch.go#L170)。这些链接是核对依据，不是本地运行日志。

## 最终 recovery “22 项”的准确来源

P1 修复完成后，实际执行：

```bash
/Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python -m unittest discover -s .github/release-tools -p test_release_recovery.py && git diff --check
```

该调用只输出 `.output`，可见历史没有退出码。实际 unittest 尾部：

```text
----------------------------------------------------------------------
Ran 21 tests in 0.805s

OK
```

之后仅增加分页发现反例：第一页 100 个非目标的已发布 Release，第二页才含目标 draft。生产实现未再变化。新增测试实际单独执行：

```bash
PYTHONPATH=.github/release-tools /Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python -m unittest test_release_recovery.ReleaseRecoveryTest.test_paginated_release_list_finds_draft_after_published_page && git diff --check && git status --short
```

该调用只输出 `.output`，可见历史没有退出码。实际输出中的 unittest 段末尾：

```text
----------------------------------------------------------------------
Ran 1 test in 0.007s

OK
```

该输出之后是工作树状态文本，不能将上述片段称为整条 shell 输出最后几行。

因此最终 recovery 覆盖为此前 21 项成功证据加新增分页 1 项成功证据，共 22 项。**本线程没有再次执行单次 22 项聚合，也没有 `Ran 22 tests` 的原始输出。**新增测试后只增加 `REAL_PAGES` 原函数引用及新 testcase，没有改写此前 21 项的测试体或生产实现。最后测得的候选哈希见上文。

## actionlint、bash -n 与 diff 检查

实际使用的固定 actionlint 路径：

```text
/Users/sc/.codex/validation/sub2api-kin/20261011-ci-001/bin/actionlint
```

曾实际执行以下组合：

```bash
/Users/sc/.codex/validation/sub2api-kin/20261011-ci-001/bin/actionlint .github/workflows/release-recovery.yml && bash -n .github/release-tools/release-images.sh && /Users/sc/.codex/validation/sub2api-kin/20261011-ci-002/venv/bin/python -m unittest discover -s .github/release-tools -p test_workflow_security.py
```

初次调用完整输出了返回对象，实际提供 `chunk_id=b21a8a`、`exit_code=1`。原错误：

```text
.github/workflows/release-recovery.yml:67:73: could not parse as YAML: mapping values are not allowed in this context [syntax-check]
   |
67 |         run: python -I -m pip install --require-hashes --only-binary=:all: -r "$RUNNER_TEMP/release-tools/requirements-release.txt"
   |                                                                         ^~
```

因 `&&` 短路，该次调用不能证明其后的 bash -n 或 workflow-security 已执行。随后将该 `run` 改为 YAML block scalar，再执行同一组合。该次只输出 `.output`，可见历史没有退出码；实际输出包含：

```text
----------------------------------------------------------------------
Ran 11 tests in 0.138s

OK
```

该尾部说明该组合已到达末尾的 workflow-security unittest；actionlint 和 bash -n 本身没有输出文本。但各子命令没有独立退出码日志，不能补成原始 `exit_code=0` 元数据。

两阶段 verify-only 工作流定型后还实际执行过：

```bash
git diff --check && /Users/sc/.codex/validation/sub2api-kin/20261011-ci-001/bin/actionlint .github/workflows/release-recovery.yml && bash -n .github/release-tools/release-images.sh
```

该调用仅输出 `.output`，可见输出为空，未提供 `exit_code`。没有落盘日志。先前交付将其报告为通过；本文保留“实际执行、输出为空、退出码未在可见历史提供”的证据边界，不制造成功退出码。

后续 P1 修复未改动 workflow 或 release-images.sh，因此没有重复 actionlint/bash -n。P1 修复及新增分页测试的命令均包含 `git diff --check`，相关工具输出没有 diff 错误文本；这些调用同样没有独立退出码元数据。

## 真实 OCI 与原 artifact 的只读检查

此部分是其他成功证据的定位，不替代上面的回归检查，也不代表云端 verification 或正式发布。

只读 OCI 输入在磁盘上：

```text
/Users/sc/.codex/validation/sub2api-kin/20261012-rel002/published-oci
```

本线程执行 Python 内联脚本调用 `supply_chain.inspect_image(...)`，使用原 source/version、linux/amd64 和固定主 digest。完整工具返回对象实际提供 `chunk_id=90d29f`、`exit_code=0`，原输出节选：

```json
{
  "digest": "sha256:fa433c2b36935717ecd7a84641998c88c02fc0d4a14b06f31009a0d525c995d3",
  "platforms": {
    "linux/amd64": "sha256:db1819ef0c08eeaa0c0400529aab4b22dad22c037f47b04dc9cc631f5145b8e6"
  },
  "attestation_types": {
    "sha256:db1819ef0c08eeaa0c0400529aab4b22dad22c037f47b04dc9cc631f5145b8e6": [
      "https://spdx.dev/Document",
      "https://slsa.dev/provenance/v1"
    ]
  },
  "created": "2026-10-12T16:38:21Z"
}
```

真实原 artifact 下载脚本使用 `original_run`、`artifacts_for`、`download`、`zip_data` 和原 producer 的 `verify_binaries`，并核对 binary-toolchain material。运行脚本使用临时目录，退出后目录由 `TemporaryDirectory` 清理，没有保存原始检查日志或另存 ZIP。脚本通过 session `73567` 续取结果；完整 `write_stdin` 返回实际提供 `chunk_id=745663`、`exit_code=0`。

实际输出登记的运输对象：

| artifact | ID | 验证字节数 | 输出中的 SHA256 |
|---|---:|---:|---|
| version-file | 11759893580 | 150 | `d75873a863647f43abd437914234e87aade0649208dc5e766b90eaa7dc6c5f35` |
| release-guard | 11760182811 | 94309 | `817bfed38e99fdcc1f379fb82a082b2d0d8de6b0c13400ebaa8d592063c35c28` |
| release-binary-linux-amd64 | 11760283330 | 39339278 | `16762055073b050dd1c0caac22bfcdad071a228a168b83a7470940395ebfdad7` |
| release-evidence-binary-linux-amd64 | 11760313311 | 186975 | `3e4d6498e572c979ba47bd82e8b96e34c71817deb4f6b58d684c55d9f799a66b` |
| toolchain-binary-linux-amd64 | 11759624897 | 885 | `bb3d8c47eb8224feb756522730a150331a8024865f1607e0613aa37bc8db61f6` |

输出末尾实际包含：

```json
  "exact_producer_verification": true,
  "toolchain_material_verification": true
```

主代理提供的以下磁盘路径仅是原始输入/已有记录，不是本线程生成的 raw log：

```text
/Users/sc/.codex/validation/sub2api-kin/20261012-rel002/release-artifacts
/Users/sc/.codex/validation/sub2api-kin/20261012-rel002/oci-layout-diagnosis.json
/Users/sc/.codex/validation/sub2api-kin/20261012-rel002/partial-publication.json
```

## 当前证据缺口与未执行项

- 没有聚合 95 项或最终 recovery 22 项的单次原始日志；本文只登记实际 94、18、21、1 和四项红→绿运行。
- 大部分调用只保留 `.output`，缺少可见的 shell `exit_code`；本文没有补造这些字段。明确出现的退出码只按相应完整工具返回对象登记。
- 没有本线程创建的持久化 raw log 路径。本文是根据 runtime 输出整理的证据记录，本身不能冒充原始运行日志。
- 没有聚合 94 时全部输入文件的历史内容哈希；复用范围依据实际编辑范围，不扩称完整可重放快照。
- 本线程没有执行最终独立复核的 14 个反例，没有该复核原始输出。
- 云端 verify-only workflow、实际本机 registry/Release 写入、生产部署均未由本线程执行；测试中的 API/draft 反例为内存服务合同模拟。
- GitHub 与 registry 之间仍无原子事务，检查后的新外部并发变化可能留下部分状态；本线程检查不证明消除了这一窗口。

本次证据整理没有重跑任何验证，没有修改 `.github` 或主代理文件，也没有 commit/push/发布。
