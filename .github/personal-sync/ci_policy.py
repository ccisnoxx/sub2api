#!/usr/bin/env python3
"""集中维护 Personal CI 的选择、同树复用和发布证据合同。仅使用标准库。"""

import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import zipfile

from sync import REPOSITORY, SHA, SOURCE, SyncError, ancestor, gh_json, git, source_at

SCHEMA = 1
POLICY = "personal-ci-scope-v1"
WORKFLOW = ".github/workflows/personal-ci.yml"
ARTIFACT = "personal-ci-evidence"
EVIDENCE_FILE = "ci-evidence.json"
MAX_ARTIFACT_BYTES = 1024 * 1024
FLAGS = ("run_backend", "run_frontend", "run_shell", "run_release_helpers", "run_tps",
         "run_sync_contracts", "run_backend_security", "run_frontend_security")
CHANGED = ("frontend_changed", "backend_changed", "dependencies_changed", "deploy_changed",
           "release_tools_changed", "wide_merge_changed")
JOB_FLAGS = {"existing-ci / unit": "run_backend", "existing-ci / integration": "run_backend",
             "existing-ci / recording-race": "run_backend", "existing-ci / golangci-lint": "run_backend",
             "existing-ci / frontend": "run_frontend", "existing-ci / shell": "run_shell",
             "existing-ci / release-helpers": "run_release_helpers", "tps": "run_tps",
             "sync-contracts": "run_sync_contracts", "existing-security / backend-security": "run_backend_security",
             "existing-security / frontend-security": "run_frontend_security"}
GROUPS = {"existing-ci": FLAGS[:4], "existing-security": FLAGS[6:],
          "tps": ("run_tps",), "sync-contracts": ("run_sync_contracts",)}
LOCKS = {"backend/go.mod", "backend/go.sum", "frontend/package.json", "frontend/pnpm-lock.yaml",
         ".github/release-tools/requirements-release.txt"}
TRUST_CONTROLS = {WORKFLOW, ".github/workflows/backend-ci.yml", ".github/workflows/security-scan.yml",
                  ".github/personal-sync/ci_policy.py", ".github/personal-sync/check_binding.py",
                  ".github/personal-sync/sync.py"}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"),
                                     ensure_ascii=True).encode()).hexdigest()


def require_sha(value):
    if not isinstance(value, str) or not SHA.fullmatch(value):
        raise SyncError("证据必须使用完整小写 SHA")
    return value


def api(path):
    return gh_json("api", f"repos/{REPOSITORY}/{path}")


def pages(path, key):
    result = gh_json("api", "--paginate", "--slurp", f"repos/{REPOSITORY}/{path}")
    if not isinstance(result, list):
        raise SyncError("GitHub 分页响应无效")
    return [row for page in result for row in (page[key] if key else page)]


def parents(sha):
    require_sha(sha)
    return git("rev-list", "--parents", "-n", "1", sha).split()[1:]


def tree(sha):
    return git("rev-parse", f"{require_sha(sha)}^{{tree}}")


def files_at(sha):
    return git("ls-tree", "-r", "-z", sha).split("\0")[:-1]


def text_at(sha, path):
    # 缺失只由已读取的 Git tree 确认；命令错误不能解释为缺失。
    paths = {row.split("\t", 1)[1] for row in files_at(sha)}
    return git("show", f"{sha}:{path}") if path in paths else ""


def changed_paths(base, candidate):
    fields = git("diff", "--name-status", "-z", "--find-renames", base, candidate, "--").split("\0")
    result, index = [], 0
    while index < len(fields) - 1:
        status = fields[index]
        count = 2 if status.startswith(("R", "C")) else 1
        result.extend(fields[index + 1:index + 1 + count])
        index += count + 1
    return sorted(set(result))


def workflow_parts(text):
    """项目 workflow 的顶层 jobs 块；全局定义变化保守选择该文件的全部合同。"""
    header, jobs, current = [], {}, None
    in_jobs = False
    for line in text.splitlines():
        if line == "jobs:":
            in_jobs = True
            continue
        if in_jobs and line and not line[0].isspace() and not line.startswith("#"):
            in_jobs, current = False, None
        match = re.fullmatch(r"  ([A-Za-z0-9_-]+):\s*", line) if in_jobs else None
        if match:
            current = match[1]
            jobs[current] = []
        if in_jobs and current:
            jobs[current].append(line)
        elif not in_jobs:
            header.append(line)
    return "\n".join(header), {name: "\n".join(lines) for name, lines in jobs.items()}


