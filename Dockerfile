# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 安装 git (下载依赖必备)
RUN apk add --no-cache git

# 2. 拷贝所有代码
COPY . .

# 3. 下载依赖 (使用 Go 官方代理，速度快)
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod tidy

# 4. 编译
# 只要 main.go 是修复版的，这里绝对不会再报错了
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 阶段二：运行环境
FROM alpine:latest

WORKDIR /app

# 安装证书和时区
RUN apk add --no-cache ca-certificates tzdata

# 拷贝编译结果
COPY --from=builder /app/webhook-app .
# 拷贝模板
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]