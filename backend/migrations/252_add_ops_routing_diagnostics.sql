-- 仅保存本次历史选择快照；既有记录与旧写入者保持 SQL NULL，不默认或回填统计。
-- 应用回退保留扩展列与迁移记录，旧应用可继续显式列读写；恢复新应用可继续读取已有快照。
ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS routing_diagnostics JSONB NULL;
