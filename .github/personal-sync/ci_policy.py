"""CI 路径分类、手动完整检查与结果聚合；不执行远端写入。"""

import argparse
import json
import os
from pathlib import Path, PurePosixPath
import subprocess

from sync import SHA, SyncError, git

FULL_JOBS = {
    "existing-ci / shell", "existing-ci / test", "existing-ci / frontend",
    "existing-ci / golangci-lint", "existing-ci / release-helpers",
    "existing-security / backend-security", "existing-security / frontend-security",
    "sync-contracts", "full-ci-certified", "personal-ready",
}
# 只对已确认的局部展示/无状态辅助合同标记低风险；风险标记供维护者选择验证。
# 账号、用户、配置和凭据经常不含敏感关键词，不能以名称未命中推断安全。
LOW_RISK_PATHS = {
    "backend/internal/pkg/pagination/pagination.go",
    "backend/internal/pkg/pagination/pagination_test.go",
    "frontend/src/components/common/HelpTooltip.vue",
    "frontend/src/components/common/__tests__/HelpTooltip.spec.ts",
}


def is_document(path):
    return (path in {"LICENSE", "NOTICE", "SECURITY.md"} or
            (path.endswith(".md") and ("/" not in path or
             path.startswith(("docs/", "openspec/", ".github/")))))


def classify(paths):
    flags = dict.fromkeys(("docs", "backend", "frontend", "migrations", "dependencies",
                          "workflows", "deploy", "high_risk", "go_dependencies",
                          "frontend_dependencies", "tooling"), False)
    for path in paths:
        if path.startswith("/") or ".." in PurePosixPath(path).parts:
            raise SyncError("变更路径必须位于仓库内")
        if is_document(path):
            flags["docs"] = True
            continue
        if path.startswith("backend/"):
            flags["backend"] = True
            flags["high_risk"] |= path not in LOW_RISK_PATHS
        elif path.startswith("frontend/"):
            flags["frontend"] = True
            flags["high_risk"] |= path not in LOW_RISK_PATHS
        elif path.startswith("deploy/"):
            flags["deploy"] = flags["high_risk"] = True
        elif path.startswith((".github/", "tools/")):
            flags["tooling"] = flags["high_risk"] = True
        else:
            # 未归类的构建、配置或运行文件需要人工复核，不能冒充文档。
            flags["tooling"] = flags["high_risk"] = True
        flags["migrations"] |= path.startswith(("backend/migrations/", "backend/ent/migrate/"))
        flags["workflows"] |= path.startswith(".github/workflows/")
        flags["go_dependencies"] |= path in {"backend/go.mod", "backend/go.sum"}
        flags["frontend_dependencies"] |= path in {"frontend/package.json", "frontend/pnpm-lock.yaml"}
    flags["dependencies"] = flags["go_dependencies"] or flags["frontend_dependencies"]
    return flags


def changed_paths(base, source):
    if not SHA.fullmatch(source) or not SHA.fullmatch(base) or base == "0" * 40:
        raise SyncError("无法确定完整 diff 基础；请固定有效 base SHA")
    # 禁用 rename 检测，同时检查旧路径和新路径，避免重命名绕过敏感路径。
    result = subprocess.run(["git", "diff", "--name-only", "--no-renames", "-z", base, source],
                            check=True, stdout=subprocess.PIPE)
    return [p for p in result.stdout.decode().split("\0") if p]


def successful_jobs(jobs, required):
    return all(any(j["name"] == name and j["status"] == "completed" and
                   j["conclusion"] == "success" for j in jobs) for name in required)


def ci_title(source, base, request):
    return f"Personal CI | {source} | base {base} | request {request}"


