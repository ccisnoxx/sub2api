"""执行路径相关的本地/Quick 检查，复用现有测试入口。"""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[2]


def run(*command, cwd=ROOT):
    print("+ " + " ".join(command), flush=True)
    subprocess.run(command, cwd=cwd, check=True)


def docs(paths):
    for name in ("LICENSE", "NOTICE"):
        if not (ROOT / name).is_file() or not (ROOT / name).stat().st_size:
            raise ValueError(f"缺少归属文件：{name}")
    for name in paths:
        file = ROOT / name
        if file.suffix != ".md" or not file.is_file():
            continue
        text = re.sub(r"```.*?```", "", file.read_text(), flags=re.S)
        for target in re.findall(r"!?\[[^\]]*\]\(([^\s)]+)(?:\s+[^)]*)?\)", text):
            link = urlsplit(target.strip("<>"))
            if link.scheme or link.netloc or not link.path:
                continue
            dest = (ROOT if link.path.startswith("/") else file.parent) / unquote(link.path.lstrip("/"))
            if not dest.exists():
                raise ValueError(f"本地链接无效：{name} → {target}")
    print("归属文件与变更 Markdown 的本地链接检查通过；外部链接不在本检查范围。")


def backend(paths):
    packages = sorted({"./" + str(Path(p).parent.relative_to("backend")) for p in paths
                       if p.startswith("backend/") and p.endswith(".go") and (ROOT / p).parent.is_dir()})
    spec = importlib.util.spec_from_file_location("unit_runner", ROOT / "tools/test-backend-unit.py")
    runner = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(runner)
    sharded = {"./internal/" + name: name for name in runner.SHARDED_PACKAGES}
    other = [p for p in packages if p not in sharded]
    if other:
        run("go", "test", "-tags=unit", "-timeout=60s", *other, cwd=ROOT / "backend")
    for package in packages:
        if package in sharded:
            name = sharded[package]
            runner.run_sharded_package(name, package, runner.SHARDED_PACKAGES[name])
    if packages:
        run("go", "vet", *packages, cwd=ROOT / "backend")
    run("go", "build", "./...", cwd=ROOT / "backend")


def frontend(paths):
    files = [p.removeprefix("frontend/") for p in paths if p.startswith("frontend/") and
             Path(p).suffix in {".vue", ".ts", ".js", ".tsx", ".jsx", ".mjs", ".cjs"} and (ROOT / p).is_file()]
    if files:
        run("pnpm", "exec", "eslint", *files, cwd=ROOT / "frontend")
    run("pnpm", "run", "typecheck", cwd=ROOT / "frontend")
    if files:
        run("pnpm", "exec", "vitest", "related", "--run", "--passWithNoTests", *files, cwd=ROOT / "frontend")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("kind", choices=("docs", "backend", "frontend"))
    args = parser.parse_args()
    paths = json.loads(os.environ["CI_PLAN"])["paths"]
    {"docs": docs, "backend": backend, "frontend": frontend}[args.kind](paths)


if __name__ == "__main__":
    main()
