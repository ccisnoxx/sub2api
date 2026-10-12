"""验证工作流的远程 Action 固定策略，防止更新重新引入浮动引用。"""

from pathlib import Path
import re
import unittest

import yaml


REMOTE_PIN = re.compile(r"[\w.-]+/[\w.-]+(?:/[\w./-]+)?@[0-9a-f]{40}\Z")
VERSION_COMMENT = re.compile(r"\s+#\s+v\d+\.\d+\.\d+\s*$")


def pin_errors(text):
    lines = text.splitlines()
    errors = []

    def visit(node):
        if isinstance(node, yaml.MappingNode):
            for key, value in node.value:
                if key.value == "uses":
                    line = value.start_mark.line
                    reference = value.value
                    # 本地 Action 和可复用工作流随本次 checkout 的源码固定。
                    if not reference.startswith("./"):
                        if not REMOTE_PIN.fullmatch(reference):
                            errors.append(f"第 {line + 1} 行：远程 Action 必须固定到完整 SHA")
                        if not VERSION_COMMENT.search(lines[line]):
                            errors.append(f"第 {line + 1} 行：缺少同一行的精确版本注释")
                visit(value)
        elif isinstance(node, yaml.SequenceNode):
            for value in node.value:
                visit(value)

    visit(yaml.compose(text))
    return errors


class ActionPinTests(unittest.TestCase):
    def test_repository_workflows_are_pinned(self):
        directory = Path(__file__).resolve().parents[1] / "workflows"
        workflows = sorted([*directory.glob("*.yml"), *directory.glob("*.yaml")])
        self.assertTrue(workflows, "没有找到工作流")
        for path in workflows:
            with self.subTest(workflow=path.name):
                self.assertEqual(pin_errors(path.read_text()), [])

    def test_floating_tags_and_branches_are_rejected(self):
        for reference in ("v6", "v6.1.0", "main"):
            with self.subTest(reference=reference):
                self.assertTrue(pin_errors(f"uses: actions/checkout@{reference} # v6.1.0\n"))

    def test_short_sha_is_rejected(self):
        self.assertTrue(pin_errors("uses: actions/checkout@1234567 # v6.1.0\n"))

    def test_version_comment_is_required_on_same_line(self):
        reference = "actions/checkout@" + "a" * 40
        self.assertTrue(pin_errors(f"# v6.1.0\nuses: {reference}\n"))

    def test_local_workflows_and_pinned_subactions_are_accepted(self):
        text = ("jobs:\n  local:\n    uses: ./.github/workflows/backend-ci.yml\n"
                "  remote:\n    steps:\n      - uses: actions/cache/restore@"
                + "a" * 40 + " # v6.1.0\n")
        self.assertEqual(pin_errors(text), [])


if __name__ == "__main__":
    unittest.main()
