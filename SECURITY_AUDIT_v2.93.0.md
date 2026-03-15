# NAS-OS v2.93.0 安全审计报告

**审计日期**: 2026-03-16  
**审计部门**: 刑部（安全合规）  
**项目版本**: v2.93.0  
**审计范围**: 代码安全扫描、依赖安全检查、敏感信息泄露检查

---

## 执行摘要

| 指标 | 数值 |
|------|------|
| 扫描文件数 | 446 |
| 代码行数 | 274,533 |
| 发现问题总数 | 1,681 |
| 高危问题 | 171 |
| 中危问题 | 806 |
| 低危问题 | 704 |
| Go 标准库漏洞 | 5 |

**风险等级**: ⚠️ **中等风险** - 需要关注高危漏洞

---

## 一、Go 标准库漏洞 (govulncheck)

发现 **5 个标准库安全漏洞**，需升级 Go 版本至 1.26.1：

| 漏洞ID | 描述 | 影响 | 修复版本 |
|--------|------|------|----------|
| GO-2026-4603 | html/template URL 未转义 | XSS 风险 | Go 1.26.1 |
| GO-2026-4602 | os.FileInfo 可从 Root 逃逸 | 路径遍历 | Go 1.26.1 |
| GO-2026-4601 | net/url IPv6 解析错误 | URL 注入 | Go 1.26.1 |
| GO-2026-4600 | x509 证书名称约束 Panic | 拒绝服务 | Go 1.26.1 |
| GO-2026-4599 | x509 邮件约束执行错误 | 证书验证绕过 | Go 1.26.1 |

### 修复建议
```bash
# 升级 Go 版本
go upgrade to 1.26.1
```

---

## 二、高危漏洞详情

### 2.1 命令注入 (G702) - 10 处

**风险**: 攻击者可执行任意命令

**受影响文件**:
- `internal/vm/snapshot.go:319,289,276`
- `internal/vm/manager.go:716,620`

**示例代码**:
```go
// internal/vm/snapshot.go:319
cmd := exec.Command("qemu-img", args...)
// vm.DiskPath 可能被污染
```

**修复建议**:
- 使用参数化命令执行
- 严格验证所有输入
- 使用 `exec.CommandContext` 并限制超时

### 2.2 路径遍历 (G703) - 48 处

**风险**: 攻击者可访问任意文件

**受影响文件**:
- `plugins/filemanager-enhance/main.go:452`
- `internal/webdav/server.go:852,851,849,789`

**修复建议**:
- 使用 `filepath.Clean()` 清理路径
- 验证路径是否在允许的根目录内
- 使用 `fs.FS` 限制文件系统访问

### 2.3 TLS 不安全 (G402) - 7 处

**风险**: 中间人攻击

**受影响文件**:
- `internal/cloudsync/providers.go:399`
- `internal/backup/sync.go:659`
- `internal/backup/cloud.go:121`
- `internal/auth/ldap.go:184`
- `internal/ldap/client.go:104`

**示例代码**:
```go
// internal/backup/cloud.go:121
tlsConfig := &tls.Config{
    InsecureSkipVerify: true, // 危险！
}
```

**修复建议**:
- 移除 `InsecureSkipVerify: true`
- 使用有效的 TLS 证书
- 如需自签名证书，显式添加 CA

### 2.4 硬编码凭证 (G101) - 7 处

**风险**: 凭证泄露

**受影响文件**:
- `internal/office/types.go:581`
- `internal/cloudsync/providers.go:1320,670`
- `internal/auth/oauth2.go:379-389,364-374`

**修复建议**:
- 移除硬编码凭证
- 使用环境变量或密钥管理服务
- 审计所有涉及敏感信息的代码

### 2.5 整数溢出 (G115) - 89 处

**风险**: 数据损坏、逻辑错误

**受影响模块**:
- `internal/system/monitor.go` - 网络统计
- `internal/storage/smart_monitor.go` - SMART 数据
- `internal/quota/optimizer/optimizer.go` - 配额计算
- `internal/photos/handlers.go` - 图片处理

**修复建议**:
- 使用 `math/bits` 包进行溢出检查
- 对大数值使用 `big.Int`
- 添加边界检查

