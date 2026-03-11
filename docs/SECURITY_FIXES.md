# 安全修复建议

## 已执行修复

### ✅ Go 版本升级
- **问题**: GO-2026-4601, GO-2026-4600, GO-2026-4599
- **修复**: 更新 `go.mod` 从 `go 1.25.0` 到 `go 1.26.1`
- **操作**: 运行 `go mod tidy` 和 `go get -u` 更新依赖

## 待修复问题

### 🔴 高危

#### 1. 路径遍历风险 (G703)
**位置**: `internal/backup/manager.go:648`

**问题代码**:
```go
return os.WriteFile(dstPath, data, info.Mode())
```

**修复方案**:
```go
// 添加路径验证
cleanPath := filepath.Clean(dstPath)
if !strings.HasPrefix(cleanPath, expectedBaseDir) {
    return fmt.Errorf("invalid path: %s", dstPath)
}
return os.WriteFile(cleanPath, data, info.Mode())
```

**优先级**: 立即修复

---

#### 2. 符号链接 TOCTOU 风险 (G122)
**位置**: `internal/backup/manager.go:644`

**问题代码**:
```go
data, err := os.ReadFile(path)
```

**修复方案**:
```go
// 使用 os.Root 或验证符号链接
realPath, err := filepath.EvalSymlinks(path)
if err != nil {
    return err
}
// 验证 realPath 在预期目录内
data, err := os.ReadFile(realPath)
```

**优先级**: 立即修复

---

### 🟡 中危

#### 3. 弱加密原语 (G401)
**位置**: `internal/network/ddns_providers.go:559`

**问题代码**:
```go
func sha1Hex(s string) string {
    h := sha1.New()
    h.Write([]byte(s))
    return hex.EncodeToString(h.Sum(nil))
}
```

**修复方案**:
```go
import "crypto/sha256"

func sha256Hex(s string) string {
    h := sha256.New()
    h.Write([]byte(s))
    return hex.EncodeToString(h.Sum(nil))
}
```

**注意**: 需要检查所有调用 `sha1Hex` 的地方，确保兼容性

**优先级**: 7 天内修复

---

#### 4. 命令注入风险 (G204)
**位置**: `pkg/btrfs/btrfs.go` (多处)

**问题代码**:
```go
cmd = exec.Command("mount", args...)
```

**修复方案**:
```go
// 验证所有参数
for _, arg := range args {
    if strings.Contains(arg, ";") || strings.Contains(arg, "|") || 
       strings.Contains(arg, "&") || strings.Contains(arg, "$") {
        return fmt.Errorf("invalid argument: %s", arg)
    }
}
cmd = exec.Command("mount", args...)
```

**优先级**: 7 天内修复

---

## 修复检查清单

- [ ] 更新 Go 版本至 1.26.1
- [ ] 运行 `go mod tidy`
- [ ] 修复路径遍历问题 (backup/manager.go)
- [ ] 修复 TOCTOU 问题 (backup/manager.go)
- [ ] 升级 SHA1 到 SHA256 (ddns_providers.go)
- [ ] 添加命令参数验证 (btrfs.go)
- [ ] 重新运行安全扫描验证修复
- [ ] 更新测试用例
- [ ] 代码审查
- [ ] 发布安全补丁版本

## 验证修复

```bash
# 运行完整安全检查
./scripts/security-check.sh --notify

# 验证无高危问题
gosec -fmt=text ./... | grep "Severity: HIGH" || echo "✓ 无高危问题"
govulncheck ./... | grep "Vulnerability" || echo "✓ 无依赖漏洞"
```

## 参考文档

- [SECURITY_RESPONSE.md](./SECURITY_RESPONSE.md) - 安全响应流程
- [SECURITY_CRON_SETUP.md](./SECURITY_CRON_SETUP.md) - 定时检查设置
- [security-scan.yml](../.github/workflows/security-scan.yml) - CI/CD 集成
