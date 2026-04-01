# 六部任务分配 - v2.293.0

**发布日期**: 2026-03-28
**主题**: 竞品学习、人脸识别增强、应用生态优化

---

## 竞品学习要点

### 群晖 Synology Photos
- ✅ 条件相册（按人物/对象/地点/镜头自动生成）
- ✅ 安全分享（链接密码、过期时间、Shared Space协作）
- ✅ RAW支持、Apple TV/Android TV支持
- ✅ Image Assistant桌面端转码

### 飞牛fnOS 1.1
- ✅ 网盘原生挂载（115/夸克/百度/阿里云盘）
- ✅ 本地AI人脸识别（Intel核显加速）
- ✅ QWRT软路由一体化
- ✅ Cloudflare Tunnel远程访问
- ✅ 应用中心改版

### TrueNAS 24.10
- ✅ Docker Compose原生支持
- ✅ RAIDZ单盘扩展
- ✅ 勒索软件检测（v26）

---

## 六部任务分配

### 🔴 兵部（软件工程）

#### 任务1: 人脸识别条件相册
- 实现按人物/对象/地点自动生成相册
- 参考群晖Synology Photos设计
- 文件: `internal/ai/face/conditional_album.go`

#### 任务2: Intel核显加速支持
- 研究OpenVINO/VAAPI集成
- 优化人脸检测性能
- 文件: `internal/ai/face/gpu_accel.go`

#### 任务3: HEIC/HEVC转码服务
- 支持iOS照片格式
- 服务端转码API
- 文件: `internal/media/transcode.go`

---

### 🟡 礼部（品牌营销）

#### 任务1: 人脸识别WebUI优化
- 人物卡片展示
- 人脸命名流程
- 合并确认界面
- 文件: `webui/pages/ai-photos.html`

#### 任务2: 应用中心UI优化
- 分类筛选优化
- 搜索功能增强
- 一键安装流程
- 文件: `webui/pages/apps.html`

#### 任务3: 文档更新
- CHANGELOG更新
- README功能亮点更新

---

### 🟢 工部（DevOps）

#### 任务1: Cloudflare Tunnel集成
- 研究cloudflared集成方案
- 无需开放端口远程访问
- 文件: `internal/tunnel/cloudflare.go`

#### 任务2: RAIDZ扩展研究
- 评估ZFS RAIDZ扩展可行性
- 技术方案设计
- 文件: `docs/RAIDZ_EXPANSION_DESIGN.md`

#### 任务3: CI/CD优化
- 构建时间优化
- 多架构镜像验证

---

### 🔵 刑部（安全合规）

#### 任务1: 安全分享功能
- 链接密码保护
- 过期时间设置
- 访问日志记录
- 文件: `internal/shares/secure_share.go`

#### 任务2: 人脸隐私合规增强
- 知情同意确认流程
- 数据导出API
- 删除审计日志
- 文件: `internal/ai/face/privacy_enhanced.go`

---

### 🟠 户部（财务运营）

#### 任务1: AI服务成本分析
- 人脸识别GPU成本评估
- 向量存储成本分析
- 用户计费模型设计
- 文件: `internal/billing/ai_cost.go`

#### 任务2: 存储成本优化建议
- 基于使用量的成本报告
- 节省建议生成

---

### 🟣 吏部（项目管理）

#### 任务1: 版本规划v2.293.0
- 里程碑定义
- 任务优先级排序
- 文件: `ROADMAP.md`

#### 任务2: 六部协调
- 进度跟踪
- 阻塞问题解决
- 发布时间协调

---

## 里程碑节点

| 节点 | 时间 | 目标 |
|------|------|------|
| M1 | 03-28 18:00 | 任务启动，各部开始开发 |
| M2 | 03-29 12:00 | 核心功能开发完成 |
| M3 | 03-29 18:00 | 测试完成，文档更新 |
| M4 | 03-30 10:00 | 发布准备 |
| M5 | 03-30 14:00 | v2.293.0发布 |

---

## 质量目标

| 指标 | 目标值 |
|------|--------|
| 测试覆盖率 | ≥38% |
| go vet错误 | 0 |
| 安全漏洞(高危) | 0新增 |
| 文档覆盖率 | 85% |

---

**制定时间**: 2026-03-28 14:53
**制定人**: 司礼监