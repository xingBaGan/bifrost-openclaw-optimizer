# Docker 部署指南

Bifrost + Embedding Service 的完整 Docker 部署方案。

## 📦 架构

```
┌─────────────────────────────────────────┐
│         Docker Network (bifrost)         │
│                                          │
│  ┌────────────────┐  ┌────────────────┐ │
│  │   Bifrost      │  │   Embedding    │ │
│  │   Gateway      │──│   Service      │ │
│  │   :8080, :8081 │  │   :8001        │ │
│  └────────────────┘  └────────────────┘ │
└─────────────────────────────────────────┘
         │                    │
    Port 8080/8081        Port 8001
         │                    │
    [Host Machine]       [Host Machine]
```

## 🚀 快速启动

### 1. 准备环境

```bash
# 克隆或进入项目目录
cd /Users/jzj/bifrost

# 复制环境变量模板
cp .env.example .env

# 编辑 .env 文件，填入你的 API Keys
vim .env
```

**`.env` 示例**:
```bash
OPENAI_KEY=sk-proj-xxx
OPENROUTER_KEY=sk-or-xxx
DEEPSEEK_KEY=sk-xxx
```

### 2. 一键启动

```bash
# 构建并启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f embedding-service
docker-compose logs -f bifrost
```

### 3. 验证部署

```bash
# 检查 Embedding Service
curl http://localhost:8001/health
# 应返回: {"status":"ok","model_ready":true,"routes_count":4}

# 检查 Bifrost
curl http://localhost:8080/health
# 应返回 Bifrost 的健康状态

# 测试完整流程
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "user", "content": "写一个快排算法"}
    ]
  }'
```

## 📊 服务说明

### Embedding Service
- **镜像**: `bifrost-embedding:latest`
- **端口**: `8001`
- **功能**: 语义分类服务
- **启动时间**: ~40秒（模型加载）
- **内存**: ~1.5GB
- **CPU**: 2核心推荐

### Bifrost Gateway
- **镜像**: `bifrost:local-dynamic`
- **端口**: 
  - `8080` - HTTP API
  - `8081` - Admin UI
- **功能**: AI 网关
- **依赖**: embedding-service (健康后启动)

## 🔧 配置

### config.json 说明

```json
{
  "plugins": [
    {
      "enabled": true,
      "name": "classifier",
      "config": {
        "embedding_service": {
          "enabled": true,
          "url": "http://embedding-service:8001",  // 容器内部通信
          "timeout_ms": 500,
          "confidence_threshold": 0.5,
          "fallback_to_rules": true
        }
      }
    }
  ]
}
```

**重要**: 
- `url` 使用容器名称 `embedding-service` 而非 `localhost`
- 容器间通过 Docker 网络 `bifrost-network` 通信

### 环境变量覆盖

可以通过环境变量覆盖配置:
```bash
# 在 docker-compose.yml 的 bifrost 服务中添加
environment:
  - EMBEDDING_SERVICE_URL=http://embedding-service:8001
  - EMBEDDING_CONFIDENCE_THRESHOLD=0.6
```

## 🛠️ 常用命令

### 启动/停止

```bash
# 启动所有服务
docker-compose up -d

# 停止所有服务
docker-compose down

# 停止并删除卷（清理所有数据）
docker-compose down -v

# 重启特定服务
docker-compose restart embedding-service
docker-compose restart bifrost
```

### 查看状态

```bash
# 查看所有服务状态
docker-compose ps

# 查看资源使用
docker stats bifrost bifrost-embedding

# 查看日志
docker-compose logs -f --tail=100

# 只看 embedding service 日志
docker-compose logs -f embedding-service
```

### 重新构建

```bash
# 重新构建所有镜像
docker-compose build

# 只重建 embedding service
docker-compose build embedding-service

# 强制重建（不使用缓存）
docker-compose build --no-cache

# 重建并重启
docker-compose up -d --build
```

### 进入容器调试

```bash
# 进入 embedding service 容器
docker exec -it bifrost-embedding bash

# 进入 bifrost 容器
docker exec -it bifrost bash

# 在容器内测试
docker exec bifrost-embedding curl http://localhost:8001/health
```

## 🐛 故障排查

### 问题 1: Embedding Service 启动失败

**症状**:
```bash
docker-compose logs embedding-service
# Error: Model download failed
```

