# SUP平台 OpenAPI 规范文档

## 文档结构

```
docs/openapi/
├── README.md          # 本说明文件
└── openapi.yaml       # OpenAPI 3.0.3 完整规范文档
```

## 文档概览

`openapi.yaml` 包含 SUP聚合供货平台的完整 API 规范，基于 OpenAPI 3.0.3 标准编写。

### 覆盖的 API 模块

| 模块 | 前缀 | 认证方式 | 说明 |
|------|------|----------|------|
| Health | `/health`, `/readiness` | 无 | 健康检查探针 |
| Auth | `/auth` | 无 | 登录认证 |
| Admin | `/admin/*` | JWT Bearer | 管理后台全部功能 |
| TenantAdmin | `/tenant-admin/*` | JWT Bearer | 租户管理（多租户模式） |
| OpenAPI-Customer | `/openapi/customer/*` | Legacy签名 | 客户端API（兼容亿乐协议） |
| OpenAPI-Supplier | `/openapi/supplier/*` | Legacy签名 | 供货商API |

### 认证方式

1. **JWT Bearer** — 管理后台使用
   ```
   Authorization: Bearer <token>
   ```
   通过 `POST /auth/login` 获取 token。

2. **Legacy签名** — OpenAPI端点使用
   ```
   AppId: <应用ID>
   AppTimestamp: <Unix时间戳>
   AppToken: SHA1(AppId + AppSecret + RequestURI + Timestamp)
   AppNonce: <随机字符串>  (可选)
   ```

## 如何使用

### Swagger UI 在线预览

开发环境启动后，访问：
```
http://localhost:8080/swagger/index.html
```

### 导入 Postman

1. 打开 Postman → Import
2. 选择 `docs/openapi/openapi.yaml` 文件
3. 选择 "OpenAPI 3.0" 格式导入
4. 所有接口将自动按 Tag 分组

### 使用 Swagger Editor

1. 访问 https://editor.swagger.io/
2. File → Import File → 选择 `openapi.yaml`
3. 即可查看完整的交互式文档

### 使用 Redoc 渲染

```bash
npx @redocly/cli preview-docs docs/openapi/openapi.yaml
```

### 代码生成

可使用 OpenAPI Generator 生成客户端 SDK：
```bash
# 生成 TypeScript 客户端
npx @openapitools/openapi-generator-cli generate \
  -i docs/openapi/openapi.yaml \
  -g typescript-axios \
  -o generated/ts-client
```

## 如何更新维护

### 更新流程

1. **代码优先**：先修改 Go 代码中的 handler 和 Swagger 注释
2. **同步 OpenAPI**：将变更同步到 `openapi.yaml`
3. **验证格式**：确保 YAML 格式正确

### 验证工具

```bash
# 使用 redocly lint 验证
npx @redocly/cli lint docs/openapi/openapi.yaml

# 或使用 swagger-cli
npx swagger-cli validate docs/openapi/openapi.yaml
```

### 约定规则

- 所有响应统一使用 `{code, message, data}` 结构
- 金额字段使用 decimal 字符串（如 `"99.99"`）
- 时间字段使用 Unix 时间戳（秒）
- 分页参数：管理后台用 `page`+`size`，OpenAPI 用 `page`+`pageSize`
- HTTP 状态码统一 200，通过 `code` 字段区分业务结果

### 与 Swagger 文档的关系

- `docs/swagger.json` / `docs/swagger.yaml` — 由 swaggo 从代码注释自动生成（Swagger 2.0）
- `docs/openapi/openapi.yaml` — 手动维护的 OpenAPI 3.0.3 规范（更完整、更规范）

两者应保持接口一致，openapi.yaml 为权威版本。
