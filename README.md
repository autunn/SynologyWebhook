# SynologyWebhook

这是一个为群晖 NAS 用户量身定制的 Webhook 转发工具。它可以接收群晖系统通知，并将其精美地转发到您的**企业微信**中。

# SynologyWebhook

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/autunn/SynologyWebhook/build.yml?branch=main&label=Build%20Status&logo=github)
![Docker Pulls](https://img.shields.io/docker/pulls/autunn/synologywebhook?logo=docker)
![Docker Image Size](https://img.shields.io/docker/image-size/autunn/synologywebhook/latest?logo=docker)

### ✨ 项目特点

* **多架构支持**：原生支持 `x86_64` (amd64) 和 `arm64` 架构，群晖全机型通用。
* **可视化配置**：内置高颜值 Web 配置后台，无需手动修改代码。
* **安全性**：支持企业微信官方加解密验证，确保接口安全。
* **灵活推送**：支持直接请求微信服务器或通过自定义 API 代理发送。

### 🚀 快速部署 (Docker Compose)

```yaml
version: '3'
services:
  synology-webhook:
    image: autunn/synologywebhook:latest
    container_name: SynologyWebhook
    restart: always
    ports:
      - "5080:5080"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Shanghai

```

### 📖 使用流程

1. **部署容器**：使用上述 Compose 文件启动。
2. **初始化配置**：浏览器访问 `http://NAS_IP:5080`，填写企业微信相关参数。
3. **设置群晖 Webhook**：
* 进入群晖 **控制面板** > **通知设置** > **Webhook**。
* 新增 Webhook，选择“自定义”。
* URL 填写：`http://NAS_IP:5080/webhook`。
* HTTP 方法选择 `POST`。


4. **验证**：在群晖中点击“发送测试消息”，您的企业微信将收到带随机风景封面的精美通知卡片。

