# Fusion Pool 存储分层使用指南

**文档版本**: v1.0 | **更新日期**: 2026-03-29 | **编制**: 礼部

---

## 一、功能概述

### 1.1 什么是 Fusion Pool

Fusion Pool 是 NAS-OS 的智能存储分层系统，借鉴群晖 DSM 7.3 的 Synology Tiering 技术，实现：

- **热数据 SSD 加速**：高频访问数据自动保持在 SSD 层
- **冷数据 HDD 存储**：低频数据自动下沉到 HDD 层
- **成本优化**：减少昂贵 SSD 的使用量，降低存储成本
- **性能保障**：关键数据始终保持高速访问

### 1.2 适用场景

| 场景 | 说明 |
|------|------|
| 数据密集型应用 | 频繁读写的大型数据库、缓存 |
| 影音媒体库 | 在线播放的热门内容保持在 SSD |
| 开发环境 | 项目源码、编译产物快速访问 |
| 企业文档 | 常用文档快速访问，历史文档归档 |

### 1.3 核心优势

| 优势 | 说明 |
|------|------|
| 自动分层 | 无需手动管理，系统智能判断 |
| 灵活配置 | 可自定义分层策略 |
| 成本节省 | 减少 SSD 采购成本 |
| 性能提升 | 热数据始终高速访问 |

---

## 二、存储层级说明

### 2.1 三级存储架构

```
┌─────────────────────────────────────────────────────────┐
│                      Fusion Pool                         │
├─────────────────────────────────────────────────────────┤
│  热存储层 (SSD)                                          │
│  - NVMe SSD                                              │
│  - 最快响应时间 (<1ms)                                    │
│  - 容量较小，成本较高                                     │
│  - 存放高频访问数据                                       │
├─────────────────────────────────────────────────────────┤
│  温存储层 (SSD/HDD 混合)                                  │
│  - SATA SSD 或高性能 HDD                                  │
│  - 中等响应时间 (5-10ms)                                  │
│  - 存放中等频率数据                                       │
├─────────────────────────────────────────────────────────┤
│  冷存储层 (HDD/云存储)                                    │
│  - 大容量 HDD                                            │
│  - 或云存储 (OSS/S3)                                     │
│  - 较慢响应时间 (10-50ms)                                 │
│  - 存放低频访问数据                                       │
└─────────────────────────────────────────────────────────┘
```

### 2.2 数据分类标准

| 分类 | 访问频率 | 典型数据 | 推荐存储层 |
|------|----------|----------|-----------|
| **热数据** | 每日访问 | 系统文件、项目源码、热门媒体 | SSD |
| **温数据** | 每周访问 | 近期文档、普通媒体 | SSD/HDD 混合 |
| **冷数据** | 每月或更少 | 历史归档、备份、日志 | HDD/云存储 |

---

## 三、创建 Fusion Pool

### 3.1 Web UI 方式

1. **进入存储管理**
   - 登录 Web 界面
   - 点击「存储」→「存储池管理」

2. **创建 Fusion Pool**
   - 点击「创建存储池」
   - 选择「Fusion Pool」类型

3. **配置存储层**
   - 选择 SSD 设备（热存储层）
   - 选择 HDD 设备（冷存储层）
   - 可选配置云存储作为扩展层

4. **配置分层策略**
   - 选择预设策略或自定义
   - 设置分层阈值

5. **确认创建**
   - 查看容量预览
   - 点击「创建」

### 3.2 CLI 方式

```bash
# 创建 Fusion Pool
sudo nasctl fusion-pool create mypool \
  --ssd /dev/nvme0n1 \
  --hdd /dev/sda,/dev/sdb \
  --policy hot-first

# 查看创建结果
sudo nasctl fusion-pool show mypool
```

### 3.3 API 方式

```bash
curl -X POST http://localhost:8080/api/v1/fusion-pools \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mypool",
    "ssdDevices": ["/dev/nvme0n1"],
    "hddDevices": ["/dev/sda", "/dev/sdb"],
    "policy": {
      "type": "hot-first",
      "hotThreshold": 7,
      "coldThreshold": 30
    }
  }'
```

---

## 四、分层策略配置

### 4.1 预设策略

| 策略名称 | 说明 | 适用场景 |
|----------|------|----------|
| **hot-first** | 优先保持热数据在 SSD | 数据密集型应用 |
| **cost-optimize** | 优化成本，最小化 SSD 使用 | 成本敏感场景 |
| **balanced** | 平衡性能与成本 | 通用场景 |
| **media-optimize** | 媒体优化，热门内容保持 SSD | 影音媒体库 |

### 4.2 自定义策略

