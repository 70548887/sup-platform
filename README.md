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
