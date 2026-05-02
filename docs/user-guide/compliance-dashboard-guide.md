# 合规仪表盘指南

> **版本**: v2.477.0+ | **适用版本**: NAS-OS v2.477.0 及以上

## 概述

合规仪表盘对标群晖 Security Advisor 和 TrueNAS STIG Compliance，提供统一的安全合规状态监控。支持 CIS、STIG、GDPR 等多个安全框架的自动化检查，生成综合合规评分和趋势报告。

## 核心特性

- **多框架支持**：CIS Benchmark、STIG、GDPR 安全标准
- **11 项内置检查**：涵盖访问控制、网络安全、存储加密、审计日志等
- **加权评分**：按严重级别加权计算综合合规评分（0-100）
- **自动修复**：可开启自动修复已知问题
- **趋势追踪**：保留 90 天评分趋势数据
- **阈值告警**：低于设定分数自动告警通知

## 合规检查项

### 内置检查（11 项）

| 检查 ID | 检查项 | 类别 | 严重级别 | 框架 |
|---------|--------|------|----------|------|
| acc-001 | SSH Root 登录禁用 | 访问控制 | 严重 | CIS |
| acc-002 | 密码复杂度策略 | 密码策略 | 高 | CIS |
| acc-003 | MFA 启用状态 | 访问控制 | 高 | CIS |
| net-001 | 防火墙启用状态 | 网络安全 | 严重 | CIS |
| net-002 | 不必要的端口检查 | 网络安全 | 中 | CIS |
| sto-001 | 存储加密状态 | 加密 | 高 | STIG |
| sto-002 | 快照保留策略 | 备份保护 | 中 | CIS |
| aud-001 | 审计日志启用 | 审计日志 | 严重 | STIG |
| aud-002 | 日志保留期限 | 审计日志 | 中 | GDPR |
| upd-001 | 系统更新状态 | 系统更新 | 高 | CIS |
| bak-001 | 备份验证状态 | 备份保护 | 高 | CIS |

### 评分权重

| 严重级别 | 权重 | 说明 |
|----------|------|------|
| Critical（严重） | 5 | 必须修复，影响安全基线 |
| High（高） | 4 | 强烈建议修复 |
| Medium（中） | 3 | 建议修复 |
| Low（低） | 2 | 可选修复 |
| Info（信息） | 1 | 仅供参考 |

## API 接口

### 获取合规评分

```bash
curl http://localhost:8080/api/v1/compliance/score
```

响应示例：
```json
{
  "overall": 78,
  "by_category": {
    "access_control": 85,
    "network_security": 90,
    "storage_security": 60,
    "audit_logging": 70,
    "encryption": 65,
    "system_updates": 80,
    "backup_protection": 75,
    "password_policy": 90
  },
  "total_checks": 11,
  "passed_checks": 8,
  "failed_checks": 3,
  "critical_fails": 1,
  "last_updated": "2026-05-02T19:00:00Z"
}
```

### 获取检查结果

```bash
curl http://localhost:8080/api/v1/compliance/results
```

### 手动触发检查

```bash
curl -X POST http://localhost:8080/api/v1/compliance/check
```

### 获取评分趋势

```bash
# 最近 30 天的趋势数据
curl "http://localhost:8080/api/v1/compliance/trends?days=30"
```

### 获取报告摘要

```bash
curl http://localhost:8080/api/v1/compliance/report
```

## 默认配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 检查间隔 | 1 小时 | 自动检查频率 |
| 评分阈值 | 70 | 低于此分数触发告警 |
| 自动修复 | 关闭 | 是否自动修复可修复的问题 |
| 趋势保留 | 90 天 | 评分趋势数据保留天数 |
| 低分告警 | 开启 | 评分低于阈值时通知 |

## 检查类别

| 类别 | 说明 | 关联检查 |
|------|------|----------|
| 访问控制 | 用户认证和授权 | acc-001, acc-003 |
| 密码策略 | 密码复杂度和策略 | acc-002 |
| 网络安全 | 防火墙和端口管理 | net-001, net-002 |
| 存储安全/加密 | 数据加密状态 | sto-001 |
| 审计日志 | 日志记录和保留 | aud-001, aud-002 |
| 系统更新 | 安全补丁状态 | upd-001 |
| 备份保护 | 备份完整性 | sto-002, bak-001 |

## 最佳实践

1. **定期检查**：保持默认 1 小时检查间隔
2. **关注严重项**：Critical 级别问题应优先修复
3. **评分趋势**：持续关注评分走势，避免安全基线下降
4. **自定义检查**：根据企业安全策略注册自定义检查项
5. **合规报告**：定期导出报告用于审计和合规审查