**解决**:
1. 检查网络连接
2. 使用国内镜像源（已在 Dockerfile 配置）
3. 增加启动超时时间:
```yaml
healthcheck:
  start_period: 60s  # 从 40s 增加到 60s
```

### 问题 2: Bifrost 无法连接 Embedding Service

**症状**:
```
Embedding classification failed: connection refused
```

**解决**:
1. 检查 embedding service 是否健康:
```bash
docker-compose ps embedding-service
# 状态应该是 healthy
```

2. 检查网络连通性:
```bash
docker exec bifrost curl http://embedding-service:8001/health
```

3. 查看 embedding service 日志:
```bash
docker-compose logs embedding-service
```

### 问题 3: 端口冲突

**症状**:
```
Error: port 8001 already in use
```

**解决**:
1. 修改 docker-compose.yml 端口映射:
```yaml
ports:
  - "8002:8001"  # 使用 8002 而非 8001
```

2. 或停止占用端口的进程:
```bash
lsof -ti:8001 | xargs kill
```

### 问题 4: 内存不足

**症状**:
```
OOMKilled
```

**解决**:
在 docker-compose.yml 中限制内存:
```yaml
embedding-service:
  deploy:
    resources:
      limits:
        memory: 2G
      reservations:
        memory: 1G
```

## 📈 性能优化

### 1. 使用本地模型缓存

```yaml
embedding-service:
  volumes:
    - ~/.cache/huggingface:/root/.cache/huggingface
```

### 2. 多实例部署（负载均衡）

```yaml
embedding-service:
  deploy:
    replicas: 3
```

然后在 Bifrost 前添加 Nginx 负载均衡。

### 3. GPU 加速

修改 Dockerfile:
```dockerfile
FROM python:3.12-slim

# 安装 CUDA 相关库
RUN pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu118
```

docker-compose.yml:
```yaml
embedding-service:
  runtime: nvidia
  environment:
    - NVIDIA_VISIBLE_DEVICES=0
```

## 🔒 生产环境部署

### 安全建议

1. **不要暴露 8001 端口** 到公网（只供内部使用）:
```yaml
embedding-service:
  ports: []  # 不映射到主机
```

2. **使用环境变量管理密钥**:
```bash
# 使用 Docker Secrets 或外部密钥管理
docker secret create openai_key ./openai_key.txt
```

3. **启用日志轮转**:
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### 高可用部署

```yaml
version: '3.8'

services:
  embedding-service:
    deploy:
      replicas: 2
      restart_policy:
        condition: on-failure
        max_attempts: 3
      resources:
        limits:
          cpus: '2'
          memory: 2G

  bifrost:
    deploy:
      replicas: 2
      restart_policy:
        condition: on-failure
```

## 📚 相关文档

- [集成指南](docs/EMBEDDING_INTEGRATION.md)
- [Bifrost 配置](docs/UPGRADE_SUMMARY.md)
- [故障排查](docs/EMBEDDING_INTEGRATION.md#故障处理)

## ✅ 部署检查清单

部署前检查:
- [ ] 已创建 `.env` 文件并填写 API Keys
- [ ] 已检查 `config.json` 中 embedding_service.url 是否正确
- [ ] 端口 8001, 8080, 8081 未被占用
- [ ] Docker 和 Docker Compose 已安装
- [ ] 有足够的磁盘空间（至少 5GB）

部署后验证:
- [ ] `docker-compose ps` 显示所有服务为 healthy
- [ ] `curl http://localhost:8001/health` 返回 200
- [ ] `curl http://localhost:8080/health` 返回 200
- [ ] 发送测试请求能正确路由和分类
- [ ] 查看日志确认 embedding 分类正常工作

## 🎯 总结

使用 Docker Compose 部署 Bifrost + Embedding Service:

**优点**:
- ✅ 一键启动，简化部署
- ✅ 服务隔离，互不干扰
- ✅ 自动健康检查和重启
- ✅ 网络配置自动化
- ✅ 易于扩展和维护

**命令速查**:
```bash
# 启动
docker-compose up -d

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f

# 重启
docker-compose restart

# 停止
docker-compose down
```

需要帮助? 查看 [EMBEDDING_INTEGRATION.md](docs/EMBEDDING_INTEGRATION.md)
