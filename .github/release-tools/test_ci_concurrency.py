"""对实际并发分组表达式验证同步证据门禁不会取消显式 Full。"""

from pathlib import Path
import re
from types import SimpleNamespace
import unittest

import yaml


class CIConcurrencyTests(unittest.TestCase):
    def test_sync_auto_events_and_explicit_full_use_distinct_groups(self):
        workflow = yaml.load((Path(__file__).resolve().parents[1] / "workflows/personal-ci.yml").read_text(),
                             Loader=yaml.BaseLoader)
        expression = re.findall(r"\$\{\{(.*?)\}\}", workflow["concurrency"]["group"])[-1]
        # 此表达式只使用 Actions 与 Python 一致的短路布尔/字符串运算和 startsWith。
        expression = re.sub(r"!(?!=)", "not ", expression).replace("&&", " and ").replace("||", " or ")
        cases = [("pull_request", "ready_for_review", "codex/sync-klno-v1", "", "", "quick"),
                 ("pull_request", "labeled", "codex/sync-klno-v1", "full-ci", "", "quick"),
                 ("pull_request", "ready_for_review", "codex/verify-personal-ci-v1", "", "", "quick"),
                 ("pull_request", "labeled", "codex/verify-personal-ci-v1", "full-ci", "", "quick"),
                 ("workflow_dispatch", "", "", "", "full", "full"),
                 ("workflow_dispatch", "", "", "", "quick", "quick"),
                 ("pull_request", "ready_for_review", "codex/feature", "", "", "full"),
                 ("pull_request", "synchronize", "codex/feature", "", "", "quick"),
                 ("push", "", "", "", "", "full")]
        for event, action, head, label, mode, expected in cases:
            with self.subTest(event=event, action=action, head=head):
                context = {"github": SimpleNamespace(event_name=event, head_ref=head,
                           event=SimpleNamespace(action=action, label=SimpleNamespace(name=label))),
                           "inputs": SimpleNamespace(mode=mode), "startsWith": str.startswith}
                self.assertEqual(eval(expression, {"__builtins__": {}}, context), expected)


if __name__ == "__main__":
    unittest.main()
