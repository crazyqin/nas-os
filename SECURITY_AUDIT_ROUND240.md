# 安全审计报告 Round 240 — VM Secure Boot安全评估

**审计轮次**: Round 240  
**审计日期**: 2026-04-29  
**审计范围**: VM Secure Boot设计 + `internal/vm/` 全量代码  
**审计人**: 刑部  
**审计级别**: 深度安全评估

---

## 1. 审计摘要

| 指标 | 数值 |
|------|------|
| 审计文件数 | 9 |
| 发现问题总数 | 17 |
| 🔴 严重 (Critical) | 4 |
| 🟠 高危 (High) | 5 |
| 🟡 中危 (Medium) | 5 |
| 🔵 低危 (Low) | 3 |
| Secure Boot实现状态 | ❌ 未实现 |

---

## 2. Secure Boot 设计评估

### 2.1 设计文档审查

审查了两份设计文档：
- `docs/design/vm-secure-boot-design.md` (v2.463.0)
- `docs/design/vm-secure-boot.md` (v1.0)

**评估结论：设计合理但存在关键缺陷。**

| 设计项 | 状态 | 说明 |
|--------|------|------|
| UEFI Secure Boot 链条 | ✅ 设计完整 | PK → KEK → DB → DBX 层次清晰 |
| TPM 2.0 集成 | ✅ 设计完整 | 对标 TrueNAS 26 |
| 密钥管理 | ⚠️ 设计阶段 | 文档标注"下一轮"，M2阶段才实现 |
| 安全策略引擎 | ✅ 设计完整 | strict/standard/audit 三模式 |
| OVMF 固件集成 | ✅ 设计合理 | 使用开源 OVMF |
| API 接口设计 | ✅ 设计完整 | RESTful 接口定义清楚 |
| 实现时间线 | ⚠️ 风险 | 5阶段跨度大(M1-M5)，M2(第240轮)才开始核心实现 |

### 2.2 实现差距评估

| 项目 | 设计 | 实现 | 差距 |
|------|------|------|------|
| `internal/vm/secureboot/` | 设计了6个模块 | **空目录，0个文件** | 🔴 完全未实现 |
| `go-tpm` 依赖 | 设计要求 | `go.mod` 中不存在 | 🔴 未引入 |
| x509证书处理 | 设计要求 | 无相关代码 | 🔴 未实现 |
| SecureBootManager | 设计要求 | 不存在 | 🔴 未实现 |
| ImportConfig.SecureBoot | 引用字段 | **从未被使用** | ⚠️ 死代码 |

**结论：Secure Boot在Round 240仍处于纯设计阶段，没有任何实现代码。**

### 2.3 两份设计文档不一致

| 差异点 | `vm-secure-boot-design.md` | `vm-secure-boot.md` |
|--------|---------------------------|---------------------|
| 目标TrueNAS版本 | TrueNAS 26 | TrueNAS 25.10 |
| 安全审计轮次 | Round 235 | 无标注 |
| 实现计划 | M1(235轮)-M5(255轮) | Phase 1(v2.480)-Phase 3(v2.500) |
| 目录结构 | 未指定 | `internal/vm/security/` |

**风险：两份文档版本控制混乱，可能导致实现方向偏差。**

---

## 3. 现有代码安全问题（internal/vm/）

### 3.1 🔴 严重 (Critical)

#### C-01: VNC 监听 0.0.0.0 无认证
- **文件**: `manager.go:269`
- **位置**: `generateLibvirtXML()` 函数
- **问题**: VNC 监听地址硬编码为 `0.0.0.0`，暴露到所有网络接口，且无密码认证
- **风险**: 任何能访问该端口的攻击者可直接控制虚拟机桌面，获取完整系统控制权
- **代码片段**:
  ```go
  <graphics type='vnc' port='%d' autoport='no' listen='0.0.0.0'>
    <listen type='address' address='0.0.0.0'/>
  </graphics>
  ```
