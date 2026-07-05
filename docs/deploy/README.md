# SUP平台部署指南

## 概述

SUP聚合供货平台支持多种部署方式，适用于从本地开发到生产环境的不同场景。

## 支持的部署方式

| 部署方式 | 适用场景 | 复杂度 |
|---------|---------|--------|
| Docker Compose | 本地开发、测试环境 | ⭐ |
| Kubernetes（原生） | 生产环境、需精细控制 | ⭐⭐⭐ |
| Helm Chart | 生产环境、标准化部署 | ⭐⭐ |

## 架构组件

```
┌─────────────┐     ┌─────────────┐
│   Ingress   │────▶│  API Server │ (Port 8080)
└─────────────┘     └──────┬──────┘
                           │
                    ┌──────┴──────┐
                    │             │
              ┌─────▼─────┐ ┌────▼────┐
              │   MySQL    │ │  Redis  │
              │   (8.0)    │ │(7-alpine)│
              └───────────┘ └─────────┘
                    │
              ┌─────▼─────┐
              │   Worker   │ (异步任务处理)
              └───────────┘
```

## 快速导航

- [Docker Compose 本地开发](./docker-compose.md)
- [Kubernetes 原生部署](./kubernetes.md)
- [Helm Chart 部署指南](./helm-guide.md)
- [部署前检查清单](./checklist.md)

## 镜像构建

项目使用多阶段构建，产出三个二进制文件：

| 二进制文件 | 用途 | 入口命令 |
|-----------|------|---------|
| `api` | HTTP API 服务 | 默认 ENTRYPOINT |
| `worker` | 异步任务处理 | `worker` |
| `init-tenant` | 租户初始化 | `init-tenant` |

```bash
# 构建镜像
docker build -t sup-platform:latest .
```

## 环境要求

- Docker 24+
- Kubernetes 1.25+（K8s部署）
- Helm 3.12+（Helm部署）
- MySQL 8.0
- Redis 7+
