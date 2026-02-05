# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 拷贝刚才创建的 go.mod
COPY go.mod ./

# 2. 自动整理依赖 (会根据 go.mod 自动下载 gin 框架并生成 go.sum)
# 我们去掉了 GOPROXY 设置，使用 Go 官方默认源，这在 GitHub Actions 环境下最稳
RUN go mod tidy

# 3. 拷贝源代码
COPY main.go ./

# 4. 拷贝模板 (如果有的话，防止编译找不到路径，虽然编译不需要它，但保持逻辑一致)
COPY templates ./templates

# 5. 编译
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-app .

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