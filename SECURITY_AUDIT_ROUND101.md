# 安全审计报告 - 第101轮

**审计日期**: 2026-03-30  
**审计范围**: nas-os-fix 项目  
**版本**: v2.253.289+

---

## 一、govulncheck 漏洞扫描

### 执行状态
⚠️ **无法完成** - 项目存在编译错误

编译错误详情：
- `internal/storage/wake_priority_queue.go`: Pop 方法签名错误 (应为 `any` 类型)
- `internal/ai/face/cluster.go`: 类型错误和未使用变量
- `internal/ai/advisor.go`: 未定义方法 `GetProvider`
- `internal/album/ai/clip.go`: 多处未定义函数和错误调用
- `internal/security/event_monitor.go`: 与 `audit.go` 类型重复定义

### 建议
1. 先修复编译错误
2. 重新运行 `govulncheck ./...`

---

## 二、新增代码安全性检查

### 审计重点文件
| 文件 | 变更行数 | 风险等级 |
|---|---|---|
| `internal/security/audit.go` | +1079 | 中 |
| `internal/iscsi/manager.go` | +23 | 低 |
| `internal/network/diagnostics.go` | +19 | 低 |
| `internal/notify/notifier.go` | +19 | 低 |

### 正面发现 ✅

1. **Context 超时控制** - 新增代码正确使用 `context.Context`：
   - `iscsi/manager.go`: 所有 exec.Command 改为 exec.CommandContext
   - `network/diagnostics.go`: 网络操作添加超时控制
   - `notify/notifier.go`: HTTP 请求添加 context 和 10s 超时

2. **密码传递方式安全** - `iscsi/manager.go`:
   ```go
   execCmd.Stdin = strings.NewReader(secret)  // 避免命令行泄露
   ```
   密码通过 stdin 传递，不暴露在命令行参数中 ✅

3. **无硬编码敏感信息** - BotToken、ChatID、SMTP 配置等均为结构体字段，从配置加载

4. **敏感文件检测** - 新增敏感文件模式检测：
   ```go
   sensitivePatterns := []string{"password", "secret", "key", "credential", ".pem", ".key", ".env"}
   ```

### 风险发现 ⚠️

1. **类型重复定义** - 高风险
   - `internal/security/event_monitor.go` 和 `audit.go` 都定义了：
     - `AnomalyDetector` 结构体（字段不同）
     - `AnomalyResult` 结构体（字段不同）
     - `NewAnomalyDetector()` 函数
   - 导致编译失败，需重构

2. **HTTP 客户端未设置 TLS 配置** - 中风险
   - `notify/notifier.go` 新增 HTTP POST 但未显式配置 TLS 验证
   - 建议：添加 `InsecureSkipVerify: false` 显式声明

---

## 三、API 权限控制验证

### 现有机制 ✅

1. **中间件架构完善** - `internal/rbac/middleware.go`:
   - `UserIDKey`, `UserRoleKey`, `PermissionKey` 上下文键
   - `SkipPaths` 和 `PublicPaths` 路径配置
   - Token 提取函数 `ExtractBearerToken`

2. **路由注释规范** - `internal/security/handlers.go`:
   ```go
   // 注意：调用方应在应用此路由组前添加认证和权限中间件
   // 这些都是敏感的安全操作，应该限制为管理员权限
   ```

3. **权限分组**:
   - `/firewall/*` - 标记需要管理员权限
   - `/fail2ban/*` - 标记需要管理员权限
   - `/audit/*` - 部分接口需要管理员权限

### 风险点 ⚠️

1. **依赖调用方添加中间件** - 审计模块无内置权限检查
   - 如果调用方忘记添加中间件，API 可能暴露
   - 建议：在 handler 内部添加权限验证逻辑作为备用

2. **新增 audit.go 无路由定义** - 需确认：
   - 新增的 `AlertNotifier`, `AnomalyDetector` 是否暴露为 API？
   - 如果暴露，需添加权限中间件

---

## 四、敏感信息泄露风险检查

### 检查项

| 检查项 | 状态 |
|---|---|
| 硬编码密码/Token | ✅ 无发现 |
| 命令行密码泄露 | ✅ 已修复 (stdin 传递) |
| 日志泄露敏感信息 | ✅ 无日志输出敏感字段 |
| 响应包含敏感字段 | ⚠️ 需验证 |

### 需关注

1. `TelegramConfig.BotToken` - 配置字段，JSON 序列化时需注意：
   - 建议添加 `json:"bot_token,omitempty"` 避免空值暴露
   - 或使用自定义 JSON marshal 隐藏敏感字段

2. `AlertNotifier` 通知内容 - 需确认：
   - 通知消息是否包含敏感操作详情？
   - 是否泄露文件路径/用户名等？

---

## 五、总结与建议

### 高优先级
1. **修复编译错误** - 类型重复定义问题，需合并或重构 `event_monitor.go` 和 `audit.go`
2. **重新运行 govulncheck** - 编译修复后执行完整漏洞扫描

### 中优先级
1. 为新增 API 显式添加权限验证（不依赖调用方）
2. HTTP 客户端显式配置 TLS 安全选项
3. 检查 JSON 序列化是否泄露敏感配置字段

### 低优先级
1. 添加敏感信息字段的 JSON marshal 自定义处理
2. 完善单元测试覆盖新增安全功能

---

**审计人**: 刑部  
**状态**: 部分完成（编译错误阻塞漏洞扫描）