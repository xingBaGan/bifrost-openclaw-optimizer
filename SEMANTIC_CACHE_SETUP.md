# Semantic Caching 配置指南

**添加日期**: 2026-04-06  
**状态**: ✅ 已配置，待部署测试

---

## 📋 已完成的配置

### 1. Docker Compose 更新

添加了 Redis 服务作为缓存存储：

```yaml
redis:
  image: redis:7-alpine
  container_name: bifrost-redis
  ports:
    - "6379:6379"
  volumes:
    - redis-data:/data
  command: redis-server --appendonly yes --maxmemory 2gb --maxmemory-policy allkeys-lru
  healthcheck:
    test: ["CMD", "redis-cli", "ping"]
  restart: unless-stopped
```

**特性**:
- ✅ 持久化存储（appendonly）
- ✅ 内存限制 2GB
- ✅ LRU 淘汰策略（自动清理旧缓存）
- ✅ 健康检查
- ✅ Bifrost 依赖 Redis 启动

### 2. Bifrost 配置更新

在 `config.json` 中添加了 semantic_cache 配置：

```json
{
  "semantic_cache": {
    "enabled": true,
    "provider": "openai",
    "embedding_model": "text-embedding-3-small",
    "dimension": 1536,
    "ttl_seconds": 3600,
    "threshold": 0.85,
    "max_history_messages": 3,
    "cleanup_on_shutdown": false,
    "vector_store": {
      "type": "redis",
      "config": {
        "address": "redis:6379",
        "namespace": "bifrost:cache",
        "db": 0
      }
    }
  }
}
```

### 3. 测试脚本

创建了 `test_semantic_cache.sh` 用于验证缓存功能。

---

## 🚀 部署步骤

### 1. 停止现有服务

```bash
cd /Users/jzj/bifrost
docker-compose down
```

### 2. 启动所有服务（包括 Redis）

```bash
docker-compose up -d
```

### 3. 查看服务状态

```bash
docker-compose ps
```

预期输出:
```
NAME                STATUS                PORTS
bifrost             Up (healthy)          8080-8081
bifrost-embedding   Up (healthy)          8001
bifrost-redis       Up (healthy)          6379
```

### 4. 验证 Redis 连接

```bash
# 方式 1: 通过 docker exec
docker exec bifrost-redis redis-cli ping
# 预期输出: PONG

# 方式 2: 检查日志
docker-compose logs redis | tail -20
```

### 5. 运行缓存测试

```bash
./test_semantic_cache.sh
```

---

## 📊 配置参数说明

### 核心参数

| 参数 | 值 | 说明 |
|------|-----|------|
| `enabled` | true | 启用语义缓存 |
| `provider` | openai | 使用 OpenAI embedding API |
| `embedding_model` | text-embedding-3-small | 模型选择（便宜且快） |
| `dimension` | 1536 | 向量维度 |
| `ttl_seconds` | 3600 | 缓存有效期 1 小时 |
| `threshold` | 0.85 | 相似度阈值（0-1） |

### threshold 阈值建议

| 阈值 | 匹配严格度 | 适用场景 |
|------|-----------|----------|
| 0.9-1.0 | 极严格 | 精确匹配，很少命中 |
| 0.85-0.9 | 严格 | **推荐，平衡准确率和命中率** |
| 0.75-0.85 | 中等 | 更多命中，可能有误匹配 |
| 0.6-0.75 | 宽松 | 高命中率，但准确性下降 |

### ttl_seconds 建议

| TTL | 使用场景 |
|-----|---------|
| 300 (5分钟) | 实时数据、经常变化的内容 |
| 1800 (30分钟) | 一般对话、临时会话 |
| **3600 (1小时)** | **代码生成、FAQ（推荐）** |
| 86400 (24小时) | 稳定文档、长期有效的内容 |

---

## 🔑 如何使用缓存

### HTTP API 方式

在请求 header 中添加 `x-bf-cache-key`：

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-bf-cache-key: user-123-session-456" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "写一个快排"}]
  }'
```

### Cache Key 设计策略

**按用户缓存** (推荐):
```
x-bf-cache-key: user-{user_id}
```
- 每个用户独立缓存
- 避免跨用户泄露
- 个性化体验

**按会话缓存**:
```
x-bf-cache-key: session-{session_id}
```
- 同一会话内共享缓存
- 会话结束后过期

**按项目缓存**:
```
x-bf-cache-key: project-{project_id}
```
- 同一项目成员共享缓存
- 提高团队协作效率

**全局缓存** (⚠️ 谨慎使用):
```
x-bf-cache-key: global
```
- 所有用户共享
- 注意隐私和安全问题

---

## 🔍 监控和调试

### 查看缓存统计

```bash
# 连接到 Redis
docker exec -it bifrost-redis redis-cli

# 查看缓存数量
DBSIZE

# 查看所有 bifrost 缓存 key
KEYS bifrost:cache:*

# 查看某个 key 的 TTL（剩余时间）
TTL bifrost:cache:xxxxx

# 查看 key 的内容
GET bifrost:cache:xxxxx

