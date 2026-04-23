# Spotlight 安全审计报告

**审计编号**: Round231  
**审计时间**: 2026-04-24 02:35 GMT+8  
**审计范围**: Spotlight 相关代码 (internal/smb/spotlight*.go, internal/search/spotlight.go)  
**扫描工具**: gosec v2  

---

## 扫描概览

| 指标 | 数值 |
|------|------|
| 扫描文件数 | 26 |
| 扫描代码行数 | 17,091 |
| 发现问题总数 | 26 |
| 高危问题 | 1 |
| 中危问题 | 19 |
| 低危问题 | 6 |

---

## Spotlight 专属问题

### 🟡 中危 - spotlight_integration.go:547

**规则**: G304 - 潜在路径遍历  
**代码**:
```go
content, err := os.ReadFile(path)
```
**风险**: 变量 `path` 来自外部输入，可能被利用进行路径遍历攻击  
**建议**: 确保调用方已对 `path` 进行规范化验证，限制在允许的共享目录范围内

### 🔵 低危 - spotlight_integration.go:500

**规则**: G104 - 错误未处理  
**代码**:
```go
idx.walkAndIndex(fullPath, filesIndexed, sizeIndexed)
```
**风险**: 递归调用返回的错误被忽略，可能导致索引不完整  
**建议**: 检查并记录错误日志

### 🔵 低危 - spotlight.go:432, 434

**规则**: G104 - 错误未处理  
**代码**:
```go
s.indexer.IndexFile(ctx, change.Path)
s.indexer.RemoveFromIndex(ctx, change.Path)
```
**风险**: 文件变更处理时的错误被忽略  
**建议**: 添加错误日志记录

---

## SMB 模块关联问题

以下问题虽不在 Spotlight 核心文件中，但与 Spotlight 使用的 SMB 基础设施相关：

### 🔴 高危 - config.go:519

**规则**: G703 - 污点分析检测到路径遍历  
**代码**:
```go
if err := os.WriteFile(backupPath, data, 0640); err != nil {
```
**风险**: `backupPath` 拼接自 `configPath + ".bak"`，若 configPath 受控可造成任意文件写入  
**建议**: 使用 `filepath.Base()` 提取文件名，验证路径不包含 `..` 等危险序列

### 🟡 中危 - 多处文件权限 0640

**规则**: G302/G306 - 文件权限过高  
**位置**:
- security.go:670, 718, 724
- manager.go:300, 323
- config.go:491, 519

**风险**: 配置文件权限 0640 允许组用户读取敏感信息  
**建议**: 配置文件应使用 0600 权限，仅限所有者访问

---

## 风险评估

### 高风险项
1. **config.go 路径遍历** - 需要立即审查 `configPath` 来源，确认是否受用户输入控制

### 中风险项
1. **文件路径验证不足** - 多处使用变量路径读取文件，需确保调用链已做安全验证
2. **文件权限过宽** - 建议收紧至 0600

### 低风险项
1. **错误处理缺失** - 不影响安全，但影响可靠性和可调试性

---

## 修复优先级

| 优先级 | 问题 | 文件 | 行号 |
|--------|------|------|------|
| P0 | 路径遍历 (G703) | config.go | 519 |
| P1 | 路径遍历 (G304) | spotlight_integration.go | 547 |
| P2 | 文件权限 (G302/G306) | 多处 | - |
| P3 | 错误处理 (G104) | spotlight.go, spotlight_integration.go | 432, 434, 500 |

---

## 结论

Spotlight 功能本身设计合理，核心搜索功能未见严重安全漏洞。主要风险集中在：

1. **SMB 配置管理模块** - 存在高危路径遍历风险，需优先修复
2. **文件系统操作** - 多处路径变量需确保已做安全验证
3. **权限配置** - 建议收紧配置文件权限

**建议**: 在修复 P0/P1 问题前，暂不建议在生产环境暴露 Spotlight 相关 API。

---

*审计完成*