# NAS-OS API 示例代码

**版本**: v2.78.0  
**更新日期**: 2026-03-16

---

## 概述

本文档提供 NAS-OS API 的完整代码示例，涵盖 Python、JavaScript、cURL 等多种语言和工具。

---

## 目录

- [认证示例](#认证示例)
- [存储管理示例](#存储管理示例)
- [用户管理示例](#用户管理示例)
- [共享管理示例](#共享管理示例)
- [监控示例](#监控示例)
- [备份管理示例](#备份管理示例)
- [容器管理示例](#容器管理示例)

---

## 认证示例

### cURL

```bash
# 登录获取 Token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your-password"}'

# 保存 Token 到变量
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"your-password"}' | jq -r '.data.token')

# 使用 Token 请求
curl http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN"
```

### Python

```python
import requests

class NASOSClient:
    def __init__(self, base_url, username, password):
        self.base_url = base_url
        self.token = None
        self.login(username, password)
    
    def login(self, username, password):
        """登录获取 Token"""
        response = requests.post(
            f"{self.base_url}/api/v1/auth/login",
            json={"username": username, "password": password}
        )
        response.raise_for_status()
        self.token = response.json()["data"]["token"]
    
    def get_headers(self):
        """获取认证头"""
        return {"Authorization": f"Bearer {self.token}"}
    
    def get_volumes(self):
        """获取卷列表"""
        response = requests.get(
            f"{self.base_url}/api/v1/volumes",
            headers=self.get_headers()
        )
        response.raise_for_status()
        return response.json()

# 使用示例
client = NASOSClient("http://localhost:8080", "admin", "your-password")
volumes = client.get_volumes()
print(volumes)
```

### JavaScript (Node.js)

```javascript
const axios = require('axios');

class NASOSClient {
  constructor(baseUrl, username, password) {
    this.baseUrl = baseUrl;
    this.token = null;
    this.client = axios.create({ baseURL: baseUrl });
  }

  async login(username, password) {
    const response = await this.client.post('/api/v1/auth/login', {
      username,
      password
    });
    this.token = response.data.data.token;
    this.client.defaults.headers.common['Authorization'] = `Bearer ${this.token}`;
  }

  async getVolumes() {
    const response = await this.client.get('/api/v1/volumes');
    return response.data;
  }
}

// 使用示例
(async () => {
  const client = new NASOSClient('http://localhost:8080');
  await client.login('admin', 'your-password');
  const volumes = await client.getVolumes();
  console.log(volumes);
})();
```

---

## 存储管理示例

### 创建 Btrfs 卷

#### cURL

```bash
# 创建单盘卷
curl -X POST http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "data",
    "devices": ["/dev/sda"],
    "profile": "single"
  }'

# 创建 RAID1 卷
curl -X POST http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "secure",
    "devices": ["/dev/sda", "/dev/sdb"],
    "profile": "raid1"
  }'

# 创建 RAID5 卷
curl -X POST http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "storage",
    "devices": ["/dev/sda", "/dev/sdb", "/dev/sdc"],
    "profile": "raid5"
  }'
```

#### Python

```python
def create_volume(client, name, devices, profile="single"):
    """创建存储卷"""
    response = requests.post(
        f"{client.base_url}/api/v1/volumes",
        headers=client.get_headers(),
        json={
            "name": name,
            "devices": devices,
            "profile": profile
        }
    )
    response.raise_for_status()
    return response.json()

# 使用示例
result = create_volume(client, "data", ["/dev/sda", "/dev/sdb"], "raid1")
print(f"Volume created: {result}")
```

### 创建和管理快照

#### Python

```python
def create_snapshot(client, volume_name, snapshot_name, readonly=True):
    """创建快照"""
    response = requests.post(
        f"{client.base_url}/api/v1/volumes/{volume_name}/snapshots",
        headers=client.get_headers(),
        json={
            "snapshot_name": snapshot_name,
            "readonly": readonly
        }
    )
    response.raise_for_status()
    return response.json()

def list_snapshots(client, volume_name):
    """列出快照"""
    response = requests.get(
        f"{client.base_url}/api/v1/volumes/{volume_name}/snapshots",
        headers=client.get_headers()
    )
    response.raise_for_status()
    return response.json()

def restore_snapshot(client, volume_name, snapshot_name):
    """恢复快照"""
    response = requests.post(
        f"{client.base_url}/api/v1/volumes/{volume_name}/snapshots/{snapshot_name}/restore",
        headers=client.get_headers()
    )
    response.raise_for_status()
    return response.json()

# 使用示例
create_snapshot(client, "data", "daily-20260316")
snapshots = list_snapshots(client, "data")
print(f"Snapshots: {snapshots}")
```

---

## 用户管理示例

### 批量创建用户

#### Python

```python
def batch_create_users(client, users):
    """批量创建用户"""
    response = requests.post(
        f"{client.base_url}/api/v1/users/batch",
        headers=client.get_headers(),
        json={"users": users}
    )
    response.raise_for_status()
    return response.json()

# 使用示例
users = [
    {"username": "user1", "password": "Pass123!", "role": "user"},
    {"username": "user2", "password": "Pass456!", "role": "operator"},
    {"username": "user3", "password": "Pass789!", "role": "readonly"}
]
result = batch_create_users(client, users)
print(f"Created {len(result['data']['created'])} users")
```

### 用户权限管理

#### cURL

```bash
# 获取用户权限
curl http://localhost:8080/api/v1/users/user-001/permissions \
  -H "Authorization: Bearer $TOKEN"

# 授予权限
curl -X POST http://localhost:8080/api/v1/users/user-001/permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"permission": "storage:admin"}'

# 撤销权限
curl -X DELETE http://localhost:8080/api/v1/users/user-001/permissions/storage:admin \
  -H "Authorization: Bearer $TOKEN"
```

---

## 共享管理示例

### 创建 SMB 共享

#### Python

```python
def create_smb_share(client, name, path, comment="", guest_ok=False):
    """创建 SMB 共享"""
    response = requests.post(
        f"{client.base_url}/api/v1/shares",
        headers=client.get_headers(),
        json={
            "name": name,
            "path": path,
            "type": "smb",
            "comment": comment,
            "guest_ok": guest_ok
        }
    )
    response.raise_for_status()
    return response.json()

# 使用示例
share = create_smb_share(
    client, 
    "public", 
    "/data/public", 
    comment="公共共享目录",
    guest_ok=True
)
print(f"Created SMB share: {share}")
```

### 创建 NFS 导出

#### cURL

```bash
# 创建 NFS 导出
curl -X POST http://localhost:8080/api/v1/shares \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "backup",
    "path": "/data/backup",
    "type": "nfs",
    "clients": [
      {
        "address": "192.168.1.0/24",
        "options": "rw,sync,no_root_squash"
      }
    ]
  }'
```

---

## 监控示例

### 获取系统状态

#### Python

```python
def get_system_stats(client):
    """获取系统统计"""
    response = requests.get(
        f"{client.base_url}/api/v1/monitor/stats",
        headers=client.get_headers()
    )
    response.raise_for_status()
    return response.json()

def get_disk_health(client):
    """获取磁盘健康状态"""
    response = requests.get(
        f"{client.base_url}/api/v1/monitor/disk-health",
        headers=client.get_headers()
    )
    response.raise_for_status()
    return response.json()

def get_alerts(client, active_only=True):
    """获取告警列表"""
    params = {"active_only": active_only} if active_only else {}
    response = requests.get(
        f"{client.base_url}/api/v1/monitor/alerts",
        headers=client.get_headers(),
        params=params
    )
    response.raise_for_status()
    return response.json()

# 使用示例
stats = get_system_stats(client)
print(f"CPU: {stats['data']['cpu_percent']}%")
print(f"Memory: {stats['data']['memory_percent']}%")

alerts = get_alerts(client)
print(f"Active alerts: {len(alerts['data']['alerts'])}")
```

### WebSocket 实时监控

#### JavaScript

```javascript
// 连接 WebSocket
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onopen = () => {
  console.log('WebSocket connected');
  // 订阅监控事件
  ws.send(JSON.stringify({
    action: 'subscribe',
    channels: ['monitor', 'alerts']
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  switch (data.type) {
    case 'monitor':
      updateDashboard(data.payload);
      break;
    case 'alert':
      showAlert(data.payload);
      break;
  }
};

function updateDashboard(stats) {
  document.getElementById('cpu').textContent = stats.cpu_percent + '%';
  document.getElementById('memory').textContent = stats.memory_percent + '%';
}

function showAlert(alert) {
  console.log(`[${alert.level}] ${alert.message}`);
}
```

---

## 备份管理示例

### 创建定时备份任务

#### Python

```python
def create_backup_schedule(client, name, source, dest, cron, retention="7d"):
    """创建定时备份任务"""
    response = requests.post(
        f"{client.base_url}/api/v1/backup/schedules",
        headers=client.get_headers(),
        json={
            "name": name,
            "source": source,
            "destination": dest,
            "cron": cron,
            "retention": retention,
            "incremental": True
        }
    )
    response.raise_for_status()
    return response.json()

# 使用示例
# 每天凌晨 2 点执行备份
schedule = create_backup_schedule(
    client,
    "daily-documents",
    "/data/documents",
    "/backup/documents",
    "0 2 * * *",
    "7d"
)
print(f"Backup schedule created: {schedule}")
```

### 执行即时备份

#### cURL

```bash
# 创建即时备份
curl -X POST http://localhost:8080/api/v1/backup/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "source": "/data/important",
    "destination": "/backup/important",
    "compression": "zstd",
    "encrypt": true
  }'

# 查看备份任务状态
curl http://localhost:8080/api/v1/backup/jobs/job-001/status \
  -H "Authorization: Bearer $TOKEN"
```

---

## 容器管理示例

### 创建 Docker 容器

#### Python

```python
def create_container(client, name, image, ports=None, volumes=None, env=None):
    """创建 Docker 容器"""
    response = requests.post(
        f"{client.base_url}/api/v1/containers",
        headers=client.get_headers(),
        json={
            "name": name,
            "image": image,
            "ports": ports or [],
            "volumes": volumes or [],
            "env": env or []
        }
    )
    response.raise_for_status()
    return response.json()

# 使用示例 - 创建 Nginx 容器
container = create_container(
    client,
    "nginx-web",
    "nginx:latest",
    ports=["80:80"],
    volumes=["/data/www:/usr/share/nginx/html"]
)
print(f"Container created: {container}")

# 启动容器
def start_container(client, container_id):
    response = requests.post(
        f"{client.base_url}/api/v1/containers/{container_id}/start",
        headers=client.get_headers()
    )
    response.raise_for_status()
    return response.json()

start_container(client, container['data']['id'])
```

### 容器批量操作

#### cURL

```bash
# 列出所有容器
curl http://localhost:8080/api/v1/containers \
  -H "Authorization: Bearer $TOKEN"

# 批量停止容器
curl -X POST http://localhost:8080/api/v1/containers/batch/stop \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"container_ids": ["container-1", "container-2"]}'

# 获取容器日志
curl "http://localhost:8080/api/v1/containers/nginx-web/logs?follow=false&tail=100" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 错误处理示例

### Python 错误处理

```python
import requests
from requests.exceptions import HTTPError, ConnectionError

class NASOSError(Exception):
    """NAS-OS API 错误"""
    def __init__(self, code, message):
        self.code = code
        self.message = message
        super().__init__(f"[{code}] {message}")

def safe_request(func):
    """安全请求装饰器"""
    def wrapper(*args, **kwargs):
        try:
            response = func(*args, **kwargs)
            response.raise_for_status()
            data = response.json()
            if data.get('code', 0) != 0:
                raise NASOSError(data['code'], data.get('message', 'Unknown error'))
            return data
        except HTTPError as e:
            if e.response.status_code == 401:
                raise NASOSError(401, "Authentication failed")
            raise
        except ConnectionError:
            raise NASOSError(-1, "Connection failed")
    return wrapper

# 使用示例
@safe_request
def get_volumes_safe(client):
    return requests.get(
        f"{client.base_url}/api/v1/volumes",
        headers=client.get_headers()
    )

try:
    volumes = get_volumes_safe(client)
except NASOSError as e:
    print(f"Error: {e}")
```

---

## 完整示例应用

### Python 监控脚本

```python
#!/usr/bin/env python3
"""
NAS-OS 监控脚本
定期检查系统状态并发送告警
"""

import time
import requests
from datetime import datetime

class NASMonitor:
    def __init__(self, base_url, username, password):
        self.base_url = base_url
        self.token = None
        self.login(username, password)
    
    def login(self, username, password):
        response = requests.post(
            f"{self.base_url}/api/v1/auth/login",
            json={"username": username, "password": password}
        )
        self.token = response.json()["data"]["token"]
    
    def get_headers(self):
        return {"Authorization": f"Bearer {self.token}"}
    
    def check_disk_usage(self, threshold=80):
        """检查磁盘使用率"""
        response = requests.get(
            f"{self.base_url}/api/v1/volumes",
            headers=self.get_headers()
        )
        alerts = []
        for volume in response.json()["data"]["volumes"]:
            if volume["usage_percent"] > threshold:
                alerts.append({
                    "type": "disk_usage",
                    "volume": volume["name"],
                    "usage": volume["usage_percent"],
                    "message": f"Volume {volume['name']} usage: {volume['usage_percent']}%"
                })
        return alerts
    
    def check_system_health(self):
        """检查系统健康状态"""
        response = requests.get(
            f"{self.base_url}/api/v1/monitor/stats",
            headers=self.get_headers()
        )
        data = response.json()["data"]
        return {
            "cpu": data["cpu_percent"],
            "memory": data["memory_percent"],
            "uptime": data["uptime"]
        }

# 运行监控
if __name__ == "__main__":
    monitor = NASMonitor(
        "http://localhost:8080",
        "admin",
        "your-password"
    )
    
    print(f"[{datetime.now()}] Starting monitor...")
    
    while True:
        # 检查磁盘
        alerts = monitor.check_disk_usage(threshold=80)
        if alerts:
            for alert in alerts:
                print(f"ALERT: {alert['message']}")
        
        # 检查系统
        health = monitor.check_system_health()
        print(f"CPU: {health['cpu']}%, Memory: {health['memory']}%")
        
        time.sleep(60)  # 每分钟检查一次
```

---

## 相关文档

- [API 文档索引](README.md) - API 模块文档
- [API 使用指南](../API_GUIDE.md) - 完整 API 参考
- [Swagger UI](../swagger/) - 交互式 API 文档

---

*最后更新：2026-03-16*