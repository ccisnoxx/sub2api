"""检查文档链接与 Quick 外部命令的失败传播。"""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import ci_checks


class QuickChecksTests(unittest.TestCase):
    def test_missing_link_fails_and_deleted_document_is_not_read(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(ci_checks, "ROOT", Path(directory)):
            root = Path(directory)
            for name in ("LICENSE", "NOTICE"):
                (root / name).write_text("归属")
            (root / "README.md").write_text("[入口](missing.md)")
            with self.assertRaisesRegex(ValueError, "链接无效"):
                ci_checks.docs(["README.md"])
            (root / "missing.md").write_text("存在")
            ci_checks.docs(["README.md", "deleted.md"])

    def test_frontend_command_failure_is_not_swallowed(self):
        with tempfile.TemporaryDirectory() as directory:
            executable = Path(directory) / "pnpm"
            executable.write_text('#!/bin/sh\nexit 37\n')
            executable.chmod(0o755)
            env = {**os.environ, "PATH": directory + os.pathsep + os.environ["PATH"],
                   "CI_PLAN": json.dumps({"paths": []})}
            result = subprocess.run([os.sys.executable, ci_checks.__file__, "frontend"], env=env,
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("typecheck", result.stdout)
            self.assertIn("37", result.stderr)


if __name__ == "__main__":
    unittest.main()
