# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1.【核心修复】先拷贝所有文件
# 必须把 main.go 和 go.mod 先拷进去，go mod tidy 才能识别出你需要 gin 框架
COPY . .

# 2. 自动修复依赖
# 这一步会根据拷贝进来的代码，自动下载 gin 包并生成正确的文件
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod tidy

# 3. 编译
# 此时依赖已经齐了，编译绝对能通过
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