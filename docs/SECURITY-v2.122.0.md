# 安全审计报告 v2.122.0

**审计日期**: 2026-03-16  
**审计部门**: 刑部（法务合规）  
**项目**: nas-os  
**版本**: v2.122.0

---

## 一、审计概要

### 1.1 代码统计

| 指标 | 数值 |
|------|------|
| Go 源文件总数 | 665 个 |
| 测试文件数 | 211 个 |
| 总代码行数 | 277,680 行 |
| 依赖包数量 | 245 个 |
| go vet 问题 | 0 |

### 1.2 gosec 扫描结果

| 严重级别 | 数量 |
|----------|------|
| HIGH | 171 |
| MEDIUM | 796 |
| LOW | 701 |
| **总计** | **1,668** |

### 1.3 主要风险分布

| 规则ID | 描述 | 数量 |
|--------|------|------|
| G104 | 未检查错误返回值 | 701 |
| G304 | 文件路径注入风险 | 230 |
| G204 | 命令执行风险 | 192 |
| G301 | 目录权限问题 | 181 |
| G306 | 文件权限问题 | 154 |
| G115 | 整数溢出转换 | 91 |
| G703 | defer 中调用主函数 | 48 |

---

## 二、安全风险分析

### 2.1 🔴 高危风险

#### 2.1.1 硬编码凭证 (G101)

发现 7 处疑似硬编码凭证，需人工复核：

```
internal/auth/oauth2.go:334-389 - OAuth2 提供商配置函数
internal/cloudsync/providers.go:670 - Token URL 硬编码
internal/cloudsync/providers.go:1320 - API 端点硬编码
internal/office/types.go:581 - 错误常量
```

**风险评级**: 中等（多为配置模板函数，非实际凭证）

**建议**: 
- OAuth2 ClientID/ClientSecret 应通过环境变量或配置文件注入
- 确认 `internal/office/types.go:581` 是否为误报

#### 2.1.2 整数溢出转换 (G115)

发现 91 处整数类型转换可能导致溢出：

```go
// 示例位置
internal/system/monitor.go:760-761 - uint64 → int64
internal/storage/smart_monitor.go:606-617 - uint64 → int
internal/monitor/metrics_collector.go:843-849 - uint64 → int
```

**风险评级**: 高

**建议**:
- 添加边界检查，确保转换前值在目标类型范围内
- 使用 `math.MaxInt64` 等常量进行校验

#### 2.1.3 文件路径注入 (G304)

发现 230 处文件路径使用用户输入，涉及 80 个文件：

```
高风险文件:
- internal/backup/restore.go
- internal/files/manager.go
- plugins/filemanager-enhance/main.go
- internal/reports/exporter.go
```

**风险评级**: 高

**现状分析**:
项目已有安全工具包，但未充分使用：
- `pkg/safeguards/paths.go` - 提供 `SecureJoin`, `SafeReadFile`, `SafeWriteFile`
- `pkg/security/sanitize.go` - 提供 `SafePath`, `ValidateFilename`, `SanitizeCommandArg`
- `internal/security/v2/safe_file_manager.go` - 提供文件管理器级别的安全封装

**问题**: 经检查，`internal/backup/restore.go` 等关键文件未导入和使用这些安全函数。

**建议**:
1. 在所有文件操作中使用 `safeguards.SecureJoin()` 替代 `filepath.Join()`
2. 使用 `safeguards.SafeReadFile()` 替代 `os.ReadFile()`
3. 添加路径白名单和遍历检测
4. 对关键模块进行安全重构

### 2.2 🟠 中危风险

#### 2.2.1 命令执行风险 (G204)

发现 192 处命令执行，涉及 35 个文件：

```go
// 示例
internal/vm/manager.go:369 - exec.Command("qemu-img", ...)
internal/security/v2/disk_encryption.go:561 - exec.Command("cryptsetup", ...)
pkg/btrfs/btrfs.go:537 - 执行 btrfs 命令
```

**风险评级**: 中

**建议**:
- 避免使用 `sh -c` 执行拼接命令
- 对命令参数进行严格校验
- 使用固定路径查找可执行文件

#### 2.2.2 错误处理缺失 (G104)

发现 701 处未检查返回错误：

```go
// 常见模式
defer f.Close()  // 未检查错误
result, _ := someFunc()  // 忽略错误
```

**风险评级**: 中

**建议**:
- 对关键操作的错误必须处理
- defer 中的 Close 可使用 `defer func() { _ = f.Close() }` 明确忽略

#### 2.2.3 文件权限问题 (G301/G306)

- G301: 181 处目录权限问题
- G306: 154 处文件权限问题

**建议**:
- 敏感目录权限应为 0750 或更严格
- 配置文件权限应为 0600
- 使用 `os.MkdirAll(path, 0750)` 替代 0755

### 2.3 🟡 低危风险

#### 2.3.1 panic 使用

发现 4 处 panic 调用：

```go
internal/quota/manager.go:636 - crypto/rand 失败
internal/tags/manager.go:132 - crypto/rand 失败
internal/users/manager.go:264,273 - crypto/rand 失败
internal/users/manager.go:967 - 系统随机数失败
```

**风险评级**: 低（合理使用场景）

**说明**: crypto/rand 失败时 panic 是可接受的做法，因为这意味着系统熵源故障。

---

## 三、敏感信息处理审计

### 3.1 凭证存储

代码中敏感信息的存储和传输处理：

