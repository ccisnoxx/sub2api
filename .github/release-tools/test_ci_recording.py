"""执行实际 CI Bash 片段，验证 recording 检查选择和失败传播。"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


class RecordingCITests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow = yaml.safe_load(
            (Path(__file__).resolve().parents[1] / "workflows/backend-ci.yml").read_text()
        )
        cls.step = next(
            step for step in workflow["jobs"]["test"]["steps"]
            if step.get("name") == "OpenAI recording concurrency checks"
        )

    def run_step(self, *, bound, module_present, go_exit):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            if module_present:
                (root / "internal/pkg/upstreamrecord").mkdir(parents=True)
            bin_dir = root / "bin"
            bin_dir.mkdir()
            go = bin_dir / "go"
            go.write_text(
                '#!/bin/sh\nprintf "%s\\n" "$@" > "$GO_ARGUMENTS"\n'
                'exit "$FAKE_GO_EXIT"\n'
            )
            go.chmod(0o755)
            arguments = root / "go-arguments"
            env = dict(os.environ, PATH=str(bin_dir) + os.pathsep + os.environ["PATH"],
                       RECORDING_SOURCE_SHA="a" * 40 if bound else "",
                       GO_ARGUMENTS=str(arguments), FAKE_GO_EXIT=str(go_exit))
            result = subprocess.run(
                ["bash", "-e", "-c", self.step["run"]], cwd=root, env=env,
                capture_output=True, text=True,
            )
            args = arguments.read_text().splitlines() if arguments.exists() else None
            return result, args

    def assert_race_command(self, args):
        self.assertEqual(args, ["test", "-race", "-tags=unit", "-timeout=60s",
                                "-run", "^Test(Recorder|OpenAIRecording)",
                                "./internal/pkg/upstreamrecord", "./internal/service"])

    def test_workflow_binds_personal_source_input(self):
        # SHA 必须直接来自共享 CI 的绑定输入，不能靠模块存在性放宽 Personal CI。
        self.assertEqual(
            self.step.get("env", {}).get("RECORDING_SOURCE_SHA"),
            "${{ inputs.source_sha }}",
        )

    def test_main_without_feature_reports_not_applicable(self):
        result, args = self.run_step(bound=False, module_present=False, go_exit=37)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIsNone(args)
        self.assertIn("不适用", result.stdout)

    def test_bound_personal_without_module_does_not_skip_failure(self):
        result, args = self.run_step(bound=True, module_present=False, go_exit=2)
        self.assert_race_command(args)
        self.assertEqual(result.returncode, 2)

    def test_feature_present_in_ordinary_pr_runs_complete_race_command(self):
        result, args = self.run_step(bound=False, module_present=True, go_exit=0)
        self.assert_race_command(args)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_bound_personal_propagates_race_failure(self):
        result, args = self.run_step(bound=True, module_present=True, go_exit=37)
        self.assert_race_command(args)
        self.assertEqual(result.returncode, 37)


if __name__ == "__main__":
    unittest.main()
