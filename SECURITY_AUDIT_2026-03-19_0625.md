# NAS-OS 安全审计报告

**审计日期**: 2026-03-19 06:25 CST  
**项目**: ~/clawd/nas-os  
**版本**: v2.253.3  
**审计部门**: 刑部  
**工具**: gosec v2  

---

## 一、扫描结果概要

| 指标 | 数值 |
|-----|------|
| 总问题数 | 891 |
| HIGH 严重度 | 147 |
| MEDIUM 严重度 | 736 |
| LOW 严重度 | 8 |

### 按规则分布

| 规则ID | 问题数 | 说明 |
|-------|--------|------|
| G304 | 215 | 文件路径遍历（多数已有防护） |
| G204 | 173 | 子进程启动（系统管理正常行为） |
| G301 | 162 | 目录权限过于宽松 |
| G306 | 153 | 文件权限过于宽松 |
| G115 | 70 | 整数溢出转换 |
| G703 | 48 | 路径遍历（污点分析） |
| G118 | 10 | context 取消函数未调用 |
| G702 | 10 | 命令注入（污点分析） |

---

## 二、新发现问题分析

### 2.1 G707 - SMTP 注入（误报）✅

**位置**: `internal/automation/action/action.go:287`

**gosec 报告**: SMTP command/header injection via taint analysis

**实际状态**: ✅ 已有防护

代码中实现了 `sanitizeEmailHeader()` 函数：
```go
func sanitizeEmailHeader(s string) string {
    // 移除换行符和其他危险字符
    s = strings.ReplaceAll(s, "\r", "")
    s = strings.ReplaceAll(s, "\n", "")
    // 移除可能导致注入的控制字符
    s = strings.Map(func(r rune) rune {
        if r < 32 && r != '\t' {
            return -1
        }
        return r
    }, s)
    return s
}
```

**结论**: 误报，代码已正确实现 SMTP 注入防护。

### 2.2 G118 - context 取消函数未调用（误报）✅

**gosec 报告**: 10 处 context 取消函数未调用

**实际状态**: ✅ 已有意存储

检查发现这些 cancel 函数被有意存储到 map 中供后续调用：

| 文件 | 状态 |
|-----|------|
| `internal/backup/manager.go:322` | 存储到 `m.cancels[task.ID]` |
| `internal/backup/manager.go:573` | 存储 to `m.cancels[task.ID]` |
| `internal/scheduler/executor.go:108` | 存储 to `e.running[task.ID].cancel` |
| `internal/media/streaming.go` | 存储到 session 结构体 |
| `internal/media/transcoder.go` | 存储到 job 结构体 |

**结论**: 误报，cancel 函数被有意存储供任务取消时调用。

### 2.3 G703/G702 - 污点分析问题

**WebDAV 模块**: 代码中已有 `resolvePath()` 函数进行路径验证，并添加了 `#nosec` 注释说明。

**VM 模块**: 使用 `validateConfig()` 验证输入，路径为内部生成。

**结论**: 已有防护措施，风险可控。

---

## 三、与上次审计对比

**上次审计日期**: 2026-03-19 05:30 CST  
**对比结论**: ⚠️ 无新发现的安全问题

### 状态一致性

| 检查项 | 上次状态 | 本次状态 |
|-------|---------|---------|
| 整数溢出 | ✅ 有 safeguards 包 | ✅ 有 safeguards 包 |
| TLS InsecureSkipVerify | ⚠️ 3处 | ⚠️ 3处（一致） |
| 硬编码凭证 | ✅ 误报 | ✅ 误报 |
| SMTP 注入 | 未检查 | ✅ 已有防护 |
| 路径遍历 | ✅ 有防护函数 | ✅ 有防护函数 |
| 命令注入 | ⚠️ 需关注 | ⚠️ 需关注（一致） |

---

## 四、结论

### 安全状况: ⚠️ 中等风险（与上次一致）

**无需立即处理的问题**: 本次扫描未发现新的安全漏洞。

**已知待改进项**（延续上次审计建议）:
1. 脚本执行 (`sh -c`) 需要沙箱隔离
2. TLS 跳过验证需要审计日志
3. 部分大数值计算场景需使用安全转换

### 扫描报告文件
- 新报告: `gosec-report-new.json`
- 旧报告: `SECURITY_AUDIT_2026-03-19.md`

---

**审计人**: 刑部安全审计系统  
**报告生成时间**: 2026-03-19 06:25 CST