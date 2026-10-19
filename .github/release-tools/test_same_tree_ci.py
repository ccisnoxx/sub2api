"""以真实 Git 树和父提交保护 Full CI 的同树合并复用边界。"""

import copy
from contextlib import ExitStack
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import personal_release as release


class SameTreeCITest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.git("init", "-q")
        self.git("config", "user.name", "CI")
        self.git("config", "user.email", "ci@example.invalid")
        (self.root / "app").write_text("base\n")
        self.git("add", "app")
        self.git("commit", "-qm", "base")
        self.base = self.git("rev-parse", "HEAD")
        (self.root / "app").write_text("candidate\n")
        self.git("commit", "-qam", "candidate")
        self.candidate = self.git("rev-parse", "HEAD")
        self.tree = self.git("rev-parse", "HEAD^{tree}")
        self.sha = self.commit(self.tree, self.base, self.candidate)
        self.pr = {"number": 57, "state": "closed", "merged": True,
                   "merged_at": "2026-10-19T00:00:00Z", "merge_commit_sha": self.sha,
                   "base": {"ref": "personal", "sha": self.base, "repo": {"full_name": release.REPOSITORY}},
                   "head": {"ref": "codex/candidate", "sha": self.candidate,
                            "repo": {"full_name": release.REPOSITORY}}}
        self.run = {"id": 123, "workflow_id": 42, "path": ".github/workflows/personal-ci.yml",
                    "repository": {"full_name": release.REPOSITORY},
                    "head_repository": {"full_name": release.REPOSITORY},
                    "head_branch": self.pr["head"]["ref"], "head_sha": self.candidate,
                    "status": "completed", "conclusion": "success", "event": "workflow_dispatch",
                    "display_title": release.ci_title(self.candidate, self.base, "full")}
        self.jobs = [{"name": name, "status": "completed", "conclusion": "success"}
                     for name in release.FULL_JOBS]
        self.source = {"upstream_tag": "v0.2.17-klno.1", "upstream_sha": "b" * 40}
        self.calls = []
        self.original_api = release.api
        self.stack = ExitStack()
        self.addCleanup(self.stack.close)
        self.stack.enter_context(patch.dict(os.environ, {"GITHUB_REPOSITORY": release.REPOSITORY}))
        self.stack.enter_context(patch.object(release, "urlopen", side_effect=AssertionError("测试禁止真实网络")))
        self.stack.enter_context(patch.object(release, "api", side_effect=self.api))
        self.stack.enter_context(patch.object(release, "pages", side_effect=self.pages))
        self.stack.enter_context(patch.object(release, "git", side_effect=self.release_git))
        self.stack.enter_context(patch.object(release, "ancestor", side_effect=self.ancestor))
        self.assert_base = self.stack.enter_context(patch.object(release, "assert_base"))
        self.stack.enter_context(patch.object(release, "source_at", return_value=self.source))

    def git(self, *args):
        return subprocess.check_output(["git", *args], cwd=self.root, text=True,
                                       stderr=subprocess.PIPE).strip()

    def ancestor(self, base, candidate):
        result = subprocess.run(["git", "merge-base", "--is-ancestor", base, candidate], cwd=self.root)
        self.assertIn(result.returncode, (0, 1))
        return result.returncode == 0

    def commit(self, tree, *parents):
        args = ["commit-tree", tree, "-m", "merge"]
        for parent in parents:
            args.extend(("-p", parent))
        return self.git(*args)

    def release_git(self, *args):
        if args == ("rev-parse", "HEAD"):
            return self.sha
        if args[0] == "ls-remote":
            return self.source["upstream_sha"] + "\trefs/tags/" + self.source["upstream_tag"]
        return self.git(*args)

    def api(self, path, *args, **kwargs):
        self.assertFalse(args, "门禁不得写入或调度 CI")
        self.calls.append(path)
        if path.endswith("/actions/workflows/personal-ci.yml"):
            return {"id": 42}
        if path.endswith("/actions/runs/123"):
            return self.run
        if path.endswith("/pulls/57"):
            self.assertEqual(kwargs, {"api_version": "2022-11-28"})
            return self.pr
        self.assertFalse(kwargs, "其他 API 保留默认版本")
        raise AssertionError(path)

    def pages(self, path, key=None):
        self.calls.append(path)
        if path.endswith("/pulls"):
            # 2026-03-10 的列表不含 merge_commit_sha，不能依赖旧版形状。
            return [{k: v for k, v in self.pr.items() if k != "merge_commit_sha"}]
        if key == "workflow_runs":
            if "head_sha=" + self.candidate in path:
                self.assertIn("branch=codex%2Fcandidate", path)
                return [self.run]
            return []
        if path.endswith("/actions/runs/123/jobs?filter=latest") and key == "jobs":
            return self.jobs
        raise AssertionError(path)

    def test_explicit_and_automatic_reuse_preserve_final_source_and_fixed_run(self):
        for run_id in ("123", ""):
            with self.subTest(run_id=run_id):
                self.assertEqual(release.gate(self.sha, run_id), (self.source, self.run))
                self.assert_base.assert_called_with(self.sha)
                self.assertNotEqual(self.sha, self.run["head_sha"])

    def test_pr_detail_version_is_pinned_without_changing_other_api_versions(self):
        with patch.dict(os.environ, {"GH_TOKEN": "fixture"}), patch.object(
                release, "request", return_value=({}, {})) as request:
            self.original_api("repos/example/pulls/57", api_version="2022-11-28")
            self.assertEqual(request.call_args.args[1]["X-GitHub-Api-Version"], "2022-11-28")
            self.original_api("repos/example/actions/runs/123")
            self.assertEqual(request.call_args.args[1]["X-GitHub-Api-Version"], "2026-03-10")

    def test_content_mode_and_type_changes_even_in_docs_or_workflows_require_new_full(self):
        for name, mode in (("docs/notes.md", "text"), (".github/workflows/ci.yml", "text"),
                           ("app", "executable"), ("app", "symlink")):
            with self.subTest(name=name, mode=mode):
                self.git("read-tree", self.candidate)
                if mode == "executable":
                    self.git("update-index", "--chmod=+x", "app")
                else:
                    blob = self.git("rev-parse", self.candidate + ":app")
                    self.git("update-index", "--add", "--cacheinfo",
                             "120000" if mode == "symlink" else "100644", blob, name)
                self.sha = self.commit(self.git("write-tree"), self.base, self.candidate)
                with self.assertRaises(release.SyncError):
                    release.gate(self.sha, "123")
                self.assertFalse(any("/pulls" in p for p in self.calls))

    def test_squash_rebase_octopus_and_changed_merge_base_do_not_reuse_same_tree(self):
        other_base = self.commit(self.git("rev-parse", self.base + "^{tree}"), self.base)
        for parents in ((self.base,), (self.candidate,), (self.base, self.candidate, other_base),
                        (other_base, self.candidate)):
            with self.subTest(parents=parents):
                self.sha = self.commit(self.tree, *parents)
                with self.assertRaises(release.SyncError):
                    release.gate(self.sha, "123")

    def test_unrelated_unmerged_foreign_or_changed_pr_cannot_authorize_reuse(self):
        original = copy.deepcopy(self.pr)
        for field, value in (("merged", False), ("state", "open"), ("merged_at", None),
                             ("merge_commit_sha", "c" * 40), ("base.ref", "main"),
                             ("base.sha", "c" * 40), ("base.repo.full_name", "other/sub2api"),
                             ("head.sha", "c" * 40), ("head.ref", "codex/other"),
                             ("head.repo", None), ("head.repo.full_name", "other/sub2api")):
            self.pr = copy.deepcopy(original)
            target = self.pr
            parts = field.split(".")
            for part in parts[:-1]:
                target = target[part]
            target[parts[-1]] = value
            with self.subTest(field=field), self.assertRaises(release.SyncError):
                release.gate(self.sha, "123")

    def test_reused_ci_still_requires_trusted_manual_full_and_every_bound_job(self):
        original = self.run.copy()
        for change in ({"event": "pull_request"}, {"event": "push"}, {"workflow_id": 99},
                       {"path": ".github/workflows/backend-ci.yml"},
                       {"head_repository": {"full_name": "other/sub2api"}},
                       {"repository": {"full_name": "other/sub2api"}}, {"head_sha": self.base},
                       {"head_branch": "other"}, {"status": "in_progress"},
                       {"conclusion": "failure"}, {"conclusion": "cancelled"},
                       {"display_title": release.ci_title(self.candidate, self.base, "quick")},
                       {"display_title": release.ci_title(self.candidate, "c" * 40, "full")}):
            self.run = {**original, **change}
            with self.subTest(change=change), self.assertRaises(release.SyncError):
                release.gate(self.sha, "123")
        self.run = original
        for name in release.FULL_JOBS:
            self.jobs = [j for j in self.jobs if j["name"] != name]
            with self.subTest(missing_job=name), self.assertRaisesRegex(release.SyncError, "全部成功"):
                release.gate(self.sha, "123")
            self.jobs.append({"name": name, "status": "completed", "conclusion": "success"})

    def test_rechecks_same_binding_and_never_switches_fixed_run_on_failure(self):
        self.assertEqual(release.gate(self.sha, "123")[1]["id"], 123)
        self.run["conclusion"] = "failure"
        with self.assertRaises(release.SyncError):
            release.gate(self.sha, "123")
        self.assertFalse(any("/runs?" in p for p in self.calls))
        self.run["conclusion"] = "success"
        self.pr["head"]["sha"] = self.base
        with self.assertRaises(release.SyncError):
            release.gate(self.sha, "123")


if __name__ == "__main__":
    unittest.main()
