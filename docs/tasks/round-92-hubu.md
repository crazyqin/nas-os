# 第92轮户部任务 - NVMe监控UI + 全局搜索UI

## 背景
完善前端界面，展示NVMe健康监控和全局搜索功能。

## 任务要求

### 1. NVMe健康看板组件
- 文件位置: `web/src/components/NVMeDashboard.tsx`
- 功能:
  - 显示所有NVMe设备列表
  - 温度、健康度、寿命图表
  - SMART属性详情
  - 预警阈值配置

### 2. 全局搜索框组件
- 文件位置: `web/src/components/GlobalSearch.tsx`
- 功能:
  - Cmd/Ctrl+K 快捷键唤起
  - 模糊搜索文件、用户、设置
  - 分类展示搜索结果
  - 键盘导航支持

### 3. API集成
- NVMe: GET `/api/v1/hardware/nvme`
- 搜索: GET `/api/v1/search?q={query}&type={type}`

### 4. UI规范
- 使用React + TypeScript
- Tailwind CSS样式
- 响应式设计
- 暗色主题支持

## 交付物
- `web/src/components/NVMeDashboard.tsx`
- `web/src/components/GlobalSearch.tsx`
- `web/src/hooks/useNVMe.ts`
- `web/src/hooks/useSearch.ts`