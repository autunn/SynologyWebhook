# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1.【关键修复】安装 git
# Alpine 镜像默认没有 git，go mod tidy 下载依赖时经常需要它，否则会报错 exit code 1
RUN apk add --no-cache git

# 2. 拷贝所有文件
COPY . .

# 3.【核心大招】自修复依赖环境
# 强制删除可能存在的旧配置，现场重新生成，确保 100% 匹配当前代码
# 这样就彻底排除了 "go.mod 文件冲突" 或 "go.mod 缺失" 的问题
RUN rm -f go.mod go.sum && \
    go mod init SynologyWebhook && \
    go mod tidy

# 4. 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 阶段二：运行环境
FROM alpine:latest

WORKDIR /app

# 安装证书和时区
RUN apk add --no-cache ca-certificates tzdata

# 拷贝编译结果
COPY --from=builder /app/webhook-app .
# 拷贝模板文件夹
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]