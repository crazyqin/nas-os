# HDD去重压缩引擎 (hdddedup)

对标群晖 DSM 7.4 的 HDD 级后处理去重与压缩功能，释放存储空间而不牺牲系统性能。

## 功能特性

### PostProcessDedup - 后处理去重
- 基于内容哈希的数据块去重
- 可配置的块大小（4KB-1MB）
- 异步后台处理，不影响前台IO
- 去重率统计与报告

### SmartCompression - 智能压缩
- 多种压缩算法支持（LZ4、ZSTD、GZIP）
- 基于文件类型的智能压缩策略
- 压缩率与性能平衡
- 可配置压缩级别

### ScheduleManager - 调度管理
- 灵活的去重压缩调度策略
- 按时间窗口执行（如凌晨低峰期）
- 按存储池/目录配置不同策略
- 资源使用限制（CPU/IO限流）

### EfficiencyReport - 效率报告
- 存储节省统计
- 去重率与压缩率分析
- 历史趋势图表数据
- 优化建议生成

## 目录结构

```
internal/hdddedup/
├── hdddedup.go      # 核心去重引擎
├── compress.go      # 压缩算法实现
├── scheduler.go     # 调度管理
├── report.go        # 效率报告
├── handler.go       # HTTP API 处理器
├── types.go         # 类型定义
└── hdddedup_test.go # 单元测试
```

## API 端点

### 去重管理
- `POST /hdddedup/jobs` - 创建去重任务
- `GET /hdddedup/jobs` - 列出任务
- `GET /hdddedup/jobs/:id` - 获取任务状态
- `DELETE /hdddedup/jobs/:id` - 取消任务

### 压缩配置
- `GET /hdddedup/compress/config` - 获取压缩配置
- `PUT /hdddedup/compress/config` - 更新压缩配置
- `GET /hdddedup/compress/policies` - 列出压缩策略
- `POST /hdddedup/compress/policies` - 创建压缩策略

### 调度管理
- `GET /hdddedup/schedules` - 列出调度
- `POST /hdddedup/schedules` - 创建调度
- `PUT /hdddedup/schedules/:id` - 更新调度
- `DELETE /hdddedup/schedules/:id` - 删除调度

### 效率报告
- `GET /hdddedup/reports/efficiency` - 获取效率报告
- `GET /hdddedup/reports/history` - 历史报告
- `GET /hdddedup/reports/savings` - 节省统计

## 使用示例

```go
// 创建去重压缩引擎
config := &HDDDedupConfig{
    ChunkSize:       64 * 1024, // 64KB
    CompressAlgo:    "zstd",
    CompressLevel:   3,
    MaxConcurrency:  4,
    ScheduleEnabled: true,
}
engine := NewHDDDedupEngine(config)

// 启动引擎
engine.Start()
defer engine.Stop()

// 创建去重任务
job, err := engine.CreateDedupJob("/data/pool1")
if err != nil {
    log.Fatal(err)
}

// 等待完成并获取报告
report := engine.GetEfficiencyReport()
fmt.Printf("去重率: %.2f%%, 压缩率: %.2f%%\n", report.DedupRatio, report.CompressRatio)
```

## 测试

```bash
cd nas-os
go test ./internal/hdddedup/... -v
```
