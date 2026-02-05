# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 拷贝 go.mod
COPY go.mod ./

# 2. 下载依赖 (使用官方源，直连 GitHub Actions 服务器速度快)
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod tidy

# 3. 拷贝源代码
COPY main.go ./

# 4. 拷贝模板
COPY templates ./templates

# 5. 编译 (⚡️ 修复点：去掉了 -ldflags 参数，只保留最基本的编译指令)
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 阶段二：运行环境
FROM alpine:latest

WORKDIR /app

# 安装必要的证书和时区
RUN apk add --no-cache ca-certificates tzdata

# 拷贝编译好的程序
COPY --from=builder /app/webhook-app .
# 拷贝网页模板
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]