def workflow_selection(base, candidate, path):
    old_header, old = workflow_parts(text_at(base, path))
    new_header, new = workflow_parts(text_at(candidate, path))
    changed = set(old) | set(new) if old_header != new_header else {
        name for name in set(old) | set(new) if old.get(name) != new.get(name)}
    if path.endswith("backend-ci.yml"):
        mapping = {name.rsplit(" / ", 1)[1]: flag for name, flag in JOB_FLAGS.items()
                   if name.startswith("existing-ci / ")}
        mapping["test"] = "run_backend"  # 原有合并 job 拆分时保留影响归属。
    elif path.endswith("security-scan.yml"):
        mapping = {"backend-security": "run_backend_security", "frontend-security": "run_frontend_security"}
    else:
        mapping = {"tps": "run_tps", "sync-contracts": "run_sync_contracts"}
        selected = set()
        if "existing-ci" in changed:
            selected.update(FLAGS[:4])
        if "existing-security" in changed:
            selected.update(FLAGS[6:])
        mapping.update({"existing-ci": "run_sync_contracts", "existing-security": "run_sync_contracts"})
    if path != WORKFLOW:
        selected = set()
    selected.update(mapping[name] for name in changed if name in mapping)
    if changed - set(mapping) - {"binding", "changes", "personal-ready"}:
        selected.update(FLAGS[:4] if path.endswith("backend-ci.yml") else FLAGS)
    return selected | {"run_sync_contracts"}


def selection(base, candidate):
    paths = changed_paths(base, candidate) if base else []
    flags = dict.fromkeys(FLAGS, False)
    changed = dict.fromkeys(CHANGED, False)
    wide = not base or source_at(base) != source_at(candidate)
    for path in paths:
        if path.startswith(("docs/", "openspec/")) or ("/" not in path and path.endswith(".md")) or path == "LICENSE":
            continue
        if path in (WORKFLOW, ".github/workflows/backend-ci.yml", ".github/workflows/security-scan.yml"):
            for flag in workflow_selection(base, candidate, path):
                flags[flag] = True
        elif path.startswith(".github/personal-sync/"):
            flags["run_sync_contracts"] = flags["run_release_helpers"] = True
        elif path.startswith(".github/release-tools/") or path in (
                ".github/workflows/release.yml", ".github/workflows/personal-release.yml"):
            changed["release_tools_changed"] = flags["run_release_helpers"] = True
        elif path.startswith("frontend/"):
            changed["frontend_changed"] = flags["run_frontend"] = flags["run_tps"] = True
        elif path.startswith("backend/") or path == "tools/test_backend_unit_runner_test.py":
            changed["backend_changed"] = flags["run_backend"] = True
        elif path.startswith("deploy/personal/") or path == ".github/workflows/personal-deploy-ci.yml":
            changed["deploy_changed"] = flags["run_release_helpers"] = True
        elif path.startswith("deploy/") or path.startswith("Dockerfile") or path == ".dockerignore":
            changed["deploy_changed"] = flags["run_shell"] = True
        elif path in ("tools/check_pnpm_audit_exceptions.py", ".github/audit-exceptions.yml"):
            flags["run_frontend_security"] = True
        elif path == ".github/workflows/sync-upstream.yml":
            flags["run_sync_contracts"] = True
        else:
            # 未知路径可能参与构建；只有显式列出的文档可免应用检查。
            changed["frontend_changed"] = changed["backend_changed"] = True
            flags["run_backend"] = flags["run_frontend"] = flags["run_tps"] = True
        if path in LOCKS:
            changed["dependencies_changed"] = True
            if path.startswith("backend/"):
                flags["run_backend_security"] = True
            elif path.startswith("frontend/"):
                flags["run_frontend_security"] = True
    if wide:
        flags = dict.fromkeys(FLAGS, True)
        changed = dict.fromkeys(CHANGED, True)
    if flags["run_tps"]:
        flags["run_frontend"] = True
    return {**changed, **flags, "paths": paths}


