# 内网穿透用户指南

> **版本**: v2.354.0  
> **对标**: 飞牛fnOS FN Connect免费内网穿透服务

---

## 目录

1. [功能概述](#功能概述)
2. [快速开始](#快速开始)
3. [配置说明](#配置说明)
4. [使用场景](#使用场景)
5. [常见问题](#常见问题)
6. [对比FN Connect](#对比fn-connect)

---

## 功能概述

nas-os 内网穿透服务允许您从任何地方访问您的 NAS，无需配置路由器端口转发。

### 核心特性

| 特性 | 说明 |
|------|------|
| 免费额度 | 1GB/月免费流量（对标FN Connect） |
| HTTPS支持 | 安全加密传输 |
| 多协议 | frp/nps/Cloudflare Tunnel可选 |
| 零配置 | 自动生成访问URL |

---

## 快速开始

### 1. 创建隧道

```bash
nasctl tunnel create --name my-web --local-port 8080
```

### 2. 启动隧道

```bash
nasctl tunnel start my-web
```

### 3. 获取访问URL

```bash
nasctl tunnel url my-web
# 输出: https://my-web.connect.nas-os.io
```

---

## 配置说明

### API端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/tunnels | 获取隧道列表 |
| POST | /api/v1/tunnels | 创建隧道 |
| GET | /api/v1/tunnels/:id | 获取隧道详情 |
| POST | /api/v1/tunnels/:id/start | 启动隧道 |
| POST | /api/v1/tunnels/:id/stop | 停止隧道 |
| DELETE | /api/v1/tunnels/:id | 删除隧道 |

---

## 使用场景

### Web服务访问
远程访问NAS Web管理界面

### 文件分享
生成临时分享链接

### API访问
远程调用NAS API

---

## 常见问题

### Q: 如何增加免费额度？
A: 免费用户每月1GB，可通过订阅扩展。

### Q: 支持哪些协议？
A: HTTP、HTTPS、WebSocket。

---

## 对比FN Connect

| 功能 | nas-os | 飞牛FN Connect |
|------|:------:|:--------------:|
| 免费额度 | 1GB/月 | 类似 |
| HTTPS | ✅ | ✅ |
| 自定义域名 | 📋 规划 | ✅ |
| 三网优化 | 📋 规划 | ✅ |

---

> 更多文档请访问 [docs.openclaw.ai](https://docs.openclaw.ai)