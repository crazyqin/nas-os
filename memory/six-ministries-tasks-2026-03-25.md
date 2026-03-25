# 六部任务分配 - v2.278.0

## 当前状态
- **版本**: v2.277.0
- **日期**: 2026-03-25
- **Actions状态**: Docker Publish运行中，CI/CD失败（noctx问题），GitHub Release失败（编译错误已修复）

---

## 🔴 P0 紧急任务

### 兵部（软件工程）- noctx修复
**任务**: 修复golangci-lint noctx错误（共51处）
**优先级**: 按模块分批处理

| 模块 | 文件 | 错误数 | 状态 |
|------|------|--------|------|
| backup | smart_manager_v2.go, sync.go | 2 | 待处理 |
| cloudsync | provider_baidu.go | 1 | 待处理 |
| cluster | loadbalancer.go | 1 | 待处理 |
| container | container.go | 6 | 待处理 |
| database | optimizer.go | 14 | 待处理 |
| dashboard | widgets.go | 1 | 待处理 |
| docker | app_version.go, jellyfin.go | 4 | 待处理 |
| ftp | server.go | 4 | 待处理 |
| network | ddns.go, ddns_providers.go, diagnostics.go | 4 | 待处理 |
| notification | channels.go | 4 | 待处理 |
| office | manager.go | 2 | 待处理 |
| photos | ai.go | 2 | 待处理 |

**修复模式**:
- `exec.Command` → `exec.CommandContext`
- `http.NewRequest` → `http.NewRequestWithContext`
- `client.Get/Post` → `client.Do(req)` + context
- `db.Exec/Query` → `db.ExecContext/QueryContext`

### 刑部（安全审计）
**任务**: 安全扫描跟进
- 检查gosec报告
- 审核noctx修复的安全性
- 状态: 待分配

### 工部（DevOps）
**任务**: Actions状态监控
- [x] 编译错误已修复
- [ ] 监控Docker Publish完成
- [ ] 重新触发CI/CD
- 状态: 进行中

---

## 竞品研究总结 (Exa搜索 2026-03-25)

### 🚀 飞牛fnOS 最新动态 (2026-01)
- **v1.1.15 更新**:
  - FN Connect 远程WebDAV访问
  - DDNS服务商扩展（ChangeIP, DynDNS, No-IP等）
  - 原生迅雷上线
  - 影视直链与代理播放优化
- **核心优势**: 免费国产NAS，FN Connect内网穿透，网盘挂载

### 📊 群晖DSM 7.3 (2025-10)
- **Synology Tiering**: 冷热数据分层，SSD Cache优化
- **AI Console**: 数据脱敏，敏感信息标记（43万+部署）
- **Drive 4.0**: 共享标签、文件请求、文件锁定
- **硬盘限制解除**: Plus/Value/J系列支持第三方硬盘

### 🐟 TrueNAS Fangtooth 25.04.2 (2026-02)
- **Virtualization**: Electric Eel虚拟化恢复
- **Secure Boot**: Windows 11 VM TPM支持
- **Fast Clone**: Veeam SMB快克隆
- **App IP**: 应用独立IP分配
- **RAIDZ扩展**: 5X加速
- **Fast Dedup**: 快速去重
- **用户数**: 100,000+ 采用

---

## 差距分析

| 功能 | nas-os状态 | 竞品状态 | 优先级 |
|------|-----------|---------|--------|
| 网盘挂载 | 🔲 规划中 | fnOS已实现 | P0 |
| 内网穿透 | 🔲 开发中 | fnOS FN Connect | P0 |
| 冷热分层 | ✅ 已实现 | DSM Tiering | P1 |
| AI脱敏 | ✅ 已实现 | DSM AI Console | P1 |
| RAIDZ扩展 | 🔲 规划中 | TrueNAS 5X加速 | P1 |
| Fast Dedup | 🔲 规划中 | TrueNAS已实现 | P2 |
| 虚拟化 | 🔲 规划中 | TrueNAS/LXC/KVM | P2 |

---

## 执行计划

### 第一轮: 编译修复 ✅
- [x] 修复ARM架构int32->int64类型转换

### 第二轮: noctx修复（进行中）
- [ ] 兵部处理51个noctx错误
- [ ] 刑部审核安全性
- [ ] 工部重新触发CI/CD

### 第三轮: 版本发布
- [ ] 提交修复
- [ ] 触发CI/CD
- [ ] 发布v2.278.0

---

## 下一步
1. 兵部开始处理noctx问题
2. 等待Docker Publish完成
3. 重新触发CI/CD
4. 发布v2.278.0

---

**司礼监**
2026-03-25 11:31