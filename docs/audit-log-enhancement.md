# NAS-OS 审计日志增强方案

**文档版本:** 1.0  
**创建日期:** 2026-03-26  
**审查部门:** 刑部（法务合规）  
**状态:** 设计方案

---

## 一、背景与目标

### 1.1 背景

NAS-OS 作为网络存储操作系统，SMB/NFS 是核心文件共享服务。为满足合规要求（GDPR、《个人信息保护法》、等保2.0），需要完善的访问审计日志。

### 1.2 目标

- 记录所有SMB/NFS文件访问行为
- 支持安全事件追溯和取证
- 满足合规审计要求
- 最小化性能影响

---

## 二、现状分析

### 2.1 已有实现

SMB审计已实现（`internal/audit/enhanced/smb_audit_hook.go`）：

| 功能 | 状态 | 说明 |
|------|------|------|
| 会话连接/断开 | ✅ 已实现 | 记录用户、IP、时间 |
| 文件打开/关闭 | ✅ 已实现 | 记录路径、访问模式 |
| 文件读/写 | ✅ 已实现 | 记录字节统计 |
| 文件删除/重命名 | ✅ 已实现 | 记录操作详情 |
| 权限变更 | ✅ 已实现 | 记录新旧权限 |
| 审计级别控制 | ✅ 已实现 | None/Minimal/Standard/Detailed/Full |
| 排除规则 | ✅ 已实现 | 可排除共享、用户、路径 |

### 2.2 缺失功能

| 功能 | 状态 | 优先级 |
|------|------|--------|
| NFS审计 | ❌ 未实现 | 高 |
| 审计日志加密 | ❌ 未实现 | 中 |
| 日志完整性校验 | ⚠️ 部分实现 | 高 |
| 实时告警 | ❌ 未实现 | 中 |
| GDPR数据导出/删除 | ❌ 未实现 | 高 |

---

## 三、NFS审计设计方案

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                     NFS Service                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │ NFSv3    │  │ NFSv4    │  │ NLM      │  │ RQUOTAD  │    │
│  │ Handler  │  │ Handler  │  │ Handler  │  │ Handler  │    │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘    │
│       │             │             │             │          │
│       └─────────────┴──────┬──────┴─────────────┘          │
│                            │                                │
│                            ▼                                │
│                  ┌──────────────────┐                       │
│                  │  NFSAuditHook    │ ◄── 新增              │
│                  └────────┬─────────┘                       │
└───────────────────────────┼─────────────────────────────────┘
                            │
                            ▼
                  ┌──────────────────┐
                  │  NFSAuditManager │ ◄── 新增
                  └────────┬─────────┘
                           │
                           ▼
                  ┌──────────────────┐
                  │   AuditStorage   │
                  │  (统一存储层)     │
                  └──────────────────┘
```

### 3.2 NFS审计类型定义

```go
// NFSAuditEntry NFS审计条目
type NFSAuditEntry struct {
    ID            string            `json:"id"`
    Timestamp     time.Time         `json:"timestamp"`
    
    // 客户端信息
    ClientIP      string            `json:"client_ip"`
    ClientPort    int               `json:"client_port"`
    AuthType      NFSAuthType       `json:"auth_type"` // sys, krb5, krb5i, krb5p
    
    // 用户信息
    UID           int               `json:"uid"`
    GID           int               `json:"gid"`
    Username      string            `json:"username,omitempty"`
    Groups        []int             `json:"groups,omitempty"`
    
    // 操作信息
    Operation     NFSOperation      `json:"operation"`
    ExportPath    string            `json:"export_path"`
    FilePath      string            `json:"file_path,omitempty"`
    FileType      string            `json:"file_type,omitempty"` // file, dir, symlink
    
    // 操作详情
    Status        string            `json:"status"` // success, failure, denied
    ErrorCode     int               `json:"error_code,omitempty"`
    ErrorMessage  string            `json:"error_message,omitempty"`
    BytesRead     int64             `json:"bytes_read,omitempty"`
    BytesWritten  int64             `json:"bytes_written,omitempty"`
    
    // 附加信息
    FileHandle    string            `json:"file_handle,omitempty"`
    Offset        int64             `json:"offset,omitempty"`
    Count         int               `json:"count,omitempty"`
    Metadata      map[string]any    `json:"metadata,omitempty"`
}

