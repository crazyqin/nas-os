# 勒索防护使用教程

**版本**: v2.396.0 | **更新日期**: 2026-04-05 | **对标**: TrueNAS Ransomware Defense

---

## 什么是勒索防护？

nas-os 勒索防护系统可实时检测勒索软件攻击行为，自动阻断威胁并保护数据。

### 核心能力

| 能力 | 说明 |
|------|------|
| **实时监控** | SMB/NFS 文件操作实时分析 |
| **诱饵检测** | 隐藏诱饵文件，触发即告警 |
| **行为分析** | 批量加密、扩展名修改检测 |
| **自动响应** | 阻断会话、创建快照、锁定目录 |
| **WriteOnce** | 不可变存储，数据写入后无法修改 |

---

## 快速启用

### WebUI 配置步骤

1. 「安全管理」→ 「勒索防护」
2. 开启以下选项：
   - ✅ SMB 实时监控
   - ✅ NFS 实时监控
   - ✅ 诱饵文件检测
   - ✅ 自动阻断攻击
   - ✅ 自动创建快照

3. 配置告警通知：
   - 邮件地址
   - Webhook URL

4. 选择保护目录
5. 点击「应用」

---

## 诱饵文件配置

### 什么是诱饵文件？

诱饵文件是隐藏在共享目录中的「陷阱」文件。攻击者触碰诱饵即触发告警。

### 配置诱饵路径

「安全管理」→ 「勒索防护」→ 「诱饵设置」

```yaml
security:
  ransomware:
    honeypot:
      enabled: true
      files:
        - "/shared/.honeypot/财务报表.xlsx"
        - "/shared/.honeypot/合同文档.docx"
        - "/shared/.honeypot/重要资料.pdf"
```

### 诱饵特性

- 对普通用户不可见
- 攻击者扫描时可见
- 任何操作（读取/修改/删除）触发告警
- 自动阻断攻击者会话

---

## WriteOnce 不可变存储

### 什么是 WriteOnce？

WriteOnce 是 nas-os 独家功能，创建写入后不可修改删除的存储区域。这是勒索防护的终极防线。

### 配置不可变目录

「存储管理」→ 「数据集」→ 选择数据集 → 「WriteOnce 设置」

```bash
# 命令行配置
nas-os dataset set-writeonce tank/backup
```

### WriteOnce 特性

| 特性 | 说明 |
|------|------|
| 一次写入 | 文件写入后不可修改 |
| 禁止删除 | 文件无法删除 |
| 时间锁定 | 可设置保留期限 |
| 合规审计 | 满足 WORM 合规要求 |

### 推荐保护目录

- 备份数据 `/data/backup`
- 财务数据 `/data/financial`
- 合同文档 `/data/contracts`
- 重要档案 `/data/archive`

---

## 自动响应机制

### 检测到攻击时的自动动作

| 动作 | 执行时机 | 说明 |
|------|----------|------|
| 阻断会话 | 立即 | 终止攻击者 SMB/NFS 连接 |
| 创建快照 | 立即 | 保存当前数据状态 |
| 锁定共享 | 立即 | 受影响共享变为只读 |
| IP 封锁 | 立即 | 封禁攻击者 IP |
| 发送告警 | 立即 | 邮件/Webhook 通知管理员 |

### 响应配置

```yaml
security:
  ransomware:
    auto_response:
      block_session: true      # 阻断会话
      create_snapshot: true    # 创建快照
      lock_share: true         # 锁定共享
      block_ip: true           # IP封禁
      notify_email: admin@example.com
      notify_webhook: https://hooks.example.com/alert
```

---

## 恢复流程

### 攻击发生后的恢复步骤

#### 1. 确认攻击

查看告警详情：
- 「安全管理」→ 「勒索防护」→ 「事件记录」
- 查看攻击来源 IP、受影响文件范围

#### 2. 隔离系统

```bash
# 暂停 SMB/NFS 服务
nas-os service stop smb
nas-os service stop nfs
```

#### 3. 快照恢复

「存储管理」→ 「快照」→ 选择快照 → 「回滚」

