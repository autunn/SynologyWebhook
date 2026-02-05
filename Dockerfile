# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 安装 git (关键修复：解决 go get 失败的问题)
RUN apk add --no-cache git

# 2. 拷贝所有文件 (简单粗暴，防止漏文件)
COPY . .

# 3. 强力修复：无论你仓库里有没有传 go.mod，先删一遍，确保环境纯净
# 这样就彻底解决了 "go.mod already exists" 的报错
RUN rm -f go.mod go.sum

# 4. 重新初始化并下载依赖 (使用默认官方源，GitHub Actions 连接最快)
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod init SynologyWebhook && \
    go get github.com/gin-gonic/gin && \
    go mod tidy

# 5. 编译
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-app .

# 阶段二：运行环境
FROM alpine:latest

WORKDIR /app

# 安装必要的证书和时区数据
RUN apk add --no-cache ca-certificates tzdata

# 从编译阶段拷贝成果
COPY --from=builder /app/webhook-app .
# 拷贝网页模板文件夹
COPY templates ./templates

# 暴露端口
EXPOSE 5080

# 挂载数据卷
VOLUME ["/app/data"]

# 启动命令
CMD ["./webhook-app"]