// NFSOperation NFS操作类型
type NFSOperation string

const (
    NFSOpLookup    NFSOperation = "lookup"
    NFSOpRead      NFSOperation = "read"
    NFSOpWrite     NFSOperation = "write"
    NFSOpCreate    NFSOperation = "create"
    NFSOpRemove    NFSOperation = "remove"
    NFSOpRename    NFSOperation = "rename"
    NFSOpMkdir     NFSOperation = "mkdir"
    NFSOpRmdir     NFSOperation = "rmdir"
    NFSOpSymlink   NFSOperation = "symlink"
    NFSOpReadlink  NFSOperation = "readlink"
    NFSOpGetattr   NFSOperation = "getattr"
    NFSOpSetattr   NFSOperation = "setattr"
    NFSOpReaddir   NFSOperation = "readdir"
    NFSOpFsstat    NFSOperation = "fsstat"
    NFSOpAccess    NFSOperation = "access"
    NFSOpCommit    NFSOperation = "commit"
    NFSOpLock      NFSOperation = "lock"
    NFSOpUnlock    NFSOperation = "unlock"
)
```

### 3.3 NFS审计钩子实现

```go
// NFSAuditHook NFS审计钩子
type NFSAuditHook struct {
    manager    *NFSAuditManager
    sessions   map[string]*NFSSessionInfo
    mu         sync.RWMutex
}

// OnAccess 文件访问钩子
func (h *NFSAuditHook) OnAccess(clientIP string, uid, gid int, exportPath, filePath string, op NFSOperation) {
    entry := &NFSAuditEntry{
        Timestamp:   time.Now(),
        ClientIP:    clientIP,
        UID:         uid,
        GID:         gid,
        Operation:   op,
        ExportPath:  exportPath,
        FilePath:    filePath,
    }
    h.manager.Log(entry)
}

// OnRead 文件读取钩子
func (h *NFSAuditHook) OnRead(clientIP string, uid int, exportPath, filePath string, offset, length int64) {
    // 记录读取操作
}

// OnWrite 文件写入钩子
func (h *NFSAuditHook) OnWrite(clientIP string, uid int, exportPath, filePath string, offset, length int64) {
    // 记录写入操作
}
```

---

## 四、合规性增强

### 4.1 GDPR合规要求

| GDPR条款 | 要求 | 实现方案 |
|----------|------|----------|
| Art. 5(1)(a) | 数据处理合法性、公平性、透明性 | 隐私政策声明，用户知情同意 |
| Art. 5(1)(b) | 数据收集目的限制 | 明确审计目的，禁止其他用途 |
| Art. 5(1)(c) | 数据最小化原则 | 仅记录必要信息，敏感数据脱敏 |
| Art. 5(1)(d) | 数据准确性 | 提供数据更正机制 |
| Art. 5(1)(e) | 数据保留期限 | 自动清理过期日志 |
| Art. 5(1)(f) | 数据安全 | 加密存储，访问控制 |
| Art. 15 | 访问权 | 提供用户数据导出API |
| Art. 17 | 删除权（被遗忘权） | 提供用户数据删除API |
| Art. 20 | 数据可携带权 | 导出标准格式（JSON/CSV） |
| Art. 25 | 隐私保护设计 | 默认脱敏，最小权限 |

### 4.2 数据脱敏规则

```go
// AuditDesensitizer 审计日志脱敏器
type AuditDesensitizer struct {
    rules []DesensitizationRule
}

