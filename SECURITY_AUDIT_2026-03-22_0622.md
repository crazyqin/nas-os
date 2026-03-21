# 安全审计报告

**项目**: nas-os  
**审计时间**: 2026-03-22 06:22 CST  
**扫描工具**: gosec v2.x  
**审计员**: 刑部（安全审计子代理）

---

## 执行摘要

| 严重程度 | 数量 | 状态 |
|---------|------|------|
| 🔴 **高危 (HIGH)** | 143 | ⚠️ 需关注 |
| 🟠 **中危 (MEDIUM)** | 716 | ⚠️ 需关注 |
| 🟢 **低危 (LOW)** | 1 | ✅ 可接受 |
| **总计** | **860** | |

### 与上次扫描对比

| 指标 | 本次 | 上次 (v2.253.146) | 变化 |
|------|------|-------------------|------|
| 高危 | 143 | 143 | ➖ 无变化 |
| 中危 | 716 | 716 | ➖ 无变化 |
| 低危 | 1 | 1 | ➖ 无变化 |
| **总计** | **860** | **860** | **➖ 无变化** |

**结论**: 安全状态稳定，无新增或修复的问题。

---

## 漏洞类型分布

| 规则ID | 描述 | 数量 | 风险 |
|--------|------|------|------|
| G304 | 文件路径注入 | 216 | 🔴 高 |
| G204 | 命令执行/注入 | 175 | 🔴 高 |
| G301 | 文件权限不当 | 165 | 🟠 中 |
| G306 | 文件权限不当 | 135 | 🟠 中 |
| G115 | 整数溢出 | 67 | 🔴 高 |
| G703 | 未检查返回值 | 48 | 🟠 中 |
| G302 | 文件权限不当 | 10 | 🟠 中 |
| G702 | 未检查返回值 | 10 | 🟠 中 |
| G101 | 硬编码凭证 | 7 | 🟠 中 |
| G122 | 不安全随机数 | 7 | 🟠 中 |
| G107 | SSRF风险 | 5 | 🟠 中 |
| G110 | 潜在DoS | 3 | 🟠 中 |
| G118 | 未检查返回值 | 3 | 🟠 中 |
| G117 | 未检查返回值 | 2 | 🟠 中 |
| G705 | 未检查返回值 | 2 | 🟠 中 |
| G104 | 未检查错误 | 1 | 🟠 中 |
| G305 | 文件遍历 | 1 | 🟠 中 |
| G402 | TLS配置问题 | 1 | 🟠 中 |
| G404 | 弱随机数 | 1 | 🟠 中 |
| G707 | 未检查返回值 | 1 | 🟠 中 |

---

## 高风险模块 (问题数 ≥ 10)

| 模块 | 问题数 | 主要风险类型 |
|------|--------|-------------|
| `internal/webdav/server.go` | 30 | 文件路径注入、命令执行 |
| `internal/files/manager.go` | 19 | 文件路径注入 |
| `internal/service/systemd.go` | 19 | 命令执行 |
| `internal/backup/manager.go` | 15 | 文件路径注入、命令执行 |
| `internal/backup/config_backup.go` | 14 | 文件路径注入 |
| `internal/cloudsync/providers.go` | 14 | 硬编码凭证、文件路径注入 |
| `internal/backup/sync.go` | 13 | 文件路径注入 |
| `internal/photos/handlers.go` | 13 | 文件路径注入 |
| `internal/photos/manager.go` | 13 | 文件路径注入 |
| `internal/automation/action/action.go` | 12 | 命令执行 |
| `internal/photos/ai.go` | 12 | 文件路径注入 |
| `internal/vm/manager.go` | 12 | 命令执行 |
| `internal/container/compose.go` | 11 | 命令执行 |
| `internal/docker/appstore.go` | 11 | 文件路径注入 |
| `internal/transfer/chunked.go` | 11 | 文件路径注入 |

---

## 详细风险分析

### 1. 文件路径注入 (G304) - 216处

**风险等级**: 🔴 高危  
**CWE**: CWE-22 (路径遍历)

