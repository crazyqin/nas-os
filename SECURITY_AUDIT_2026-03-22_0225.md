# NAS-OS 安全审计报告
**审计时间**: 2026-03-22 02:25 GMT+8  
**审计部门**: 刑部  
**项目版本**: v2.253.146  
**项目路径**: /home/mrafter/clawd/nas-os

---

## 一、执行摘要

| 检查项 | 状态 | 说明 |
|--------|------|------|
| gosec 静态扫描 | ⚠️ 860 条告警 | 较上次减少 29 条 (889→860) |
| 敏感信息泄露 | ✅ 通过 | 无硬编码密钥，.env.example 使用占位符 |
| 新漏洞引入 | ✅ 无 | 无新增高危漏洞 |
| RBAC 权限控制 | ✅ 完善 | 完整的角色/策略/ACL/审计实现 |
| 密码存储 | ✅ 安全 | bcrypt + AES-256-GCM + argon2id |

---

## 二、gosec 扫描结果

### 2.1 统计概览

| 指标 | 本次扫描 | 上次扫描 | 变化 |
|------|----------|----------|------|
| 文件数 | 462 | 462 | - |
| 代码行数 | 285,626 | 284,901 | +725 |
| 告警总数 | 860 | 889 | -29 |
| #nosec 忽略 | 78 | 74 | +4 |

### 2.2 HIGH 级别问题分布

| 规则 | 数量 | 说明 | 风险评估 |
|------|------|------|----------|
| G115 | 67 | 整数溢出转换 | 低风险 - 需逐项评估 |
| G703 | 48 | 路径遍历(污点分析) | 中风险 - 已有防护 |
| G702 | 10 | 命令注入(污点分析) | 中风险 - 需审查 |
| G101 | 7 | 潜在硬编码凭证 | 误报 - 均为 URL/错误消息 |
| G122 | 7 | TOCTOU 竞争条件 | 低风险 - 文件遍历回调 |
| G118 | 1 | Context 泄漏 | 低风险 |
| G402 | 1 | TLS 跳过验证 | 已缓解 - 仅测试环境 |
| G404 | 1 | 弱随机数 | 低风险 |
| G707 | 1 | SMTP 注入 | 需审查 |

### 2.3 MEDIUM 级别问题分布

| 规则 | 数量 | 说明 |
|------|------|------|
| G304 | 216 | 文件路径通过变量构造 |
| G204 | 175 | 子进程启动使用变量 |
| G301 | 165 | 目录创建权限 (0755) |
| G306 | 135 | 文件写入权限 (0644) |
| G107 | 5 | HTTP 请求使用变量 URL |
| G110 | 3 | 解压缩风险 |

---

## 三、关键安全问题分析

### 3.1 G703 路径遍历 (48 处)

**主要位置**:
- `internal/webdav/server.go` - WebDAV 文件操作
- `internal/backup/encrypt.go` - 备份加密
- `plugins/filemanager-enhance/main.go` - 文件管理插件

**缓解措施**:
```go
// internal/webdav/server.go:205
cleanPath := filepath.Clean("/" + decodedPath)
// 验证路径不以 .. 开头
if strings.Contains(decodedPath, "..") {
    return "", errors.New("invalid path")
}
```

**评估**: WebDAV 模块已有路径清洗和验证，风险可控。建议添加单元测试覆盖边界情况。

### 3.2 G702 命令注入 (10 处)

**主要位置**:
- `internal/snapshot/replication.go` - btrfs send/receive
- `internal/snapshot/executor.go` - 脚本执行

**示例**:
```go
// internal/snapshot/executor.go:194
cmd := exec.CommandContext(ctx, "sh", "-c", script)
```

**评估**: 脚本来源为用户配置，需确保输入验证。建议：
1. 限制可用命令白名单
2. 添加脚本沙箱执行

### 3.3 G402 TLS 跳过验证 (1 处)

**位置**: `internal/auth/ldap.go:159`

```go
skipVerify := config.SkipTLSVerify && os.Getenv("ENV") == "test"
tlsConfig := &tls.Config{
    InsecureSkipVerify: skipVerify,
}
```

**评估**: 安全实现 - 仅在测试环境且配置允许时跳过验证。

### 3.4 G707 SMTP 注入 (1 处)