// DefaultAuditDesensitizationRules 默认审计脱敏规则
func DefaultAuditDesensitizationRules() []DesensitizationRule {
    return []DesensitizationRule{
        // IP地址部分隐藏
        {
            Field:    "client_ip",
            Strategy: StrategyPartial,
            ShowFirst: 2,
            ShowLast:  0,
        },
        // 用户名部分隐藏
        {
            Field:    "username",
            Strategy: StrategyPartial,
            ShowFirst: 1,
            ShowLast:  1,
        },
        // 文件路径敏感目录隐藏
        {
            Field:    "file_path",
            Strategy: StrategyMask,
            Patterns: []string{"/home/*/private/*", "/*/.ssh/*", "/*/.gnupg/*"},
        },
    }
}
```

### 4.3 日志保留策略

```go
// AuditRetentionPolicy 审计日志保留策略
type AuditRetentionPolicy struct {
    // 默认保留天数
    DefaultRetentionDays int `json:"default_retention_days"`
    
    // 按操作类型保留
    RetentionByCategory map[string]int `json:"retention_by_category"`
    
    // 敏感操作额外保留
    SensitiveOpRetentionDays int `json:"sensitive_op_retention_days"`
    
    // 压缩设置
    CompressAfterDays int `json:"compress_after_days"`
    
    // 归档设置
    ArchiveAfterDays int `json:"archive_after_days"`
    ArchivePath      string `json:"archive_path"`
}

// DefaultRetentionPolicy 默认保留策略
func DefaultRetentionPolicy() *AuditRetentionPolicy {
    return &AuditRetentionPolicy{
        DefaultRetentionDays:     90,  // 默认保留90天
        RetentionByCategory: map[string]int{
            "file_read":      30,  // 读操作30天
            "file_write":     90,  // 写操作90天
            "file_delete":    365, // 删除操作1年
            "permission":     365, // 权限变更1年
            "admin":          365, // 管理操作1年
        },
        SensitiveOpRetentionDays: 365,
        CompressAfterDays:        7,
        ArchiveAfterDays:         90,
    }
}
```

---

## 五、日志完整性保护

### 5.1 区块链式哈希链

```go
// AuditIntegrityChecker 审计日志完整性校验器
type AuditIntegrityChecker struct {
    lastHash []byte
    key      []byte
    mu       sync.Mutex
}

// ComputeHash 计算日志条目哈希
func (c *AuditIntegrityChecker) ComputeHash(entry *AuditEntry) []byte {
    h := sha256.New()
    
    // 包含上一条日志的哈希（形成链）
    h.Write(c.lastHash)
    
    // 包含当前日志的关键字段
    h.Write([]byte(entry.ID))
    h.Write([]byte(entry.Timestamp.Format(time.RFC3339Nano)))
    h.Write([]byte(entry.UserID))
    h.Write([]byte(entry.Operation))
    h.Write([]byte(entry.Resource))
    
    // 使用密钥签名
    h.Write(c.key)
    
    hash := h.Sum(nil)
    c.lastHash = hash
    
    return hash
}

// VerifyChain 验证日志链完整性
func (c *AuditIntegrityChecker) VerifyChain(entries []*AuditEntry) (bool, int) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    for i, entry := range entries {
        expected := c.ComputeHash(entry)
        if !bytes.Equal(expected, entry.Hash) {
            return false, i // 返回被篡改的位置
        }
    }
    
    return true, -1
}
```

### 5.2 数字签名

```go
// AuditSigner 审计日志签名器
type AuditSigner struct {
    privateKey ed25519.PrivateKey
    publicKey  ed25519.PublicKey
}

// Sign 对日志条目进行签名
func (s *AuditSigner) Sign(entry *AuditEntry) []byte {
    data := s.canonicalize(entry)
    return ed25519.Sign(s.privateKey, data)
}

// Verify 验证签名
func (s *AuditSigner) Verify(entry *AuditEntry, signature []byte) bool {
    data := s.canonicalize(entry)
    return ed25519.Verify(s.publicKey, data, signature)
}
```

---

## 六、实时告警机制

### 6.1 告警规则

```go
// AuditAlertRule 审计告警规则
type AuditAlertRule struct {
    ID          string        `json:"id"`
    Name        string        `json:"name"`
    Description string        `json:"description"`
    Enabled     bool          `json:"enabled"`
    
    // 触发条件
    Conditions  []AlertCondition `json:"conditions"`
    
    // 时间窗口
    WindowSize  time.Duration `json:"window_size"`
    Threshold   int           `json:"threshold"`
    
    // 告警动作
    Actions     []AlertAction `json:"actions"`
    Severity    string        `json:"severity"` // info, warning, critical
}

