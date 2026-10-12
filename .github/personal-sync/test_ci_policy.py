"""验证路径过滤、候选证据、合并复用和跳过检查的真实边界。"""

import copy
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

    def test_sensitive_boundaries_require_full(self):
        for path in ("backend/migrations/999.sql", "backend/internal/repository/users.go",
                     "backend/internal/service/billing.go", "backend/internal/service/openai_ws.go",
                     "backend/internal/server/middleware/auth.go", "frontend/src/api/client.ts",
                     "frontend/src/views/user/PaymentView.vue", ".github/workflows/release.yml",
                     "deploy/personal/deploy-hostdzire.sh", "frontend/pnpm-lock.yaml", "backend/go.sum"):
            with self.subTest(path=path):
                self.assertTrue(policy.classify([path])["high_risk"])
        self.assertFalse(policy.classify(["frontend/src/components/common/HelpTooltip.vue"])["high_risk"])

    def test_actual_owners_and_new_unknown_runtime_paths_require_full(self):
        for path in ("backend/internal/service/user_service.go", "backend/internal/service/account_service.go",
                     "backend/internal/service/group_service.go", "backend/internal/service/new_helper.go",
                     "frontend/src/views/admin/UsersView.vue", "frontend/src/components/account/credentialsBuilder.ts",
                     "frontend/src/components/common/NewComponent.vue"):
            with self.subTest(path=path):
                self.assertTrue(self.plan("synchronize", [path])["require_full"])

    def test_dependency_scopes_are_distinct(self):
        go = policy.classify(["backend/go.mod"])
        front = policy.classify(["frontend/pnpm-lock.yaml"])
        self.assertTrue(go["go_dependencies"])
        self.assertFalse(go["frontend_dependencies"])
        self.assertTrue(front["frontend_dependencies"])
        self.assertFalse(front["go_dependencies"])

    def plan(self, action, paths, evidence="", event_name="pull_request"):
        event = {"action": action, "before": B, "label": {"name": "full-ci"},
                 "pull_request": {"base": {"sha": B}}} if event_name == "pull_request" else {"before": B}
        with patch.object(policy, "changed_paths", return_value=paths), \
             patch.object(policy, "full_evidence", return_value=evidence), \
             patch.object(policy, "merge_evidence", return_value=evidence):
            return policy.plan(event, event_name, A, "quick", True)

    def test_intermediate_high_risk_pr_is_quick_and_gate_requires_full(self):
        result = self.plan("synchronize", ["backend/internal/service/billing.go"])
        self.assertEqual(result["mode"], "quick")
        self.assertTrue(result["require_full"])
        self.assertFalse(result["scan_go"])

    def test_label_and_ready_request_full_once_then_reuse_success(self):
        for action in ("labeled", "ready_for_review"):
            self.assertEqual(self.plan(action, ["frontend/src/x.vue"])["mode"], "full")
            reused = self.plan(action, ["frontend/src/x.vue"], "12")
            self.assertEqual(reused["mode"], "quick")
            self.assertEqual(reused["evidence_run_id"], "12")

    def test_push_docs_quick_ordinary_full_and_merge_evidence_reused(self):
        self.assertEqual(self.plan("", ["README.md"], event_name="push")["mode"], "quick")
        self.assertEqual(self.plan("", ["frontend/src/x.vue"], event_name="push")["mode"], "full")
        reused = self.plan("", ["backend/go.mod"], "12", "push")
        self.assertEqual(reused["mode"], "quick")
        self.assertFalse(reused["scan_go"] or reused["scan_frontend"])

    def test_sync_automatic_pr_only_checks_explicit_evidence_and_never_skips_gate(self):
        for ref in ("codex/sync-klno-v1", "codex/verify-personal-ci-v1"):
            event = {"action": "labeled", "label": {"name": "bug"},
                     "pull_request": {"base": {"sha": B}, "head": {"ref": ref}}}
            with self.subTest(ref=ref), patch.object(policy, "changed_paths", return_value=["backend/go.mod"]), \
                 patch.object(policy, "full_evidence", return_value=""):
                value = policy.plan(event, "pull_request", A, personal=True)
                self.assertEqual(value["mode"], "quick")
                self.assertTrue(value["check_only"] and value["require_full"])
                env = {"CI_PLAN": json.dumps(value), "RESULTS": json.dumps({
                    "binding": {"result": "success"}, "full-ci-certified": {"result": "skipped"}})}
                with patch.dict(os.environ, env), patch("sys.argv", ["ci_policy", "aggregate", "--personal"]):
                    with self.assertRaises(SyncError):
                        policy.main()
                value["evidence_run_id"] = "12"
                env["CI_PLAN"] = json.dumps(value)
                env["RESULTS"] = json.dumps({"binding": {"result": "success"}, "full-ci-certified": {"result": "success"},
                                             "existing-ci": {"result": "skipped"}, "existing-security": {"result": "skipped"}})
                with patch.dict(os.environ, env), patch("sys.argv", ["ci_policy", "aggregate", "--personal"]):
                    policy.main()

    def test_unrelated_label_still_requires_full_for_permission_owner(self):
        event = {"action": "labeled", "label": {"name": "bug"}, "pull_request": {"base": {"sha": B}}}
        with patch.object(policy, "changed_paths", return_value=["backend/internal/service/user_service.go"]), \
             patch.object(policy, "full_evidence", return_value=""):
            value = policy.plan(event, "pull_request", A, personal=True)
            self.assertTrue(value["require_full"])
            self.assertFalse(value["check_only"])

    def test_standalone_manual_quick_requires_a_real_diff(self):
        with patch.dict(os.environ, {"BASE_SHA": A}), self.assertRaises(SyncError):
            policy.plan({}, "workflow_dispatch", A)