**位置**: `internal/automation/action/action.go:287`

```go
return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
```

**建议**: 对邮件头字段进行转义，防止 CRLF 注入。

---

## 四、RBAC 配置审查

### 4.1 架构评估

**组件完整性**: ✅ 完善
- 角色定义: `internal/auth/rbac.go` - 支持 Admin/User/Guest/System
- 权限检查: `internal/auth/rbac_middleware.go` - 中间件实现
- ACL 管理: `internal/rbac/share_acl.go` - 资源级权限
- 审计日志: `internal/rbac/audit.go` - 完整审计跟踪

### 4.2 安全特性

| 特性 | 状态 | 说明 |
|------|------|------|
| 角色继承 | ✅ | 支持角色继承链，防止循环依赖 |
| 会话缓存 | ✅ | 5 分钟 TTL，自动清理过期会话 |
| 默认拒绝 | ✅ | `StrictMode: true` 默认配置 |
| IP 黑白名单 | ✅ | 中间件支持 IP 访问控制 |
| 审计日志 | ✅ | 记录权限检查、拒绝、认证失败 |
| 所有权检查 | ✅ | 资源所有者自动获得全部权限 |

### 4.3 潜在改进建议

1. **权限缓存失效**: 添加角色变更后主动刷新缓存
2. **权限模板**: 已有 readonly/editor/operator 模板，可扩展更多场景
3. **API 文档**: 添加 RBAC API 的 OpenAPI 文档

---

## 五、敏感信息泄露检查

### 5.1 硬编码凭证检查

**检查命令**:
```bash
grep -rn "sk-[a-zA-Z0-9]{20,}\|AKIA[A-Z0-9]{16}\|ghp_[a-zA-Z0-9]{36}" --include="*.go"
```

**结果**: ✅ 无真实凭证泄露

**G101 误报分析**:
- `internal/auth/oauth2.go` - OAuth2 配置 URL（非密钥）
- `internal/cloudsync/providers.go` - OAuth Token URL（非密钥）
- `internal/office/types.go` - 错误消息常量

### 5.2 配置文件检查

**.env.example**: ✅ 使用 `changeme` 占位符
```env
GRAFANA_ADMIN_PASSWORD=changeme
SMTP_PASSWORD=changeme
```

### 5.3 密钥存储检查

| 数据类型 | 存储方式 | 位置 |
|----------|----------|------|
| 用户密码 | bcrypt | internal/users/manager.go |
| TOTP 密钥 | AES-256-GCM | internal/auth/secret_encryption.go |
| 备份密钥 | argon2id | internal/security/v2/manager.go |

---

## 六、与上次审计对比

### 6.1 新增问题

通过 diff 分析，新增约 30 处告警，主要在：
- `internal/backup/config_backup.go` - 配置备份路径处理
- `internal/backup/encrypt.go` - 加密文件操作

均为已知模式（G304/G301/G306），无新增高危漏洞类型。

### 6.2 已修复问题

- 告警总数减少 29 条
- #nosec 注释增加 4 处（开发团队已标记已知风险）

---

## 七、修复建议

### 7.1 高优先级

| 问题 | 建议 | 工作量 |
|------|------|--------|
| G707 SMTP 注入 | 对邮件头字段进行 CRLF 过滤 | 小 |
| G702 命令注入 | 添加脚本命令白名单 | 中 |

### 7.2 中优先级

| 问题 | 建议 | 工作量 |
|------|------|--------|
| G703 路径遍历 | 添加路径遍历单元测试 | 中 |
| G115 整数溢出 | 逐项评估，添加边界检查 | 大 |

### 7.3 低优先级

| 问题 | 建议 |
|------|------|
| G122 TOCTOU | 考虑使用 os.Root (Go 1.24+) |
| G301/G306 文件权限 | 评估是否需要更严格的权限 |

---

## 八、结论

**本次审计结论**: ✅ 安全状况良好

- 无新增高危漏洞
- 无敏感信息泄露
- RBAC 架构完善，实现正确
- 告警数持续减少，安全态势向好

**建议关注**:
1. SMTP 注入风险 (G707) - 建议尽快修复
2. 命令注入防护 (G702) - 添加输入验证/白名单
3. 定期复查 gosec 告警，跟踪修复进展

---

**刑部**  
2026-03-22