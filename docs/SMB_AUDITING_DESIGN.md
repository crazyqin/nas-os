# SMB Auditing 设计文档

## 对标产品
- **TrueNAS 25.10**: SMB Session审计日志
- **群晖DSM 7.3**: Active Insight文件活动监控

## 功能概述
SMB Auditing记录所有SMB共享操作，用于安全审计、合规记录、异常检测。

## 技术架构

### 核心组件
```
┌─────────────────────────────────────────┐
│           SMB Auditing System           │
├─────────────────────────────────────────┤
│  ┌─────────────────────────────────┐    │
│  │      Event Collector            │    │
│  │  - 文件读/写/删除                │    │
│  │  - 目录创建/删除                 │    │
│  │  - 权限变更                      │    │
│  │  - 会话连接/断开                 │    │
│  └─────────────────────────────────┘    │
├─────────────────────────────────────────┤
│  ┌─────────────────────────────────┐    │
│  │      Log Processor              │    │
│  │  - 结构化存储                    │    │
│  │  - 异常检测                      │    │
│  │  - 告警触发                      │    │
│  └─────────────────────────────────┘    │
├─────────────────────────────────────────┤
│  ┌─────────────────────────────────┐    │
│  │      Storage Backend            │    │
│  │  - SQLite (本地)                 │    │
│  │  - Elasticsearch (可选)         │    │
│  │  - 远程Syslog                    │    │
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

## 记录字段

### SMB操作日志
```go
type SMBAuditLog struct {
    ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    User         string    `json:"user"`
    Share        string    `json:"share"`
    Operation    string    `json:"operation"` // read, write, delete, create
    Path         string    `json:"path"`
    ClientIP     string    `json:"client_ip"`
    BytesRead    int64     `json:"bytes_read"`
    BytesWritten int64     `json:"bytes_written"`
    Success      bool      `json:"success"`
    Duration     int64     `json:"duration_ms"`
}
```

### 会话日志
```go
type SMBSessionLog struct {
    ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    User         string    `json:"user"`
    ClientIP     string    `json:"client_ip"`
    ClientName   string    `json:"client_name"`
    Share        string    `json:"share"`
    Action       string    `json:"action"` // connect, disconnect
    SessionID    string    `json:"session_id"`
    Duration     int64     `json:"duration_sec"`
}
```

## API设计

### 获取审计日志
```go
// GET /api/v1/smb/audit/logs
type AuditLogQuery struct {
    Start       time.Time `json:"start"`
    End         time.Time `json:"end"`
    User        string    `json:"user,omitempty"`
    Share       string    `json:"share,omitempty"`
    Operation   string    `json:"operation,omitempty"`
    Success     *bool     `json:"success,omitempty"`
    Limit       int       `json:"limit"`
    Offset      int       `json:"offset"`
}
```

### 异常检测告警
```go
// POST /api/v1/smb/audit/rules
type AuditRule struct {
    Name        string `json:"name"`
    Pattern     string `json:"pattern"` // regex pattern
    Threshold   int    `json:"threshold"` // trigger count
    Window      int    `json:"window_sec"` // time window
    Action      string `json:"action"` // alert, block
    Notify      bool   `json:"notify"`
}
```

## 异常检测规则

| 规则 | 触发条件 | 响应动作 |
|------|----------|----------|
| 大量删除 | 100文件/分钟 | 告警+记录 |
| 异常读取 | 大量敏感文件 | 告警 |
| 暴力破解 | 10失败登录 | 封禁IP |
| 勒索特征 | 快速加密模式 | 告警+WriteOnce锁定 |

## 与竞品对比

| 功能 | TrueNAS 25.10 | nas-os | 差异 |
|------|---------------|--------|------|
| 操作记录 | ✅ | 📋 | 相同 |
| 会话记录 | ✅ Stateful | 📋 | 相同 |
| 异常检测 | ✅ Connect Plus | 📋 | nas-os有WriteOnce优势 |
| 勒索检测 | ✅ | ✅ WriteOnce | nas-os独家 |
| WebUI | ✅ 完善 | 📋 | 需跟进 |

## 安全特性
- 日志加密存储
- 日志不可篡改（可选WriteOnce）
- 访问权限控制（仅管理员）
- 远程日志导出（合规要求）

---

**设计者**: 刑部
**审核**: 司礼监
**状态**: 📋 待开发
**优先级**: P0