def plan(event, event_name, source, requested="quick", personal=False):
    if requested not in {"quick", "full"}:
        raise SyncError("CI mode 必须为 quick 或 full")
    pr = event.get("pull_request", {})
    if pr:
        base = pr["base"]["sha"]
    elif event_name == "push":
        base = event["before"]
    else:
        base = os.environ.get("BASE_SHA", "")
    if not personal and event_name == "workflow_dispatch" and requested == "quick" and base == source:
        raise SyncError("手动 Quick 必须指定不同于候选的有效 base SHA；完整检查请选择 full")
    paths = changed_paths(base, source)
    flags = classify(paths)
    # PR、push 和机器人反馈都保持 Quick，Full 只接受明确的手动请求。
    mode = requested if event_name == "workflow_dispatch" else "quick"
    check_only = personal and event_name == "pull_request" and pr.get("head", {}).get("ref", "").startswith(
        ("codex/sync-klno-", "codex/verify-personal-ci-"))
    return {**flags, "mode": mode, "source_sha": source, "diff_base": base,
            "paths": paths,
            "check_only": check_only,
            "require_full": personal and mode == "full",
            "scan_go": mode == "full" or flags["go_dependencies"],
            "scan_frontend": mode == "full" or flags["frontend_dependencies"]}


def require_results(results, selected):
    failures = {name: results.get(name, {}).get("result", "missing") for name in selected
                if results.get(name, {}).get("result") != "success"}
    unexpected = {name: value["result"] for name, value in results.items()
                  if name not in selected and value["result"] not in {"success", "skipped"}}
    if failures or unexpected:
        raise SyncError(f"必要检查失败、取消或缺失：{failures | unexpected}")


def emit(value):
    print(json.dumps(value, ensure_ascii=False, indent=2))
    if os.environ.get("GITHUB_OUTPUT"):
        with open(os.environ["GITHUB_OUTPUT"], "a") as stream:
            stream.write("plan=" + json.dumps(value, separators=(",", ":")) + "\n")
            for key, item in value.items():
                if isinstance(item, (str, bool)):
                    stream.write(f"{key}={str(item).lower() if isinstance(item, bool) else item}\n")
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a") as stream:
            stream.write("```json\n" + json.dumps(value, ensure_ascii=False, indent=2) + "\n```\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("plan", "aggregate", "certify"))
    parser.add_argument("--personal", action="store_true")
    args = parser.parse_args()
    if args.command == "plan":
        supplied = os.environ.get("CI_PLAN", "")
        value = json.loads(supplied) if supplied else plan(
            json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text()), os.environ["GITHUB_EVENT_NAME"],
            git("rev-parse", "HEAD"), os.environ.get("CI_MODE", "quick"), args.personal)
        emit(value)
        return
    value = json.loads(os.environ["CI_PLAN"])
    results = json.loads(os.environ["RESULTS"])
    if args.command == "certify":
        if value["mode"] != "full" or os.environ["GITHUB_EVENT_NAME"] != "workflow_dispatch":
            raise SyncError("完整检查认证只接受手动 Full，不接受 Quick 或自动事件")
        require_results(results, {"binding", "existing-ci", "existing-security", "sync-contracts"})
    elif args.personal:
        selected = {"binding"} if value["check_only"] else {"binding", "existing-ci"}
        if not value["check_only"] and (value["scan_go"] or value["scan_frontend"]):
            selected.add("existing-security")
        if not value["check_only"] and (value["mode"] == "full" or value["tooling"] or value["workflows"]):
            selected.add("sync-contracts")
        if value["require_full"]:
            selected.add("full-ci-certified")
        require_results(results, selected)
    else:
        selected = {"changes"}
        for job, flag in (("test", "backend"), ("frontend", "frontend"),
                          ("golangci-lint", "backend"), ("shell", "deploy"),
                          ("release-helpers", "tooling"), ("docs", "docs")):
            if value["mode"] == "full" or (job != "golangci-lint" and value[flag]) or (job == "release-helpers" and value["workflows"]):
                selected.add(job)
        require_results(results, selected)


if __name__ == "__main__":
    try:
        main()
    except (SyncError, ValueError, KeyError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error))
