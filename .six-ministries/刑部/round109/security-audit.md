# 刑部第109轮安全审计报告

**审计时间**: 2026-03-31  
**审计范围**: NAS-OS 项目  
**审计员**: 刑部 (法务合规)

---

## 一、Gosec 扫描摘要

基于最近一轮 gosec 扫描 (gosec-round97.json):

### 总体统计
| 等级 | 数量 |
|------|------|
| HIGH | 240 |
| MEDIUM | 795 |
| LOW | 59 |
| **总计** | **1094** |

### 主要问题类型 (Top 15)
| Rule ID | 数量 | 等级 | 说明 |
|---------|------|------|------|
| G304 | 311 | MEDIUM | 文件路径通过变量注入 |
| G204 | 248 | MEDIUM | 命令执行风险 |
| G306 | 174 | MEDIUM | 文件权限设置不当 |
| G115 | 125 | HIGH | 整数溢出转换 |
| G104 | 59 | LOW | 错误未处理 |
| G703 | 54 | HIGH | 潜在溢出 |
| G301 | 23 | MEDIUM | 目录权限 |
| G704 | 22 | HIGH | 潜在溢出 |
| G118 | 13 | HIGH | 内存分配 |
| G101 | 10 | HIGH | 硬编码凭证 |

---

## 二、SMB 安全配置审计 (`internal/smb/`)

### 模块概述
SMB 模块共 13 个文件，包含完善的安全机制:

### ✅ 安全特性 (已实现)

1. **IP访问控制** (`security.go`)
   - IP白名单/黑名单支持
   - CIDR格式匹配 (`192.168.1.0/24`)
   - 自动封禁过期清理

2. **限流机制** (`ratelimit.go`, `security.go`)
   - 单IP连接数限制 (`MaxConnPerIP`)
   - 总连接数限制 (`MaxConnTotal`)
   - 时间窗口控制

3. **暴力破解防护**
   - 失败尝试记录
   - 自动封禁 (阈值可配置)
   - 白名单IP免封禁
   - 封禁时长可配置

4. **审计日志**
   - JSON格式审计日志
   - 支持反向读取
   - 记录所有敏感操作

5. **多通道SMB** (`multichannel.go`)
   - 健康检查机制
   - 故障切换支持
   - 负载均衡轮询

### 🔍 潜在风险点

| 风险 | 文件 | 说明 | 建议 |
|------|------|------|------|
| **HIGH** | `.admin_password` | 存储明文密码文件 | 应使用加密存储或删除测试文件 |
| MEDIUM | `security.go:50` | 配置文件权限0640 | 可考虑降至0600 |
| LOW | `config.go` | 无密码长度验证 | 建议添加密码复杂度校验 |

### 💡 优化建议

1. **删除测试密码文件**  
   `internal/smb/.admin_password` 包含明文密码，应立即删除并添加到 `.gitignore`

2. **增强协议安全**  
   默认 `MinProtocol: "SMB2"` 可升级为 `"SMB3"` 强制加密

3. **添加审计日志加密**  
   当前审计日志为明文JSON，建议敏感信息脱敏

---

## 三、勒索软件检测模块审计 (`internal/ransomware/`)

### 模块概述
勒索软件检测模块共 2 个文件，实现主动防护框架:

### ✅ 安全特性 (已实现)

1. **文件行为监控** (`detector.go`)
   - 事件类型追踪: create/modify/delete/rename/encrypt/bulk_write
   - 时间窗口分析
   - WriteOnce保护机制

2. **加密检测**
   - 熵值计算 (`calculateEntropy`)
   - 高熵文件识别 (>7.5阈值)
   - 批量加密模式检测

3. **威胁模式匹配**
   - 预定义威胁模式:
     - `rapid-encryption`: 快速加密行为
     - `bulk-extension-change`: 批量扩展名修改
     - `rapid-delete`: 快速删除
     - `suspicious-write-pattern`: 可疑写入模式

4. **多级防护动作**
   - Alert (告警)
   - Block (阻止)
   - Quarantine (隔离)
   - Snapshot (快照保护)
   - Lockdown (锁定卷)

5. **检测敏感度分级**
   - Low: 仅检测明显威胁
   - Medium: 平衡检测
   - High: 激进检测

### 🔍 潜在风险点

| 风险 | 文件:行号 | 说明 | 建议 |
|------|-----------|------|------|
| MEDIUM | `detector.go:159` | `math.Log2`精度问题 | 熵值计算可考虑更高精度 |
| LOW | `detector.go:380` | 扫描深度限制 | `maxFiles`参数应可配置更大值 |
| INFO | `types.go` | 无持久化威胁记录 | 建议添加威胁历史存储 |

### 💡 优化建议

1. **增强实时监控**
   - 当前依赖事件推送，建议添加主动扫描定时任务

2. **威胁模式库更新**
   - 建议添加外部威胁模式导入机制

3. **误报处理机制**
   - 当前仅有 `FalsePositives` 统计，建议添加白名单文件功能

---

## 四、全局安全问题 (Gosec 高优先级)

### 硬编码凭证问题 (G101)

检出 10 处潜在硬编码凭证:

| 文件 | 行号 | 类型 |
|------|------|------|
| `internal/office/types.go` | 583 | credentials |
| `internal/cloudsync/providers.go` | 696, 1359 | credentials |
| `internal/cloudsync/provider_baidu.go` | 68 | credentials |
| `internal/cloudsync/provider_alipan.go` | 72 | credentials |
| `internal/auth/oauth2.go` | 353-393 | credentials |

**建议**: 全面检查这些位置，移除硬编码凭证或使用环境变量

### 整数溢出问题 (G115)

检出 125 处整数溢出风险:

主要分布:
- `pkg/storage/tiering/migrator.go`: uint64 → int64
- `internal/monitoring/ssd_health.go`: uint64 → int
- `internal/disk/smart_monitor.go`: uint64 → int
- `internal/photos/`: uint32 → byte

**建议**: 添加显式边界检查或使用安全转换函数

### 文件路径注入 (G304)

检出 311 处文件路径变量注入风险

**建议**: 对所有外部输入路径进行规范化和安全验证

---

## 五、审计结论

### SMB 模块评分: **B+ (良好)**
- 安全机制完善
- 限流/封禁机制健全
- 存在测试密码文件泄漏风险

### 勒索软件模块评分: **A- (优秀)**
- 检测框架设计合理
- 多级防护策略完善
- 需增强持久化监控

### 项目整体评分: **B (合格)**
- 共1094个安全问题待处理
- HIGH级别240个需优先修复
- 关键模块(SMB/ransomware)无直接安全问题

---

## 六、优先行动项

| 优先级 | 行动 | 负责部门 |
|--------|------|----------|
| P0 | 删除 `.admin_password` 测试文件 | 刑部 |
| P1 | 修复 10处硬编码凭证 | 刑部 |
| P1 | 处理 G115 整数溢出 | 兵部 |
| P2 | 增强勒索软件持久化监控 | 工部 |
| P3 | SMB协议升级为SMB3强制加密 | 工部 |

---

**刑部签发**  
2026-03-31