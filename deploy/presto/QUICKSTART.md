# NAS-OS Presto 快速开始

## 一键部署

```bash
# 1. 进入部署目录
cd deploy/presto

# 2. 配置环境变量
cp .env.example .env
vi .env  # 根据实际环境修改

# 3. 执行部署
./deploy.sh --install
```

## 访问服务

| 服务 | 地址 | 说明 |
|------|------|------|
| Presto API | http://localhost:8090 | 传输服务 |
| Prometheus | http://localhost:9091 | 监控数据 |
| Grafana | http://localhost:3001 | 可视化面板 |

## Grafana 登录

- 用户名: `admin`
- 密码: `presto123`

## 常用命令

```bash
# 查看状态
docker compose -f docker-compose.presto.yml ps

# 查看日志
docker compose -f docker-compose.presto.yml logs -f

# 停止服务
docker compose -f docker-compose.presto.yml down

# 重启服务
docker compose -f docker-compose.presto.yml restart
```

## 传输文件

```bash
# 创建传输任务
curl -X POST http://localhost:8090/api/transfers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-transfer",
    "source_path": "/mnt/transfer/src/test.tar.gz",
    "dest_path": "/mnt/transfer/dst/test.tar.gz"
  }'

# 查看传输列表
curl http://localhost:8090/api/transfers

# 查看统计信息
curl http://localhost:8090/api/stats
```

## 告警配置

告警会通过邮件发送到 `.env` 中配置的 `ALERT_EMAIL` 地址。

如需修改告警规则，编辑 `monitoring/alerts-presto.yml` 文件。

## 更多信息

详细文档请参考 [README.md](./README.md)
