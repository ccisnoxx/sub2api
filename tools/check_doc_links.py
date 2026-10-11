#!/usr/bin/env python3
"""检查公开文档的本地链接；--online 额外检查外部文档 URL。"""

import argparse
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urldefrag, urlsplit


ROOT = Path(__file__).resolve().parents[1]
LINKS = re.compile(r'\]\(([^\s)]+)(?:\s+"[^"]*")?\)|(?:href|src|srcset)="([^"]+)"')


def anchors(text):
    """使用 GitHub 标题锚点规则，保留中文与重复标题的序号。"""
    result = set()
    counts = {}
    for title in re.findall(r"^#{1,6}\s+(.+)$", text, re.MULTILINE):
        slug = re.sub(r"[^\w\- ]", "", title.lower().strip()).replace(" ", "-")
        count = counts.get(slug, 0)
        counts[slug] = count + 1
        result.add(slug if count == 0 else f"{slug}-{count}")
    result.update(re.findall(r'(?:id|name)="([^"]+)"', text))
    return result


def external_status(url):
    # 不使用本机 curl 配置或凭据；只请求文档链接，不执行返回内容。
    command = ["curl", "-q", "--silent", "--show-error", "--location", "--head",
         "--connect-timeout", "10", "--max-time", "30", "--output", "/dev/null",
         "--write-out", "%{http_code}", url]
    result = subprocess.run(command, capture_output=True, text=True)
    if not result.returncode and result.stdout.strip() == "405":
        # 部分站点拒绝 HEAD，改用 GET 确认可读性；其他失败仍如实报告。
        command.remove("--head")
        result = subprocess.run(command, capture_output=True, text=True)
    status = result.stdout.strip()
    if result.returncode or not status.isdigit() or not 200 <= int(status) < 400:
        return f"{url}: HTTP {status or 'unknown'} {result.stderr.strip()}".strip()
    return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--online", action="store_true")
    parser.add_argument("paths", nargs="*", type=Path)
    args = parser.parse_args()
    files = args.paths or sorted([
        *ROOT.glob("README*.md"), *ROOT.glob("docs/**/*.md"),
        *ROOT.glob("deploy/**/*.md"),
    ])
    errors = []
    urls = set()
    local_count = 0
    for file in files:
        content = file.read_text()
        # 围栏内是示例或模板文本，不能作为本地文档路径。
        content = re.sub(r"```.*?```", "", content, flags=re.DOTALL)
        for match in LINKS.finditer(content):
            link = next(value for value in match.groups() if value)
            parsed = urlsplit(link)
            if parsed.scheme in ("http", "https"):
                urls.add(urldefrag(link)[0])
                continue
            if parsed.scheme:
                continue
            local_count += 1
            target = file.parent / unquote(parsed.path) if parsed.path else file
            if not target.is_file():
                errors.append(f"{file.relative_to(ROOT)}: 不存在 {link}")
            elif parsed.fragment and target.suffix == ".md":
                if unquote(parsed.fragment) not in anchors(target.read_text()):
                    errors.append(f"{file.relative_to(ROOT)}: 标题锚点不存在 {link}")
    if args.online:
        with ThreadPoolExecutor(max_workers=6) as pool:
            errors.extend(error for error in pool.map(external_status, sorted(urls)) if error)
    for error in errors:
        print(error)
    print(f"文档 {len(files)} 个，本地链接 {local_count} 个，外部 URL {len(urls)} 个；"
          f"外部检查{'已执行' if args.online else '未执行'}；错误 {len(errors)} 个。")
    return bool(errors)


if __name__ == "__main__":
    raise SystemExit(main())
