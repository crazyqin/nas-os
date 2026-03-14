-- Migration: Initial Schema
-- Version: 001
-- Created: 2026-03-15T00:00:00+08:00

-- 系统配置表
CREATE TABLE IF NOT EXISTS system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    type TEXT DEFAULT 'string',
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    email TEXT,
    role TEXT DEFAULT 'user',
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);

-- 会话表
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token TEXT UNIQUE NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 存储池表
CREATE TABLE IF NOT EXISTS storage_pools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    path TEXT NOT NULL,
    type TEXT DEFAULT 'directory',
    quota_bytes INTEGER,
    used_bytes INTEGER DEFAULT 0,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 共享表
CREATE TABLE IF NOT EXISTS shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    path TEXT NOT NULL,
    type TEXT DEFAULT 'smb',
    is_public INTEGER DEFAULT 0,
    is_readonly INTEGER DEFAULT 0,
    comment TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 快照表
CREATE TABLE IF NOT EXISTS snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    path TEXT NOT NULL,
    pool_id INTEGER,
    size_bytes INTEGER DEFAULT 0,
    is_automated INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (pool_id) REFERENCES storage_pools(id) ON DELETE SET NULL
);

-- 备份任务表
CREATE TABLE IF NOT EXISTS backup_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    source_path TEXT NOT NULL,
    destination TEXT NOT NULL,
    schedule TEXT,
    retention_days INTEGER DEFAULT 30,
    last_run DATETIME,
    next_run DATETIME,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    action TEXT NOT NULL,
    resource TEXT,
    details TEXT,
    ip_address TEXT,
    user_agent TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_storage_pools_name ON storage_pools(name);
CREATE INDEX IF NOT EXISTS idx_shares_name ON shares(name);
CREATE INDEX IF NOT EXISTS idx_snapshots_pool ON snapshots(pool_id);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_active ON backup_jobs(is_active);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON audit_logs(created_at);

-- 插入默认管理员
INSERT OR IGNORE INTO users (username, password_hash, role)
VALUES ('admin', '$2a$10$YourHashedPasswordHere', 'admin');

-- 插入默认配置
INSERT OR IGNORE INTO system_config (key, value, type, description) VALUES
    ('system.name', 'NAS-OS', 'string', '系统名称'),
    ('system.version', '2.36.0', 'string', '系统版本'),
    ('network.domain', 'local', 'string', '网络域名'),
    ('network.workgroup', 'WORKGROUP', 'string', 'SMB 工作组'),
    ('backup.default_retention', '30', 'int', '默认备份保留天数');