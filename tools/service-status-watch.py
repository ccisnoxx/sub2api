#!/usr/bin/env python3
"""独立于聚合进程的只读检查；连接复用 libpq 环境或 service 文件。"""

import json
import math
import subprocess


WATCH_SQL = """
SELECT json_build_object(
 'enabled',c.enabled, 'has_platforms',jsonb_array_length(c.platforms)>0,
 'epoch_age_seconds',EXTRACT(EPOCH FROM transaction_timestamp()-e.started),
 'watermark_age_seconds',EXTRACT(EPOCH FROM transaction_timestamp()-w.observed_through),
 'aggregation_error',w.last_error_code<>'',
 'source_error',COALESCE(w.source_error_at >= GREATEST(e.started,transaction_timestamp()-interval '10 minutes'),false)
   OR (w.gap_code='source_error' AND w.observed_through>=e.started),
 'configuration_changed',w.config_version<>c.version
)
FROM service_status_config c CROSS JOIN service_status_watermark w
CROSS JOIN LATERAL (SELECT min(value::timestamptz) AS started FROM jsonb_each_text(c.observation_starts)) e
WHERE c.id=1 AND w.id=1
"""
PROBE_SQL = """BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout='2s';
SET LOCAL lock_timeout='1s';
""" + WATCH_SQL + "; COMMIT;"


def evaluate(sample):
    """只返回固定码和布尔状态，不运输原始数据库错误或请求字段。"""
    for key in ("enabled", "has_platforms", "aggregation_error", "source_error", "configuration_changed"):
        if type(sample[key]) is not bool:
            raise ValueError("检查数据无效")
    if not sample["enabled"] or not sample["has_platforms"]:
        return {"event": "service_status_watch", "outcome": "disabled", "alert_codes": []}
    epoch_age = sample["epoch_age_seconds"]
    age = sample["watermark_age_seconds"]
    for value in (epoch_age, age):
        if value is not None and (type(value) not in (int, float) or not math.isfinite(value)):
            raise ValueError("检查数据无效")
    if epoch_age is None:
        raise ValueError("缺少启用区间")
    codes = []
    if age is not None and age < 0:
        codes.append("invalid_watermark")
    elif epoch_age > 180:
        if age is None:
            codes.append("watermark_missing")
        elif age > 180:
            codes.append("watermark_stale")
        if sample["configuration_changed"]:
            codes.append("configuration_changed")
    if sample["aggregation_error"]:
        codes.append("aggregation_error")
    if sample["source_error"]:
        codes.append("source_error")
    return {"event": "service_status_watch", "outcome": "alert" if codes else "current", "alert_codes": codes}


def main():
    try:
        result = subprocess.run(
            ["psql", "-X", "-w", "-qAt", "-v", "ON_ERROR_STOP=1", "-c", PROBE_SQL],
            capture_output=True, text=True, timeout=5, check=True,
        )
        event = evaluate(json.loads(result.stdout))
    except (OSError, subprocess.SubprocessError, ValueError, KeyError, TypeError):
        # stderr 可能包含连接目标或凭据；只输出固定不可用码。
        event = {"event": "service_status_watch", "outcome": "unavailable", "alert_codes": ["watch_unavailable"]}
    print(json.dumps(event, ensure_ascii=False, separators=(",", ":")))
    return 2 if event["outcome"] == "unavailable" else int(bool(event["alert_codes"]))


if __name__ == "__main__":
    raise SystemExit(main())
