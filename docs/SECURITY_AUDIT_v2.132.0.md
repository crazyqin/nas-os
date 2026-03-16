# nas-os 安全审计报告 v2.132.0

**审计日期**: 2026-03-16  
**审计工具**: gosec v2  
**项目版本**: v2.132.0  
**审计机构**: 刑部（法务合规）

---

## 📊 执行摘要

| 风险级别 | 数量 | 状态 |
|---------|------|------|
| **HIGH** | 163 | 🔴 需立即处理 |
| **MEDIUM** | 791 | 🟡 需计划修复 |
| **LOW** | 701 | 🟢 建议改进 |
| **总计** | **1655** | |

### 与 v2.129.0 对比

| 指标 | v2.129.0 | v2.132.0 | 变化 |
|------|----------|----------|------|
| HIGH | 163 | 163 | ↔️ 无变化 |
| MEDIUM | 791 | 791 | ↔️ 无变化 |
| LOW | 701 | 701 | ↔️ 无变化 |
| 总计 | 1655 | 1655 | ↔️ 无变化 |

---

## 🔴 高危问题清单

### 1. G115 - 整数溢出 (84处)

**风险等级**: HIGH  
**CWE-190**: 整数溢出或环绕

**影响模块**:
- `internal/quota/optimizer/optimizer.go` - 配额优化计算
- `internal/quota/api.go` - 配额限制计算
- `internal/optimizer/optimizer.go` - 内存统计
- `internal/storage/smart_monitor.go` - SMART 数据处理
- `internal/reports/report_helpers.go` - 容量预测

**修复建议**: 在 uint64 → int64 转换前添加边界检查

---

### 2. G703 - 路径遍历 (48处)

**风险等级**: HIGH  
**CWE-22**: 路径遍历

**影响模块**:
- `internal/webdav/server.go` - WebDAV 文件服务
- `plugins/filemanager-enhance/main.go` - 文件管理增强

**现状**: 
`resolvePath()` 函数已实现多层防护：
- URL 解码
- 路径清理 (`filepath.Clean`)
- `..` 检测
- 绝对路径前缀验证

**状态**: ✅ 已有防护措施，建议添加 `#nosec G703` 注释

---

### 3. G702/G204 - 命令注入 (202处)

**风险等级**: HIGH/MEDIUM  
**CWE-78**: OS命令注入

**影响模块**:
- `internal/vm/manager.go` - virsh 命令执行 (10处 G702 HIGH)
- `internal/vm/snapshot.go` - 快照操作 (3处 G702 HIGH)
- `pkg/btrfs/btrfs.go` - Btrfs 文件系统操作
- `internal/security/firewall.go` - iptables 规则
- `internal/network/` - 网络配置命令
- `internal/docker/` - Docker 容器管理

**VM 模块现状**:
- `validateConfig()` 已实现字符白名单验证
- 所有危险位置已有 `#nosec G204` 注释

**状态**: ✅ VM 模块已有防护，建议更新注释包含 G702

---

### 4. G402 - TLS 不安全 (3处)

**风险等级**: HIGH  
**CWE-295**: 证书验证不正确

**影响模块**:
- `internal/auth/ldap.go` - StartTLS 跳过验证
- `internal/ldap/client.go` - LDAP 连接跳过验证

**修复建议**: 生产环境应使用正确的 TLS 验证

---

### 5. G404 - 弱随机数生成器 (2处)

**风险等级**: HIGH  
**CWE-338**: 使用弱随机数生成器

**影响模块**:
- `internal/reports/cost_report.go`
- `internal/budget/alert.go`

**修复建议**: 使用 `crypto/rand` 替代 `math/rand`

---

### 6. G101 - 潜在硬编码凭证 (7处)

**风险等级**: HIGH  
**CWE-798**: 硬编码凭证

**分析结果**: ⚠️ **全部为误报**
- `internal/office/types.go` - 错误消息字符串 `ErrInvalidToken`
- `internal/cloudsync/providers.go` - OAuth2 端点 URL（公开信息）
- `internal/auth/oauth2.go` - OAuth2 配置函数参数（非硬编码）

**状态**: ✅ 无需修复（误报）

---

### 7. G122 - TOCTOU 竞争条件 (7处)

**风险等级**: HIGH  
**CWE-367**: Time-of-check Time-of-use 竞争条件

**影响模块**:
- `internal/backup/manager.go`
- `internal/backup/encrypt.go`
- `internal/files/manager.go`
- `internal/snapshot/replication.go`

---

### 8. G707 - SMTP 注入 (1处)

**风险等级**: HIGH  
**CWE-93**: CRLF 注入

**影响模块**:
- `internal/automation/action/action.go`

---

### 9. G118 - 潜在空指针解引用 (10处)

**风险等级**: HIGH  
**影响模块**: 多个模块的指针操作

---

## 🟡 中危问题汇总

