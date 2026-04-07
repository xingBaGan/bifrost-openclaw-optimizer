# Bifrost OpenClaw Optimizer 🚀

Bifrost 是一个专为 **OpenClaw** 设计的高性能网关与意图识别优化器。它通过内置的语义路由（Semantic Router）和极致瘦身的 Embedding 服务，实现了请求的精准分发与显著的成本缩减。

---

## ✨ 核心优势
- **双库分发**：支持 **GitHub (GHCR)** 国际源与 **腾讯云 (CCR)** 国内极速源。
- **语义路由**：内置智能意图识别，自动将请求分发至最合适的模型后端。
- **结构化输出优化**：优先使用 **Kimi 2.5** 处理 Tools/JSON 请求，获得更高的稳定性和逻辑一致性。
- **白嫖级部署**：全程 GitHub Actions 自动编译，用户只需 `docker compose up`。

---

## 🚀 快速开始 (生产部署)

这是最简单的安装方式，无需编译，直接拉取镜像并在 1 分钟内跑通。

### 1. 准备环境
确保你的服务器已安装 `Docker` 和 `Docker Compose`。

### 2. 获取部署配置
下载项目中的 `docker-compose.prod.yml` 并重命名为 `docker-compose.yml`：
```bash
wget https://github.com/xingBaGan/bifrost-openclaw-optimizer/raw/main/docker-compose.prod.yml -O docker-compose.yml
```

### 3. 配置网关 (`config.json`)
在同级目录下创建 `config.json`（参考项目根目录的配置示例），配置你的各个模型提供商。

### 4. 启动服务
```bash
docker compose up -d
```
> **提示**：如果国内 VPS 拉取速度慢，请确保 `docker-compose.yml` 中使用的是 `hkccr.ccs.tencentyun.com` 路径。

---

## 🛠️ 开发者指南 (本地构建)

如果你想修改代码并进行本地构建：

1. **安装依赖 (Python)**
   本项目使用极速包管理工具 `uv`。
   ```bash
   cd embedding_service
   uv sync
   ```

2. **本地镜像构建**
   ```bash
   docker compose up -d --build
   ```

3. **测试意图识别**
   ```bash
   # 测试 Embedding 服务健康状态
   curl http://localhost:8001/health
   ```

---

## 🏗️ 项目架构

```text
.
├── bifrost (Go Gateway)          # 核心网关，负责请求分发与插件管理
├── embedding_service (Python)    # 轻量级语义路由服务 (基于 Semantic Router)
├── plugins                       # 自定义 Go 插件 (分类器/计费等)
├── .github/workflows             # 自动化双规构建流水线
└── docker-compose.prod.yml       # 生产部署专用配置
```

---

## 🗺️ Roadmap

### M0: 基础稳固
- [x] 多供应商支持 (OpenAI/DeepSeek/Kimi)
- [x] 与 OpenClaw 深度集成
- [x] 初版意图识别路由 (Embedding Service)
- [x] 结构化输出路由优化 (Kimi 2.5 优先)

### M1: 性能与可靠性
- [ ] 动态权重负载均衡
- [ ] 熔断与自动降级机制
- [ ] 成本限制与用户配额

---

## 🛠️ 常见问题与网络排查 (Troubleshooting)

### 1. VPS 上 Docker 连不上宿主机代理？
如果你使用 [clash-for-linux-install](https://github.com/nelvko/clash-for-linux-install) 等脚本，必须开启“允许局域网”才能让容器访问宿主机代理：

1. **执行编辑命令**：
   ```bash
   clashmixin -e
   ```
2. **修正配置**：确保以下两项配置正确（默认端口为 `7890`）：
   ```yaml
   allow-lan: true
   mixed-port: 7890
   ```
3. **保存退出**后，Bifrost 将通过 `host.docker.internal:7890` 自动路由流量。

---

## 🔗 相关项目

- **[OpenClaw](https://github.com/openclaw/openclaw)**: 一个基于多种大模型的智能聚合搜索与智能助手框架。
- **[OpenClaw 优化指南](https://github.com/OnlyTerp/openclaw-optimization-guide)**: 深入浅出的 OpenClaw 性能与路由优化手册。
- **[Bifrost (Original)](https://github.com/maximhq/bifrost)**: 本项目底层基于的 Go 语言高性能 API 网关。

---

## 📜 许可证

开源于 MIT 协议。
