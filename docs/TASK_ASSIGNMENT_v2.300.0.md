# 六部任务分配 - v2.300.0

> 版本: v2.300.0 | 日期: 2026-03-29 | 编制: 司礼监

## 🎯 本轮重点（竞品对标深化）

基于竞品分析（群晖DSM 7.3/TrueNAS 25.10），本轮聚焦：

| 功能 | 对标竞品 | 优先级 | 负责部门 |
|------|----------|--------|----------|
| AI服务正式版完善 | 群晖AI Console/TrueNAS | P0 | 兵部 |
| 本地LLM一键部署 | 群晖DSM/Ollama | P0 | 工部 |
| 模型管理UI | 群晖DSM 7.3 | P1 | 礼部 |
| GPU调度优化 | TrueNAS/飞牛fnOS | P1 | 工部 |
| 条件相册实现 | 群晖Synology Photos | P1 | 兵部 |
| AI人脸识别服务 | 飞牛fnOS/群晖 | P0 | 兵部 |
| 审计日志增强 | TrueNAS Enterprise | P1 | 刑部 |
| Token计费接口 | 企业需求 | P2 | 户部 |

---

## 🔧 六部任务详情

### 兵部（软件工程）

**任务1: 条件相册核心实现**
- 实现基于人物/对象/地点/时间的自动相册生成
- 参考: 群晖 Synology Photos 条件相册
- 位置: `internal/photos/conditional_album.go`

**任务2: AI人脸识别服务**
- 人脸检测与识别流水线
- 人脸聚类与人物管理
- 参考: 飞牛fnOS Intel核显加速
- 位置: `internal/ai/face/`

**任务3: Ollama集成框架**
- 本地LLM一键部署接口设计
- 模型下载与管理
- 位置: `internal/ai/ollama/`

**交付物**:
- `internal/photos/conditional_album.go`
- `internal/photos/album_rule.go`
- `internal/ai/face/detector.go`
- `internal/ai/face/cluster.go`
- `internal/ai/ollama/client.go`

---

### 工部（DevOps）

**任务1: GPU调度优化**
- GPU资源分配策略
- 多任务GPU调度
- Intel核显/AMD/NVIDIA支持

**任务2: 本地LLM部署脚本**
- Ollama Docker部署模板
- GPU Passthrough配置
- 自动化安装脚本

**任务3: CI优化验证**
- 确保CI/CD稳定运行
- 构建缓存优化

**交付物**:
- `internal/ai/gpu/scheduler.go`
- `deploy/ollama/docker-compose.yml`
- `scripts/install-ollama.sh`
- `.github/workflows/` 检查

---

### 礼部（品牌营销）

**任务1: 模型管理UI设计**
- 模型列表、下载进度、版本管理
- GPU状态可视化
- 参考群晖DSM 7.3模型管理

**任务2: 条件相册UI**
- 规则创建界面（人物/时间/地点）
- 相册预览与编辑

**任务3: 文档更新**
- CHANGELOG.md v2.300.0
- README.md AI服务正式版说明
- ROADMAP.md 更新

**交付物**:
- `webui/pages/ai-models.html`
- `webui/pages/conditional-album.html`
- `CHANGELOG.md` 更新
- `README.md` 更新

---

### 刑部（法务合规）

**任务1: 审计日志增强**
- AI服务调用审计
- 人脸识别隐私审计
- 操作日志完整记录

**任务2: 多用户隔离验证**
- AI服务用户隔离
- 模型访问权限控制

**交付物**:
- `internal/audit/ai_audit.go`
- `SECURITY_AUDIT.md` 更新

---

### 户部（财务预算）

**任务1: Token计费接口**
- API调用计费统计
- Token消耗记录
- 成本分摊计算

**任务2: AI服务成本分析更新**
- GPU资源成本
- 模型推理成本

**交付物**:
- `internal/billing/token_counter.go`
- `reports/ai-cost-v2.300.0.md`

---

### 吏部（项目管理）

**任务1: 版本管理**
- VERSION 更新为 v2.300.0
- MILESTONES.md 更新

**任务2: 任务协调**
- 收集各部门进度
- 整合提交
- GitHub Release发布

**交付物**:
- `VERSION` 更新 ✅
- `internal/version/version.go` 更新
- `MILESTONES.md` 更新
- `CHANGELOG.md` 更新
- GitHub Release

---

## 📊 竞品学习成果（Q1 2026总结）

### 群晖 DSM 7.3
- ✅ Synology Tiering: 热冷数据自动分层（已实现Fusion Pool）
- ✅ AI Console: 数据脱敏（已实现Data Mask）
- ✅ Synology Photos: 条件相册（本轮实现）
- ✅ 本地LLM支持（本轮实现）
- ✅ Drive 4.0: 文件锁定、共享标签
- ✅ 多应用生态: Photos/Audio Station/Drive/Office/Chat/MailPlus

### TrueNAS Scale 25.10
- ✅ 勒索软件检测与防护（已实现）
- ✅ Docker Compose原生支持
- ✅ RAIDZ Expansion（待研究）
- ✅ Fast Dedup快速去重
- ✅ 多系统管理: TrueCommand/Cloud
- ✅ SSO/RBAC/Auditing（已实现基础）
- ✅ GPU Sharing（本轮优化）
- ✅ 多平台支持: Apps/VMs/LXC

### 飞牛 fnOS 1.1
- ✅ 应用中心改版（已实现基础）
- ✅ 网盘原生挂载（框架已设计）
- ✅ 本地AI人脸识别+Intel核显加速（本轮实现）
- ✅ Cloudflare Tunnel（已实现框架）
- ✅ QWRT软路由集成

---

## 🎯 nas-os差异化优势

### 独家/领先功能
1. **WriteOnce不可变存储** - 勒索病毒防护/合规归档
2. **Fusion Pool智能分层** - 热冷数据自动分层
3. **勒索软件检测器** - 主动威胁检测
4. **AI数据脱敏** - 训练数据隐私保护
5. **OpenAI兼容API** - 一套API对接多提供商
6. **内网穿透服务** - FRP/WireGuard框架

### 本轮新增
7. **条件相册** - 智能相册自动生成
8. **本地LLM一键部署** - Ollama集成
9. **GPU智能调度** - 多任务资源优化

---

## ✅ 交付标准

1. 代码通过 golangci-lint
2. 单元测试覆盖核心功能
3. 文档同步更新
4. VERSION 正确递增 (v2.300.0)
5. GitHub Release 发布

---

*司礼监签发*
*2026-03-29 01:53*