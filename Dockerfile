# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app
# 移除国内代理，使用官方源更适合 GitHub Actions 环境
# ENV GOPROXY=https://goproxy.cn,direct 

# 1. 这里只拷贝 main.go，千万不要写 COPY . . 或者 COPY go.mod ...
COPY main.go ./

# 2. 关键步骤：在容器内部自动生成依赖文件
# 这样就不需要你在 GitHub 上维护 go.mod/go.sum 了，彻底解决“找不到文件”的问题
RUN go mod init SynologyWebhook && \
    go get github.com/gin-gonic/gin && \
    go mod tidy

# 3. 编译
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