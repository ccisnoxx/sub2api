-- 只读容量证据；不输出用户、账号、请求、模型或分组标识。
-- 使用 psql -X -v ON_ERROR_STOP=1 -f tools/service-status-capacity.sql；连接参数由环境或 service 文件提供。
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL statement_timeout = '5s';
SET LOCAL lock_timeout = '1s';
SET LOCAL TIME ZONE 'UTC';

SELECT clock_timestamp() AS sampled_at, current_setting('server_version') AS postgres_version,
       enabled, observed_through, last_error_code,
       clock_timestamp()-observed_through AS watermark_age
FROM service_status_config CROSS JOIN service_status_watermark;

SELECT relname AS table_name, pg_table_size(relid) AS table_bytes,
       pg_indexes_size(relid) AS index_bytes, pg_total_relation_size(relid) AS total_bytes,
       n_live_tup AS live_tuples_estimate, n_dead_tup AS dead_tuples_estimate,
       last_autovacuum, autovacuum_count, last_autoanalyze
FROM pg_stat_user_tables
WHERE relid IN ('service_status_facts_1m'::regclass,'service_status_scope_states'::regclass,
               'service_status_incidents'::regclass,'service_status_config'::regclass,'service_status_watermark'::regclass)
ORDER BY relname;

-- 逻辑日行数：十分钟重算会反复 DELETE/INSERT，n_tup_ins 不能充当每日新增事实数。
SELECT date_trunc('day',bucket_start) AS utc_day, count(*) AS logical_fact_rows,
       min(bucket_start) AS first_bucket, max(bucket_start) AS last_bucket
FROM service_status_facts_1m
WHERE bucket_start >= date_trunc('day',CURRENT_TIMESTAMP)-interval '31 days'
  AND bucket_start < date_trunc('minute',CURRENT_TIMESTAMP)
GROUP BY 1 ORDER BY 1;

SELECT count(*) AS retained_fact_rows, min(bucket_start) AS first_bucket, max(bucket_start) AS last_bucket,
       round(avg(pg_column_size(f)),2) AS mean_tuple_bytes,
       count(*) FILTER (WHERE bucket_start < CURRENT_TIMESTAMP-interval '31 days') AS expired_fact_rows
FROM service_status_facts_1m f;

SELECT max(scope_rows) AS peak_observed_scopes_per_minute
FROM (SELECT count(*) AS scope_rows FROM service_status_facts_1m GROUP BY bucket_start) minutes;

SELECT count(*) FILTER (WHERE xact_start < clock_timestamp()-interval '60 seconds') AS long_transactions,
       (SELECT rolsuper OR pg_has_role(current_user,'pg_read_all_stats','MEMBER')
        FROM pg_roles WHERE rolname=current_user) AS activity_complete
FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid();
SELECT stats_reset FROM pg_stat_database WHERE datname=current_database();
COMMIT;
