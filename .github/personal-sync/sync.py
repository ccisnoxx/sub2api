#!/usr/bin/env python3
"""从固定 KlN 发布线准备保留 personal 历史的同步 PR。仅使用标准库。"""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile


UPSTREAM = "KlN-4096/sub2api"
REPOSITORY = "ccisnoxx/sub2api"
TAG = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-klno\.(0|[1-9][0-9]*)")
SHA = re.compile(r"[0-9a-f]{40}")
SOURCE = "deploy/personal-source.json"
METADATA = ".github/personal-sync/candidate.json"
CONTROL = [".github/personal-sync", ".github/workflows/sync-upstream.yml",
           ".github/workflows/personal-ci.yml"]


class SyncError(RuntimeError):
    pass


def run(*args, check=True):
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and result.returncode:
        raise SyncError(f"命令失败：{args[0]} {args[1]}\n{result.stderr.strip()}")
    return result


def git(*args):
    return run("git", *args).stdout.strip()


def ancestor(old, new):
    result = run("git", "merge-base", "--is-ancestor", old, new, check=False)
    if result.returncode not in (0, 1):
        raise SyncError(result.stderr)
    return result.returncode == 0


def version(tag):
    match = TAG.fullmatch(tag)
    if not match:
        raise SyncError("标签必须为 vX.Y.Z-klno.N，版本段不允许前导零")
    return tuple(map(int, match.groups()))


def select_tag(refs):
    tags = [ref.removeprefix("refs/tags/") for ref in refs
            if ref.startswith("refs/tags/") and TAG.fullmatch(ref.removeprefix("refs/tags/"))]
    if not tags:
        raise SyncError("KlN 发布线没有有效发布标签")
    return max(tags, key=version)


def validate_input(target):
    if target and not (TAG.fullmatch(target) or SHA.fullmatch(target)):
        raise SyncError("输入只接受 KlN 发布标签或 40 位小写完整 SHA")


def source_at(base):
    source = json.loads(git("show", f"{base}:{SOURCE}"))
    if source.get("upstream_repository") != UPSTREAM or not SHA.fullmatch(source.get("upstream_sha", "")):
        raise SyncError("来源记录的仓库或 SHA 无效")
    if source.get("upstream_tag") is not None:
        version(source["upstream_tag"])
    if not ancestor(source["upstream_sha"], base):
        raise SyncError("来源 SHA 不在 personal 历史中")
    return source


def remote_head(remote, branch):
    rows = git("ls-remote", "--heads", remote, f"refs/heads/{branch}").splitlines()
    return rows[0].split()[0] if rows else None


def assert_base(base):
    if remote_head("origin", "personal") != base:
        raise SyncError("personal 基础分支已变化；停止并从最新基础重新运行")


def prepare(base, tag, target, rehearsal=False):
    """调用前目标对象已固定；返回候选信息，冲突或历史改写不产生远端写入。"""
    source = source_at(base)
    if not ancestor(source["upstream_sha"], target):
        raise SyncError("上游目标不是记录来源的后代，可能回退或改写历史；需要人工差异审查")
    if tag and source["upstream_tag"] and version(tag) < version(source["upstream_tag"]):
        raise SyncError("拒绝回退到更早发布标签")
    if not rehearsal and target == source["upstream_sha"] and tag == source["upstream_tag"]:
        return {"state": "no_update", "base_sha": base, "upstream_sha": target, "upstream_tag": tag}

    label = tag.removeprefix("v") if tag else f"sha-{target[:12]}"
    branch = (f"codex/verify-personal-ci-{base[:12]}" if rehearsal else
              f"codex/sync-klno-{label}-{base[:12]}-{target[:12]}")
    # 本地工作树由 workflow checkout 或临时仓库持有，不重置调用者的已有文件。
    git("switch", "--detach", base)
    if not rehearsal:
        result = run("git", "merge", "--no-ff", "--no-edit", target, check=False)
        if result.returncode:
            conflicts = git("diff", "--name-only", "--diff-filter=U")
            run("git", "merge", "--abort")
            raise SyncError(f"合并停止；目标 {target}；冲突文件：\n{conflicts}\n{result.stderr.strip()}")
        changed = git("diff", "--name-only", base, "HEAD", "--", *CONTROL)
        if changed:
            raise SyncError(f"上游修改了个人同步控制文件，需要人工审查，不能自动覆盖：\n{changed}")
        Path(SOURCE).write_text(json.dumps({"upstream_repository": UPSTREAM,
            "upstream_tag": tag, "upstream_sha": target}, indent=2) + "\n")
    metadata = {"mode": "rehearsal" if rehearsal else "upgrade", "base_sha": base,
                "upstream_tag": tag, "upstream_sha": target}
    Path(METADATA).parent.mkdir(parents=True, exist_ok=True)
    Path(METADATA).write_text(json.dumps(metadata, indent=2) + "\n")
    git("add", SOURCE, METADATA)
    git("commit", "-m", "ci: 演练机器人候选检查（无上游升级）" if rehearsal else f"chore: 记录 KlN 来源 {tag or target}")
    return {**metadata, "state": "candidate", "branch": branch, "candidate_sha": git("rev-parse", "HEAD"),
            "migrations": git("diff", "--name-only", base, "HEAD", "--", "backend/migrations", "backend/ent/migrate")}


