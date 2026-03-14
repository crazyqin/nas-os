-- Migration: Add Metrics and Monitoring Tables
-- Version: 002
-- Created: 2026-03-15T00:00:00+08:00

-- 系统指标历史表
CREATE TABLE IF NOT EXISTS system_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric_type TEXT NOT NULL,
    metric_name TEXT NOT NULL,
    value REAL NOT NULL,
    unit TEXT,
    labels TEXT,
    collected_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 服务状态表
CREATE TABLE IF NOT EXISTS service_status (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    service_name TEXT NOT NULL,
    status TEXT NOT NULL,
    message TEXT,
    last_check DATETIME DEFAULT CURRENT_TIMESTAMP,
    uptime_seconds INTEGER DEFAULT 0
);

-- 告警规则表
CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    metric_type TEXT NOT NULL,
    condition TEXT NOT NULL,
    threshold REAL NOT NULL,
    duration_seconds INTEGER DEFAULT 60,
    severity TEXT DEFAULT 'warning',
    is_enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 告警历史表
CREATE TABLE IF NOT EXISTS alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER,
    message TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT DEFAULT 'firing',
    fired_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at DATETIME,
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_metrics_type ON system_metrics(metric_type);
CREATE INDEX IF NOT EXISTS idx_metrics_time ON system_metrics(collected_at);
CREATE INDEX IF NOT EXISTS idx_service_status_name ON service_status(service_name);
CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(is_enabled);
CREATE INDEX IF NOT EXISTS idx_alert_history_status ON alert_history(status);
CREATE INDEX IF NOT EXISTS idx_alert_history_fired ON alert_history(fired_at);

-- 插入默认告警规则
INSERT OR IGNORE INTO alert_rules (name, metric_type, condition, threshold, severity) VALUES
    ('high_cpu_usage', 'cpu', 'greater_than', 80, 'warning'),
    ('critical_cpu_usage', 'cpu', 'greater_than', 95, 'critical'),
    ('high_memory_usage', 'memory', 'greater_than', 85, 'warning'),
    ('critical_memory_usage', 'memory', 'greater_than', 95, 'critical'),
    ('disk_space_low', 'disk', 'greater_than', 85, 'warning'),
    ('disk_space_critical', 'disk', 'greater_than', 95, 'critical');