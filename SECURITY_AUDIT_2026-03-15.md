# NAS-OS 安全审计报告

**审计日期**: 2026-03-15
**审计人**: 刑部安全审计组
**项目版本**: 当前开发版本

---

## 一、执行摘要

### 审计范围
1. 硬编码密钥/密码检查
2. SQL 注入风险
3. API 权限控制审查
4. 命令注入风险
5. 依赖安全检查
6. 配置安全审查

### 风险评级

| 严重程度 | 数量 | 说明 |
|----------|------|------|
| 🔴 高风险 | 2 | 需立即修复 |
| 🟠 中风险 | 4 | 需尽快修复 |
| 🟡 低风险 | 3 | 建议修复 |

---

## 二、发现的安全问题

### 🔴 高风险问题

#### 1. API 缺乏全局认证保护

**位置**: `internal/web/server.go:444-500`

**问题描述**: API 路由组 `/api/v1` 未添加全局认证中间件，大量敏感 API 端点可直接访问。

**受影响的 API**:
```
/api/v1/volumes         - 卷管理（创建/删除存储）
/api/v1/users           - 用户管理
/api/v1/system/info     - 系统信息
/api/v1/network/*       - 网络配置
/api/v1/docker/*        - Docker 管理
/api/v1/vms/*           - 虚拟机管理
```

**风险**: 未授权用户可完全控制系统，包括：
- 创建/删除存储卷
- 管理用户账户
- 配置网络和防火墙
- 操作容器和虚拟机

**修复建议**:
```go
func (s *Server) setupRoutes() {
    // 创建认证中间件
    authMiddleware := auth.NewAuthMiddleware(s.userMgr, s.rbacMgr)
    
    // API 路由组添加认证中间件
    api := s.engine.Group("/api/v1")
    api.Use(authMiddleware.RequireAuth())  // 添加全局认证
    {
        // 公开接口单独处理
        api.GET("/system/health", s.getHealth)  // 健康检查可公开
        // ... 其他需要认证的接口
    }
    
    // 公开路由组（无需认证）
    public := s.engine.Group("/api/v1/public")
    {
        public.POST("/auth/login", s.login)
        public.GET("/system/health", s.getHealth)
    }
}
```

#### 2. Grafana 默认密码

**位置**: `docker-compose.prod.yml:108-109`

**问题代码**:
```yaml
environment:
  - GF_SECURITY_ADMIN_USER=${GRAFANA_ADMIN_USER:-admin}
  - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:-admin123}
```

**风险**: 使用默认密码 `admin123`，攻击者可轻松登录 Grafana 仪表板获取系统监控数据和敏感信息。

**修复建议**:
```yaml
environment:
  - GF_SECURITY_ADMIN_USER=${GRAFANA_ADMIN_USER}
  - GF_SECURITY_ADMIN_PASSWORD__FILE=/run/secrets/grafana_admin_password
  - GF_USERS_ALLOW_SIGN_UP=false
secrets:
  grafana_admin_password:
    file: ./secrets/grafana_admin_password.txt
```

---

### 🟠 中风险问题

#### 3. CSRF 密钥硬编码默认值

**位置**: `internal/web/middleware.go:24-26`

**问题代码**:
```go
csrfKey := os.Getenv("NAS_CSRF_KEY")
if csrfKey == "" {
    csrfKey = "change-this-to-a-32-byte-secret-key-now!"
}
```

**风险**: 未设置环境变量时使用硬编码默认值，攻击者可伪造 CSRF token。

**修复建议**:
```go
csrfKey := os.Getenv("NAS_CSRF_KEY")
if csrfKey == "" {
    // 生产环境必须设置，否则拒绝启动
    return nil, errors.New("NAS_CSRF_KEY environment variable is required")
}
if len(csrfKey) < 32 {
    return nil, errors.New("NAS_CSRF_KEY must be at least 32 bytes")
}
```

#### 4. 命令注入风险（exec.Command 变量使用）

**位置**: 多个文件，共 57 处

**主要涉及文件**:
- `pkg/btrfs/btrfs.go` (12处)
- `internal/network/firewall.go` (7处)
- `internal/network/portforward.go` (9处)
- `internal/docker/manager.go` (5处)

**问题示例** (`internal/network/portforward.go:167-200`):
```go
dnatCmd := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING",
    "-p", rule.Protocol,  // 来自用户输入
    "--dport", fmt.Sprintf("%d", rule.ExternalPort),
    "-j", "DNAT",
    "--to-destination", fmt.Sprintf("%s:%d", rule.InternalIP, rule.InternalPort))
```

**风险**: 虽然 `exec.Command` 不会通过 shell 执行，但如果参数来自不可信来源，仍可能通过特殊值绕过预期行为。

**修复建议**:
```go
// 添加输入验证白名单
var validProtocols = map[string]bool{"tcp": true, "udp": true}

func validatePortForwardRule(rule *PortForwardRule) error {
    if !validProtocols[rule.Protocol] {
        return fmt.Errorf("invalid protocol: %s", rule.Protocol)
    }
    if rule.ExternalPort < 1 || rule.ExternalPort > 65535 {
        return fmt.Errorf("invalid external port")
    }
    // IP 地址格式验证
    if net.ParseIP(rule.InternalIP) == nil {
        return fmt.Errorf("invalid internal IP")
    }
    return nil
}
```

#### 5. 容器特权模式

**位置**: `docker-compose.prod.yml:33-34`

**问题代码**:
```yaml
privileged: true
network_mode: host
```

**风险**: 
- `privileged: true` 赋予容器访问所有主机设备的权限
- `network_mode: host` 使容器共享主机网络命名空间

