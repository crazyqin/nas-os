# NAS-OS 监控部署指南

## 概述

本文档介绍如何在 NAS-OS 中部署和配置完整的监控系统，包括 Prometheus 指标收集和 Grafana 可视化。

---

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        NAS-OS 监控架构                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐          │
│  │   nasd      │    │  nasctl     │    │  其他服务    │          │
│  │ (主服务)    │    │  (CLI)      │    │             │          │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘          │
│         │                  │                  │                  │
│         └──────────────────┼──────────────────┘                  │
│                            │                                     │
│                            ▼                                     │
│                   ┌─────────────────┐                           │
│                   │ MetricsExporter │                           │
│                   │  :9090/metrics  │                           │
│                   └────────┬────────┘                           │
│                            │                                     │
│                            ▼                                     │
│                   ┌─────────────────┐                           │
│                   │   Prometheus    │                           │
│                   │     :9090       │                           │
│                   └────────┬────────┘                           │
│                            │                                     │
│                            ▼                                     │
│                   ┌─────────────────┐                           │
│                   │    Grafana      │                           │
│                   │     :3000       │                           │
│                   └─────────────────┘                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## 快速开始

### 使用 Docker Compose 部署

```bash
cd /path/to/nas-os
docker-compose -f docker-compose.monitoring.yml up -d
```

### 创建监控配置文件

```bash
# 创建 docker-compose.monitoring.yml
cat > docker-compose.monitoring.yml << 'EOF'
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:v2.48.0
    container_name: nas-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--storage.tsdb.retention.time=30d'
      - '--web.enable-lifecycle'
    restart: unless-stopped
    networks:
      - nas-network

  grafana:
    image: grafana/grafana:10.2.0
    container_name: nas-grafana
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/provisioning:/etc/grafana/provisioning:ro
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=nas-admin
      - GF_USERS_ALLOW_SIGN_UP=false
    restart: unless-stopped
    networks:
      - nas-network
    depends_on:
      - prometheus

  alertmanager:
    image: prom/alertmanager:v0.26.0
    container_name: nas-alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./monitoring/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
      - alertmanager_data:/alertmanager
    restart: unless-stopped
    networks:
      - nas-network

  node-exporter:
    image: prom/node-exporter:v1.7.0
    container_name: nas-node-exporter
    ports:
      - "9100:9100"
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro
    command:
      - '--path.procfs=/host/proc'
      - '--path.sysfs=/host/sys'
      - '--path.rootfs=/rootfs'
    restart: unless-stopped
    networks:
      - nas-network

volumes:
  prometheus_data:
  grafana_data:
  alertmanager_data:

networks:
  nas-network:
    external: true
EOF
```

---

## Prometheus 配置

### 主配置文件

创建 `monitoring/prometheus.yml`：

```yaml
# Prometheus 配置
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    monitor: 'nas-os'

# 告警规则文件
rule_files:
  - /etc/prometheus/rules/*.yml

# Alertmanager 配置
alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093

# 抓取配置
scrape_configs:
  # NAS-OS 主服务
  - job_name: 'nas-os'
    static_configs:
      - targets: ['nasd:9090']
    metrics_path: /metrics
    scrape_interval: 10s

  # Node Exporter (系统指标)
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']
    scrape_interval: 15s

  # Prometheus 自身
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  # MySQL Exporter (可选)
  - job_name: 'mysql'
    static_configs:
      - targets: ['mysql-exporter:9104']

  # Redis Exporter (可选)
  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']
```

### 告警规则

创建 `monitoring/rules/nas-alerts.yml`：

```yaml
groups:
  - name: nas-os-alerts
    rules:
      # 系统告警
      - alert: HighCPUUsage
        expr: 100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100) > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "高 CPU 使用率"
          description: "CPU 使用率超过 80%，当前值: {{ $value }}%"

      - alert: HighMemoryUsage
        expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "高内存使用率"
          description: "内存使用率超过 85%，当前值: {{ $value }}%"

      - alert: DiskSpaceLow
        expr: (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100 < 10
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "磁盘空间不足"
          description: "根分区剩余空间不足 10%，当前值: {{ $value }}%"

      # 服务告警
      - alert: ServiceDown
        expr: nas_os_service_status == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "服务停止"
          description: "服务 {{ $labels.service }} 已停止运行"

      - alert: BackupFailed
        expr: nas_os_backup_status == 0
        for: 0m
        labels:
          severity: warning
        annotations:
          summary: "备份失败"
          description: "备份任务 {{ $labels.type }} ({{ $labels.id }}) 失败"

      - alert: HighAPILatency
        expr: histogram_quantile(0.95, rate(nas_os_api_latency_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API 延迟过高"
          description: "95% 请求延迟超过 1 秒"

      - alert: HighErrorRate
        expr: rate(nas_os_api_errors_total[5m]) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API 错误率过高"
          description: "API 错误率: {{ $value }} 错误/秒"

      # 存储告警
      - alert: StoragePoolDegraded
        expr: nas_os_storage_pool_health == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "存储池降级"
          description: "存储池 {{ $labels.pool }} 处于降级状态"

      - alert: VolumeAlmostFull
        expr: nas_os_volume_usage_percent > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "卷空间即将耗尽"
          description: "卷 {{ $labels.volume }} 使用率超过 90%"
```