**典型场景**:
```
internal/trash/manager.go:356 - Potential file inclusion via variable
internal/transfer/chunked.go:463 - Potential file inclusion via variable
```

**影响**: 攻击者可能通过构造特殊路径访问或操作预期之外的文件。

**建议**:
- 对所有外部输入的路径进行规范化处理
- 使用 `filepath.Clean()` 清理路径
- 验证路径是否在允许的目录范围内
- 考虑使用沙箱机制限制文件访问

---

### 2. 命令执行/注入 (G204) - 175处

**风险等级**: 🔴 高危  
**CWE**: CWE-78 (OS命令注入)

**典型场景**:
```
internal/snapshot/replication.go:1126 - Subprocess launched with variable
internal/service/systemd.go:318 - Subprocess launched with variable
```

**影响**: 可能导致远程命令执行漏洞。

**建议**:
- 避免直接拼接用户输入到命令
- 使用参数化方式执行命令
- 实现严格的命令白名单
- 对所有输入进行严格的验证和转义

---

### 3. 整数溢出 (G115) - 67处

**风险等级**: 🔴 高危  
**CWE**: CWE-190 (整数溢出)

**典型场景**:
```
internal/disk/smart_monitor.go:1130 - uint64 → int 转换
internal/vm/iso.go:228 - int64 → uint64 转换
```

**影响**: 可能导致数据截断、意外行为或安全问题。

**建议**:
- 添加边界检查
- 使用安全的类型转换函数
- 在关键路径添加溢出检测

---

### 4. 文件权限问题 (G301/G306/G302) - 310处

**风险等级**: 🟠 中危  
**CWE**: CWE-732 (关键资源权限配置不当)

**影响**: 可能导致敏感文件被非授权访问。

**建议**:
- 审查所有文件创建/修改操作的权限设置
- 确保配置文件权限为 0600
- 确保目录权限为 0755 或更严格
- 使用 `os.FileMode(0600)` 显式设置权限

---

### 5. 硬编码凭证 (G101) - 7处

**风险等级**: 🟠 中危  
**CWE**: CWE-798 (硬编码凭证)

**典型场景**:
```
internal/auth/oauth2.go - OAuth2 配置包含凭证字段
internal/cloudsync/providers.go - OAuth token URL
```

**分析**: 大部分为 OAuth2 Provider 配置，实际凭证通过参数传入，属于误报。但应审查确认无真实硬编码密钥。

**建议**:
- 审查所有 G101 告警
- 确保敏感凭证通过环境变量或密钥管理服务获取
- 对可疑告警添加 `#nosec G101` 注释并说明原因

---

## 优先修复建议

### 立即处理 (P0)
1. **命令注入点**: 审查 `internal/service/systemd.go`、`internal/snapshot/*.go` 中的命令执行
2. **文件路径注入**: 审查 `internal/webdav/server.go`、`internal/files/manager.go` 中的路径处理

### 短期处理 (P1)
3. **权限加固**: 统一审查文件权限设置
4. **整数溢出**: 对关键路径添加边界检查

### 长期改进 (P2)
5. **建立安全编码规范**
6. **集成 gosec 到 CI/CD 流程**
7. **定期安全审计**

---

## 附录

### A. 扫描配置

```bash
gosec -fmt json -out gosec-scan-current.json ./...
```

### B. 相关文件

- 当前扫描结果: `gosec-scan-current.json`
- 上次扫描结果: `gosec-scan-v2.253.146.json`

### C. CWE 参考

| CWE ID | 名称 | 链接 |
|--------|------|------|
| CWE-22 | 路径遍历 | https://cwe.mitre.org/data/definitions/22.html |
| CWE-78 | OS命令注入 | https://cwe.mitre.org/data/definitions/78.html |
| CWE-190 | 整数溢出 | https://cwe.mitre.org/data/definitions/190.html |
| CWE-732 | 关键资源权限配置不当 | https://cwe.mitre.org/data/definitions/732.html |
| CWE-798 | 硬编码凭证 | https://cwe.mitre.org/data/definitions/798.html |

---

**报告生成时间**: 2026-03-22 06:22 CST  
**审计状态**: ✅ 完成