// DefaultAlertRules 默认告警规则
func DefaultAlertRules() []*AuditAlertRule {
    return []*AuditAlertRule{
        // 暴力破解检测
        {
            ID:   "brute_force_login",
            Name: "登录暴力破解检测",
            Conditions: []AlertCondition{
                {Field: "event_type", Op: "eq", Value: "login_failure"},
            },
            WindowSize: 5 * time.Minute,
            Threshold:  5,
            Actions:    []AlertAction{{Type: "block_ip"}, {Type: "notify_admin"}},
            Severity:   "critical",
        },
        // 敏感文件访问
        {
            ID:   "sensitive_file_access",
            Name: "敏感文件访问告警",
            Conditions: []AlertCondition{
                {Field: "file_path", Op: "match", Value: "^/etc/.*"},
                {Field: "operation", Op: "in", Value: []string{"read", "write"}},
            },
            Threshold: 1,
            Actions:   []AlertAction{{Type: "notify_admin"}},
            Severity:  "warning",
        },
        // 大量数据外传
        {
            ID:   "data_exfiltration",
            Name: "数据外传检测",
            Conditions: []AlertCondition{
                {Field: "operation", Op: "eq", Value: "read"},
                {Field: "bytes_read", Op: "gt", Value: 100 * 1024 * 1024}, // 100MB
            },
            WindowSize: 1 * time.Hour,
            Threshold:  3,
            Actions:    []AlertAction{{Type: "notify_admin"}, {Type: "throttle"}},
            Severity:   "critical",
        },
    }
}
```

### 6.2 告警通知

```go
// AlertNotifier 告警通知器
type AlertNotifier interface {
    Send(ctx context.Context, alert *AuditAlert) error
}

// EmailNotifier 邮件通知
type EmailNotifier struct {
    smtp     *SMTPConfig
    template *template.Template
}

// WebhookNotifier Webhook通知
type WebhookNotifier struct {
    endpoint string
    secret   string
}

// SyslogNotifier Syslog通知
type SyslogNotifier struct {
    addr     string
    priority syslog.Priority
}
```

---

## 七、API设计

### 7.1 审计日志查询API

```yaml
# GET /api/v1/audit/logs
parameters:
  - name: start_time
    type: string
    format: date-time
  - name: end_time
    type: string
    format: date-time
  - name: user_id
    type: string
  - name: operation
    type: string
    enum: [read, write, delete, rename, permission_change]
  - name: resource_type
    type: string
    enum: [file, share, user, system]
  - name: status
    type: string
    enum: [success, failure, denied]
  - name: min_risk_score
    type: integer
  - name: limit
    type: integer
    default: 100
  - name: offset
    type: integer
    default: 0

responses:
  200:
    schema:
      type: object
      properties:
        total:
          type: integer
        entries:
          type: array
          items:
            $ref: '#/definitions/AuditEntry'
```

### 7.2 用户数据导出API (GDPR Art. 15, 20)

```yaml
# GET /api/v1/audit/export
parameters:
  - name: user_id
    type: string
    required: true
  - name: format
    type: string
    enum: [json, csv]
    default: json
  - name: start_time
    type: string
    format: date-time
  - name: end_time
    type: string
    format: date-time

responses:
  200:
    description: 用户审计日志导出
    schema:
      type: object
      properties:
        user_id:
          type: string
        export_time:
          type: string
          format: date-time
        total_entries:
          type: integer
        entries:
          type: array
```

### 7.3 用户数据删除API (GDPR Art. 17)

```yaml
# DELETE /api/v1/audit/user/{user_id}
parameters:
  - name: user_id
    type: string
    required: true
  - name: retention_override
    type: boolean
    default: false
    description: 是否覆盖保留策略

responses:
  200:
    description: 用户审计日志已删除
    schema:
      type: object
      properties:
        user_id:
          type: string
        deleted_entries:
          type: integer
        deleted_at:
          type: string
          format: date-time

  403:
    description: 由于合规要求无法删除（保留期内）