- **CVSS**: 9.1 (Critical)
- **修复建议**:
  1. 默认监听 `127.0.0.1`，仅在用户明确启用时开放外网
  2. 强制 VNC 密码认证（`passwd` 属性）
  3. 添加 noVNC token 认证机制
  4. 通过防火墙规则限制 VNC 端口访问范围

#### C-02: VM API 端点无认证/授权
- **文件**: `handlers.go:38-62`
- **位置**: `RegisterRoutes()` 函数
- **问题**: 所有 VM API 端点（创建/删除/启停/迁移/备份）均无认证中间件
- **风险**: 未授权用户可任意操控虚拟机，包括删除、迁移、获取 VNC 连接
- **CVSS**: 9.8 (Critical)
- **修复建议**:
  1. 所有 VM 管理 API 添加 JWT/Session 认证中间件
  2. 实现 RBAC 权限控制（admin/user/viewer）
  3. 敏感操作（删除/迁移/备份）需二次确认

#### C-03: ISO 下载 SSRF 漏洞
- **文件**: `extended.go:149-165`
- **位置**: `DownloadISO()` 函数
- **问题**: 接受任意 URL 下载 ISO，无白名单校验，攻击者可探测内网服务
- **风险**: 服务器端请求伪造(SSRF)，可扫描内网、读取云元数据服务
- **代码片段**:
  ```go
  func (m *Manager) DownloadISO(ctx context.Context, name, url string) (*ISOImage, error) {
      // 直接使用用户提供的 url，无任何校验
      cmd := exec.CommandContext(ctx, "wget", "-O", isoPath, url)
  ```
- **CVSS**: 8.6 (Critical)
- **修复建议**:
  1. 实现 URL 白名单（仅允许已知镜像源域名）
  2. 禁止私有 IP/URL（`127.0.0.0/8`、`10.0.0.0/8`、`169.254.169.254` 等）
  3. 使用 HTTP 客户端替代 `wget`，便于控制请求行为
  4. 限制下载文件大小

#### C-04: XML 注入 — ISOPath 未转义
- **文件**: `manager.go:227-233`
- **位置**: `generateLibvirtXML()` 函数
- **问题**: `vm.ISOPath` 直接插入 libvirt XML，未做 XML 转义
- **风险**: 恶意路径可注入任意 XML，可能导致 libvirt 配置篡改
- **代码片段**:
  ```go
  <source file='%s'/>  // vm.ISOPath 直接插入，未转义
  ```
- **CVSS**: 7.7 (High)
- **修复建议**:
  1. 使用 Go 的 `xml.EscapeText()` 转义所有插入 XML 的用户输入
  2. 使用 `encoding/xml` 库构建 XML，避免字符串拼接
  3. 验证 ISOPath 仅包含合法路径字符

---

### 3.2 🟠 高危 (High)

#### H-01: CloneVM 锁竞争条件
- **文件**: `extended.go:210-215`
- **位置**: `CloneVM()` 函数
- **问题**: 在持有 `m.mu.Lock()` 的情况下调用 `m.mu.Unlock()` 后再调用 `CreateVM()`（内部也会加锁），导致窗口期内并发状态不一致
- **代码片段**:
  ```go
  m.mu.Unlock()
  vm, err := m.CreateVM(ctx, config) // CreateVM 内部也会 Lock
  m.mu.Lock()
  ```
- **风险**: 并发克隆操作可能导致重复 VM 名称、资源冲突
- **修复建议**: 重构为先收集模板数据（只读锁），再单独调用 CreateVM

#### H-02: ExportVM 同样的锁竞争条件
- **文件**: `extended.go:262-267`
- **位置**: `ExportVM()` 函数
- **问题**: 与 CloneVM 相同的锁释放/重获模式
- **风险**: 并发导出可能产生不一致状态
- **修复建议**: 同 H-01

