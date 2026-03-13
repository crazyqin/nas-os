# NAS-OS 项目安全审计报告

**审计日期**: 2026-03-14
**审计部门**: 刑部
**项目位置**: ~/clawd/nas-os

---

## 📋 审计摘要

| 项目 | 状态 | 风险等级 |
|------|------|----------|
| 依赖安全 | ✅ 通过 | 低风险 |
| 代码安全 | ⚠️ 需关注 | 中风险 |
| API 安全 | ✅ 良好 | 低风险 |
| 合规性 | ✅ 通过 | 无风险 |

---

## 🔴 高风险问题

### 1. 路径遍历漏洞 (G703)

**位置**: `internal/backup/manager.go:644-648`

```go
data, err := os.ReadFile(path)
return os.WriteFile(dstPath, data, info.Mode())
```

**风险**: 攻击者可通过构造恶意路径访问/修改任意文件。

**建议**:
- 对所有文件路径输入进行规范化验证
- 使用 `filepath.Clean()` 和 `filepath.Rel()` 确保路径在预期目录内
- 添加路径白名单校验

---

### 2. 文件系统竞态条件 (G122)

**位置**: `internal/backup/manager.go:644`

**风险**: filepath.Walk 回调中的路径操作存在 TOCTOU (Time-of-Check to Time-of-Use) 竞态条件，符号链接可被利用绕过安全检查。

**建议**:
- 使用 `os.Root` API (Go 1.24+) 进行根目录限制的文件操作
- 在操作前验证符号链接目标
- 使用 `O_NOFOLLOW` 标志防止符号链接跟随

---

### 3. 弱加密原语 (G401)

**位置**: `internal/network/ddns_providers.go:559`

```go
h := sha1.New()
```

**风险**: SHA1 已被证明存在碰撞漏洞，不应用于安全敏感场景。

**说明**: 代码注释表明这是阿里云 DNS API 规范要求，用于 HMAC-SHA1 签名。HMAC-SHA1 目前仍被认为是安全的，但建议关注 API 升级。

**建议**:
- 添加明确的安全注释说明使用原因
- 监控阿里云 API 是否支持更强的算法
- 非签名场景禁止使用 SHA1

---

## 🟡 中风险问题

### 4. 子进程命令注入风险 (G204)

**位置**: 多处，共 225 处 `exec.Command` 调用

**高风险位置**:
- `pkg/btrfs/btrfs.go` - mount/umount 命令
- `internal/backup/encrypt.go` - openssl/gpg 命令
- `internal/docker/manager.go` - docker 命令

**风险**: 变量直接传递给 shell 命令可能导致命令注入。

**建议**:
- 所有用户输入在传递给命令前进行严格验证
- 使用参数化命令而非字符串拼接
- 对 mount 点、设备名等使用白名单字符集验证
- 示例验证函数:

```go
func sanitizeMountPoint(path string) error {
    // 只允许字母、数字、斜杠、下划线、连字符
    matched, _ := regexp.MatchString(`^[\w/\-.]+$`, path)
    if !matched {
        return errors.New("invalid mount point")
    }
    return nil
}
```

---

### 5. 敏感数据处理

**位置**: 多处配置结构体

```go
// internal/backup/config.go
SecretKey     string `json:"secretKey"`
RemotePassword string `json:"remotePassword,omitempty"`

// internal/cloudsync/types.go
SecretKey string `json:"secretKey,omitempty"`
```

**风险**: 敏感信息可能在日志、API 响应中泄露。

**建议**:
- 使用自定义类型隐藏敏感字段 (不实现 json.Marshaler 或返回 `***`)
- 配置存储时加密敏感字段
- 日志输出时过滤敏感字段
- 示例:

```go
type SecretString string

func (s SecretString) MarshalJSON() ([]byte, error) {
    return json.Marshal("***")
}

func (s SecretString) String() string {
    return "***"
}
```

---

### 6. 备份加密密码传递方式

**位置**: `internal/backup/encrypt.go:37-46`

```go
cmd := exec.Command(
    "openssl",
    "enc",
    "-aes-256-cbc",
    // ...
    "-pass", "pass:"+e.password,  // 密码通过命令行传递
)
```

**风险**: 密码通过命令行参数传递，可在进程列表中暴露。

