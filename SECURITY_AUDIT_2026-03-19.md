# NAS-OS 安全审计报告

**审计日期**: 2026-03-19  
**项目**: ~/clawd/nas-os  
**审计部门**: 刑部  
**审计范围**: 整数溢出、TLS 配置、硬编码凭证、注入风险

---

## 一、审计结论

**整体安全状况**: ⚠️ **中等风险**（与上次审计一致）

| 检查项 | 状态 | 问题数 | 说明 |
|-------|------|--------|------|
| 整数溢出 (uint64→int64) | ✅ 已有防护 | 17处使用 safeguards | 已实现安全转换包 |
| TLS InsecureSkipVerify | ⚠️ 需关注 | 4处 | 有条件启用，需审计 |
| 硬编码凭证 | ✅ 通过 | 0 | 无实际凭证泄露 |
| 命令注入 | ⚠️ 需关注 | 4处 sh -c | 部分有防护，部分需加强 |
| SQL 注入 | ✅ 通过 | 0 | 使用参数化查询 |
| 路径遍历 | ✅ 有防护 | - | 有 validatePath 函数 |

---

## 二、详细发现

### 2.1 整数溢出风险 ✅ 已有防护

**安全转换包** (`pkg/safeguards/convert.go`):
```go
// 已实现的安全转换函数
func SafeUint64ToInt64(val uint64) (int64, error)
func SafeUint64ToInt(val uint64) (int, error)
func SafeAddUint64(a, b uint64) (uint64, error)
func SafeMulUint64(a, b uint64) (uint64, error)
```

**使用情况统计**:
- 17处代码使用了 safeguards 包进行安全转换
- 主要位置: `internal/monitor/metrics_collector.go`, `internal/search/engine.go`, `internal/quota/optimizer/optimizer.go`

**潜在风险点** (未使用安全转换):
- `internal/reports/storage_usage_report.go` - 多处 uint64 运算
- `internal/reports/capacity_planning.go` - 存储容量预测计算
- `internal/performance/collector.go` - 性能指标计算

**建议**: 对存储容量、性能指标等大数值计算场景，统一使用 safeguards 包。

### 2.2 TLS InsecureSkipVerify 使用情况 ⚠️

**发现 4 处使用**:

| 文件 | 行号 | 风险等级 | 说明 |
|-----|------|---------|------|
| `internal/backup/cloud.go` | 136 | 🟡 中 | 用户配置 `cfg.Insecure` 控制 |
| `internal/backup/sync.go` | 670 | 🟡 中 | 用户配置控制 |
| `internal/cloudsync/providers.go` | 399 | 🟡 中 | 用户配置控制 |
| `internal/ldap/client.go` | 73, 107 | 🟢 低 | 仅测试环境启用 (`ENV=test`) |

**LDAP 实现较好**:
```go
// 仅测试环境允许跳过 TLS 验证
skipVerify := c.config.SkipTLSVerify && os.Getenv("ENV") == "test"
```

**建议**:
1. 对云备份场景，添加配置审计日志
2. 考虑添加证书固定 (certificate pinning) 选项

### 2.3 硬编码凭证检查 ✅ 通过

**检查结果**: 无实际硬编码凭证

**误报分析**:
- `internal/cloudsync/providers.go` - OAuth2 Token URL，非凭证
- `internal/security/v2/handlers.go` - OTP URL 模板，非凭证
- `internal/web/api_types.go` - API 示例字段，非凭证

**良好实践**:
- 备份密钥通过环境变量传递 (`NAS_BACKUP_KEY`)
- iSCSI CHAP 密码通过 stdin 传递
- 配置文件有 `Sanitize()` 方法脱敏

### 2.4 命令注入风险 ⚠️ 需关注

**exec.Command 使用统计**: 316 处

**高风险 `sh -c` 执行**: 4 处

| 文件 | 行号 | 风险 | 防护措施 |
|-----|------|------|---------|
| `internal/snapshot/executor.go` | 120 | 🟡 中 | 无，脚本来自配置 |
| `internal/usbmount/manager.go` | 1006 | 🟢 低 | `sanitizeEnvValue()` + 白名单 |
| `internal/container/volume.go` | 234, 300 | 🟡 中 | `filepath.Base()` 清理 |