```

---

## 八、实施计划

### 8.1 阶段一：NFS审计基础（2周）

- [ ] 实现 `NFSAuditHook`
- [ ] 实现 `NFSAuditManager`
- [ ] 集成到现有NFS服务
- [ ] 单元测试和集成测试

### 8.2 阶段二：合规性增强（1周）

- [ ] 实现数据脱敏
- [ ] 实现日志保留策略
- [ ] 实现GDPR数据导出API
- [ ] 实现GDPR数据删除API

### 8.3 阶段三：完整性保护（1周）

- [ ] 实现哈希链校验
- [ ] 实现数字签名
- [ ] 定期完整性验证任务

### 8.4 阶段四：实时告警（1周）

- [ ] 实现告警规则引擎
- [ ] 实现多种通知渠道
- [ ] 告警规则配置界面

---

## 九、性能考量

### 9.1 性能目标

| 指标 | 目标值 |
|------|--------|
| 审计日志写入延迟 | < 5ms (P99) |
| 审计对文件操作影响 | < 2% 性能损耗 |
| 日志查询响应时间 | < 500ms (10000条) |
| 存储空间 | < 100MB/天/1000用户 |

### 9.2 优化策略

1. **异步写入**: 审计日志异步写入，不阻塞主操作
2. **批量提交**: 累积一定数量后批量写入
3. **索引优化**: 为常用查询字段建立索引
4. **压缩存储**: 旧日志自动压缩
5. **分级存储**: 热数据SSD，冷数据HDD

---

## 十、文档更新建议

### 10.1 LEGAL.md 需补充内容

```markdown
## 七、审计日志合规

### 7.1 数据处理声明

NAS-OS 审计日志用于：
- 安全事件追溯和取证
- 合规审计要求
- 系统性能分析

### 7.2 用户权利

根据GDPR和中国《个人信息保护法》，用户有权：
- 访问自己的审计日志记录（API: GET /api/v1/audit/export）
- 请求删除审计日志（API: DELETE /api/v1/audit/user/{user_id}）
- 导出审计日志数据（支持JSON/CSV格式）

### 7.3 数据保留

- 常规操作日志：90天
- 敏感操作日志：1年
- 删除操作日志：1年
- 管理员操作日志：1年

### 7.4 第三方数据传输

使用云端AI服务时，审计日志不包含AI请求内容。
AI请求内容由AI服务模块单独处理，详见AI服务隐私政策。
```

---

## 十一、附录

### A. 审计事件类型对照表

| 事件类型 | SMB | NFS | 说明 |
|----------|-----|-----|------|
| connect | ✅ | ✅ | 连接建立 |
| disconnect | ✅ | ✅ | 连接断开 |
| file_open | ✅ | ✅ | 文件打开 |
| file_close | ✅ | ✅ | 文件关闭 |
| file_read | ✅ | ✅ | 文件读取 |
| file_write | ✅ | ✅ | 文件写入 |
| file_create | ✅ | ✅ | 文件创建 |
| file_delete | ✅ | ✅ | 文件删除 |
| file_rename | ✅ | ✅ | 文件重命名 |
| permission_change | ✅ | ✅ | 权限变更 |
| ownership_change | ✅ | ✅ | 所有者变更 |
| file_lock | ✅ | ✅ | 文件锁定 |
| file_unlock | ✅ | ✅ | 文件解锁 |

### B. 风险评分模型

```go
// RiskScoreCalculator 风险评分计算器
func CalculateRiskScore(entry *AuditEntry) int {
    score := 0
    
    // 基础分
    switch entry.Operation {
    case "file_delete", "permission_change":
        score += 30
    case "file_write":
        score += 20
    case "file_read":
        score += 10
    }
    
    // 敏感路径加分
    if isSensitivePath(entry.FilePath) {
        score += 20
    }
    
    // 非工作时间加分
    if isOffHours(entry.Timestamp) {
        score += 10
    }
    
    // 失败操作加分
    if entry.Status == "failure" || entry.Status == "denied" {
        score += 15
    }
    
    // 异常IP加分
    if isUnusualIP(entry.ClientIP) {
        score += 15
    }
    
    // 限制最高分
    if score > 100 {
        score = 100
    }
    
    return score
}
```

---

*本文档由刑部（法务合规）编写 | 下次审查日期：2026-06-26*