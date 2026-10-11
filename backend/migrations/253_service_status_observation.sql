-- S4 源关联元数据只新增可空列；历史 NULL 不回填，应用回退保留列和 ledger。
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS service_status_observation JSONB;
ALTER TABLE ops_error_logs ADD COLUMN IF NOT EXISTS service_status_observation JSONB;
COMMENT ON COLUMN usage_logs.service_status_observation IS '内部真实请求生命周期观察，独立于计费 request_id；历史与未采集为 NULL';
COMMENT ON COLUMN ops_error_logs.service_status_observation IS '内部请求/WS 逻辑 turn 的 attempt 或 terminal 事实；不进入旧 DTO';
