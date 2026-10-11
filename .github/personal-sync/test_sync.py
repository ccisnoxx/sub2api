"""在临时 Git 仓库验证可观察的历史、失败与检查绑定合同。"""

import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import check_binding
import sync


class InputTests(unittest.TestCase):
    def test_numeric_klno_order_includes_prerelease_tags(self):
        refs = ["refs/tags/v0.2.14-klno.9", "refs/tags/v0.2.14-klno.10",
                "refs/tags/v0.2.14", "refs/tags/v0.2.14-klno.10^{}", "refs/tags/v0.2.14-klno.01"]
        self.assertEqual(sync.select_tag(refs), "v0.2.14-klno.10")
        self.assertEqual(sync.select_tag(refs + ["refs/tags/v0.3.0-klno.1"]), "v0.3.0-klno.1")

    def test_shell_and_revision_inputs_rejected(self):
        for value in ["$(touch /tmp/bad)", "v0.2.14-klno.3;id", "--upload-pack=id", "main", "a" * 39, "A" * 40]:
            with self.subTest(value=value), self.assertRaises(sync.SyncError):
                sync.validate_input(value)
        sync.validate_input("a" * 40)
        sync.validate_input("v0.2.14-klno.3")
        sync.validate_input("")

    def test_dispatch_must_match_workflow_sha(self):
        with self.assertRaisesRegex(sync.SyncError, "SHA 不一致"):
            check_binding.binding({}, "workflow_dispatch", "a" * 40, "b" * 40, "1", "c" * 40, "refs/heads/codex/example")
        with self.assertRaisesRegex(sync.SyncError, "完整"):
            check_binding.binding({}, "workflow_dispatch", "main", "b" * 40, "1", "c" * 40, "refs/heads/codex/example")


