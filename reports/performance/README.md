# 性能报告目录

此目录用于存放 NAS-OS 性能测试和分析报告。

## 目录结构

```
reports/performance/
├── heap_YYYYMMDD_HHMMSS.prof    # 堆内存 Profile
├── heap_YYYYMMDD_HHMMSS.svg     # 堆内存可视化
├── goroutine_*.prof              # Goroutine Profile
├── allocs_*.prof                 # 内存分配 Profile
├── cpu_*.prof                    # CPU Profile
├── bench_mem_*.txt               # 基准测试输出
└── memory_report_*.md            # 完整内存分析报告
```

## 如何运行基准测试

### 运行所有基准测试

```bash
# 运行所有基准测试
go test -bench=. ./tests/benchmark/...

# 运行并显示内存分配
go test -bench=. -benchmem ./tests/benchmark/...

# 运行特定测试
go test -bench=BenchmarkAPI ./tests/benchmark/...
```

### API 响应时间基准测试

```bash
# 测试 API 端点响应时间
go test -bench=BenchmarkAPI_ ./tests/benchmark/...

# 并发测试
go test -bench=BenchmarkAPI_Concurrent ./tests/benchmark/...
```

### 存储操作基准测试

```bash
# 存储操作性能
go test -bench=BenchmarkStorage_ ./tests/benchmark/...
```

### 文件上传基准测试

```bash
# 文件上传性能
go test -bench=BenchmarkAPI_FileUpload ./tests/benchmark/...
```

### 内存分配基准测试

```bash
# 内存分配分析
go test -bench=BenchmarkAPI_MemoryAllocation -benchmem ./tests/benchmark/...
```

## 内存分析

### 使用内存分析脚本

```bash
# 堆内存分析
./scripts/memory_profile.sh heap

# Goroutine 分析
./scripts/memory_profile.sh goroutine

# CPU 分析（30秒）
./scripts/memory_profile.sh cpu 30

# 完整分析报告
./scripts/memory_profile.sh full

# 实时监控（每5秒刷新）
./scripts/memory_profile.sh monitor 5

# 运行内存基准测试
./scripts/memory_profile.sh benchmark
```

### 使用 go tool pprof 交互式分析

```bash
# 启动 Web UI
go tool pprof -http=:8080 reports/performance/heap_xxx.prof

# 命令行分析
go tool pprof reports/performance/heap_xxx.prof
# (pprof) top10
# (pprof) list functionName
# (pprof) web

# 比较两个快照
go tool pprof -base=old.prof new.prof
```

## 性能指标参考

### API 响应时间目标

| 端点类型 | 目标响应时间 |
|---------|-------------|
| 健康检查 | < 1ms |
| 简单查询 | < 10ms |
| 文件列表 | < 50ms |
| 搜索操作 | < 100ms |
| 文件上传 | 取决于文件大小 |

### 内存使用目标

| 指标 | 目标值 |
|-----|-------|
| Goroutine 数量 | < 10000 |
| 堆内存增长 | < 1MB/min（空闲） |
| GC 暂停 | < 1ms |

## 报告归档

建议定期归档性能报告：

```bash
# 创建月度归档
tar -czf performance_$(date +%Y%m).tar.gz reports/performance/*.prof reports/performance/*.txt reports/performance/*.md

# 清理旧报告（保留最近30天）
find reports/performance -name "*.prof" -mtime +30 -delete
```