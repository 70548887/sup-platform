# SUP平台 API 约定文档

## 1. 响应格式

所有API统一返回以下JSON结构：

```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

| code | 含义 |
|------|------|
| 0 | 成功 |
| 1 | 业务错误 |
| 2 | 参数错误 |
| 100 | 认证/授权错误 |
| 429 | 请求限流/配额耗尽 |

HTTP状态码统一为200，通过 `code` 字段区分业务结果。

## 2. 认证方式

### Legacy签名认证（/openapi/*）

适用于外部应用调用客户端/供货商API。

| Header | 说明 |
|--------|------|
| AppId / Appid | 应用ID |
| AppTimestamp / Apptimestamp | Unix时间戳（秒），5分钟有效期 |
| AppToken / Apptoken | 签名：SHA1(AppId + AppSecret + RequestURI + Timestamp) |
| AppNonce / Appnonce | （可选）随机字符串，用于防重放 |

带 `AppNonce` 时，签名算法变为：SHA1(AppId + AppSecret + RequestURI + Timestamp + Nonce)，同一Nonce在5分钟内不可重复使用。

### JWT认证（/admin、/tenant-admin）

管理后台使用 Bearer Token 认证。

| Header | 说明 |
|--------|------|
| Authorization | Bearer <JWT Token> |

JWT Claims 包含：UserID、Role（admin/supplier/customer）、TenantID。

## 3. 分页参数

### Legacy OpenAPI（/openapi/customer、/openapi/supplier）

与亿乐API保持兼容：

| 参数 | 类型 | 说明 |
|------|------|------|
| page | int | 页码，从1开始 |
| pageSize | int | 每页条数，默认20，最大100 |

响应中分页字段：`page`、`pageSize`、`total`。

### 管理后台（/admin、/tenant-admin）

| 参数 | 类型 | 说明 |
|------|------|------|
| page | int | 页码，从1开始 |
| size | int | 每页条数，默认20，最大100 |

响应中分页字段：`page`、`size`、`total`。

## 4. 限流与配额

- 限流触发时返回：`code=429, message="rate limit exceeded"`
- 配额耗尽时返回：`code=429, message="API配额已用尽，请升级套餐"`
- HTTP状态码仍为200，客户端应检查 `code` 字段判断

## 5. 多租户

- 多租户模式通过配置 `MULTI_TENANT_ENABLED=true` 开启
- `/tenant-admin` 端点自动按JWT中的TenantID过滤数据
- 租户角色：boss（全权限）、finance（财务）、ops（运维）、support（客服）
