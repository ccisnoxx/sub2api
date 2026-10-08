import argparse
import copy
import os
import unittest
from unittest.mock import patch

import personal_release as release

SHA = "a" * 40
BASE = "v0.2.14-klno.3"
TAG = BASE + "-tps.1"
RUN = {"id": 123, "workflow_id": 42, "path": ".github/workflows/personal-ci.yml",
       "head_repository": {"full_name": release.REPOSITORY}, "head_branch": "personal",
       "head_sha": SHA, "status": "completed", "conclusion": "success", "event": "push",
       "display_title": f"Personal CI | {SHA} | base {SHA}", "check_suite_id": 7}
SOURCE = {"upstream_tag": BASE, "upstream_sha": "b" * 40}


class PersonalReleaseTest(unittest.TestCase):
    def test_version_allocation_is_numeric_and_reuses_same_sha(self):
        existing = {BASE + "-tps.9": "c" * 40, BASE + "-tps.10": "d" * 40,
                    "v0.2.14-klno.5-tps.20": "e" * 40}
        self.assertEqual(release.allocate(BASE, SHA, existing), BASE + "-tps.11")
        existing[TAG] = SHA
        self.assertEqual(release.allocate(BASE, SHA, existing), TAG)
        existing[BASE + "-tps.2"] = SHA
        with self.assertRaises(release.SyncError):
            release.allocate(BASE, SHA, existing)

    def test_pr_wrong_sha_foreign_repository_failed_and_cancelled_ci_are_rejected(self):
        self.assertTrue(release.trusted_ci(RUN, SHA, 42))
        changes = ({"event": "pull_request"}, {"head_sha": "c" * 40},
                   {"head_repository": {"full_name": "other/sub2api"}},
                   {"head_branch": "codex/candidate"}, {"conclusion": "failure"},
                   {"conclusion": "cancelled"}, {"workflow_id": 99},
                   {"display_title": f"Personal CI | {SHA} | base {'c' * 40}"})
        for change in changes:
            with self.subTest(change=change):
                self.assertFalse(release.trusted_ci({**RUN, **change}, SHA, 42))

    def test_documentation_only_does_not_hide_build_or_runtime_changes(self):
        self.assertTrue(release.documentation_only(["docs/dev-journal.md", "openspec/changes/x/tasks.md"]))
        for path in ("backend/resources/prompt.md", "frontend/src/main.ts", ".github/workflows/release.yml", "Dockerfile.goreleaser"):
            self.assertFalse(release.documentation_only(["docs/dev-journal.md", path]))

    def gate_fixture(self, check_suite=7):
        protection = {"protected": True, "protection": {"required_status_checks": {"enforcement_level": "everyone", "checks": [
            {"context": "personal-ready", "app_id": 15368}]}}}
        def api(path, *args, **kwargs):
            if path.endswith("personal-ci.yml"):
                return {"id": 42}
            if "/actions/runs/" in path:
                return RUN
            if path.endswith("branches/personal"):
                return protection
            raise AssertionError(path)
        checks = [{"name": "personal-ready", "status": "completed", "conclusion": "success",
                   "app": {"id": 15368}, "check_suite": {"id": check_suite}}]
        def git(*args):
            return SHA if args[0] == "rev-parse" else "b" * 40 + "\trefs/tags/" + BASE
        return api, checks, git

    def test_gate_rejects_successful_check_from_another_run(self):
        for suite in (7, 8):
            api, checks, git = self.gate_fixture(suite)
            with patch.object(release, "api", side_effect=api), patch.object(release, "pages", return_value=checks), \
                 patch.object(release, "git", side_effect=git), patch.object(release, "assert_base"), \
                 patch.object(release, "source_at", return_value=SOURCE):
                if suite == 7:
                    self.assertEqual(release.gate(SHA, "123"), (SOURCE, RUN))
                else:
                    with self.assertRaisesRegex(release.SyncError, "必要检查未通过"):
                        release.gate(SHA, "123")

    def test_wrong_checkout_and_changed_personal_stop_before_api(self):
        with patch.object(release, "git", return_value="c" * 40), patch.object(release, "api") as api:
            with self.assertRaisesRegex(release.SyncError, "checkout"):
                release.gate(SHA)
            api.assert_not_called()
        with patch.object(release, "git", return_value=SHA), \
             patch.object(release, "assert_base", side_effect=release.SyncError("基础变化")), \
             patch.object(release, "api") as api:
            with self.assertRaisesRegex(release.SyncError, "基础变化"):
                release.gate(SHA)
            api.assert_not_called()

    def test_new_tag_then_retry_reuses_tag_and_completed_publication(self):
        existing = {}
        writes = []
        def api(path, payload=None, **kwargs):
            writes.append((path, payload))
            if path.endswith("git/refs"):
                existing[TAG] = payload["sha"]
        args = argparse.Namespace(sha=SHA, ci_run_id="123")
        with patch.object(release, "gate", return_value=(SOURCE, RUN)), \
             patch.object(release, "tags", side_effect=lambda: existing.copy()), \
             patch.object(release, "assert_base"), patch.object(release, "api", side_effect=api), \
             patch.object(release, "publication", return_value=(None, False)), patch.object(release, "output"):
            release.prepare(args)
            release.prepare(args)
            self.assertEqual(sum(p.endswith("git/refs") for p, _ in writes), 1)
            self.assertEqual([v["inputs"]["tag"] for p, v in writes if p.endswith("dispatches")], [TAG, TAG])
            with patch.object(release, "publication", return_value=("sha256:fixed", True)):
                count = len(writes)
                release.prepare(args)
                self.assertEqual(len(writes), count)

    def test_tag_collision_and_wrong_workflow_source_never_publish(self):
        args = argparse.Namespace(sha=SHA, ci_run_id="123", tag=TAG)
        with patch.dict(os.environ, {"GITHUB_REF": "refs/heads/personal", "GITHUB_SHA": SHA}), \
             patch.object(release, "gate", return_value=(SOURCE, RUN)), \
             patch.object(release, "tags", return_value={TAG: "c" * 40}), \
             patch.object(release, "publication") as publish:
            with self.assertRaisesRegex(release.SyncError, "个人标签"):
                release.authorize(args)
            publish.assert_not_called()
        with patch.dict(os.environ, {"GITHUB_REF": "refs/heads/main", "GITHUB_SHA": SHA}), \
             patch.object(release, "gate") as gate:
            with self.assertRaisesRegex(release.SyncError, "dispatch"):
                release.authorize(args)
            gate.assert_not_called()

    def test_occupied_image_checks_revision_version_and_platform(self):
        config = {"os": "linux", "architecture": "amd64", "config": {"Labels": {
            "org.opencontainers.image.revision": SHA, "org.opencontainers.image.version": TAG[1:],
            "org.opencontainers.image.source": "https://github.com/" + release.REPOSITORY}}}
        for mutation in (None, "revision", "version", "platform"):
            data = copy.deepcopy(config)
            if mutation == "platform":
                data["architecture"] = "arm64"
            elif mutation:
                data["config"]["Labels"]["org.opencontainers.image." + mutation] = "wrong"
            responses = [({"token": "fixture"}, {}), ({"config": {"digest": "sha256:config"}},
                        {"Docker-Content-Digest": "sha256:fixed"}), (data, {})]
            with patch.dict(os.environ, {"GH_TOKEN": "fixture"}), patch.object(release, "api", return_value={}), \
                 patch.object(release, "request", side_effect=responses):
                if mutation is None:
                    self.assertEqual(release.image_state(TAG, SHA), "sha256:fixed")
                else:
                    with self.assertRaisesRegex(release.SyncError, "禁止覆盖"):
                        release.image_state(TAG, SHA)

    def test_missing_image_after_published_release_is_not_rebuilt(self):
        with patch.object(release, "image_state", return_value=None), \
             patch.object(release, "api", return_value={"draft": False}):
            with self.assertRaisesRegex(release.SyncError, "禁止重建"):
                release.publication(TAG, SHA)

    def test_personal_changes_during_remote_queries_prevent_authorization(self):
        args = argparse.Namespace(sha=SHA, ci_run_id="123", tag=TAG)
        with patch.dict(os.environ, {"GITHUB_REF": "refs/heads/personal", "GITHUB_SHA": SHA}), \
             patch.object(release, "gate", return_value=(SOURCE, RUN)), \
             patch.object(release, "tags", return_value={TAG: SHA}), \
             patch.object(release, "publication", return_value=(None, False)), \
             patch.object(release, "assert_base", side_effect=release.SyncError("基础变化")), \
             patch.object(release, "output") as output:
            with self.assertRaisesRegex(release.SyncError, "基础变化"):
                release.authorize(args)
            output.assert_not_called()


if __name__ == "__main__":
    unittest.main()
