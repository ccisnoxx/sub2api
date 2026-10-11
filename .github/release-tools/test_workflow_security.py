"""检查工作流的权限、凭据、PR 隔离与产物上传边界。"""

import copy
from pathlib import Path
import re
import unittest

import yaml


WORKFLOWS = Path(__file__).resolve().parents[1] / "workflows"
READ = {"contents": "read"}
# 每项额外授权的实际 API/Git 用途见 docs/ci/CI-005.md。
JOB_PERMISSIONS = {
    ("release.yml", "prepare"): {**READ, "actions": "read", "checks": "read", "packages": "read"},
    ("release.yml", "release"): {"contents": "write", "packages": "write", "actions": "read", "checks": "read"},
    ("personal-release.yml", "prepare"): {"contents": "write", "actions": "write", "checks": "read", "packages": "read"},
    ("sync-upstream.yml", "sync"): {"contents": "write", "pull-requests": "write", "actions": "write"},
    ("personal-ci.yml", "binding"): {**READ, "pull-requests": "read"},
    ("personal-ci.yml", "personal-ready"): {**READ, "pull-requests": "read"},
}
EXCLUSIONS = {
    "!**/.env", "!**/.env.*", "!**/*.env", "!**/*.sql", "!**/*.dump",
    "!**/*.backup", "!**/*.bak", "!**/*.db", "!**/*.sqlite*", "!**/*.log", "!**/.git/**",
}
# 运输产物一天；dry-run 报告三天；脱敏构建证明九十天。
ARTIFACTS = {
    "release-tooling": (1, ["${{ runner.temp }}/release-tools/"]),
    "personal-sync-tooling": (1, ["${{ runner.temp }}/personal-sync/"]),
    "release-guard": (1, ["${{ runner.temp }}/release-guard/"]),
    "version-file": (1, ["backend/cmd/server/VERSION"]),
    "frontend-dist": (1, ["backend/internal/web/dist/"]),
    "toolchain-frontend": (90, ["frontend-toolchain.json"]),
    "toolchain-binary-${{ matrix.goos }}-${{ matrix.goarch }}": (90, ["binary-toolchain.json"]),
    "release-evidence-binary-${{ matrix.goos }}-${{ matrix.goarch }}": (90, ["binary-evidence/"]),
    "release-binary-${{ matrix.goos }}-${{ matrix.goarch }}": (1, ["matrix-output/"]),
    "toolchain-publication": (90, ["publication-toolchain.json"]),
    "personal-publication": (90, ["publication.json"]),
    "release-supply-chain-${{ github.run_id }}-${{ github.run_attempt }}": (90, ["release-evidence/"]),
    "release-dry-run-oci-${{ github.run_id }}-${{ github.run_attempt }}": (1, ["${{ runner.temp }}/sub2api-*.oci.tar"]),
    "release-dry-run-report": (3, ["dist/checksums.txt", "dist/artifacts.json", "dist/metadata.json", "publication-toolchain.json"]),
    "security-toolchain": (30, ["${{ runner.temp }}/security-toolchain.json"]),
    "toolchain-review": (35, ["toolchain-review.json"]),
}
RELEASE_GROUP = "release-${{ github.repository }}-${{ github.event_name == 'workflow_dispatch' && inputs.dry_run == true && 'dry-run' || 'publication' }}"


def load(name):
    # BaseLoader 保留 on 键，避免 YAML 1.1 把它解释成布尔值。
    return yaml.load((WORKFLOWS / name).read_text(), Loader=yaml.BaseLoader)


def policy_errors(name, workflow):
    errors = []
    if workflow.get("permissions") != READ:
        errors.append("顶层权限必须仅 contents: read")
    events = workflow.get("on", {})
    pr_workflow = "pull_request" in events or "pull_request_target" in events
    if pr_workflow and re.search(r"\bsecrets\b", re.sub(r"secrets\.GITHUB_TOKEN\b", "", str(workflow))):
        errors.append("PR 禁止读取生产 secrets 或传递 secrets 上下文")
    for job_id, job in workflow["jobs"].items():
        key = (name, job_id)
        permissions = job.get("permissions", workflow.get("permissions"))
        if permissions != JOB_PERMISSIONS.get(key, READ):
            errors.append(f"{job_id}：权限超出实际用途")
        if pr_workflow:
            if job.get("secrets"):
                errors.append(f"{job_id}：PR 禁止向复用工作流传 secrets")
            if "pull_request_target" in events:
                if "github.repository == 'Wei-Shaw/sub2api'" not in job.get("if", ""):
                    errors.append(f"{job_id}：上游 CLA 必须在本 fork 禁用")
                if any(s.get("run") or s.get("uses", "").startswith("actions/checkout@") for s in job.get("steps", [])):
                    errors.append(f"{job_id}：CLA 禁止检出或执行 PR 代码")
        for step in job.get("steps", []):
            action = step.get("uses", "").split("@")[0]
            settings = step.get("with", {})
            if action == "actions/checkout":
                expected = "true" if key == ("sync-upstream.yml", "sync") else "false"
                if settings.get("persist-credentials") != expected:
                    errors.append(f"{job_id}：checkout 凭据仅允许同步 Git push 持久化")
            if action == "actions/upload-artifact":
                policy = ARTIFACTS.get(settings.get("name"))
                paths = settings.get("path", "").splitlines()
                if policy is None or ([p for p in paths if not p.startswith("!")] != policy[1] or settings.get("retention-days") != str(policy[0])):
                    errors.append(f"{job_id}：artifact 路径或保留期未获授权")
                if not EXCLUSIONS.issubset(paths) or settings.get("include-hidden-files") != "false":
                    errors.append(f"{job_id}：artifact 缺少敏感文件排除")
    if name == "release.yml":
        if workflow["concurrency"] != {"group": RELEASE_GROUP, "cancel-in-progress": "false"}:
            errors.append("正式发布和 dry-run 必须隔离并发，禁止取消运行中发布")
        if set(events) != {"push", "workflow_dispatch"} or events["push"]["tags"] != ["v*-klno.*-tps.*"]:
            errors.append("Release 只能由正式标签或维护者手动触发")
    if name == "personal-release.yml":
        gate = workflow["jobs"]["prepare"].get("if", "")
        for required in ("github.event.workflow_run.head_repository.full_name == github.repository", "github.event.workflow_run.head_branch == 'personal'", "github.event.workflow_run.event == 'push'", "github.event.workflow_run.event == 'workflow_dispatch'"):
            if required not in gate:
                errors.append("Personal release 必须拒绝 fork/PR workflow_run")
        if set(events) != {"workflow_run", "workflow_dispatch"} or workflow["concurrency"].get("cancel-in-progress") != "false":
            errors.append("Personal release 禁止 PR 触发或取消运行中发布")
    if name == "sync-upstream.yml":
        if set(events) != {"schedule", "workflow_dispatch"} or "github.ref == 'refs/heads/main'" not in workflow["jobs"]["sync"].get("if", ""):
            errors.append("同步写权限只能用于 main 调度")
    return errors