**USB Mount 实现较好**:
```go
// sanitizeEnvValue 移除注入字符
func sanitizeEnvValue(value string) string {
    replacer := strings.NewReplacer(
        ";", "", "|", "", "&", "", "`", "", "$", "",
        "\n", "", "\r", "",
    )
    return replacer.Replace(value)
}
```

**建议**:
1. `internal/snapshot/executor.go` - 对脚本内容进行沙箱限制
2. `internal/container/volume.go` - 验证 `backupFile` 路径格式

### 2.5 SQL 注入风险 ✅ 通过

**检查结果**: 使用参数化查询

**良好实践示例**:
```go
// internal/tags/manager.go
err := m.db.QueryRow("SELECT 1 FROM tags WHERE name = ?", input.Name).Scan(&exists)
```

**唯一风险点**:
```go
// internal/database/optimizer.go:325
_, err := o.db.Exec(fmt.Sprintf("ANALYZE %s", table))
```
此处 `table` 来自内部调用，非用户输入，风险可控。

### 2.6 路径遍历防护 ✅ 有防护

**文件管理器防护** (`plugins/filemanager-enhance/main.go`):
```go
func (p *FileManagerEnhance) isPathAllowed(path string) bool {
    cleanRoot := filepath.Clean(p.rootPath)
    cleanPath := filepath.Clean(filepath.Join(cleanRoot, path))
    // 确保路径在根目录下
    if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) && cleanPath != cleanRoot {
        return false
    }
    return true
}
```

**内部文件操作** (`internal/files/manager.go`):
- 有 `validatePath()` 函数验证用户路径

---

## 三、加密实现评估 ✅ 良好

**算法选择**:
- ✅ AES-256-GCM (AEAD，推荐)
- ✅ Argon2id 密钥派生 (抗暴力破解)

**实现位置**: `internal/security/v2/encryption.go`

```go
func NewEncryptionManager() *EncryptionManager {
    return &EncryptionManager{
        config: EncryptionConfig{
            Algorithm:     "aes-256-gcm",
            KeyDerivation: "argon2id",
        },
    }
}
```

---

## 四、修复优先级建议

### P0 - 立即处理

| 项目 | 位置 | 建议 |
|-----|------|------|
| 脚本执行沙箱 | `internal/snapshot/executor.go:120` | 限制脚本权限，添加审计日志 |

### P1 - 本周内

| 项目 | 位置 | 建议 |
|-----|------|------|
| 整数溢出 | `internal/reports/*.go` | 对大数值计算使用 safeguards |
| TLS 审计 | `internal/backup/cloud.go` | 添加跳过证书验证的审计日志 |

### P2 - 持续改进

| 项目 | 位置 | 建议 |
|-----|------|------|
| 命令注入 | `internal/container/volume.go` | 增强路径验证 |
| 证书固定 | 云备份模块 | 考虑添加证书固定选项 |

---

## 五、安全架构评价

| 方面 | 评估 | 说明 |
|-----|------|------|
| 凭证管理 | ✅ 优秀 | 无硬编码，环境变量传递敏感信息 |
| 加密方案 | ✅ 优秀 | AES-256-GCM + Argon2id |
| SQL 安全 | ✅ 优秀 | 全面使用参数化查询 |
| 整数安全 | ✅ 良好 | 有 safeguards 包，部分位置需补全 |
| 命令执行 | ⚠️ 中等 | 需加强脚本执行防护 |
| TLS 配置 | ⚠️ 中等 | 有条件启用，需审计日志 |

---

## 六、总结

本次审计延续了之前的安全评估，项目整体安全架构设计合理。主要改进：

**自上次审计以来的改进**:
- ✅ 整数溢出防护包 (safeguards) 已广泛使用
- ✅ USB Mount 添加了环境变量清理函数
- ✅ 文件管理器路径验证完善

**待改进项**:
1. 脚本执行 (`sh -c`) 需要沙箱隔离
2. TLS 跳过验证需要审计日志
3. 部分大数值计算场景需使用安全转换

**整体评估**: 项目安全实践良好，主要风险在命令执行层面，建议优先加固脚本执行安全。

---

**审计人**: 刑部安全审计系统  
**报告生成时间**: 2026-03-19 05:30 CST