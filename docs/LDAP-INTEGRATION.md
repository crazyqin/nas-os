# LDAP/AD 集成指南

**版本**: v2.31.0  
**更新日期**: 2026-03-15

---

## 📋 目录

1. [概述](#概述)
2. [支持的服务器类型](#支持的服务器类型)
3. [配置步骤](#配置步骤)
4. [OpenLDAP 配置](#openldap-配置)
5. [Active Directory 配置](#active-directory-配置)
6. [FreeIPA 配置](#freeipa-配置)
7. [用户认证流程](#用户认证流程)
8. [组同步](#组同步)
9. [API 参考](#api-参考)
10. [故障排除](#故障排除)

---

## 概述

NAS-OS 支持与 LDAP（轻量级目录访问协议）和 Active Directory 集成，实现统一身份认证。通过 LDAP 集成，您可以：

- 使用现有企业用户账号登录 NAS-OS
- 自动同步用户组和权限
- 集中管理用户身份
- 支持单点登录 (SSO) 场景

### 功能特性

| 功能 | 说明 |
|------|------|
| 多服务器支持 | 支持配置多个 LDAP/AD 服务器 |
| LDAPS/TLS | 支持安全连接和证书验证 |
| 用户同步 | 自动同步用户信息 |
| 组同步 | 自动同步用户组成员关系 |
| 属性映射 | 灵活的用户属性映射配置 |

---

## 支持的服务器类型

NAS-OS 支持以下 LDAP 服务器：

### OpenLDAP

开源 LDAP 实现，适用于 Linux/Unix 环境。

### Active Directory (AD)

微软企业目录服务，适用于 Windows 环境。

### FreeIPA

Red Hat 开发的身份管理解决方案，集成了 LDAP 和 Kerberos。

---

## 配置步骤

### 1. 准备工作

在配置 LDAP 集成前，请确保：

- LDAP/AD 服务器已正确配置并运行
- 拥有绑定账号（Bind DN）和密码
- 确认 LDAP 服务器地址和端口
- 准备好 CA 证书（如使用 LDAPS）

### 2. 添加 LDAP 配置

通过 Web 界面或 API 添加 LDAP 服务器配置：

**Web 界面**:
1. 登录 NAS-OS 管理界面
2. 导航到「设置」→「安全设置」→「LDAP 集成」
3. 点击「添加服务器」
4. 填写服务器信息并保存

**API 方式**:
```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "company-ldap",
    "url": "ldap://ldap.example.com:389",
    "bind_dn": "cn=admin,dc=example,dc=com",
    "bind_password": "your-password",
    "base_dn": "dc=example,dc=com",
    "user_filter": "(uid=%s)",
    "group_filter": "(memberUid=%s)",
    "enabled": true
  }'
```

### 3. 测试连接

保存配置后，测试连接是否正常：

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs/company-ldap/test \
  -H "Authorization: Bearer TOKEN"
```

### 4. 测试用户登录

```bash
curl -X POST http://localhost:8080/api/v1/ldap/auth \
  -H "Content-Type: application/json" \
  -d '{
    "config_name": "company-ldap",
    "username": "testuser",
    "password": "user-password"
  }'
```

---

## OpenLDAP 配置

### 基本配置参数

| 参数 | 说明 | 示例值 |
|------|------|--------|
| URL | LDAP 服务器地址 | `ldap://ldap.example.com:389` 或 `ldaps://ldap.example.com:636` |
| Bind DN | 绑定账号 DN | `cn=admin,dc=example,dc=com` |
| Bind Password | 绑定账号密码 | `your-password` |
| Base DN | 搜索基础 DN | `dc=example,dc=com` |
| User Filter | 用户搜索过滤器 | `(uid=%s)` |
| Group Filter | 组搜索过滤器 | `(memberUid=%s)` |

### 属性映射配置

```json
{
  "attribute_map": {
    "username": "uid",
    "email": "mail",
    "first_name": "givenName",
    "last_name": "sn",
    "full_name": "cn",
    "groups": "memberOf"
  }
}
```

### 完整配置示例

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "openldap-server",
    "url": "ldaps://ldap.example.com:636",
    "bind_dn": "cn=admin,dc=example,dc=com",
    "bind_password": "admin-password",
    "base_dn": "ou=users,dc=example,dc=com",
    "user_filter": "(uid=%s)",
    "group_filter": "(memberUid=%s)",
    "attribute_map": {
      "username": "uid",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn",
      "full_name": "cn",
      "groups": "memberOf"
    },
    "use_tls": true,
    "skip_tls_verify": false,
    "ca_cert_path": "/etc/ssl/certs/ldap-ca.crt",
    "enabled": true,
    "is_ad": false
  }'
```

### 使用 StartTLS

如果您的 OpenLDAP 服务器支持 StartTLS：

```json
{
  "url": "ldap://ldap.example.com:389",
  "use_tls": false,
  "skip_tls_verify": false
}
```

系统会自动在连接后发起 StartTLS 升级。

---

## Active Directory 配置

### AD 特殊配置

Active Directory 使用不同的属性名称，配置时需注意：

| 参数 | AD 默认值 |
|------|-----------|
| User Filter | `(sAMAccountName=%s)` |
| Group Filter | `(member=%s)` |
| Username 属性 | `sAMAccountName` |
| Full Name 属性 | `displayName` |

### 完整配置示例

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ad-server",
    "url": "ldaps://ad.example.com:636",
    "bind_dn": "CN=ldap-bind,CN=Users,DC=example,DC=com",
    "bind_password": "bind-password",
    "base_dn": "DC=example,DC=com",
    "user_filter": "(sAMAccountName=%s)",
    "group_filter": "(member=%s)",
    "attribute_map": {
      "username": "sAMAccountName",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn",
      "full_name": "displayName",
      "groups": "memberOf"
    },
    "use_tls": true,
    "skip_tls_verify": false,
    "enabled": true,
    "is_ad": true
  }'
```

### AD 域控制器配置

对于 AD 环境，建议使用 LDAPS (端口 636) 确保安全：

1. 从域控制器导出 CA 证书
2. 将证书放置在 NAS-OS 服务器上
3. 配置 `ca_cert_path` 指向证书文件

### 用户 Principal Name (UPN)

AD 用户可使用 UPN 格式登录（如 `user@example.com`）。如需支持，请调整用户过滤器：

```
(|(sAMAccountName=%s)(userPrincipalName=%s))
```

---

## FreeIPA 配置

FreeIPA 是 Red Hat 开发的身份管理解决方案，集成了 LDAP 和 Kerberos。

### 配置示例

```bash
curl -X POST http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "freeipa-server",
    "url": "ldaps://ipa.example.com:636",
    "bind_dn": "uid=admin,cn=users,cn=accounts,dc=example,dc=com",
    "bind_password": "admin-password",
    "base_dn": "cn=users,cn=accounts,dc=example,dc=com",
    "user_filter": "(uid=%s)",
    "group_filter": "(member=%s)",
    "attribute_map": {
      "username": "uid",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn",
      "full_name": "cn",
      "groups": "memberOf"
    },
    "use_tls": true,
    "enabled": true,
    "is_ad": false
  }'
```

### FreeIPA 用户组

FreeIPA 用户组位于 `cn=groups,cn=accounts,dc=example,dc=com`。如需搜索组：

```
Base DN: cn=groups,cn=accounts,dc=example,dc=com
Group Filter: (member=%s)
```

---

## 用户认证流程

### 认证流程图

```
用户输入用户名密码
        ↓
查找 LDAP 配置
        ↓
连接 LDAP 服务器 (LDAPS/StartTLS)
        ↓
绑定管理员账号
        ↓
搜索用户 DN
        ↓
使用用户 DN 和密码验证
        ↓
获取用户属性和组
        ↓
返回认证结果
```

### 认证 API

```bash
POST /api/v1/ldap/auth
Content-Type: application/json

{
  "config_name": "company-ldap",
  "username": "zhangsan",
  "password": "user-password"
}
```

**成功响应**:

```json
{
  "code": 0,
  "message": "认证成功",
  "data": {
    "username": "zhangsan",
    "email": "zhangsan@example.com",
    "first_name": "San",
    "last_name": "Zhang",
    "full_name": "Zhang San",
    "groups": ["developers", "admins"],
    "dn": "uid=zhangsan,ou=users,dc=example,dc=com"
  }
}
```

**失败响应**:

```json
{
  "code": 401,
  "message": "LDAP 认证失败",
  "data": null
}
```

---

## 组同步

### 自动组映射

NAS-OS 支持将 LDAP 组自动映射为系统角色：

```bash
curl -X POST http://localhost:8080/api/v1/ldap/group-mappings \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "config_name": "company-ldap",
    "ldap_group": "nas-admins",
    "nas_role": "admin"
  }'
```

### 查看组映射

```bash
curl http://localhost:8080/api/v1/ldap/group-mappings \
  -H "Authorization: Bearer TOKEN"
```

### 删除组映射

```bash
curl -X DELETE http://localhost:8080/api/v1/ldap/group-mappings/{mapping_id} \
  -H "Authorization: Bearer TOKEN"
```

---

## API 参考

### 配置管理 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/ldap/configs` | GET | 获取所有 LDAP 配置 |
| `/api/v1/ldap/configs` | POST | 创建 LDAP 配置 |
| `/api/v1/ldap/configs/{name}` | GET | 获取指定配置 |
| `/api/v1/ldap/configs/{name}` | PUT | 更新配置 |
| `/api/v1/ldap/configs/{name}` | DELETE | 删除配置 |
| `/api/v1/ldap/configs/{name}/test` | POST | 测试连接 |

### 认证 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/ldap/auth` | POST | LDAP 用户认证 |
| `/api/v1/ldap/users/search` | GET | 搜索 LDAP 用户 |

### 示例请求

**获取所有配置**:
```bash
curl http://localhost:8080/api/v1/ldap/configs \
  -H "Authorization: Bearer TOKEN"
```

**更新配置**:
```bash
curl -X PUT http://localhost:8080/api/v1/ldap/configs/company-ldap \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "use_tls": true
  }'
```

**删除配置**:
```bash
curl -X DELETE http://localhost:8080/api/v1/ldap/configs/company-ldap \
  -H "Authorization: Bearer TOKEN"
```

**搜索用户**:
```bash
curl "http://localhost:8080/api/v1/ldap/users/search?config_name=company-ldap&query=zhang" \
  -H "Authorization: Bearer TOKEN"
```

---

## 故障排除

### 常见错误

#### 1. 连接失败 (ErrLDAPConnectionFailed)

**原因**:
- LDAP 服务器地址或端口错误
- 网络不通
- 防火墙阻止

**解决方法**:
```bash
# 测试网络连通性
telnet ldap.example.com 389

# 测试 LDAPS
openssl s_client -connect ldap.example.com:636
```

#### 2. 绑定失败 (ErrLDAPBindFailed)

**原因**:
- Bind DN 格式错误
- Bind 密码错误
- 账号被锁定或禁用

**解决方法**:
- 检查 Bind DN 格式是否正确
- 使用 `ldapsearch` 验证绑定：
```bash
ldapsearch -x -H ldap://ldap.example.com \
  -D "cn=admin,dc=example,dc=com" \
  -W -b "dc=example,dc=com"
```

#### 3. 用户未找到 (ErrLDAPUserNotFound)

**原因**:
- Base DN 错误
- User Filter 格式错误
- 用户不在搜索范围内

**解决方法**:
- 验证 Base DN 是否正确
- 检查 User Filter 中占位符格式是否为 `%s`

#### 4. 认证失败 (ErrLDAPAuthFailed)

**原因**:
- 用户密码错误
- 用户账号被锁定
- 密码策略限制

**解决方法**:
- 检查用户密码是否正确
- 确认用户账号状态

#### 5. TLS 证书错误

**原因**:
- CA 证书路径错误
- 证书过期或不匹配
- 服务器名称与证书不匹配

**解决方法**:
```bash
# 检查证书
openssl x509 -in /etc/ssl/certs/ldap-ca.crt -text -noout

# 测试证书链
openssl verify -CAfile /etc/ssl/certs/ldap-ca.crt server.crt
```

### 日志调试

启用 LDAP 调试日志：

```bash
# 设置日志级别
curl -X PUT http://localhost:8080/api/v1/logging/level \
  -H "Authorization: Bearer TOKEN" \
  -d '{"level": "debug"}'

# 查看 LDAP 相关日志
journalctl -u nas-os | grep -i ldap
```

### 性能优化

1. **使用连接池**: 系统自动管理 LDAP 连接池
2. **缓存用户信息**: 启用用户信息缓存减少查询
3. **优化搜索范围**: 使用更精确的 Base DN

```bash
# 配置缓存
curl -X PUT http://localhost:8080/api/v1/ldap/configs/company-ldap/cache \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "enabled": true,
    "ttl": "5m",
    "max_entries": 1000
  }'
```

---

## 安全建议

1. **始终使用 LDAPS 或 StartTLS** 加密传输
2. **使用专用绑定账号** 仅授予必要的读取权限
3. **定期轮换绑定密码**
4. **启用证书验证** 不跳过 TLS 验证
5. **限制可登录用户** 通过组过滤限制访问范围

---

## 相关文档

- [用户管理指南](USER_GUIDE.md)
- [API 文档](API_GUIDE.md)
- [安全管理指南](SECURITY-AUDIT-2026-03-14.md)

---

**最后更新**: 2026-03-15