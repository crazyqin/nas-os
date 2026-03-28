# Cloudflare Tunnel集成设计

## 背景
飞牛fnOS支持Cloudflare Tunnel实现无公网IP远程访问，无需开放端口。本设计研究nas-os集成方案。

## 技术原理

### Cloudflare Tunnel工作流程
1. 本地运行cloudflared客户端
2. 与Cloudflare边缘建立加密隧道
3. 外部请求通过隧道转发到本地服务
4. 无需暴露本地IP或端口

### 优势
- 无公网IP也能远程访问
- 无需开放防火墙端口
- DDoS防护内置
- TLS自动管理

## 集成方案

### 方案一：独立cloudflared服务
```yaml
# systemd服务
[Unit]
Description=Cloudflare Tunnel for nas-os
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/cloudflared tunnel run --token <TOKEN>
Restart=always

[Install]
WantedBy=multi-user.target
```

### 方案二：容器化部署
```yaml
# docker-compose.yml
services:
  cloudflared:
    image: cloudflare/cloudflared:latest
    command: tunnel run
    environment:
      - TUNNEL_TOKEN=${CLOUDFLARE_TOKEN}
    restart: unless-stopped
```

### 方案三：nas-os原生集成
- 内置cloudflared管理模块
- WebUI配置界面
- 自动证书获取
- 与现有内网穿透模块整合

## API设计

```
/api/v1/tunnel
  GET    /status         - Tunnel状态
  POST   /enable         - 启用Tunnel
  POST   /disable        - 禁用Tunnel
  GET    /config         - 获取配置
  PUT    /config         - 更新配置
  POST   /connect        - 连接Tunnel
  POST   /disconnect     - 断开Tunnel
```

## 配置项

```yaml
tunnel:
  enabled: true
  provider: cloudflare  # 或 frp, ngrok
  token: ""             # Cloudflare Tunnel Token
  subdomain: "my-nas"   # 子域名
  services:
    - name: webui
      url: http://localhost:8080
    - name: smb
      url: tcp://localhost:445
```

## 安全考量

1. **Token安全存储**：加密存储Tunnel Token
2. **访问控制**：可选Cloudflare Access策略
3. **日志审计**：记录所有隧道访问
4. **备用方案**：frp内网穿透作为备用

## 实施计划

| 阶段 | 内容 | 优先级 |
|------|------|--------|
| P1 | cloudflared容器部署 | 高 |
| P1 | WebUI基本配置 | 高 |
| P2 | 自动Token获取流程 | 中 |
| P2 | 多服务隧道配置 | 中 |
| P3 | Cloudflare Access集成 | 低 |

## 与现有功能整合

nas-os已有内网穿透功能，Cloudflare Tunnel可作为增强选项：
- 用户可选择Cloudflare Tunnel或现有方案
- 自动检测网络环境推荐最优方案
- 统一管理界面

## 参考
- Cloudflare Tunnel官方文档
- 飞牛fnOS FN Connect设计