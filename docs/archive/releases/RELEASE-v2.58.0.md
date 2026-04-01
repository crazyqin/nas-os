# NAS-OS v2.58.0 发布公告

**发布日期**: 2026-03-15  
**版本类型**: Stable

---

## 🎉 新功能介绍

### 成本分析系统

v2.58.0 引入了强大的成本分析能力，帮助您更好地理解和优化 NAS 资源消耗：

- **成本概览仪表板**
  - 当前周期成本总览
  - 按服务类型分类统计（存储/带宽/服务）
  - 与上一周期对比分析
  - 节省金额明细

- **趋势分析**
  - 日/周/月粒度成本趋势
  - 按服务、用户、类型分组
  - 趋势方向与变化百分比
  - 最大/最小/平均值统计

- **成本预测**
  - 基于历史数据的智能预测
  - 多场景模拟（正常/增长/节省）
  - 预测置信区间
  - 影响因素分析

- **优化建议**
  - 自动识别节省机会
  - 一键应用优化方案
  - 潜在节省金额估算

```bash
# 获取成本概览
curl http://localhost:8080/api/v1/billing/cost/overview

# 查看成本趋势
curl "http://localhost:8080/api/v1/billing/cost/trends?granularity=daily"

# 获取成本预测
curl "http://localhost:8080/api/v1/billing/cost/forecast?months=3"

# 查看优化建议
curl http://localhost:8080/api/v1/billing/cost/recommendations
```

### 预算警报系统

全新的预算管理功能，让成本控制更加智能：

- **预算设置**
  - 总体预算或按服务/用户设置
  - 月度/季度/年度周期
  - 自定义重置日期

- **多阈值警报**
  - 多个百分比阈值（如 50%、80%、100%）
  - 触发状态跟踪
  - 历史触发记录

- **多渠道通知**
  - 邮件通知
  - Webhook 集成
  - 自定义通知渠道

- **警报测试**
  - 发送测试警报验证配置
  - 查看通知发送状态

```bash
# 创建预算
curl -X POST http://localhost:8080/api/v1/billing/budgets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "月度总体预算",
    "amount": 2000,
    "period": "monthly",
    "alert_thresholds": [50, 80, 100],
    "alert_channels": ["email", "webhook"]
  }'

# 测试警报
curl -X POST http://localhost:8080/api/v1/billing/budgets/budget-001/test-alert
```

### 发票与支付系统

完整的账单管理和在线支付能力：

- **发票管理**
  - 自动生成周期账单
  - 手动创建自定义发票
  - 发票详情与项目明细
  - PDF 导出与邮件发送

- **在线支付**
  - 支付宝支付集成
  - 微信支付集成
  - 银行转账支持
  - 实时支付状态更新

- **余额管理**
  - 账户余额充值
  - 余额支付
  - 交易记录查询
  - 自动余额抵扣

- **退款处理**
  - 在线申请退款
  - 退款状态跟踪
  - 退款记录管理

```bash
# 获取发票列表
curl http://localhost:8080/api/v1/billing/invoices

# 创建支付
curl -X POST http://localhost:8080/api/v1/billing/payments \
  -H "Content-Type: application/json" \
  -d '{
    "invoice_id": "INV-2026-03-001",
    "method": "alipay"
  }'

# 使用余额支付
curl -X POST http://localhost:8080/api/v1/billing/payments/balance \
  -H "Content-Type: application/json" \
  -d '{"invoice_id": "INV-2026-03-001"}'
```

---

## 📦 升级指南

### 从 v2.57.0 升级

#### 方式一：二进制文件升级

```bash
# 停止服务
sudo systemctl stop nas-os

# 备份配置
sudo cp -r /etc/nas-os /etc/nas-os.bak

# 下载新版本 (根据架构选择)
wget https://github.com/crazyqin/nas-os/releases/download/v2.58.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 启动服务
sudo systemctl start nas-os

# 验证版本
nasd --version
```

#### 方式二：Docker 升级

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.58.0

# 停止旧容器
docker stop nasd
docker rm nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.58.0

# 验证
docker logs nasd | head -5
```

### 配置变更

v2.58.0 新增配置项（`/etc/nas-os/config.yaml`）：

```yaml
billing:
  enabled: true
  cost_analysis:
    enabled: true
    prediction_months: 3
    recommendations_enabled: true
  
  budget:
    enabled: true
    default_thresholds: [50, 80, 100]
    notification_channels:
      email:
        enabled: true
        recipients: []
      webhook:
        enabled: false
        url: ""
  
  payment:
    enabled: true
    methods:
      - alipay
      - wechat
      - bank_transfer
    alipay:
      app_id: ""
      private_key: ""
      public_key: ""
    wechat:
      app_id: ""
      mch_id: ""
      api_key: ""
  
  invoice:
    enabled: true
    auto_generate: true
    prefix: "INV"
    due_days: 30
```

### 数据库迁移

v2.58.0 新增以下数据表：

- `billing_budgets` - 预算配置
- `budget_alerts` - 预算警报记录
- `payments` - 支付记录
- `refunds` - 退款记录
- `balance_transactions` - 余额交易记录
- `alert_channels` - 通知渠道配置

系统启动时会自动执行迁移，无需手动操作。

---

## ⚠️ 已知问题

1. **支付宝沙箱环境**
   - 测试环境可能存在回调延迟
   - 建议：使用正式环境进行生产部署

2. **成本预测准确度**
   - 历史数据不足时预测置信度较低
   - 建议：至少积累 3 个月用量数据后再依赖预测

3. **Webhook 超时**
   - 外部 Webhook 调用默认 5 秒超时
   - 建议：确保接收服务响应时间 <3 秒

---

## 📥 下载

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.58.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.58.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.58.0/nasd-linux-armv7) |

**Docker 镜像**:
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.58.0
```

---

## 🙏 贡献者

感谢以下部门的贡献：

- **户部** - 成本分析系统、预算警报、支付系统
- **兵部** - 支付网关集成
- **礼部** - API 文档、发布公告

---

## 📚 相关链接

- [完整更新日志](../CHANGELOG.md)
- [API 文档](./API_GUIDE.md)
- [计费 API](./api/billing-api.md)
- [发票 API](./api/invoice-api.md)
- [用户指南](./user-guide/)
- [GitHub Issues](https://github.com/crazyqin/nas-os/issues)

---

**NAS-OS 团队**  
2026-03-15