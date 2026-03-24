# NAS-OS 安全扫描报告

**扫描时间**: 2026-03-24 18:08  
**扫描工具**: gosec v2  
**工作目录**: /home/mrafter/nas-os

## 扫描摘要

| 指标 | 数值 |
|------|------|
| 扫描文件数 | 509 |
| 代码行数 | 314,003 |
| 已忽略问题 | 79 |
| **发现问题** | **753** |

## 严重性分布

| 级别 | 数量 | 说明 |
|------|------|------|
| **高危 (HIGH)** | **0** | ✅ 无高危问题 |
| 中危 (MEDIUM) | ~700+ | 文件权限、弱加密 |
| 低危 (LOW) | ~5 | 未处理错误 |

## 主要问题分析

### 1. G306 - 文件写入权限过宽 (中危)

**问题数量**: ~680+ 处  
**CWE**: CWE-276 (Incorrect Permission Assignment)  
**描述**: `os.WriteFile` 使用 0640 权限，gosec 建议使用 0600 或更严格权限

**示例位置**:
- `internal/scheduler/scheduler.go:134`
- `internal/reports/exporter.go:211`
- `internal/backup/manager.go`
- 等大量文件

**风险评估**: 
- 0640 = 所有者读写 + 组用户只读
- 实际风险较低，配置/数据文件通常需要组可读
- 建议：根据实际需求评估，敏感数据文件可降至 0600

### 2. G505/G501 - 弱加密原语 (中危)

**问题数量**: 4 处  
**CWE**: CWE-327 (Use of Broken Crypto)

| 文件 | 问题 |
|------|------|
| `internal/tunnel/turn.go:8` | crypto/sha1 |
| `internal/cloudsync/provider_quark.go:8` | crypto/sha1 |
| `internal/cloudsync/provider_alipan.go:8` | crypto/sha1 |
| `internal/cloudsync/provider_115.go:8` | crypto/md5 |

**风险评估**:
- 这些可能是用于文件校验/哈希计算，非加密用途
- SHA1/MD5 用于校验和是可接受的
- 若用于安全敏感场景需替换为 SHA256+

### 3. G104 - 未处理错误 (低危)

**问题数量**: ~5 处  
**CWE**: CWE-703 (Improper Check for Exceptional Conditions)

**示例位置**:
- `internal/audit/enhanced/smb_audit_api.go:291` - JSON 编码错误未处理
- `internal/audit/enhanced/smb_audit_api.go:283` - HTTP 写入错误未处理

**建议**: 添加错误处理，至少记录日志

## 安全状态评估

### ✅ 无高危问题

项目代码未发现高危安全漏洞，基础安全状况良好。

### ⚠️ 需关注项

1. **文件权限**: 0640 vs 0600 的警告数量巨大，但实际风险需根据部署环境评估
2. **弱哈希算法**: 4处使用 SHA1/MD5，需确认用途是否为安全敏感场景

### 建议优先级

| 优先级 | 问题 | 建议 |
|--------|------|------|
| P2 | G505/G501 弱加密 | 检查用途，非安全敏感可忽略 |
| P3 | G306 文件权限 | 评估敏感文件，按需收紧权限 |
| P3 | G104 错误处理 | 添加错误日志记录 |

---

**刑部 安全合规报告**  
*生成于 2026-03-24*