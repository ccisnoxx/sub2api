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
from sync import REPOSITORY, SHA, SyncError, UPSTREAM, ancestor, assert_base, git, source_at
from ci_policy import FULL_JOBS, ci_title, successful_jobs

PERSONAL_TAG = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-klno\.(0|[1-9][0-9]*)-tps\.([1-9][0-9]*)")
IMAGE = "ghcr.io/ccisnoxx/sub2api"


def request(url, headers=None, payload=None, missing=False, method=None):
    data = json.dumps(payload).encode() if payload is not None else None
    req = Request(url, data=data, headers=headers or {}, method=method)
    try:
        with urlopen(req, timeout=60) as response:
            body = response.read()
            return (json.loads(body) if body else None), response.headers
    except HTTPError as error:
        if missing and error.code == 404:
            return None, error.headers
        # 不能将权限、网络或服务失败解释为不存在，也不输出认证内容。
        raise SyncError(f"远端请求失败：HTTP {error.code}，{url.split('?')[0]}") from error


def api(path, payload=None, missing=False, method=None, *, api_version="2026-03-10"):
    return request("https://api.github.com/" + path, {
        "Authorization": "Bearer " + os.environ["GH_TOKEN"],
        "Accept": "application/vnd.github+json", "Content-Type": "application/json",
        "X-GitHub-Api-Version": api_version}, payload, missing, method)[0]


def pages(path, key=None):
    result = []
    for page in range(1, 101):
        data = api(f"{path}{'&' if '?' in path else '?'}per_page=100&page={page}")
        rows = data[key] if key else data
        result.extend(rows)
        if len(rows) < 100:
            return result
    raise SyncError("分页超过上限，停止而不是使用不完整结果")


def trusted_ci(run, sha, workflow_id, *, base=None, branch="personal"):
    return (run["workflow_id"] == workflow_id and run["path"] == ".github/workflows/personal-ci.yml"
            and run["repository"]["full_name"] == REPOSITORY
            and run["head_repository"]["full_name"] == REPOSITORY
            and run["head_branch"] == branch and run["head_sha"] == sha
            and run["status"] == "completed" and run["conclusion"] == "success"
            and run["event"] == "workflow_dispatch"
            and run["display_title"] == ci_title(sha, base or sha, "full"))


def ci_bindings(sha):
    """先接受最终提交；仅普通同树 PR merge 可复用合并前的完整检查。"""
    yield sha, sha, "personal"
    parents = git("show", "-s", "--format=%P", sha).split()
    if len(parents) != 2 or not all(SHA.fullmatch(p) for p in parents):
        return
    base, candidate = parents
    # 完整 root tree 包括工作流、依赖、权限与文档；不以运行路径子集代替。
    if (git("rev-parse", f"{sha}^{{tree}}") != git("rev-parse", f"{candidate}^{{tree}}")
            or not ancestor(base, candidate)):
        return
    for row in pages(f"repos/{REPOSITORY}/commits/{sha}/pulls"):
        if not row["merged_at"]:
            continue
        # 2026-03-10 删除了 merge_commit_sha；详情固定旧版，不能省略合并身份校验。
        pr = api(f"repos/{REPOSITORY}/pulls/{row['number']}", api_version="2022-11-28")
        if (pr["merged"] is True and pr["state"] == "closed" and pr["merge_commit_sha"] == sha
                and pr["base"]["repo"]["full_name"] == REPOSITORY
                and pr["base"]["ref"] == "personal" and pr["base"]["sha"] == base
                and pr["head"]["repo"] is not None
                and pr["head"]["repo"]["full_name"] == REPOSITORY
                and pr["head"]["sha"] == candidate):
            yield candidate, base, pr["head"]["ref"]


