# NAS-OS 安全审计报告 - 第163轮

**审计日期**: 2026-04-04
**版本**: v2.393.0
**审计范围**: 漏洞扫描、敏感信息泄露、RBAC配置、输入验证

---

## 一、govulncheck 漏洞扫描

### 发现漏洞 (5个)

| ID | 漏洞描述 | 影响模块 | 修复版本 |
|---|---|---|---|
| GO-2026-4603 | html/template URL 未转义 | reports/web | go1.26.1 |
| GO-2026-4602 | os.FileInfo 可逃逸 Root | webshare/docker | go1.26.1 |
| GO-2026-4601 | net/url IPv6 解析错误 | office/cloudsync | go1.26.1 |
| GO-2026-4600 | x509 名称约束 panic | ldap | go1.26.1 |
| GO-2026-4599 | x509 邮件约束错误 | ldap | go1.26.1 |

### 建议
**立即升级 Go 版本至 go1.26.1+**，所有漏洞均为标准库问题，升级即可修复。

---

## 二、敏感信息泄露风险

### 低风险 (已做防护)
- ✅ 密码/密钥通过环境变量传递 (`NAS_BACKUP_KEY`, `NAS_CSRF_KEY`)
- ✅ LDAP/TLS 配置不硬编码
- ✅ OAuth token 存储有加密机制

### 需关注
| 文件 | 问题 | 建议 |
|---|---|---|
| `internal/users/manager.go:201` | 初始密码写入日志 | 移除或使用 `slog.Debug` |
| `internal/auth/enhanced_mfa_manager.go:137` | 解密失败打印用户ID | 使用脱敏日志 |
| `cmd/backup/main.go:365` | 密钥ID打印到终端 | 仅在调试模式显示 |

---

## 三、RBAC 权限配置评估

### 设计亮点 ✅
- **默认拒绝**: `StrictMode: true` 配置
- **分层权限**: 角色 → 用户组 → 直接权限 → 策略
- **缓存机制**: 5分钟 TTL，自动失效
- **审计日志**: `AuditEnabled` + 回调支持
- **策略优先级**: 支持显式拒绝策略

### 代码质量
- `internal/rbac/manager.go`: 完整的权限检查逻辑
- `internal/rbac/middleware.go`: Gin 中间件集成良好
- 测试覆盖: 有 `rbac_test.go`, `middleware_test.go`

### 建议
- 无重大问题，RBAC 设计合理

---

## 四、输入验证检查

### 文件路径遍历 ✅ 已防护
`plugins/filemanager-enhance/main.go:519-541`:
```go
// 拒绝 ".." 路径
if strings.Contains(path, "..") { return false }
// 安全校验：最终路径必须在根目录内
if !strings.HasPrefix(finalPath, cleanRoot+string(filepath.Separator)) { return false }
```

### SQL 注入 ✅ 已防护
使用参数化查询: `internal/tags/manager.go:728`
```go
rows, err := m.db.QueryContext(ctx, query, "%"+keyword+"%")
```

### 命令注入 ⚠️ 需审查
`internal/apps/manager.go` 多处使用 `exec.CommandContext`:
- 参数来源需确保已验证
- 当前实现固定命令路径（docker/tar/rsync），风险可控
- **建议**: 添加参数白名单验证

### 整数解析 ⚠️ 部分忽略错误
`internal/docker/app_handlers.go:470-471`:
```go
limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))  // 忽略错误
offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
```
**建议**: 添加错误检查或设置合理默认值上限

---

## 五、TLS/SSL 配置

### 安全实践 ✅
- `internal/ldap/client.go:73`: `InsecureSkipVerify: false` 默认值
- 仅测试环境 (`ENV=test`) 允许跳过验证
- 有 `#nosec G402` 注释标记

### 需确认
- `internal/media/scraper_enhanced.go:81`: 无条件跳过验证
  **建议**: 添加环境检查或配置开关

---

## 六、总结

| 类别 | 状态 | 优先级 |
|---|---|---|
| Go 标准库漏洞 | 5个需修复 | **高** |
| RBAC 配置 | 良好 | - |
| 敏感信息泄露 | 低风险 | 中 |
| 输入验证 | 大部分良好 | 中 |
| 命令注入风险 | 需审查 | 中 |

### 修复优先级

1. **立即**: 升级 Go 至 1.26.1+
2. **本周**: 修复日志敏感信息泄露
3. **下周**: 添加命令参数验证、整数解析错误检查

---

**审计人**: 刑部 (六部轮值)
**下次审计**: 第164轮