def publish(candidate):
    base, branch = candidate["base_sha"], candidate["branch"]
    assert_base(base)
    existing = remote_head("origin", branch)
    if existing:
        git("fetch", "--no-tags", "origin", f"refs/heads/{branch}")
        if git("rev-parse", f"{existing}^{{tree}}") != git("rev-parse", "HEAD^{tree}") or not ancestor(base, existing):
            raise SyncError("同名候选内容不一致；禁止覆盖或强推，请人工审查")
        candidate["candidate_sha"] = existing
    else:
        result = run("git", "push", "origin", f"HEAD:refs/heads/{branch}", check=False)
        if result.returncode:
            raise SyncError("候选普通推送失败；核对分支规则及 Contents/Workflows 写入权限。"
                            "可用已授权本机凭据处理，不能删除工作流改动或强推。\n" + result.stderr.strip())
    assert_base(base)
    return candidate


def gh_json(*args):
    return json.loads(run("gh", *args).stdout)


def create_pr(candidate):
    branch = candidate["branch"]
    prs = gh_json("pr", "list", "--repo", REPOSITORY, "--state", "all", "--head", branch,
                  "--base", "personal", "--json", "number,url,state,headRefOid")
    if prs and (len(prs) != 1 or prs[0]["state"] != "OPEN" or prs[0]["headRefOid"] != candidate["candidate_sha"]):
        raise SyncError("已有候选 PR 已关闭、合并或被修改；停止，尊重人工处理结果")
    mode = "流程演练：无上游升级，请勿合并" if candidate["mode"] == "rehearsal" else f"同步 KlN {candidate['upstream_tag'] or candidate['upstream_sha']}"
    body = f"""{mode}

- 上游：`{UPSTREAM}`，标签 `{candidate['upstream_tag']}`，SHA `{candidate['upstream_sha']}`
- personal 基础：`{candidate['base_sha']}`
- 候选 SHA：`{candidate['candidate_sha']}`
- 迁移文件：{candidate['migrations'] or '无'}
- 检查：[候选 SHA 的 Actions](https://github.com/{REPOSITORY}/actions?query=branch%3A{branch})；创建 PR 不代表检查成功。

显式 dispatch Personal CI，包含 TPS 检查及现有 CI/Security Scan。只接受候选 SHA 和最新 personal 基础对应的成功结果；基础变化后重新准备候选。
升级 PR 必须使用 **merge commit**，保留 TPS 与上游祖先关系，不使用 squash/rebase。首次 TPS 发布仍使用原上游基线；本 PR 不触发发布或部署。
"""
    with tempfile.TemporaryDirectory() as directory:
        body_file = Path(directory) / "body.md"
        body_file.write_text(body)
        if prs:
            run("gh", "pr", "edit", str(prs[0]["number"]), "--repo", REPOSITORY,
                "--title", mode, "--body-file", str(body_file))
            pr = prs[0]
        else:
            url = run("gh", "pr", "create", "--repo", REPOSITORY, "--base", "personal", "--head", branch,
                      "--title", mode, "--body-file", str(body_file), "--draft").stdout.strip()
            pr = gh_json("pr", "view", url, "--repo", REPOSITORY, "--json", "number,url,headRefOid")
    candidate.update(pr_number=pr["number"], pr_url=pr["url"])
    assert_base(candidate["base_sha"])
    return candidate


