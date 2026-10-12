"""执行实际 TPS CI 步骤，验证 main 适用性及 personal 失败传播。"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


class TPSCITests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow = yaml.safe_load((Path(__file__).resolve().parents[1] / "workflows/backend-ci.yml").read_text())
        cls.step = next(s for s in workflow["jobs"]["frontend"]["steps"]
                        if s.get("name") == "TPS contracts and frontend build")

    def run_step(self, bound, present, fail_command=""):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            if present:
                module = root / "frontend/src/components/admin/usage/UsageTps.vue"
                module.parent.mkdir(parents=True)
                module.touch()
            binary = root / "pnpm"
            binary.write_text('#!/bin/sh\nprintf "%s\\n" "$*" >> "$COMMAND_LOG"\n'
                              'case "$*" in *"$FAIL_COMMAND"*) [ -z "$FAIL_COMMAND" ] || exit 37;; esac\n')
            binary.chmod(0o755)
            log = root / "commands"
            env = dict(os.environ, PATH=str(root) + os.pathsep + os.environ["PATH"],
                       TPS_SOURCE_SHA="a" * 40 if bound else "", FAIL_COMMAND=fail_command, COMMAND_LOG=str(log))
            result = subprocess.run(["bash", "-e", "-c", self.step["run"]], cwd=root, env=env,
                                    capture_output=True, text=True)
            return result, log.read_text().splitlines()

    def test_main_without_tps_still_builds(self):
        result, commands = self.run_step(False, False)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("不适用", result.stdout)
        self.assertEqual(commands, ["--dir frontend run build"])

    def test_personal_source_cannot_skip_missing_tps_and_propagates_test_failure(self):
        self.assertEqual(self.step["env"]["TPS_SOURCE_SHA"], "${{ inputs.source_sha }}")
        result, commands = self.run_step(True, False, "vitest")
        self.assertEqual(result.returncode, 37)
        self.assertIn("UsageTps.spec.ts", commands[0])
        self.assertEqual(len(commands), 1)

    def test_present_module_runs_contract_lint_and_build_with_failure_propagation(self):
        result, commands = self.run_step(False, True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(len(commands), 3)
        self.assertIn("UsageTps.vue", commands[1])
        for fail in ("eslint", "run build"):
            with self.subTest(fail=fail):
                result, _ = self.run_step(True, True, fail)
                self.assertEqual(result.returncode, 37)


if __name__ == "__main__":
    unittest.main()
