# 阶段一：编译
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 拷贝依赖描述文件
COPY go.mod ./

# 2. 下载依赖 (使用官方源，确保无代理问题)
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod tidy

# 3. 拷贝源代码
COPY main.go ./

# 4. 编译 (极简模式：去掉所有可能导致报错的 flags)
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 阶段二：运行
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/webhook-app .
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]