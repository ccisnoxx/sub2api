-- S4 匿名聚合与既有计费/V1/V2 独立。由迁移事务原子创建，不回填历史。
-- 应用回退只停用 S4 并保留表及 ledger；部分失败由同一迁移身份重试恢复。
SET LOCAL lock_timeout = '5s';
CREATE TABLE IF NOT EXISTS service_status_config (
 id SMALLINT PRIMARY KEY CHECK (id=1), version BIGINT NOT NULL DEFAULT 1,
 enabled BOOLEAN NOT NULL DEFAULT FALSE, platforms JSONB NOT NULL DEFAULT '["openai"]',
 minimum_samples INTEGER NOT NULL DEFAULT 5 CHECK(minimum_samples BETWEEN 1 AND 10000),
 warning_error_rate DOUBLE PRECISION NOT NULL DEFAULT 0.05 CHECK(warning_error_rate>0 AND warning_error_rate<1),
 outage_error_rate DOUBLE PRECISION NOT NULL DEFAULT 0.9 CHECK(outage_error_rate>warning_error_rate AND outage_error_rate<=1),
 abnormal_windows INTEGER NOT NULL DEFAULT 2 CHECK(abnormal_windows BETWEEN 1 AND 10),
 recovery_windows INTEGER NOT NULL DEFAULT 3 CHECK(recovery_windows BETWEEN 1 AND 10),
 observation_starts JSONB NOT NULL DEFAULT '{}'
);
INSERT INTO service_status_config(id) VALUES(1) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS service_status_facts_1m (
 bucket_start TIMESTAMPTZ NOT NULL, platform TEXT NOT NULL, group_id BIGINT NOT NULL, requested_model TEXT NOT NULL,
 success BIGINT NOT NULL DEFAULT 0, failure BIGINT NOT NULL DEFAULT 0, excluded BIGINT NOT NULL DEFAULT 0, unknown BIGINT NOT NULL DEFAULT 0,
 last_qualified_at TIMESTAMPTZ, last_success_at TIMESTAMPTZ, gap_code TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(bucket_start,platform,group_id,requested_model),
 CHECK(success>=0 AND failure>=0 AND excluded>=0 AND unknown>=0)
);
CREATE TABLE IF NOT EXISTS service_status_scope_states (
 platform TEXT NOT NULL, group_id BIGINT NOT NULL, requested_model TEXT NOT NULL,
 last_evidence_at TIMESTAMPTZ, last_observation_at TIMESTAMPTZ NOT NULL,
 abnormal_count INTEGER NOT NULL DEFAULT 0, recovery_count INTEGER NOT NULL DEFAULT 0,
 config_version BIGINT NOT NULL, observation_start TIMESTAMPTZ NOT NULL,
 last_health TEXT NOT NULL DEFAULT 'unknown', PRIMARY KEY(platform,group_id,requested_model)
);
CREATE TABLE IF NOT EXISTS service_status_incidents (
 id UUID PRIMARY KEY, platform TEXT NOT NULL, group_id BIGINT NOT NULL, requested_model TEXT NOT NULL,
 phase TEXT NOT NULL CHECK(phase IN('detected','ongoing','recovering','awaiting_data','resolved')),
 detected_at TIMESTAMPTZ NOT NULL, last_evidence_at TIMESTAMPTZ NOT NULL, last_abnormal_at TIMESTAMPTZ NOT NULL,
 resolved_at TIMESTAMPTZ, updates JSONB NOT NULL DEFAULT '[]'
);
CREATE UNIQUE INDEX IF NOT EXISTS service_status_incidents_one_open ON service_status_incidents(platform,group_id,requested_model) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS service_status_incidents_resolved ON service_status_incidents(resolved_at DESC,id) WHERE resolved_at IS NOT NULL;
CREATE TABLE IF NOT EXISTS service_status_watermark (
 id SMALLINT PRIMARY KEY CHECK(id=1), observed_through TIMESTAMPTZ, config_version BIGINT NOT NULL DEFAULT 1,
 coverage JSONB NOT NULL DEFAULT '[]', gap_code TEXT NOT NULL DEFAULT '', last_error_code TEXT NOT NULL DEFAULT '', source_error_at TIMESTAMPTZ
);
INSERT INTO service_status_watermark(id) VALUES(1) ON CONFLICT DO NOTHING;

-- 分组可见性由同一数据库事务冻结，短暂关开也不能沿用旧确认计数。
-- 配置共享锁阻止聚合/配置写入，同时允许不同分组的冻结事务并行。
CREATE OR REPLACE FUNCTION service_status_group_visibility_change() RETURNS TRIGGER AS $$
DECLARE
 change_at TIMESTAMPTZ;
 incident RECORD;
 next_updates JSONB;
BEGIN
 IF TG_OP='UPDATE' THEN
  IF (OLD.status='active' AND OLD.deleted_at IS NULL) = (NEW.status='active' AND NEW.deleted_at IS NULL) THEN
   RETURN NEW;
  END IF;
 END IF;
 PERFORM id FROM service_status_config WHERE id=1 FOR SHARE;
 -- 获锁后形成基线，排除锁等待期间产生的旧恢复证据。
 change_at := clock_timestamp();
 UPDATE service_status_scope_states SET abnormal_count=0,recovery_count=0,
  last_evidence_at=GREATEST(last_evidence_at,change_at),last_health='unknown'
  WHERE group_id=OLD.id;
 FOR incident IN SELECT id,phase,updates FROM service_status_incidents WHERE group_id=OLD.id AND resolved_at IS NULL LOOP
  IF incident.phase<>'awaiting_data' THEN
   next_updates:=incident.updates || jsonb_build_array(jsonb_build_object('phase','awaiting_data','at',to_char(change_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')));
   IF jsonb_array_length(next_updates)>32 THEN
    SELECT jsonb_agg(v ORDER BY n) INTO next_updates FROM jsonb_array_elements(next_updates) WITH ORDINALITY AS t(v,n)
     WHERE n=1 OR n>jsonb_array_length(next_updates)-31;
   END IF;
   UPDATE service_status_incidents SET phase='awaiting_data',updates=next_updates WHERE id=incident.id;
  END IF;
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE TRIGGER service_status_group_visibility
 AFTER UPDATE OF status,deleted_at OR DELETE ON groups
 FOR EACH ROW EXECUTE FUNCTION service_status_group_visibility_change();