或命令行：
```bash
nas-os snapshot rollback tank/data@ransomware-protect-20260405
```

#### 4. 日志分析

查看攻击路径：
- 文件访问日志
- SMB 会话日志
- 快照创建记录

#### 5. 安全加固

- 更新检测规则
- 加强访问权限控制
- 扩大 WriteOnce 保护范围

---

## 与 TrueNAS 对比

| 功能 | nas-os | TrueNAS 26 | 说明 |
|------|:------:|:----------:|------|
| SMB 实时监控 | ✅ | ✅ | 实时分析文件操作 |
| NFS 实时监控 | ✅ | ✅ | NFS 文件访问监控 |
| 诱饵文件检测 | ✅ | ✅ | 隐藏陷阱触发告警 |
| 自动阻断会话 | ✅ | ✅ | 立即终止攻击连接 |
| 自动快照保护 | ✅ | ✅ | 检测到威胁自动快照 |
| **WriteOnce** | ✅ **独家** | ❌ | 不可变存储终极防护 |
| 快照对比检测 | ⚠️ 规划中 | ✅ | 快照空间异常检测 |
| macOS Spotlight 集成 | ❌ | ✅ SMB Spotlight | macOS 搜索集成 |

### nas-os 独家优势：WriteOnce

WriteOnce 是 TrueNAS 没有的功能：
- 数据写入后物理不可修改
- 勒索软件无法加密已写入数据
- 合规归档（金融、医疗、政务）
- 一键还原到任意时间点

---

## 最佳实践

### ✅ 推荐配置

| 配置项 | 建议 |
|--------|------|
| 监控所有共享 | SMB/NFS 全覆盖 |
| 诱饵文件分布 | 每共享至少 3 个诱饵 |
| 关键目录 WriteOnce | 备份/财务/合同 |
| 实时告警 | 邮件 + Webhook 双通道 |
| 定期测试恢复 | 每月演练快照回滚 |

### ⚠️ 注意事项

- 勒索监控盘必须 **永不休眠**（电源管理集成）
- WriteOnce 目录数据无法删除，预留足够空间
- 快照策略需与勒索防护联动
- 告警通知确保及时送达

---

## 常见问题

### Q1: WriteOnce 数据能删除吗？

不能。WriteOnce 是物理不可变存储：
- 文件写入后锁定
- 只有保留期满后自动释放
- 或管理员手动解锁（需审计审批）

### Q2: 勒索防护影响性能吗？

轻微影响：
- SMB/NFS 增加约 5% 延迟
- 文件写入需通过行为分析
- 可调整检测灵敏度平衡性能

### Q3: 监控盘为什么不能休眠？

磁盘休眠时无法：
- 实时监控文件变更
- 创建紧急保护快照
- 触发诱饵文件告警

必须确保勒索监控盘在电源管理中标记为「永不休眠」。

### Q4: 检测到攻击后用户还能访问吗？

被阻断的攻击者无法访问。合法用户：
- 共享可能暂时只读
- 快速恢复后正常访问
- 可配置「仅阻断攻击者」策略

---

## API 参考

### 查询防护状态

```bash
GET /api/v1/security/ransomware/status
```

响应：
```json
{
  "enabled": true,
  "monitored_shares": ["smb-share1", "nfs-share1"],
  "honeypot_files": 6,
  "writeonce_datasets": ["tank/backup"],
  "events_today": 0
}
```

### 手动触发快照保护

```bash
POST /api/v1/security/ransomware/emergency-snapshot
{
  "datasets": ["tank/data"]
}
```

---

## 相关文档

- [RANSOMWARE_DEFENSE.md](../RANSOMWARE_DEFENSE.md) - 勒索防护白皮书
- [DISK_SPIN_DOWN_SECURITY.md](../DISK_SPIN_DOWN_SECURITY.md) - 电源管理安全设计
- [writeonce-guide.md](../writeonce-guide.md) - WriteOnce 详细指南

---

**文档维护**: 礼部 | **对标**: TrueNAS 26 Ransomware Defense