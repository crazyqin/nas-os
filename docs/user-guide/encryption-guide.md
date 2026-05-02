# 加密存储指南

> **版本**: v2.475.0+ | **适用版本**: NAS-OS v2.475.0 及以上

## 概述

NAS-OS 提供两层加密保护：**加密卷（Vault）** 用于保护敏感数据，**文件夹级加密** 用于透明加密特定目录。两者均使用 AES-256-GCM 认证加密，密钥通过 PBKDF2 从用户密码派生。

## 加密架构

### 密钥层级

```
用户密码 → PBKDF2 (100,000次迭代) → 主密钥 (AES-256)
                                         ├── 数据密钥 A → 加密文件夹 A
                                         ├── 数据密钥 B → 加密文件夹 B
                                         └── Vault 密钥 → 加密卷数据
```

### 加密文件格式

每个加密文件包含 64 字节头部：
- 魔数标识：`NAS-ENC`
- 格式版本：`1`
- 非随机数据 + 文件数据（64KB 分块加密）

---

## 一、加密卷（Vault）

加密卷提供独立的加密存储空间，使用密码保护，锁定后数据不可访问。

### 创建加密卷

```bash
curl -X POST http://localhost:8080/api/v1/vaults \
  -H "Content-Type: application/json" \
  -d '{
    "name": "机密文件",
    "password": "YourStr0ngP@ssword"
  }'
```

> 密码要求：8-128 字符

### 解锁加密卷

```bash
curl -X POST http://localhost:8080/api/v1/vaults/{id}/unlock \
  -H "Content-Type: application/json" \
  -d '{"password": "YourStr0ngP@ssword"}'
```

### 锁定加密卷

```bash
curl -X POST http://localhost:8080/api/v1/vaults/{id}/lock
```

### 列出所有加密卷

```bash
curl http://localhost:8080/api/v1/vaults
```

### 删除加密卷

```bash
curl -X DELETE http://localhost:8080/api/v1/vaults/{id}
```

### Vault 状态

| 状态 | 说明 |
|------|------|
| `locked` | 已锁定，数据不可访问 |
| `unlocked` | 已解锁，可正常读写 |

---

## 二、文件夹级加密

文件夹级加密对指定目录进行透明加密，写入时自动加密，读取时自动解密。

### 解锁主密钥

首次使用需用密码解锁主密钥：

```bash
curl -X POST http://localhost:8080/api/v1/encryption/unlock \
  -H "Content-Type: application/json" \
  -d '{
    "password": "YourMasterPassword",
    "salt": "<base64-salt>"
  }'
```

### 创建加密文件夹

```bash
curl -X POST http://localhost:8080/api/v1/encryption/folders \
  -H "Content-Type: application/json" \
  -d '{
    "name": "私人文件",
    "path": "/vault/private",
    "physicalPath": "/data/.encrypted/private"
  }'
```

### 列出加密文件夹

```bash
curl http://localhost:8080/api/v1/encryption/folders
```

### 解锁/锁定文件夹

```bash
# 解锁
curl -X POST http://localhost:8080/api/v1/encryption/folders/{id}/unlock

# 锁定
curl -X POST http://localhost:8080/api/v1/encryption/folders/{id}/lock
```

### 密钥轮换

定期轮换数据密钥增强安全性：

```bash
curl -X POST http://localhost:8080/api/v1/encryption/folders/{id}/rotate-key
```

### 锁定主密钥

```bash
curl -X POST http://localhost:8080/api/v1/encryption/lock
```

### 查看加密状态

```bash
curl http://localhost:8080/api/v1/encryption/status
curl http://localhost:8080/api/v1/encryption/stats
```

---

## 安全参数

| 参数 | 值 | 说明 |
|------|-----|------|
| 加密算法 | AES-256-GCM | 认证加密，防篡改 |
| 密钥派生 | PBKDF2 | 100,000 次迭代 |
| 盐值长度 | 32 字节 | 随机生成 |
| Nonce 长度 | 12 字节 | AES-GCM 标准 |
| 分块大小 | 64 KB | 流式加密，支持大文件 |

## 最佳实践

1. **使用强密码**：至少 12 位，包含大小写字母、数字和特殊字符
2. **定期轮换密钥**：每 90 天轮换一次数据密钥
3. **锁定不用的卷**：离开工位时手动锁定加密卷
4. **备份密钥**：妥善保管密码，丢失后数据无法恢复
5. **物理安全**：加密卷保护逻辑安全，仍需配合物理安全措施

## 注意事项

- **密码丢失 = 数据丢失**：没有后门，没有密码重置
- **性能影响**：加密会带来约 5-15% 的读写性能下降
- **内存安全**：主密钥仅在解锁状态保存在内存中
- **卸载前锁定**：系统关机前确保所有加密卷已锁定