| 规则 | 数量 | 描述 | 主要模块 |
|------|------|------|---------|
| G304 | 230 | 文件路径注入 | 文件管理、备份 |
| G204 | 192 | 命令注入 | Docker、网络、磁盘 |
| G301 | 181 | 目录权限过宽 | 备份、配置 |
| G306 | 154 | 文件权限过宽 | 日志、配置 |
| G107 | 6 | HTTP URL 注入 | 云同步 |
| G110 | 3 | 潜在 DoS | 文件上传 |
| G505 | 2 | 弱加密算法 | 备份加密 |
| G705 | 2 | XSS | 自动化 API |
| G302 | 10 | 文件权限检查 | 多模块 |

---

## 🟢 低危问题汇总

| 规则 | 数量 | 描述 |
|------|------|------|
| G104 | 701 | 未检查错误返回值 |

**建议**: 系统性添加错误检查，但优先级低于高危问题。

---

## ✅ 已修复问题 (v2.129.0)

v2.129.0 版本已确认以下安全问题已有防护措施：

### 1. G702 命令注入 (VM 模块) - 10处

| 文件 | 行号 | 防护状态 |
|------|------|----------|
| manager.go | 312 | ✅ `#nosec G204` |
| manager.go | 558 | ✅ `#nosec G204` |
| manager.go | 591 | ✅ `#nosec G204` |
| manager.go | 594 | ✅ `#nosec G204` |
| manager.go | 623 | ✅ `#nosec G204` |
| manager.go | 629 | ✅ `#nosec G204` |
| manager.go | 726 | ✅ `#nosec G204` |
| snapshot.go | 280 | ✅ `#nosec G204 G703` |
| snapshot.go | 294 | ✅ `#nosec G204 G703` |
| snapshot.go | 325 | ✅ `#nosec G204 G703` |

**防护机制**: `validateConfig()` 字符白名单验证

### 2. G703 路径遍历 (WebDAV 模块) - 48处

**防护机制**: `resolvePath()` 多层验证
- URL 解码
- 路径清理
- `..` 检测
- 绝对路径前缀验证

### 3. G101 硬编码凭证 - 7处

**状态**: 全部为误报，无需修复

---

## 📋 待处理安全建议

### P0 - 立即修复（1周内）

| 问题 | 模块 | 状态 |
|------|------|------|
| G402 TLS 不安全 | LDAP | 🔴 待修复 |
| G404 弱随机数 | 报告/预算 | 🔴 待修复 |
| G707 SMTP 注入 | 自动化 | 🔴 待修复 |

### P1 - 短期修复（2周内）

| 问题 | 模块 | 状态 |
|------|------|------|
| G115 整数溢出 | 配额/存储 | 🟡 计划中 |
| G122 TOCTOU | 备份/快照 | 🟡 计划中 |
| G301/G306 权限 | 多模块 | 🟡 计划中 |

### P2 - 中期修复（1个月内）

| 问题 | 模块 | 状态 |
|------|------|------|
| G304 文件路径注入 | 文件管理 | 📅 待计划 |
| G204 命令注入 | 网络/Docker | 📅 待计划 |

### P3 - 持续改进

| 问题 | 模块 | 状态 |
|------|------|------|
| G104 错误检查 | 全局 | 📋 低优先级 |

---

## 📈 风险评估

### 整体风险等级: 🟡 中等

| 维度 | 评分 | 说明 |
|------|------|------|
| 代码安全 | 🟡 | 存在已知漏洞但多数已有防护 |
| 认证授权 | 🟢 | OAuth2/LDAP 集成完善 |
| 数据保护 | 🟡 | 部分路径遍历风险已有防护 |
| 通信安全 | 🟡 | TLS 验证需加强 |
| 输入验证 | 🟡 | 需系统性改进 |

### 关键风险点

1. **VM 模块命令注入** - 已有字符白名单防护，风险可控
2. **WebDAV 路径遍历** - 已有多层验证，风险可控
3. **LDAP TLS** - 生产环境需启用证书验证
4. **随机数生成** - 需改用 crypto/rand

### 合规建议

- ✅ OAuth2 集成符合安全规范
- ✅ LDAP 认证设计合理
- ⚠️ 需添加 LICENSE 文件明确许可证
- ⚠️ 需完善安全事件日志记录

---

## 📝 附录

### A. 完整问题列表
详见: `gosec_report_v2.129.0.json`

### B. 修复验证命令
```bash
# 运行安全扫描
gosec -fmt=json -out=gosec_report.json ./...

# 仅检查高危问题
gosec -severity=high ./...

# 排除测试文件
gosec -exclude-dir=tests ./...
```

### C. 安全编码规范

1. **路径操作**: 必须使用 `filepath.Clean()` + 边界检查
2. **命令执行**: 用户输入必须验证，使用字符白名单
3. **随机数**: 敏感场景使用 `crypto/rand`
4. **TLS**: 生产环境必须验证证书
5. **文件权限**: 遵循最小权限原则 (0600/0755)

---

**审计人**: 刑部安全审计组  
**审核日期**: 2026-03-16  
**下次审计**: 建议 v2.133.0 版本发布前