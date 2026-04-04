# SMB Security Audit Documentation

## 刑部第160轮：SMB安全审计增强

> 本文档遵循安全审计最佳实践，参考 TrueNAS Scale 24.04 和 ISO 27001 标准

---

## 1. 概述

### 1.1 审计目标

SMB安全审计系统用于：
- 记录所有SMB文件访问操作
- 追踪用户行为和访问模式
- 检测异常访问和潜在安全威胁
- 支持合规审计和事后调查
- 提供安全事件溯源能力

### 1.2 审计级别

| 级别 | 名称 | 记录内容 | 适用场景 |
|------|------|----------|----------|
| none | 无审计 | 不记录任何日志 | 测试环境 |
| minimal | 最小审计 | 连接/断开事件 | 低安全需求 |
| standard | 标准审计 | 会话+文件操作摘要 | 一般生产环境 |
| detailed | 详细审计 | 所有操作详情+权限变更 | 中等安全需求 |
| full | 完整审计 | 所有操作+内容摘要 | 高安全需求/合规场景 |

---

## 2. 审计日志字段规范

### 2.1 核心字段（必记录）

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| event_id | string | 事件唯一标识 | smb-1703275200-abc12345 |
| timestamp | datetime | 事件发生时间（UTC） | 2026-04-04T17:00:00Z |
| session_id | string | SMB会话标识 | sess-12345 |
| client_ip | string | 客户端IP地址 | 192.168.1.100 |
| username | string | 访问用户名 | admin |
| operation | string | 操作类型 | read/write/delete |
| file_path | string | 操作文件路径 | /share/docs/report.pdf |
| status | string | 操作结果 | success/failure/denied |

### 2.2 扩展字段（标准级别以上）

| 字段 | 类型 | 说明 | 记录级别 |
|------|------|------|----------|
| share_name | string | SMB共享名称 | standard+ |
| domain | string | 用户域 | standard+ |
| client_port | int | 客户端端口 | standard+ |
| computer_name | string | 客户端计算机名 | standard+ |
| bytes_read | int64 | 读取字节量 | standard+ |
| bytes_written | int64 | 写入字节量 | standard+ |
| is_directory | bool | 是否目录操作 | standard+ |
| protocol_version | string | SMB协议版本 | standard+ |
| encryption | string | 加密状态 | standard+ |

### 2.3 安全增强字段（详细级别以上）

| 字段 | 类型 | 说明 | 安全用途 |
|------|------|------|----------|
| old_path | string | 重命名/移动原路径 | 跟踪文件变更轨迹 |
| new_path | string | 重命名/移动新路径 | 跟踪文件变更轨迹 |
| old_perms | string | 变更前权限 | 权限变更审计 |
| new_perms | string | 变更后权限 | 权限变更审计 |
| old_owner | string | 变更前所有者 | 所有者变更审计 |
| new_owner | string | 变更后所有者 | 所有者变更审计 |
| content_digest | string | 内容SHA256摘要(前16字节) | 文件完整性验证 |
| lock_type | string | 锁类型（共享/排他） | 锁竞争分析 |
| lock_range | string | 锁定范围 | 锁竞争分析 |
| duration | int64 | 操作耗时(ms) | 性能异常检测 |

### 2.4 失败/拒绝字段

| 字段 | 类型 | 说明 | 用途 |
|------|------|------|------|
| error_code | int | 错误码 | 故障诊断 |
| error_message | string | 错误描述 | 故障诊断 |
| status | string | failure/denied | 安全事件识别 |

---

## 3. 审计日志轮转策略

### 3.1 轮转触发条件

```go
type RotationPolicy struct {
    // 大小触发：单文件超过指定大小时轮转
    MaxSizeMB         int    // 默认: 100MB
    
    // 时间触发：定期轮转
    RotateInterval    time.Duration  // 默认: 24小时
    
    // 条目触发：单文件超过指定条目数时轮转
    MaxEntriesPerFile int    // 默认: 100000
    
    // 保留策略：旧日志保留天数
    MaxAgeDays        int    // 默认: 90天
    
    // 压缩策略：是否压缩旧日志
    CompressOldLogs   bool   // 默认: true
    
    // 压缩阈值：超过此天数的日志压缩
    CompressAfterDays int    // 默认: 7天
}
```

### 3.2 轮转执行流程

```
1. 检查触发条件（大小/时间/条目）
2. 关闭当前日志文件
3. 重命名当前文件：smb-audit-2026-04-04.log → smb-audit-2026-04-04-001.log
4. 创建新日志文件
5. 更新文件指针和大小计数器
6. 后台压缩旧日志（可选）
```

### 3.3 日志命名规范

