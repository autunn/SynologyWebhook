# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 先拷贝所有文件 (go.mod, main.go 等)
# 必须先有代码，go mod tidy 才知道你需要什么包
COPY . .

# 2. 下载依赖
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod tidy

# 3. 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 阶段二：运行环境
FROM alpine:latest

WORKDIR /app

# 安装证书和时区
RUN apk add --no-cache ca-certificates tzdata

# 拷贝编译结果
COPY --from=builder /app/webhook-app .
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]