def bindings(sha, base, selected, pr_number):
    rows = files_at(sha)
    controls, locks, selectors = [], [], []
    for row in rows:
        path = row.split("\t", 1)[1]
        if path.startswith((".github/", "tools/")) or path in LOCKS or path.endswith("Makefile") or (
                path.startswith("frontend/") and "/" not in path[len("frontend/"):]):
            controls.append(row)
        if path in LOCKS:
            locks.append(row)
        if path.startswith(".github/workflows/") and path.endswith((".yml", ".yaml")):
            for line in text_at(sha, path).splitlines():
                if (re.match(r"\s*(?:-\s*)?(uses|runs-on|(?:[a-z-]*version(?:-file)?)):", line)
                        or re.search(r"\bgo install\s+\S+@\S+", line)):
                    selectors.append([path, line.strip()])
        if path == "frontend/package.json":
            package = json.loads(text_at(sha, path))
            selectors.extend([[path, {key: package[key]}] for key in ("packageManager", "engines") if key in package])
        if path == "backend/go.mod":
            selectors.extend([[path, line] for line in text_at(sha, path).splitlines()
                              if line.startswith(("go ", "toolchain "))])
    inputs = {"source_tree": tree(sha), "base_sha": base, "pr_number": str(pr_number),
              "selected": {key: selected[key] for key in FLAGS}, "policy": POLICY}
    return {"tree": tree(sha), "config_digest": digest(controls), "lock_digest": digest(locks),
            "selectors": selectors, "input_digest": digest(inputs), "inputs": inputs}


def make_plan(candidate, base, number=""):
    require_sha(candidate)
    require_sha(base)
    if number and (not str(number).isdigit() or int(number) < 1):
        raise SyncError("PR 编号无效")
    history = parents(candidate)
    diff_base = base if base != candidate else (history[0] if history else "")
    if diff_base and not ancestor(diff_base, candidate):
        raise SyncError("选择基线不是候选祖先")
    selected = selection(diff_base, candidate)
    return {"schema": SCHEMA, "policy": POLICY, "repository": REPOSITORY,
            "candidate_sha": candidate, "base_sha": base, "diff_base": diff_base,
            "pr_number": str(number), "source": source_at(candidate), **selected,
            "bindings": bindings(candidate, diff_base, selected, number),
            "reused": False, "reuse_run_id": "", "reuse": None, "reuse_reason": "候选检查直接执行"}


def validate_plan(plan):
    expected = make_plan(plan["candidate_sha"], plan["base_sha"], plan["pr_number"])
    for key in ("schema", "policy", "repository", "candidate_sha", "base_sha", "diff_base",
                "pr_number", "source", "paths", "bindings", *CHANGED):
        if plan[key] != expected[key]:
            raise SyncError(f"CI 计划不等于实际 Git 输入：{key}")
    if type(plan["reused"]) is not bool or any(type(plan[key]) is not bool for key in FLAGS):
        raise SyncError("CI 选择项必须为布尔值")
    if any(plan[key] != (False if plan["reused"] else expected[key]) for key in FLAGS):
        raise SyncError("CI 计划选择项被改写")
    return expected


def validate_results(plan, results):
    for name in ("binding", "changes", *GROUPS):
        value = results[name]["result"]
        selected = name in ("binding", "changes") or any(plan[key] for key in GROUPS.get(name, ()))
        if value != "success" and (value != "skipped" or selected):
            raise SyncError(f"必要检查未通过：{name}={value}")


def validate_jobs(plan, jobs, complete=True):
    names = {}
    for job in jobs:
        name = job["name"]
        if name in names:
            raise SyncError(f"实际 job 重名：{name}")
        names[name] = job
        if name == "personal-ready" and not complete:
            if job["status"] != "in_progress":
                raise SyncError("attest 必须在实际 personal-ready job 内执行")
        elif job["status"] != "completed" or job["conclusion"] not in ("success", "skipped"):
            raise SyncError(f"实际 job 失败：{name}")
    required = ["binding", "changes"] + (["personal-ready"] if complete else [])
    required += [name for name, flag in JOB_FLAGS.items() if plan[flag]]
    for name in required:
        if name not in names or names[name]["status"] != "completed" or names[name]["conclusion"] != "success":
            raise SyncError(f"实际必要 job 缺失或未成功：{name}")
    return {name: job["conclusion"] for name, job in names.items() if name != "personal-ready"}


def timestamp(value):
    value = datetime.fromisoformat(value.replace("Z", "+00:00"))
    if value.tzinfo is None:
        raise SyncError("证据时间必须包含时区")
    return value