# 清空所有缓存（测试用）
FLUSHDB

# 退出
exit
```

### 查看缓存命中率

Bifrost 日志中会显示缓存调试信息：

```bash
docker-compose logs bifrost | grep -i cache
```

### Prometheus Metrics (如果配置了)

```
bifrost_cache_hits_total
bifrost_cache_misses_total
bifrost_cache_hit_rate
bifrost_cache_latency_seconds
```

---

## ⚙️ 运行时调整

### 覆盖 TTL（单次请求）

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "x-bf-cache-key: test" \
  -H "x-bf-cache-ttl: 7200" \
  -d '{...}'
```

### 覆盖相似度阈值（单次请求）

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "x-bf-cache-key: test" \
  -H "x-bf-cache-threshold: 0.9" \
  -d '{...}'
```

### 跳过缓存（单次请求）

```bash
# 不添加 x-bf-cache-key header 即可跳过缓存
curl -X POST http://localhost:8080/v1/chat/completions \
  -d '{...}'
```

---

## 💰 成本估算

### Embedding API 成本

**OpenAI text-embedding-3-small**:
- 价格: $0.02 / 1M tokens
- 平均请求 embedding: ~100 tokens
- 成本: **$0.000002 / 请求**

**每月成本** (1000 req/day):
- Embedding: $0.000002 × 1000 × 30 = **$0.06/月**
- Redis (自托管): **$0**

### 节省收益

假设:
- LLM 平均成本: $0.015/请求
- 缓存命中率: 30%

**每月节省** (1000 req/day):
- 无缓存: $0.015 × 1000 × 30 = $450
- 有缓存: $0.015 × 700 × 30 + $0.06 = $315.06
- **节省: $134.94/月** (30% 节省)

如果命中率达到 50%:
- **节省: $225/月** (50% 节省)

---

## ⚠️ 注意事项

### 1. 隐私和安全

- ❌ **不要**在 cache key 中包含敏感信息
- ❌ **不要**使用 `global` cache key 跨用户共享敏感内容
- ✅ **推荐**按用户隔离缓存

### 2. 内存管理

Redis 配置了:
- 最大内存: 2GB
- 淘汰策略: allkeys-lru（自动删除最少使用的）

预估容量:
- 每个缓存条目: ~2KB
- 2GB 可存储: ~1,000,000 条目

### 3. 缓存失效

缓存会在以下情况失效:
- TTL 过期（自动）
- Redis 重启（如果没有持久化）
- 手动清理（FLUSHDB）
- 内存满时 LRU 淘汰

### 4. 与 Prompt Caching 共存

Semantic caching 在 Bifrost 层，不影响 provider 的 prompt caching：

```
Request → [Bifrost Semantic Cache]  (Layer 1)
          ↓ Miss
       [Provider Prompt Cache]       (Layer 2)
          ↓ Miss
       [LLM Inference]
```

两层缓存互补，最大化节省！

---

## 🐛 故障排查

### 问题 1: Redis 连接失败

**症状**: Bifrost 日志显示 "failed to connect to redis"

**解决**:
```bash
# 检查 Redis 是否运行
docker-compose ps redis

# 检查网络连接
docker exec bifrost ping redis

# 重启 Redis
docker-compose restart redis
```

### 问题 2: 缓存不生效

**症状**: 相同请求仍然很慢

**检查清单**:
1. ✅ 是否添加了 `x-bf-cache-key` header？
2. ✅ Bifrost 配置中 `enabled: true`？
3. ✅ Redis 是否正常运行？
4. ✅ 检查 Bifrost 日志是否有错误

**调试**:
```bash
# 查看 Bifrost 日志
docker-compose logs bifrost | grep -i cache

# 查看 Redis 日志
docker-compose logs redis

# 手动测试 Redis
docker exec bifrost-redis redis-cli ping
```

### 问题 3: 缓存命中率低

**可能原因**:
- threshold 设置太高（0.85 → 尝试 0.75）
- TTL 太短（请求间隔 > TTL）
- cache key 太细粒度（每次都不同）

**优化**:
```json
{
  "threshold": 0.75,  // 降低阈值
  "ttl_seconds": 7200  // 延长 TTL
}
```

---

## 📚 相关文档

- **Bifrost 官方文档**: https://docs.getbifrost.ai/features/semantic-caching
- **Redis 文档**: https://redis.io/docs/
- **OpenAI Embeddings**: https://platform.openai.com/docs/guides/embeddings

---

## ✅ 下一步

1. **部署测试** (30 分钟)
   ```bash
   docker-compose down && docker-compose up -d
   ./test_semantic_cache.sh
   ```

2. **监控观察** (1-2 天)
   - 查看缓存命中率
   - 分析实际节省成本
   - 调优 threshold 和 TTL

3. **生产优化** (1 周)
   - 根据数据调整参数
   - 设计合理的 cache key 策略
   - 添加监控告警

---

**配置完成日期**: 2026-04-06  
**待部署**: 需要运行 `docker-compose up -d` 启动 Redis  
**测试脚本**: `./test_semantic_cache.sh`