```
/var/log/nas-os/audit/smb/
├── smb-audit-2026-04-04.log          # 当前日志
├── smb-audit-2026-04-03-001.log.gz   # 昨天日志（压缩）
├── smb-audit-2026-04-02-001.log.gz   # 前天日志（压缩）
├── smb-audit-2026-04-01-001.log      # 未压缩旧日志
└── ...
```

### 3.4 清理策略

- **保留期**: 默认保留90天
- **压缩**: 7天以上日志自动gzip压缩
- **删除**: 超过保留期日志自动删除
- **手动清理**: API支持手动触发清理

---

## 4. 审计日志查询API

### 4.1 查询接口

**GET /api/audit/smb/events**

查询参数：
| 参数 | 类型 | 说明 |
|------|------|------|
| limit | int | 返回条数限制（默认100，最大1000） |
| offset | int | 分页偏移 |
| start_time | datetime | 开始时间（RFC3339） |
| end_time | datetime | 结束时间（RFC3339） |
| session_id | string | 会话ID过滤 |
| share_name | string | 共享名过滤 |
| username | string | 用户名过滤 |
| client_ip | string | 客户端IP过滤 |
| operation | string | 操作类型过滤 |
| file_path | string | 文件路径过滤（精确匹配） |
| file_path_pattern | string | 文件路径模式匹配（新增） |
| status | string | 状态过滤 |

响应示例：
```json
{
  "events": [
    {
      "event_id": "smb-1703275200-abc12345",
      "timestamp": "2026-04-04T17:00:00Z",
      "session_id": "sess-12345",
      "client_ip": "192.168.1.100",
      "username": "admin",
      "share_name": "documents",
      "operation": "write",
      "file_path": "/share/docs/report.pdf",
      "status": "success",
      "bytes_written": 102400
    }
  ],
  "total": 1500,
  "limit": 100,
  "offset": 0
}
```

### 4.2 统计接口

**GET /api/audit/smb/statistics**

响应示例：
```json
{
  "total_events": 15000,
  "events_by_type": {
    "read": 8000,
    "write": 4000,
    "delete": 1000,
    "connect": 500
  },
  "events_by_share": {
    "documents": 8000,
    "media": 5000,
    "backup": 2000
  },
  "events_by_user": {
    "admin": 5000,
    "user1": 3000,
    "user2": 2000
  },
  "events_by_client": {
    "192.168.1.100": 5000,
    "192.168.1.101": 3000
  },
  "bytes_read": 524288000,
  "bytes_written": 104857600,
  "failed_operations": 50,
  "denied_operations": 20,
  "hourly_distribution": {
    "0": 500,
    "1": 200,
    ...
    "23": 800
  }
}
```

### 4.3 聚合分析接口（新增）

**GET /api/audit/smb/analyze**

查询参数：
| 参数 | 类型 | 说明 |
|------|------|------|
| group_by | string | 聚合维度（user/client/share/hour/operation） |
| start_time | datetime | 分析时间范围开始 |
| end_time | datetime | 分析时间范围结束 |
| top_n | int | Top N数量（默认10） |

响应示例：
```json
{
  "group_by": "user",
  "results": [
    {"key": "admin", "count": 5000, "bytes_read": 104857600, "bytes_written": 52428800},
    {"key": "user1", "count": 3000, "bytes_read": 52428800, "bytes_written": 10485760}
  ],
  "time_range": {
    "start": "2026-04-01T00:00:00Z",
    "end": "2026-04-04T00:00:00Z"
  }
}
```

### 4.4 异常检测接口（新增）

**GET /api/audit/smb/anomalies**

检测规则：
- 短时间大量删除操作
- 异常时间访问（非工作时间）
- 大量失败/拒绝操作
- 单用户异常高访问量
- 稀有文件访问模式

响应示例：
```json
{
  "anomalies": [
    {
      "type": "mass_delete",
      "severity": "high",
      "description": "用户user1在10分钟内删除50个文件",
      "details": {
        "username": "user1",
        "delete_count": 50,
        "time_window": "10m"
      },
      "timestamp": "2026-04-04T17:30:00Z"
    }
  ],
  "total_anomalies": 1
}
```

### 4.5 导出接口

**GET /api/audit/smb/export**

支持格式：json、csv

---

## 5. 安全审计报告模板

### 5.1 日报模板

```markdown
# SMB安全审计日报

**日期**: {date}
**审计级别**: {audit_level}

## 访问统计

| 指标 | 数值 |
|------|------|
| 总事件数 | {total_events} |
| 活跃用户数 | {active_users} |
| 活跃客户端数 | {active_clients} |
| 读取流量 | {bytes_read_mb}MB |
| 写入流量 | {bytes_written_mb}MB |

## 异常事件

| 类型 | 数量 | 详情 |
|------|------|------|
| 失败操作 | {failed_ops} | {top_failures} |
| 拒绝操作 | {denied_ops} | {top_denials} |

## Top 10 访问用户

{top_users_table}

## Top 10 访问文件

{top_files_table}

## 建议

{recommendations}
```