#### H-03: RestoreVMBackup 接受任意文件路径
- **文件**: `extended.go:289-291`
- **位置**: `RestoreVMBackup()` 函数
- **问题**: `backupPath` 参数直接来自用户输入，未限制路径范围
- **风险**: 攻击者可读取并解压系统任意 `.tar.gz` 文件，利用 tar slip 或路径遍历
- **修复建议**:
  1. 将 backupPath 限制在预定义的备份目录内
  2. 验证路径前缀为 `m.storagePath + "/backups/"`
  3. 解压时检查归档成员路径

#### H-04: VNC 连接信息无 Token 保护
- **文件**: `manager.go:326-336` + `types.go:103`
- **位置**: `GetVNCConnection()` 函数
- **问题**: 返回的 VNCConnection 结构体定义了 `Token` 字段但从未填充，仅返回 Host:Port
- **风险**: 任何人知道 VM ID 即可获取 VNC 连接信息
- **修复建议**: 实现一次性 token 机制，验证后生成 token，连接时校验

#### H-05: GPU VFIO 驱动绑定使用不安全的命令模式
- **文件**: `gpu.go:174-176`
- **位置**: `bindToVFIO()` 函数
- **问题**: 使用 `exec.Command("echo", ..., ">", ...)` 模式 — 这不会执行 shell 重定向，实际上 `>` 会作为普通参数传递给 echo
- **代码片段**:
  ```go
  cmd := exec.Command("echo", "vfio-pci", ">",
      fmt.Sprintf("/sys/bus/pci/devices/%s/driver_override", pciAddr))
  ```
- **风险**: 该功能实际上不工作，且 pciAddr 未验证格式，若修复为 shell 命令则存在命令注入风险
- **修复建议**:
  1. 使用 `os.WriteFile()` 替代 echo 重定向
  2. 验证 PCI 地址格式（`XX:XX.X`）

---

### 3.3 🟡 中危 (Medium)

#### M-01: 错误信息泄露内部路径
- **文件**: 多处
- **问题**: 错误响应中包含完整文件路径（如 `/mnt/vms/vm-xxx/disk.qcow2`）
- **风险**: 信息泄露，辅助攻击者了解系统结构
- **修复建议**: 生产环境仅返回通用错误信息，详细路径记录到日志

#### M-02: ISO 上传无文件大小限制
- **文件**: `iso.go` — `UploadISO()` 函数
- **问题**: `io.Copy` 无大小限制，攻击者可上传超大文件耗尽磁盘
- **风险**: 磁盘耗尽导致拒绝服务
- **修复建议**: 使用 `io.LimitReader()` 限制上传大小，上限与 `GetUploadInfo()` 返回的 maxSize 一致

#### M-03: USB 设备 ID 格式校验不足
- **文件**: `manager.go:241-248`
- **位置**: `generateLibvirtXML()` USB 直通部分
- **问题**: USB ID 仅检查 `:` 分隔，未验证十六进制格式
- **风险**: 非法格式可能导致 libvirt XML 解析异常
- **修复建议**: 使用正则 `^[0-9a-fA-F]{4}$` 验证 vendor:product ID

#### M-04: ImportConfig.SecureBoot 字段为死代码
- **文件**: `import-enhanced.go:30`
- **问题**: `ImportConfig.SecureBoot` 字段已声明但从未在 `ImportVM()` 中使用
- **风险**: 用户设置 SecureBoot=true 无效，造成虚假安全感
- **修复建议**: 在 Round 240 M2 实现中集成该字段

#### M-05: 无请求速率限制
- **文件**: `handlers.go`
- **问题**: 所有 API 端点无速率限制
- **风险**: 暴力破解、资源耗尽攻击
- **修复建议**: 使用中间件实现速率限制（如 `golang.org/x/time/rate`）

---

### 3.4 🔵 低危 (Low)

