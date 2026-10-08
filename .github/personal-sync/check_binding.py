#!/usr/bin/env python3
"""核对实际 checkout、候选及当前 personal；检查结束时必须再核对一次。"""

import json
import os
from pathlib import Path

from sync import REPOSITORY, SHA, SyncError, ancestor, assert_base, gh_json, git


def binding(event, event_name, candidate_input, base_input, pr_input, workflow_sha, workflow_ref):
    if event_name == "workflow_dispatch":
        candidate, base, number = candidate_input, base_input, pr_input
        if not SHA.fullmatch(candidate) or not SHA.fullmatch(base):
            raise SyncError("dispatch 必须提供完整候选 SHA 和基础 SHA")
        if candidate != workflow_sha:
            raise SyncError("dispatch ref 的 SHA 与候选 SHA 不一致，检查不能挂到其他提交")
    elif event_name == "pull_request":
        pr = event["pull_request"]
        candidate, base, number = pr["head"]["sha"], pr["base"]["sha"], str(event["number"])
    elif event_name == "push" and workflow_ref == "refs/heads/personal":
        candidate = base = workflow_sha
        number = ""
    else:
        raise SyncError("不支持的检查入口")
    if number and (not str(number).isdigit() or int(number) < 1):
        raise SyncError("PR 编号无效")
    return candidate, base, str(number)


def check():
    if os.environ["GITHUB_REPOSITORY"] != REPOSITORY:
        raise SyncError("检查只用于个人 fork")
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    candidate, base, number = binding(event, os.environ["GITHUB_EVENT_NAME"],
        os.environ.get("CANDIDATE_SHA", ""), os.environ.get("BASE_SHA", ""),
        os.environ.get("PR_NUMBER", ""), os.environ["GITHUB_SHA"], os.environ["GITHUB_REF"])
    if git("rev-parse", "HEAD") != candidate:
        raise SyncError("实际 checkout 不等于候选 SHA")
    assert_base(base)
    if not ancestor(base, candidate):
        raise SyncError("候选没有合入当前 personal 基础，必须更新后重新检查")
    if number:
        pr = gh_json("api", f"repos/{REPOSITORY}/pulls/{number}")
        if (pr["state"] != "open" or pr["base"]["ref"] != "personal" or
                pr["base"]["sha"] != base or pr["head"]["sha"] != candidate or
                pr["head"]["repo"]["full_name"] != REPOSITORY):
            raise SyncError("PR 的仓库、候选或基础已变化，旧检查不再有效")
    elif os.environ["GITHUB_REF"] != "refs/heads/personal" or candidate != base:
        raise SyncError("非 personal 检查必须绑定实际 PR")
    print(f"候选 {candidate}；当前 personal {base}；PR {number or '无'}")


if __name__ == "__main__":
    try:
        check()
    except (SyncError, ValueError, KeyError) as error:
        raise SystemExit(str(error))
