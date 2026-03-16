# 安全审计报告 v2.101.0

**审计日期**: 2026-03-16  
**审计工具**: gosec v2  
**项目**: nas-os  
**版本**: v2.101.0  
**审计人**: 刑部（安全合规）

---

## 执行摘要

本次安全审计共发现 **1678 个安全问题**，按严重程度分布如下：

| 严重级别 | 数量 | 占比 |
|---------|------|------|
| HIGH | 169 | 10.1% |
| MEDIUM | 808 | 48.2% |
| LOW | 701 | 41.8% |

### 核心风险领域

1. **命令注入风险** - 10处（G702）
2. **路径遍历漏洞** - 48处（G703）
3. **XSS漏洞** - 2处（G705）
4. **硬编码凭证风险** - 7处（G101）
5. **TLS不安全配置** - 4处（G402）
6. **解压缩炸弹风险** - 4处（G110）
7. **整数溢出** - 91处（G115）

---

## 高危漏洞详情

### 1. 命令注入漏洞 (G702) - 严重度: HIGH

**影响**: 攻击者可执行任意系统命令，完全控制服务器

**发现位置**:

| 文件 | 行号 | 风险描述 |
|-----|------|---------|
| `internal/vm/manager.go` | 309, 553, 585, 587, 615, 620, 716 | VM管理命令注入 |
| `internal/vm/snapshot.go` | 276, 289, 319 | 快照操作命令注入 |

**修复建议**:
```go
// 错误示例
cmd := exec.Command("qemu-img", "create", userPath, size)

// 正确示例 - 使用参数化
cmd := exec.Command("qemu-img", "create", filepath.Base(userPath), size)
// 或使用安全的白名单验证
if !isValidPath(userPath) {
    return errors.New("invalid path")
}
```

### 2. 路径遍历漏洞 (G703) - 严重度: HIGH

**影响**: 攻击者可读取/写入任意文件，导致数据泄露或系统被控制

**受影响模块**:

| 模块 | 问题数 | 文件 |
|-----|-------|------|
| WebDAV服务 | 31处 | `internal/webdav/server.go` |
| VM管理 | 4处 | `internal/vm/manager.go`, `snapshot.go` |
| 备份系统 | 6处 | `internal/backup/` |
| 文件管理插件 | 1处 | `plugins/filemanager-enhance/main.go` |
| 安全模块 | 2处 | `internal/security/v2/encryption.go` |

**修复建议**:
```go
// 安全路径处理函数
func safePath(baseDir, userPath string) (string, error) {
    fullPath := filepath.Join(baseDir, userPath)
    absPath, err := filepath.Abs(fullPath)
    if err != nil {
        return "", err
    }
    absBase, _ := filepath.Abs(baseDir)
    if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
        return "", errors.New("path traversal detected")
    }
    return absPath, nil
}
```

### 3. XSS跨站脚本攻击 (G705) - 严重度: HIGH

**影响**: 攻击者可注入恶意脚本，窃取用户会话或执行恶意操作

**发现位置**:
- `internal/automation/api/handlers.go`: 278, 409

**修复建议**:
```go
import "html"

// 对用户输入进行转义
safeOutput := html.EscapeString(userInput)
```

---

## 中危漏洞详情

### 4. 硬编码凭证风险 (G101) - 严重度: MEDIUM

**影响**: 敏感信息泄露风险

**发现位置**:

| 文件 | 行号 | 风险点 |
|-----|------|-------|
| `internal/auth/oauth2.go` | 334-389 | OAuth2配置（ClientSecret字段） |
| `internal/cloudsync/providers.go` | 670, 1320 | Token URL暴露 |

**修复建议**:
- 使用环境变量或安全配置存储敏感凭证
- 启用 secrets 管理服务（如 Vault、AWS Secrets Manager）
- 确保 `.gitignore` 包含敏感配置文件

### 5. TLS不安全配置 (G402) - 严重度: MEDIUM

**影响**: 中间人攻击风险

**发现位置**:
- `internal/auth/ldap.go`: 154, 184
- `internal/ldap/client.go`: 74, 104

**代码示例**:
```go
tlsConfig := &tls.Config{InsecureSkipVerify: true}
```

**修复建议**:
- 生产环境禁用 `InsecureSkipVerify`
- 使用有效证书或配置内部CA
- 如确需跳过验证，仅限测试环境

### 6. 解压缩炸弹风险 (G110) - 严重度: MEDIUM

**影响**: 资源耗尽攻击（DoS）

**发现位置**:
- `internal/transfer/chunked.go`: 254
- `internal/files/manager.go`: 1011
- `internal/compress/manager.go`: 391
- `internal/backup/config_backup.go`: 476

**修复建议**:
```go
import "io"

// 限制解压缩大小
type limitedReader struct {
    r io.Reader
    n int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
    if l.n <= 0 {
        return 0, errors.New("size limit exceeded")
    }
    n, err := l.r.Read(p)
    l.n -= int64(n)
    return n, err
}

// 使用限制读取器
limited := &limitedReader{r: gzipReader, n: maxFileSize}
io.Copy(dst, limited)
```

