# S3 对象存储网关使用指南

> **版本**: v2.480.0  
> **更新日期**: 2026-05-02

---

## 概述

NAS-OS S3 对象存储网关将您的 NAS 存储空间暴露为兼容 Amazon S3 API 的对象存储服务，方便应用和工具通过标准 S3 协议访问。

### 适用场景

- **应用集成** - 支持 S3 SDK 的应用直接对接 NAS
- **数据迁移** - 从云 S3 迁移到本地 NAS，或反向
- **备份目标** - 使用 S3 兼容工具备份到 NAS
- **开发测试** - 本地 S3 环境，无需云服务费用
- **混合云** - 本地 S3 + 云 S3 统一管理

---

## 快速开始

### 1. 启用 S3 网关

1. 进入「存储」→「对象存储」→「S3 网关」
2. 点击「启用」
3. 配置参数：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| 监听端口 | S3 API 端口 | 9000 |
| 绑定地址 | 监听的网络接口 | 0.0.0.0 |
| 存储后端 | 使用哪个存储池 | 自动选择 |
| 区域标识 | S3 Region | us-east-1 |

### 2. 创建 Access Key

1. 进入「S3 网关」→「访问密钥」
2. 点击「创建」
3. 记录 Access Key ID 和 Secret Access Key
4. 配置权限策略（读写/只读/自定义）

### 3. 客户端连接

```bash
# 使用 AWS CLI
aws configure set aws_access_key_id YOUR_ACCESS_KEY
aws configure set aws_secret_access_key YOUR_SECRET_KEY
aws configure set default.region us-east-1

# 列出 Bucket
aws --endpoint-url http://NAS_IP:9000 s3 ls

# 上传文件
aws --endpoint-url http://NAS_IP:9000 s3 cp file.txt s3://my-bucket/

# 下载文件
aws --endpoint-url http://NAS_IP:9000 s3 cp s3://my-bucket/file.txt ./
```

---

## Bucket 管理

### 创建 Bucket

1. 进入「S3 网关」→「Bucket 管理」
2. 点击「创建 Bucket」
3. 设置名称（全局唯一，小写字母+数字+连字符）
4. 选择存储池
5. 可选：启用版本控制、设置生命周期策略

### 权限控制

| 权限 | 说明 |
|------|------|
| 完全控制 | 所有 S3 操作 |
| 读写 | 读取和写入对象 |
| 只读 | 仅读取对象 |
| 自定义 | 基于前缀的细粒度权限 |

---

## 高级功能

### 版本控制

启用后，每次覆盖或删除对象都会保留历史版本：
- 可恢复任意历史版本
- 删除标记可撤销
- 存储空间会相应增加

### 生命周期策略

自动管理对象生命周期：
```yaml
lifecycle:
  rules:
    - prefix: "logs/"
      transition_days: 30
      transition_class: HDD    # 30天后移到HDD
      expiration_days: 365     # 365天后删除
```

### 复制策略

支持跨 Bucket 或跨 NAS 复制：
- 实时复制（写入时同步）
- 定时复制（按计划同步）
- 双向复制（双向同步）

---

## 兼容性

### 支持的 S3 操作

| 操作 | 说明 |
|------|------|
| PUT/GET/DELETE Object | 基本对象操作 |
| ListObjects/ListObjectsV2 | 列举对象 |
| Multipart Upload | 分片上传 |
| CopyObject | 对象复制 |
| HeadObject | 获取对象元数据 |
| GetBucketLocation | 获取 Bucket 区域 |
| PutBucketLifecycleConfiguration | 生命周期配置 |

### 已验证的客户端

| 客户端 | 版本 | 状态 |
|--------|------|------|
| AWS CLI | v2.x | ✅ 完全兼容 |
| boto3 (Python) | 1.26+ | ✅ 完全兼容 |
| MinIO Client (mc) | 最新 | ✅ 完全兼容 |
| rclone | v1.60+ | ✅ 完全兼容 |
| S3Browser | 最新 | ✅ 完全兼容 |
| CyberDuck | 最新 | ✅ 完全兼容 |

---

## 常见问题

### Q: 连接超时？

检查：
1. NAS 防火墙是否放行 S3 端口
2. S3 网关服务是否正常运行
3. 客户端 endpoint URL 是否正确

### Q: 上传大文件失败？

确认：
1. 是否启用了 Multipart Upload
2. 单次上传大小限制（默认 5GB）
3. 网络连接是否稳定

### Q: 性能不如预期？

优化建议：
1. 使用 SSD 存储池作为 S3 后端
2. 增大并发连接数
3. 启用连接池复用
4. 检查网络带宽是否为瓶颈

---

*文档版本：v2.480.0 | 最后更新：2026-05-02*
