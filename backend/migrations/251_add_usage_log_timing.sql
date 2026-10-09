-- 新计时和终态只追加观测字段；不回填历史时点，不改旧首字设置或聚合。
-- 迁移由 runner 的事务原子应用；锁等待失败时回滚，随后按同一身份重试。
-- 应用回退保留这些列及 migration ledger，不执行删列回滚。
SET LOCAL lock_timeout = '5s';

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS timing_version SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS strict_first_token_ms INTEGER,
    ADD COLUMN IF NOT EXISTS last_token_ms INTEGER,
    ADD COLUMN IF NOT EXISTS first_output_ms INTEGER,
    ADD COLUMN IF NOT EXISTS first_output_kind VARCHAR(16),
    ADD COLUMN IF NOT EXISTS audio_output_tokens INTEGER,
    ADD COLUMN IF NOT EXISTS completion_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS is_complete BOOLEAN,
    ADD COLUMN IF NOT EXISTS usage_source VARCHAR(32) NOT NULL DEFAULT 'unknown';

COMMENT ON COLUMN usage_logs.timing_version IS '0=历史或未接入采集器；1=已验证的新计时口径';
COMMENT ON COLUMN usage_logs.strict_first_token_ms IS '首个有效文本、推理或工具输出的相对毫秒数；独立于旧 first_token_ms';
COMMENT ON COLUMN usage_logs.last_token_ms IS '同一起点下最后一个有效 Token 类输出的时点';
COMMENT ON COLUMN usage_logs.first_output_ms IS '首个有效输出时点，包括可识别的媒体输出';
COMMENT ON COLUMN usage_logs.first_output_kind IS 'text/reasoning/tool/image/audio/compaction；NULL=未观察到';
COMMENT ON COLUMN usage_logs.audio_output_tokens IS '可信音频输出 Token；NULL=未知，0=确认没有音频';
COMMENT ON COLUMN usage_logs.completion_status IS 'unknown/completed/client_disconnected/upstream_error/interrupted';
COMMENT ON COLUMN usage_logs.is_complete IS '已观察终态的完整性；NULL=未知';
COMMENT ON COLUMN usage_logs.usage_source IS 'unknown/upstream_final/upstream_partial；只描述既有 Token 来源，不改变计费';
