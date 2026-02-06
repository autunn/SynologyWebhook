<div align="center">
  <img src="https://raw.githubusercontent.com/autunn/SynologyWebhook/main/logo.png" width="120" alt="Synology Webhook Logo">
  <h1>Synology Webhook</h1>
  <p>
    <b>连接群晖 NAS 与 企业微信的现代化桥梁</b>
  </p>
  <p>
    <a href="https://hub.docker.com/r/autunn/synologywebhook">
      <img src="https://img.shields.io/docker/pulls/autunn/synologywebhook?style=flat-square&color=007bff" alt="Docker Pulls">
    </a>
    <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License">
    <img src="https://img.shields.io/badge/Go-1.21-00ADD8?style=flat-square&logo=go" alt="Go Version">
  </p>
</div>

---

## 📖 简介

这是一个轻量级、高性能的 Webhook 转发工具，专为 **Synology NAS** 用户设计。它可以接收群晖的系统通知，并将其精美地推送到 **企业微信 (WeChat Work)**。

## ✨ 特性

- 🎨 **精美 UI**：内置现代化管理界面，所见即所得。
- 🔒 **安全验证**：完整支持企业微信回调验证，数据加密传输。
- 🚀 **一键部署**：支持 Docker 多架构（AMD64/ARM64）。
- 📷 **图文并茂**：支持自定义通知图片和跳转链接。

## 🐳 Docker 快速部署

```bash
docker run -d \
  --name synology-webhook \
  -p 5080:5080 \
  -v $(pwd)/data:/app/data \
  --restart always \
  autunn/synologywebhook:latest
```

## ⚙️ 配置方法

1. 启动容器后，访问 `http://你的NASIP:5080`。
2. 填写企业微信的 `CorpID`、`AgentID`、`Secret` 等信息。
3. 点击保存，配置即时生效。
4. 在群晖控制面板 -> 通知设置 -> Webhook 中填入回调地址。

---
<div align="center">
  <sub>Made with ❤️ by autunn</sub>
</div>
