# 六部任务分配 - 第68轮开发 (v2.291.0)

**日期**: 2026-03-28
**目标版本**: v2.291.0
**主题**: 应用中心完善 + 人脸识别开发启动

---

## 竞品学习要点（本轮重点）

### 群晖 DSM 7.3 学习
- **套件中心设计**: 应用分类、一键安装、状态监控
- **Photos应用**: 人脸识别、智能相册、场景分类
- **Synology Tiering**: 热数据SSD/冷数据HDD自动分层（已实现Fusion Pool）

### 飞牛fnOS 1.1 学习
- **应用中心改版**: 分类筛选+搜索+一键安装（已实现）
- **本地AI人脸识别**: Intel核显加速（本轮开发）
- **网盘刮削直链播放**: STRM支持（规划中）
- **QWRT软路由集成**: NAS一键软路由（规划中）

---

## 兵部（软件工程）任务

### P0 - 人脸识别核心开发
```
任务：实现本地人脸识别功能

具体工作：
1. internal/ai/face_detector.go - 人脸检测服务
   - 调用CLIP或其他模型检测人脸
   - 返回人脸坐标和特征向量

2. internal/ai/face_embedding.go - 人脸特征提取
   - 提取128维/512维人脸特征向量
   - 存储到Qdrant向量库

3. internal/ai/face_cluster.go - 人脸聚类算法
   - 相似人脸自动分组
   - 支持人工标注人名

4. internal/photos/face_service.go - 人脸相册服务
   - 人脸相册API
   - 人脸标签管理
   - 人物相册生成

技术选型：
- 参考 fnOS 使用 face-recognition 或 insightface
- 支持 Intel GPU 加速（核显）
- 纯CPU fallback模式

参考：
- docs/COMPETITOR_ANALYSIS.md - fnOS人脸识别章节
- 群晖 Photos 人脸相册设计
```

### P1 - 应用中心模板扩展
```
任务：添加更多应用模板

具体工作：
apps/templates/ 新增：
- jellyfin.yml    # 媒体服务器
- plex.yml        # 媒体服务器
- homeassistant.yml  # 智能家居
- transmission.yml   # 下载器
- nextcloud.yml      # 私有云盘
- vaultwarden.yml    # 密码管理
```

---

## 工部（DevOps）任务

### P0 - CI/CD Node.js 20 弃用修复
```
任务：修复 GitHub Actions Node.js 弃用警告

修改文件：
.github/workflows/ci-cd.yml
.github/workflows/docker.yml

更新 action 版本：
- actions/checkout@v4 → v5
- actions/setup-go@v5 → v6
- docker/build-push-action@v6 → v7
- docker/login-action@v3 → v4
- docker/setup-buildx-action@v3 → v4
- docker/setup-qemu-action@v3 → v4

验证：
- 本地 act 测试或 GitHub Actions 测试
```

### P1 - 应用模板验证
```
任务：验证所有应用模板可用

具体工作：
1. 测试 apps/templates/ 下所有模板
2. 确保 docker-compose config 验证通过
3. 记录测试结果到文档
```

---

## 户部（财务运营）任务

### P1 - 应用资源统计完善
```
任务：完善应用资源统计功能

具体工作：
1. internal/reports/app_usage.go - 完善统计逻辑
2. 添加：
   - 应用启动时间统计
   - 应用存储增量统计
   - 应用网络IO统计

3. API 端点完善：
   GET /api/v1/apps/{id}/stats
```

---

## 礼部（品牌营销）任务

### P1 - 应用中心文档
```
任务：编写应用中心用户文档

具体工作：
1. docs/user-guide/app-center.md - 用户指南
   - 如何浏览应用目录
   - 如何安装应用
   - 如何配置应用
   - 如何管理已安装应用

2. docs/user-guide/templates.md - 应用模板说明
   - 各应用模板用途说明
   - 推荐配置参数
```

---

## 刑部（法务合规）任务

### P0 - 人脸识别隐私合规
```
任务：人脸识别隐私合规设计

具体工作：
1. docs/privacy/face-recognition-privacy.md
   - 人脸数据存储说明
   - 用户知情同意机制
   - 人脸数据删除流程

2. internal/ai/face_privacy.go
   - 人脸数据加密存储
   - 人脸数据删除API
   - 人脸数据导出功能

合规要点：
- 人脸数据仅存本地
- 用户可随时删除
- 提供数据导出功能
- 不与外部服务共享
```

---

## 吏部（项目管理）任务

### P0 - 版本管理
```
任务：版本号更新、文档同步

具体工作：
1. VERSION 文件更新 v2.290.0 → v2.291.0
2. internal/version/version.go 更新
3. ROADMAP.md 更新
4. MILESTONES.md 更新
5. CHANGELOG.md 更新

本轮里程碑：
- 应用中心完善
- 人脸识别开发启动
- CI/CD 弃用警告修复
```

---

## 提交规范

- 提交格式：`feat(module): 描述`
- 分支策略：直接提交 master
- 测试要求：新增代码需有测试覆盖
- 文档要求：API 变更需更新文档

---

**司礼监协调**：各部门完成后向司礼监汇报，司礼监统一合并提交 GitHub。