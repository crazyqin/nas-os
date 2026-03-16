# 安全审计报告 v2.105.0

**审计日期**: 2026-03-16 10:07 CST  
**审计工具**: gosec v2 + 手动审计  
**项目**: nas-os  
**版本**: v2.105.0  
**审计人**: 刑部（安全合规）

---

## 执行摘要

本次安全审计针对 v2.105.0 版本进行了全面的安全评估，包括静态代码分析、敏感信息检查、依赖安全性审查等方面。

**总体评估:** ⚠️ 需要关注（存在高危漏洞，建议修复后发布）

发现 **1678 个安全问题**，按严重程度分布如下：

| 严重级别 | 数量 | 占比 |
|---------|------|------|
| HIGH | 169 | 10.1% |
| MEDIUM | 808 | 48.2% |
| LOW | 701 | 41.8% |

### 核心风险领域

1. **整数溢出风险** - 91处（G115）
2. **路径遍历漏洞** - 48处（G703）
3. **命令注入风险** - 10处（G702）
4. **硬编码凭证风险** - 7处（G101）
5. **TOCTOU竞态条件** - 7处（G122）
6. **TLS不安全配置** - 4处（G402）

---

## 高危漏洞详情

### 1. 整数溢出漏洞 (G115) - 严重度: HIGH (91处)

**影响**: 数据损坏、逻辑错误、潜在安全绕过

**主要分布**:

| 文件 | 问题数 |
|-----|-------|
| `internal/storage/smart_monitor.go` | 4 |
| `internal/monitor/metrics_collector.go` | 4 |
| `internal/disk/smart_monitor.go` | 2 |
| `internal/reports/datasource.go` | 5 |
| `internal/quota/optimizer/optimizer.go` | 4 |
| `internal/system/monitor.go` | 2 |

**修复建议**:
```go
func safeUint64ToInt64(v uint64) (int64, error) {
    if v > math.MaxInt64 {
        return 0, errors.New("value too large")
    }
    return int64(v), nil
}
```

### 2. 路径遍历漏洞 (G703) - 严重度: HIGH (48处)

**影响**: 攻击者可读取/写入任意文件，导致数据泄露或系统被控制

**受影响模块**:

| 模块 | 问题数 | 文件 |
|-----|-------|------|
| WebDAV服务 | 30处 | `internal/webdav/server.go` |
| VM管理 | 5处 | `internal/vm/manager.go`, `snapshot.go` |
| 备份系统 | 6处 | `internal/backup/` |
| 安全模块 | 2处 | `internal/security/v2/encryption.go` |

**修复建议**:
```go
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

### 3. 命令注入漏洞 (G702) - 严重度: HIGH (10处)

**影响**: 攻击者可执行任意系统命令，完全控制服务器

**发现位置**:

| 文件 | 行号 | 风险描述 |
|-----|------|---------|
| `internal/vm/manager.go` | 309, 553, 585, 587, 615, 620, 716 | VM管理命令注入 |
| `internal/vm/snapshot.go` | 276, 289, 319 | 快照操作命令注入 |

**修复建议**:
```go
// 使用参数化执行，避免命令拼接
cmd := exec.Command("qemu-img", "create", filepath.Base(userPath), size)
// 添加白名单验证
if !isValidPath(userPath) {
    return errors.New("invalid path")
}
```

### 4. 硬编码凭证风险 (G101) - 严重度: HIGH (7处)

**影响**: 敏感信息泄露风险

**发现位置**:

| 文件 | 行号 | 风险点 |
|-----|------|-------|
| `internal/auth/oauth2.go` | 334-389 | OAuth2 ClientSecret字段 |
| `internal/cloudsync/providers.go` | 670, 1320 | Token URL暴露 |
| `internal/office/types.go` | 581 | Token相关 |

**修复建议**:
- 使用环境变量或安全配置存储敏感凭证
- 启用 secrets 管理服务（如 Vault、AWS Secrets Manager）

### 5. TOCTOU竞态条件 (G122) - 严重度: HIGH (7处)

**影响**: 时间检查-使用竞态，可能导致权限绕过

**发现位置**:

| 文件 | 行号 |
|-----|------|
| `internal/snapshot/replication.go` | 827 |
| `internal/plugin/hotreload.go` | 348 |
| `internal/files/manager.go` | 892 |
| `internal/backup/manager.go` | 822 |
| `internal/backup/encrypt.go` | 83 |
| `internal/backup/config_backup.go` | 262 |
| `internal/backup/advanced/verification.go` | 192 |

**修复建议**: 使用原子操作或文件锁保护关键操作

### 6. TLS不安全配置 (G402) - 严重度: HIGH (4处)

**影响**: 中间人攻击风险

**发现位置**:

| 文件 | 行号 | 描述 |
|-----|------|------|
| `internal/auth/ldap.go` | 154, 184 | InsecureSkipVerify |
| `internal/ldap/client.go` | 74, 104 | InsecureSkipVerify |

**状态**: ⚠️ 有 `#nosec` 注释说明，用户可配置，但需关注

