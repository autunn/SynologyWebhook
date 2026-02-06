<p align="center">
  <img src="https://raw.githubusercontent.com/autunn/SynologyWebhook/main/logo.png" width="180" alt="Synology Webhook Logo" />
</p>

<p align="center">
  <h2>Synology Webhook</h2>
  <p>连接群晖 NAS 与 企业微信的现代化桥梁</p>
</p>

<p align="center">
  <a href="https://github.com/autunn/SynologyWebhook">
    <img src="https://img.shields.io/badge/GitHub-Source%20Code-000000?style=flat-square&logo=github" />
  </a>
  <a href="https://hub.docker.com/r/autunn/synologywebhook">
    <img src="https://img.shields.io/docker/pulls/autunn/synologywebhook?style=flat-square&logo=docker&color=0db7ed" />
  </a>
  <a href="https://hub.docker.com/r/autunn/synologywebhook">
    <img src="https://img.shields.io/docker/image-size/autunn/synologywebhook?style=flat-square&logo=docker" />
  </a>
  <img src="https://img.shields.io/badge/License-MIT-2ecc71?style=flat-square" />
</p>


## 简介 (Introduction)

Synology Webhook 是一个用于将群晖 NAS 消息推送到企业微信的 Webhook 服务。

## 特性 (Features)

- 精美 UI
- 安全验证
- 多架构支持
- 图文并茂

## 快速启动 (Quick Start)

### Docker CLI

```bash
docker run -d \
  --name synology-webhook \
  -p 5080:5080 \
  -v $(pwd)/data:/app/data \
  -e ADMIN_PASSWORD=这里填你的强密码 \
  --restart always \
  autunn/synologywebhook:latest
```

### Docker Compose

```yaml
version: '3'
services:
  webhook:
    image: autunn/synologywebhook:latest
    container_name: synology-webhook
    ports:
      - "5080:5080"
    volumes:
      - ./data:/app/data
    environment:
      - ADMIN_PASSWORD=这里填你的强密码
    restart: always
```

## 配置指南 (Configuration)

配置企业微信应用的 CorpID 与 Secret，并在群晖中设置 Webhook。

## 挂载卷 (Volumes)

- `/app/data`：持久化配置与数据

## License

MIT License
