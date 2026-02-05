# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 安装 Git (下载依赖必备)
RUN apk add --no-cache git

# 2. 拷贝源代码
COPY . .

# 3. 设置 Go 代理 (关键修复：使用 goproxy.cn 解决网络超时问题)
ENV GOPROXY=https://goproxy.cn,direct

# 4. 暴力重置依赖 (拆分成多步，更稳健)
# 先删旧文件
RUN rm -f go.mod go.sum
# 初始化新模块
RUN go mod init SynologyWebhook
# 显式下载 gin 框架 (比 tidy 更强力)
RUN go get github.com/gin-gonic/gin
# 最后整理依赖
RUN go mod tidy

# 5. 编译
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