```yaml
# /etc/nas-os/fusion-pool-policy.yaml
policy:
  name: custom-policy
  
  # 分层规则
  tiering:
    # 热数据判定：最近 N 天内访问
    hot_threshold_days: 7
    # 冷数据判定：超过 N 天未访问
    cold_threshold_days: 30
    # 最大 SSD 使用率
    max_ssd_usage: 80
    
  # 迁移规则
  migration:
    # 迁移时间窗口（避免高峰期）
    schedule: "02:00-06:00"
    # 每次迁移最大数据量 (GB)
    max_migration_size: 50
    # 迁移速率限制 (MB/s)
    rate_limit: 100
    
  # 排除规则
  exclusions:
    # 排除特定目录
    paths:
      - "/system"
      - "/cache"
    # 排除特定文件类型
    extensions:
      - ".log"
      - ".tmp"
```

### 4.3 应用自定义策略

```bash
# 应用策略
sudo nasctl fusion-pool policy apply mypool --file custom-policy.yaml

# 查看当前策略
sudo nasctl fusion-pool policy show mypool
```

---

## 五、数据迁移管理

### 5.1 自动迁移

系统根据策略自动执行数据迁移：

- **热数据提升**：当冷层数据访问频率升高，自动迁移到热层
- **冷数据下沉**：当热层数据长时间未访问，自动迁移到冷层
- **空间平衡**：当热层空间不足，优先下沉最冷数据

### 5.2 手动迁移

```bash
# 手动将文件提升到 SSD
sudo nasctl fusion-pool promote mypool /data/videos/popular.mp4

# 手动将文件下沉到 HDD
sudo nasctl fusion-pool demote mypool /data/archives/old-data.tar

# 手动迁移整个目录
sudo nasctl fusion-pool promote mypool /data/projects/current --recursive

# 查看迁移状态
sudo nasctl fusion-pool migration status mypool
```

### 5.3 迁移历史查看

```bash
# 查看迁移历史
sudo nasctl fusion-pool migration history mypool --days 7

# 输出示例
Migration History (Last 7 Days)
==============================
2026-03-28 02:00  SSD → HDD  120 files (45GB)
2026-03-27 02:00  HDD → SSD  35 files (12GB)
2026-03-26 02:00  SSD → HDD  80 files (32GB)
```

---

## 六、监控与报告

### 6.1 实时监控

**Web UI 监控面板**：
- 存储层容量使用率
- 数据分布比例
- 缓存命中率
- 迁移队列状态

**CLI 监控**：

```bash
# 查看存储层状态
sudo nasctl fusion-pool status mypool

# 输出示例
Fusion Pool: mypool
==================
SSD Layer:
  Capacity: 1TB
  Used: 800GB (80%)
  Hot Data: 600GB (75%)
  Warm Data: 150GB (19%)
  Cold Data: 50GB (6%)

HDD Layer:
  Capacity: 10TB
  Used: 5TB (50%)
  Hot Data: 500GB (10%)
  Warm Data: 3.5TB (70%)
  Cold Data: 1TB (20%)

Efficiency Score: 85/100
Cache Hit Rate: 92%
```

### 6.2 Prometheus 指标

```bash
# 获取 Prometheus 格式指标
curl http://localhost:8080/metrics | grep fusion_pool

# 主要指标
nas_fusion_pool_ssd_capacity_bytes
nas_fusion_pool_ssd_used_bytes
nas_fusion_pool_hdd_capacity_bytes
nas_fusion_pool_hdd_used_bytes
nas_fusion_pool_hot_data_ratio
nas_fusion_pool_cache_hit_rate
nas_fusion_pool_migration_total
nas_fusion_pool_efficiency_score
```

### 6.3 效率报告

```bash
# 生成效率报告
sudo nasctl fusion-pool report mypool --period weekly

# 输出示例
Weekly Efficiency Report (2026-03-22 ~ 2026-03-28)
================================================
Tiering Score: 85/100 (Excellent)
Cache Hit Rate: 92% (Optimal)

Data Distribution:
  Hot: 60% (1.8TB)
  Warm: 20% (600GB)
  Cold: 20% (600GB)

Migrations:
  Total: 156 tasks
  Success: 155 (99.4%)
  Failed: 1 (0.6%)

Recommendations:
  ✓ SSD usage optimal (80%)
  ⚠ Consider adding SSD capacity
  💡 Cold data could be archived to cloud
```

---

## 七、常见问题解答

### Q1: Fusion Pool 和普通存储池有什么区别？

**A**: 
| 特性 | Fusion Pool | 普通存储池 |
|------|-------------|-----------|
| 数据分层 | ✅ 自动 | ❌ 无 |
| SSD 缓存 | ✅ 多级智能 | ⚠️ 单级固定 |
| 成本优化 | ✅ 自动 | ❌ 需手动 |
| 性能保障 | ✅ 热数据优先 | ⚠️ 平均分配 |

### Q2: 如何判断数据是否应该提升到 SSD？

**A**: 系统自动根据访问频率判断。默认配置：
- 7 天内访问过的数据视为热数据
- 超过 30 天未访问视为冷数据

可自定义阈值调整判断标准。

### Q3: 迁移会影响正常使用吗？

