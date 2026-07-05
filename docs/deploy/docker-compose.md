# Docker Compose 本地开发部署

## 快速启动

```bash
# 1. 复制环境变量
cp .env.example .env

# 2. 启动所有服务
docker compose up -d

# 3. 初始化租户（首次部署）
docker compose exec api init-tenant

# 4. 查看服务状态
docker compose ps
```

启动后访问：`http://localhost:8080`

## 服务说明

| 服务 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| api | 本地构建 | 8080 | HTTP API 服务，主入口 |
| worker | 本地构建 | - | 异步任务处理（无外部端口） |
| mysql | mysql:8.0 | 3306 | 主数据库 |
| redis | redis:7-alpine | 6379 | 缓存和队列 |

## 环境变量配置

### 应用配置

| 变量 | 默认值 | 说明 |
|------|-------|------|
| `APP_NAME` | sup-platform | 应用名称 |
| `APP_PORT` | 8080 | 应用监听端口 |
| `APP_MODE` | release | 运行模式（debug/release） |

### 数据库配置

| 变量 | 默认值 | 说明 |
|------|-------|------|
| `DB_HOST` | mysql | 数据库主机 |
| `DB_PORT` | 3306 | 数据库端口 |
| `DB_USER` | root | 数据库用户 |
| `DB_PASSWORD` | - | 数据库密码（必须修改） |
| `DB_NAME` | sup_platform | 数据库名称 |

### Redis配置

| 变量 | 默认值 | 说明 |
|------|-------|------|
| `REDIS_ADDR` | redis:6379 | Redis 地址 |
| `REDIS_PASSWORD` | - | Redis 密码 |
| `REDIS_DB` | 0 | Redis 数据库编号 |
| `REDIS_ENABLED` | true | 是否启用 Redis |
| `REDIS_PREFIX` | sup | Key 前缀 |

### 安全配置

| 变量 | 默认值 | 说明 |
|------|-------|------|
| `JWT_SECRET` | - | JWT签名密钥（必须修改） |
| `JWT_EXPIRE` | 72 | Token过期时间（小时） |
| `CARD_ENCRYPT_KEY` | - | 卡密加密密钥（必须修改） |

### 多租户配置

| 变量 | 默认值 | 说明 |
|------|-------|------|
| `MULTI_TENANT_ENABLED` | false | 是否启用多租户 |
| `DEFAULT_TENANT_ID` | 1 | 默认租户ID |

## 数据持久化

Docker Compose 使用命名卷持久化数据：

| 卷名 | 挂载路径 | 说明 |
|------|---------|------|
| `mysql_data` | /var/lib/mysql | MySQL 数据文件 |
| `redis_data` | /data | Redis 持久化数据 |

```bash
# 查看卷
docker volume ls | grep sup

# 备份 MySQL 数据
docker compose exec mysql mysqldump -u root -p sup_platform > backup.sql

# 恢复数据
docker compose exec -T mysql mysql -u root -p sup_platform < backup.sql
```

## 常用命令

```bash
# 查看日志
docker compose logs -f api
docker compose logs -f worker

# 重启单个服务
docker compose restart api

# 重新构建并启动
docker compose up -d --build

# 停止所有服务
docker compose down

# 停止并删除数据卷（⚠️ 慎用）
docker compose down -v
```

## 常见问题

### Q: 启动后无法连接数据库

MySQL 服务启动需要一定时间，`api` 服务通过 `depends_on` + `healthcheck` 确保 MySQL 就绪后才启动。如仍有问题：

```bash
# 检查 MySQL 健康状态
docker compose exec mysql mysqladmin ping -h localhost

# 手动等待 MySQL 就绪后重启 API
docker compose restart api
```

### Q: 端口冲突

修改 `.env` 中的端口映射：

```bash
API_PORT=9080
DB_PORT=13306
REDIS_PORT=16379
```

### Q: 如何进入容器调试

```bash
# 进入 API 容器
docker compose exec api sh

# 进入 MySQL
docker compose exec mysql mysql -u root -p
```

### Q: Worker 服务有什么用

Worker 处理异步任务（如订单回调、通知发送等），使用与 API 相同的镜像但以不同命令启动：

```yaml
command: worker
```
