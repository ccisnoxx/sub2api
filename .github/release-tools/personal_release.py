#!/usr/bin/env python3
"""个人版本分配、可信 CI 门禁和不可覆盖的镜像恢复。"""

import argparse
import base64
import json
import os
from pathlib import Path
import re
import sys
from urllib.error import HTTPError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "personal-sync"))
from sync import REPOSITORY, SHA, SyncError, UPSTREAM, assert_base, git, source_at

PERSONAL_TAG = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-klno\.(0|[1-9][0-9]*)-tps\.([1-9][0-9]*)")
IMAGE = "ghcr.io/ccisnoxx/sub2api"


def request(url, headers=None, payload=None, missing=False):
    data = json.dumps(payload).encode() if payload is not None else None
    req = Request(url, data=data, headers=headers or {})
    try:
        with urlopen(req, timeout=60) as response:
            body = response.read()
            return (json.loads(body) if body else None), response.headers
    except HTTPError as error:
        if missing and error.code == 404:
            return None, error.headers
        # 不能将权限、网络或服务失败解释为不存在，也不输出认证内容。
        raise SyncError(f"远端请求失败：HTTP {error.code}，{url.split('?')[0]}") from error


def api(path, payload=None, missing=False):
    return request("https://api.github.com/" + path, {
        "Authorization": "Bearer " + os.environ["GH_TOKEN"],
        "Accept": "application/vnd.github+json", "Content-Type": "application/json",
        "X-GitHub-Api-Version": "2026-03-10"}, payload, missing)[0]


def pages(path, key=None):
    result = []
    for page in range(1, 101):
        data = api(f"{path}{'&' if '?' in path else '?'}per_page=100&page={page}")
        rows = data[key] if key else data
        result.extend(rows)
        if len(rows) < 100:
            return result
    raise SyncError("分页超过上限，停止而不是使用不完整结果")


def trusted_ci(run, sha, workflow_id):
    return (run["workflow_id"] == workflow_id and run["path"] == ".github/workflows/personal-ci.yml"
            and run["head_repository"]["full_name"] == REPOSITORY
            and run["head_branch"] == "personal" and run["head_sha"] == sha
            and run["status"] == "completed" and run["conclusion"] == "success"
            and run["event"] in ("push", "workflow_dispatch")
            and run["display_title"] == f"Personal CI | {sha} | base {sha}")


def gate(sha, run_id=""):
    if os.environ.get("GITHUB_REPOSITORY", REPOSITORY) != REPOSITORY or not SHA.fullmatch(sha):
        raise SyncError("发布仓库或完整源码 SHA 无效")
    if git("rev-parse", "HEAD") != sha:
        raise SyncError("实际 checkout 不等于待发布 SHA")
    assert_base(sha)
    source = source_at(sha)
    if not source["upstream_tag"]:
        raise SyncError("未发布上游 SHA 必须先明确版本归属")
    refs = dict(line.split()[::-1] for line in git("ls-remote", f"https://github.com/{UPSTREAM}.git",
        f"refs/tags/{source['upstream_tag']}", f"refs/tags/{source['upstream_tag']}^{{}}").splitlines())
    actual = refs.get(f"refs/tags/{source['upstream_tag']}^{{}}", refs.get(f"refs/tags/{source['upstream_tag']}"))
    if actual != source["upstream_sha"]:
        raise SyncError("来源记录不等于 KlN 实际发布标签")
    workflow = api(f"repos/{REPOSITORY}/actions/workflows/personal-ci.yml")
    if run_id:
        if not str(run_id).isdigit():
            raise SyncError("CI run ID 无效")
        run = api(f"repos/{REPOSITORY}/actions/runs/{run_id}")
    else:
        runs = pages(f"repos/{REPOSITORY}/actions/workflows/personal-ci.yml/runs?head_sha={sha}&branch=personal", "workflow_runs")
        matching = [r for r in runs if trusted_ci(r, sha, workflow["id"])]
        if not matching:
            raise SyncError("最终 personal SHA 没有可信的完整成功 CI")
        run = matching[0]
    if not trusted_ci(run, sha, workflow["id"]):
        raise SyncError("CI 不是当前仓库、最终 personal SHA 的成功检查；PR 结果不可用于发布")
    branch = api(f"repos/{REPOSITORY}/branches/personal")
    required = branch["protection"]["required_status_checks"]
    if not branch["protected"] or required["enforcement_level"] != "everyone":
        raise SyncError("personal 保护或管理员约束被关闭")
    if {"context": "personal-ready", "app_id": 15368} not in required["checks"]:
        raise SyncError("personal-ready/GitHub Actions 必要保护缺失")
    checks = pages(f"repos/{REPOSITORY}/commits/{sha}/check-runs?filter=latest", "check_runs")
    for rule in required["checks"]:
        matching = [c for c in checks if c["name"] == rule["context"]
                    and (rule["app_id"] in (None, -1) or c["app"]["id"] == rule["app_id"])
                    and c["status"] == "completed" and c["conclusion"] == "success"]
        if rule["context"] == "personal-ready":
            matching = [c for c in matching if c["check_suite"]["id"] == run["check_suite_id"]]
        if not matching:
            raise SyncError(f"最终 SHA 的实际必要检查未通过：{rule['context']}")
    return source, run


