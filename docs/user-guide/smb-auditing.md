# SMB 审计用户指南

**版本**: v2.421.0  
**更新日期**: 2026-04-24

---

## 概述

NAS-OS SMB 审计模块提供全面的 SMB 文件访问审计功能，帮助管理员追踪用户操作、满足合规要求、检测安全威胁。

### 主要功能

- **会话审计**：记录连接/断开、用户认证
- **文件操作审计**：打开/关闭/读/写/删除/重命名
- **权限变更审计**：ACL修改记录
- **审计级别控制**：灵活的记录粒度
- **排除规则**：可排除特定共享、用户、路径

### 适用场景

| 场景 | 说明 |
|------|------|
| 安全审计 | 追踪可疑访问行为 |
| 合规要求 | GDPR、等保2.0、ISO 27001 |
| 数据取证 | 安全事件调查 |
| 用户行为分析 | 了解文件访问模式 |

---

## 快速开始

### 1. 启用 SMB 审计

1. 进入 **控制面板** → **服务** → **SMB** → **审计设置**
2. 选择审计级别（见下表）
3. 配置排除规则（可选）
4. 保存配置

### 2. 查看审计日志

1. 进入 **系统** → **审计日志**
2. 选择分类：`smb` 或 `file_access`
3. 设置时间范围和筛选条件
4. 点击日志条目查看详情

---

## 审计级别说明

| 级别 | 说明 | 记录内容 |
|------|------|----------|
| **None** | 关闭审计 | 不记录任何操作 |
| **Minimal** | 最小审计 | 连接、断开、认证失败 |
| **Standard** | 标准审计 | 连接、文件打开/关闭、删除 |
| **Detailed** | 详细审计 | 包含读写统计、路径详情 |
| **Full** | 完全审计 | 所有操作，包含字节统计 |

### 推荐配置

- **一般场景**：Standard 级别
- **合规场景**：Detailed 或 Full 级别
- **高流量场景**：Standard + 排除规则

---

## 配置步骤

### 步骤一：设置审计级别

#### Web界面配置

1. 进入 **SMB服务** → **审计配置**
2. 选择审计级别
3. 设置日志保留策略
4. 启用数字签名（推荐）

#### API配置

```bash
curl -X PUT https://nas.local/api/v1/services/smb/audit/config \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "level": "detailed",
    "retention_days": 90,
    "enable_signature": true
  }'
```

### 步骤二：配置排除规则

排除规则可减少不必要的日志记录。

#### 排除类型

| 排除类型 | 说明 | 示例 |
|----------|------|------|
| 共享排除 | 排除整个共享 | `public` 共享 |
| 用户排除 | 排除特定用户 | `backup-service` |
| 路径排除 | 排除特定路径 | `/tmp/*` |
| 操作排除 | 排除特定操作 | 只记录删除 |

#### Web界面配置

1. 进入 **SMB服务** → **审计配置** → **排除规则**
2. 点击 **添加规则**
3. 选择排除类型和目标
4. 保存配置

#### API配置

```bash
curl -X POST https://nas.local/api/v1/services/smb/audit/exclusions \
  -H "Authorization: Bearer <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "share",
    "value": "public",
    "reason": "公共共享无需审计"
  }'
```

### 步骤三：设置日志保留策略

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| 最大日志条数 | 保存的最大日志数量 | 1,000,000 |
| 最大保留天数 | 日志保留时间 | 365 天 |
| 启用压缩 | 压缩历史日志 | 开启 |
| 启用数字签名 | 防止日志篡改 | 开启 |

---

## 审计日志内容

### 会话审计日志

| 字段 | 说明 |
|------|------|
| timestamp | 操作时间 |
| event_type | session_connect/session_disconnect |
| user | 用户名 |
| client_ip | 客户端IP地址 |
| share | 共享名称 |
| session_id | SMB会话ID |
| status | 成功/失败 |

### 文件操作日志

| 字段 | 说明 |
|------|------|
| timestamp | 操作时间 |
| event_type | open/close/read/write/delete/rename |
| user | 用户名 |
| client_ip | 客户端IP |
| share | 共享名称 |
| file_path | 文件路径 |
| access_mode | 读/写/读写 |
| bytes_read | 读取字节数（如适用） |
| bytes_written | 写入字节数（如适用） |
| status | 成功/失败/拒绝 |

### 权限变更日志

| 字段 | 说明 |
|------|------|
| timestamp | 操作时间 |
| event_type | acl_change |
| user | 操作用户 |
| file_path | 文件路径 |
| old_permissions | 原权限 |
| new_permissions | 新权限 |

---

## 合规报告

### 支持的合规标准

| 标准 | 说明 | 报告内容 |
|------|------|----------|
| **GDPR** | 欧盟数据保护条例 | 数据访问、导出、删除记录 |
| **等级保护** | 中国网络安全等级保护 | 认证事件、访问控制、安全审计 |
| **ISO 27001** | 信息安全管理体系 | 访问控制、审计日志完整性 |
| **HIPAA** | 美国医疗数据保护 | 医疗数据访问记录 |
| **PCI DSS** | 支付卡数据安全 | 金融数据访问审计 |

