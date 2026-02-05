# 阶段一：编译
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 1. 设置代理，保证下载速度和稳定性
ENV GOPROXY=https://proxy.golang.org,direct

# 2. 【关键修改】先拷贝所有文件进去
# 之前是先 copy go.mod 再 copy main.go，导致 tidy 找不到代码引用
COPY . .

# 3. 下载依赖
# 现在 main.go 已经在里面了，tidy 就能看到你需要 gin 框架，并自动下载
RUN go mod tidy

# 4. 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-app .

# 阶段二：运行
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/webhook-app .
COPY templates ./templates

EXPOSE 5080
VOLUME ["/app/data"]

CMD ["./webhook-app"]