#### L-01: 内置模板硬编码创建时间
- **文件**: `manager.go:182-220`
- **位置**: `createBuiltInTemplates()` 函数
- **问题**: `CreatedAt: time.Now()` — 每次启动时间不同，导致配置文件不可复现
- **风险**: 无安全影响，代码质量问题
- **修复建议**: 使用固定时间或省略创建时间

#### L-02: getLibvirtStats 中的统计逻辑错误
- **文件**: `manager.go:414-416`
- **位置**: `getLibvirtStats()` 函数
- **问题**: CPU 使用率使用 memory 指标计算，逻辑错误
- **风险**: 统计数据不准确，可能导致运维误判
- **修复建议**: 使用正确的 CPU time 差值计算

#### L-03: MigrateVM 缺少 TLS 验证
- **文件**: `extended.go:237`
- **问题**: `qemu+tcp://` 迁移使用明文 TCP，无加密
- **风险**: 迁移数据可被中间人窃听（含 VM 磁盘数据）
- **修复建议**: 使用 `qemu+tls://` 替代，配置 TLS 证书

---

## 4. Secure Boot 密钥管理安全性评估

### 4.1 当前状态：无实现

| 评估项 | 状态 | 风险 |
|--------|------|------|
| PK (Platform Key) 管理 | ❌ 无实现 | 🔴 |
| KEK (Key Exchange Key) 管理 | ❌ 无实现 | 🔴 |
| DB (Signature Database) 管理 | ❌ 无实现 | 🔴 |
| DBX (Forbidden Signatures) 管理 | ❌ 无实现 | 🔴 |
| 密钥生成 | ❌ 无实现 | 🔴 |
| 密钥轮换 | ❌ 无实现 | 🔴 |
| 密钥备份/恢复 | ❌ 无实现 | 🔴 |
| 签名验证 | ❌ 无实现 | 🔴 |
| TPM 集成 | ❌ 无实现 | 🔴 |

### 4.2 设计阶段密钥管理风险点

基于设计文档预审：

| 风险 | 说明 | 建议 |
|------|------|------|
| 密钥存储位置 | 设计未明确 PK/KEK 存储位置 | 应使用 HSM 或加密密钥库，非明文文件 |
| 密钥生成算法 | 设计未指定算法和密钥长度 | RSA-2048+ 或 ECDSA P-256+ |
| 密钥轮换策略 | 设计未提及 | 应设计 PK 轮换 + KEK 轮换流程 |
| 自定义密钥风险 | `AllowCustomKeys: true` 允许用户导入任意密钥 | 应限制为管理员权限 + 审计日志 |
| 标准模式密钥来源 | 设计提及"微软标准密钥"但未说明获取方式 | 应从 Microsoft UEFI CA 获取，非自行生成 |

---

## 5. 合规检查

### 5.1 NIST SP 800-193 (平台固件弹性)

| 要求 | 状态 | 说明 |
|------|------|------|
| 保护：固件完整性保护 | 🔴 不合规 | Secure Boot 未实现 |
| 检测：固件完整性检测 | 🔴 不合规 | 无签名验证 |
| 恢复：安全恢复机制 | 🟡 部分合规 | 快照/备份机制存在但无完整性验证 |

### 5.2 UEFI Forum Secure Boot 规范

| 要求 | 状态 | 说明 |
|------|------|------|
| PK/KEK/DB/DBX 层次 | 🔴 不合规 | 未实现 |
| UEFI 变量安全存储 | 🔴 不合规 | 未实现 |
| 签名验证流程 | 🔴 不合规 | 未实现 |
| Audit Mode 支持 | 🔴 不合规 | 未实现 |

### 5.3 Go 代码安全最佳实践

| 要求 | 状态 | 说明 |
|------|------|------|
| 命令注入防护 | 🟡 部分合规 | VM名称验证存在，但其他参数不充分 |
| 输入验证 | 🟡 部分合规 | 部分参数有验证，路径/URL缺失 |
| 密码/密钥存储 | 🔴 不合规 | 无密钥管理基础设施 |
| 认证授权 | 🔴 不合规 | API 端点无认证 |
| 错误处理 | 🟡 部分合规 | 错误信息泄露内部路径 |