### 生成合规报告

1. 进入 **审计日志** → **合规报告**
2. 选择合规标准
3. 设置报告周期
4. 点击 **生成报告**

---

## 日志导出

### 导出格式

| 格式 | 说明 | 适用场景 |
|------|------|----------|
| JSON | 结构化数据 | 程序处理、系统集成 |
| CSV | 表格格式 | Excel分析、报表制作 |
| XML | 标记语言 | 企业系统集成 |

### Web界面导出

1. 设置筛选条件
2. 点击 **导出** 按钮
3. 选择格式和是否包含签名
4. 下载文件

### API导出

```bash
# 导出JSON格式
curl -X GET "https://nas.local/api/v1/audit/export?category=smb&format=json" \
  -H "Authorization: Bearer <token>" \
  -o smb-audit.json

# 导出CSV格式
curl -X GET "https://nas.local/api/v1/audit/export?category=smb&format=csv&start=2026-04-01&end=2026-04-30" \
  -H "Authorization: Bearer <token>" \
  -o smb-audit.csv
```

---

## 最佳实践

### 1. 审计级别选择

| 场景 | 推荐级别 |
|------|----------|
| 一般企业使用 | Standard |
| 金融/医疗行业 | Detailed 或 Full |
| 开发测试环境 | Minimal |
| 高流量生产环境 | Standard + 排除规则 |

### 2. 排除规则配置

建议排除：
- **系统服务账户**：backup-service、sync-service
- **公共共享**：无需审计的公开数据
- **临时路径**：`/tmp/*`、缓存目录

### 3. 日志保留策略

| 场景 | 推荐保留期限 |
|------|--------------|
| 一般合规 | 90-180 天 |
| 金融行业 | 7 年 |
| 医疗行业 | 6 年 |
| 法律要求 | 按具体法规 |

### 4. 安全加固

- **启用数字签名**：防止日志篡改
- **定期备份日志**：异地存储
- **设置告警规则**：异常行为通知
- **定期审查日志**：每周检查安全事件

---

## 故障排查

### 问题一：审计日志不记录

**症状**：配置后无日志生成

**排查步骤**：

1. 检查审计是否启用：
   ```bash
   cat /etc/samba/smb.conf | grep audit
   ```

2. 检查审计级别是否为 None
3. 检查排除规则是否过于宽泛
4. 查看系统日志：`/var/log/nas-os/smb-audit.log`

### 问题二：日志量过大

**症状**：磁盘空间快速消耗

**解决方案**：

1. 降低审计级别
2. 添加排除规则
3. 启用日志压缩
4. 减少保留天数

### 问题三：完整性验证失败

**症状**：数字签名验证失败

**排查**：

1. 检查系统时间是否正确
2. 检查签名密钥是否变更
3. 查看详细错误信息定位问题日志

---

## 常见问题

### Q1: 审计会影响性能吗？

| 审计级别 | 性能影响 |
|----------|----------|
| Minimal | < 1% |
| Standard | 1-3% |
| Detailed | 3-5% |
| Full | 5-10% |

建议高流量场景使用 Standard 级别 + 排除规则。

### Q2: 审计日志可以删除吗？

- **数字签名启用时**：日志不可单独删除（破坏完整性）
- **合规场景**：需按保留策略自动清理
- **取证场景**：日志应备份保存

### Q3: 如何查询特定用户的操作？

Web界面：
1. 进入审计日志
2. 用户筛选框输入用户名
3. 设置时间范围

API查询：
```bash
curl -X GET "https://nas.local/api/v1/audit/logs?category=smb&user=john" \
  -H "Authorization: Bearer <token>"
```

### Q4: 审计日志是否包含文件内容？

**否**。审计只记录操作元数据（路径、用户、时间），不记录文件内容。

---

## API 参考

### 查询 SMB 审计日志

```bash
curl -X GET "https://nas.local/api/v1/audit/logs?category=smb&limit=50" \
  -H "Authorization: Bearer <token>"
```

### 查询特定文件访问记录

```bash
curl -X GET "https://nas.local/api/v1/audit/logs?category=smb&path=/data/finance" \
  -H "Authorization: Bearer <token>"
```

### 查询认证失败事件

```bash
curl -X GET "https://nas.local/api/v1/audit/logs?category=smb&event_type=auth_fail" \
  -H "Authorization: Bearer <token>"
```

### 生成合规报告

```bash
curl -X POST "https://nas.local/api/v1/audit/compliance/report" \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "standard": "gdpr",
    "period": "2026-04-01/2026-04-30"
  }'
```

---

## 相关文档

- [审计日志增强方案](../audit-log-enhancement.md)
- [SMB多通道指南](smb-multichannel.md)
- [审计模块通用指南](audit-guide.md)
- [权限管理指南](permission-guide.md)

---

*文档编制: 礼部文档组*
*最后更新: 2026-04-24*