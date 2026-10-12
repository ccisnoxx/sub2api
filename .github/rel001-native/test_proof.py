"""验证外层截止能留出 SIGTERM 清理时间，无法退出时明确失败。"""
import importlib.util
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("proof", Path(__file__).with_name("proof.py"))
proof = importlib.util.module_from_spec(spec)
spec.loader.exec_module(proof)


class HarnessDeadlineTest(unittest.TestCase):
    def test_database_requires_all_real_paths_and_never_exports_log_output(self):
        import json
        names = ['TestUPSTREAM007UpgradePaths' + suffix for suffix in
                 ('/empty', '/old-kin', '/already-personal', '')]
        rows = [{'Action': 'output', 'Output': 'synthetic-private-fixture'},
                *({'Action': 'pass', 'Test': name, 'Elapsed': 1.0} for name in names)]
        encoded = '\n'.join(json.dumps(row) for row in rows)
        self.assertEqual(set(proof.database_result(encoded, 0)['tests']), set(names))
        self.assertNotIn('synthetic-private-fixture', str(proof.database_result(encoded, 0)))
        for invalid, code in ((rows[:-1], 0), (rows, 1),
            ([*rows, {'Action': 'skip', 'Test': names[0]}], 0),
            ([*rows, {'Action': 'fail', 'Test': names[0]}], 0)):
            with self.assertRaises(RuntimeError):
                proof.database_result('\n'.join(json.dumps(row) for row in invalid), code)

    def test_missing_diagnostics_does_not_block_resource_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            private = root / "rel001-perf-aabbcc"
            private.mkdir()
            secret = private / "fixture.env"
            secret.write_text("synthetic")
            containers = {"rel001-perf-aabbcc-app", "rel001-perf-aabbcc-postgres"}
            networks = {"rel001-perf-aabbcc-net"}
            def docker(*args, **kwargs):
                if args[1:3] == ("ps", "-a"):
                    return "\n".join(sorted(containers))
                if args[1:3] == ("rm", "-fv"):
                    containers.remove(args[-1])
                    return ""
                if args[1:3] == ("network", "ls"):
                    return "\n".join(networks)
                if args[1:3] == ("network", "rm"):
                    networks.remove(args[-1])
                    return ""
                self.fail("unexpected command")
            report = {"runtime_oom_lines": []}
            with patch.object(proof, "command", side_effect=docker), patch.object(proof.subprocess, "run",
                    side_effect=proof.subprocess.TimeoutExpired("docker logs", 15)):
                proof.cleanup(report, root)
            self.assertEqual(report["diagnostics_failure_code"], "docker_logs_unavailable")
            self.assertFalse(containers or networks or secret.exists())

    def test_timeout_allows_signal_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            marker = Path(directory) / "cleaned"
            script = Path(directory) / "child.py"
            script.write_text("import signal, sys\nfrom pathlib import Path\n"
                "def cleanup(*args):\n Path(sys.argv[1]).write_text('cleaned'); sys.exit(1)\n"
                "signal.signal(signal.SIGTERM, cleanup)\nsignal.pause()\n")
            with self.assertRaisesRegex(RuntimeError, "^harness_timeout$"):
                proof.execute_harness([sys.executable, str(script), str(marker)], timeout=2, grace=2)
            self.assertEqual(marker.read_text(), "cleaned")

    def test_unresponsive_child_is_killed_and_fails(self):
        with tempfile.TemporaryDirectory() as directory:
            script = Path(directory) / "child.py"
            script.write_text("import signal\nsignal.signal(signal.SIGTERM, signal.SIG_IGN)\nsignal.pause()\n")
            with self.assertRaisesRegex(RuntimeError, "^harness_hard_timeout$"):
                proof.execute_harness([sys.executable, str(script)], timeout=2, grace=0.1)


if __name__ == "__main__":
    unittest.main()
