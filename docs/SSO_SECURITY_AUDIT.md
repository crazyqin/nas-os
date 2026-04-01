# 刑部：SSO安全审计方案

## 概述
对标TrueNAS SSO/RBAC，设计nas-os多系统统一认证与安全审计方案。

## SSO集成设计

### 1. 统一认证架构

```
┌─────────────────────────────────────────────────────────┐
│                  SSO Gateway (主节点)                    │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │ OAuth2      │  │ SAML 2.0    │  │ LDAP/AD     │      │
│  │ Provider    │  │ Provider    │  │ Provider    │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐      │
│  │ Token       │  │ Session     │  │ Permission  │      │
│  │ Manager     │  │ Manager     │  │ Sync        │      │
│  └─────────────┘  └─────────────┘  └─────────────┘      │
└─────────────────────────────────────────────────────────┘
           │                    │                    │
           ▼                    ▼                    ▼
    ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
    │   Node A    │      │   Node B    │      │   Node C    │
    │  (子节点)   │      │  (子节点)   │      │  (子节点)   │
    └─────────────┘      └─────────────┘      └─────────────┘
```

### 2. OAuth2集成

```go
// OAuth2Provider OAuth2认证提供商
type OAuth2Provider struct {
    Name         string
    ClientID     string
    ClientSecret string
    AuthURL      string
    TokenURL     string
    Scopes       []string
}

// OAuth2Config OAuth2配置
type OAuth2Config struct {
    Providers []OAuth2Provider
    DefaultProvider string
    TokenExpiry time.Duration
    RefreshEnabled bool
}

// OAuth2Flow 认证流程
func (s *SSOService) OAuth2Flow(ctx context.Context, provider string) (*TokenResponse, error) {
    // 1. 获取授权URL
    authURL := s.GetAuthURL(provider)
    
    // 2. 用户授权后获取code
    code := s.WaitForCallback(provider)
    
    // 3. 交换code获取token
    token := s.ExchangeCode(provider, code)
    
    // 4. 获取用户信息
    userInfo := s.GetUserInfo(provider, token)
    
    // 5. 创建本地session
    session := s.CreateSession(userInfo, token)
    
    return session, nil
}
```

### 3. SAML集成

```go
// SAMLProvider SAML认证提供商
type SAMLProvider struct {
    Name           string
    EntityID       string
    MetadataURL    string
    SSOURL         string
    Certificate    string
}

// SAMLConfig SAML配置
type SAMLConfig struct {
    Providers      []SAMLProvider
    DefaultProvider string
    AttributeMap   map[string]string // 属性映射
    SessionExpiry  time.Duration
}
```

### 4. LDAP/AD集成

```go
// LDAPProvider LDAP/AD认证提供商
type LDAPProvider struct {
    Name       string
    ServerURL  string
    BaseDN     string
    BindDN     string
    BindPassword string
    UserFilter string
    GroupFilter string
    AttributeMap map[string]string
}

// LDAPAuth LDAP认证流程
func (s *SSOService) LDAPAuth(ctx context.Context, username, password string) (*User, error) {
    // 1. 连接LDAP服务器
    conn := s.ConnectLDAP()
    
    // 2. 验证用户凭据
    userDN := s.FindUserDN(username)
    err := s.BindUser(userDN, password)
    
    // 3. 获取用户属性
    attrs := s.GetUserAttributes(userDN)
    
    // 4. 获取用户组
    groups := s.GetUserGroups(userDN)
    
    // 5. 映射权限
    permissions := s.MapGroupsToPermissions(groups)
    
    return &User{Username: username, Groups: groups, Permissions: permissions}, nil
}
```

## 跨节点权限同步

### 1. 权限同步机制

```go
// PermissionSync 权限同步服务
type PermissionSync struct {
    masterNode string
    nodes      NodeRegistry
    syncQueue  *SyncQueue
}

// SyncPermissions 同步权限到所有节点
func (s *PermissionSync) SyncPermissions(ctx context.Context, userID string, permissions []Permission) error {
    // 1. 在主节点更新权限
    s.UpdateMasterPermissions(userID, permissions)
    
    // 2. 推送到所有子节点
    for _, node := range s.nodes.GetAll() {
        err := s.PushPermissions(node, userID, permissions)
        if err != nil {
            // 记录失败，后续重试
            s.syncQueue.Enqueue(userID, node.ID)
        }
    }
    
    return nil
}

// PermissionSyncRequest 权限同步请求
type PermissionSyncRequest struct {
    UserID       string
    Permissions  []Permission
    Timestamp    time.Time
    SyncID       string // 用于追踪同步状态
}
```

### 2. 会话管理