---

## Alertmanager 配置

创建 `monitoring/alertmanager.yml`：

```yaml
global:
  resolve_timeout: 5m
  # Slack 配置 (可选)
  # slack_api_url: 'https://hooks.slack.com/services/xxx'

route:
  group_by: ['alertname', 'severity']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: 'default'
  routes:
    - match:
        severity: critical
      receiver: 'critical'
    - match:
        severity: warning
      receiver: 'warning'

receivers:
  - name: 'default'
    webhook_configs:
      - url: 'http://nasd:8080/api/v1/alerts'
        send_resolved: true

  - name: 'critical'
    webhook_configs:
      - url: 'http://nasd:8080/api/v1/alerts/critical'
        send_resolved: true
    # 邮件通知
    email_configs:
      - to: 'admin@example.com'
        from: 'nas-os@example.com'
        smarthost: 'smtp.example.com:587'
        auth_username: 'nas-os@example.com'
        auth_password: 'password'

  - name: 'warning'
    webhook_configs:
      - url: 'http://nasd:8080/api/v1/alerts/warning'
        send_resolved: true

inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'instance']
```

---

## Grafana 配置

### 数据源配置

创建 `monitoring/provisioning/datasources/prometheus.yml`：

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
```

### 仪表板配置

创建 `monitoring/provisioning/dashboards/dashboard.yml`：

```yaml
apiVersion: 1

providers:
  - name: 'NAS-OS'
    orgId: 1
    folder: 'NAS-OS'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    options:
      path: /etc/grafana/provisioning/dashboards/json
```

### 系统监控仪表板

创建 `monitoring/provisioning/dashboards/json/nas-os-system.json`：

```json
{
  "dashboard": {
    "title": "NAS-OS 系统监控",
    "uid": "nas-system",
    "panels": [
      {
        "title": "CPU 使用率",
        "type": "gauge",
        "gridPos": {"x": 0, "y": 0, "w": 8, "h": 6},
        "targets": [
          {
            "expr": "100 - (avg by(instance) (irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100)",
            "legendFormat": "CPU Usage"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "max": 100,
            "min": 0,
            "unit": "percent",
            "thresholds": {
              "mode": "absolute",
              "steps": [
                {"color": "green", "value": null},
                {"color": "yellow", "value": 70},
                {"color": "red", "value": 85}
              ]
            }
          }
        }
      },
      {
        "title": "内存使用率",
        "type": "gauge",
        "gridPos": {"x": 8, "y": 0, "w": 8, "h": 6},
        "targets": [
          {
            "expr": "(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100",
            "legendFormat": "Memory Usage"
          }
        ],
        "fieldConfig": {
          "defaults": {
            "max": 100,
            "min": 0,
            "unit": "percent",
            "thresholds": {
              "mode": "absolute",
              "steps": [
                {"color": "green", "value": null},
                {"color": "yellow", "value": 70},
                {"color": "red", "value": 85}
              ]
            }
          }
        }
      },
      {
        "title": "磁盘使用率",
        "type": "gauge",
        "gridPos": {"x": 16, "y": 0, "w": 8, "h": 6},
        "targets": [
          {
            "expr": "nas_os_disk_usage_percent",
            "legendFormat": "{{ device }}"
          }
        ]
      },
      {
        "title": "系统负载",
        "type": "timeseries",
        "gridPos": {"x": 0, "y": 6, "w": 12, "h": 6},
        "targets": [
          {
            "expr": "nas_os_load_1m",
            "legendFormat": "1m"
          },
          {
            "expr": "nas_os_load_5m",
            "legendFormat": "5m"
          },
          {
            "expr": "nas_os_load_15m",
            "legendFormat": "15m"
          }
        ]
      },
      {
        "title": "网络流量",
        "type": "timeseries",
        "gridPos": {"x": 12, "y": 6, "w": 12, "h": 6},
        "targets": [
          {
            "expr": "rate(node_network_receive_bytes_total{device!=\"lo\"}[5m])",
            "legendFormat": "RX {{ device }}"
          },
          {
            "expr": "rate(node_network_transmit_bytes_total{device!=\"lo\"}[5m])",
            "legendFormat": "TX {{ device }}"
          }
        ]
      }
    ]
  }
}
```

---

## 服务集成

### 启用 MetricsExporter

在 `nasd` 主服务中集成：

```go
package main

import (
    "github.com/nas-os/internal/monitor"
)

