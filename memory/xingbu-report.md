## 刑部安全报告

### 审计概要
项目：nas-os  
审计时间：2026-03-20  
审计范围：代码安全审查  

---

### 高危问题

#### 1. 命令注入风险 (CRITICAL)

**位置**: `internal/backup/manager.go:450-559`

**问题代码**:
```go
cmd := exec.CommandContext(ctx, "tar", "czf", backupPath, "-C", cfg.Source, ".")
cmd = exec.CommandContext(ctx, "scp", scpArgs...)
cmd := exec.CommandContext(ctx, "rsync", args...)
cmd := exec.CommandContext(ctx, "openssl", "enc", "-aes-256-cbc", "-salt", "-in", backupPath, "-out", encryptedPath, "-pass", "env:NAS_BACKUP_KEY="+encryptKey)
```

**风险分析**:
- `backupPath`、`cfg.Source`、`scpArgs`、`args` 等参数如果未严格验证，可被注入恶意命令
- 特别是 `encryptKey` 直接拼接到环境变量中，存在泄露风险
- rsync 的 `args` 如果包含用户输入，可能导致命令注入

**影响**: 远程代码执行 (RCE)

**修复建议**:
1. 所有外部命令参数必须使用白名单验证
2. 禁止直接拼接用户输入到命令参数
3. 使用 `exec.Command` 的参数数组形式（当前已使用，但参数来源需验证）
4. 对文件路径进行严格校验，禁止 `..`、`;`、`|`、`&` 等特殊字符

---

#### 2. 路径遍历漏洞 (HIGH)

**位置**: `plugins/filemanager-enhance/main.go:230-520`

**问题代码**:
```go
for _, src := range req.Files {
    filename := filepath.Base(src)
    dst := filepath.Join(req.Target, filename)
    // 直接使用用户输入的文件路径
}
```

**风险分析**:
- 虽然有 `validatePaths` 函数进行路径校验，但校验逻辑在第 516-527 行：
```go
cleanRoot := filepath.Clean(p.rootPath)
cleanPath := filepath.Clean(filepath.Join(cleanRoot, path))
if !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) && cleanPath != cleanRoot {
    return fmt.Errorf("路径不在允许范围内")
}
```
- 但 `validatePaths` 仅在部分函数中被调用，`batchCopy`、`batchMove` 等函数中的路径验证依赖 `validatePaths`，需要确认所有路径操作都经过验证

**影响**: 攻击者可读取/写入任意系统文件

**修复建议**:
1. 强制所有文件操作路径必须经过 `validatePaths` 验证
2. 使用 `filepath.Clean()` + `strings.HasPrefix()` 双重校验
3. 禁止符号链接跟随（可选）

---

### 中危问题

#### 3. 敏感信息泄露风险 (MEDIUM)

**位置**: `internal/backup/manager.go:66-80`

**问题**:
```go
RemotePassword string `json:"-"` // 敏感信息，不序列化到 JSON
EncryptionKey   string `json:"-"` // 敏感信息，不序列化到 JSON
```

**风险分析**:
- 虽然标记为 `json:"-"` 不序列化到 JSON，但：
  1. 第 476 行将 `encryptKey` 放入环境变量：`cmd.Env = append(os.Environ(), "NAS_BACKUP_KEY="+encryptKey)`
  2. 环境变量可被 `/proc/[pid]/environ` 读取
  3. 可能出现在日志、错误信息中

**修复建议**:
1. 使用安全的内存存储机制（如 `memguard`）
2. 避免将密钥放入环境变量，改用文件描述符传递
3. 确保密钥在使用后立即清零内存

---

#### 4. SQL 注入风险较低 (MEDIUM-LOW)

**位置**: `internal/database/optimizer.go`, `internal/system/monitor.go`

**分析**:
```go
rows, err := m.db.Query(query, timeRange)  // 参数化查询 ✓
_, err := m.db.Exec(query, args...)         // 参数化查询 ✓
```

**结论**: 
- 大部分数据库操作使用参数化查询（`$1`, `?` 占位符）
- 但第 325 行存在动态 SQL 构建：
```go
_, err := o.db.Exec(fmt.Sprintf("ANALYZE %s", table))
```
- 需确保 `table` 变量来自可信源

**修复建议**:
1. 对动态表名进行白名单校验
2. 使用正则表达式验证表名格式：`^[a-zA-Z_][a-zA-Z0-9_]*$`

---

#### 5. 敏感文件路径泄露 (MEDIUM)

**位置**: `internal/backup/cloud.go:78`

**问题**:
```go
"has_secret_key": cc.SecretKey != "",
```

**风险分析**:
- 仅检查 SecretKey 是否存在，未暴露具体值
- 但攻击者可通过此字段判断系统是否配置了密钥
- 属于信息泄露，但影响有限

---

### 低危问题

#### 6. 缺少输入长度限制 (LOW)

**问题**: 多处未对用户输入进行长度限制

**修复建议**:
- 文件名长度限制（建议 255 字符）
- 路径长度限制（建议 4096 字符）
- 批量操作数量限制（建议 1000 个文件/次）

---

#### 7. 错误信息可能泄露内部路径 (LOW)

**位置**: 多处 `fmt.Errorf` 调用

**问题**:
```go
return fmt.Errorf("备份配置不存在：%s", id)
return fmt.Errorf("创建目标目录失败: %w", err)
```

**修复建议**:
- 对外暴露的错误信息应脱敏，避免泄露系统路径

---

### 修复建议优先级

| 优先级 | 问题 | 预计工作量 |
|--------|------|-----------|
| P0 | 命令注入风险 | 2-3 天 |
| P0 | 路径遍历漏洞 | 1-2 天 |
| P1 | 敏感信息泄露 | 1 天 |
| P2 | SQL 动态表名验证 | 0.5 天 |
| P3 | 输入长度限制 | 0.5 天 |
| P3 | 错误信息脱敏 | 0.5 天 |

---

### 风险评估

**整体安全等级**: 中等风险

**主要风险点**:
1. 备份模块存在命令注入风险（高危）
2. 文件管理插件存在路径遍历风险（高危）
3. 密钥管理需加强（中危）

**积极方面**:
1. 敏感字段正确标记 `json:"-"` 避免序列化
2. 数据库操作主要使用参数化查询
3. 路径验证函数已实现

**建议**:
1. 优先修复 P0 级别的命令注入和路径遍历问题
2. 引入安全审计工具（如 `gosec`）到 CI/CD 流程
3. 增加安全相关的单元测试
4. 定期进行依赖安全扫描（`govulncheck`）

---

### 依赖安全检查

**检查结果**: 项目依赖较新，未发现明显过时的依赖包

**建议**:
```bash
# 安装 gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# 安装 govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# 运行安全扫描
gosec ./...
govulncheck ./...
```

---

*刑部审计完毕*