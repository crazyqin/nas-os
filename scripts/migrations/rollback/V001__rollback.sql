-- Rollback: Initial Schema
-- Version: 001
-- WARNING: This will drop all tables and data!

-- Drop tables in reverse order of creation
DROP INDEX IF EXISTS idx_audit_logs_created;
DROP INDEX IF EXISTS idx_audit_logs_user;
DROP INDEX IF EXISTS idx_backup_jobs_active;
DROP INDEX IF EXISTS idx_snapshots_pool;
DROP INDEX IF EXISTS idx_shares_name;
DROP INDEX IF EXISTS idx_storage_pools_name;
DROP INDEX IF EXISTS idx_sessions_expires;
DROP INDEX IF EXISTS idx_sessions_token;
DROP INDEX IF EXISTS idx_users_username;

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS backup_jobs;
DROP TABLE IF EXISTS snapshots;
DROP TABLE IF EXISTS shares;
DROP TABLE IF EXISTS storage_pools;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS system_config;