**修复建议**:
```yaml
# 使用精确的设备授权替代特权模式
devices:
  - /dev/sda:/dev/sda
  - /dev/sdb:/dev/sdb
# 使用 capabilities 精确授权
cap_add:
  - SYS_ADMIN  # Btrfs 操作需要
  - NET_ADMIN  # 网络配置需要
# 网络隔离
networks:
  - nas-internal
```

#### 6. 首次启动打印默认密码

**位置**: `internal/users/manager.go:158-166`

**问题代码**:
```go
fmt.Println("========================================")
fmt.Println("⚠️  首次启动：默认管理员账号已创建")
fmt.Println("   用户名: admin")
fmt.Printf("   密码: %s\n", defaultPassword)
```

**风险**: 虽然使用随机密码，但输出到 stdout 可能在日志系统中暴露。

**修复建议**:
```go
// 方案1: 写入密码文件，设置严格权限
passwordFile := "/var/lib/nas-os/.initial_admin_password"
os.WriteFile(passwordFile, []byte(defaultPassword), 0600)
fmt.Println("默认管理员密码已写入", passwordFile)

// 方案2: 生成一次性密码链接
resetToken := generateResetToken()
fmt.Printf("请访问以下链接设置管理员密码: https://nas.local/setup?token=%s\n", resetToken)
```

---

### 🟡 低风险问题

#### 7. 错误处理不完善

**位置**: 多处（gosec 报告 67 处 G104）

**典型问题**: 忽略错误返回值
```go
os.MkdirAll(dir, 0755)  // 未检查错误
```

**修复建议**:
```go
if err := os.MkdirAll(dir, 0750); err != nil {
    log.Printf("创建目录失败: %v", err)
    return err
}
```

#### 8. 文件权限过于宽松

**位置**: 多处配置目录使用 0755

**建议**: 敏感配置目录使用 0750 或更严格权限。

#### 9. CORS 配置允许 OPTIONS 预检请求任意源

**位置**: `internal/web/middleware.go:68-71`

```go
if !allowed {
    if c.Request.Method == "OPTIONS" {
        c.Header("Access-Control-Allow-Origin", "*")  // 允许任意源
    }
}
```

**建议**: 限制预检请求的允许源。

---

## 三、API 权限控制审计

### 当前状态
项目已实现完善的 RBAC 权限系统：
- ✅ `auth/rbac.go` - 完整的角色权限模型
- ✅ `auth/rbac_middleware.go` - 权限中间件
- ✅ 支持 Admin/User/Guest/System 角色
- ✅ 资源级 ACL 控制

### 问题
**关键问题**: RBAC 中间件未被应用到 API 路由！

**修复优先级**: 🔴 高

---

## 四、SQL 注入审计

### 审计结果: ✅ 无风险

所有数据库操作使用参数化查询：

**示例** (`internal/system/monitor.go:738`):
```go
query := `INSERT OR REPLACE INTO alerts (id, type, level, message, source, timestamp, acknowledged, resolved) 
          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
_, err = m.db.Exec(query, alert.ID, alert.Type, ...)
```

---

## 五、依赖安全

### 主要依赖版本
| 依赖 | 版本 | 状态 |
|------|------|------|
| golang.org/x/crypto | v0.48.0 | ✅ 最新 |
| gin-gonic/gin | v1.11.0 | ✅ 最新 |
| aws/aws-sdk-go-v2 | v1.41.3 | ✅ 最新 |
| gorilla/websocket | v1.5.3 | ✅ 最新 |

### 建议
```bash
# 定期执行依赖漏洞扫描
go list -m -json all | nancy sleuth
# 或使用 govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

---

## 六、修复优先级

### 立即修复（24小时内）
1. ✅ 为 API 添加全局认证中间件
2. ✅ 移除 Grafana 默认密码

### 尽快修复（1周内）
1. CSRF 密钥强制要求环境变量
2. 为 exec.Command 参数添加输入验证
3. 评估 privileged 模式替代方案

### 计划修复（下个版本）
1. 完善错误处理
2. 收紧文件权限
3. 改进首次启动密码分发机制

---

## 七、安全最佳实践建议

### 1. 生产部署清单
- [ ] 设置强密码（至少16位，包含大小写字母、数字、特殊字符）
- [ ] 配置 HTTPS 证书
- [ ] 启用防火墙，仅开放必要端口
- [ ] 定期备份数据
- [ ] 启用审计日志

### 2. 环境变量清单
```bash
# 必须设置
NAS_CSRF_KEY=<32字节随机密钥>
GRAFANA_ADMIN_PASSWORD=<强密码>
JWT_SECRET=<32字节随机密钥>

# 可选
SMTP_HOST=smtp.example.com
SMTP_USER=noreply@example.com
SMTP_PASS=<SMTP密码>
```

### 3. 安全配置文件权限
```bash
chmod 600 /etc/nas-os/config.yaml
chmod 600 /etc/nas-os/mfa-config.json
chmod 700 /etc/nas-os/
```

---

## 八、结论

### 总体评价
NAS-OS 项目安全基础扎实：
- ✅ 完善的认证体系（本地/OAuth2/LDAP/MFA）
- ✅ 健全的密码策略
- ✅ 完整的 RBAC 权限模型
- ✅ SQL 注入防护良好
- ✅ 审计日志功能完善

### 主要问题
- ❌ API 路由未应用认证中间件（关键）
- ⚠️ 部分配置使用默认值/硬编码

### 评级: B → 需修复高风险问题后可达 A

---

**审计人**: 刑部安全审计组  
**审计日期**: 2026-03-15  
**下次审计**: 建议在下次版本发布前