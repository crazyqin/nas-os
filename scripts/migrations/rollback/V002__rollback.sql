-- Rollback: Metrics Tables
-- Version: 002
-- WARNING: This will drop metrics tables

DROP INDEX IF EXISTS idx_metrics_timestamp;
DROP INDEX IF EXISTS idx_metrics_name;
DROP INDEX IF EXISTS idx_alerts_acknowledged;
DROP INDEX IF EXISTS idx_alerts_severity;

DROP TABLE IF EXISTS alert_history;
DROP TABLE IF EXISTS system_metrics;