---

## 6. 修复优先级

### P0 — 立即修复（Block M2 实现）

| ID | 问题 | 预计工时 |
|----|------|----------|
| C-02 | API 认证/授权中间件 | 8h |
| C-03 | ISO 下载 SSRF 防护 | 4h |
| C-04 | XML 注入防护 | 4h |
| C-01 | VNC 监听地址 + 认证 | 6h |

### P1 — M2 前完成（Secure Boot 实现前提）

| ID | 问题 | 预计工时 |
|----|------|----------|
| H-01 | CloneVM 锁竞争修复 | 2h |
| H-02 | ExportVM 锁竞争修复 | 2h |
| H-03 | 备份路径限制 | 2h |
| H-04 | VNC Token 认证 | 4h |
| H-05 | GPU 驱动绑定修复 | 2h |

### P2 — M3 前完成

| ID | 问题 | 预计工时 |
|----|------|----------|
| M-01 | 错误信息脱敏 | 2h |
| M-02 | ISO 上传大小限制 | 1h |
| M-03 | USB ID 格式验证 | 1h |
| M-04 | 死代码清理 | 0.5h |
| M-05 | 速率限制中间件 | 4h |

### P3 — 持续改进

| ID | 问题 | 预计工时 |
|----|------|----------|
| L-01 | 模板时间修复 | 0.5h |
| L-02 | 统计逻辑修正 | 1h |
| L-03 | 迁移 TLS 加密 | 4h |

---

## 7. Secure Boot 实现安全建议

### 7.1 实现前必须完成

1. **修复 C-01~C-04** — 所有严重漏洞在 Secure Boot 实现前必须关闭
2. **引入认证中间件** — Secure Boot API 必须有权限控制
3. **统一设计文档** — 合并两份设计文档，消除版本不一致

### 7.2 密钥管理安全要求

```
密钥存储: HSM 或加密密钥库 (非明文文件)
密钥算法: RSA-2048+ 或 ECDSA P-256+
密钥轮换: PK 5年, KEK 3年, DB 按需
访问控制: 仅 root/admin 可操作密钥
审计日志: 所有密钥操作记录审计
备份加密: 密钥备份必须加密存储
```

### 7.3 实现架构建议

```
internal/vm/secureboot/
├── manager.go      # SecureBootManager (核心管理)
├── keys.go         # 密钥管理 (PK/KEK/DB/DBX)
├── keys_test.go    # 密钥测试
├── signature.go    # 签名验证引擎
├── signature_test.go
├── policy.go       # 安全策略引擎
├── policy_test.go
├── ovmf.go         # OVMF 固件集成
├── ovmf_test.go
├── api.go          # API Handler (必须含认证)
└── api_test.go
```

---

## 8. 结论

**整体风险评估：🔴 高风险**

VM Secure Boot 设计文档质量良好，但存在两个核心问题：

1. **现有代码基础安全问题严重** — 4个Critical级别漏洞（无认证、SSRF、XML注入、VNC暴露），在这些基础问题修复前，Secure Boot 的实现将建立在不安全的基础之上。

2. **Secure Boot 实现完全缺失** — `internal/vm/secureboot/` 为空目录，设计中的 M2 阶段（第240轮）任务未开始，`go-tpm` 依赖未引入，ImportConfig.SecureBoot 为死代码。

**建议**：
- **暂缓 M2 实现**，优先修复 P0 级漏洞
- **合并两份设计文档**为单一权威版本
- **引入密钥管理基础设施**后再开始 Secure Boot 编码
- **下一审计轮次(Round 245)**重点检查 P0 修复情况 + Secure Boot 初步实现

---

**刑部签章**  
**审计时间**: 2026-04-29 20:21 CST  
**下次审计**: Round 245
