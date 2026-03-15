# NAS-OS v2.71.0 安全审计报告

**审计日期**: 2026-03-15
**版本**: v2.71.0
**审计部门**: 刑部

---

## 一、执行摘要

### 安全评级：⚠️ 需要关注 (B)

| 评估项 | 得分 | 说明 |
|--------|------|------|
| 代码静态分析 | ✅ A | go vet 无问题 |
| 代码格式规范 | ✅ A | gofmt 无问题 |
| Go 标准库漏洞 | ❌ C | 5 个已知漏洞，需升级 Go |
| 依赖安全性 | ✅ A | 无已知漏洞 |
| 敏感数据处理 | ✅ A | 无硬编码密钥 |
| LICENSE 合规 | ✅ A | MIT License 正确 |

---

## 二、版本变更分析

### 2.1 变更概览

v2.71.0 版本主要变更：

```
8f6d9b7 chore: 版本更新到 v2.71.0
a9f4a46 fix: 修复 disk/shares 测试代码类型错误
```

### 2.2 变更文件

| 文件 | 变更类型 | 风险评估 |
|------|----------|----------|
| `internal/version/version.go` | 版本号更新 | ✅ 低风险 |
| `internal/disk/handlers.go` | 接口抽象 | ✅ 低风险 - 改用接口类型 |
| `internal/shares/handlers.go` | 接口抽象 | ✅ 低风险 - 改用接口类型 |
| `internal/disk/handlers_test.go` | 测试修复 | ✅ 无风险 |
| `internal/shares/handlers_test.go` | 测试重构 | ✅ 无风险 |
| `internal/shares/interfaces.go` | 新增接口 | ✅ 低风险 |
| `internal/disk/monitor.go` | 新增接口 | ✅ 低风险 |
| `internal/dashboard/health/checker_test.go` | 新增测试 | ✅ 无风险 |

### 2.3 安全影响分析

**变更性质**: 测试代码修复 + 接口抽象重构

**安全评估**: 
- 无安全相关代码变更
- 接口抽象提升了可测试性和可维护性
- 测试覆盖率提升有助于发现潜在问题

---

## 三、Go 标准库漏洞（重要）

### 🔴 发现 5 个 Go 标准库漏洞

| 编号 | CVE | 漏洞描述 | 当前版本 | 修复版本 |
|------|-----|----------|----------|----------|
| GO-2026-4603 | CVE-2026-XXXX | html/template 不转义特定字符 | go1.26 | go1.26.1 |
| GO-2026-4602 | CVE-2026-XXXX | os.FileInfo 可从 Root 逃逸 | go1.26 | go1.26.1 |
| GO-2026-4601 | CVE-2026-XXXX | net/url IPv6 解析错误 | go1.26 | go1.26.1 |
| GO-2026-4600 | CVE-2026-XXXX | crypto/x509 证书名称约束检查异常 | go1.26 | go1.26.1 |
| GO-2026-4599 | CVE-2026-XXXX | crypto/x509 邮件约束强制执行错误 | go1.26 | go1.26.1 |

### 影响范围

```
#1: internal/reports/enhanced_export.go - template.Template.Execute
#2: internal/reports/cost_report.go - os.ReadDir
#3: internal/office/manager.go - url.Parse
#4: internal/ldap/client.go - x509.Certificate.Verify
#5: internal/ldap/client.go - x509.Certificate.Verify
```

### 🛠️ 修复建议

```bash
# 升级 Go 版本到 1.26.1 或更高
go install golang.org/dl/go1.26.1@latest
go1.26.1 download

# 更新项目的 go.mod
# go 1.26 -> go 1.26.1
```

**优先级**: 🔴 高 - 建议立即修复

---

## 四、依赖包安全检查

### 4.1 可用更新（非关键）

以下依赖包有可用更新，但非安全相关：

| 包名 | 当前版本 | 最新版本 |
|------|----------|----------|
| github.com/aws/aws-sdk-go-v2 | v1.41.3 | v1.41.4 |
| github.com/aws/aws-sdk-go-v2/config | v1.32.11 | v1.32.12 |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.96.4 | v1.97.1 |
| github.com/RoaringBitmap/roaring/v2 | v2.4.5 | v2.15.0 |
| github.com/Azure/go-ntlmssp | v0.0.0-20221128193559 | v0.1.0 |

### 4.2 安全特性依赖

