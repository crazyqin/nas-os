# 六部任务分配 - 第66轮开发 (v2.290.0)

**日期**: 2026-03-28
**目标版本**: v2.290.0
**主题**: 应用中心框架 + AI 相册完善

---

## 兵部（软件工程）任务

### P0 - 应用中心框架
```
任务：设计并实现应用中心核心框架

具体工作：
1. internal/apps/service.go - 应用服务管理器
2. internal/apps/catalog.go - 应用目录结构
3. internal/apps/installer.go - 应用安装器
4. pkg/app/types.go - 应用类型定义
5. pkg/app/template.go - 应用模板解析

设计规范：
- 支持 Docker 容器应用
- 支持一键安装/卸载
- 支持应用配置持久化
- 支持应用状态监控
- 参考 Docker Compose 格式

目录结构：
internal/apps/
  - service.go      # 应用服务核心
  - catalog.go      # 应用目录管理
  - installer.go    # 安装/卸载逻辑
  - manager.go      # 应用生命周期
  - repository.go   # 应用仓库（本地/远程）
pkg/app/
  - types.go        # App, AppTemplate, AppStatus 类型
  - template.go     # 模板解析
  - config.go       # 应用配置
```

### P1 - 人脸识别准备
```
任务：为 AI 相册添加人脸识别基础设施

具体工作：
1. internal/ai/face_detector.go - 人脸检测接口
2. internal/ai/face_embedding.go - 人脸特征提取
3. internal/ai/face_cluster.go - 人脸聚类算法
4. internal/photos/face_service.go - 人脸相册服务

参考：
- 使用 face-recognition 或类似库
- 支持人脸分组、人脸标签
```

---

## 工部（DevOps）任务

### P0 - CI/CD Node.js 20 弃用修复
```
问题：GitHub Actions 显示 Node.js 20 已弃用警告

修复方案：
1. 更新 .github/workflows/*.yml 中的 action 版本
2. 使用支持 Node.js 24 的新版本 action：
   - actions/checkout@v5
   - actions/setup-go@v6
   - docker/build-push-action@v7
   - docker/login-action@v4
   - docker/setup-buildx-action@v4
   - docker/setup-qemu-action@v4

文件修改：
.github/workflows/ci-cd.yml
.github/workflows/docker.yml
.github/workflows/security.yml
```

### P1 - 应用中心 Docker Compose 设计
```
任务：设计应用中心的 Docker 部署方案

具体工作：
1. 创建应用中心默认应用模板：
   - templates/nginx.yml
   - templates/postgres.yml
   - templates redis.yml
   - templates/qdrant.yml (AI 向量库)

2. 设计应用 Compose 文件结构：
   apps/
     templates/
       nginx.yml
       postgres.yml
       redis.yml
       qdrant.yml
       jellyfin.yml
       plex.yml
       homeassistant.yml
     compose/           # 用户安装的应用
```

---

## 户部（财务运营）任务

### P1 - 应用中心成本报告
```
任务：应用资源使用统计

具体工作：
1. internal/reports/app_usage_report.go - 应用资源统计
2. 记录每个应用：
   - CPU 使用
   - 内存使用
   - 存储占用
   - 网络流量

3. 集成到现有成本分析模块
```

---

## 礼部（品牌营销）任务

### P0 - 应用中心 WebUI 设计
```
任务：设计应用中心用户界面

具体工作：
1. webui/pages/apps.html - 应用中心页面
2. 设计元素：
   - 应用目录浏览
   - 应用详情页
   - 安装配置表单
   - 已安装应用列表
   - 应用状态监控

3. 参考 DSM 套件中心风格
```

### P1 - 竞品分析文档完善
```
任务：完善竞品分析报告

具体工作：
1. 更新 docs/COMPETITIVE_ANALYSIS_*.md
2. 补充最新版本信息
3. 添加用户评价分析
```

---

## 刑部（法务合规）任务

### P0 - 应用中心安全审计
```
任务：应用中心安全设计

具体工作：
1. 应用权限隔离设计
2. 应用沙箱方案
3. 应用网络隔离
4. 应用数据访问控制
5. 应用安装审计日志
```

---

## 吏部（项目管理）任务

### P0 - 版本管理
```
任务：版本号更新、文档同步

具体工作：
1. VERSION 文件更新
2. ROADMAP.md 更新
3. MILESTONES.md 更新
4. CHANGELOG.md 更新
```

---

## 提交规范

- 提交格式：`feat(module): 描述`
- 分支策略：直接提交 master
- 测试要求：新增代码需有测试覆盖
- 文档要求：API 变更需更新文档

---

**司礼监协调**：各部门完成后向司礼监汇报，司礼监统一合并提交。