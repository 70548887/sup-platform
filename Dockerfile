# 多阶段构建
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git

# 下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /init-tenant ./cmd/init-tenant

# 运行镜像
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai

COPY --from=builder /api /usr/local/bin/api
COPY --from=builder /worker /usr/local/bin/worker
COPY --from=builder /init-tenant /usr/local/bin/init-tenant

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
