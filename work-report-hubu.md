# 户部工作报告 - 第38轮

**日期**: 2026-03-25
**部门**: 户部
**轮次**: 第38轮

## 资源统计结果

| 指标 | 数量 | 环比变化 |
|------|------|----------|
| Go 源文件 | 895 | ↑146 |
| 测试文件 | 311 | ↑38 |
| 代码行数 | 500,127 | ↑79,545 |
| 功能模块 | 76 | ↑7 |

## 模块Top 5

| 模块 | 代码行数 | 占比 |
|------|----------|------|
| reports | 35,206 | 7.0% |
| security | 29,460 | 5.9% |
| quota | 19,525 | 3.9% |
| backup | 18,544 | 3.7% |
| ai (新) | 10,059 | 2.0% |

---

## 🆕 AI模型成本优化评估 (2026-03-25)

完成 `internal/ai/` 模块的成本优化分析，详细报告: `memory/ai-cost-analysis.md`

### 核心发现

**1. 云端API定价对比 (2026年3月)**

| 提供商 | 输入($/1M) | 输出($/1M) | 备注 |
|--------|------------|------------|------|
| **DeepSeek V3.2** | $0.28 | $0.42 | 最便宜 |
| OpenAI GPT-4o | $2.50 | $10.00 | 标杆 |
| 智谱AI GLM-4 | ¥0.10 | ¥0.10 | 国产优选 |

**2. 本地GPU部署ROI**

| GPU | 初始投入 | 月度成本 | ROI周期(vs OpenAI) |
|-----|----------|----------|---------------------|
| RTX 4060 Ti 16GB | ¥2,500 | ¥81 | 6个月 |
| RTX 4090 24GB | ¥8,500 | ¥268 | 8个月 |
| **Intel Arc B580** | ¥1,500 | ¥52 | 4个月 ⭐ |

**3. Intel核显加速**

- Arc系列支持Quick Sync Video + XMX引擎
- 7B模型推理: ~180 tokens/sec
- 性价比: NVIDIA同性能50-70%价格
- 推荐: 视频处理+AI一体场景

### 推荐方案

| 用户类型 | 推荐方案 | 月成本 |
|----------|----------|--------|
| 个人/轻度 | DeepSeek API | <¥10 |
| 中小企业 | 本地RTX 4060 Ti | ¥81 |
| 企业/重度 | vLLM集群 | 按需 |

### 实施建议

- **短期**: 集成DeepSeek作为默认API提供商
- **中期**: 添加Intel OpenVINO核显加速支持
- **长期**: 云端↔本地自动切换

---

## 存储优化收益

- 分层存储节省: **77%**
- SSD缓存回本: 3-6个月
- 云归档最优: 阿里云 ¥10/TB/月

## 商业化定价建议

| 版本 | 年费 | 目标用户 |
|------|------|----------|
| 社区版 | 免费 | 个人 |
| 专业版 | ¥299 | 高级用户 |
| 企业版 | ¥1,999 | 中小企业 |

## RAIDZ扩展技术调研

**调研结论**：

| 文件系统 | RAID扩展 | 稳定性 | 推荐度 |
|----------|----------|--------|--------|
| ZFS RAIDZ | 单命令原子操作 | 稳定 | ⭐⭐⭐⭐⭐ |
| btrfs RAID1/10 | 多步骤操作 | 稳定 | ⭐⭐⭐⭐ |
| btrfs RAID5/6 | 多步骤操作 | 不稳定 | ⚠️不推荐 |

**关键发现**：
- TrueNAS Fangtooth实现RAIDZ扩展加速5倍
- OpenZFS 2.3正式支持RAIDZ Expansion
- btrfs RAID5/6仍标记为不稳定（write hole问题）
- 建议nas-os短期优化btrfs RAID1/10扩展体验，中期引入ZFS选项

**实现建议**：
- 短期：封装btrfs balance API，禁用RAID5/6（6-7人周）
- 中期：引入ZFS作为可选存储后端（16-22人周）

详细报告: `docs/RAIDZ_EXPANSION_RESEARCH.md`

## 汇总

```json
{
  "dept": "户部",
  "round": 38,
  "go_files": 895,
  "test_files": 311,
  "code_lines": 500127,
  "modules": 76,
  "march_commits": 1326,
  "ai_cost_analysis": {
    "cheapest_api": "DeepSeek V3.2 ($0.28/$0.42 per 1M tokens)",
    "recommended_gpu": "RTX 4060 Ti 16GB (¥2,500, 6-month ROI)",
    "budget_gpu": "Intel Arc B580 (¥1,500, 4-month ROI)",
    "intel_arc_support": "Quick Sync Video + XMX, ~180 t/s on 7B models"
  },
  "storage_savings": "77%",
  "raid_research": {
    "zfs_expansion": "稳定可用",
    "btrfs_raid56": "不稳定禁止",
    "recommendation": "短期优化btrfs RAID1/10，中期引入ZFS"
  }
}
```

---
*户部 · 第38轮统计完成 · AI成本评估完成 · RAIDZ扩展调研完成*