**建议**:
- 使用环境变量传递: `-pass env:PASSWORD`
- 或使用文件: `-pass file:/path/to/keyfile`
- 或使用 OpenSSL 的 `-kfile` 选项从文件读取

---

## 🟢 低风险/信息性问题

### 7. API 认证与授权

**现状**: 
- ✅ 实现了完整的 JWT 认证中间件
- ✅ 实现了 RBAC 权限管理
- ✅ 支持多种 MFA 方式 (TOTP, SMS, WebAuthn, 备份码)
- ✅ 会话缓存与过期机制

**改进建议**:
- 添加登录失败次数限制 (防暴力破解)
- 实现敏感操作的二次验证
- 添加审计日志持久化

---

### 8. 文件权限

**位置**: 多处文件写入使用 `0644` 权限

```go
os.WriteFile(path, data, 0644)
```

**建议**: 
- 敏感配置文件使用 `0600` 限制为所有者可读写
- 密钥文件使用 `0400` 只读

---

## ✅ 通过的安全检查

### 依赖安全扫描

```
govulncheck ./...
No vulnerabilities found.
```

所有依赖无已知安全漏洞。

### 许可证合规

- 项目使用 MIT 许可证，开源合规
- 主要依赖许可证均为宽松许可证 (MIT, Apache-2.0, BSD)

### SQL 注入防护

- 使用参数化查询，无直接字符串拼接 SQL

---

## 📊 风险评估矩阵

| 问题类型 | 影响范围 | 利用难度 | 风险等级 |
|---------|---------|---------|---------|
| 路径遍历 | 高 | 低 | 🔴 高 |
| TOCTOU 竞态 | 高 | 中 | 🔴 高 |
| 命令注入 | 高 | 中 | 🟡 中 |
| 敏感数据泄露 | 中 | 低 | 🟡 中 |
| 密码命令行传递 | 中 | 中 | 🟡 中 |
| 弱加密原语 | 低 | 高 | 🟢 低 |

---

## 🛠️ 优先修复建议

### P0 - 立即修复
1. 路径遍历漏洞 - 添加路径验证函数
2. TOCTOU 竞态 - 使用 os.Root API

### P1 - 短期修复
3. 命令注入风险 - 实现输入白名单验证
4. 敏感数据处理 - 实现敏感字段掩码
5. 密码传递方式 - 改用环境变量

### P2 - 中期改进
6. 文件权限收紧
7. 登录失败限制
8. 审计日志持久化

---

## 📝 代码修复示例

### 路径安全验证

```go
package security

import (
    "errors"
    "path/filepath"
    "strings"
)

// SafeJoin 安全地连接基础路径和用户提供的路径
func SafeJoin(baseDir, userPath string) (string, error) {
    // 清理路径
    cleaned := filepath.Clean(userPath)
    
    // 移除开头的斜杠，防止绝对路径
    cleaned = strings.TrimPrefix(cleaned, "/")
    
    // 连接路径
    fullPath := filepath.Join(baseDir, cleaned)
    
    // 确保结果路径在 baseDir 内
    rel, err := filepath.Rel(baseDir, fullPath)
    if err != nil {
        return "", err
    }
    
    if strings.HasPrefix(rel, "..") {
        return "", errors.New("path traversal detected")
    }
    
    return fullPath, nil
}
```

### 敏感字段掩码

```go
package types

import "encoding/json"

type SecretField struct {
    value string
}

func NewSecretField(value string) SecretField {
    return SecretField{value: value}
}

func (s SecretField) Value() string {
    return s.value
}

func (s SecretField) MarshalJSON() ([]byte, error) {
    return json.Marshal("***REDACTED***")
}

func (s *SecretField) UnmarshalJSON(data []byte) error {
    return json.Unmarshal(data, &s.value)
}

func (s SecretField) String() string {
    return "***REDACTED***"
}
```

---

## 📌 结论

NAS-OS 项目整体安全架构设计合理，已实现认证、授权、加密等核心安全功能。主要风险集中在文件操作和外部命令执行部分，建议按优先级逐步修复。

**下一步行动**:
1. 创建安全修复分支
2. 按优先级逐项修复
3. 添加安全测试用例
4. 建立代码安全审查流程

---

*刑部 安全审计组*