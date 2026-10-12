"""水位停滞、重新启用、源错误和故障隐私的可观察合同。"""

import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("status_watch", Path(__file__).with_name("service-status-watch.py"))
watch = importlib.util.module_from_spec(spec)
spec.loader.exec_module(watch)


class StatusWatchTest(unittest.TestCase):
    def sample(self):
        return dict(enabled=True, has_platforms=True, epoch_age_seconds=600,
                    watermark_age_seconds=60, aggregation_error=False,
                    source_error=False, configuration_changed=False)

    def test_stopped_aggregation_and_recovery(self):
        sample = self.sample()
        for age, expected in [(180, []), (181, ["watermark_stale"]), (240, ["watermark_stale"]), (30, [])]:
            with self.subTest(age=age):
                sample["watermark_age_seconds"] = age
                self.assertEqual(watch.evaluate(sample)["alert_codes"], expected)

    def test_missing_future_epoch_and_disabled(self):
        cases = [({"watermark_age_seconds": None}, ["watermark_missing"]),
                 ({"watermark_age_seconds": -1}, ["invalid_watermark"]),
                 ({"epoch_age_seconds": -60, "watermark_age_seconds": None}, []),
                 ({"epoch_age_seconds": 180, "watermark_age_seconds": 900}, []),
                 ({"epoch_age_seconds": 181, "watermark_age_seconds": 900}, ["watermark_stale"]),
                 ({"enabled": False, "source_error": True, "aggregation_error": True}, []),
                 ({"has_platforms": False, "watermark_age_seconds": None}, [])]
        for changes, expected in cases:
            with self.subTest(changes=changes):
                sample = self.sample(); sample.update(changes)
                self.assertEqual(watch.evaluate(sample)["alert_codes"], expected)

    def test_errors_are_not_masked_by_stale_watermark(self):
        sample = self.sample()
        sample.update(watermark_age_seconds=240, source_error=True, aggregation_error=True, configuration_changed=True)
        self.assertEqual(watch.evaluate(sample)["alert_codes"],
                         ["watermark_stale", "configuration_changed", "aggregation_error", "source_error"])

    def test_process_failures_do_not_leak_stderr(self):
        errors = [FileNotFoundError("private-path"), subprocess.TimeoutExpired("psql", 5, stderr="secret"),
                  subprocess.CalledProcessError(1, "psql", stderr="private-connection")]
        for error in errors:
            with self.subTest(error=type(error).__name__), patch.object(watch.subprocess, "run", side_effect=error):
                output = io.StringIO()
                with contextlib.redirect_stdout(output):
                    self.assertEqual(watch.main(), 2)
                self.assertEqual(json.loads(output.getvalue())["alert_codes"], ["watch_unavailable"])
                self.assertNotIn("private", output.getvalue()); self.assertNotIn("secret", output.getvalue())

    def test_main_exit_status_and_invalid_results(self):
        for raw, expected in [(json.dumps(self.sample()), 0), ("", 2), ("{}", 2), ("null", 2)]:
            with self.subTest(raw=raw), patch.object(watch.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, raw)):
                with contextlib.redirect_stdout(io.StringIO()):
                    self.assertEqual(watch.main(), expected)
        sample = self.sample(); sample["source_error"] = True
        with patch.object(watch.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, json.dumps(sample))):
            with contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(watch.main(), 1)
        for value in (True, float("nan"), "60"):
            sample = self.sample(); sample["watermark_age_seconds"] = value
            with self.assertRaises(ValueError):
                watch.evaluate(sample)


if __name__ == "__main__":
    unittest.main()