def tags():
    result = {}
    for row in git("ls-remote", "--tags", "origin", "refs/tags/*-tps.*").splitlines():
        sha, ref = row.split()
        name = ref.removeprefix("refs/tags/").removesuffix("^{}")
        if PERSONAL_TAG.fullmatch(name):
            if ref.endswith("^{}") or name not in result:
                result[name] = sha
    return result


def allocate(base, sha, existing):
    same = [tag for tag, target in existing.items() if target == sha and tag.startswith(base + "-tps.")]
    if len(same) > 1:
        raise SyncError("同一源码存在多个个人版本，停止并人工核对")
    if same:
        return same[0]
    numbers = [int(tag.rsplit(".", 1)[1]) for tag in existing if tag.startswith(base + "-tps.")]
    return f"{base}-tps.{max(numbers, default=0) + 1}"


def documentation_only(paths):
    return bool(paths) and all(p.startswith(("docs/", "openspec/")) or
        ("/" not in p and p.endswith(".md")) for p in paths)


def image_state(tag, sha):
    """读取实际 OCI config；已存在的版本必须匹配 SHA、版本及唯一 amd64 平台。"""
    version = tag.removeprefix("v")
    package = api("users/ccisnoxx/packages/container/sub2api", missing=True)
    if package is None:
        return None
    credentials = base64.b64encode(("ccisnoxx:" + os.environ["GH_TOKEN"]).encode()).decode()
    token, _ = request("https://ghcr.io/token?" + urlencode({"service": "ghcr.io",
        "scope": "repository:ccisnoxx/sub2api:pull"}), {"Authorization": "Basic " + credentials})
    headers = {"Authorization": "Bearer " + token["token"], "Accept":
        "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json"}
    root = "https://ghcr.io/v2/ccisnoxx/sub2api/"
    manifest, response_headers = request(root + "manifests/" + version, headers, missing=True)
    if manifest is None:
        return None
    digest = response_headers["Docker-Content-Digest"]
    if "manifests" in manifest:
        # Buildx 的证明清单不是运行平台；只能存在一个实际运行清单。
        runnable = [m for m in manifest["manifests"] if m.get("platform", {}).get("os") != "unknown"]
        if len(runnable) != 1 or runnable[0]["platform"] != {"architecture": "amd64", "os": "linux"}:
            raise SyncError("已有版本的镜像平台不等于唯一 linux/amd64")
        manifest, _ = request(root + "manifests/" + runnable[0]["digest"], headers)
    config, _ = request(root + "blobs/" + manifest["config"]["digest"], headers)
    labels = config["config"]["Labels"]
    if (config["os"] != "linux" or config["architecture"] != "amd64" or
            labels.get("org.opencontainers.image.revision") != sha or
            labels.get("org.opencontainers.image.version") != version or
            labels.get("org.opencontainers.image.source") != "https://github.com/" + REPOSITORY):
        raise SyncError("已有镜像的源码、版本或平台不匹配，禁止覆盖")
    return digest