### 7. 整数溢出 (G115) - 严重度: HIGH (MEDIUM置信度)

**影响**: 数据损坏、逻辑错误

**共发现 91 处**，主要分布在：

| 文件 | 问题数 |
|-----|-------|
| `internal/storage/smart_monitor.go` | 4 |
| `internal/monitor/metrics_collector.go` | 4 |
| `internal/disk/smart_monitor.go` | 2 |
| `internal/reports/datasource.go` | 5 |
| `internal/quota/optimizer/optimizer.go` | 4 |

**修复建议**:
```go
// 添加边界检查
func safeUint64ToInt64(v uint64) (int64, error) {
    if v > math.MaxInt64 {
        return 0, errors.New("value too large")
    }
    return int64(v), nil
}
```

---

## 低危漏洞详情

### 8. 未处理错误 (G104) - 数量: 701

**影响**: 程序异常、静默失败

**修复建议**: 系统性审查所有忽略错误返回值的调用

### 9. 文件权限问题 (G301/G306/G302)

| 规则 | 数量 | 描述 |
|-----|------|-----|
| G301 | 180 | 目录权限应 ≤ 0750 |
| G306 | 154 | 写文件权限应 ≤ 0600 |
| G302 | 10 | 文件权限应 ≤ 0600 |

### 10. 子进程变量注入 (G204) - 数量: 206

**影响**: 命令执行风险

### 11. 文件包含漏洞 (G304) - 数量: 229

**影响**: 敏感文件读取

---

## 环境变量敏感信息检查

**发现**:
- SMTP配置从环境变量读取：`SMTP_HOST`, `SMTP_USER`, `SMTP_PASS`
- Docker配置：`DOCKER_HOST`, `GITHUB_TOKEN`
- CSRF密钥：`NAS_CSRF_KEY`

**状态**: ✅ 正确使用环境变量，未发现硬编码

---

## 修复优先级

### P0 - 立即修复（7天内）

1. **命令注入 (G702)** - 10处
2. **路径遍历 (G703)** - 核心模块31处WebDAV问题
3. **XSS (G705)** - 2处

### P1 - 高优先级（14天内）

1. **TLS不安全配置 (G402)** - 4处
2. **解压缩炸弹 (G110)** - 4处
3. **硬编码凭证审查 (G101)** - 7处

### P2 - 中优先级（30天内）

1. **整数溢出 (G115)** - 91处
2. **文件权限问题** - 344处

### P3 - 低优先级（持续改进）

1. **未处理错误 (G104)** - 701处
2. **子进程变量注入 (G204)** - 206处
3. **文件包含漏洞 (G304)** - 229处

---

## 安全最佳实践建议

### 1. 输入验证
- 对所有用户输入进行白名单验证
- 使用 `filepath.Clean()` 和路径边界检查
- 对命令参数进行转义或参数化

### 2. 安全配置
- 生产环境禁用 `InsecureSkipVerify`
- 文件权限统一设置为最小必要权限
- 敏感凭证使用 secrets 管理

### 3. 资源保护
- 实现请求大小限制
- 解压缩操作添加大小/数量限制
- 添加超时和取消机制

### 4. 错误处理
- 所有错误必须处理或明确忽略（使用 `_` 并注释原因）
- 敏感错误信息不应返回给用户

### 5. 代码审计
- 定期运行 gosec 扫描（建议 CI 集成）
- 新代码必须通过安全扫描才能合并

---

## 附录

### 扫描命令
```bash
gosec -fmt=json -out=gosec_report_v2.101.0.json ./...
```

### 按规则统计

| 规则ID | 数量 | 描述 |
|--------|------|------|
| G104 | 701 | 未处理错误 |
| G304 | 229 | 文件包含漏洞 |
| G204 | 206 | 子进程变量注入 |
| G301 | 180 | 目录权限问题 |
| G306 | 154 | 写文件权限问题 |
| G115 | 91 | 整数溢出 |
| G703 | 48 | 路径遍历 |
| G118 | 10 | Goroutine上下文问题 |
| G302 | 10 | 文件权限问题 |
| G702 | 10 | 命令注入 |
| G101 | 7 | 硬编码凭证 |
| G107 | 7 | URL变量注入 |
| G122 | 7 | TOCTOU竞态条件 |
| G110 | 4 | 解压缩炸弹 |
| G402 | 4 | TLS不安全配置 |
| G505 | 3 | 弱加密算法(SHA1) |
| G117 | 2 | 敏感字段暴露 |
| G705 | 2 | XSS漏洞 |
| G120 | 1 | 表单大小限制 |
| G305 | 1 | 压缩包文件遍历 |
| G707 | 1 | SMTP注入 |

---

**审计完成时间**: 2026-03-16 08:51  
**报告生成**: 刑部安全审计组