**A**: 
- 迁移默认在夜间低峰期执行
- 迁移过程中数据仍可正常访问
- 可配置速率限制避免资源竞争

### Q4: SSD 层空间不足怎么办？

**A**: 
1. 系统自动下沉最冷数据腾出空间
2. 可手动迁移不常用数据到 HDD
3. 考虑增加 SSD 容量

### Q5: 如何查看特定文件的存储层？

**A**: 
```bash
sudo nasctl fusion-pool file-location mypool /data/file.txt

# 输出
File: /data/file.txt
Layer: SSD (Hot)
Last Access: 2026-03-27
Access Count (7d): 15
```

### Q6: 可以强制文件保持在 SSD 吗？

**A**: 
```bash
# 锁定文件在 SSD 层
sudo nasctl fusion-pool pin mypool /data/important.db

# 取消锁定
sudo nasctl fusion-pool unpin mypool /data/important.db
```

### Q7: 如何关闭自动分层？

**A**: 
```bash
# 暂停自动迁移
sudo nasctl fusion-pool pause mypool

# 恢复自动迁移
sudo nasctl fusion-pool resume mypool
```

### Q8: 如何导出效率报告？

**A**: 
```bash
# 导出为 CSV
sudo nasctl fusion-pool report mypool --format csv --output report.csv

# 导出为 PDF
sudo nasctl fusion-pool report mypool --format pdf --output report.pdf
```

---

## 八、最佳实践

### 8.1 配置建议

| 场景 | SSD/HDD 比例 | 策略建议 |
|------|-------------|----------|
| 数据密集型 | 1:4 | hot-first，热阈值 3 天 |
| 媒体库 | 1:10 | media-optimize，热阈值 7 天 |
| 企业文档 | 1:8 | balanced，热阈值 14 天 |
| 开发环境 | 1:2 | hot-first，排除日志目录 |

### 8.2 监控建议

- 定期查看效率报告（每周）
- 关注 SSD 使用率（建议 < 80%）
- 检查缓存命中率（建议 > 85%）
- 注意迁移失败任务

### 8.3 性能调优

```yaml
# 性能优先配置
policy:
  tiering:
    hot_threshold_days: 3    # 更激进的热数据判定
    max_ssd_usage: 90        # 允许 SSD 更高使用率
  migration:
    rate_limit: 200          # 更快的迁移速度
```

---

## 九、故障排查

### 9.1 分层不生效

**排查步骤**：
```bash
# 1. 检查 Fusion Pool 状态
sudo nasctl fusion-pool status mypool

# 2. 检查策略是否应用
sudo nasctl fusion-pool policy show mypool

# 3. 检查迁移服务
sudo systemctl status nas-os-tiering

# 4. 查看日志
sudo journalctl -u nas-os-tiering -n 50
```

### 9.2 迁移失败

**常见原因**：
- 磁盘空间不足
- 文件被占用（正在写入）
- 权限问题

**解决方法**：
```bash
# 查看失败详情
sudo nasctl fusion-pool migration failures mypool

# 重试失败任务
sudo nasctl fusion-pool migration retry mypool TASK_ID

# 强制迁移（关闭占用检测）
sudo nasctl fusion-pool promote mypool /data/file --force
```

---

## 十、API 参考

### 10.1 端点列表

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/fusion-pools` | GET | 列出所有 Fusion Pool |
| `/api/v1/fusion-pools` | POST | 创建 Fusion Pool |
| `/api/v1/fusion-pools/:id` | GET | 获取详情 |
| `/api/v1/fusion-pools/:id` | DELETE | 删除 Fusion Pool |
| `/api/v1/fusion-pools/:id/status` | GET | 获取状态 |
| `/api/v1/fusion-pools/:id/policy` | GET | 获取策略 |
| `/api/v1/fusion-pools/:id/policy` | PUT | 更新策略 |
| `/api/v1/fusion-pools/:id/promote` | POST | 提升数据 |
| `/api/v1/fusion-pools/:id/demote` | POST | 下沉数据 |
| `/api/v1/fusion-pools/:id/report` | GET | 获取效率报告 |

### 10.2 示例请求

**创建 Fusion Pool**：
```json
POST /api/v1/fusion-pools
{
  "name": "media-pool",
  "ssdDevices": ["nvme0n1"],
  "hddDevices": ["sda", "sdb"],
  "policy": {
    "type": "media-optimize",
    "hotThresholdDays": 7,
    "coldThresholdDays": 30
  }
}
```

**获取状态**：
```json
GET /api/v1/fusion-pools/media-pool/status

Response:
{
  "ssd": {
    "capacity": 1073741824000,
    "used": 858993459200,
    "usagePercent": 80
  },
  "hdd": {
    "capacity": 21474836480000,
    "used": 10737418240000,
    "usagePercent": 50
  },
  "efficiency": {
    "score": 85,
    "cacheHitRate": 92,
    "hotDataRatio": 60
  }
}
```

---

**礼部编制**
2026-03-29