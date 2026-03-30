# WriteOnce 不可变存储使用指南

> **版本**: v2.255.0
> **最后更新**: 2026-03-24

## 功能概述

WriteOnce（一次写入，多次读取）是 NAS-OS 提供的不可变存储功能，基于 btrfs 只读快照实现。该功能可确保数据在锁定期间无法被修改、删除或加密，是防止勒索病毒攻击和保护关键数据的理想解决方案。

### 核心特性

- **不可变快照**：基于 btrfs 只读快照，确保数据完整性
- **灵活锁定时长**：支持 7 天、30 天、永久三种锁定时长
- **防勒索保护**：可选的 `chattr +i` 属性，双重保护
- **完整生命周期管理**：锁定、解锁、延长、恢复
- **批量操作**：支持批量锁定多个目录
- **审计追踪**：完整的操作记录和状态查询

## 使用场景

### 1. 防勒索病毒保护

勒索病毒会加密用户数据并索要赎金。WriteOnce 通过创建只读快照，确保即使系统被勒索病毒感染，关键数据仍有不可篡改的备份。

**推荐配置**：
- 锁定时长：30 天或永久
- 启用防勒索保护：`protectFromRansomware: true`

```bash
# 锁定重要文档目录
curl -X POST http://localhost:8080/api/v1/immutable \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/important-docs",
    "duration": "30d",
    "description": "重要文档防勒索保护",
    "protectFromRansomware": true
  }'
```

### 2. 合规归档

许多行业法规要求保留关键数据不可篡改（如金融、医疗、法律）。WriteOnce 提供 WORM（Write Once Read Many）存储能力，满足合规要求。

**适用场景**：
- 金融交易记录归档
- 医疗病历长期保存
- 法律文档存证
- 审计日志归档

```bash
# 永久锁定合规数据
curl -X POST http://localhost:8080/api/v1/immutable \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/compliance/financial-records",
    "duration": "permanent",
    "description": "金融交易记录合规归档",
    "tags": ["compliance", "financial", "2026"],
    "protectFromRansomware": true
  }'
```

### 3. 关键数据备份保护

在进行系统维护、软件升级或数据迁移前，锁定关键数据目录，确保操作过程中数据安全。

```bash
# 系统维护前锁定
curl -X POST http://localhost:8080/api/v1/immutable \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/database",
    "duration": "7d",
    "description": "系统维护期间的数据库保护"
  }'
```

### 4. 合同与重要文件保护

保护合同、证书、授权书等重要文件，防止意外删除或篡改。

## API 使用示例

### 基础操作

#### 1. 锁定目录

```bash
# 锁定单个目录
POST /api/v1/immutable
{
  "path": "/data/contracts",
  "duration": "30d",
  "description": "合同文件保护",
  "tags": ["contracts", "legal"],
  "protectFromRansomware": true
}
```

**响应**：
```json
{
  "code": 0,
  "message": "锁定成功",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "active",
    "lockedAt": "2026-03-24 14:00:00",
    "expiresAt": "2026-04-23 14:00:00",
    "snapshotPath": "/data/.immutable/immutable_20260324-140000_abc123"
  }
}
```

#### 2. 查看锁定状态

```bash
# 查看所有锁定记录
GET /api/v1/immutable

# 按状态过滤
GET /api/v1/immutable?status=active

# 按路径搜索
GET /api/v1/immutable?pathContains=/data/contracts
```

#### 3. 获取统计信息

```bash
GET /api/v1/immutable/statistics
```

**响应**：
```json
{
  "code": 0,
  "data": {
    "totalRecords": 15,
    "totalSize": 10737418240,
    "byStatus": {
      "active": 12,
      "expired": 2,
      "unlocked": 1
    },
    "byDuration": {
      "7d": 3,
      "30d": 10,
      "permanent": 2
    }
  }
}
```

#### 4. 检查路径保护状态

```bash
# 检查指定路径是否受保护
POST /api/v1/immutable/check-ransomware
{
  "path": "/data/important-docs"
}
```

**响应**：
```json
{
  "code": 0,
  "data": {
    "path": "/data/important-docs",
    "protected": true,
    "recordId": "550e8400-e29b-41d4-a716-446655440000",
    "lockedAt": "2026-03-24 14:00:00",
    "expiresAt": "2026-04-23 14:00:00",
    "protectedByRansomware": true
  }
}
```

### 快捷操作

#### 快速锁定

