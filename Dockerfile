# 编译阶段
FROM golang:1.21-alpine AS builder
WORKDIR /app
# 使用代理加速
ENV GOPROXY=https://goproxy.cn,direct
COPY . .
RUN go mod init SynologyWebhook && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 运行阶段
FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/webhook-app .
COPY --from=builder /app/templates ./templates
EXPOSE 5080
VOLUME ["/app/data"]
CMD ["./webhook-app"]