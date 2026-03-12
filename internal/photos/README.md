# AI 相册功能文档

## 概述

AI 相册模块为 NAS-OS 提供智能照片管理功能，包括人脸识别、场景分类、物体检测、智能相册自动生成等。

## 核心功能

### 1. 人脸识别
- 自动检测照片中的人脸
- 人物聚类（相同人脸归为同一人）
- 支持人物命名和管理
- 按人物筛选照片

### 2. 场景识别
- 自动识别照片场景类型
- 支持的场景：风景、人像、食物、宠物、文档、室内、室外、夜景、日落、海滩、山脉、城市等
- 按场景分类浏览

### 3. 物体识别
- 检测照片中的物体
- 支持的物体：车辆、建筑、植物、动物等
- 按物体搜索照片

### 4. 智能分类
- 自动按人物分组
- 按场景分组
- 按地点分组（基于 GPS）
- 按时间分组

### 5. 相似照片检测
- 基于场景、物体、颜色、时间计算相似度
- 查找相似照片
- 检测重复照片

### 6. 记忆回顾
- 历史上的今天
- 年度回忆
- 自动生成回忆相册

## 技术架构

### 本地 AI 引擎
- 优先使用本地模型（离线可用）
- 支持 CPU 推理（兼容 ARM 设备）
- 颜色提取（内置实现）

### 云端 AI 引擎（可选）
- 支持接入云端 API（更高精度）
- 可配置 API 密钥
- 支持 Azure Face API、AWS Rekognition 等

### 异步处理
- 后台任务队列
- 不阻塞照片上传
- 进度显示和任务管理

## API 接口

### AI 分析
```
POST /api/v1/photos/ai/analyze/:photoId    # 分析单张照片
POST /api/v1/photos/ai/analyze/batch       # 批量分析照片
GET  /api/v1/photos/ai/stats               # 获取 AI 统计
GET  /api/v1/photos/ai/tasks               # 列出 AI 任务
```

### 智能相册
```
GET  /api/v1/photos/ai/smart-albums        # 列出智能相册
POST /api/v1/photos/ai/smart-albums        # 创建智能相册
DELETE /api/v1/photos/ai/smart-albums/:id  # 删除智能相册
```

### 回忆
```
GET /api/v1/photos/ai/memories             # 获取回忆列表
```

### 数据管理
```
POST /api/v1/photos/ai/reanalyze           # 重新分析所有照片
POST /api/v1/photos/ai/clear               # 清除 AI 数据
```

## Web UI

访问 `/pages/ai-photos.html` 使用 AI 相册功能。

### 功能标签页
- **概览**: 显示统计信息和快速操作
- **人物**: 浏览识别的人物
- **场景**: 按场景分类查看照片
- **智能相册**: 管理自动生成的相册
- **回忆**: 查看历史上的今天
- **任务**: 监控 AI 处理任务进度
- **设置**: 配置 AI 功能开关

## 配置

### 启用/禁用 AI 功能
在相册设置中可以开关以下功能：
- 人脸识别
- 场景分类
- 物体检测

### 云端 AI 配置
如需更高精度，可在设置中配置云端 AI：
1. 启用"使用云端 AI 引擎"
2. 输入 API Key
3. 配置 API Endpoint

## 性能优化

### CPU 优化
- 使用轻量级模型
- 支持 ARM NEON 指令集
- 批量处理优化

### 内存优化
- 流式处理大图片
- 智能缓存机制
- 定期清理内存

### 存储优化
- AI 数据增量保存
- 压缩存储格式
- 按需加载

## 数据结构

### AIClassification
```go
type AIClassification struct {
    PhotoID    string
    Faces      []FaceInfo
    Objects    []string
    Scene      string
    Colors     []string
    IsNSFW     bool
    Confidence float32
    Metadata   map[string]interface{}
}
```

### SmartAlbum
```go
type SmartAlbum struct {
    ID         string
    Name       string
    Type       string  // person, scene, object, location, time
    Criteria   map[string]interface{}
    PhotoIDs   []string
    AutoUpdate bool
}
```

## 使用示例

### 批量分析照片
```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/analyze/batch
```

### 创建智能相册
```bash
curl -X POST http://localhost:8080/api/v1/photos/ai/smart-albums \
  -H "Content-Type: application/json" \
  -d '{
    "name": "家人照片",
    "type": "person",
    "criteria": {"person": "张三"}
  }'
```

### 获取 AI 统计
```bash
curl http://localhost:8080/api/v1/photos/ai/stats
```

## 注意事项

1. **首次分析**: 大量照片首次分析需要较长时间，建议在空闲时进行
2. **存储空间**: AI 数据会占用额外存储空间（约照片总数的 5-10%）
3. **隐私保护**: 人脸数据本地存储，不会上传到云端（除非启用云端 AI）
4. **ARM 设备**: 在 ARM 设备上建议使用本地轻量模型

## 未来计划

- [ ] 集成 go-face 实现本地人脸识别
- [ ] 集成 MobileNet 实现场景分类
- [ ] 集成 YOLO 实现物体检测
- [ ] 支持更多云端 AI 提供商
- [ ] 照片质量评分
- [ ] 自动标签生成
- [ ] 照片故事自动生成

## 故障排除

### AI 分析任务卡住
检查任务队列，重启 AI Manager：
```bash
# 查看任务状态
curl http://localhost:8080/api/v1/photos/ai/tasks

# 清除卡住的任务
curl -X POST http://localhost:8080/api/v1/photos/ai/clear
```

### 人脸识别不准确
- 确保照片质量良好
- 尝试启用云端 AI（更高精度）
- 手动校正人物名称

### 性能问题
- 减少并发任务数
- 在设置中降低分析频率
- 使用云端 AI 分担本地压力
