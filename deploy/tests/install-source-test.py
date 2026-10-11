#!/usr/bin/env python3
"""从实际安装入口验证下载来源和镜像选择，不访问网络或运行部署。"""

import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]


class InstallSourceTest(unittest.TestCase):
    def test_documented_compose_commands_keep_image_override_last(self):
        for name in ("README.md", "README_CN.md", "README_JA.md", "deploy/README.md"):
            for line in (ROOT / name).read_text().splitlines():
                if line.startswith("docker compose "):
                    with self.subTest(document=name, command=line):
                        parts = line.split()
                        files = []
                        index = 2
                        while index < len(parts) and parts[index] == "-f":
                            files.append(parts[index + 1])
                            index += 2
                        self.assertEqual(len(files), 2)
                        self.assertIn(files[0], ("docker-compose.yml", "docker-compose.local.yml"))
                        self.assertEqual(files[-1], "compose.personal.yml")

    def test_installer_release_lookup_uses_fork(self):
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            curl = temp / "curl"
            curl.write_text('#!/bin/sh\nprintf "%s\\n" "$@" > "$REQUEST_LOG"\n'
                            'printf \'{"tag_name":"v0.2.14-klno.5-tps.2"}\\n\'\n')
            curl.chmod(0o755)
            env = dict(os.environ, PATH=f"{temp}:{os.environ['PATH']}", REQUEST_LOG=str(temp / "request"))
            subprocess.run(["bash", "-c", "source <(sed '$d' \"$1\"); get_latest_version",
                            "bash", str(ROOT / "deploy/install.sh")], env=env, check=True, capture_output=True)
            request = (temp / "request").read_text()
            self.assertIn("https://api.github.com/repos/ccisnoxx/sub2api/releases/latest", request)
            self.assertNotIn("Wei-Shaw", request)

    def test_preparation_downloads_fork_templates(self):
        with tempfile.TemporaryDirectory() as directory:
            temp = Path(directory)
            curl = temp / "curl"
            curl.write_text('''#!/bin/sh
printf '%s\n' "$@" >> "$REQUEST_LOG"
url=$2
case "$url" in
  https://raw.githubusercontent.com/ccisnoxx/sub2api/main/deploy/docker-compose.local.yml) cp "$SOURCE_ROOT/deploy/docker-compose.local.yml" "$4" ;;
  https://raw.githubusercontent.com/ccisnoxx/sub2api/main/deploy/.env.example) cp "$SOURCE_ROOT/deploy/.env.example" "$4" ;;
  *) exit 1 ;;
esac
''')
            curl.chmod(0o755)
            env = dict(os.environ, PATH=f"{temp}:{os.environ['PATH']}", REQUEST_LOG=str(temp / "request"),
                       SOURCE_ROOT=str(ROOT))
            # 准备器生成临时凭据，捕获输出且不写入测试日志。
            subprocess.run(["bash", str(ROOT / "deploy/docker-deploy.sh")], cwd=temp, env=env,
                           check=True, capture_output=True)
            self.assertNotIn("Wei-Shaw", (temp / "request").read_text())
            self.assertIn("ghcr.io/ccisnoxx/sub2api", (temp / "docker-compose.yml").read_text())
            self.assertIn("SUB2API_IMAGE=ghcr.io/ccisnoxx/sub2api@sha256:", (temp / ".env").read_text())
            self.assertEqual((temp / ".env").stat().st_mode & 0o777, 0o600)


if __name__ == "__main__":
    unittest.main()
