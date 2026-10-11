#!/usr/bin/env python3
"""核对实际工具版本，将源码、依赖摘要和工具版本保存为构建元数据。"""

import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess


POLICY = Path(__file__).with_name("toolchain.json")
COMMANDS = {
    "node": (["node", "--version"], r"v(\d+\.\d+\.\d+)"),
    "pnpm": (["pnpm", "--version"], r"(\d+\.\d+\.\d+)"),
    "go": (["go", "env", "GOVERSION"], r"go(\d+\.\d+\.\d+)"),
    "goreleaser": (["goreleaser", "--version"], r"GitVersion:\s*v?(\d+\.\d+\.\d+)"),
    "buildx": (["docker", "buildx", "version"], r"(v\d+\.\d+\.\d+)"),
}


def output(command):
    return subprocess.check_output(command, text=True).strip()


def installed_version(tool):
    if tool == "python":
        return platform.python_version()
    if tool == "pyyaml":
        return importlib.metadata.version("PyYAML")
    if tool == "govulncheck":
        # Go 的构建信息记录安装模块的精确版本，不依赖扫描输出格式。
        executable = shutil.which("govulncheck")
        if executable is None:
            raise FileNotFoundError("找不到 govulncheck")
        text = output(["go", "version", "-m", executable])
        pattern = r"mod\s+golang.org/x/vuln\s+(v\d+\.\d+\.\d+)"
    else:
        command, pattern = COMMANDS[tool]
        text = output(command)
    match = re.search(pattern, text)
    if not match:
        raise ValueError(f"无法读取 {tool} 的精确版本：{text}")
    return match[1]


def report(tools, destination):
    pins = json.loads(POLICY.read_text())
    actual = {}
    for tool in tools:
        expected = pins.get(tool)
        if tool == "go":
            expected = re.search(r"^go (\d+\.\d+\.\d+)$", Path("backend/go.mod").read_text(), re.M)[1]
        version = installed_version(tool)
        if version != expected:
            raise ValueError(f"{tool} 版本不一致：预期 {expected}，实际 {version}")
        actual[tool] = version
        print(f"{tool}: {version}")
    inputs = [Path("backend/go.mod"), Path("backend/go.sum"),
              Path("frontend/package.json"), Path("frontend/pnpm-lock.yaml"),
              POLICY, POLICY.with_name("requirements-release.txt")]
    # 只记录白名单输入的哈希，不导出环境变量、配置内容或完整日志。
    metadata = {
        "source_sha": output(["git", "rev-parse", "HEAD"]),
        "workflow_sha": os.environ.get("GITHUB_SHA"),
        "run_id": os.environ.get("GITHUB_RUN_ID"),
        "platform": {"system": platform.system(), "machine": platform.machine()},
        "configured": pins,
        "actual": actual,
        "inputs_sha256": {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in inputs},
    }
    Path(destination).write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--tools", nargs="+", choices=[*COMMANDS, "python", "pyyaml", "govulncheck"], required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    report(args.tools, args.output)


if __name__ == "__main__":
    main()