```go
// SessionManager 多节点会话管理
type SessionManager struct {
    sessions    map[string]*Session
    nodeSessions map[string][]string // 节点→会话ID映射
    expiry      time.Duration
}

// Session 会话信息
type Session struct {
    ID           string
    UserID       string
    Provider     string
    CreatedAt    time.Time
    ExpiresAt    time.Time
    RefreshToken string
    Nodes        []string // 已授权的节点列表
}

// ValidateNodeAccess 验证节点访问权限
func (s *SessionManager) ValidateNodeAccess(sessionID, nodeID string) bool {
    session := s.GetSession(sessionID)
    if session == nil || session.ExpiresAt.Before(time.Now()) {
        return false
    }
    
    // 检查会话是否授权访问该节点
    return s.HasNodePermission(session.UserID, nodeID)
}
```

## 跨节点操作审计

### 1. 审计日志架构

```go
// CrossNodeAuditLog 跨节点审计日志
type CrossNodeAuditLog struct {
    masterNode  string
    auditStore  AuditStorage
    aggregators map[string]*NodeAuditAggregator
}

// AuditEvent 审计事件
type AuditEvent struct {
    ID          string
    Timestamp   time.Time
    UserID      string
    SourceNode  string
    TargetNode  string // 可选，跨节点操作
    Action      string
    Resource    string
    Result      string // success, failure, denied
    Details     map[string]string
    SessionID   string
    IPAddress   string
}

// LogAudit 记录审计事件
func (a *CrossNodeAuditLog) LogAudit(ctx context.Context, event *AuditEvent) error {
    // 1. 本地存储
    a.auditStore.Store(event)
    
    // 2. 推送到主节点聚合
    if event.TargetNode != "" {
        a.PushToMaster(event)
    }
    
    return nil
}

// AggregateAudit 聚合审计日志
func (a *CrossNodeAuditLog) AggregateAudit(ctx context.Context, query *AuditQuery) ([]AuditEvent, error) {
    // 1. 从主节点获取聚合日志
    events := a.auditStore.Query(query)
    
    // 2. 如果查询涉及特定节点，从该节点获取详细日志
    if query.Node != "" {
        nodeEvents := a.FetchFromNode(query.Node, query)
        events = append(events, nodeEvents...)
    }
    
    return events, nil
}
```

### 2. 审计报告

```go
// AuditReportGenerator 审计报告生成器
type AuditReportGenerator struct {
    auditLog *CrossNodeAuditLog
}

// GenerateSecurityReport 生成安全审计报告
func (g *AuditReportGenerator) GenerateSecurityReport(ctx context.Context, period ReportPeriod) (*SecurityReport, error) {
    // 1. 获取审计事件
    events := g.auditLog.QueryByPeriod(period)
    
    // 2. 分析统计
    stats := g.AnalyzeEvents(events)
    
    // 3. 生成报告
    report := &SecurityReport{
        Period: period,
        Summary: stats,
        FailedLogins: g.FilterFailedLogins(events),
        CrossNodeOps: g.FilterCrossNodeOps(events),
        PermissionChanges: g.FilterPermissionChanges(events),
        Anomalies: g.DetectAnomalies(events),
    }
    
    return report, nil
}

// SecurityReport 安全审计报告
type SecurityReport struct {
    Period          ReportPeriod
    Summary         AuditSummary
    FailedLogins    []AuditEvent
    CrossNodeOps    []AuditEvent
    PermissionChanges []AuditEvent
    Anomalies       []AuditAnomaly
    Recommendations []SecurityRecommendation
}
```

## 安全最佳实践

| 主题 | 要求 |
|------|------|
| Token管理 | 使用JWT，设置合理过期时间，支持刷新 |
| 会话安全 | HTTPS必须，Cookie secure flag，CSRF防护 |
| 权限最小化 | 默认无权限，按需授权 |
| 审计完整 | 所有跨节点操作必须记录 |
| 密码策略 | 复杂度要求，定期更换，禁用弱密码 |
| MFA强制 | 管理员操作必须MFA验证 |

## 实现计划

| 阶段 | 任务 | 时间 |
|------|------|------|
| M1 | OAuth2/SAML Provider集成 | 04-03 |
| M2 | PermissionSync跨节点同步 | 04-05 |
| M3 | SessionManager多节点会话 | 04-08 |
| M4 | CrossNodeAuditLog审计日志 | 04-10 |
| M5 | AuditReportGenerator报告生成 | 04-15 |

## 与现有系统集成

- 扩展 `internal/auth/oauth2.go` OAuth2实现
- 扩展 `internal/auth/ldap.go` LDAP实现
- 扩展 `internal/auth/security_audit.go` 审计功能
- 利用 `internal/auth/rbac.go` 权限管理基础