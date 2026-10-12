"""发布控制脚本的来源、运输、写入前复核和无发布权限的 dry-run 合同。"""

import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[2]


class ReleaseControlTest(unittest.TestCase):
    def setUp(self):
        self.release = yaml.safe_load((ROOT / ".github/workflows/release.yml").read_text())
        self.personal = yaml.safe_load((ROOT / ".github/workflows/personal-release.yml").read_text())

    def test_prepare_uses_main_guard_before_application_checkout(self):
        steps = self.personal["jobs"]["prepare"]["steps"]
        self.assertEqual(steps[0]["with"]["ref"], "main")
        preserve = next(i for i, s in enumerate(steps) if s.get("name") == "Preserve trusted control tooling")
        source = next(i for i, s in enumerate(steps) if s.get("name") == "Checkout trusted personal source")
        self.assertLess(preserve, source)
        self.assertIn('$RUNNER_TEMP/release-tools/personal_release.py', steps[-1]["run"])

    def test_formal_guard_is_transported_from_main_and_rechecked_before_each_publisher(self):
        prepare = self.release["jobs"]["prepare"]["steps"]
        control = next(i for i, s in enumerate(prepare) if s.get("name") == "Checkout workflow tooling")
        save = next(i for i, s in enumerate(prepare) if s.get("name") == "Preserve trusted publication guard")
        source = next(i for i, s in enumerate(prepare) if s.get("name") == "Checkout selected application source")
        self.assertEqual(prepare[control]["with"]["ref"], "${{ env.DRY_RUN == 'true' && github.sha || 'main' }}")
        self.assertEqual(prepare[save]["if"], "env.DRY_RUN != 'true'")
        self.assertIs(prepare[control]["with"]["persist-credentials"], False)
        self.assertLess(control, save)
        self.assertLess(save, source)
        upload = next(s for s in prepare if s.get("with", {}).get("name") == "release-guard")
        # 上传允许增加排除规则，但可信 guard 的唯一正向来源不能变化。
        self.assertEqual([p for p in upload["with"]["path"].splitlines() if not p.startswith("!")],
                         ["${{ runner.temp }}/release-guard/"])
        self.assertEqual(upload["with"]["if-no-files-found"], "error")
        steps = self.release["jobs"]["release"]["steps"]
        download = next(i for i, s in enumerate(steps) if s.get("with", {}).get("name") == "release-guard")
        self.assertEqual(steps[download]["with"]["path"], "${{ runner.temp }}/release-guard/")
        for collection in (prepare, steps):
            for step in collection:
                if "personal_release.py" in step.get("run", ""):
                    self.assertIn('$RUNNER_TEMP/release-guard/release-tools/personal_release.py', step["run"])
                    self.assertEqual(step["if"], "env.DRY_RUN != 'true'")
        for guard_name, publisher_name in (
            ("Recheck final source, CI and version occupancy before publishing", "Build images and publish manifests"),
            ("Recheck final source, CI and tag before Release publication", "Publish existing archives and release notes"),
        ):
            guard = next(i for i, s in enumerate(steps) if s.get("name") == guard_name)
            publisher = next(i for i, s in enumerate(steps) if s.get("name") == publisher_name)
            self.assertLess(download, guard)
            self.assertLess(guard, publisher)
            self.assertFalse(steps[guard].get("continue-on-error", False))

    def test_saved_main_guard_ignores_application_owned_replacement_scripts(self):
        preserve = next(s for s in self.release["jobs"]["prepare"]["steps"]
                        if s.get("name") == "Preserve trusted publication guard")
        helpers = next(s for s in self.release["jobs"]["prepare"]["steps"]
                       if s.get("name") == "Preserve tooling for releases of older tags")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for name in ("release-tools", "personal-sync"):
                shutil.copytree(ROOT / ".github" / name, root / ".github" / name,
                                ignore=shutil.ignore_patterns("__pycache__"))
            control = root
            subprocess.run(["git", "init", "-q", str(control)], check=True, capture_output=True)
            subprocess.run(["git", "-C", str(control), "add", "."], check=True, capture_output=True)
            subprocess.run(["git", "-C", str(control), "-c", "user.name=CI", "-c", "user.email=ci@example.invalid",
                            "commit", "-qm", "控制面测试夹具"], check=True, capture_output=True)
            env = {"PATH": os.environ["PATH"], "RUNNER_TEMP": str(root / "runner")}
            (root / "runner").mkdir()
            subprocess.run(["bash", "-e", "-c", helpers["run"]], cwd=root, env=env, check=True, capture_output=True)
            subprocess.run(["bash", "-e", "-c", preserve["run"]], cwd=root, env=env, check=True, capture_output=True)
            for name, script in (("release-tools", "personal_release.py"), ("personal-sync", "sync.py"),
                                 ("release-tools", "release_matrix.py"), ("release-tools", "supply_chain.py")):
                target = root / ".github" / name / script
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_text('raise SystemExit("应用脚本被执行")\n')
            guard = root / "runner/release-guard/release-tools/personal_release.py"
            result = subprocess.run([sys.executable, str(guard), "--help"], cwd=root, env=env,
                                    text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("authorize", result.stdout)
            self.assertEqual(guard.read_bytes(), (ROOT / ".github/release-tools/personal_release.py").read_bytes())
            for helper in ("release_matrix.py", "supply_chain.py"):
                result = subprocess.run([sys.executable, str(root / "runner/release-tools" / helper), "--help"],
                                        cwd=root, env=env, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(guard.read_bytes(), (ROOT / ".github/release-tools/personal_release.py").read_bytes())
            expected = subprocess.check_output(["git", "-C", str(control), "rev-parse", "HEAD"], text=True).strip()
            self.assertEqual((root / "runner/release-guard/control-sha").read_text().strip(), expected)

    def test_all_publish_steps_keep_the_prepare_ci_run_id(self):
        job = self.release["jobs"]["release"]
        self.assertEqual(job["env"]["CI_RUN_ID"], "${{ needs.prepare.outputs.ci_run_id }}")
        for step in job["steps"]:
            with self.subTest(step=step.get("name", step.get("uses"))):
                self.assertNotIn("CI_RUN_ID", step.get("env", {}))

    def test_final_release_guard_rejects_changed_missing_or_inconsistent_digest(self):
        step = next(s for s in self.release["jobs"]["release"]["steps"]
                    if s.get("name") == "Recheck final source, CI and tag before Release publication")
        digest = "sha256:" + "d" * 64
        for current, publication, images, expected in (
            (digest, digest, [digest], 0),
            ("sha256:" + "e" * 64, digest, [digest], 1),
            ("", digest, [digest], 1),
            ("sha256:" + "e" * 64, "sha256:" + "e" * 64, [digest], 1),
            (digest, digest, [], 1),
        ):
            with self.subTest(current=current, publication=publication, images=images), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                summary = root / "release-evidence/summary/fixture.release-summary.json"
                summary.parent.mkdir(parents=True)
                summary.write_text(json.dumps({"images": [{"digest": d} for d in images]}))
                (root / "publication.json").write_text(json.dumps({"digest": publication}))
                fake = root / "python3"
                fake.write_text(f'''#!{sys.executable} -I
import json, os, sys
args = sys.argv[1:]
if args and args[0] == "-I":
    args = args[1:]
if args[0].endswith('/personal_release.py'):
    assert '/release-guard/release-tools/' in args[0]
    assert args[args.index('--ci-run-id') + 1] == '123'
    print(json.dumps({{'digest': os.environ['CURRENT_DIGEST']}}))
else:
    os.execv({sys.executable!r}, [{sys.executable!r}, '-B', *sys.argv[1:]])
''')
                fake.chmod(0o755)
                for module in ("json", "re", "pathlib"):
                    (root / (module + ".py")).write_text('raise SystemExit("应用模块被执行")\n')
                env = {"PATH": str(root) + os.pathsep + os.environ["PATH"], "RUNNER_TEMP": str(root),
                       "RELEASE_SHA": "a" * 40, "RELEASE_TAG": "v0.2.14-klno.3-tps.1", "CI_RUN_ID": "123",
                       "CURRENT_DIGEST": current}
                result = subprocess.run(["bash", "-e", "-c", step["run"]], cwd=root, env=env,
                                        text=True, capture_output=True)
                self.assertEqual(result.returncode, expected, result.stderr)

    def test_dependency_installer_does_not_import_application_pip(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'pip.py').write_text('raise SystemExit("应用 pip 被执行")\n')
            for name in ('prepare', 'release'):
                steps = self.release['jobs'][name]['steps']
                command = next(s['run'] for s in steps if 'pip install' in s.get('run', ''))
                prefix = command.split('pip install', 1)[0].split()[1:]
                result = subprocess.run([sys.executable, *prefix, 'pip', '--version'], cwd=root,
                                        env={'PATH': os.environ['PATH'], 'PYTHONPATH': str(root)},
                                        text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn('pip ', result.stdout)

    def test_dry_run_source_binding_needs_no_token_and_rejects_wrong_sha(self):
        step = next(s for s in self.release["jobs"]["prepare"]["steps"]
                    if s.get("name") == "Bind requested source SHA")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            git = root / "git"
            git.write_text('#!/bin/sh\nprintf "%s\\n" "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"\n')
            git.chmod(0o755)
            for sha, code in (("a" * 40, 0), ("b" * 40, 1)):
                with self.subTest(sha=sha):
                    env = {"PATH": str(root) + os.pathsep + os.environ["PATH"], "SOURCE_SHA": sha, "DRY_RUN": "true"}
                    result = subprocess.run(["bash", "-e", "-c", step["run"]], env=env, capture_output=True, text=True)
                    self.assertEqual(result.returncode, code, result.stderr)
        steps = self.release["jobs"]["release"]["steps"]
        for name in ("Login to GitHub Container Registry", "Login to DockerHub", "Verify published OCI revision, version and platform"):
            self.assertIn("env.DRY_RUN != 'true'", next(s for s in steps if s.get("name") == name)["if"])
        publisher = next(s for s in steps if s.get("name") == "Publish existing archives and release notes")
        self.assertIn("env.DRY_RUN == 'true' && '--snapshot --skip=publish,announce'", publisher["with"]["args"])


if __name__ == "__main__":
    unittest.main()