class GitTests(unittest.TestCase):
    def setUp(self):
        self.previous = Path.cwd()
        self.directory = tempfile.TemporaryDirectory()
        self.root = Path(self.directory.name)
        self.repo = self.root / "work"
        self.repo.mkdir()
        os.chdir(self.repo)
        sync.git("init", "-b", "main")
        sync.git("config", "user.name", "test")
        sync.git("config", "user.email", "test@example.invalid")
        Path("shared.txt").write_text("base\n")
        self.initial = self.commit("base")
        sync.git("tag", "v0.2.14-klno.9")
        sync.git("switch", "-c", "upstream")
        Path("new.txt").write_text("upstream10\n")
        self.target = self.commit("upstream10")
        sync.git("tag", "-a", "v0.2.14-klno.10", "-m", "prerelease")
        sync.git("switch", "-c", "personal", self.initial)
        Path("tps.txt").write_text("1040 / 24650 = 42.19 tok/s\n")
        Path(sync.SOURCE).parent.mkdir()
        Path(sync.SOURCE).write_text(json.dumps({"upstream_repository": sync.UPSTREAM,
            "upstream_tag": "v0.2.14-klno.9", "upstream_sha": self.initial}) + "\n")
        self.base = self.commit("TPS feature")
        self.remote = self.root / "origin.git"
        sync.git("init", "--bare", str(self.remote))
        sync.git("remote", "add", "origin", str(self.remote))
        sync.git("push", "origin", "main", "personal")

    def tearDown(self):
        os.chdir(self.previous)
        self.directory.cleanup()

    def commit(self, message):
        sync.git("add", ".")
        sync.git("commit", "-m", message)
        return sync.git("rev-parse", "HEAD")

    def prepare(self, rehearsal=False):
        return sync.prepare(self.base, "v0.2.14-klno.10", self.target, rehearsal)

    def test_merge_preserves_tps_upstream_and_main_history(self):
        candidate = sync.publish(self.prepare())
        self.assertTrue(sync.ancestor(self.base, candidate["candidate_sha"]))
        self.assertTrue(sync.ancestor(self.target, candidate["candidate_sha"]))
        self.assertEqual(Path("tps.txt").read_text(), "1040 / 24650 = 42.19 tok/s\n")
        self.assertEqual(Path("new.txt").read_text(), "upstream10\n")
        self.assertEqual(json.loads(Path(sync.SOURCE).read_text())["upstream_sha"], self.target)
        self.assertEqual(sync.remote_head("origin", "personal"), self.base)
        self.assertEqual(sync.remote_head("origin", "main"), self.initial)
        self.assertEqual(sync.git("rev-parse", "v0.2.14-klno.10^{commit}"), self.target)

    def test_no_update_has_no_candidate_or_push(self):
        result = sync.prepare(self.base, "v0.2.14-klno.9", self.initial)
        self.assertEqual(result["state"], "no_update")
        self.assertFalse(Path(sync.METADATA).exists())
        self.assertEqual(sync.remote_head("origin", "personal"), self.base)

    def test_rerun_reuses_identical_remote_sha(self):
        first = sync.publish(self.prepare())
        second = sync.publish(self.prepare())
        self.assertEqual(first["candidate_sha"], second["candidate_sha"])
        self.assertEqual(first["branch"], second["branch"])

    def test_modified_candidate_is_not_overwritten(self):
        first = sync.publish(self.prepare())
        Path("unexpected.txt").write_text("external change")
        altered = self.commit("external change")
        sync.git("push", "origin", f"HEAD:refs/heads/{first['branch']}")
        with self.assertRaisesRegex(sync.SyncError, "禁止覆盖"):
            sync.publish(self.prepare())
        self.assertEqual(sync.remote_head("origin", first["branch"]), altered)

    def test_same_tree_without_upstream_history_is_not_reused(self):
        first = sync.publish(self.prepare())
        tree = sync.git("rev-parse", "HEAD^{tree}")
        squashed = sync.git("commit-tree", tree, "-p", self.base, "-m", "same tree squash")
        sync.git("push", "origin", f"{squashed}:refs/heads/squash-sample")
        sync.git("--git-dir", str(self.remote), "update-ref", f"refs/heads/{first['branch']}", squashed)
        with self.assertRaisesRegex(sync.SyncError, "丢失上游 merge 历史"):
            sync.publish(self.prepare())
        self.assertEqual(sync.remote_head("origin", first["branch"]), squashed)

    def test_ci_rejects_same_tree_candidate_without_source_history(self):
        self.prepare()
        tree = sync.git("rev-parse", "HEAD^{tree}")
        squashed = sync.git("commit-tree", tree, "-p", self.base, "-m", "same tree squash")
        sync.git("switch", "--detach", squashed)
        event_file = self.root / "event.json"
        event_file.write_text("{}")
        pr = {"state": "open", "base": {"ref": "personal", "sha": self.base},
              "head": {"sha": squashed, "repo": {"full_name": sync.REPOSITORY}}}
        environment = {"GITHUB_REPOSITORY": sync.REPOSITORY, "GITHUB_EVENT_PATH": str(event_file),
                       "GITHUB_EVENT_NAME": "workflow_dispatch", "CANDIDATE_SHA": squashed,
                       "BASE_SHA": self.base, "PR_NUMBER": "1", "GITHUB_SHA": squashed,
                       "GITHUB_REF": "refs/heads/codex/example"}
        with patch.dict(os.environ, environment), patch.object(check_binding, "gh_json", return_value=pr):
            with self.assertRaisesRegex(sync.SyncError, "来源 SHA 不在 personal 历史"):
                check_binding.check()

    def test_resolver_uses_isolated_tags_even_after_klno_rewrite(self):
        upstream_remote = self.root / "upstream.git"
        sync.git("init", "--bare", str(upstream_remote))
        sync.git("remote", "add", "kln-source", str(upstream_remote))
        # 发布标签保留在原提交；当前 klno 指向另一个历史。
        sync.git("push", "kln-source", "personal:klno", "refs/tags/v0.2.14-klno.9", "refs/tags/v0.2.14-klno.10")
        sync.git("tag", "-f", "v0.2.14-klno.10", self.initial)
        tag, target = sync.resolve_target("")
        self.assertEqual((tag, target), ("v0.2.14-klno.10", self.target))
        self.assertEqual(sync.git("rev-parse", "v0.2.14-klno.10^{commit}"), self.initial)
        self.assertEqual(sync.git("rev-parse", "refs/personal-sync/tags/v0.2.14-klno.10^{commit}"), self.target)
        with self.assertRaisesRegex(sync.SyncError, "未发布 SHA 不属于"):
            sync.resolve_target(self.target)

    def test_conflict_aborts_without_remote_writes(self):
        sync.git("switch", "upstream")
        Path("shared.txt").write_text("upstream conflict\n")
        target = self.commit("upstream conflict")
        sync.git("switch", "personal")
        Path("shared.txt").write_text("personal conflict\n")
        base = self.commit("personal conflict")
        with self.assertRaisesRegex(sync.SyncError, "shared.txt"):
            sync.prepare(base, "v0.2.14-klno.11", target)
        self.assertEqual(sync.git("rev-parse", "HEAD"), base)
        self.assertFalse(Path(sync.git("rev-parse", "--git-path", "MERGE_HEAD")).exists())
        self.assertEqual(sync.remote_head("origin", "personal"), self.base)

    def test_rewritten_upstream_stops(self):
        sync.git("switch", "--orphan", "rewritten")
        Path("rewrite.txt").write_text("rewritten root\n")
        rewritten = self.commit("rewritten")
        with self.assertRaisesRegex(sync.SyncError, "改写历史"):
            sync.prepare(self.base, "v0.2.14-klno.11", rewritten)

    def test_base_change_stops_push_and_new_base_has_new_branch(self):
        candidate = self.prepare()
        sync.git("switch", "personal")
        Path("later.txt").write_text("base advanced\n")
        new_base = self.commit("advance personal")
        sync.git("push", "origin", "personal")
        with self.assertRaisesRegex(sync.SyncError, "基础分支已变化"):
            sync.publish(candidate)
        newer = sync.prepare(new_base, "v0.2.14-klno.10", self.target)
        self.assertNotEqual(candidate["branch"], newer["branch"])
        self.assertTrue(sync.ancestor(new_base, newer["candidate_sha"]))

    def test_permissions_failure_keeps_maintenance_branches(self):
        hook = self.remote / "hooks" / "pre-receive"
        hook.write_text("#!/bin/sh\nprintf '%s\\n' 'workflows permission denied' >&2\nexit 1\n")
        hook.chmod(0o755)
        with self.assertRaisesRegex(sync.SyncError, "Contents/Workflows"):
            sync.publish(self.prepare())
        self.assertEqual(sync.remote_head("origin", "personal"), self.base)
        self.assertEqual(sync.remote_head("origin", "main"), self.initial)

    def test_upstream_control_changes_stop_instead_of_restoring_old_logic(self):
        sync.git("switch", "upstream")
        path = Path(".github/workflows/sync-upstream.yml")
        path.parent.mkdir(parents=True)
        path.write_text("unsafe inherited synchronization\n")
        target = self.commit("upstream workflow")
        with self.assertRaisesRegex(sync.SyncError, "同步控制文件"):
            sync.prepare(self.base, "v0.2.14-klno.11", target)

    def test_upstream_cannot_weaken_reusable_ci_or_security(self):
        for name in ["backend-ci.yml", "security-scan.yml"]:
            with self.subTest(name=name):
                sync.git("switch", "--detach", self.target)
                path = Path(".github/workflows") / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("weakened check\n")
                target = self.commit("weaken upstream check")
                with self.assertRaisesRegex(sync.SyncError, "同步控制文件"):
                    sync.prepare(self.base, "v0.2.14-klno.11", target)

    def test_manual_sha_record_does_not_invent_release_tag(self):
        sync.prepare(self.base, None, self.target)
        self.assertIsNone(json.loads(Path(sync.SOURCE).read_text())["upstream_tag"])

    def test_rehearsal_never_changes_upstream_source(self):
        candidate = self.prepare(rehearsal=True)
        self.assertEqual(candidate["mode"], "rehearsal")
        self.assertEqual(json.loads(Path(sync.SOURCE).read_text())["upstream_sha"], self.initial)
        self.assertFalse(Path("new.txt").exists())

    def test_completed_check_rejects_new_base(self):
        sync.git("switch", "personal")
        Path("later.txt").write_text("later")
        self.commit("later")
        sync.git("push", "origin", "personal")
        with self.assertRaisesRegex(sync.SyncError, "基础分支已变化"):
            sync.assert_base(self.base)


