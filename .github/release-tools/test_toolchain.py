"""保护精确工具版本、哈希安装、扫描失败传播及构建元数据合同。"""

import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import shutil
import sys
import tempfile
import unittest
from unittest.mock import patch

import yaml

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("toolchain", Path(__file__).with_name("toolchain.py"))
toolchain = importlib.util.module_from_spec(spec)
spec.loader.exec_module(toolchain)
PINS = json.loads(toolchain.POLICY.read_text())


class ToolchainPolicyTests(unittest.TestCase):
    def test_setup_versions_and_go_source_are_exact(self):
        seen = set()
        for path in sorted((ROOT / ".github/workflows").glob("*.yml")):
            workflow = yaml.safe_load(path.read_text())
            for job in workflow["jobs"].values():
                for step in job.get("steps", []):
                    action = step.get("uses", "").split("@")[0]
                    tool = {"actions/setup-node": "node", "pnpm/action-setup": "pnpm",
                            "actions/setup-python": "python", "goreleaser/goreleaser-action": "goreleaser"}.get(action)
                    with self.subTest(workflow=path.name, action=action):
                        if tool:
                            key = {"node": "node-version", "python": "python-version"}.get(tool, "version")
                            self.assertEqual(str(step["with"][key]), PINS[tool])
                            seen.add(tool)
                        if action == "actions/setup-go":
                            self.assertEqual(step["with"]["go-version-file"], "backend/go.mod")
                            self.assertIs(step["with"]["check-latest"], False)
                            self.assertEqual(workflow["env"]["GOTOOLCHAIN"], "local")
        self.assertEqual(seen, {"node", "pnpm", "python", "goreleaser"})

    def test_python_install_requires_binary_hashes(self):
        installs = []
        for path in (ROOT / ".github/workflows").glob("*.yml"):
            for job in yaml.safe_load(path.read_text())["jobs"].values():
                for step in job.get("steps", []):
                    command = step.get("run", "")
                    if "pip install" in command:
                        installs.append(command)
                        self.assertIn("--require-hashes", command)
                        self.assertIn("--only-binary=:all:", command)
        # 原四处安装与 release-recovery 的一处安装均须通过上述逐项限制。
        self.assertEqual(len(installs), 5)
        requirements = toolchain.POLICY.with_name("requirements-release.txt").read_text()
        self.assertIn("PyYAML==" + PINS["pyyaml"], requirements)
        self.assertEqual(len(re.findall(r"--hash=sha256:[0-9a-f]{64}", requirements)), 3)

    def test_scanner_remains_a_failing_gate_and_records_database_version(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/security-scan.yml").read_text())
        step = next(step for step in workflow["jobs"]["backend-security"]["steps"] if step.get("name") == "Run govulncheck")
        self.assertIn("go install golang.org/x/vuln/cmd/govulncheck@" + PINS["govulncheck"], step["run"])
        self.assertIn("govulncheck -show version,verbose ./...", step["run"])
        self.assertNotIn("@latest", step["run"])
        self.assertNotIn("-json", step["run"])
        self.assertNotIn("||", step["run"])
        self.assertFalse(step.get("continue-on-error", False))

    def test_docker_builder_tools_and_package_manager_are_fixed(self):
        dockerfile = (ROOT / "deploy/Dockerfile").read_text()
        self.assertRegex(dockerfile, r"ARG NODE_IMAGE=node:" + re.escape(PINS["docker_node"]) + r"-alpine@sha256:[0-9a-f]{64}")
        go = re.search(r"^go (\d+\.\d+\.\d+)$", (ROOT / "backend/go.mod").read_text(), re.M)[1]
        self.assertRegex(dockerfile, r"ARG GOLANG_IMAGE=golang:" + re.escape(go) + r"-alpine@sha256:[0-9a-f]{64}")
        self.assertIn("GOTOOLCHAIN=local", dockerfile)
        self.assertIn("pnpm@" + PINS["pnpm"], dockerfile)
        package = json.loads((ROOT / "frontend/package.json").read_text())
        self.assertEqual(package["packageManager"], "pnpm@" + PINS["pnpm"])
        steps = yaml.safe_load((ROOT / ".github/workflows/release.yml").read_text())["jobs"]["release"]["steps"]
        buildx = next(step for step in steps if step.get("name") == "Set up Docker Buildx")
        qemu = next(step for step in steps if step.get("name") == "Set up QEMU")
        self.assertEqual(buildx["with"]["version"], PINS["buildx"])
        self.assertEqual(buildx["with"]["driver-opts"], "image=" + PINS["buildkit_image"])
        self.assertEqual(qemu["with"]["image"], PINS["qemu_image"])

    def test_monthly_review_is_read_only_and_cannot_upgrade_during_build(self):
        workflow = yaml.safe_load((ROOT / ".github/workflows/toolchain-review.yml").read_text())
        triggers = workflow.get("on", workflow.get(True))
        self.assertEqual(triggers["schedule"], [{"cron": "23 4 1 * *"}])
        self.assertEqual(workflow["permissions"], {"contents": "read"})
        script = next(step["run"] for step in workflow["jobs"]["review"]["steps"] if step.get("name") == "Record installed policy and check official releases")
        self.assertNotIn("pip install", script)
        self.assertNotIn("go install", script)
        self.assertNotIn("git push", script)

    def test_binary_build_and_report_use_the_explicitly_installed_executable(self):
        steps = yaml.safe_load((ROOT / ".github/workflows/release.yml").read_text())["jobs"]["build-binaries"]["steps"]
        install = next(i for i, step in enumerate(steps) if step.get("name") == "Install pinned binary build tool")
        build = next(i for i, step in enumerate(steps) if step.get("name") == "Build and archive without publishing")
        report = next(i for i, step in enumerate(steps) if step.get("name") == "Record binary build tool versions")
        self.assertIs(steps[install]["with"]["install-only"], True)
        self.assertEqual(steps[install]["with"]["version"], PINS["goreleaser"])
        self.assertLess(install, build)
        self.assertLess(build, report)
        self.assertTrue(steps[build]["run"].startswith("goreleaser release "))


class ToolchainReportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.previous = Path.cwd()
        os.chdir(ROOT)
        self.addCleanup(os.chdir, self.previous)
        self.destination = Path(self.temp.name) / "metadata.json"

    def test_metadata_records_actual_versions_source_and_safe_input_hashes(self):
        with patch.object(toolchain, "installed_version", return_value=PINS["python"]), patch.dict(os.environ, {"CI_TEST_SECRET": "never-export-this"}):
            toolchain.report(["python"], self.destination)
        metadata = json.loads(self.destination.read_text())
        self.assertEqual(metadata["actual"], {"python": PINS["python"]})
        self.assertRegex(metadata["source_sha"], r"^[0-9a-f]{40}$")
        self.assertRegex(metadata["inputs_sha256"]["pnpm-lock.yaml"], r"^[0-9a-f]{64}$")
        self.assertNotIn("never-export-this", self.destination.read_text())

    def test_wrong_version_and_command_failure_do_not_write_success_metadata(self):
        for result in ("0.0.0", subprocess.CalledProcessError(1, ["node", "--version"])):
            with self.subTest(result=result), patch.object(toolchain, "installed_version", **({"side_effect": result} if isinstance(result, Exception) else {"return_value": result})):
                with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                    toolchain.report(["node"], self.destination)
                self.assertFalse(self.destination.exists())

    def test_transported_tooling_reports_old_source_without_source_owned_policy(self):
        directory = Path(self.temp.name)
        source = directory / "source"
        source.mkdir()
        for name in ("backend/go.mod", "backend/go.sum", "frontend/package.json", "frontend/pnpm-lock.yaml"):
            dest = source / name
            dest.parent.mkdir(exist_ok=True)
            shutil.copyfile(ROOT / name, dest)
        tools = directory / "transported"
        tools.mkdir()
        for name in ("toolchain.py", "toolchain.json", "requirements-release.txt"):
            shutil.copyfile(toolchain.POLICY.with_name(name), tools / name)
        subprocess.run(["git", "init", "-q"], cwd=source, check=True)
        subprocess.run(["git", "add", "."], cwd=source, check=True)
        subprocess.run(["git", "-c", "user.name=CI", "-c", "user.email=ci@example.invalid", "commit", "-qm", "fixture"], cwd=source, check=True)
        subprocess.run([sys.executable, str(tools / "toolchain.py"), "--tools", "python", "--output", str(self.destination)], cwd=source, check=True)
        metadata = json.loads(self.destination.read_text())
        self.assertEqual(metadata["source_sha"], subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=source, text=True).strip())
        self.assertFalse((source / ".github").exists())

    def test_version_parsing_and_missing_tool_fail_explicitly(self):
        with patch.object(toolchain, "output", return_value="GitVersion:    2.18.3\nGitCommit:     fixture"):
            self.assertEqual(toolchain.installed_version("goreleaser"), "2.18.3")
        with patch.object(toolchain, "output", return_value="unknown"):
            with self.assertRaises(ValueError):
                toolchain.installed_version("node")
        with patch.object(toolchain.shutil, "which", return_value=None):
            with self.assertRaises(FileNotFoundError):
                toolchain.installed_version("govulncheck")


if __name__ == "__main__":
    unittest.main()
