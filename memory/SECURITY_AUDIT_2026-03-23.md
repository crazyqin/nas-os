# NAS-OS 安全审计报告

**审计日期**: 2026-03-23
**审计人员**: 刑部
**审计范围**: /home/mrafter/nas-os

---

## 一、漏洞检查 (govulncheck)

### 依赖版本状态

| 依赖包 | 当前版本 | 状态 |
|--------|----------|------|
| github.com/quic-go/quic-go | v0.59.0 | ✅ 安全 |
| github.com/blevesearch/bleve/v2 | v2.5.7 | ✅ 安全 |
| golang.org/x/crypto | v0.49.0 | ✅ 安全 |
| golang.org/x/text | v0.35.0 | ✅ 安全 |
| golang.org/x/image | v0.37.0 | ✅ 安全 |
| github.com/gin-gonic/gin | v1.12.0 | ✅ 安全 |
| github.com/prometheus/client_golang | v1.23.2 | ✅ 安全 |
| github.com/gorilla/websocket | v1.5.3 | ✅ 安全 |
| google.golang.org/protobuf | v1.36.11 | ✅ 安全 |
| go.opentelemetry.io/otel/sdk | v1.42.0 | ✅ 安全 |

### 结论

**未发现已知漏洞**。所有关键依赖均为最新安全版本：

- quic-go: 修复了 CVE-2025-4233 (v0.57.0+)
- bleve: 修复了 GO-2022-0470 无认证访问 (v2.5.0+)
- gin: 修复了文件名注入和日志注入漏洞 (v1.9.1+)
- protobuf: 修复了无限循环漏洞 (v1.33.0+)
- otel/sdk: 修复了 PATH 劫持漏洞 (v1.40.0+)

---

## 二、敏感信息泄露风险

### 2.1 代码审查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 硬编码密码/密钥 | ✅ 未发现 | 代码中使用参数传递，无硬编码 |
| OAuth2 配置 | ✅ 安全 | clientSecret/appSecret 为函数参数，运行时配置 |
| 环境变量使用 | ✅ 安全 | 敏感配置通过 `os.Getenv` 加载 |
| API Key 存储 | ⚠️ 注意 | `SetAPIKey` 方法需确保不记录日志 |

### 2.2 配置文件审查

| 文件 | 状态 | 说明 |
|------|------|------|
| .env.example | ✅ 安全 | 使用 `changeme` 占位符，符合最佳实践 |
| .gitignore | ✅ 安全 | 排除了敏感文件 |
| docker-compose.prod.yml | ✅ 安全 | 从环境变量读取敏感配置 |

### 2.3 建议

1. **日志脱敏**: 确保 `SetAPIKey` 等方法不会在日志中记录敏感值
2. **密钥轮换**: 建议实施定期密钥轮换机制
3. **审计日志**: 对敏感操作增加审计日志记录

---

## 三、RBAC 配置安全性

### 3.1 架构评估

| 特性 | 状态 | 说明 |
|------|------|------|
| 默认拒绝策略 | ✅ 已实现 | `StrictMode: true` |
| 角色分层 | ✅ 已实现 | admin > operator > readonly > guest |
| 权限继承 | ✅ 已实现 | 支持用户组权限继承 |
| 策略优先级 | ✅ 已实现 | 支持 Allow/Deny 策略，Deny 优先 |
| 缓存机制 | ✅ 已实现 | 5分钟 TTL，可配置 |
| 审计日志 | ✅ 已实现 | 可配置审计回调 |
| 持久化安全 | ✅ 已实现 | 文件权限 0600 |

### 3.2 权限模型

```
admin     -> *:* (完全控制)
operator  -> system/storage/share/network/service/backup:read+write
readonly  -> system/storage/share/network/service/log/monitor:read
guest     -> system:read
```

### 3.3 安全特性

- ✅ **最小权限原则**: 默认角色按职责分配最小必要权限
- ✅ **拒绝优先**: Deny 策略优先于 Allow
- ✅ **通配符支持**: 支持 `resource:*` 和 `*:action` 格式
- ✅ **上下文传递**: 用户信息通过 context 安全传递

### 3.4 建议

1. **会话超时**: 添加权限缓存刷新机制，角色变更后立即生效
2. **密码策略**: 在用户管理模块增强密码复杂度要求
3. **MFA 支持**: 考虑为管理员角色增加多因素认证

---

## 四、其他安全发现

### 4.1 良好实践

- ✅ iSCSI 密码通过 stdin 传递，避免命令行泄露
- ✅ LDAP SkipTLSVerify 仅在测试环境启用
- ✅ 插件加载有网络限制 (`ALLOW_HTTP_PLUGIN`, `ALLOW_PRIVATE_NETWORK_PLUGIN`)
- ✅ CSRF 保护已实现 (`NAS_CSRF_KEY`)

### 4.2 待改进项

| 项目 | 风险等级 | 建议 |
|------|----------|------|
| JWT 密钥配置 | 中 | 确保 `ONLYOFFICE_JWT_SECRET` 使用强随机值 |
| SMTP 密码 | 中 | 建议使用密钥管理服务 |
| 插件签名验证 | 低 | 考虑增加插件签名验证机制 |

---

## 五、总结

### 安全评级: ⭐⭐⭐⭐☆ (良好)

| 领域 | 评分 | 说明 |
|------|------|------|
| 依赖安全 | 5/5 | 所有依赖已更新到安全版本 |
| 代码安全 | 4/5 | 无硬编码密钥，日志脱敏可加强 |
| RBAC 设计 | 5/5 | 实现完善，符合最小权限原则 |
| 配置安全 | 4/5 | 环境变量使用正确，建议增强密钥管理 |

### 优先修复建议

1. **立即**: 确保 `.env` 文件不被提交到版本控制
2. **短期**: 增加敏感操作的审计日志
3. **中期**: 考虑集成密钥管理服务 (如 HashiCorp Vault)

---

**审计完成**