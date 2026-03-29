# 第92轮吏部任务 - 版本管理 + Release创建

## 背景
完成v2.317.0版本发布，维护版本管理。

## 任务要求

### 1. 创建v2.317.0 Release
- 确认所有代码已合并
- 更新CHANGELOG.md
- 创建GitHub Release
- 上传构建产物

### 2. CHANGELOG更新
新增v2.317.0条目:
```markdown
## [v2.317.0] - 2026-03-30

### 新功能
- 内网穿透服务（Cloudflare Tunnel集成）
- NVMe健康监控UI看板
- 全局搜索组件（Cmd/Ctrl+K）
- 网盘挂载框架
- 勒索防护自动快照

### 优化
- 熵值分析性能提升
- CI/CD构建时间优化
- 变更追踪误报率降低

### 文档
- AI相册使用指南
- NVMe监控配置说明
- 功能对比矩阵更新
```

### 3. 里程碑进度追踪
- 更新`docs/MILESTONE_v2.320.0.md`
- 标记已完成功能
- 更新进度百分比

### 4. 版本号更新
- 更新`internal/version/version.go`
- 更新`README.md`版本显示

## 交付物
- v2.317.0 Release创建
- CHANGELOG.md更新
- 里程碑进度文档