---

## 中危漏洞详情

### 7. 未处理错误 (G104) - 数量: 701

**影响**: 程序异常、静默失败

**修复建议**: 系统性审查所有忽略错误返回值的调用

### 8. 子进程变量注入 (G204) - 数量: 206

**影响**: 命令执行风险

### 9. 文件包含漏洞 (G304) - 数量: 229

**影响**: 敏感文件读取

### 10. 文件权限问题 (G301/G306/G302) - 数量: 344

| 规则 | 数量 | 描述 |
|-----|------|-----|
| G301 | 180 | 目录权限应 ≤ 0750 |
| G306 | 154 | 写文件权限应 ≤ 0600 |
| G302 | 10 | 文件权限应 ≤ 0600 |

---

## 敏感信息处理审计

### ✅ 环境变量正确使用

项目正确使用环境变量存储敏感信息：

- `NAS_CSRF_KEY` - CSRF密钥
- `SMTP_HOST/USER/PASS` - SMTP配置
- `DOCKER_HOST` - Docker配置
- `GITHUB_TOKEN` - GitHub Token

### ⚠️ 敏感字段JSON暴露

以下字段可被JSON序列化，存在泄露风险：

| 文件 | 字段 |
|------|------|
| `internal/backup/sync.go:96` | SecretKey |
| `internal/backup/sync.go:101` | Password |
| `internal/backup/manager.go:157` | Password |
| `internal/snapshot/replication.go:88` | APIKey |
| `internal/cloudsync/types.go:89` | SecretKey |

**建议**: 添加 `json:"-"` 标签防止意外序列化

---

## v2.101.0 -> v2.105.0 变更审计

### 变更统计

- 50 files changed
- 54994 insertions
- 1535 deletions

### 主要变更

1. **弃用API修复** ✅
   - `ioutil.ReadFile` → `os.ReadFile`
   - `ldap.DialTLS` → `ldap.DialURL`

2. **敏感信息处理改进** ✅
   - `CloudConfig.Sanitize()` 方法正确隐藏敏感字段

3. **新增CI辅助脚本** ✅
   - `scripts/ci-helper.sh` 添加

4. **删除备份文件** ✅
   - 清理 `.bak` 文件

---

## 依赖安全性

### 过时依赖 (需更新)

```
github.com/Azure/go-ntlmssp v0.0.0-20221128 → v0.1.0 可用
github.com/aws/aws-sdk-go-v2 v1.41.3 → v1.41.4 可用
github.com/aws/aws-sdk-go-v2/service/s3 v1.96.4 → v1.97.1 可用
github.com/GoogleCloudPlatform/opentelemetry-operations-go v1.30.0 → v1.31.0
```

### 建议

运行以下命令检查已知漏洞：
```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

---

## 修复优先级

### P0 - 立即修复（发布前）

1. **命令注入 (G702)** - 10处 - 核心VM模块
2. **路径遍历 (G703)** - 48处 - WebDAV/备份模块

### P1 - 高优先级（7天内）

1. **整数溢出 (G115)** - 91处
2. **TOCTOU竞态 (G122)** - 7处
3. **硬编码凭证审查 (G101)** - 7处

### P2 - 中优先级（30天内）

1. **TLS不安全配置 (G402)** - 4处
2. **文件权限问题** - 344处
3. **敏感字段添加 `json:"-"` 标签**

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
- 添加超时和取消机制

### 4. 错误处理
- 所有错误必须处理或明确忽略（使用 `_` 并注释原因）
- 敏感错误信息不应返回给用户

### 5. 代码审计
- 定期运行 gosec 扫描（建议 CI 集成）
- 新代码必须通过安全扫描才能合并

---

## 结论

NAS-OS v2.105.0 在安全控制方面整体架构良好，已实现多项安全防护措施。但存在以下需要关注的问题：

**必须修复后发布**:
- 命令注入漏洞 (10处) - VM模块
- 路径遍历漏洞 (48处) - WebDAV/备份模块

**建议尽快修复**:
- 整数溢出问题 (91处)
- TOCTOU竞态条件 (7处)
- 敏感字段JSON序列化问题

**安全状态**: ⚠️ 建议修复 P0 问题后发布

---

**审计完成时间**: 2026-03-16 10:07 CST  
**下次审计建议**: 重大功能变更后或每季度一次