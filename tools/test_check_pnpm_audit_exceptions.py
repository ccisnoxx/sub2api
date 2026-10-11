#!/usr/bin/env python3
import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from datetime import date, timedelta


CHECKER = Path(__file__).with_name("check_pnpm_audit_exceptions.py")
GHSA = "GHSA-4r6h-8v6p-xvw6"


class AuditExceptionPolicyTest(unittest.TestCase):
    def setUp(self):
        self.audit = {
            "metadata": {"vulnerabilities": {"high": 1, "critical": 0}},
            "advisories": {"1": {
                "module_name": "xlsx", "severity": "high", "github_advisory_id": GHSA,
                "findings": [{"version": "0.18.5", "paths": [". > xlsx@0.18.5"]}],
            }},
        }
        self.exception = {
            "package": "xlsx", "advisory": GHSA, "severity": "high",
            "reason": "只生成管理员用量工作簿，不读入文件。", "mitigation": "静态限制写出 API。",
            "owner": "@ccisnoxx", "evidence": "docs/security/SEC-002.md",
            "expires_on": (date.today() + timedelta(days=30)).isoformat(),
            "affected_version": "0.18.5", "dependency_path": ". > xlsx@0.18.5",
        }

    def run_checker(self, audit=None, exceptions=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "audit.json").write_text(json.dumps(self.audit if audit is None else audit))
            items = [self.exception] if exceptions is None else exceptions
            lines = ["version: 1", "exceptions:"]
            for item in items:
                lines.extend(f"{'  -' if i == 0 else '   '} {key}: {json.dumps(value, ensure_ascii=False)}"
                             for i, (key, value) in enumerate(item.items()))
            (root / "exceptions.yml").write_text("\n".join(lines), encoding="utf-8")
            return subprocess.run(
                [sys.executable, str(CHECKER), "--audit", str(root / "audit.json"),
                 "--exceptions", str(root / "exceptions.yml")],
                capture_output=True, text=True, check=False,
            )

    def test_exact_valid_exception_passes(self):
        result = self.run_checker()
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_future_advisory_on_same_package_is_not_ignored(self):
        self.audit["advisories"]["1"]["github_advisory_id"] = "GHSA-aaaa-bbbb-cccc"
        self.assertNotEqual(self.run_checker().returncode, 0)

    def test_version_and_all_paths_must_match(self):
        for finding in [
            {"version": "0.18.4", "paths": [". > xlsx@0.18.5"]},
            {"version": "0.18.5", "paths": [". > xlsx@0.18.5", ". > other > xlsx@0.18.5"]},
            {"version": "0.18.5", "paths": []},
        ]:
            with self.subTest(finding=finding):
                self.audit["advisories"]["1"]["findings"] = [finding]
                self.assertNotEqual(self.run_checker().returncode, 0)

    def test_repeated_advisory_does_not_hide_another_path(self):
        self.audit["advisories"]["2"] = copy.deepcopy(self.audit["advisories"]["1"])
        self.audit["advisories"]["2"]["findings"][0]["paths"] = [". > other > xlsx@0.18.5"]
        self.assertNotEqual(self.run_checker().returncode, 0)

    def test_severity_change_requires_review(self):
        self.audit["advisories"]["1"]["severity"] = "critical"
        self.assertNotEqual(self.run_checker().returncode, 0)

    def test_missing_owner_reason_or_evidence_fails(self):
        for field in ("owner", "reason", "evidence", "mitigation", "expires_on"):
            with self.subTest(field=field):
                item = {k: v for k, v in self.exception.items() if k != field}
                self.assertNotEqual(self.run_checker(exceptions=[item]).returncode, 0)

    def test_placeholder_owner_and_broad_scopes_fail(self):
        for field, value in [
            ("owner", "security@your-domain"), ("owner", "@your-domain"),
            ("advisory", "*"), ("affected_version", "*"), ("dependency_path", "*"),
        ]:
            with self.subTest(field=field, value=value):
                self.assertNotEqual(self.run_checker(exceptions=[{**self.exception, field: value}]).returncode, 0)

    def test_expired_exception_fails_even_without_current_vulnerability(self):
        self.exception["expires_on"] = (date.today() - timedelta(days=1)).isoformat()
        self.assertNotEqual(self.run_checker().returncode, 0)
        clean = {"advisories": {}, "metadata": {"vulnerabilities": {"high": 0, "critical": 0}}}
        self.assertNotEqual(self.run_checker(audit=clean).returncode, 0)

    def test_expiry_date_is_valid_through_that_day(self):
        self.exception["expires_on"] = date.today().isoformat()
        self.assertEqual(self.run_checker().returncode, 0)

    def test_invalid_date_and_duplicate_exception_fail(self):
        self.assertNotEqual(self.run_checker(exceptions=[{**self.exception, "expires_on": "not-a-date"}]).returncode, 0)
        self.assertNotEqual(self.run_checker(exceptions=[self.exception, self.exception]).returncode, 0)

    def test_endpoint_error_or_unknown_report_is_not_a_clean_audit(self):
        for report in [
            {"error": {"code": "ERR_PNPM_AUDIT_ENDPOINT_NOT_EXISTS"}}, {}, [],
            {"advisories": {}},
            {"advisories": {}, "metadata": {"vulnerabilities": {"high": 1, "critical": 0}}},
        ]:
            with self.subTest(report=report):
                self.assertNotEqual(self.run_checker(audit=report, exceptions=[]).returncode, 0)

    def test_clean_audit_needs_no_exception(self):
        report = {"advisories": {}, "metadata": {"vulnerabilities": {"high": 0, "critical": 0}}}
        self.assertEqual(self.run_checker(audit=report, exceptions=[]).returncode, 0)


if __name__ == "__main__":
    unittest.main()
