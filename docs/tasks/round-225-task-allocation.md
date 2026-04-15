# 第225轮六部协同任务分配

> **司礼监调度**: 2026-04-13 08:56
> **当前版本**: v2.454.0
> **目标版本**: v2.455.0
> **主题**: WebShare TrueSearch预研 + SMB Spotlight Phase2 + App Pool Migration完善 + 竞品对标深化

---

## 司礼监工作汇报

### CI/CD状态（第225轮）
- ✅ 所有Actions无异常（无运行中run）
- ✅ 上轮commit e104d719已推送，CI/CD全部success
- ✅ Docker Publish成功，GitHub Release已发布v2.453.0
- ⚠️ 本地工作区有未提交改动（pool_migration.go + spotlight.go），纳入本轮处理

### 竞品调研摘要（第225轮）

**TrueNAS 26（2026年最新发布）**:
- **WebShare + TrueSearch**: 全文内容搜索+Web界面+Passkey认证 ✅ 本轮预研启动
- **Ransomware Defense**: 多重检测（蜜罐/行为/签名/快照对比）+ 自动响应 ✅ nas-os WriteOnce已领先
- **SMB Stateful Failover**: 企业HA零中断，v2.454继续对标
- **SMB Spotlight**: macOS Finder搜索 ✅ Phase1本轮完善
- **OpenZFS 2.4**: Hybrid Pool（Flash+HDD混合）+ RAIDZ Expansion ✅ 双轨已有
- **Linux Kernel 6.18 LTS**: 保持跟踪

**群晖DSM 7.3+**:
- Active Backup for Business: 整机备份 → P1规划
- Drive多设备同步: 冲突解决策略 → P1规划
- Synology Tiering: 冷热分层 → Fusion Pool对标
- Hyper Backup: 多目的地 → 已有

**飞牛fnOS**:
- FN Connect内网穿透 → ✅ FRP完成
- 按需唤醒硬盘 → ✅ 三级休眠已有
- Docker Compose支持 → 已有

---

## 六部任务分配

### 🔴 兵部（软件工程）
**负责人**: 兵部
**优先级**: P0
**工作目录**: `/home/mrafter/nas-os`

**任务1: WebShare TrueSearch预研设计**
- 学习TrueNAS 26 WebShare + TrueSearch实现
- 设计nas-os全文内容搜索架构
- 输出: `docs/research/truenas-webshare-truesearch-design.md`
- 要求:
  - 设计TrueSearch索引架构（支持文件名/内容/类型）
  - 设计Passkey认证方案
  - 设计加密数据集排除策略
  - 评估zinc/Typesense/Elasticsearch等方案

**任务2: SMB Spotlight Phase2完善**
- 文件: `internal/search/spotlight.go`
- 要求:
  - 完善中文分词支持（sego/gse）
  - 支持自然语言搜索
  - Finder实时预览优化
  - 完善测试用例
- 提交: `internal/search/spotlight.go`, `internal/search/spotlight_test.go`

**任务3: App Pool Migration完善**
- 文件: `internal/app/pool_migration.go`（注意本地已有未提交改动）
- 要求:
  - 完善迁移进度跟踪和状态机
  - 错误恢复和回滚机制
  - 健康检查完善

### 🟡 户部（财务/资源）
**负责人**: 户部
**优先级**: P1

**任务1: Drive多设备同步方案设计**
- 学习群晖Drive同步机制
- 设计nas-os多设备文件同步方案
- 输出: `docs/research/drive-sync-design.md`
- 要求:
  - 设计冲突解决策略（CRDT/LWW/手动）
  - 增量同步算法设计
  - 端到端加密设计
  - 带宽限制和压缩策略

**任务2: 版本资源统计**
- 统计v2.454.0代码资源
- 输出: 源文件数、代码行数、测试文件数
- 命令: `find . -name "*.go" -not -path "./vendor/*" | wc -l`

### 🟢 礼部（文档/品牌）
**负责人**: 礼部
**优先级**: P1

**任务1: CHANGELOG更新**
- 更新CHANGELOG.md记录v2.454.0
- 文件: `CHANGELOG.md`
- 要求: 格式符合Keep a Changelog规范，包含:
  - WebShare TrueSearch预研启动
  - SMB Spotlight Phase2进展
  - App Pool Migration完善
  - TrueNAS 26竞品对标深化

**任务2: README版本同步**
- 更新README.md版本号和下载链接
- 文件: `README.md`
- 要求: 检查所有版本号同步到v2.454.0

### 🔵 工部（DevOps/运维）
**负责人**: 工部
**优先级**: P0

**任务1: go.mod依赖检查**
- 运行 `go mod tidy && go mod verify`
- 目标: 无过期依赖、无安全漏洞引入
- 提交: go.mod, go.sum

**任务2: CI/CD Artifacts问题持续监控**
- 监控Actions运行状态
- 关注: build-artifacts job是否成功
- 修复: 如有异常立即处理

### 🟣 刑部（安全/合规）
**负责人**: 刑部
**优先级**: P1

**任务1: Ransomware Defense安全评估**
- 学习TrueNAS 26 Ransomware Defense安全机制
- 输出: `docs/research/ransomware-defense-security.md`
- 要求:
  - 蜜罐文件检测机制分析
  - 行为分析检测方法
  - 自动响应安全评估
  - 与nas-os WriteOnce对比

**任务2: govulncheck安全扫描**
- 运行 `go run golang.org/x/vuln/cmd/govulncheck ./...`
- 目标: 确保无新漏洞引入
- 提交: govulncheck报告

### 🟠 吏部（项目管理）
**负责人**: 吏部
**优先级**: P2

**任务1: v2.455.0里程碑更新**
- 更新MILESTONES.md添加v2.455.0规划
- 文件: `MILESTONES.md`
- 要求:
  - 添加M111里程碑
  - 规划TrueSearch设计Phase1
  - 规划Drive同步设计Phase1
  - 规划SMB Spotlight Phase2完成

**任务2: 六部轮值记录**
- 创建 `docs/tasks/round-225-task-allocation.md`
- 记录本轮各部任务分配

---

## 验收标准
- ✅ CI/CD全部通过
- ✅ go vet无错误
- ✅ 测试全部通过（`go test ./... -count=1`）
- ✅ CHANGELOG已更新
- ✅ 版本号已同步至v2.455.0
- ✅ GitHub已推送并发布Release

## 后续规划（v2.456.0+）
- TrueSearch Phase1开发
- Drive多设备同步Phase1开发
- SMB Stateful Failover架构预研
- Ransomware Defense功能增强
