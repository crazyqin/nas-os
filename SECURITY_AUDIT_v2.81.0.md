# 安全审计报告 v2.81.0

**审计日期**: 2026-03-16  
**项目**: nas-os  
**版本**: v2.81.0  
**审计工具**: gosec v2, govulncheck  

---

## 执行摘要

本次安全审计发现项目存在 **1675 个代码安全问题** 和 **5 个依赖漏洞**。

### 风险等级分布

| 严重程度 | 数量 | 占比 |
|---------|------|------|
| 🔴 HIGH | 167 | 10.0% |
| 🟠 MEDIUM | 805 | 48.1% |
| 🟡 LOW | 703 | 42.0% |

### 依赖漏洞 (govulncheck)

发现 **5 个标准库漏洞**，均为 Go 1.26 版本的已知问题，**需升级至 Go 1.26.1** 修复：

| 漏洞ID | 组件 | 描述 | 修复版本 |
|--------|------|------|----------|
| GO-2026-4603 | html/template | meta content URL 未转义 | go1.26.1 |
| GO-2026-4602 | os | FileInfo 可从 Root 逃逸 | go1.26.1 |
| GO-2026-4601 | net/url | IPv6 主机解析错误 | go1.26.1 |
| GO-2026-4600 | crypto/x509 | 格式错误证书导致 panic | go1.26.1 |
| GO-2026-4599 | crypto/x509 | 邮件约束执行错误 | go1.26.1 |

---

## 详细发现

### 1. 代码安全问题 (gosec)

#### 1.1 高危问题 (167 个)

| 规则ID | 数量 | 描述 | 风险 |
|--------|------|------|------|
| G115 | 89 | 整数溢出转换 (uint64 → int64/int) | 可能导致数据截断或溢出 |
| G703 | 48 | os.Stderr/os.Stdout 未检查错误 | 输出错误可能被忽略 |
| G702 | 10 | log.Fatal 调用 | 可能导致程序异常退出 |
| G118 | 8 | 未检查 MarshalBinary 返回值 | 序列化失败可能被忽略 |
| G101 | 7 | 硬编码凭证/密钥 | 可能泄露敏感信息 |
| G122 | 7 | 未检查 crypto/rand.Read 返回值 | 随机数生成可能失败 |
| G402 | 4 | TLS 配置问题 | InsecureSkipVerify 或弱加密 |
| G707 | 1 | os.Exit 调用 | 可能跳过清理逻辑 |

##### G115 整数溢出问题示例

```go
// internal/system/monitor.go:760-761
// uint64 到 int64 的转换可能导致溢出
value := int64(someUint64)  // 危险！大值会溢出

// 建议：添加边界检查
if someUint64 > math.MaxInt64 {
    return errors.New("value exceeds int64 range")
}
value := int64(someUint64)
```

##### G101 硬编码凭证示例

```go
// 检查发现的文件中是否存在硬编码的密钥、密码等
// 建议使用环境变量或配置文件管理敏感信息
```

#### 1.2 中危问题 (805 个)

| 规则ID | 数量 | 描述 |
|--------|------|------|
| G304 | 228 | 文件路径可通过变量控制 (路径遍历风险) |
| G204 | 206 | exec.Command 参数可通过变量控制 (命令注入风险) |
| G301 | 180 | 目录权限过于宽松 (>0750) |
| G306 | 154 | 文件权限过于宽松 (>0600) |
| G302 | 10 | 文件权限设置问题 |
| G107 | 7 | HTTP 请求 URL 可通过变量控制 (SSRF 风险) |
| G110 | 4 | 潜在的解压炸弹 (Zip Slip) |
| G117 | 2 | 死循环风险 |
| G705 | 2 | goroutine 泄漏风险 |
| G120 | 1 | 未检查返回值 |
| G305 | 1 | 文件遍历风险 |

##### G304 路径遍历风险示例

```go
// 危险：用户输入直接用于文件路径
filepath.Join(baseDir, userInput)

// 建议：验证和清理路径
cleanPath := filepath.Clean(userInput)
if strings.Contains(cleanPath, "..") {
    return errors.New("invalid path")
}
fullPath := filepath.Join(baseDir, cleanPath)
```

##### G204 命令注入风险示例

