"""验证自动 Quick、手动 Full、路径过滤和失败传播。"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import ci_policy as policy
from sync import REPOSITORY, SyncError, git

A, B = "a" * 40, "b" * 40


class PathPolicyTests(unittest.TestCase):
    def test_documentation_does_not_hide_runtime_markdown_or_unknown_files(self):
        flags = policy.classify(["README_CN.md", "docs/ci/CI-000.md", "NOTICE"])
        self.assertTrue(flags["docs"])
        self.assertFalse(flags["backend"] or flags["frontend"] or flags["high_risk"])
        for path in ("backend/resources/prompt.md", "deploy/README.md", "Dockerfile.goreleaser", ".github/audit-exceptions.yml"):
            with self.subTest(path=path):
                self.assertTrue(policy.classify(["docs/foo.md", path])["high_risk"])

    def test_sensitive_boundaries_remain_marked_high_risk(self):
        for path in ("backend/migrations/999.sql", "backend/internal/repository/users.go",
                     "backend/internal/service/billing.go", "backend/internal/service/openai_ws.go",
                     "backend/internal/server/middleware/auth.go", "frontend/src/api/client.ts",
                     "frontend/src/views/user/PaymentView.vue", ".github/workflows/release.yml",
                     "deploy/personal/deploy-hostdzire.sh", "frontend/pnpm-lock.yaml", "backend/go.sum"):
            with self.subTest(path=path):
                self.assertTrue(policy.classify([path])["high_risk"])
        self.assertFalse(policy.classify(["frontend/src/components/common/HelpTooltip.vue"])["high_risk"])

    def test_high_risk_pr_keeps_quick_feedback_without_full_gate(self):
        for path in ("backend/internal/service/user_service.go", "backend/internal/service/account_service.go",
                     "backend/internal/service/group_service.go", "backend/internal/service/new_helper.go",
                     "frontend/src/views/admin/UsersView.vue", "frontend/src/components/account/credentialsBuilder.ts",
                     "frontend/src/components/common/NewComponent.vue"):
            with self.subTest(path=path):
                value = self.plan("synchronize", [path])
                self.assertTrue(value["high_risk"])
                self.assertFalse(value["require_full"])

    def test_dependency_scopes_are_distinct(self):
        go = policy.classify(["backend/go.mod"])
        front = policy.classify(["frontend/pnpm-lock.yaml"])
        self.assertTrue(go["go_dependencies"])
        self.assertFalse(go["frontend_dependencies"])
        self.assertTrue(front["frontend_dependencies"])
        self.assertFalse(front["go_dependencies"])

    def plan(self, action, paths, event_name="pull_request", requested="quick"):
        event = {"action": action, "before": B, "label": {"name": "full-ci"},
                 "pull_request": {"base": {"sha": B}}} if event_name == "pull_request" else {"before": B}
        with patch.object(policy, "changed_paths", return_value=paths):
            return policy.plan(event, event_name, A, requested, True)

    def test_pr_ready_labels_and_push_never_automatically_request_full(self):
        for event_name, action in (("pull_request", "synchronize"), ("pull_request", "ready_for_review"),
                                   ("pull_request", "labeled"), ("push", "")):
            with self.subTest(event=event_name, action=action):
                result = self.plan(action, ["backend/internal/service/billing.go"], event_name, "full")
                self.assertEqual(result["mode"], "quick")
                self.assertFalse(result["require_full"])

    def test_manual_full_runs_all_required_checks(self):
        result = self.plan("", [], "workflow_dispatch", "full")
        self.assertEqual(result["mode"], "full")
        self.assertTrue(result["require_full"] and result["scan_go"] and result["scan_frontend"])

    def test_quick_dependency_audits_keep_their_scope(self):
        result = self.plan("synchronize", ["backend/go.mod"])
        self.assertTrue(result["scan_go"])
        self.assertFalse(result["scan_frontend"] or result["require_full"])

    def test_sync_automatic_pr_only_checks_binding_without_full_certification(self):
        for ref in ("codex/sync-klno-v1", "codex/verify-personal-ci-v1"):
            event = {"pull_request": {"base": {"sha": B}, "head": {"ref": ref}}}
            with self.subTest(ref=ref), patch.object(policy, "changed_paths", return_value=["backend/go.mod"]):
                value = policy.plan(event, "pull_request", A, personal=True)
                self.assertTrue(value["check_only"])
                self.assertFalse(value["require_full"])
                env = {"CI_PLAN": json.dumps(value), "RESULTS": json.dumps({
                    "binding": {"result": "success"}, "full-ci-certified": {"result": "skipped"},
                    "existing-ci": {"result": "skipped"}, "existing-security": {"result": "skipped"}})}
                with patch.dict(os.environ, env), patch("sys.argv", ["ci_policy", "aggregate", "--personal"]):
                    policy.main()

    def test_standalone_manual_quick_requires_a_real_diff(self):
        with patch.dict(os.environ, {"BASE_SHA": A}), self.assertRaises(SyncError):
            policy.plan({}, "workflow_dispatch", A)


class ResultTests(unittest.TestCase):
    def test_aggregate_allows_only_unselected_skips(self):
        policy.require_results({"docs": {"result": "success"}, "test": {"result": "skipped"}}, {"docs"})
        for result in ("skipped", "failure", "cancelled", "missing"):
            with self.subTest(result=result), self.assertRaises(SyncError):
                policy.require_results({"docs": {"result": result}}, {"docs"})
        with self.assertRaises(SyncError):
            policy.require_results({"docs": {"result": "success"}, "test": {"result": "failure"}}, {"docs"})

    def test_quick_and_automatic_events_cannot_certify_full(self):
        for mode, event in (("quick", "workflow_dispatch"), ("full", "push"), ("full", "pull_request")):
            env = {"CI_PLAN": json.dumps({"mode": mode}), "RESULTS": "{}", "GITHUB_EVENT_NAME": event}
            with self.subTest(mode=mode, event=event), patch.dict(os.environ, env), \
                 patch("sys.argv", ["ci_policy", "certify"]), self.assertRaisesRegex(SyncError, "手动 Full"):
                policy.main()

    def test_manual_full_certification_propagates_selected_job_failures(self):
        success = {job: {"result": "success"} for job in
                   ("binding", "existing-ci", "existing-security", "sync-contracts")}
        for state in ("success", "failure", "cancelled", "skipped", "missing"):
            results = {**success, "existing-ci": {"result": state}}
            env = {"CI_PLAN": json.dumps({"mode": "full"}), "RESULTS": json.dumps(results),
                   "GITHUB_EVENT_NAME": "workflow_dispatch"}
            with self.subTest(state=state), patch.dict(os.environ, env), patch("sys.argv", ["ci_policy", "certify"]):
                if state == "success":
                    policy.main()
                else:
                    with self.assertRaises(SyncError):
                        policy.main()


class GitEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.temp = tempfile.TemporaryDirectory()
        os.chdir(self.temp.name)
        git("init", "-b", "personal")
        git("config", "user.name", "CI")
        git("config", "user.email", "ci@example.invalid")
        Path("base").write_text("base")
        self.base = self.commit()

    def tearDown(self):
        os.chdir(self.previous)
        self.temp.cleanup()

    def commit(self):
        git("add", ".")
        git("commit", "-qm", "测试")
        return git("rev-parse", "HEAD")

    def test_rename_preserves_sensitive_deleted_path(self):
        Path("backend/migrations").mkdir(parents=True)
        Path("backend/migrations/999.sql").write_text("SELECT 1;")
        base = self.commit()
        git("mv", "backend/migrations/999.sql", "documentation.md")
        source = self.commit()
        paths = policy.changed_paths(base, source)
        self.assertIn("backend/migrations/999.sql", paths)
        self.assertTrue(policy.classify(paths)["high_risk"])



if __name__ == "__main__":
    unittest.main()
