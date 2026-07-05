# Kubernetes 原生部署

## 前置条件

- Kubernetes 1.25+
- kubectl 已配置集群访问
- 存储类（StorageClass）可用（用于数据库持久化，如使用外部数据库可忽略）
- 容器镜像仓库（Harbor / ACR / ECR 等）
- MySQL 8.0 实例（推荐使用云托管数据库）
- Redis 7+ 实例（推荐使用云托管 Redis）

## Namespace 规划

```bash
# 创建命名空间
kubectl create namespace sup-platform

# 设置默认命名空间
kubectl config set-context --current --namespace=sup-platform
```

建议环境隔离：

| Namespace | 用途 |
|-----------|------|
| `sup-dev` | 开发环境 |
| `sup-staging` | 预发布环境 |
| `sup-platform` | 生产环境 |

## ConfigMap 配置

非敏感配置使用 ConfigMap：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sup-platform-config
  namespace: sup-platform
data:
  APP_NAME: "sup-platform"
  APP_PORT: "8080"
  APP_MODE: "release"
  DB_HOST: "mysql.sup-platform.svc.cluster.local"
  DB_PORT: "3306"
  DB_USER: "root"
  DB_NAME: "sup_platform"
  REDIS_ADDR: "redis.sup-platform.svc.cluster.local:6379"
  REDIS_DB: "0"
  REDIS_ENABLED: "true"
  REDIS_PREFIX: "sup"
  JWT_EXPIRE: "72"
  MULTI_TENANT_ENABLED: "false"
  DEFAULT_TENANT_ID: "1"
```

## Secret 配置

敏感信息使用 Secret：

```bash
# 生成密钥
JWT_SECRET=$(openssl rand -hex 32)
CARD_ENCRYPT_KEY=$(openssl rand -hex 32)

kubectl create secret generic sup-platform-secret \
  --namespace=sup-platform \
  --from-literal=DB_PASSWORD='your-db-password' \
  --from-literal=REDIS_PASSWORD='your-redis-password' \
  --from-literal=JWT_SECRET="${JWT_SECRET}" \
  --from-literal=CARD_ENCRYPT_KEY="${CARD_ENCRYPT_KEY}" \
  --from-literal=ADMIN_PASSWORD='your-admin-password'
```

或使用 YAML 文件（base64 编码）：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sup-platform-secret
  namespace: sup-platform
type: Opaque
data:
  DB_PASSWORD: <base64-encoded>
  REDIS_PASSWORD: <base64-encoded>
  JWT_SECRET: <base64-encoded>
  CARD_ENCRYPT_KEY: <base64-encoded>
  ADMIN_PASSWORD: <base64-encoded>
```

## Deployment 配置

### API 服务

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sup-platform-api
  namespace: sup-platform
  labels:
    app: sup-platform
    component: api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: sup-platform
      component: api
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: sup-platform
        component: api
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: api
          image: your-registry/sup-platform:latest
          ports:
            - containerPort: 8080
              protocol: TCP
          envFrom:
            - configMapRef:
                name: sup-platform-config
            - secretRef:
                name: sup-platform-secret
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readiness
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 3
            failureThreshold: 3
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
            limits:
              cpu: "1"
              memory: 1Gi
```

### Worker 服务

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sup-platform-worker
  namespace: sup-platform
  labels:
    app: sup-platform
    component: worker
spec:
  replicas: 2
  selector:
    matchLabels:
      app: sup-platform
      component: worker
  template:
    metadata:
      labels:
        app: sup-platform
        component: worker
    spec:
      terminationGracePeriodSeconds: 30
      containers:
        - name: worker
          image: your-registry/sup-platform:latest
          command: ["/usr/local/bin/worker"]
          envFrom:
            - configMapRef:
                name: sup-platform-config
            - secretRef:
                name: sup-platform-secret
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
```

## Service 配置

```yaml
apiVersion: v1
kind: Service
metadata:
  name: sup-platform
  namespace: sup-platform
spec:
  type: ClusterIP
  selector:
    app: sup-platform
    component: api
  ports:
    - port: 8080
      targetPort: 8080
      protocol: TCP
```

## Ingress 配置

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: sup-platform
  namespace: sup-platform
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - sup.yourdomain.com
      secretName: sup-platform-tls
  rules:
    - host: sup.yourdomain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: sup-platform
                port:
                  number: 8080
```

## 数据库初始化

首次部署时，使用 Job 运行 `init-tenant` 初始化租户：

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: sup-platform-init-tenant
  namespace: sup-platform
spec:
  backoffLimit: 3
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: init-tenant
          image: your-registry/sup-platform:latest
          command: ["/usr/local/bin/init-tenant"]
          envFrom:
            - configMapRef:
                name: sup-platform-config
            - secretRef:
                name: sup-platform-secret
          env:
            - name: INIT_TENANT_NAME
              value: "我的公司"
            - name: ADMIN_USERNAME
              value: "admin"
```

```bash
# 执行初始化
kubectl apply -f job-init-tenant.yaml

# 查看 Job 状态
kubectl get jobs -n sup-platform
kubectl logs job/sup-platform-init-tenant -n sup-platform
```

## 健康检查端点

| 端点 | 用途 | 探针类型 |
|------|------|---------|
| `/health` | 进程存活检查 | Liveness |
| `/readiness` | 服务就绪检查（含依赖检查） | Readiness |

- **Liveness**: 检查失败时 kubelet 重启容器
- **Readiness**: 检查失败时从 Service endpoints 移除，不接收流量

## HPA 自动扩缩

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: sup-platform-api
  namespace: sup-platform
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: sup-platform-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

## 部署命令汇总

```bash
# 应用所有配置
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f deployment-api.yaml
kubectl apply -f deployment-worker.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml
kubectl apply -f hpa.yaml

# 检查部署状态
kubectl rollout status deployment/sup-platform-api -n sup-platform
kubectl get pods -n sup-platform

# 查看日志
kubectl logs -f deployment/sup-platform-api -n sup-platform
kubectl logs -f deployment/sup-platform-worker -n sup-platform
```
