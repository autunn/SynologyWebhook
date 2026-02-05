# 阶段一：编译环境
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 拷贝 go.mod (如果有的话)
COPY go.mod ./

# 2. 下载依赖
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod tidy

# 3. 拷贝源代码
COPY main.go ./

# 4. 拷贝模板
COPY templates ./templates

# 5. 编译
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o webhook-app .

# 阶段二：运行环境
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/webhook-app .
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]