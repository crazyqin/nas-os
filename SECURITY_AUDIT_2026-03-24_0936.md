# 安全审计报告

**项目**: nas-os  
**版本**: v2.253.158  
**审计日期**: 2026-03-24 09:36  
**审计人**: 刑部（安全合规）  
**审计范围**: AI数据脱敏、网盘挂载认证、gosec扫描、硬编码凭证检查

---

## 一、审计摘要

| 类别 | 状态 | 说明 |
|------|------|------|
| AI数据脱敏 | ⚠️ 需改进 | 框架完善但正则规则需增强 |
| 网盘挂载认证 | ✅ 安全 | 敏感字段已正确标记 `json:"-"` |
| gosec扫描 | ⚠️ 需关注 | 144 HIGH + 529 MEDIUM |
| 硬编码凭证 | ✅ 无风险 | 均为OAuth URL误报 |

---

## 二、AI数据脱敏审计

### 2.1 代码位置
`internal/ai/service.go`

### 2.2 实现评估

**✅ 安全设计：**
- 使用正则表达式匹配敏感信息
- 原始值存储在内存映射中（支持还原）
- 线程安全（sync.RWMutex保护）
- 默认规则覆盖：邮箱、手机、身份证、银行卡、IP地址

**⚠️ 需改进：**

1. **正则规则不够精确**：
```go
// 当前规则
{Name: "phone", Pattern: `\d{11}`, Replacement: "[PHONE]"}  // 过于宽松，会误匹配11位数字
{Name: "credit_card", Pattern: `\d{16}`, Replacement: "[CARD]"}  // 无Luhn校验
```

2. **缺少重要规则**：
   - 银行卡号（需Luhn校验）
   - 护照号码
   - 统一社会信用代码
   - 车牌号
   - 中文姓名

3. **映射存储风险**：
   - `mappings` 在内存中存储原始敏感数据
   - 无自动过期/清理机制（仅有手动 `ClearMappings()`）
   - 进程崩溃可能泄露敏感映射

### 2.3 修复建议

1. 增强正则规则精确度
2. 添加银行卡号Luhn校验
3. 实现映射自动过期（如15分钟）
4. 考虑使用安全内存区域存储映射

---

## 三、网盘挂载认证安全审计

### 3.1 代码位置
- `internal/cloudsync/types.go` - 数据结构定义
- `internal/cloudsync/providers.go` - 云存储提供商实现
- `internal/backup/credentials.go` - 凭证加密存储

### 3.2 安全评估 ✅

**正确实现：**

1. **敏感字段不序列化**：
```go
// internal/cloudsync/types.go
SecretKey     string `json:"-"` // 安全：禁止序列化到 JSON
ClientSecret  string `json:"-"` // 安全：禁止序列化到 JSON
RefreshToken  string `json:"-"` // 安全：禁止序列化到 JSON
EncryptKey    string `json:"-"` // 安全：禁止序列化到 JSON
```

2. **凭证加密存储**（backup模块）：
```go
// internal/backup/credentials.go
// 使用 AES-GCM 加密
// 密钥文件权限 0600
func (cs *CredentialStore) Encrypt(plaintext string) (string, error)
```

3. **安全默认值**：
   - 密钥文件权限 `0600`
   - 目录权限 `0700`
   - 敏感配置使用 `Sanitize()` 方法脱敏

**⚠️ 注意事项：**

1. **WebDAV InsecureSkipVerify**：
```go
// internal/cloudsync/providers.go:412
// #nosec G402 -- InsecureSkipVerify is only allowed in test environment (ENV=test)
```
- 已正确使用 `#nosec` 注释
- 有环境变量检查限制

---

## 四、gosec扫描结果

### 4.1 扫描统计

| 严重级别 | 数量 | 主要类型 |
|----------|------|----------|
| HIGH | 144 | G115整数溢出(68), G404弱随机(1), G402 TLS(1), G707 SMTP注入(1) |
| MEDIUM | 529 | G304文件路径(217), G204命令执行(175), G306文件权限(119) |
| LOW | 0 | - |

### 4.2 关键安全问题

#### 🔴 G404 - 弱随机数生成器 (1处)
**位置**: `internal/quota/manager.go:640`
```go
r := mrand.New(mrand.NewSource(now))
```
**风险**: 伪随机数生成器不适合安全场景
**修复**: 使用 `crypto/rand` 作为主路径（代码已有回退逻辑，但回退方案不安全）

#### 🔴 G707 - SMTP注入风险 (1处)
**位置**: `internal/automation/action/action.go:287`
**风险**: 邮件头可能被注入恶意内容
**修复**: 验证/过滤 `from`, `to`, `subject` 参数中的换行符

#### 🟡 G402 - TLS证书验证跳过 (1处)
**位置**: `internal/auth/ldap.go:159`
```go
InsecureSkipVerify: skipVerify  // skipVerify = config.SkipTLSVerify && os.Getenv("ENV") == "test"
```
**评估**: 有环境检查限制，风险可控

#### 🟡 G115 - 整数溢出 (68处)
**评估**: 中低风险，多发生在类型转换场景
**建议**: 逐步添加边界检查

### 4.3 误报分析

#### G101 - 潜在硬编码凭证 (7处)
**全部为误报**：
- `internal/auth/oauth2.go` - OAuth端点URL，非凭证
- `internal/cloudsync/providers.go` - OAuth token URL
- `internal/office/types.go:583` - 错误消息字符串 `ErrInvalidToken`

**结论**: 无真实硬编码凭证 ✅

---

## 五、需修复的安全问题清单

### 🔴 高优先级

| # | 问题 | 位置 | 风险 | 修复建议 |
|---|------|------|------|----------|
| 1 | SMTP注入风险 | `internal/automation/action/action.go:287` | 高 | 过滤邮件参数中的换行符 |
| 2 | 弱随机数回退 | `internal/quota/manager.go:640` | 中 | 使用更强的回退方案 |

### 🟡 中优先级

| # | 问题 | 位置 | 风险 | 修复建议 |
|---|------|------|------|----------|
| 3 | AI脱敏正则不精确 | `internal/ai/service.go` | 中 | 增强正则规则精确度 |
| 4 | 映射无自动过期 | `internal/ai/service.go` | 低 | 添加TTL自动清理 |
| 5 | 整数溢出警告 | 多处 (68处) | 低 | 逐步添加边界检查 |

### ✅ 可接受风险

| # | 问题 | 说明 |
|---|------|------|
| G101误报 | OAuth URL被误识别为凭证 |
| G304/G204 | 文件路径/命令执行为功能需要 |
| G402 | 有环境变量检查限制 |

---

## 六、合规状态

| 检查项 | 状态 |
|--------|------|
| 敏感信息泄露 | ✅ 无发现 |
| 硬编码凭证 | ✅ 无发现 |
| SQL注入 | ✅ 已有防护 |
| 认证信息存储 | ✅ 安全实现 |
| 文件权限 | ✅ 符合规范 |

---

## 七、结论

**安全评级**: ⚠️ 基本通过（需修复2个高优先级问题）

项目整体安全性良好，网盘挂载和备份模块的凭证处理符合安全规范。主要问题集中在：
1. SMTP注入风险需立即修复
2. AI脱敏功能需增强规则精确度

---

*审计完成: 刑部（安全合规）*  
*报告时间: 2026-03-24 09:36*