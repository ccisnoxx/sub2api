"""在真实临时 Git 历史和受控 REST 响应上验证 CI 可观察合同。"""

import copy
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import ci_policy as policy
from sync import SyncError, UPSTREAM, git

NOW = datetime(2026, 10, 9, 12, tzinfo=timezone.utc)


def results(plan):
    return {name: {"result": "success" if name in ("binding", "changes") or any(
        plan[key] for key in policy.GROUPS.get(name, ())) else "skipped"}
        for name in ("binding", "changes", *policy.GROUPS)}


def jobs(plan):
    names = ["binding", "changes", "personal-ready"] + [name for name, flag in policy.JOB_FLAGS.items() if plan[flag]]
    return [{"name": name, "status": "completed", "conclusion": "success", "completed_at": NOW.isoformat()}
            for name in names]


class GitPolicyTests(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.directory = tempfile.TemporaryDirectory()
        os.chdir(self.directory.name)
        git("init", "-b", "main")
        git("config", "user.name", "CI contract")
        git("config", "user.email", "ci@example.invalid")
        Path("README.md").write_text("base\n")
        self.upstream = self.commit("upstream")
        self.write(policy.SOURCE, json.dumps({"upstream_repository": UPSTREAM,
            "upstream_tag": "v0.2.14-klno.3", "upstream_sha": self.upstream}))
        for path in policy.TRUST_CONTROLS:
            self.write(path, "name: Test\non: push\njobs:\n  frontend:\n    runs-on: ubuntu-latest\n" if path.endswith(".yml") else "# control\n")
        self.write("frontend/src/App.vue", "base\n")
        self.write("backend/internal/handler.go", "base\n")
        self.write("backend/go.mod", "module example.invalid\ngo 1.26.4\n")
        self.write("backend/go.sum", "base\n")
        self.write("frontend/package.json", '{"dependencies":{}}\n')
        self.write("frontend/pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
        self.base = self.commit("personal base")

    def tearDown(self):
        os.chdir(self.previous)
        self.directory.cleanup()

    def write(self, path, content):
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        Path(path).write_text(content)

    def commit(self, message):
        git("add", ".")
        git("commit", "-m", message)
        return git("rev-parse", "HEAD")

    def candidate(self, path="frontend/src/App.vue", content="changed\n"):
        self.write(path, content)
        return self.commit("candidate")

    def test_ui_only_and_documentation_only_select_distinct_checks(self):
        sha = self.candidate()
        plan = policy.make_plan(sha, self.base, "5")
        self.assertTrue(plan["run_frontend"])
        self.assertTrue(plan["run_tps"])
        self.assertFalse(plan["run_backend"])
        self.assertFalse(plan["run_backend_security"])
        self.assertFalse(plan["run_frontend_security"])
        git("switch", "--detach", self.base)
        sha = self.candidate("docs/guide.md")
        self.assertFalse(any(policy.make_plan(sha, self.base, "5")[key] for key in policy.FLAGS))

    def test_dependency_scan_follows_affected_ecosystem(self):
        for path, selected, other in (("frontend/pnpm-lock.yaml", "run_frontend_security", "run_backend_security"),
                                      ("backend/go.sum", "run_backend_security", "run_frontend_security")):
            with self.subTest(path=path):
                git("switch", "--detach", self.base)
                sha = self.candidate(path)
                plan = policy.make_plan(sha, self.base, "5")
                self.assertTrue(plan["dependencies_changed"])
                self.assertTrue(plan[selected])
                self.assertFalse(plan[other])

    def test_upstream_source_change_selects_every_contract(self):
        git("switch", "-c", "upstream", self.upstream)
        upstream = self.candidate("docs/upstream.md")
        git("switch", "--detach", self.base)
        git("merge", "--no-ff", "--no-edit", upstream)
        self.write(policy.SOURCE, json.dumps({"upstream_repository": UPSTREAM,
            "upstream_tag": "v0.2.14-klno.4", "upstream_sha": upstream}))
        sha = self.commit("record source")
        plan = policy.make_plan(sha, self.base, "5")
        self.assertTrue(plan["wide_merge_changed"])
        self.assertTrue(all(plan[key] for key in policy.FLAGS))

    def test_rename_both_ends_and_delete_are_counted(self):
        Path("docs").mkdir()
        git("mv", "frontend/src/App.vue", "docs/moved.md")
        git("rm", "backend/internal/handler.go")
        sha = self.commit("rename and delete")
        plan = policy.make_plan(sha, self.base, "5")
        self.assertIn("frontend/src/App.vue", plan["paths"])
        self.assertIn("docs/moved.md", plan["paths"])
        self.assertTrue(plan["run_frontend"])
        self.assertTrue(plan["run_backend"])

    def test_unknown_build_path_is_not_documentation(self):
        sha = self.candidate("build/custom.json")
        plan = policy.make_plan(sha, self.base, "5")
        self.assertTrue(plan["run_frontend"])
        self.assertTrue(plan["run_backend"])
        self.assertFalse(plan["run_frontend_security"])

    def test_workflow_local_job_change_selects_its_real_impact(self):
        path = ".github/workflows/backend-ci.yml"
        sha = self.candidate(path, "name: Test\non: push\njobs:\n  frontend:\n    runs-on: ubuntu-24.04\n")
        plan = policy.make_plan(sha, self.base, "5")
        self.assertTrue(plan["run_frontend"])
        self.assertTrue(plan["run_sync_contracts"])
        self.assertFalse(plan["run_backend"])
        self.assertFalse(plan["run_frontend_security"])

    def test_gate_source_change_runs_both_contract_suites(self):
        sha = self.candidate(".github/personal-sync/ci_policy.py")
        plan = policy.make_plan(sha, self.base, "5")
        self.assertTrue(plan["run_sync_contracts"])
        self.assertTrue(plan["run_release_helpers"])
        self.assertFalse(plan["run_backend"])

    def test_personal_deployment_contracts_use_helper_job(self):
        for path in ("deploy/personal/deploy_hostdzire.py", ".github/workflows/personal-deploy-ci.yml"):
            git("switch", "--detach", self.base)
            plan = policy.make_plan(self.candidate(path), self.base, "5")
            self.assertTrue(plan["deploy_changed"])
            self.assertTrue(plan["run_release_helpers"])
            self.assertFalse(plan["run_shell"])

    def test_failed_cancelled_or_skipped_selected_results_rejected(self):
        plan = policy.make_plan(self.candidate(), self.base, "5")
        for value in ("failure", "cancelled", "skipped"):
            data = results(plan)
            data["existing-ci"]["result"] = value
            with self.subTest(value=value), self.assertRaisesRegex(SyncError, "必要检查"):
                policy.validate_results(plan, data)
        policy.validate_results(plan, results(plan))
        data = results(plan)
        data["existing-security"]["result"] = "cancelled"
        with self.assertRaises(SyncError):
            policy.validate_results(plan, data)

    def test_missing_skipped_or_failed_actual_jobs_cannot_be_self_attested(self):
        plan = policy.make_plan(self.candidate(), self.base, "5")
        for value in ("missing", "skipped", "failure"):
            data = jobs(plan)
            if value == "missing":
                data = [job for job in data if job["name"] != "existing-ci / frontend"]
            else:
                next(job for job in data if job["name"] == "existing-ci / frontend")["conclusion"] = value
            with self.subTest(value=value), self.assertRaises(SyncError):
                policy.validate_jobs(plan, data)

    def fixture(self, path="frontend/src/App.vue", event="workflow_dispatch"):
        candidate = self.candidate(path)
        plan = policy.make_plan(candidate, self.base, "5")
        merge = git("commit-tree", policy.tree(candidate), "-p", self.base, "-p", candidate, "-m", "merge PR 5")
        self.pr = {"number": 5, "state": "closed", "merged_at": NOW.isoformat(), "merge_commit_sha": merge,
                   "base": {"ref": "personal", "sha": self.base, "repo": {"full_name": policy.REPOSITORY}},
                   "head": {"ref": "codex/test", "sha": candidate, "repo": {"full_name": policy.REPOSITORY}}}
        self.run = {"id": 17, "run_attempt": 1, "workflow_id": 42, "path": policy.WORKFLOW,
                    "repository": {"full_name": policy.REPOSITORY}, "head_repository": {"full_name": policy.REPOSITORY},
                    "head_sha": candidate, "head_branch": "codex/test", "event": event,
                    "status": "completed", "conclusion": "success", "run_started_at": (NOW - timedelta(minutes=10)).isoformat(),
                    "created_at": (NOW - timedelta(minutes=10)).isoformat(),
                    "updated_at": NOW.isoformat(), "display_title": f"Personal CI | {candidate} | base {self.base}",
                    "pull_requests": [{"number": 5, "head": {"sha": candidate}, "base": {"sha": self.base}}],
                    "referenced_workflows": [{"path": policy.REPOSITORY + "/.github/workflows/backend-ci.yml@refs/heads/codex/test",
                                               "ref": "refs/heads/codex/test" if event == "workflow_dispatch" else "refs/pull/5/merge",
                                               "sha": candidate if event == "workflow_dispatch" else merge}]}
        if plan["run_frontend_security"]:
            self.run["referenced_workflows"].append({"path": policy.REPOSITORY + "/.github/workflows/security-scan.yml@refs/heads/codex/test",
                                                       "sha": candidate})
        self.actual_jobs = jobs(plan)
        self.evidence = {"schema": policy.SCHEMA, "repository": policy.REPOSITORY, "run_id": 17, "run_attempt": 1,
                         "workflow_sha": candidate if event == "workflow_dispatch" else merge,
                         "generated_at": NOW.isoformat(), "plan": plan, "results": results(plan),
                         "jobs": policy.validate_jobs(plan, self.actual_jobs)}
        self.merge = merge
        return policy.make_plan(merge, merge)

    def api(self, path):
        if path == "actions/workflows/personal-ci.yml":
            return {"id": 42}
        if path == "actions/runs/17":
            return self.run
        if path == "git/ref/heads/main":
            return {"object": {"sha": self.base}}
        if path.startswith("git/commits/"):
            sha = path.rsplit("/", 1)[1]
            return {"sha": sha, "tree": {"sha": policy.tree(sha)},
                    "parents": [{"sha": parent} for parent in policy.parents(sha)]}
        raise AssertionError(path)

    def pages(self, path, key):
        if path.startswith("commits/"):
            return [self.pr]
        if "/attempts/" in path:
            return self.actual_jobs
        if path.startswith("actions/workflows/"):
            return [self.run]
        raise AssertionError(path)

    def mocked_remote(self):
        return (patch.object(policy, "api", side_effect=self.api),
                patch.object(policy, "pages", side_effect=self.pages),
                patch.object(policy, "read_artifact", side_effect=lambda *args: self.evidence))

    def test_same_tree_merge_reuses_actual_successful_candidate_ci(self):
        plan = self.fixture()
        one, two, three = self.mocked_remote()
        with one, two, three:
            plan = policy.find_reuse(plan, NOW)
            self.assertTrue(plan["reused"])
            self.assertFalse(any(plan[key] for key in policy.FLAGS))
            policy.validate_plan(plan)
            self.assertEqual(policy.verify_reuse(plan, 42, NOW), self.evidence)

    def test_automatic_pr_reuse_requires_unchanged_trusted_controls(self):
        plan = self.fixture(event="pull_request")
        self.run["pull_requests"] = []  # 仓库真实 REST：合并后关联列表可能为空。
        one, two, three = self.mocked_remote()
        with one, two, three:
            self.assertTrue(policy.find_reuse(plan, NOW)["reused"])
        git("switch", "--detach", self.base)
        plan = self.fixture(path=".github/personal-sync/ci_policy.py", event="pull_request")
        one, two, three = self.mocked_remote()
        with one, two, three:
            self.assertFalse(policy.find_reuse(plan, NOW)["reused"])
            self.assertIn("可信 main", plan["reuse_reason"])

    def test_different_tree_single_parent_and_nonancestor_base_do_not_reuse(self):
        plan = self.fixture()
        different = git("commit-tree", policy.tree(self.base), "-p", self.base, "-p", self.run["head_sha"], "-m", "resolved")
        single = git("commit-tree", policy.tree(self.run["head_sha"]), "-p", self.base, "-m", "squash")
        unrelated = git("commit-tree", policy.tree(self.base), "-p", self.upstream, "-m", "unrelated base")
        bad_base = git("commit-tree", policy.tree(self.run["head_sha"]), "-p", unrelated,
                       "-p", self.run["head_sha"], "-m", "wrong parents")
        for sha in (different, single, bad_base):
            with self.subTest(sha=sha):
                self.assertIsNone(policy.merge_candidate(policy.make_plan(sha, sha)))
        with self.assertRaisesRegex(SyncError, "祖先"):
            policy.make_plan(self.run["head_sha"], unrelated, "5")

    def test_foreign_failed_wrong_workflow_runs_are_not_reused(self):
        for field, value in (("conclusion", "failure"), ("conclusion", "cancelled"), ("workflow_id", 99),
                             ("head_repository", {"full_name": "other/repo"}), ("path", ".github/workflows/fake.yml")):
            git("switch", "--detach", self.base)
            plan = self.fixture()
            self.run[field] = value
            one, two, three = self.mocked_remote()
            with self.subTest(field=field, value=value), one, two, three:
                self.assertFalse(policy.find_reuse(plan, NOW)["reused"])

    def test_attempt_plan_jobs_workflow_inputs_and_merge_pr_must_match(self):
        for mutation in ("attempt", "digest", "job", "workflow", "pr", "base"):
            git("switch", "--detach", self.base)
            plan = self.fixture()
            if mutation == "attempt":
                self.evidence["run_attempt"] = 2
            elif mutation == "digest":
                self.evidence["plan"]["bindings"]["input_digest"] = "forged"
            elif mutation == "job":
                self.evidence["jobs"]["existing-ci / frontend"] = "skipped"
            elif mutation == "workflow":
                self.evidence["workflow_sha"] = self.base
            elif mutation == "base":
                self.evidence["plan"]["base_sha"] = self.upstream
            else:
                self.pr["merge_commit_sha"] = self.base
            one, two, three = self.mocked_remote()
            with self.subTest(mutation=mutation), one, two, three:
                if mutation == "pr":
                    self.assertFalse(policy.find_reuse(plan, NOW)["reused"])
                else:
                    with self.assertRaises(SyncError):
                        policy.find_reuse(plan, NOW)

    def test_missing_evidence_falls_back_but_api_errors_fail(self):
        plan = self.fixture()
        with patch.object(policy, "api", side_effect=self.api), patch.object(policy, "pages", side_effect=self.pages), \
             patch.object(policy, "read_artifact", return_value=None):
            self.assertFalse(policy.find_reuse(plan, NOW)["reused"])
        with patch.object(policy, "api", side_effect=SyncError("API 权限错误")), \
             patch.object(policy, "pages", side_effect=self.pages):
            with self.assertRaisesRegex(SyncError, "权限"):
                policy.find_reuse(plan, NOW)

    def test_security_evidence_has_24h_limit_functional_evidence_does_not(self):
        plan = self.fixture("frontend/pnpm-lock.yaml")
        one, two, three = self.mocked_remote()
        with one, two, three:
            self.assertFalse(policy.find_reuse(plan, NOW + timedelta(hours=25))["reused"])
        git("switch", "--detach", self.base)
        plan = self.fixture()
        one, two, three = self.mocked_remote()
        with one, two, three:
            self.assertTrue(policy.find_reuse(plan, NOW + timedelta(days=20))["reused"])

    def test_failed_job_rerun_reuses_successful_scan_within_original_run(self):
        plan = self.fixture("frontend/pnpm-lock.yaml")
        self.run.update(run_attempt=2, created_at=(NOW - timedelta(minutes=30)).isoformat(),
                        run_started_at=(NOW - timedelta(minutes=1)).isoformat())
        self.evidence["run_attempt"] = 2
        scan = next(job for job in self.actual_jobs if job["name"] == "existing-security / frontend-security")
        scan["completed_at"] = (NOW - timedelta(minutes=5)).isoformat()
        one, two, three = self.mocked_remote()
        with one, two, three:
            self.assertTrue(policy.find_reuse(copy.deepcopy(plan), NOW)["reused"])
            self.assertFalse(policy.find_reuse(copy.deepcopy(plan), NOW + timedelta(hours=25))["reused"])
            scan["completed_at"] = (NOW - timedelta(minutes=31)).isoformat()
            with self.assertRaisesRegex(SyncError, "安全扫描时间"):
                policy.find_reuse(copy.deepcopy(plan), NOW)

    def test_timestamp_outside_real_run_rejected(self):
        self.fixture()
        self.evidence["generated_at"] = (NOW + timedelta(minutes=1)).isoformat()
        one, two, three = self.mocked_remote()
        with one, two, three, self.assertRaisesRegex(SyncError, "时间"):
            policy.validate_evidence(self.evidence, self.run, 42, NOW)

    def test_final_lightweight_ci_verifies_chain_and_pr_run_cannot_publish(self):
        plan = self.fixture()
        candidate_run, candidate_evidence, candidate_jobs = copy.deepcopy(self.run), copy.deepcopy(self.evidence), self.actual_jobs
        one, two, three = self.mocked_remote()
        with one, two, three:
            plan = policy.find_reuse(plan, NOW)
        final_run = {**candidate_run, "id": 18, "head_sha": self.merge, "head_branch": "personal", "event": "push",
                     "display_title": f"Personal CI | {self.merge} | base {self.merge}", "referenced_workflows": []}
        final_jobs = jobs(plan)
        final_evidence = {**candidate_evidence, "run_id": 18, "workflow_sha": self.merge, "plan": plan,
                          "results": results(plan), "jobs": policy.validate_jobs(plan, final_jobs)}
        def pages(path, key):
            if "runs/18/" in path:
                return final_jobs
            return self.pages(path, key)
        def artifact(run, *args):
            return final_evidence if run["id"] == 18 else candidate_evidence
        with patch.object(policy, "api", side_effect=self.api), patch.object(policy, "pages", side_effect=pages), \
             patch.object(policy, "read_artifact", side_effect=artifact):
            self.assertEqual(policy.verify_release_evidence(self.merge, final_run, 42, NOW), final_evidence)
            with self.assertRaisesRegex(SyncError, "最终 personal"):
                policy.verify_release_evidence(candidate_run["head_sha"], candidate_run, 42, NOW)
            self.run["run_attempt"] = 2
            with self.assertRaisesRegex(SyncError, "attempt"):
                policy.verify_release_evidence(self.merge, final_run, 42, NOW)

    def test_cli_plan_writes_single_line_outputs(self):
        sha = self.candidate()
        environment = {"GITHUB_REPOSITORY": policy.REPOSITORY, "CANDIDATE_SHA": sha, "BASE_SHA": self.base,
                       "PR_NUMBER": "5", "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REF": "refs/heads/codex/test",
                       "GITHUB_OUTPUT": str(Path.cwd() / "outputs")}
        with patch.dict(os.environ, environment), patch("sys.argv", ["ci_policy.py", "plan"]), patch("builtins.print"):
            policy.main()
        lines = Path("outputs").read_text().splitlines()
        outputs = dict(line.split("=", 1) for line in lines)
        self.assertEqual(outputs["run_frontend"], "true")
        self.assertEqual(outputs["run_backend"], "false")
        self.assertEqual(json.loads(outputs["plan_json"])["candidate_sha"], sha)

    def test_attest_checks_current_attempt_and_actual_required_jobs(self):
        self.fixture()
        plan = self.evidence["plan"]
        ready = next(job for job in self.actual_jobs if job["name"] == "personal-ready")
        ready.update(status="in_progress", conclusion=None)
        environment = {"GITHUB_RUN_ID": "17", "GITHUB_RUN_ATTEMPT": "1", "GITHUB_EVENT_NAME": "workflow_dispatch"}
        one, two, three = self.mocked_remote()
        with patch.dict(os.environ, environment), one, two, three:
            evidence = policy.attest(plan, results(plan), self.run["head_sha"])
            self.assertEqual(evidence["jobs"]["existing-ci / frontend"], "success")
            self.run["run_attempt"] = 2
            with self.assertRaisesRegex(SyncError, "attempt"):
                policy.attest(plan, results(plan), self.run["head_sha"])


class ArtifactTests(unittest.TestCase):
    def fixture(self, entries=None):
        body = io.BytesIO()
        with zipfile.ZipFile(body, "w", zipfile.ZIP_DEFLATED) as archive:
            for name, value in entries or [(policy.EVIDENCE_FILE, '{"test": true}')]:
                archive.writestr(name, value)
        blob = body.getvalue()
        run = {"id": 17, "head_sha": "a" * 40, "run_started_at": (NOW - timedelta(minutes=10)).isoformat()}
        artifact = {"id": 23, "name": policy.ARTIFACT, "expired": False,
                    "expires_at": (NOW + timedelta(days=30)).isoformat(), "created_at": NOW.isoformat(),
                    "size_in_bytes": len(blob), "digest": "sha256:" + hashlib.sha256(blob).hexdigest(),
                    "workflow_run": {"id": 17, "head_sha": "a" * 40}}
        return run, artifact, blob

    def test_read_fixed_json_without_extracting_archive(self):
        run, artifact, blob = self.fixture()
        with patch.object(policy, "pages", return_value=[artifact]), patch.object(policy, "download_bytes", return_value=blob):
            self.assertEqual(policy.read_artifact(run, NOW), {"test": True})

    def test_duplicate_large_wrong_entry_bad_digest_and_old_attempt_rejected(self):
        for mutation in ("duplicate", "large", "entry", "digest", "attempt", "malformed"):
            run, artifact, blob = self.fixture([(policy.EVIDENCE_FILE, "invalid")]) if mutation == "malformed" else self.fixture()
            if mutation == "large":
                artifact["size_in_bytes"] = policy.MAX_ARTIFACT_BYTES + 1
            elif mutation == "entry":
                run, artifact, blob = self.fixture([("../ci-evidence.json", "{}")])
            elif mutation == "digest":
                artifact["digest"] = "sha256:wrong"
            elif mutation == "attempt":
                artifact["created_at"] = (NOW - timedelta(days=1)).isoformat()
            rows = [artifact, artifact] if mutation == "duplicate" else [artifact]
            with self.subTest(mutation=mutation), patch.object(policy, "pages", return_value=rows), \
                 patch.object(policy, "download_bytes", return_value=blob), self.assertRaises(SyncError):
                policy.read_artifact(run, NOW)

    def test_missing_or_expired_artifact_is_unavailable(self):
        run, artifact, _ = self.fixture()
        artifact["expired"] = True
        for rows in ([], [artifact]):
            with patch.object(policy, "pages", return_value=rows), patch.object(policy, "download_bytes") as download:
                self.assertIsNone(policy.read_artifact(run, NOW))
                download.assert_not_called()


if __name__ == "__main__":
    unittest.main()
