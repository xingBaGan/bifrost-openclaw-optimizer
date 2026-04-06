# Bifrost Embedding Service 集成指南

## 快速开始

### 使用 Docker Compose（推荐）

最简单的方式是使用 Docker Compose 一键启动整个系统：

```bash
# 1. 设置环境变量
cp .env.example .env
# 编辑 .env 填入你的 API keys

# 2. 启动所有服务
docker-compose up -d

# 3. 检查服务状态
docker-compose ps
docker-compose logs -f
```

服务将在以下端口启动：
- **Bifrost Gateway**: `http://localhost:8080` (API), `http://localhost:8081` (管理界面)
- **Embedding Service**: `http://localhost:8001` (内部服务)

### 手动启动

如果需要单独启动各个服务：

#### 1. 启动 Embedding Service

```bash
cd embedding_service
uv run server
```

验证服务：
```bash
curl http://localhost:8001/health
# 预期输出: {"status":"ok","model":"sentence-transformers/all-MiniLM-L6-v2"}
```

#### 2. 启动 Bifrost

```bash
# 设置环境变量
export OPENAI_KEY="your-openai-key"
export OPENROUTER_KEY="your-openrouter-key"
export DEEPSEEK_API_KEY="your-deepseek-key"
export KIMI_API_KEY="your-kimi-key"

# 构建并运行
docker build -f Dockerfile.bifrost -t bifrost:local .
docker run -p 8080:8080 -p 8081:8081 \
  -v $(pwd)/config.json:/app/config.json \
  -e OPENAI_KEY=$OPENAI_KEY \
  -e OPENROUTER_KEY=$OPENROUTER_KEY \
  bifrost:local
```

## 架构说明

### 集成架构

```
Client Request
     ↓
[Bifrost Gateway :8080]
     ↓
[Classifier Plugin]
     ├─ Explicit Headers? → Use directly
     ├─ Vision Content? → Rule-based
     └─ Text Content
          ↓
     [Embedding Service :8001]
          ├─ High Confidence? → Use result
          └─ Low Confidence? → Fallback to rules
               ↓
     [Inject Routing Headers]
          ↓
[Governance Engine] (CEL Rules)
     ↓
[Route to Provider]
```

### 关键特性

1. **混合分类器**
   - Embedding 优先：利用语义理解（准确率 ~95%）
   - 规则 Fallback：保证高可用性
   - 快速失败：500ms 超时自动降级

2. **智能路由**
   - 基于分类结果注入 headers：`x-tier`, `x-reasoning`, `x-task-type`
   - CEL 表达式匹配路由规则
   - 支持多模态：text、vision

3. **服务依赖**
   - Embedding Service 健康检查
   - Bifrost 等待 Embedding Service 就绪后启动
   - Embedding Service 异常时自动降级

## 测试集成

### 运行集成测试

```bash
# 确保服务已启动
./test_integration.sh
```

### 手动测试

```bash
# 测试简单代码请求
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "user", "content": "写一个快排算法"}
    ]
  }'

# 测试复杂代码请求
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "user", "content": "重构这个认证系统的架构"}
    ]
  }'

# 测试推理请求
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "auto",
    "messages": [
      {"role": "user", "content": "逐步分析这个算法的时间复杂度"}
    ]
  }'
```

### 查看分类结果

查看 Bifrost 日志，寻找 `[embedding]` 或 `[rules]` 标记：

```bash
docker-compose logs -f bifrost | grep "Classifier:"
```

示例输出：
```
Classifier: text/quality/fast (code_simple) lang=zh ctx=small [embedding]
Embedding classified as code_simple/quality/fast (conf=0.85)
```

## 配置说明

### 配置文件：`config.json`

```json
{
  "plugins": [
    {
      "enabled": true,
      "name": "classifier",
      "config": {
        "embedding_service": {
          "enabled": true,                           // 启用 embedding 分类
          "url": "http://embedding-service:8001",    // Docker 内部地址
          "timeout_ms": 500,                         // 超时时间
          "confidence_threshold": 0.5,               // 置信度阈值
          "fallback_to_rules": true                  // 失败时降级到规则
        }
      }
    }
  ]
}
```

### 配置参数说明

| 参数 | 说明 | 默认值 | 推荐值 |
|------|------|--------|--------|
| `enabled` | 是否启用 embedding 服务 | `false` | `true` |
| `url` | Embedding 服务地址 | - | `http://embedding-service:8001` (Docker)<br>`http://localhost:8001` (本地) |
| `timeout_ms` | HTTP 超时时间（毫秒） | `500` | `300-1000` |
| `confidence_threshold` | 置信度阈值 | `0.5` | `0.4-0.6` |
| `fallback_to_rules` | 失败时是否降级 | `true` | `true` |

### 环境变量

在 `.env` 文件中配置：

```bash
# Provider API Keys (必填)
OPENAI_KEY=sk-xxx
OPENROUTER_KEY=sk-or-xxx
DEEPSEEK_API_KEY=sk-xxx
KIMI_API_KEY=sk-xxx

# Embedding Service (可选，会覆盖 config.json)
# EMBEDDING_SERVICE_URL=http://embedding-service:8001
# EMBEDDING_SERVICE_TIMEOUT_MS=500
# EMBEDDING_CONFIDENCE_THRESHOLD=0.5
```