class DispatchTests(unittest.TestCase):
    def test_dispatch_uses_candidate_branch_and_complete_shas(self):
        candidate = {"branch": "codex/example", "candidate_sha": "a" * 40,
                     "base_sha": "b" * 40, "pr_number": 12}
        with patch.object(sync, "gh_json", return_value={"workflow_runs": []}), patch.object(sync, "run") as call:
            sync.dispatch_ci(candidate)
        args = call.call_args.args
        self.assertIn("candidate_sha=" + "a" * 40, args)
        self.assertIn("base_sha=" + "b" * 40, args)
        self.assertEqual(args[args.index("--ref") + 1], "codex/example")

    def test_only_same_sha_and_base_success_is_reused(self):
        candidate = {"branch": "codex/example", "candidate_sha": "a" * 40,
                     "base_sha": "b" * 40, "pr_number": 12}
        old = {"display_title": f"Personal CI | {'a' * 40} | base {'c' * 40}",
               "head_branch": "codex/example", "status": "completed", "conclusion": "success", "html_url": "old"}
        with patch.object(sync, "gh_json", return_value={"workflow_runs": [old]}), patch.object(sync, "run") as call:
            sync.dispatch_ci(candidate)
            call.assert_called_once()
        old["display_title"] = f"Personal CI | {'a' * 40} | base {'b' * 40}"
        with patch.object(sync, "gh_json", return_value={"workflow_runs": [old]}), patch.object(sync, "run") as call:
            self.assertEqual(sync.dispatch_ci(candidate)["ci_run"], "old")
            call.assert_not_called()


if __name__ == "__main__":
    unittest.main()
