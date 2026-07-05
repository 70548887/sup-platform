# SUP聚合供货平台 - 后端

虚拟商品供货中台，兼容亿乐SUP API协议。支持多租户SaaS架构与私有化部署。

## 快速启动

### 前置要求
- Go 1.26+
- MySQL 8.0+
- Redis 7+（可选，支持降级运行，默认 `REDIS_ENABLED=false`）

### 数据库配置
```bash
# 创建数据库
mysql -u root -proot -e "CREATE DATABASE IF NOT EXISTS sup_platform DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

### 配置环境变量
```bash
cp .env.example .env
# 本地开发默认配置：
#   DB: root:root@localhost:3306/sup_platform
#   Redis: 禁用（REDIS_ENABLED=false）
#   Admin: admin / admin123456
```

### 编译运行
```bash
# 方式一：使用 Makefile
make run          # 启动API服务（go run ./cmd/api）
make worker       # 启动异步Worker
make migrate      # 执行数据库迁移

# 方式二：手动编译
go build -o sup-api.exe ./cmd/api
go build -o sup-worker.exe ./cmd/worker
go build -o sup-migrate.exe ./cmd/migrate

# 启动API服务（首次启动自动执行数据库迁移 + 创建默认管理员）
./sup-api.exe     # → http://localhost:8080
```

### API端点

| 端点 | 功能 |
|------|------|
| GET /health | 存活检查 |
| GET /readiness | 就绪检查（含DB/Redis状态） |
| POST /auth/login | 登录获取JWT |
| /admin/* | 管理后台API（需JWT） |
| /tenant-admin/* | 租户管理API（需JWT+RBAC） |
| /openapi/customer/* | 客户端API（Legacy签名认证） |
| /openapi/supplier/* | 供货商API（Legacy签名认证） |

### 默认管理员
- 用户名：admin
- 密码：admin123456（首次启动自动创建）

### 单元测试
```bash
make test         # 或 go test ./...
make lint         # 静态检查 go vet ./...
```

## 项目架构

```
sup-platform/
├── cmd/
│   ├── api/            # HTTP API服务入口
│   ├── worker/         # 异步任务Worker入口
│   ├── migrate/        # 数据库迁移工具
│   └── init-tenant/    # 租户初始化工具
├── internal/
│   ├── module/         # 17个业务模块
│   ├── http/           # HTTP路由与Handler
│   ├── adapter/        # 外部平台适配器（亿乐等）
│   ├── config/         # 配置管理
│   ├── pkg/            # 内部通用包
│   ├── repository/     # 数据访问层
│   └── service/        # 服务注册
├── migrations/         # 数据库迁移脚本
├── web/                # 嵌入式前端静态文件
├── docs/               # 文档（OpenAPI/部署/PRD）
└── deploy/             # 部署配置（Helm Chart）
```

## 技术栈
- **语言**: Go 1.26
- **Web框架**: Gin
- **ORM**: GORM + 关键路径手写事务SQL
- **数据库**: MySQL 8.0（utf8mb4）
- **缓存**: Redis 7（可选，支持降级）
- **队列**: Asynq（基于Redis Streams）
- **认证**: JWT + Legacy签名双模式
- **加密**: AES-GCM 卡密加密
- **架构**: 多租户SaaS，17个业务模块，62个Admin API端点

## 环境变量说明

| 变量 | 默认值 | 说明 |
|------|--------|------|
| APP_PORT | 8080 | 服务端口 |
| APP_MODE | debug | 运行模式（debug/release） |
| DB_HOST | localhost | MySQL地址 |
| DB_PORT | 3306 | MySQL端口 |
| DB_USER | root | MySQL用户 |
| DB_PASSWORD | root | MySQL密码 |
| DB_NAME | sup_platform | 数据库名 |
| REDIS_ENABLED | false | 是否启用Redis |
| REDIS_ADDR | localhost:6379 | Redis地址 |
| JWT_SECRET | (随机) | JWT签名密钥 |
| JWT_EXPIRE | 72 | JWT过期时间（小时） |
| MULTI_TENANT_ENABLED | false | 是否启用多租户 |
| ADMIN_USERNAME | admin | 初始管理员用户名 |
| ADMIN_PASSWORD | admin123456 | 初始管理员密码 |

## Docker部署

```bash
# 启动全部服务（MySQL + Redis + API + Worker）
docker-compose up -d

# 仅启动依赖服务
docker-compose up -d mysql redis
```

## API兼容

完整兼容亿乐SUP API协议，支持现有客户端和供货商无缝迁移。
# SUP 聚合供货平台 (ChuBaoSup)

虚拟商品供货中台，兼容亿乐SUP API协议。

## 技术栈
- Go 1.26 + Gin + GORM
- MySQL 8.0 + Redis 7
- Asynq (异步队列)
- Vue 3 + Element Plus (前端)

## 快速开始

### 开发环境
```bash
# 启动依赖服务
docker-compose up -d mysql redis

# 运行API服务
make run

# 运行Worker
make worker
```

### 生产部署
```bash
docker-compose up -d
```

## 项目结构
- `cmd/` - 程序入口
- `internal/` - 核心业务逻辑
- `internal/module/` - 业务模块
- `internal/adapter/` - 外部平台适配器
- `web/` - 前端项目
- `docs/` - 文档
- `deploy/` - 部署配置

## API兼容
完整兼容亿乐SUP API协议，支持现有客户端和供货商无缝迁移。
