# NAS-OS 安全审计报告
**审计日期**: 2026-03-17
**审计工具**: gosec v2
**工作目录**: /home/mrafter/clawd/nas-os

---

## 📊 执行摘要

| 严重程度 | 数量 | 占比 |
|---------|------|------|
| **高危 (HIGH)** | 153 | 9.3% |
| **中危 (MEDIUM)** | 791 | 48.1% |
| **低危 (LOW)** | 701 | 42.6% |
| **总计** | **1645** | 100% |

### 整体评估
⚠️ **风险等级: 高** - 发现大量安全问题，需优先处理高危问题。

---

## 🔴 高危问题分类 (153个)

| 规则 | 数量 | 说明 |
|-----|------|-----|
| G115 整数溢出 | 74 | uint64/int64 转换可能溢出 |
| G703 路径遍历 | 48 | CWE-22 文件路径未充分验证 |
| G702 命令注入 | 10 | virsh/qemu-img 命令拼接风险 |
| G118 Context泄漏 | 10 | WithCancel 返回函数未调用 |
| G122 TOCTOU竞态 | 7 | filepath.Walk 回调中路径竞态 |
| G101 硬编码凭证 | 7 | OAuth2 URL配置（低风险） |
| G402 TLS不安全 | 3 | InsecureSkipVerify（已有环境限制） |
| G404 弱随机数 | 2 | math/rand 用于非关键场景 |
| G707 不安全模板 | 1 | 模板渲染安全风险 |

---

## 🟡 中危问题分类 (791个)

| 规则 | 数量 | 说明 |
|-----|------|-----|
| G304 文件路径注入 | 230 | 变量用于文件路径 |
| G204 子进程启动 | 192 | exec.Command 变量参数 |
| G301 目录权限 | 181 | 权限超过 0750 |
| G306 文件权限 | 154 | 权限超过 0600 |
| G302 文件权限 | 10 | 文件写入权限问题 |
| G118 XSS | 2 | 污点分析发现 XSS 风险 |
| G107 URL注入 | 6 | HTTP 请求使用变量 URL |
| G110 解压炸弹 | 3 | 解压缩未限制大小 |
| G505 弱加密 | 2 | 使用 SHA1 |
| G117 其他 | 2 | 其他问题 |

---

## 🚨 需立即处理的高危问题

### 1. G702 命令注入 (internal/vm/snapshot.go)
```go
// 风险位置: Line 280, 294, 325
// virsh/qemu-img 命令拼接用户输入
cmd := exec.Command("virsh", "snapshot-create-as", vmName, snapshotName)
```
**建议**: 严格验证 vmName 和 snapshotName，禁止特殊字符。

### 2. G703 路径遍历 (internal/webdav/server.go)
```go
// 风险位置: Line 863, 864
// 文件路径来自用户请求，未充分验证
```
**建议**: 使用 `filepath.Clean()` + `filepath.Rel()` 验证路径不超出根目录。

### 3. G118 Context 泄漏 (internal/scheduler/executor.go)
```go
// 风险位置: Line 108, 115
ctx, cancel := context.WithTimeout(parent, timeout)
// cancel() 未被调用
```
**建议**: 使用 `defer cancel()` 确保资源释放。

---

## 🔒 敏感信息检查

### 已发现
1. **docker-compose.onlyoffice.yml:26** - JWT_SECRET 示例值 `your-jwt-secret-key-change-me`
   - 风险: 低（仅示例配置）
   - 建议: 部署时替换为真实密钥

2. **测试文件中的 SSH 私钥** - 仅用于测试，无实际风险

### 未发现
- 无真实密码硬编码
- 无 API Key 泄露
- 无生产密钥暴露

---

## ✅ 已有的安全措施

1. **LDAP TLS 验证** - `InsecureSkipVerify` 仅在测试环境生效
   ```go
   skipVerify := c.config.SkipTLSVerify && os.Getenv("ENV") == "test"
   ```

2. **敏感配置通过环境变量** - GITHUB_TOKEN, NAS_CSRF_KEY 等

---

## 📋 修复建议优先级

### P0 - 立即修复 (本周)
- [ ] G702: 命令注入 - 添加输入验证和参数化
- [ ] G703: 路径遍历 - 实现路径白名单验证
- [ ] G122: TOCTOU - 使用原子文件操作

### P1 - 高优先级 (本月)
- [ ] G118: Context 泄漏 - 添加 defer cancel()
- [ ] G110: 解压炸弹 - 添加解压大小限制
- [ ] G705: XSS - 输出转义处理

### P2 - 中优先级 (季度)
- [ ] G115: 整数溢出 - 添加边界检查
- [ ] G301/G306: 权限收紧至 0750/0640
- [ ] G505: 替换 SHA1 为 SHA256

---

## 📈 趋势对比

| 版本 | 高危 | 中危 | 低危 |
|-----|-----|-----|-----|
| v2.148.0 | 153 | 791 | 701 |
| v2.149.0 | 153 | 791 | 701 |

**结论**: 问题数量稳定，建议持续改进。

---

*报告生成时间: 2026-03-17 06:45 CST*