class WorkflowSecurityTests(unittest.TestCase):
    def test_repository_workflows_follow_policy(self):
        for path in sorted(WORKFLOWS.glob("*.yml")):
            with self.subTest(workflow=path.name):
                self.assertEqual(policy_errors(path.name, load(path.name)), [])

    def test_write_permissions_in_test_job_are_rejected(self):
        flow = load("backend-ci.yml")
        flow["jobs"]["test"]["permissions"] = {"contents": "write", "packages": "write"}
        self.assertIn("test：权限超出实际用途", policy_errors("backend-ci.yml", flow))

    def test_checkout_default_and_credential_persistence_are_rejected(self):
        for value in (None, "true"):
            flow = load("backend-ci.yml")
            settings = flow["jobs"]["test"]["steps"][0]["with"]
            settings.pop("persist-credentials", None)
            if value:
                settings["persist-credentials"] = value
            self.assertIn("test：checkout 凭据仅允许同步 Git push 持久化", policy_errors("backend-ci.yml", flow))

    def test_pr_production_secret_and_inheritance_are_rejected(self):
        for mutation in ({"secrets": "inherit"}, {"env": {"TOKEN": "${{ secrets.DEPLOY_TOKEN }}"}}):
            flow = load("personal-ci.yml")
            flow["jobs"]["existing-ci"].update(mutation)
            self.assertTrue(any("PR 禁止" in e for e in policy_errors("personal-ci.yml", flow)))
        for expression in ("${{ secrets.DEPLOY_TOKEN }}", "${{ secrets['DEPLOY_TOKEN'] }}", "${{ toJSON(secrets) }}"):
            flow = load("personal-ci.yml")
            flow["env"]["TOKEN"] = expression
            self.assertTrue(any("PR 禁止" in e for e in policy_errors("personal-ci.yml", flow)))

    def test_target_pr_code_execution_is_rejected(self):
        for step in ({"run": "python pr.py"}, {"uses": "actions/checkout@sha"}):
            flow = load("cla.yml")
            flow["jobs"]["cla-check"]["steps"].append(step)
            self.assertIn("cla-check：CLA 禁止检出或执行 PR 代码", policy_errors("cla.yml", flow))

    def test_artifact_sensitive_paths_retention_and_hidden_files_are_rejected(self):
        original = load("release.yml")
        for change in ({"path": "."}, {"path": ".env\nbackup.dump\nserver.log"}, {"retention-days": "90"}, {"include-hidden-files": "true"}):
            flow = copy.deepcopy(original)
            step = next(s for s in flow["jobs"]["prepare"]["steps"] if s.get("uses", "").startswith("actions/upload-artifact@"))
            step["with"].update(change)
            self.assertTrue(any("artifact" in e for e in policy_errors("release.yml", flow)))

    def test_shared_or_cancelling_publication_concurrency_is_rejected(self):
        for change in ({"group": "release-${{ github.repository }}"}, {"cancel-in-progress": "true"}):
            flow = load("release.yml")
            flow["concurrency"].update(change)
            self.assertTrue(any("并发" in e for e in policy_errors("release.yml", flow)))

    def test_fork_release_gate_and_pr_trigger_regressions_are_rejected(self):
        flow = load("personal-release.yml")
        flow["jobs"]["prepare"]["if"] = "github.event.workflow_run.conclusion == 'success'"
        self.assertTrue(any("fork/PR" in e for e in policy_errors("personal-release.yml", flow)))
        flow = load("release.yml")
        flow["on"]["pull_request"] = {}
        self.assertTrue(any("只能" in e for e in policy_errors("release.yml", flow)))


if __name__ == "__main__":
    unittest.main()
