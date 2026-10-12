-- 只读运行诊断；不输出 SQL 正文、请求、模型、分组、账号或连接串。
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout='5s';
SET LOCAL lock_timeout='1s';
SET LOCAL TIME ZONE 'UTC';

SELECT c.enabled,c.version,jsonb_array_length(c.platforms) AS configured_platform_count,
       w.config_version AS watermark_config_version,w.observed_through,
       transaction_timestamp()-w.observed_through AS watermark_age,
       w.gap_code,w.last_error_code,w.source_error_at
FROM service_status_config c CROSS JOIN service_status_watermark w WHERE c.id=1 AND w.id=1;

SELECT a.pid,a.state,a.wait_event_type,a.wait_event,
       transaction_timestamp()-a.xact_start AS transaction_age,pg_blocking_pids(a.pid) AS blocking_pids
FROM pg_stat_activity a WHERE a.datname=current_database() AND a.pid<>pg_backend_pid()
 AND (cardinality(pg_blocking_pids(a.pid))>0 OR a.xact_start<transaction_timestamp()-interval '60 seconds');
SELECT rolsuper OR pg_has_role(current_user,'pg_read_all_stats','USAGE') AS activity_complete
FROM pg_roles WHERE rolname=current_user;

SELECT phase,count(*) AS incident_count,count(*) FILTER (WHERE resolved_at IS NULL) AS unresolved_count
FROM service_status_incidents GROUP BY phase ORDER BY phase;
SELECT last_health,count(*) AS scope_count,max(last_observation_at) AS latest_observation_at
FROM service_status_scope_states GROUP BY last_health ORDER BY last_health;
COMMIT;