def publication(tag, sha):
    digest = image_state(tag, sha)
    release = api(f"repos/{REPOSITORY}/releases/tags/{tag}", missing=True)
    if release and not release["draft"] and not digest:
        raise SyncError("已完成 Release 缺少镜像，禁止重建已发布版本")
    return digest, bool(release and not release["draft"] and digest)


def output(data):
    print(json.dumps(data, ensure_ascii=False, indent=2))
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
            for key, value in data.items():
                if isinstance(value, (str, bool, int)):
                    stream.write(f"{key}={str(value).lower() if isinstance(value, bool) else value}\n")
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as stream:
            stream.write("```json\n" + json.dumps(data, ensure_ascii=False, indent=2) + "\n```\n")


def prepare(args):
    source, run = gate(args.sha, args.ci_run_id)
    existing = tags()
    tag = allocate(source["upstream_tag"], args.sha, existing)
    if tag not in existing:
        previous = sorted((t for t in existing if t.startswith(source["upstream_tag"] + "-tps.")),
                          key=lambda t: int(t.rsplit(".", 1)[1]), reverse=True)
        for prior in previous:
            git("fetch", "--no-tags", "origin", f"refs/tags/{prior}")
            if publication(prior, existing[prior])[1]:
                paths = git("diff", "--name-only", existing[prior], args.sha).splitlines()
                if documentation_only(paths) or not paths:
                    output({"state": "documentation_only", "tag": prior, "sha": args.sha})
                    return
                break
        assert_base(args.sha)
        api(f"repos/{REPOSITORY}/git/refs", {"ref": f"refs/tags/{tag}", "sha": args.sha})
    if tags().get(tag) != args.sha:
        raise SyncError("个人标签发生冲突，禁止移动标签")
    digest, completed = publication(tag, args.sha)
    if completed:
        output({"state": "already_published", "tag": tag, "sha": args.sha, "digest": digest})
        return
    assert_base(args.sha)
    api(f"repos/{REPOSITORY}/actions/workflows/release.yml/dispatches", {"ref": "personal", "inputs": {
        "tag": tag, "source_sha": args.sha, "ci_run_id": str(run["id"]), "simple_release": True, "dry_run": False}})
    output({"state": "dispatched", "tag": tag, "sha": args.sha, "ci_run_id": run["id"],
        "upstream_tag": source["upstream_tag"], "upstream_sha": source["upstream_sha"], "digest": digest or ""})


def authorize(args):
    if not PERSONAL_TAG.fullmatch(args.tag):
        raise SyncError("正式发布只接受个人版本标签")
    if (os.environ["GITHUB_REF"] not in ("refs/heads/personal", "refs/tags/" + args.tag)
            or os.environ["GITHUB_SHA"] != args.sha):
        raise SyncError("正式 Release 必须从同一 personal SHA 的工具定义 dispatch")
    source, run = gate(args.sha, args.ci_run_id)
    if not args.tag.startswith(source["upstream_tag"] + "-tps.") or tags().get(args.tag) != args.sha:
        raise SyncError("个人标签不等于最终 SHA 或来源版本")
    digest, completed = publication(args.tag, args.sha)
    assert_base(args.sha)
    output({"skip": completed, "reuse_image": bool(digest), "digest": digest or "",
        "ci_run_id": run["id"], "upstream_tag": source["upstream_tag"], "upstream_sha": source["upstream_sha"]})


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=("prepare", "authorize", "verify-image"))
    parser.add_argument("--sha", required=True)
    parser.add_argument("--ci-run-id", default="")
    parser.add_argument("--tag", default="")
    args = parser.parse_args()
    if args.mode == "prepare":
        prepare(args)
    elif args.mode == "authorize":
        authorize(args)
    else:
        digest = image_state(args.tag, args.sha)
        if not digest:
            raise SyncError("发布后的镜像不存在")
        output({"tag": args.tag, "sha": args.sha, "digest": digest, "platform": "linux/amd64",
                "image": IMAGE + "@" + digest, "workflow_sha": os.environ["GITHUB_SHA"]})


if __name__ == "__main__":
    try:
        main()
    except (SyncError, ValueError, KeyError) as error:
        raise SystemExit(str(error))
