# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 安装 Git (下载依赖必备)
RUN apk add --no-cache git

# 2.【关键】先拷贝所有文件
# 我们直接使用你仓库里的 go.mod 和 main.go，不再自己在容器里瞎折腾
COPY . .

# 3. 下载依赖
# 使用 Go 官方全球代理 (GitHub Actions 服务器在美国，连这个最快最稳)
ENV GOPROXY=https://proxy.golang.org,direct
# 这一步会根据你的 main.go 和 go.mod 自动下载 gin，绝对不会错
RUN go mod tidy

# 4. 编译
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