使用默认配置（启用防勒索保护）快速锁定目录：

```bash
POST /api/v1/immutable/quick-lock
{
  "path": "/data/photos",
  "duration": "30d"
}
```

#### 批量锁定

一次锁定多个目录：

```bash
POST /api/v1/immutable/batch-lock
{
  "paths": [
    "/data/documents",
    "/data/photos",
    "/data/videos"
  ],
  "duration": "7d",
  "description": "批量保护重要目录"
}
```

### 高级操作

#### 延长锁定时间

```bash
POST /api/v1/immutable/{id}/extend
{
  "duration": "30d"
}
```

#### 解锁路径

**注意**：只有过期或管理员强制解锁才能解除锁定。

```bash
# 正常解锁（需已过期）
DELETE /api/v1/immutable/{id}

# 强制解锁（管理员权限）
DELETE /api/v1/immutable/{id}
{
  "force": true
}
```

#### 从快照恢复

创建不可变快照的可写副本：

```bash
POST /api/v1/immutable/{id}/restore
{
  "targetPath": "/data/restored-documents"
}
```

## CLI 命令

### 使用 nasctl 命令行工具

```bash
# 锁定目录
nasctl immutable lock /data/contracts --duration 30d --protect-ransomware

# 查看锁定记录
nasctl immutable list

# 查看路径状态
nasctl immutable status /data/contracts

# 延长锁定
nasctl immutable extend <record-id> --duration 30d

# 解锁
nasctl immutable unlock <record-id> --force

# 从快照恢复
nasctl immutable restore <record-id> --target /data/restored
```

## 最佳实践

### 1. 锁定时长选择

| 场景 | 推荐时长 | 说明 |
|------|----------|------|
| 临时备份保护 | 7 天 | 系统维护、数据迁移期间 |
| 重要数据保护 | 30 天 | 日常重要文档、照片 |
| 合规归档 | 永久 | 金融、医疗、法律等合规数据 |

### 2. 防勒索保护建议

- 始终启用 `protectFromRansomware` 选项
- 对关键目录实施定期锁定策略
- 定期检查锁定状态和统计信息
- 在勒索病毒高发期增加锁定频率

### 3. 存储空间管理

- 定期检查统计信息，了解空间占用
- 及时解锁不再需要的记录
- 设置合理的自动清理策略（默认清理已解锁超过 30 天的记录）

### 4. 权限控制

- 强制解锁（force=true）应限制为管理员
- 建议通过 RBAC 控制 WriteOnce API 访问权限
- 记录所有解锁操作的审计日志

### 5. 自动化脚本示例

```bash
#!/bin/bash
# 每日自动锁定关键目录

KEY_DIRS=(
  "/data/documents"
  "/data/contracts"
  "/data/financial"
)

for dir in "${KEY_DIRS[@]}"; do
  # 检查是否已锁定
  status=$(curl -s "http://localhost:8080/api/v1/immutable/status?path=$dir" | jq -r '.data.locked')
  
  if [ "$status" != "true" ]; then
    echo "锁定 $dir ..."
    curl -X POST http://localhost:8080/api/v1/immutable/quick-lock \
      -H "Content-Type: application/json" \
      -d "{\"path\": \"$dir\", \"duration\": \"30d\"}"
  fi
done
```

## 注意事项

1. **解锁限制**：活跃状态的记录不能随意解锁，需要等待过期或管理员强制解锁
2. **存储空间**：锁定会创建快照，占用存储空间，请确保有足够空间
3. **权限要求**：设置 `chattr +i` 需要 root 权限
4. **数据恢复**：恢复操作创建的是可写副本，原快照仍保留
5. **性能影响**：大量小文件可能影响快照创建速度

## 故障排除

### 常见问题

**Q: 锁定失败提示"路径已被锁定"**

A: 该路径已有活跃状态的锁定记录。可以先查看现有记录，选择延长锁定时间或等待过期。

**Q: 无法解锁提示"记录尚未过期"**

A: 需要使用 `force: true` 参数强制解锁（需要管理员权限）。

**Q: 快照创建失败**

A: 检查路径是否在 btrfs 卷上，以及是否有足够的存储空间。

## 相关文档

- [存储管理指南](./STORAGE_GUIDE.md)
- [备份与恢复](./BACKUP_GUIDE.md)
- [安全最佳实践](./SECURITY_GUIDE.md)

---

*文档版本: v2.255.0*
*维护团队: NAS-OS 礼部*