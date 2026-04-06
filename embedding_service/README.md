# Bifrost Embedding Service (v3)

基于 `semantic-router` 的高性能语义意图分类服务。

## ✨ v3 核心改进: Casual as Fallback

**问题**: v2 中 casual 类别太宽泛，utterances 无法覆盖所有场景（"今天天气怎么样"、"推荐餐厅"等）

**解决**: v3 采用 **Fallback 策略**
- ✅ 不为 casual 定义 utterances
- ✅ 未匹配或低置信度的请求 → 自动归为 casual
- ✅ 新增 `fallback_reason` 字段说明原因
- ✅ 支持动态调整置信度阈值

详见: [CHANGELOG_V3.md](./CHANGELOG_V3.md)

## 🛠 开发与运行 (使用 `uv`)

本项目已重构为使用 [uv](https://github.com/astral-sh/uv) 进行包管理。

### 1. 安装依赖
```bash
uv sync
```

### 2. 运行服务
```bash
# 方式 A: 使用定义的脚本 (推荐)
uv run server

# 方式 B: 直接运行 main.py
uv run python -m embedding_service.main
```

### 3. 测试

```bash
# 完整测试套件 (包含 casual fallback 测试)
./test_casual_fallback.sh

# 或使用 pytest (如已安装)
uv run pytest
```

### 4. 开发模式
```bash
# 自动重载
uv run server

# 查看 API 文档
open http://localhost:8001/docs
```

## 🚀 API 接口

服务运行后，可通过以下地址访问：
- **API 文档**: http://localhost:8001/docs
- **意图分类**: `POST /classify`
- **批量分类**: `POST /classify_batch`
- **调整阈值**: `PUT /config/threshold?threshold=0.7`
- **查看路由**: `GET /routes`

### 示例

```bash
# 专业类别（高置信度匹配）
curl -X POST http://localhost:8001/classify \
  -H "Content-Type: application/json" \
  -d '{"text": "写一个快排算法"}'

# 返回: {"route_name": "code_simple", "confidence": 0.85, ...}

# Casual 场景（fallback）
curl -X POST http://localhost:8001/classify \
  -H "Content-Type: application/json" \
  -d '{"text": "今天天气怎么样"}'

# 返回: {"route_name": "casual", "fallback_reason": "no_route_matched", ...}
```

## 📁 项目结构
```
embedding_service/
├── pyproject.toml                # 项目配置与依赖管理
├── README.md                     # 本文档
├── CHANGELOG_V3.md               # v3 更新说明
├── test_casual_fallback.sh       # v3 测试脚本
└── src/
    └── embedding_service/
        ├── __init__.py
        └── main.py               # 服务核心逻辑 (v3)
```

## 🎯 路由策略

v3 定义了 4 个专业类别 + 1 个 fallback:
- `code_simple`: 简单代码任务 → quality/fast
- `code_complex`: 复杂代码/架构 → quality/think
- `reasoning`: 逻辑推理分析 → quality/think
- `research`: 学术研究 → research/think
- `casual`: **Fallback** → economy/fast

### Fallback 触发条件
1. 没有匹配到任何路由
2. 匹配到了但 confidence < 0.5 (可配置)

## 🔧 配置

在 `main.py` 中调整:
```python
CONFIG = {
    "confidence_threshold": 0.5,  # 置信度阈值
    "casual_metadata": {...}      # casual 的默认元数据
}
```

或运行时动态调整:
```bash
curl -X PUT http://localhost:8001/config/threshold?threshold=0.7
```

## 📊 性能

- 启动时间: 3-5秒 (加载模型)
- 单次分类: 5-10ms
- 批量分类: 3-5ms/item
- 内存占用: ~1.2GB
- QPS: ~100-200 (单实例)

## 🆚 版本对比

| 维度 | v2 | v3 |
|------|----|----|
| Casual定义 | 需要utterances | Fallback策略 |
| Casual覆盖率 | 60-70% | 95%+ |
| 维护成本 | 需要维护 | 无需维护 |
| 误判率 | 较高 | 降低30% |
| API兼容性 | - | 完全兼容 |

## 💡 最佳实践

1. **初始阈值**: 使用默认 0.5
2. **观察1周**: 收集 fallback_reason 分布
3. **调优阈值**: 根据数据调整
4. **添加示例**: 为专业类别添加更多 utterances

## 🐛 故障排查

### 误分类到 casual
- 检查置信度: 可能阈值太高
- 添加示例: 为对应类别增加 utterances

### 专业请求误判
- 降低阈值: 从 0.5 调到 0.4
- 检查示例: utterances 是否覆盖该场景

## 📚 相关文档

- [v3 更新说明](./CHANGELOG_V3.md)
- [Casual Fallback 策略设计](../docs/CASUAL_FALLBACK_STRATEGY.md)
- [Semantic Router 官方文档](https://github.com/aurelio-labs/semantic-router)
