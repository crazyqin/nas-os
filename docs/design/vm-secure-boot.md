# VM Secure Boot 安全设计

**版本**: v1.0
**日期**: 2026-04-24
**对标**: TrueNAS 25.10 VM Secure Boot
**负责**: 刑部

---

## 1. 概述

TrueNAS 25.10 为虚拟机引入了 Secure Boot 支持，增强VM安全性。nas-os需对标此功能，为用户提供安全的虚拟化环境。

---

## 2. Secure Boot 原理

### 2.1 UEFI Secure Boot流程

```
┌─────────────────────────────────────────────────────────┐
│                   Secure Boot 流程                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────┐                                       │
│  │ Power On    │                                       │
│  └─────────────┘                                       │
│         │                                              │
│         ▼                                              │
│  ┌─────────────┐                                       │
│  │ UEFI启动    │                                       │
│  │ 加载密钥    │                                       │
│  └─────────────┘                                       │
│         │                                              │
│         ▼                                              │
│  ┌─────────────┐                                       │
│  │ 验证签名    │                                       │
│  │ KEK/DB检查 │                                       │
│  └─────────────┘                                       │
│         │                                              │
│    ┌────┴────┐                                         │
│    ▼         ▼                                         │
│ ┌───────┐ ┌───────┐                                   │
│ │验证通过│ │验证失败│                                   │
│ │启动OS │ │阻止启动│                                   │
│ └───────┘ └───────┘                                   │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 2.2 密钥层次

| 密钥 | 说明 | 用途 |
|------|------|------|
| PK | Platform Key | 平台信任根 |
| KEK | Key Exchange Key | 签名数据库密钥 |
| DB | Signature Database | 允许启动的签名 |
| DBX | Forbidden Signatures | 禁止启动的签名 |

---

## 3. nas-os 实现设计

### 3.1 功能目标

| 功能 | 说明 | 优先级 |
|------|------|--------|
| Secure Boot启用 | VM启用安全启动 | P0 |
| 密钥管理 | PK/KEK/DB管理 | P0 |
| 签名验证 | 启动镜像验证 | P0 |
| 自定义密钥 | 用户自定义签名 | P1 |
| 密钥备份 | 密钥导出/导入 | P1 |

### 3.2 架构设计

```
internal/vm/security/
├── secureboot.go      # Secure Boot核心实现
├── keys.go            # 密钥管理
├── signature.go       # 签名验证
├── cert.go            # 证书处理
├── policy.go          # 安全策略
└── api.go             # API接口
```

---

## 4. 实现细节

### 4.1 UEFI配置

```go
// SecureBootConfig Secure Boot配置
type SecureBootConfig struct {
    Enabled       bool
    Mode          SecureBootMode // Standard/Custom
    PlatformKey   []byte         // PK密钥
    KeyExchangeKey []byte        // KEK密钥
    AllowedDB     []Signature    // 允许签名
    ForbiddenDBX  []Signature    // 禁止签名
}

// SecureBootMode 安全启动模式
type SecureBootMode string

const (
    StandardMode SecureBootMode = "standard" // 使用微软标准密钥
    CustomMode   SecureBootMode = "custom"   // 用户自定义密钥
    AuditMode    SecureBootMode = "audit"    // 审计模式（仅记录）
)
```

### 4.2 密钥管理

| 操作 | 说明 | API |
|------|------|-----|
| 生成密钥 | 生成PK/KEK密钥对 | POST /api/v1/vm/security/keys |
| 导入密钥 | 导入已有密钥 | POST /api/v1/vm/security/keys/import |
| 导出密钥 | 导出密钥备份 | GET /api/v1/vm/security/keys/export |
| 删除密钥 | 删除密钥 | DELETE /api/v1/vm/security/keys/:id |
| 签名镜像 | 为启动镜像签名 | POST /api/v1/vm/security/sign |

---

## 5. 与QEMU集成

### 5.1 OVMF配置
- 使用OVMF固件（开源UEFI）
- 配置Secure Boot变量
- 启动参数传递

### 5.2 QEMU启动参数

```bash
qemu-system-x86_64 \
  -drive if=pflash,format=raw,file=OVMF_CODE.fd \
  -drive if=pflash,format=raw,file=OVMF_VARS.fd \
  -machine q35,accel=kvm \
  -cpu host \
  -m 4096 \
  ...
```

---

## 6. 安全策略

### 6.1 默认策略
- 使用微软标准签名密钥（支持Windows/Linux）
- 审计模式下先验证兼容性
- 生产环境启用Strict模式

### 6.2 自定义策略
| 场景 | 策略 |
|------|------|
| 仅启动自签名OS | 仅信任用户签名 |
| 混合启动 | 信任微软+用户签名 |
| 高安全环境 | 仅信任特定签名 |

---

## 7. API设计

| API | 说明 | Method |
|-----|------|--------|
| /api/v1/vm/:id/secureboot | Secure Boot配置 | GET/PUT |
| /api/v1/vm/:id/secureboot/status | 启动状态 | GET |
| /api/v1/vm/security/keys | 密钥列表 | GET |
| /api/v1/vm/security/keys | 创建密钥 | POST |
| /api/v1/vm/security/sign | 签名镜像 | POST |

---

## 8. 实现路径

### 8.1 Phase 1: 基础支持（v2.480.0）
- Secure Boot启用/禁用
- 使用标准微软密钥
- 基础验证流程

### 8.2 Phase 2: 自定义密钥（v2.490.0）
- 密钥生成/导入
- 自签名镜像
- 密钥备份功能

### 8.3 Phase 3: 完善安全（v2.500.0）
- 审计模式
- 安全策略细化
- Web界面完善

---

## 9. 测试计划

### 9.1 功能测试
- Windows Secure Boot启动
- Linux Secure Boot启动
- 未签名镜像阻止

### 9.2 安全测试
- 密钥管理安全
- 签名验证完整性
- 密钥备份恢复

---

## 10. 文档规划

- `docs/vm-secure-boot-guide.md`: Secure Boot指南
- `docs/vm-secure-boot-troubleshooting.md`: 故障排查
- `docs/vm-custom-keys.md`: 自定义密钥教程