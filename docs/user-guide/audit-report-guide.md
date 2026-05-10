# 审计报告 (Audit Report) 用户指南

> **适用版本**: v2.485.0+ | **模块路径**: `internal/auditreport/`

## 概述

审计报告模块提供全面的安全审计能力，包括审计报告生成、安全发现管理、合规检查、审计事件日志记录和安全扫描，帮助企业满足 CIS/STIG/GDPR 等合规要求。

## 功能特性

- **审计报告生成**: 自动汇总未解决发现，计算评分，生成摘要
- **发现管理**: 安全发现的完整生命周期管理(添加/更新/解决)
- **合规检查**: 支持 CIS/STIG/GDPR 标准的逐项检查
- **审计事件日志**: 用户操作/资源访问/IP/结果的完整记录
- **安全扫描**: 漏洞检测/端口扫描/配置检查
- **事件导出**: 按时间/用户/操作类型筛选导出

## API 端点

### 报告管理

```
POST /api/v1/audit/reports/generate    # 生成审计报告
GET  /api/v1/audit/reports             # 列出所有报告
GET  /api/v1/audit/reports/{id}        # 获取单个报告
DELETE /api/v1/audit/reports/{id}      # 删除报告
```

生成报告请求示例:
```json
{
  "title": "2026年5月安全审计报告",
  "period": "2026-05"
}
```

### 发现管理

```
POST   /api/v1/audit/findings          # 添加发现
PUT    /api/v1/audit/findings/{id}     # 更新发现
POST   /api/v1/audit/findings/{id}/resolve  # 解决发现
GET    /api/v1/audit/findings          # 列出所有发现
```

发现严重级别:
| 级别 | 说明 | 响应时间要求 |
|------|------|-------------|
| **Critical** | 严重安全漏洞 | 立即处理 |
| **High** | 高风险问题 | 24小时内 |
| **Medium** | 中等风险 | 7天内 |
| **Low** | 低风险 | 30天内 |
| **Info** | 信息提示 | 按计划处理 |

### 合规检查

```
POST /api/v1/audit/compliance/check    # 执行合规检查
GET  /api/v1/audit/compliance/status   # 获取合规状态
GET  /api/v1/audit/compliance          # 列出所有合规检查
```

执行合规检查请求:
```json
{
  "standard": "CIS"
}
```

支持的合规标准:
- **CIS**: Center for Internet Security 基准
- **STIG**: Security Technical Implementation Guide
- **GDPR**: 通用数据保护条例

### 审计事件

```
GET  /api/v1/audit/events              # 查询审计事件
GET  /api/v1/audit/events/export       # 导出事件日志
```

查询参数:
- `start_time` / `end_time`: 时间范围
- `user_id`: 用户筛选
- `action`: 操作类型筛选
- `limit` / `offset`: 分页

### 安全扫描

```
POST /api/v1/audit/scan                # 执行安全扫描
GET  /api/v1/audit/scan/{id}           # 获取扫描结果
```

扫描类型:
- `vulnerability`: 漏洞检测
- `port`: 端口扫描
- `config`: 配置检查

## 审计报告结构

```json
{
  "id": "report-001",
  "title": "2026年5月安全审计报告",
  "period": "2026-05",
  "generated_at": "2026-05-06T20:00:00Z",
  "score": 82.5,
  "findings": [...],
  "summary": "共发现3个问题，其中1个高风险，2个中等风险"
}
```

## 使用场景

- **合规审计**: 定期生成合规报告，满足监管要求
- **安全评估**: 全面评估系统安全状态，识别风险点
- **事件追踪**: 记录和查询所有安全相关操作
- **漏洞管理**: 跟踪安全发现的完整生命周期
- **第三方审计**: 导出审计日志供外部审计使用
