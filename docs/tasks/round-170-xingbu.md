# 刑部：第170轮安全审计报告

> **时间**: 2026-04-05
> **部门**: 刑部（安全合规）

---

## 🔴 严重安全问题

### OKX API 密钥泄露
- **文件位置**: `/home/mrafter/clawd/okx_data/config.json`
- **内容**: 真实 apiKey、secretKey、passphrase
- **建议**: 立即删除文件并轮换密钥

---

## 🟡 中等问题

- `k8s/namespace.yaml` 示例密钥未替换（生产部署风险）
- `govulncheck` 工具未安装

---

## ✅ 安全验证通过

### WriteOnce 勒索防护
- ✅ 基于 btrfs 只读快照，WORM 存储合规
- ✅ 支持 7天/30天/永久锁定
- ✅ `chattr +i` 双重保护

### 勒索检测模块 (`internal/security/ransomware/`)
- ✅ `detector.go` - 已知勒索扩展名检测、熵值检测、勒索信检测
- ✅ `tracker.go` - 滑动窗口变化追踪、扩展名级联检测
- ✅ `entropy.go` - Shannon熵计算、压缩比率测试

### 告警联动
- ✅ Discord Webhook 配置
- ⚠️ 缺少自动阻断机制

---

## 建议行动

1. **立即删除** `/home/mrafter/clawd/okx_data/config.json`
2. **轮换** OKX API 密钥
3. 安装 `govulncheck` 进行持续漏洞扫描

---

*刑部审计完成*