func main() {
    // 创建指标导出器
    exporter := monitor.NewMetricsExporter(monitor.MetricsExporterConfig{
        Namespace: "nas_os",
        Port:      9090,
        Path:      "/metrics",
    })

    // 启动导出器
    if err := exporter.Start(); err != nil {
        log.Fatalf("启动指标导出器失败: %v", err)
    }
    defer exporter.Stop()

    // 更新指标示例
    go func() {
        for {
            // 更新系统指标
            exporter.UpdateCPU(45.2, 54.8, 20.1, 15.3, 9.8)
            exporter.UpdateMemory(16*1024*1024*1024, 8*1024*1024*1024, 4*1024*1024*1024, 2*1024*1024*1024, 1*1024*1024*1024, 50.0)

            time.Sleep(10 * time.Second)
        }
    }()
}
```

### 自定义指标

```go
// 注册自定义 Gauge
myGauge, err := exporter.RegisterCustomGauge(
    "custom_metric_name",
    "自定义指标说明",
    []string{"label1", "label2"},
)

// 使用自定义 Gauge
myGauge.WithLabelValues("value1", "value2").Set(100)

// 注册自定义 Counter
myCounter, err := exporter.RegisterCustomCounter(
    "custom_counter_name",
    "自定义计数器说明",
    []string{"type"},
)

// 使用自定义 Counter
myCounter.WithLabelValues("request").Inc()
```

---

## 常用查询

### PromQL 查询示例

```promql
# CPU 使用率
100 - (avg by(instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# 内存使用率
(1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100

# 磁盘使用率
nas_os_disk_usage_percent

# API 请求速率 (QPS)
rate(nas_os_api_requests_total[5m])

# API P95 延迟
histogram_quantile(0.95, rate(nas_os_api_latency_seconds_bucket[5m]))

# 错误率
rate(nas_os_api_errors_total[5m]) / rate(nas_os_api_requests_total[5m]) * 100

# 备份成功率
sum(nas_os_backup_status == 1) / sum(nas_os_backup_status) * 100

# 服务可用性
avg_over_time(nas_os_service_status[1h]) * 100
```

---

## 运维操作

### 查看指标

```bash
# 查看 NAS-OS 指标
curl http://localhost:9090/metrics

# 查看 Prometheus 状态
curl http://localhost:9090/-/healthy
curl http://localhost:9090/-/ready

# 重新加载配置
curl -X POST http://localhost:9090/-/reload
```

### 备份与恢复

```bash
# 备份 Prometheus 数据
docker exec nas-prometheus promtool tsdb snapshot /prometheus

# 备份 Grafana 数据
docker exec nas-grafana grafana-cli admin data-migration export /var/lib/grafana

# 恢复
# 将备份文件复制到对应目录后重启容器
```

### 性能调优

```yaml
# Prometheus 性能参数
command:
  - '--storage.tsdb.retention.time=30d'      # 数据保留时间
  - '--storage.tsdb.retention.size=10GB'     # 数据保留大小
  - '--storage.tsdb.max-block-duration=2h'   # 最大块持续时间
  - '--query.timeout=2m'                      # 查询超时
  - '--query.max-concurrency=20'              # 最大并发查询
  - '--query.max-samples=50000000'            # 最大样本数
```

---

## 安全配置

### 启用认证

```yaml
# Prometheus 基础认证
basic_auth_users:
  admin: $2b$12$hNf2lSsxfm0.i4a.1kVpSOVyBCfIB51VRjgBUyv6kdnyTlgBjI0a
```

### 网络隔离

```yaml
# docker-compose.yml
services:
  prometheus:
    networks:
      - monitoring
    # 不暴露端口到宿主机
    expose:
      - "9090"
    # 仅内部访问
```

---

## 监控最佳实践

1. **告警分级**：区分 critical、warning、info 级别
2. **避免告警疲劳**：合理设置阈值和持续时间
3. **指标命名规范**：使用 `namespace_subsystem_name_unit` 格式
4. **标签管理**：避免高基数标签（如用户ID）
5. **数据保留**：根据需求设置合理的数据保留策略
6. **定期审查**：定期检查告警规则和仪表板的有效性

---

## 故障排查

### Prometheus 无法抓取指标

```bash
# 检查目标状态
curl http://localhost:9090/api/v1/targets

# 检查网络连通性
docker exec nas-prometheus ping nasd

# 检查指标端点
curl http://nasd:9090/metrics
```

### Grafana 无法显示数据

```bash
# 检查数据源配置
curl http://admin:password@localhost:3000/api/datasources

# 测试 PromQL 查询
curl 'http://localhost:9090/api/v1/query?query=up'
```

### 告警不触发

```bash
# 检查告警规则
curl http://localhost:9090/api/v1/rules

# 检查 Alertmanager 状态
curl http://localhost:9093/api/v2/status
```

---

## 参考资料

- [Prometheus 官方文档](https://prometheus.io/docs/)
- [Grafana 官方文档](https://grafana.com/docs/)
- [PromQL 查询指南](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [NAS-OS API 文档](./API_GUIDE.md)