```go
// 危险：用户输入直接拼接到命令
exec.Command("ls", userInput)

// 建议：使用参数化方式，避免 shell 解析
cmd := exec.Command("ls", "--", sanitizedInput)
```

#### 1.3 低危问题 (703 个)

| 规则ID | 数量 | 描述 |
|--------|------|------|
| G104 | 703 | 未检查错误返回值 |

##### G104 未检查错误示例

```go
// 危险：忽略错误
someFunction()

// 建议：始终检查错误
if err := someFunction(); err != nil {
    log.Printf("error: %v", err)
    return err
}
```

---

## 修复优先级建议

### P0 - 立即修复 (24小时内)

1. **升级 Go 版本至 1.26.1**
   ```bash
   # 更新 go.mod
   go mod edit -go=1.26.1
   go mod tidy
   ```

2. **审查 G101 硬编码凭证** (7处)
   - 移除所有硬编码密钥/密码
   - 使用环境变量或密钥管理服务

3. **修复 G402 TLS 配置问题** (4处)
   - 禁用 InsecureSkipVerify
   - 使用安全的 TLS 配置

### P1 - 高优先级 (1周内)

1. **G115 整数溢出** (89处)
   - 添加边界检查
   - 使用安全转换函数

2. **G122 crypto/rand 返回值** (7处)
   - 确保随机数生成错误被处理

3. **G703/G702/G707 程序控制流** (59处)
   - 审查 log.Fatal/os.Exit 调用
   - 确保错误处理不会导致程序意外终止

### P2 - 中优先级 (2周内)

1. **G304 文件路径遍历** (228处)
   - 验证和清理所有文件路径输入
   - 使用 filepath.Clean 和白名单

2. **G204 命令注入** (206处)
   - 审查所有 exec.Command 调用
   - 使用参数化方式避免 shell 注入

3. **G301/G306 文件权限** (334处)
   - 设置合理的文件/目录权限
   - 避免全局可写/可读

### P3 - 低优先级 (1个月内)

1. **G104 未检查错误** (703处)
   - 逐步修复，确保所有错误都被处理
   - 使用 linter 强制检查

---

## 受影响模块

主要问题集中在以下模块：

| 模块 | 问题数量 | 主要风险 |
|------|----------|----------|
| internal/reports | ~180 | 路径遍历、权限问题 |
| internal/storage | ~120 | 整数溢出、文件操作 |
| internal/disk | ~100 | 文件权限、路径处理 |
| internal/web | ~80 | SSRF、TLS配置 |
| internal/quota | ~70 | 整数溢出 |

---

## 修复验证步骤

修复后运行以下命令验证：

```bash
# 1. 更新依赖
go mod tidy

# 2. 运行安全扫描
gosec -fmt=json -out=gosec-report-fixed.json ./...

# 3. 运行依赖漏洞检查
govulncheck ./...

# 4. 运行测试
go test ./...
```

---

## 附录

### gosec 规则说明

| 规则 | 描述 |
|------|------|
| G101 | 硬编码凭证检测 |
| G104 | 未检查错误返回值 |
| G107 | SSRF 风险 |
| G110 | 解压炸弹风险 |
| G115 | 整数溢出风险 |
| G117 | 死循环风险 |
| G118 | 未检查返回值 |
| G120 | 未检查返回值 |
| G122 | 未检查返回值 |
| G204 | 命令注入风险 |
| G301 | 目录权限过宽 |
| G302 | 文件权限设置问题 |
| G304 | 路径遍历风险 |
| G305 | 文件遍历风险 |
| G306 | 文件权限过宽 |
| G402 | TLS 配置问题 |
| G505 | 弱加密算法 |
| G702 | log.Fatal 调用 |
| G703 | 未检查输出错误 |
| G705 | goroutine 泄漏 |
| G707 | os.Exit 调用 |

### 扫描命令

```bash
# 生成 JSON 报告
gosec -fmt=json -out=gosec-report-v2.81.0.json ./...

# 生成 SARIF 报告 (用于 GitHub Advanced Security)
gosec -fmt=sarif -out=gosec-report.sarif ./...

# 依赖漏洞检查
govulncheck ./...
```

---

**审计完成时间**: 2026-03-16 01:46  
**审计人员**: 刑部自动化安全审计系统