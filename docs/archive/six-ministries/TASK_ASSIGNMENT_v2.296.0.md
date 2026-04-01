# 六部任务分配 - v2.296.0

> 版本: v2.296.0 | 日期: 2026-03-28 | 编制: 司礼监

## 🎯 本轮重点（竞品对标）

基于 Q1 2026 竞品分析，本轮聚焦：

| 功能 | 对标竞品 | 优先级 | 负责部门 |
|------|----------|--------|----------|
| 内网穿透服务 | 飞牛FN Connect / 群晖QuickConnect | P1 | 工部 |
| 数据遮罩(AI训练脱敏) | 群晖AI Console | P1 | 刑部 |
| Dashboard Widget增强 | 群晖DSM / TrueNAS | P2 | 兵部 |
| 网盘原生挂载-阿里云盘 | 飞牛fnOS | P2 | 兵部 |
| 应用中心-一键安装优化 | 飞牛fnOS | P2 | 礼部 |
| 成本分析看板 | 企业需求 | P2 | 户部 |

---

## 🔧 六部任务详情

### 兵部（软件工程）

**任务1: Dashboard Widget系统增强**
- 参考 TrueNAS/群晖 Dashboard 可定制化
- 实现 widget 拖拽排序、用户自定义布局
- 新增 widget: 系统负载、存储IO、网络流量、告警汇总
- 位置: `webui/pages/dashboard.html`, `internal/dashboard/`

**任务2: 网盘原生挂载框架**
- 设计网盘挂载抽象层（阿里云盘、百度网盘）
- 实现 WebDAV/FUSE 挂载接口
- 位置: `internal/cloudmount/`

**交付物**:
- `internal/dashboard/widget_manager.go`
- `internal/dashboard/widget_layout.go`
- `internal/cloudmount/provider.go`
- `internal/cloudmount/alipan.go`（框架）
- `webui/pages/dashboard.html` 更新

---

### 工部（DevOps）

**任务1: 内网穿透服务设计**
- FRP/WireGuard 集成方案
- 配置模板、自动部署脚本
- 远程访问安全策略

**任务2: CI优化**
- 确保 Actions 运行稳定
- 构建缓存优化

**交付物**:
- `internal/tunnel/frp.go`
- `internal/tunnel/wireguard.go`
- `deploy/frp/` 配置模板
- `.github/workflows/` 优化

---

### 礼部（品牌营销）

**任务1: 应用中心UI优化**
- 一键安装流程简化
- 应用分类视觉改进
- Toast通知系统完善

**任务2: 文档更新**
- CHANGELOG.md v2.296.0
- 竞品分析更新（群晖DSM应用）
- README.md 功能列表更新

**交付物**:
- `webui/pages/apps.html` 优化
- `CHANGELOG.md` 更新
- `docs/COMPETITOR_ANALYSIS_Q1_2026.md` 补充
- `README.md` 更新

---

### 刑部（法务合规）

**任务1: 数据遮罩模块**
- AI训练数据脱敏
- PII识别框架（姓名、身份证、电话）
- 脱敏策略：替换、遮蔽、假名化
- 合规审计日志

**任务2: 勒索软件检测完善**
- 补充单元测试
- 安全审计文档更新

**交付物**:
- `internal/security/datamask/detector.go`
- `internal/security/datamask/pii.go`
- `internal/security/datamask/audit.go`
- `internal/security/ransomware/detector_test.go`
- `SECURITY_AUDIT.md` 更新

---

### 户部（财务预算）

**任务1: 成本分析看板**
- 存储成本分析
- 电费估算（基于功率监控）
- 成本趋势图表

**交付物**:
- `internal/cost/dashboard.go`
- `webui/pages/cost-analysis.html`

---

### 吏部（项目管理）

**任务1: 版本管理**
- VERSION 更新为 v2.296.0
- MILESTONES.md 更新

**任务2: 任务协调**
- 收集各部门进度
- 整合提交

**交付物**:
- `VERSION` 更新
- `MILESTONES.md` 更新
- 任务协调报告

---

## 📅 时间安排

| 阶段 | 时间 | 内容 |
|------|------|------|
| 开发 | 22:00-23:00 | 六部并行开发 |
| 整合 | 23:00-23:15 | 司礼监整合提交 |
| 发布 | 23:15 | GitHub Release |

---

## ✅ 交付标准

1. 代码通过 golangci-lint
2. 单元测试覆盖核心功能
3. 文档同步更新
4. VERSION 正确递增

---

*司礼监签发*