class EvidenceTests(unittest.TestCase):
    def test_incomplete_wrong_repository_base_sha_and_failed_jobs_are_rejected(self):
        original = {"id": 12, "workflow_id": 42, "path": ".github/workflows/personal-ci.yml",
                    "repository": {"full_name": REPOSITORY}, "head_repository": {"full_name": REPOSITORY},
                    "head_sha": A, "event": "workflow_dispatch", "status": "completed", "conclusion": "success",
                    "display_title": policy.ci_title(A, B, "full")}
        jobs = [{"name": name, "status": "completed", "conclusion": "success"} for name in policy.FULL_JOBS]
        for change in ({}, {"head_sha": B}, {"display_title": f"Personal CI | {A} | base {A}"},
                       {"head_repository": {"full_name": "foreign/repo"}}, {"conclusion": "cancelled"}):
            with self.subTest(change=change), patch.object(policy, "gh_json", return_value={"id": 42}), \
                 patch.object(policy, "pages", side_effect=[[{**original, **change}], jobs]):
                self.assertEqual(policy.full_evidence(A, B), "" if change else "12")
        for conclusion in ("skipped", "failure", "cancelled"):
            broken = copy.deepcopy(jobs)
            broken[0]["conclusion"] = conclusion
            with patch.object(policy, "gh_json", return_value={"id": 42}), \
                 patch.object(policy, "pages", side_effect=[[original], broken]):
                self.assertEqual(policy.full_evidence(A, B), "")

    def test_aggregate_allows_only_unselected_skips(self):
        policy.require_results({"docs": {"result": "success"}, "test": {"result": "skipped"}}, {"docs"})
        for result in ("skipped", "failure", "cancelled", "missing"):
            with self.subTest(result=result), self.assertRaises(SyncError):
                policy.require_results({"docs": {"result": result}}, {"docs"})
        with self.assertRaises(SyncError):
            policy.require_results({"docs": {"result": "success"}, "test": {"result": "failure"}}, {"docs"})

    def test_certification_cli_refuses_high_risk_quick_without_evidence(self):
        with tempfile.TemporaryDirectory() as directory:
            gh = Path(directory) / "gh"
            gh.write_text('#!/bin/sh\ncase "$*" in *"/runs?"*) echo \'{"workflow_runs":[]}\';; *) echo \'{"id":42}\';; esac\n')
            gh.chmod(0o755)
            env = {**os.environ, "PATH": directory + os.pathsep + os.environ["PATH"],
                   "CI_PLAN": json.dumps({"mode": "quick", "evidence_run_id": "", "source_sha": A, "diff_base": B}),
                   "RESULTS": "{}", "GITHUB_EVENT_NAME": "pull_request"}
            result = subprocess.run([os.sys.executable, str(Path(policy.__file__)), "certify"], env=env,
                                    text=True, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("full-ci", result.stderr)

    def test_certification_requeries_full_completed_during_quick(self):
        value = {"mode": "quick", "evidence_run_id": "", "source_sha": A, "diff_base": B}
        env = {"CI_PLAN": json.dumps(value), "RESULTS": "{}", "GITHUB_EVENT_NAME": "pull_request"}
        with patch.dict(os.environ, env), patch("sys.argv", ["ci_policy", "certify"]), \
             patch.object(policy, "full_evidence", return_value="12") as query:
            policy.main()
            query.assert_called_once_with(A, B)


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

    def test_only_unchanged_merge_tree_and_exact_before_reuses_full(self):
        git("switch", "-c", "candidate")
        Path("feature").write_text("tested")
        head = self.commit()
        git("switch", "personal")
        git("merge", "--no-ff", "-m", "merge", "candidate")
        merged = git("rev-parse", "HEAD")
        with patch.object(policy, "full_evidence", return_value="12") as evidence:
            self.assertEqual(policy.merge_evidence(merged, self.base), "12")
            evidence.assert_called_once_with(head, self.base)
        for before, source in ((head, merged), (self.base, head)):
            with patch.object(policy, "full_evidence") as evidence:
                self.assertEqual(policy.merge_evidence(source, before), "")
                evidence.assert_not_called()
        Path("feature").write_text("untested merge edit")
        tree_commit = self.commit()
        altered = git("commit-tree", git("rev-parse", f"{tree_commit}^{{tree}}"),
                      "-p", self.base, "-p", head, "-m", "改写合并内容")
        with patch.object(policy, "full_evidence") as evidence:
            self.assertEqual(policy.merge_evidence(altered, self.base), "")
            evidence.assert_not_called()


if __name__ == "__main__":
    unittest.main()