def dispatch_ci(candidate):
    title = f"Personal CI | {candidate['candidate_sha']} | base {candidate['base_sha']}"
    runs = gh_json("api", f"repos/{REPOSITORY}/actions/workflows/personal-ci.yml/runs?head_sha={candidate['candidate_sha']}&per_page=100")
    matching = [r for r in runs["workflow_runs"] if r["display_title"] == title and r["head_branch"] == candidate["branch"]]
    if matching and (matching[0]["status"] != "completed" or matching[0]["conclusion"] == "success"):
        candidate["ci_run"] = matching[0]["html_url"]
        return candidate
    run("gh", "workflow", "run", "personal-ci.yml", "--repo", REPOSITORY, "--ref", candidate["branch"],
        "-f", f"candidate_sha={candidate['candidate_sha']}", "-f", f"base_sha={candidate['base_sha']}",
        "-f", f"pr_number={candidate['pr_number']}")
    candidate["ci_run"] = "dispatched"
    return candidate


def main():
    if os.environ.get("GITHUB_REPOSITORY", REPOSITORY) != REPOSITORY:
        raise SyncError("工作流仅允许在个人 fork 运行")
    target_input = os.environ.get("SYNC_TARGET", "")
    rehearsal = os.environ.get("VERIFY_CI", "false") == "true"
    validate_input(target_input)
    if rehearsal and target_input:
        raise SyncError("CI 演练不能同时指定升级目标")
    if git("status", "--porcelain"):
        raise SyncError("工作树不干净，停止候选准备")
    git("fetch", "--no-tags", "origin", "refs/heads/personal")
    base = git("rev-parse", "FETCH_HEAD")
    git("remote", "add", "kln-source", f"https://github.com/{UPSTREAM}.git")
    refs = dict(line.split()[::-1] for line in git("ls-remote", "kln-source", "refs/tags/v*-klno.*").splitlines())
    tag = None if SHA.fullmatch(target_input) else target_input or select_tag(refs)
    git("fetch", "--no-tags", "kln-source", "refs/heads/klno:refs/personal-sync/klno")
    if tag:
        if f"refs/tags/{tag}" not in refs:
            raise SyncError("指定标签不存在于 KlN 上游")
        # 独立引用空间允许标签重新解析；不覆盖本 fork 的任何标签。
        git("fetch", "--no-tags", "kln-source", f"+refs/tags/{tag}:refs/personal-sync/tags/{tag}")
        target = git("rev-parse", f"refs/personal-sync/tags/{tag}^{{commit}}")
        if target != refs.get(f"refs/tags/{tag}^{{}}", refs[f"refs/tags/{tag}"]):
            raise SyncError("上游标签在解析期间变化，停止并重新运行")
    else:
        target = target_input
        git("cat-file", "-e", f"{target}^{{commit}}")
    if not ancestor(target, "refs/personal-sync/klno"):
        raise SyncError("目标不属于 KlN klno 发布线")
    git("config", "user.name", "personal-sync-bot")
    git("config", "user.email", "41898282+github-actions[bot]@users.noreply.github.com")
    candidate = prepare(base, tag, target, rehearsal)
    if candidate["state"] == "candidate":
        candidate = dispatch_ci(create_pr(publish(candidate)))
    print(json.dumps(candidate, ensure_ascii=False, indent=2))
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as summary:
            summary.write("```json\n" + json.dumps(candidate, ensure_ascii=False, indent=2) + "\n```\n")


if __name__ == "__main__":
    try:
        main()
    except (SyncError, ValueError, KeyError) as error:
        raise SystemExit(str(error))