def release_evidence(run):
    """核对已绑定手动运行的全部必要 job，不依赖分支保护或其他运行。"""
    jobs = pages(f"repos/{REPOSITORY}/actions/runs/{run['id']}/jobs?filter=latest", "jobs")
    return successful_jobs(jobs, FULL_JOBS)


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
    requested = None
    if run_id:
        if not str(run_id).isdigit():
            raise SyncError("CI run ID 无效")
        requested = api(f"repos/{REPOSITORY}/actions/runs/{run_id}")
    for candidate, base, branch in ci_bindings(sha):
        query = urlencode({"head_sha": candidate, "branch": branch})
        runs = [requested] if requested is not None else pages(
            f"repos/{REPOSITORY}/actions/workflows/personal-ci.yml/runs?{query}", "workflow_runs")
        for run in runs:
            if not trusted_ci(run, candidate, workflow["id"], base=base, branch=branch):
                continue
            if release_evidence(run):
                return source, run
            if requested is not None:
                raise SyncError("发布必须有同一次手动 Full CI 的全部成功检查；Quick、跳过或失败不能用于发布")
    if requested is not None:
        raise SyncError("CI 不是最终 personal 或其已合并同树候选的可信成功检查")
    raise SyncError("最终 personal SHA 没有可信的成功手动 Full CI，也没有可复用的同树合并检查")


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


def bound_release(tag, sha, ci_run_id):
    """远端查询后及每次写入前，重新读取同一源码的 CI 和标签占用。"""
    source, run = gate(sha, ci_run_id)
    if not tag.startswith(source["upstream_tag"] + "-tps.") or tags().get(tag) != sha:
        raise SyncError("个人标签不等于最终 SHA 或来源版本")
    return source, run


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
        source, run = gate(args.sha, str(run["id"]))
        if not tag.startswith(source["upstream_tag"] + "-tps."):
            raise SyncError("分配的个人标签与当前来源版本不一致")
        assert_base(args.sha)
        api(f"repos/{REPOSITORY}/git/refs", {"ref": f"refs/tags/{tag}", "sha": args.sha})
    if tags().get(tag) != args.sha:
        raise SyncError("个人标签发生冲突，禁止移动标签")
    digest, completed = publication(tag, args.sha)
    source, run = bound_release(tag, args.sha, str(run["id"]))
    assert_base(args.sha)
    if completed:
        output({"state": "already_published", "tag": tag, "sha": args.sha, "digest": digest})
        return
    api(f"repos/{REPOSITORY}/actions/workflows/release.yml/dispatches", {"ref": "personal", "inputs": {
        "tag": tag, "source_sha": args.sha, "ci_run_id": str(run["id"]), "simple_release": True, "dry_run": False}})
    output({"state": "dispatched", "tag": tag, "sha": args.sha, "ci_run_id": run["id"],
        "ci_source_sha": run["head_sha"], "ci_reused": run["head_sha"] != args.sha,
        "upstream_tag": source["upstream_tag"], "upstream_sha": source["upstream_sha"], "digest": digest or ""})


def publication_authorization(sha, tag, ci_run_id):
    """共享远端授权所有者；入口各自核对工作流身份，不伪装另一入口的身份。"""
    if not PERSONAL_TAG.fullmatch(tag):
        raise SyncError("正式发布只接受个人版本标签")
    source, run = bound_release(tag, sha, ci_run_id)
    digest, completed = publication(tag, sha)
    source, run = bound_release(tag, sha, str(run["id"]))
    assert_base(sha)
    return {"skip": completed, "reuse_image": bool(digest), "digest": digest or "",
        "ci_run_id": run["id"], "upstream_tag": source["upstream_tag"], "upstream_sha": source["upstream_sha"]}


def authorize(args):
    if (os.environ["GITHUB_REF"] not in ("refs/heads/personal", "refs/tags/" + args.tag)
            or os.environ["GITHUB_SHA"] != args.sha):
        raise SyncError("正式 Release 必须从同一 personal SHA 的工具定义 dispatch")
    output(publication_authorization(args.sha, args.tag, args.ci_run_id))


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