项目包含以下安全相关依赖：

| 依赖 | 用途 | 安全评估 |
|------|------|----------|
| golang.org/x/crypto | 加密库 | ✅ 最新版本 |
| github.com/pquerna/otp | TOTP 支持 | ✅ 正常使用 |
| github.com/go-ldap/ldap/v3 | LDAP 认证 | ✅ 正常使用 |

---

## 五、代码安全审计

### 5.1 静态分析结果

```bash
$ go vet ./...
(无输出 - 通过)

$ gofmt -l .
(无输出 - 通过)
```

### 5.2 敏感数据处理检查

✅ **无硬编码密钥/密码**

检查了以下模式：
- `password` - 仅用于配置结构体字段和参数
- `secret` - 仅用于配置结构体字段
- `api_key` - 未发现
- `token` - 仅用于认证令牌相关逻辑

### 5.3 SQL 注入检查

✅ **无 SQL 注入风险**

所有数据库查询使用参数化查询：
```go
rows, err := m.db.Query(query, "%"+keyword+"%")
```

### 5.4 命令注入检查

⚠️ **存在 exec.Command 调用** - 需持续关注

项目使用外部命令的场景：

| 模块 | 命令 | 风险评估 |
|------|------|----------|
| backup | tar, rsync, openssl, scp | ✅ 参数可控 |
| snapshot | btrfs send/receive | ✅ 系统命令 |
| reports | wkhtmltopdf | ⚠️ 需验证输入 |

**建议**: 确保所有外部命令参数经过验证和转义

---

## 六、安全特性评估

### 6.1 认证安全 ✅

| 特性 | 状态 | 说明 |
|------|------|------|
| 密码策略 | ✅ 完善 | 最小长度、复杂度、历史记录 |
| MFA 支持 | ✅ 完整 | TOTP, SMS, WebAuthn |
| 会话管理 | ✅ 完善 | 过期、缓存、失效机制 |
| 密码加密 | ✅ 安全 | AES-256-CBC 加密备份 |

### 6.2 密码策略详情

```go
DefaultPasswordPolicy:
  MinLength: 8
  MaxLength: 128
  RequireUppercase: true
  RequireLowercase: true
  RequireDigit: true
  RequireSpecial: true
  MinSpecialCount: 1
  PreventCommon: true    // 阻止常见弱密码
  PreventUserInfo: true  // 阻止用户信息
  HistoryCount: 5        // 密码历史
  MaxAge: 90            // 90天过期
```

### 6.3 安全审计日志 ✅

- 审计事件记录：认证、MFA、密码、会话
- 日志轮转：100MB 限制，最多 10 个文件
- 文件权限：0640（仅所有者可写）

---

## 七、LICENSE 合规检查

### 许可证类型：MIT License

```
MIT License
Copyright (c) 2025 nas-os
```

✅ **合规状态**：
- 开源许可证
- 允许商业使用
- 允许修改和分发
- 需保留版权声明

### 依赖许可证兼容性

主要依赖许可证均为兼容：
- Apache-2.0 (AWS SDK, Gin, etc.)
- BSD-3-Clause (大多数 Go 库)
- MIT (部分库)

---

## 八、建议和改进

### 🔴 高优先级

1. **升级 Go 版本** - 修复 5 个标准库漏洞
   ```bash
   # 推荐升级到 Go 1.26.1
   ```

### 🟡 中优先级

2. **更新 AWS SDK** - 虽非安全更新，但保持最新
   ```bash
   go get github.com/aws/aws-sdk-go-v2@v1.41.4
   ```

3. **添加依赖漏洞扫描** - 集成 govulncheck 到 CI
   ```yaml
   # .github/workflows/security.yml
   - name: Vulncheck
     run: go install golang.org/x/vuln/cmd/govulncheck@latest && govulncheck ./...
   ```

### 🟢 低优先级

4. **增强命令执行安全** - 添加输入验证包装器
5. **定期审计** - 建议每月执行依赖安全扫描

---

## 九、结论

v2.71.0 版本代码变更安全，主要是测试代码修复和接口抽象重构。

**主要风险**：Go 标准库存在 5 个已知漏洞，需升级到 Go 1.26.1。

**整体评价**：代码质量良好，安全机制完善。升级 Go 版本后可达到 A 级。

---

**审计完成时间**: 2026-03-15 22:15
**下次审计建议**: 升级 Go 版本后重新扫描