# VM Secure Boot安全设计

> **刑部第162轮任务** - 对标TrueNAS 25.10 VM Secure Boot
> **目标**: 设计虚拟机安全启动机制，增强VM安全性

---

## 1.概述

### 1.1 Secure Boot原理

Secure Boot通过UEFI固件验证启动链完整性：
- UEFI固件验证签名
- bootloader验证签名
- 内核验证签名
- 防止恶意代码注入

### 1.2 TrueNAS 25.10实现

TrueNAS 25.10为虚拟机添加Secure Boot支持：
- UEFI启动模式
- Secure Boot状态可配置
- 签名密钥管理

---

## 2. 安全设计

### 2.1 启动验证链

```
UEFI Firmware → verify → bootloader → verify → kernel → verify → initramfs
         ↓                     ↓                 ↓                ↓
    Platform Key          Key Exchange Key   Signed Kernel    Signed Init
        (PK)                  (KEK)         (db signatures)   (db signatures)
```

### 2.2 密钥管理

| 密钥类型 | 说明 | 管理 |
|----------|------|------|
| PK | Platform Key | 系统管理员 |
| KEK | Key Exchange Key | 系统管理员 |
| db | Authorized Signature Database | 自动更新 |
| dbx | Forbidden Signature Database | 自动更新 |

---

## 3. API设计

```yaml
# 配置VM Secure Boot
POST /api/v1/vm/:name/secure-boot
  Body:
    - enabled: true/false
    - mode: strict/relaxed
    - customKeys: 自定义密钥路径（可选）

# 查询Secure Boot状态
GET /api/v1/vm/:name/secure-boot
  Response:
    - enabled: true/false
    - status: valid/invalid/unsupported
    - keys: 密钥信息
```

---

## 4. 安全考虑

### 4.1密钥保护

- 密钥存储加密
- 密钥备份机制
- 密钥更新流程

### 4.2 故障恢复

- Secure Boot失败回退
- 签名密钥恢复
- 安全启动日志

---

**预计完成**: Phase 1 - M108