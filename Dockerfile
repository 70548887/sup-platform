FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api
RUN CGO_ENABLED=0 go build -o /worker ./cmd/worker

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /api /usr/local/bin/api
COPY --from=builder /worker /usr/local/bin/worker
EXPOSE 8080
CMD ["api"]