def download_bytes(artifact_id):
    result = subprocess.run(["gh", "api", f"repos/{REPOSITORY}/actions/artifacts/{artifact_id}/zip"],
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if result.returncode:
        raise SyncError("GitHub artifact 下载失败；不能解释为证据缺失")
    return result.stdout


def read_artifact(run, now=None):
    now = now or datetime.now(timezone.utc)
    artifacts = [a for a in pages(f"actions/runs/{run['id']}/artifacts?per_page=100", "artifacts")
                 if a["name"] == ARTIFACT]
    if not artifacts:
        return None
    if len(artifacts) != 1:
        raise SyncError("同一 run 有多份同名 CI 证据")
    artifact = artifacts[0]
    if artifact["expired"] or timestamp(artifact["expires_at"]) <= now:
        return None
    if artifact["workflow_run"]["id"] != run["id"] or artifact["workflow_run"]["head_sha"] != run["head_sha"]:
        raise SyncError("artifact 不属于所选 run/SHA")
    if not 0 < artifact["size_in_bytes"] <= MAX_ARTIFACT_BYTES:
        raise SyncError("CI 证据 artifact 大小无效")
    body = download_bytes(artifact["id"])
    if len(body) > MAX_ARTIFACT_BYTES:
        raise SyncError("CI 证据下载超过大小上限")
    actual_digest = "sha256:" + hashlib.sha256(body).hexdigest()
    if artifact["digest"] != actual_digest:
        raise SyncError("artifact 下载 digest 不匹配")
    try:
        with zipfile.ZipFile(io.BytesIO(body)) as archive:
            entries = archive.infolist()
            if len(entries) != 1 or entries[0].filename != EVIDENCE_FILE or entries[0].file_size > MAX_ARTIFACT_BYTES:
                raise SyncError("artifact 必须只含固定 ci-evidence.json")
            evidence = json.loads(archive.read(entries[0]))
    except (zipfile.BadZipFile, UnicodeError, json.JSONDecodeError) as error:
        raise SyncError("CI 证据 artifact 解析失败") from error
    if timestamp(artifact["created_at"]) < timestamp(run["run_started_at"]):
        raise SyncError("artifact 不属于当前 run attempt")
    return evidence


def valid_run(run, sha, workflow_id):
    return (run["workflow_id"] == workflow_id and run["path"].split("@", 1)[0] == WORKFLOW
            and run["repository"]["full_name"] == REPOSITORY
            and run["head_repository"]["full_name"] == REPOSITORY and run["head_sha"] == sha
            and run["status"] == "completed" and run["conclusion"] == "success")


def trusted_pr_controls(plan, workflow_sha=""):
    """自动 PR 的证据维护源码须与 base 及远端 main 完全相同。"""
    main_sha = require_sha(api("git/ref/heads/main")["object"]["sha"])
    # checkout 的全历史通常已有 main；缺少对象时只读取该精确 SHA。
    known = subprocess.run(["git", "cat-file", "-e", main_sha + "^{commit}"],
                           stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if known.returncode:
        git("fetch", "--no-tags", "origin", main_sha)
    commits = [plan["candidate_sha"], plan["base_sha"], main_sha]
    if workflow_sha:
        commits.append(workflow_sha)
    manifests = []
    for sha in commits:
        manifests.append([row for row in files_at(sha) if row.split("\t", 1)[1] in TRUST_CONTROLS])
    return (all(len(rows) == len(TRUST_CONTROLS) for rows in manifests)
            and all(rows == manifests[0] for rows in manifests[1:]))


def validate_workflow(evidence, run, trust_pr=True):
    sha = require_sha(evidence["workflow_sha"])
    plan = evidence["plan"]
    if run["event"] == "pull_request":
        if trust_pr and not trusted_pr_controls(plan):
            raise SyncError("自动 PR 的证据控制定义不等于 base/可信 main，不能复用")
        associations = [pr for pr in run["pull_requests"] if str(pr["number"]) == plan["pr_number"]
                        and pr["head"]["sha"] == plan["candidate_sha"]
                        and pr["base"]["sha"] == plan["base_sha"]]
        # GitHub 在 PR 合并后可能返回空 pull_requests；存在关联时仍须唯一匹配。
        if run["pull_requests"] and len(associations) != 1:
            raise SyncError("实际 PR run 的候选、base 或 PR 输入不一致")
    elif run["event"] not in ("workflow_dispatch", "push") or sha != run["head_sha"]:
        raise SyncError("run 无法证明实际执行 workflow_sha")
    commit = api(f"git/commits/{sha}")
    if commit["sha"] != sha or commit["tree"]["sha"] != plan["bindings"]["tree"]:
        raise SyncError("GitHub 实际 workflow commit tree 与输入不一致")
    if run["event"] == "pull_request":
        if [parent["sha"] for parent in commit["parents"]] != [plan["base_sha"], plan["candidate_sha"]]:
            raise SyncError("PR workflow_sha 不是实际输入的二父同树 merge")
        # 同树由 GitHub commit API 确认，控制定义已在两端与 main 固定。
    expected_refs = []
    if any(evidence["plan"][key] for key in FLAGS[:4]):
        expected_refs.append(".github/workflows/backend-ci.yml")
    if any(evidence["plan"][key] for key in FLAGS[6:]):
        expected_refs.append(".github/workflows/security-scan.yml")
    refs = run.get("referenced_workflows", [])
    for path in expected_refs:
        matching = [r for r in refs if r["path"].split("@", 1)[0] == f"{REPOSITORY}/{path}" and r["sha"] == sha
                    and (run["event"] != "pull_request" or r["ref"] == f"refs/pull/{plan['pr_number']}/merge")]
        if len(matching) != 1:
            raise SyncError(f"实际 reusable workflow 输入不一致：{path}")


def validate_evidence(evidence, run, workflow_id, now=None, allow_reuse=True):
    now = now or datetime.now(timezone.utc)
    plan = evidence["plan"]
    expected = validate_plan(plan)
    if (evidence["schema"] != SCHEMA or evidence["repository"] != REPOSITORY
            or evidence["run_id"] != run["id"] or evidence["run_attempt"] != run["run_attempt"]
            or not valid_run(run, plan["candidate_sha"], workflow_id)
            or run["display_title"] != f"Personal CI | {plan['candidate_sha']} | base {plan['base_sha']}"):
        raise SyncError("CI 证据的 run、attempt、仓库、workflow 或输入不匹配")
    validate_workflow(evidence, run)
    generated = timestamp(evidence["generated_at"])
    if generated < timestamp(run["run_started_at"]) or generated > timestamp(run["updated_at"]) or generated > now:
        raise SyncError("CI 证据时间不属于实际 run")
    validate_results(plan, evidence["results"])
    jobs = pages(f"actions/runs/{run['id']}/attempts/{run['run_attempt']}/jobs?per_page=100", "jobs")
    observed = validate_jobs(plan, jobs)
    if evidence["jobs"] != observed:
        raise SyncError("证据自报 jobs 与 GitHub 实际 jobs 不一致")
    if plan["reused"]:
        if not allow_reuse:
            raise SyncError("候选复用链不能继续嵌套复用")
        verify_reuse(plan, workflow_id, now)
    elif expected["run_backend_security"] or expected["run_frontend_security"]:
        scan_jobs = [job for job in jobs if JOB_FLAGS.get(job["name"]) in FLAGS[6:] and expected[JOB_FLAGS[job["name"]]]]
        # 仅重跑失败 job 时，当前 attempt 的官方列表会沿用本 run 早前成功的扫描。
        # 新证据仍绑定当前 attempt；扫描时效依据其实际完成时间，不能强迫重跑成功项。
        if any(not timestamp(run["created_at"]) <= timestamp(job["completed_at"]) <= generated for job in scan_jobs):
            raise SyncError("安全扫描时间不属于实际 run")
        if any(now - timestamp(job["completed_at"]) > timedelta(hours=24) for job in scan_jobs):
            return False
    return True


def merged_pr(merge_sha, candidate, base):
    matches = [pr for pr in pages(f"commits/{merge_sha}/pulls?per_page=100", None)
               if pr["merged_at"] and pr["merge_commit_sha"] == merge_sha
               and pr["state"] == "closed" and pr["base"]["ref"] == "personal"
               and pr["base"]["repo"]["full_name"] == REPOSITORY
               and pr["head"]["repo"]["full_name"] == REPOSITORY
               and pr["head"]["sha"] == candidate and pr["base"]["sha"] == base]
    if len(matches) > 1:
        raise SyncError("同一 merge 有多个匹配 PR，无法唯一绑定")
    return matches[0] if matches else None


def merge_candidate(plan):
    history = parents(plan["candidate_sha"])
    if len(history) != 2 or plan["candidate_sha"] != plan["base_sha"] or plan["pr_number"]:
        return None
    base, candidate = history
    if not ancestor(base, candidate) or tree(candidate) != tree(plan["candidate_sha"]):
        return None
    if source_at(candidate) != plan["source"]:
        raise SyncError("merge 候选来源不一致")
    return base, candidate


def verify_reuse(plan, workflow_id, now=None):
    relation = merge_candidate(plan)
    if not relation:
        raise SyncError("复用只接受普通二父同树 personal merge")
    base, candidate = relation
    pr = merged_pr(plan["candidate_sha"], candidate, base)
    reuse = plan["reuse"]
    if not pr or str(pr["number"]) != reuse["pr_number"]:
        raise SyncError("复用 PR 不等于实际 personal merge")
    run = api(f"actions/runs/{reuse['run_id']}")
    if run["event"] not in ("workflow_dispatch", "pull_request") or run["head_branch"] != pr["head"]["ref"]:
        raise SyncError("候选 run 必须是同仓库 PR 分支的检查")
    evidence = read_artifact(run, now)
    if evidence is None:
        raise SyncError("所复用的候选证据已缺失或过期")
    if not validate_evidence(evidence, run, workflow_id, now, allow_reuse=False):
        raise SyncError("候选安全扫描证据已超过 24 小时")
    candidate_plan = make_plan(candidate, base, str(pr["number"]))
    if evidence["plan"] != candidate_plan:
        raise SyncError("候选证据不等于真实 PR/基线检查输入")
    expected = make_plan(plan["candidate_sha"], plan["base_sha"])
    for key in ("tree", "config_digest", "lock_digest", "selectors"):
        if candidate_plan["bindings"][key] != expected["bindings"][key]:
            raise SyncError(f"复用绑定不一致：{key}")
    if {key: candidate_plan[key] for key in FLAGS} != {key: expected[key] for key in FLAGS}:
        raise SyncError("候选和最终 merge 的检查选择不一致")
    if reuse != {"run_id": run["id"], "run_attempt": run["run_attempt"], "pr_number": str(pr["number"]),
                 "candidate_sha": candidate, "base_sha": base, "evidence_digest": digest(evidence)}:
        raise SyncError("候选复用链摘要、attempt 或基线不匹配")
    if str(run["id"]) != plan["reuse_run_id"]:
        raise SyncError("复用 run 输出不一致")
    return evidence


def find_reuse(plan, now=None):
    relation = merge_candidate(plan)
    if not relation:
        plan["reuse_reason"] = "没有满足同树二父 merge 合同，执行所选检查"
        return plan
    base, candidate = relation
    pr = merged_pr(plan["candidate_sha"], candidate, base)
    if not pr:
        plan["reuse_reason"] = "没有唯一匹配的同仓库已合并 personal PR，执行所选检查"
        return plan
    workflow_id = api("actions/workflows/personal-ci.yml")["id"]
    runs = pages(f"actions/workflows/personal-ci.yml/runs?head_sha={candidate}&per_page=100", "workflow_runs")
    for listed in runs:
        if not valid_run(listed, candidate, workflow_id) or listed["event"] not in ("workflow_dispatch", "pull_request"):
            continue
        run = api(f"actions/runs/{listed['id']}")
        if not valid_run(run, candidate, workflow_id) or run["head_branch"] != pr["head"]["ref"]:
            continue
        evidence = read_artifact(run, now)
        if evidence is None:
            continue
        if run["event"] == "pull_request" and not trusted_pr_controls(evidence["plan"]):
            plan["reuse_reason"] = "自动 PR 的控制定义与 base/可信 main 不一致，执行所选检查"
            continue
        if not validate_evidence(evidence, run, workflow_id, now, allow_reuse=False):
            continue
        expected = make_plan(candidate, base, str(pr["number"]))
        if evidence["plan"] != expected:
            raise SyncError("候选 run 证据不等于实际 merge PR 输入")
        plan.update(reused=True, reuse_run_id=str(run["id"]),
                    reuse={"run_id": run["id"], "run_attempt": run["run_attempt"], "pr_number": str(pr["number"]),
                           "candidate_sha": candidate, "base_sha": base, "evidence_digest": digest(evidence)},
                    reuse_reason="同树候选的实际成功 CI 和证据已验证")
        plan.update(dict.fromkeys(FLAGS, False))
        verify_reuse(plan, workflow_id, now)
        return plan
    if not plan["reuse_reason"].startswith("自动 PR"):
        plan["reuse_reason"] = "候选成功证据缺失、过期或安全扫描超过 24 小时，执行所选检查"
    return plan


def verify_release_evidence(sha, run, workflow_id, now=None):
    evidence = read_artifact(run, now)
    if evidence is None:
        raise SyncError("新策略最终 personal CI 缺少有效证据")
    plan = evidence["plan"]
    if plan["candidate_sha"] != sha or plan["base_sha"] != sha or plan["pr_number"] or run["head_branch"] != "personal":
        raise SyncError("发布必须经过最终 personal 的新成功 CI，PR 证据不可直接发布")
    if not validate_evidence(evidence, run, workflow_id, now):
        raise SyncError("发布安全扫描证据已超过 24 小时")
    return evidence


def attest(plan, results, workflow_sha):
    validate_plan(plan)
    validate_results(plan, results)
    workflow_id = api("actions/workflows/personal-ci.yml")["id"]
    if plan["reused"]:
        verify_reuse(plan, workflow_id)
    run_id, attempt = int(os.environ["GITHUB_RUN_ID"]), int(os.environ["GITHUB_RUN_ATTEMPT"])
    run = api(f"actions/runs/{run_id}")
    if (run["run_attempt"] != attempt or run["head_sha"] != plan["candidate_sha"]
            or run["workflow_id"] != workflow_id or run["repository"]["full_name"] != REPOSITORY
            or run["head_repository"]["full_name"] != REPOSITORY
            or run["path"].split("@", 1)[0] != WORKFLOW
            or run["event"] != os.environ["GITHUB_EVENT_NAME"]
            or run["display_title"] != f"Personal CI | {plan['candidate_sha']} | base {plan['base_sha']}"):
        raise SyncError("当前 run attempt/SHA 已变化")
    validate_workflow({"workflow_sha": workflow_sha, "plan": plan}, run, trust_pr=False)
    jobs = pages(f"actions/runs/{run_id}/attempts/{attempt}/jobs?per_page=100", "jobs")
    return {"schema": SCHEMA, "repository": REPOSITORY, "run_id": run_id, "run_attempt": attempt,
            "workflow_sha": require_sha(workflow_sha), "generated_at": datetime.now(timezone.utc).isoformat(),
            "plan": plan, "results": results, "jobs": validate_jobs(plan, jobs, complete=False)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("plan", "attest"))
    mode = parser.parse_args().mode
    if os.environ["GITHUB_REPOSITORY"] != REPOSITORY:
        raise SyncError("CI 策略只允许在个人 fork 运行")
    if mode == "plan":
        plan = make_plan(os.environ["CANDIDATE_SHA"], os.environ["BASE_SHA"], os.environ.get("PR_NUMBER", ""))
        if os.environ["GITHUB_REF"] == "refs/heads/personal" and os.environ["GITHUB_EVENT_NAME"] in ("push", "workflow_dispatch"):
            plan = find_reuse(plan)
        payload = json.dumps(plan, ensure_ascii=False, separators=(",", ":"))
        print(payload)
        if os.environ.get("GITHUB_OUTPUT"):
            with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
                stream.write("plan_json=" + payload + "\n")
                for key in (*FLAGS, *CHANGED, "reused", "reuse_run_id"):
                    value = plan[key]
                    stream.write(f"{key}={str(value).lower() if type(value) is bool else value}\n")
        if os.environ.get("GITHUB_STEP_SUMMARY"):
            with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as stream:
                stream.write(plan["reuse_reason"] + "\n")
    else:
        evidence = attest(json.loads(os.environ["PLAN_JSON"]), json.loads(os.environ["RESULTS"]), os.environ["WORKFLOW_SHA"])
        Path(EVIDENCE_FILE).write_text(json.dumps(evidence, ensure_ascii=False, indent=2) + "\n")
        print(json.dumps(evidence, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except (SyncError, ValueError, KeyError, TypeError, OSError) as error:
        raise SystemExit(str(error))