---

## 三、中危漏洞详情

### 3.1 文件包含 (G304) - 228 处

**风险**: 任意文件读取/写入

**示例**:
```go
// tests/reports/generator.go:288
data, _ := os.ReadFile(userInput) // 用户输入未验证
```

### 3.2 子进程执行 (G204) - 206 处

**风险**: 命令注入

**主要影响**: `pkg/btrfs/btrfs.go` - BTRFS 操作

### 3.3 目录权限过高 (G301) - 180 处

**风险**: 信息泄露

**修复**: 目录权限应 ≤ 0750

### 3.4 文件权限过高 (G306) - 154 处

**风险**: 敏感文件泄露

**修复**: 文件权限应 ≤ 0600

### 3.5 解压炸弹 (G110) - 4 处

**风险**: 拒绝服务

**受影响文件**:
- `internal/transfer/chunked.go:254`
- `internal/files/manager.go:1011`
- `internal/compress/manager.go:391`

### 3.6 弱加密算法 (G505) - 3 处

**风险**: 哈希碰撞

**受影响**:
- `internal/security/v2/mfa.go` - SHA1
- `internal/network/ddns_providers.go` - SHA1
- `internal/auth/sms.go` - SHA1

---

## 四、依赖安全状态

### 4.1 模块验证
✅ `go mod verify` 通过 - 所有模块校验和匹配

### 4.2 可更新依赖 (部分)
以下依赖有可用更新，建议评估后升级：

| 依赖 | 当前版本 | 最新版本 |
|------|----------|----------|
| github.com/aws/aws-sdk-go-v2 | v1.41.3 | v1.41.4 |
| github.com/aws/aws-sdk-go-v2/service/s3 | v1.96.4 | v1.97.1 |
| github.com/blevesearch/bleve/v2 | v2.5.7 | (检查更新) |
| github.com/PuerkitoBio/purell | v1.1.1 | v1.2.1 |

---

## 五、敏感信息检查

### 5.1 AWS 密钥检查
✅ 未发现真实 AWS 密钥泄露
- `internal/backup/cloud_test.go` 包含示例密钥 `AKIAIOSFODNN7EXAMPLE` (AWS 官方示例)

### 5.2 敏感词检查
发现 30+ 文件包含敏感词 (password, secret, token 等)，需人工审查：
- `internal/backup/cloud.go`
- `internal/cloudsync/providers.go`
- `internal/ldap/client.go`
- `internal/auth/oauth2.go`

---

## 六、修复优先级

### 🔴 立即修复 (P0)
1. **升级 Go 至 1.26.1** - 修复 5 个标准库漏洞
2. **命令注入 (G702)** - 10 处，VM 模块关键
3. **路径遍历 (G703)** - 48 处，文件管理关键

### 🟠 尽快修复 (P1)
4. **TLS 不安全 (G402)** - 7 处
5. **硬编码凭证 (G101)** - 7 处
6. **解压炸弹 (G110)** - 4 处

### 🟡 计划修复 (P2)
7. **整数溢出 (G115)** - 89 处
8. **文件权限 (G306/G301)** - 334 处
9. **弱加密 (G505)** - 3 处

### 🟢 建议改进 (P3)
10. **文件包含 (G304)** - 228 处
11. **子进程执行 (G204)** - 206 处
12. **更新依赖** - 多个依赖有新版本

---

## 七、总结

### 安全评分
| 维度 | 评分 | 说明 |
|------|------|------|
| 代码安全 | 65/100 | 存在多处高危问题 |
| 依赖安全 | 80/100 | 标准库需升级 |
| 配置安全 | 70/100 | TLS/权限需加固 |
| **综合评分** | **72/100** | 中等风险 |

### 后续行动
1. **紧急**: 升级 Go 版本至 1.26.1
2. **重要**: 修复命令注入和路径遍历漏洞
3. **必要**: 加固 TLS 配置，移除硬编码凭证
4. **持续**: 定期执行安全扫描，监控依赖更新

---

**报告生成时间**: 2026-03-16 05:24:56 CST  
**审计人**: 刑部自动化安全审计系统