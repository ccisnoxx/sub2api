"""验证同一 ref 的手动 Quick 不会取消正在运行的 Full。"""

from pathlib import Path
import re
from types import SimpleNamespace
import unittest

import yaml


class CIConcurrencyTests(unittest.TestCase):
    def test_manual_quick_and_full_use_distinct_groups(self):
        workflow = yaml.load((Path(__file__).resolve().parents[1] / "workflows/personal-ci.yml").read_text(),
                             Loader=yaml.BaseLoader)
        def group(ref, pr, mode):
            context = {"github": SimpleNamespace(ref=ref),
                       "inputs": SimpleNamespace(pr_number=pr, mode=mode)}
            def interpolate(match):
                expression = match.group(1).replace("||", " or ").strip()
                return str(eval(expression, {"__builtins__": {}}, context))
            return re.sub(r"\$\{\{(.*?)\}\}", interpolate, workflow["concurrency"]["group"])
        for ref, pr in (("refs/heads/personal", ""), ("refs/heads/codex/example", "12")):
            with self.subTest(ref=ref, pr=pr):
                self.assertNotEqual(group(ref, pr, "quick"), group(ref, pr, "full"))
                self.assertEqual(group(ref, pr, "quick"), group(ref, pr, "quick"))


if __name__ == "__main__":
    unittest.main()
