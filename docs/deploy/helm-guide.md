# Helm 部署指南

## 前置条件

- Kubernetes 1.25+
- Helm 3.12+
- 容器镜像已推送到镜像仓库
- MySQL 8.0 和 Redis 7+ 实例就绪

## 快速安装

```bash
# 基础安装（使用默认值）
helm install sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  --create-namespace

# 指定自定义配置
helm install sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  --create-namespace \
  -f ./deploy/helm/sup-platform/values-prod.yaml

# 覆盖特定参数
helm install sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  --create-namespace \
  --set image.repository=your-registry/sup-platform \
  --set image.tag=v1.0.0 \
  --set secrets.dbPassword=your-password \
  --set secrets.jwtSecret=your-jwt-secret
```

## values.yaml 参数说明

### 镜像配置

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `image.repository` | sup-platform | 镜像仓库地址 |
| `image.tag` | latest | 镜像标签 |
| `image.pullPolicy` | IfNotPresent | 拉取策略 |
| `imagePullSecrets` | [] | 镜像拉取凭证 |

### API 服务

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `replicaCount` | 3 | API 副本数 |
| `resources.requests.cpu` | 500m | CPU 请求 |
| `resources.requests.memory` | 512Mi | 内存请求 |
| `resources.limits.cpu` | 1000m | CPU 上限 |
| `resources.limits.memory` | 1Gi | 内存上限 |
| `terminationGracePeriodSeconds` | 30 | 优雅终止等待时间 |

### Worker 服务

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `worker.enabled` | true | 是否启用 Worker |
| `worker.replicaCount` | 2 | Worker 副本数 |
| `worker.resources.requests.cpu` | 250m | CPU 请求 |
| `worker.resources.requests.memory` | 256Mi | 内存请求 |
| `worker.resources.limits.cpu` | 500m | CPU 上限 |
| `worker.resources.limits.memory` | 512Mi | 内存上限 |

### Service 配置

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `service.type` | ClusterIP | Service 类型 |
| `service.port` | 8080 | 服务端口 |

### Ingress 配置

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `ingress.enabled` | false | 是否启用 Ingress |
| `ingress.className` | nginx | Ingress Class |
| `ingress.host` | sup.local | 域名 |
| `ingress.tls.enabled` | false | 是否启用 TLS |
| `ingress.tls.secretName` | "" | TLS 证书 Secret |

### 健康检查

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `probes.liveness.path` | /health | 存活探针路径 |
| `probes.liveness.initialDelaySeconds` | 10 | 初始延迟 |
| `probes.readiness.path` | /readiness | 就绪探针路径 |
| `probes.readiness.initialDelaySeconds` | 5 | 初始延迟 |

### 自动扩缩 (HPA)

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `hpa.enabled` | true | 是否启用 HPA |
| `hpa.minReplicas` | 3 | 最小副本数 |
| `hpa.maxReplicas` | 10 | 最大副本数 |
| `hpa.targetCPU` | 70 | CPU 目标利用率(%) |

### Pod 中断预算 (PDB)

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `pdb.enabled` | true | 是否启用 PDB |
| `pdb.minAvailable` | 2 | 最小可用 Pod 数 |

### 数据库迁移 Job

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `migration.enabled` | true | 是否执行迁移 |

### 环境变量配置

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `config.appMode` | release | 运行模式 |
| `config.dbHost` | mysql | MySQL 地址 |
| `config.dbPort` | "3306" | MySQL 端口 |
| `config.dbUser` | root | MySQL 用户 |
| `config.dbName` | sup_platform | 数据库名 |
| `config.redisAddr` | redis:6379 | Redis 地址 |
| `config.redisDB` | "0" | Redis DB |
| `config.redisEnabled` | "true" | 启用 Redis |
| `config.redisPrefix` | sup | Key 前缀 |
| `config.jwtExpire` | "72" | Token 过期(小时) |

### Secrets

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `secrets.dbPassword` | "" | MySQL 密码 |
| `secrets.redisPassword` | "" | Redis 密码 |
| `secrets.jwtSecret` | "" | JWT 密钥 |
| `secrets.cardEncryptKey` | "" | 卡密加密密钥 |
| `secrets.adminPassword` | "" | 管理员密码 |

## 多环境配置

### 开发环境

```bash
helm install sup-dev ./deploy/helm/sup-platform \
  --namespace sup-dev \
  --create-namespace \
  --set replicaCount=1 \
  --set worker.replicaCount=1 \
  --set hpa.enabled=false \
  --set pdb.enabled=false \
  --set config.appMode=debug
```

### 预发布环境

```bash
helm install sup-staging ./deploy/helm/sup-platform \
  --namespace sup-staging \
  --create-namespace \
  --set replicaCount=2 \
  --set image.tag=staging-latest \
  --set ingress.enabled=true \
  --set ingress.host=staging-sup.yourdomain.com
```

### 生产环境

```bash
helm install sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  --create-namespace \
  -f ./deploy/helm/sup-platform/values-prod.yaml
```

## 升级流程

```bash
# 查看当前版本
helm list -n sup-platform

# 升级（修改镜像标签）
helm upgrade sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  --set image.tag=v1.1.0

# 使用新的 values 文件升级
helm upgrade sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  -f ./deploy/helm/sup-platform/values-prod.yaml

# 查看升级历史
helm history sup -n sup-platform
```

## 回滚操作

```bash
# 查看发布历史
helm history sup -n sup-platform

# 回滚到上一版本
helm rollback sup -n sup-platform

# 回滚到指定版本
helm rollback sup 3 -n sup-platform

# 验证回滚状态
kubectl rollout status deployment/sup-api -n sup-platform
```

## 卸载

```bash
# 卸载 release
helm uninstall sup -n sup-platform

# 如需删除命名空间
kubectl delete namespace sup-platform
```

## 调试

```bash
# 渲染模板但不安装（dry-run）
helm install sup ./deploy/helm/sup-platform \
  --namespace sup-platform \
  --dry-run --debug

# 验证 Chart
helm lint ./deploy/helm/sup-platform

# 查看生成的清单
helm template sup ./deploy/helm/sup-platform
```