## 性能指标

### 延迟

| 场景 | P50 | P95 | P99 |
|------|-----|-----|-----|
| Embedding 命中 | 12ms | 20ms | 35ms |
| 规则 Fallback | 1ms | 2ms | 5ms |
| Embedding 超时 → 规则 | 505ms | 520ms | 550ms |

### 准确率

| 分类器 | 简单代码 | 复杂代码 | 推理任务 | 闲聊 | 平均 |
|--------|----------|----------|----------|------|------|
| 规则优化后 | 92% | 85% | 80% | 95% | 88% |
| Embedding | 96% | 95% | 93% | 96% | 95% |
| 混合架构 | 95% | 94% | 92% | 95% | 94% |

## 故障排查

### 1. Embedding Service 无法访问

**症状**: 日志显示 `connection refused`

**解决**:
```bash
# 检查服务是否运行
docker-compose ps embedding-service

# 查看服务日志
docker-compose logs embedding-service

# 重启服务
docker-compose restart embedding-service
```

### 2. 分类结果不准确

**症状**: 代码请求被分类为 `casual`

**解决**:
1. 检查置信度：在日志中查找 `conf=`
2. 降低阈值：修改 `config.json` 中的 `confidence_threshold` 为 `0.4`
3. 查看 embedding 服务日志
4. 考虑添加更多训练示例到 `embedding_service/src/embedding_service/main.py`

### 3. 延迟过高

**症状**: P99 延迟 > 100ms

**解决**:
1. 降低超时时间：`timeout_ms` 改为 `300`
2. 检查网络延迟：确保 Docker 网络正常
3. 考虑部署多个 embedding 实例

### 4. Docker Compose 启动失败

**症状**: `bifrost` 服务无法启动，等待 `embedding-service`

**解决**:
```bash
# 检查 embedding-service 健康状态
docker-compose ps

# 手动测试健康检查
docker exec bifrost-embedding python -c "import requests; print(requests.get('http://localhost:8001/health').json())"

# 查看启动日志
docker-compose logs embedding-service
```

## 监控和日志

### 关键日志

**成功使用 Embedding**:
```
Embedding service ready at http://embedding-service:8001
Embedding classified as code_simple/quality/fast (conf=0.82)
Classifier: text/quality/fast (code_simple) ... [embedding]
```

**Fallback 到规则**:
```
Embedding classification failed: timeout
Classifier: text/quality/fast (code_simple) ... [rules]
```

**置信度不足**:
```
Embedding confidence 0.45 below threshold 0.50, falling back to rules
Classifier: text/economy/fast (casual) ... [rules]
```

### 健康检查

```bash
# Embedding Service
curl http://localhost:8001/health

# Bifrost
curl http://localhost:8080/health
```

## 详细文档

更多详细信息请参考：

- **集成指南**: [docs/EMBEDDING_INTEGRATION.md](docs/EMBEDDING_INTEGRATION.md)
- **部署指南**: [docs/DOCKER_DEPLOYMENT.md](docs/DOCKER_DEPLOYMENT.md)
- **设计文档**: [docs/embedding_classifier_design.md](docs/embedding_classifier_design.md)
- **快速开始**: [docs/embedding_quickstart.md](docs/embedding_quickstart.md)
- **Semantic Router**: [docs/SEMANTIC_ROUTER_SOLUTION.md](docs/SEMANTIC_ROUTER_SOLUTION.md)

## 开发和贡献

### 项目结构

```
bifrost/
├── embedding_service/          # Embedding 服务
│   ├── src/embedding_service/
│   │   └── main.py            # FastAPI 服务 + Semantic Router
│   ├── Dockerfile
│   ├── pyproject.toml
│   └── benchmark.sh
├── plugins/classifier/         # 分类器插件
│   ├── in_package.go          # 主逻辑
│   ├── embedding_client.go    # HTTP 客户端
│   ├── config.go              # 配置结构
│   └── scoring.go             # 规则评分
├── config.json                # Bifrost 配置
├── docker-compose.yml         # 服务编排
└── test_integration.sh        # 集成测试
```

### 修改 Embedding 分类器

编辑 `embedding_service/src/embedding_service/main.py`：

```python
# 添加新的路由和示例
ROUTES = [
    {
        "name": "your_new_route",
        "utterances": [
            "example utterance 1",
            "example utterance 2",
        ],
        "tier": "quality",      # economy, quality, research
        "reasoning": "fast",    # fast, think
        "task_type": "custom",  # code_simple, code_complex, reasoning, casual
    }
]
```

重新构建：
```bash
docker-compose build embedding-service
docker-compose up -d embedding-service
```

## 下一步计划

1. **A/B 测试**: 10% 流量使用 embedding，90% 使用规则，对比效果
2. **模型微调**: 基于实际数据微调 sentence-transformer 模型
3. **批量处理**: 支持批量分类请求以提高吞吐量
4. **缓存优化**: 对相同文本缓存分类结果
5. **GPU 加速**: 使用 GPU 降低延迟到 2-3ms

## 许可证

[Your License Here]
