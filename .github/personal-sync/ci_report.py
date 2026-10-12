"""只读统计 Actions；墙钟耗时不是 runner 计费时间，运行中结果单独列出。"""

import argparse
from collections import Counter
from datetime import datetime
import json
import statistics
import subprocess

from sync import REPOSITORY


def duration(row):
    return (datetime.fromisoformat(row["updatedAt"].replace("Z", "+00:00")) -
            datetime.fromisoformat(row["createdAt"].replace("Z", "+00:00"))).total_seconds() / 60


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--limit", type=int, default=20)
    parser.add_argument("--workflow", default="personal-ci.yml")
    args = parser.parse_args()
    command = ["gh", "run", "list", "--repo", REPOSITORY, "--limit", str(args.limit),
               "--json", "databaseId,name,event,status,conclusion,createdAt,updatedAt,headSha,headBranch,url"]
    if args.workflow:
        command.extend(["--workflow", args.workflow])
    rows = json.loads(subprocess.run(command, check=True, text=True, capture_output=True).stdout)
    successful = [duration(row) for row in rows if row["conclusion"] == "success"]
    print(json.dumps({"counts": dict(Counter(row["conclusion"] or row["status"] for row in rows)),
                      "successful_median_minutes": round(statistics.median(successful), 2) if successful else None,
                      "runs": [{**row, "wall_minutes": round(duration(row), 2),
                                "duration_final": row["status"] == "completed"} for row in rows]},
                     ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
