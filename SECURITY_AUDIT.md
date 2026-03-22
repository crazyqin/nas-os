# NAS-OS 安全审计报告

**审计日期**: 2026-03-22  
**审计工具**: gosec v2  
**总问题数**: 678

## 问题统计

| 严重程度 | 数量 | 类型 | CWE | 说明 |
|---------|------|------|-----|------|
| 高危 | 224 | G304 | CWE-22 | 文件路径注入风险 |
| 高危 | 186 | G204 | CWE-78 | 命令执行风险 |
| 高危 | 7 | G101 | CWE-798 | 硬编码凭据（误报） |
| 高危 | 7 | G122 | CWE-367 | TOCTOU 竞态条件 |
| 中危 | 125 | G306 | CWE-276 | 文件权限过宽 (0640 → 0600) |
| 中危 | 67 | G115 | CWE-190 | 整数溢出风险 |
| 中危+低危 | ~72 | 其他 | - | 日志格式化、错误处理等 |

## 优先修复建议

### 1. G304 文件路径注入 (高危, 224个)
**影响文件**: 98个文件未标注 `#nosec`

**修复方案**:
- 已有 `internal/security/pathutil` 安全模块
- 建议: 在处理用户输入路径时调用 `pathutil.SafePath()` 或 `pathutil.SafeJoin()`

**示例修复**:
```go
// Before
data, err := os.ReadFile(userPath)

// After
safePath, err := pathutil.SafePath(baseDir, userPath)
if err != nil {
    return err
}
data, err := os.ReadFile(safePath)
```

### 2. G204 命令执行 (高危, 186个)
**风险**: 用户输入可能被注入到 shell 命令中

**修复方案**:
- 已验证的输入可标注 `#nosec G204 -- reason`
- 未验证的输入需添加白名单验证

### 3. G101 硬编码凭据 (高危, 7个) 
**状态**: 全部为误报

**原因**: 函数参数名包含 `clientID`、`clientSecret` 等关键词，实际不是硬编码凭据

### 4. G122 TOCTOU 竞态 (高危, 7个)
**位置**: `filepath.Walk` 回调中的文件操作

**修复方案**: 使用 `os.Root` (Go 1.24+) 或在回调外处理文件操作

### 5. G306 文件权限 (中危, 125个)
**问题**: `os.WriteFile(..., 0640)` 权限过宽

**修复方案**: 将权限改为 `0600`（仅所有者读写）
```go
// Before
os.WriteFile(path, data, 0640)

// After  
os.WriteFile(path, data, 0600)
```

## 现有安全措施

✅ 已实现 `internal/security/pathutil` 路径安全验证模块  
✅ 已实现 `internal/security/scanner` 漏洞扫描模块  
✅ 已实现 `internal/security/fail2ban` 入侵防护  
⚠️ 安全模块使用率低（仅12处调用）

## 下一步行动

1. **立即**: 修复 G306 文件权限问题（可批量处理）
2. **短期**: 扩大 `pathutil` 模块使用范围
3. **中期**: 添加命令执行安全验证
4. **持续**: 代码审查确认高危问题是否真正存在风险