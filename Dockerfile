# 编译阶段
FROM golang:1.21-alpine AS builder
WORKDIR /app
# 使用代理加速
ENV GOPROXY=https://goproxy.cn,direct
# 先拷贝依赖文件
COPY go.mod go.sum ./
RUN go mod download
# 再拷贝源代码
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 运行阶段 (保持不变)
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/webhook-app .
COPY --from=builder /app/templates ./templates
EXPOSE 5080
VOLUME ["/app/data"]
CMD ["./webhook-app"]