### 5.2 周报模板

```markdown
# SMB安全审计周报

**周期**: {start_date} - {end_date}
**审计级别**: {audit_level}

## 概述

本周SMB服务共记录 {total_events} 条审计事件，
活跃用户 {active_users} 人，活跃客户端 {active_clients} 个。

## 趋势分析

### 日事件量趋势

{daily_trend_chart}

### 操作类型分布

{operation_distribution_chart}

### 流量趋势

{traffic_trend_chart}

## 安全事件

### 高风险事件

{high_risk_events}

### 异常访问模式

{anomaly_patterns}

### 权限变更记录

{permission_changes}

## 合规检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 审计日志完整性 | ✅ | 无缺失 |
| 备份覆盖 | ✅ | 已备份 |
| 异常检测 | ✅ | 无重大异常 |

## 建议

{weekly_recommendations}
```

### 5.3 事件溯源报告模板

```markdown
# SMB安全事件溯源报告

**事件ID**: {event_id}
**生成时间**: {report_time}

## 事件概要

| 字段 | 值 |
|------|------|
| 事件类型 | {operation} |
| 发生时间 | {timestamp} |
| 操作用户 | {username} |
| 客户端IP | {client_ip} |
| 目标文件 | {file_path} |
| 操作结果 | {status} |

## 会话轨迹

### 连接信息

| 字段 | 值 |
|------|------|
| 会话ID | {session_id} |
| 连接时间 | {connect_time} |
| 协议版本 | {protocol_version} |
| 加密状态 | {encryption} |

### 本次会话所有操作

{session_operations}

## 用户历史行为

### 最近7天活动

{user_recent_activity}

### 访问模式分析

{user_pattern_analysis}

## 文件访问历史

{file_access_history}

## 关联事件

{related_events}

## 风险评估

| 因素 | 评估 | 说明 |
|------|------|------|
| 时间异常 | {time_risk} | {time_risk_desc} |
| 行为异常 | {behavior_risk} | {behavior_risk_desc} |
| 权限异常 | {perm_risk} | {perm_risk_desc} |

**综合风险等级**: {overall_risk_level}

## 处置建议

{recommendations}
```

---

## 6. 安全最佳实践

### 6.1 审计配置建议

| 环境 | 建议级别 | 说明 |
|------|----------|------|
| 生产环境 | standard/detailed | 平衡性能与安全 |
| 金融/医疗 | full | 满足合规要求 |
| 测试环境 | minimal/none | 减少日志量 |

### 6.2 排除配置建议

- 接收排除：IPC$、print$（系统共享）
- 用户排除：谨慎使用，避免安全盲区
- 路径排除：仅排除非敏感路径

### 6.3 日志保护

- 日志文件权限：600（仅root可读写）
- 日志目录权限：750
- 定期备份到安全存储
- 启用日志签名（可选）

### 6.4 异常告警

建议配置告警规则：
- 大量删除操作（>50/10min）
- 大量拒绝操作（>10/1min）
- 非工作时间访问
- 新用户首次访问敏感共享
- 权限批量变更

---

## 7. 合规映射

### 7.1 ISO 27001 映射

| 控制项 | 审计支持 |
|--------|----------|
| A.12.4 事件日志 | ✅ 完整支持 |
| A.12.4.1 事件日志保护 | ✅ 文件权限控制 |
| A.12.4.2 管理员日志 | ✅ 特权操作记录 |
| A.12.4.3 日志信息保护 | ✅ 访问控制 |

### 7.2 GDPR 映射

| 要求 | 审计支持 |
|------|----------|
| 数据访问记录 | ✅ 文件访问审计 |
| 数据处理记录 | ✅ 操作类型记录 |
| 数据主体请求追踪 | ✅ 查询API支持 |

---

## 8. API完整参考

### 配置管理

```
GET  /api/audit/smb/config      # 获取配置
PUT  /api/audit/smb/config      # 更新配置
GET  /api/audit/smb/levels      # 获取级别列表
```

### 事件查询

```
GET  /api/audit/smb/events      # 查询事件
GET  /api/audit/smb/statistics  # 获取统计
GET  /api/audit/smb/analyze     # 聚合分析（新增）
GET  /api/audit/smb/anomalies   # 异常检测（新增）
GET  /api/audit/smb/export      # 导出日志
```

### 报告生成（新增）

```
POST /api/audit/smb/report/daily     # 生成日报
POST /api/audit/smb/report/weekly    # 生成周报
POST /api/audit/smb/report/trace     # 生成事件溯源报告
```

---

## 9. 版本记录

| 版本 | 日期 | 变更 |
|------|------|------|
| v1.0 | 2026-04-04 | 初始版本（刑部第160轮） |

---

*本文档由刑部编制，遵循安全审计最佳实践*