| 模块 | 敏感字段 | 加密存储 | 备注 |
|------|----------|----------|------|
| internal/backup/credentials.go | SecretKey, Password | ✅ 加密 | 使用 cs.Encrypt() |
| internal/cloudsync/providers.go | AccessKey, SecretKey | ✅ 加密 | BasicAuth 传输 |
| internal/cache/redis.go | Redis Password | ⚠️ 明文配置 | 需确认配置文件权限 |
| internal/auth/oauth2.go | ClientSecret | ⚠️ 配置注入 | 需环境变量管理 |

### 3.2 密钥派生

```go
// internal/backup/encrypt.go:175
key := pbkdf2.Key([]byte(passphrase), salt, em.config.Iterations, 32, sha256.New)
```

**评价**: 使用 PBKDF2 进行密钥派生，安全性可接受。建议：
- 迭代次数至少 100,000 次
- 考虑升级到 Argon2

---

## 四、输入验证审计

### 4.1 API 输入绑定

使用 Gin 框架的 ShouldBind 系列方法进行输入验证：

```go
// 标准模式
if err := c.ShouldBindJSON(&req); err != nil {
    return c.JSON(400, gin.H{"error": err.Error()})
}
```

**评价**: 
- ✅ 使用结构体绑定和验证
- ⚠️ 部分接口缺少自定义验证器
- 建议：添加 `validator.v10` 的自定义验证规则

### 4.2 SQL 注入防护

```go
// internal/system/monitor.go:169
db, err := sql.Open("sqlite", dbPath)

// 使用参数化查询
rows, err := m.db.Query(query, timeRange)
```

**评价**: ✅ 使用参数化查询，无 SQL 注入风险

---

## 五、依赖安全检查

### 5.1 关键依赖版本

| 依赖 | 版本 | 状态 |
|------|------|------|
| golang.org/x/crypto | v0.48.0 | ✅ 最新 |
| github.com/gin-gonic/gin | v1.11.0 | ✅ 最新 |
| modernc.org/sqlite | v1.34.5 | ✅ 最新 |
| go.etcd.io/bbolt | v1.4.0 | ✅ 最新 |
| github.com/aws/aws-sdk-go-v2 | v1.41.3 | ✅ 最新 |

### 5.2 govulncheck 漏洞扫描结果

**扫描时间**: 2026-03-16 17:30  
**Go 版本**: go1.26.0 (需升级至 go1.26.1)

发现 **5 个 Go 标准库漏洞**：

| 漏洞编号 | 组件 | 风险 | 影响 |
|----------|------|------|------|
| GO-2026-4603 | html/template | 中 | URL meta content 未转义 |
| GO-2026-4602 | os | 中 | FileInfo 可从 Root 逃逸 |
| GO-2026-4601 | net/url | 中 | IPv6 主机解析错误 |
| GO-2026-4600 | crypto/x509 | 高 | 证书名称约束检查可导致 panic |
| GO-2026-4599 | crypto/x509 | 中 | 邮件约束验证不正确 |

**受影响调用链**：
- `internal/reports/enhanced_export.go:308` - template.Execute
- `internal/ldap/client.go:83` - x509 证书验证
- `internal/office/manager.go:1158` - URL 解析
- `internal/cloudsync/providers.go:1774` - OneDrive 连接测试

**修复建议**：
```bash
# 升级 Go 到 1.26.1
# 在 CI/CD 中添加 govulncheck 步骤
govulncheck ./...
```

### 5.3 建议

- ✅ 升级 Go 至 1.26.1 修复标准库漏洞
- 定期运行 `go list -m -u all` 检查更新
- 建立 govulncheck CI 检查流程
- 考虑使用 Dependabot 自动化依赖更新

---

## 六、代码规范检查

### 6.1 注释覆盖率

- 有注释的源文件：453 / 665 (68%)
- 建议提高核心模块的文档覆盖率

### 6.2 测试覆盖率

- 测试文件：211 个
- 已有覆盖率文件，建议保持 80% 以上

### 6.3 命名规范

- ✅ 遵循 Go 命名规范
- ✅ 包名小写、简短
- ✅ 导出函数有注释

---

## 七、改进建议

### 7.1 紧急修复 (P0)

1. **文件路径注入**: 实现路径白名单和遍历检测
2. **整数溢出**: 添加关键位置的边界检查

### 7.2 高优先级 (P1)

1. **错误处理**: 修复 G104 问题，特别是 defer Close() 调用
2. **命令执行**: 审计所有 exec.Command 调用，添加参数校验
3. **文件权限**: 敏感文件和目录权限收紧

### 7.3 中优先级 (P2)

1. **依赖更新**: 建立 govulncheck CI 检查
2. **日志脱敏**: 确保敏感信息不进入日志
3. **文档完善**: 提高核心模块注释覆盖率

### 7.4 低优先级 (P3)

1. 代码静态分析集成到 CI/CD
2. 安全编码培训材料更新

---

## 八、总结

本次安全审计对 nas-os v2.122.0 版本进行了全面的安全检查：

**整体评价**: 代码安全水平良好，但存在若干需要关注的安全风险。

**关键发现**:
- 1,668 个 gosec 告警，其中 171 个高危
- 主要风险集中在文件路径处理和整数溢出
- 敏感信息处理机制基本完善
- 依赖版本保持最新

**下一步行动**:
1. 优先处理文件路径注入和整数溢出问题
2. 建立定期安全扫描机制
3. 完善安全开发规范

---

*审计人: 刑部*  
*审计日期: 2026-03-16*