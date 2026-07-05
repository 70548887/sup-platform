# 部署前检查清单

在执行生产环境部署之前，请逐项确认以下检查项。

## 环境变量验证

### 必需项（APP_MODE=release 时）

- [ ] `DB_PASSWORD` — 已设置强密码，非默认值
- [ ] `REDIS_PASSWORD` — 已设置（如 Redis 开启认证）
- [ ] `JWT_SECRET` — 已使用 `openssl rand -hex 32` 生成
- [ ] `CARD_ENCRYPT_KEY` — 已使用 `openssl rand -hex 32` 生成
- [ ] `ADMIN_PASSWORD` — 已设置强密码
- [ ] `APP_MODE` — 设置为 `release`

### 验证命令

```bash
# K8s 环境检查 Secret 是否创建
kubectl get secret sup-platform-secret -n sup-platform -o jsonpath='{.data}' | jq 'keys'

# 确认没有遗留默认值
kubectl get secret sup-platform-secret -n sup-platform -o jsonpath='{.data.DB_PASSWORD}' | base64 -d
# 输出不应为 "CHANGE_IN_PRODUCTION" 或 "root123"
```

## 数据库

- [ ] MySQL 8.0 实例已就绪
- [ ] 数据库 `sup_platform` 已创建
- [ ] 数据库用户权限已配置
- [ ] 连接测试通过

```bash
# 从集群内 Pod 测试连接
kubectl run mysql-test --rm -it --image=mysql:8.0 --restart=Never -- \
  mysql -h <DB_HOST> -u <DB_USER> -p<DB_PASSWORD> -e "SELECT 1"
```

### 数据库迁移

- [ ] 迁移 Job 已执行成功
- [ ] 数据表结构已验证

```bash
# Helm 部署会自动执行迁移 Job
kubectl get job -n sup-platform | grep migrate

# 查看迁移日志
kubectl logs job/sup-migrate -n sup-platform
```

### 租户初始化

- [ ] `init-tenant` 已执行（首次部署）
- [ ] 管理员账户可正常登录

## Redis

- [ ] Redis 7+ 实例已就绪
- [ ] 连接测试通过
- [ ] 密码已正确配置

```bash
# 测试 Redis 连接
kubectl run redis-test --rm -it --image=redis:7-alpine --restart=Never -- \
  redis-cli -h <REDIS_ADDR> -a <REDIS_PASSWORD> ping
```

## 镜像构建

- [ ] 镜像构建成功（无编译错误）
- [ ] 镜像已推送到目标仓库
- [ ] 镜像标签正确（非 latest 用于生产）
- [ ] `imagePullSecrets` 已配置（如使用私有仓库）

```bash
# 验证镜像
docker build -t sup-platform:v1.0.0 .
docker push your-registry/sup-platform:v1.0.0

# 验证拉取
kubectl run pull-test --rm -it --image=your-registry/sup-platform:v1.0.0 --restart=Never -- /usr/local/bin/api --version
```

## 健康检查

- [ ] `/health` 端点返回 200（Liveness）
- [ ] `/readiness` 端点返回 200（Readiness）
- [ ] 探针配置已验证

```bash
# 部署后验证
kubectl port-forward svc/sup-platform 8080:8080 -n sup-platform &
curl http://localhost:8080/health
curl http://localhost:8080/readiness
```

## 网络配置

- [ ] Service 创建成功
- [ ] Ingress 配置正确（如启用）
- [ ] TLS 证书有效（如启用 HTTPS）
- [ ] 域名 DNS 已解析

```bash
# 验证 Service
kubectl get svc -n sup-platform
kubectl get endpoints -n sup-platform

# 验证 Ingress
kubectl get ingress -n sup-platform
kubectl describe ingress sup-platform -n sup-platform
```

## 高可用配置

- [ ] API 副本数 >= 3
- [ ] HPA 已启用，CPU 阈值 70%
- [ ] PDB 已配置，minAvailable = 2
- [ ] 跨节点分布（反亲和性）

```bash
# 检查 Pod 分布
kubectl get pods -n sup-platform -o wide

# 检查 HPA 状态
kubectl get hpa -n sup-platform

# 检查 PDB
kubectl get pdb -n sup-platform
```

## 日志收集

- [ ] 应用日志输出到 stdout/stderr
- [ ] 日志收集组件已部署（Fluentd/Fluent Bit/Loki 等）
- [ ] 日志可通过统一平台查看

```bash
# 验证日志输出
kubectl logs deployment/sup-api -n sup-platform --tail=20
kubectl logs deployment/sup-worker -n sup-platform --tail=20
```

## 监控告警

- [ ] Prometheus metrics 端点可用（如已集成）
- [ ] Grafana Dashboard 已配置
- [ ] 关键告警规则已设置（Pod 重启、高错误率、高延迟）

## 备份策略

- [ ] 数据库定期备份已配置
- [ ] 备份恢复已验证
- [ ] Secret/ConfigMap 已纳入版本管理（加密存储）

## 最终确认

| 检查项 | 状态 |
|--------|------|
| 所有环境变量已配置 | ☐ |
| 数据库连接正常 | ☐ |
| Redis 连接正常 | ☐ |
| 镜像构建推送成功 | ☐ |
| 健康检查端点正常 | ☐ |
| 网络访问正常 | ☐ |
| 高可用配置就绪 | ☐ |
| 日志收集正常 | ☐ |

**全部通过后方可执行生产部署。**
