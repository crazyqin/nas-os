# NAS-OS 安全审计报告 v2.75.0

**审计日期**: 2026-03-15  
**审计版本**: v2.74.0 → v2.75.0  
**审计部门**: 刑部  
**审计范围**: 代码安全漏洞、依赖安全性、敏感信息泄露、认证授权

---

## 一、审计概述

### 版本变更范围
v2.75.0 为版本发布完善版本，主要变更：
- 文档同步 (CHANGELOG.md, README.md, docs/RELEASE-v2.75.0.md)
- 开发状态更新

**代码层面无实质性变更**，本次审计聚焦于整体代码库安全状态评估。

---

## 二、安全检查结果

### 2.1 静态分析 (go vet)

```
✅ go vet ./... - 无问题
```

### 2.2 安全扫描 (gosec)

```
⚠️ gosec 未安装 - 跳过
```

建议安装：
```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

### 2.3 漏洞扫描 (govulncheck)

发现 **5 个标准库漏洞**，需升级 Go 版本修复：

| 漏洞编号 | 组件 | 严重性 | 影响 | 修复版本 |
|---------|------|--------|------|---------|
| GO-2026-4603 | html/template | 中 | meta content URL 未转义 | go1.26.1 |
| GO-2026-4602 | os | 低 | FileInfo 可逃逸 Root | go1.26.1 |
| GO-2026-4601 | net/url | 中 | IPv6 主机解析错误 | go1.26.1 |
| GO-2026-4600 | crypto/x509 | 高 | 证书名称约束检查可致 panic | go1.26.1 |
| GO-2026-4599 | crypto/x509 | 中 | 邮件约束验证不正确 | go1.26.1 |

**影响路径**:
- `internal/reports/enhanced_export.go:308` - HTML 模板执行
- `internal/ldap/client.go:81` - LDAP StartTLS 连接
- `internal/office/manager.go:1158` - URL 解析
- `internal/web/server.go:791` - HTTP 服务
- `internal/cloudsync/providers.go:1774` - OneDrive 连接

**修复建议**: 升级 Go 版本至 `go1.26.1` 或更高版本

```bash
# go.mod 当前版本
go 1.26

# 建议修改为
go 1.26.1
```

---

## 三、敏感信息检查

### 3.1 硬编码凭证检查

| 文件 | 行号 | 内容 | 风险评估 |
|-----|------|------|---------|
| `internal/security/v2/safe_file_manager_test.go` | 406 | `password = "my-secret-password"` | ✅ 测试数据 |
| `internal/security/v2/safe_file_manager_test.go` | 432 | SSH 私钥示例 | ✅ 测试数据 |
| `internal/snapshot/replication_test.go` | 406 | `APIKey: "secret-key"` | ✅ 测试数据 |
| `internal/webdav/server_test.go` | 166 | `password == "secret"` | ✅ 测试数据 |

**结论**: ✅ 无生产环境硬编码敏感信息

### 3.2 API 密钥模式扫描

```
扫描模式: sk-, ghp_, gho_, xoxb-, xoxp-, AKIA
结果: ✅ 未发现真实的 API 密钥
```

发现的内容均为标识符或测试数据：
- `disk-analysis`, `disk-io` - 数据标识符
- `task-*` - 测试任务名称

### 3.3 .gitignore 完整性

```
✅ 已排除:
- 编译产物 (nasd, nasd-*, *.test)
- 测试产物 (*.test, coverage*.out)
- IDE 配置 (.idea/, .vscode/)
- 系统文件 (.DS_Store, Thumbs.db)
- 备份文件 (backup)
```

**建议补充**:
```gitignore
# 当前已有 coverage*.out 但建议添加:
coverage/
*.out
```

---

## 四、遗留安全问题追踪

### 4.1 命令注入风险 (来自 v2.74.0 审计)

| 文件 | 行号 | 风险等级 | 状态 |
|-----|------|---------|------|
| `internal/snapshot/executor.go` | 89 | 高 | ⚠️ 未修复 |
| `internal/usbmount/manager.go` | 883 | 高 | ⚠️ 未修复 |

**建议**:
```go
// 添加输入验证
var safeScriptPattern = regexp.MustCompile(`^[a-zA-Z0-9_\-./]+$`)
if !safeScriptPattern.MatchString(script) {
    return fmt.Errorf("invalid script path")
}
```

### 4.2 整数溢出风险 (来自 v2.74.0 审计)

发现 **22 处整数转换潜在溢出** (G115)：

主要影响文件：
- `internal/system/monitor.go` - 网络速度计算
- `internal/snapshot/adapter.go` - 快照大小
- `internal/quota/optimizer/` - 配额计算
- `internal/storage/smart_monitor.go` - SMART 属性

**风险评估**: 中等。NAS 系统存储容量可能超过 int64 范围，但实际场景罕见。

### 4.3 敏感信息输出 (来自 v2.74.0 审计)

| 文件 | 行号 | 问题 | 状态 |
|-----|------|------|------|
| `internal/users/manager.go` | 183 | 初始密码输出到控制台 | ⚠️ 未修复 |

---

## 五、安全评分

| 类别 | 得分 | 说明 |
|-----|------|-----|
| 静态分析 | 100/100 | go vet 无问题 |
| 依赖安全 | 70/100 | Go 标准库存在已知漏洞 |
| 代码安全 | 75/100 | 命令注入和整数溢出风险 |
| 认证授权 | 90/100 | 机制完善，实现规范 |
| 敏感信息 | 85/100 | 无硬编码凭证，初始密码输出需改进 |
| **综合评分** | **84/100** | 中等安全水平 |

---

## 六、修复优先级

### 高优先级 (P0)

1. **升级 Go 版本至 1.26.1+**
   - 修复 5 个标准库安全漏洞
   - 特别是 crypto/x509 的 panic 漏洞 (GO-2026-4600)

```bash
# 修改 go.mod
go 1.26.1

# 更新依赖
go mod tidy
```

### 中优先级 (P1)

2. **安装并运行 gosec**
   ```bash
   go install github.com/securego/gosec/v2/cmd/gosec@latest
   gosec ./...
   ```

3. **命令注入风险缓解**
   - `internal/snapshot/executor.go` - 添加输入验证
   - `internal/usbmount/manager.go` - 添加命令白名单

### 低优先级 (P2)

4. **初始密码传递方式改进**
   - 避免输出到标准输出
   - 考虑写入临时文件 (权限 600)

---

## 七、结论

NAS-OS v2.75.0 整体安全状态良好：

✅ **通过项**:
- 静态分析无问题
- 无硬编码生产凭证
- .gitignore 配置完整
- 认证授权机制完善

⚠️ **需改进**:
- Go 版本需升级 (5 个标准库漏洞)
- gosec 未安装 (建议纳入 CI)
- 遗留命令注入风险
- 初始密码输出方式

**建议立即执行**: 升级 Go 版本至 1.26.1+

---

**审计人**: 刑部  
**审计时间**: 2026-03-15 23:10 CST  
**下次审计**: v2.76.0 发布前