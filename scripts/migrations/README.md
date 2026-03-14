# Database Migration Scripts
# SQLite 数据库迁移管理

本目录包含 NAS-OS 的数据库迁移脚本。

## 迁移命名规范

```
V{version}__{description}.sql
```

例如：
- `V001__init_schema.sql` - 初始化数据库结构
- `V002__add_users_table.sql` - 添加用户表
- `V003__add_storage_pools.sql` - 添加存储池表

## 执行顺序

迁移脚本按版本号顺序执行，每个脚本只执行一次。

## 回滚脚本

回滚脚本放在 `rollback/` 子目录：
```
rollback/V002__add_users_table_rollback.sql
```

## 使用方法

```bash
# 执行所有待执行的迁移
./scripts/migrate.sh up

# 回滚最后一次迁移
./scripts/migrate.sh down

# 查看迁移状态
./scripts/migrate.sh status

# 创建新的迁移文件
./scripts/migrate.sh create "add_new_feature"
```