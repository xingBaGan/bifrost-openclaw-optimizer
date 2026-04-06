# 混合分类架构设计

## 问题: Casual类别太宽泛

semantic-router 适合**明确的、可定义的**类别，但 casual 太宽泛：
- "今天天气怎么样" ❓
- "推荐个餐厅" ❓
- "讲个笑话" ❓
- "帮我算下1+1" ❓

这些都是 casual，但很难穷举。

---

## 解决方案对比

### 方案1: Casual as Fallback ⭐️⭐️⭐️⭐️⭐️ (推荐)

**核心思想**: 不为 casual 定义 utterances，作为兜底类别

```python
# 只定义专业类别
ROUTES = [
    Route(name="code_simple", utterances=[...]),
    Route(name="code_complex", utterances=[...]),
    Route(name="reasoning", utterances=[...]),
    Route(name="research", utterances=[...]),
    # 没有 casual!
]

# 分类逻辑
decision = route_layer(text)

if decision is None or decision.confidence < 0.5:
    return "casual"  # fallback
else:
    return decision.name
```

**优点**:
- ✅ 简单直接
- ✅ 所有未匹配的自动归为 casual
- ✅ 无需维护 casual 示例
- ✅ 符合直觉: "不是专业请求就是闲聊"

**缺点**:
- ⚠️ 无法区分"真正的casual"和"其他未知类别"
- ⚠️ 置信度阈值需要调优

**适用场景**: ✅ **Bifrost 当前场景最适合**

---

### 方案2: 负样本训练 ⭐️⭐️⭐️

**核心思想**: 定义 casual 的正样本 + 负样本

```yaml
casual:
  positive_utterances:
    - "hello", "thanks", "bye"
    - "你好", "谢谢"
  negative_utterances:
    - "write code"  # 这不是casual
    - "analyze data"  # 这不是casual
```

```python
# 训练时使用对比学习
encoder.train(
    positive_pairs=[(casual_text, casual_vector)],
    negative_pairs=[(code_text, casual_vector)],  # 拉开距离
)
```

**优点**:
- ✅ 更精准的 casual 识别
- ✅ 可以区分 casual 和其他类别

**缺点**:
- ❌ 需要微调模型
- ❌ 需要大量标注数据
- ❌ 维护成本高

**适用场景**: 有大量数据和精力时

---

### 方案3: 两阶段分类 ⭐️⭐️⭐️⭐️

**核心思想**: 
1. 第一阶段: 过滤明确类别 (semantic-router)
2. 第二阶段: 规则判断是否真的是 casual

```python
# 第一阶段: semantic-router
decision = route_layer(text)

if decision and decision.confidence > 0.6:
    return decision.name  # 明确的专业类别

# 第二阶段: 规则判断casual特征
if is_greeting(text):  # "hello", "hi"
    return "casual"
elif is_short_and_simple(text):  # 长度<20字且无专业词汇
    return "casual"
elif has_casual_keywords(text):  # "天气", "餐厅", "笑话"
    return "casual"
else:
    return "casual"  # 默认
```

**优点**:
- ✅ 结合语义和规则的优势
- ✅ 可以处理边界case
- ✅ 灵活可调

**缺点**:
- ⚠️ 复杂度略高
- ⚠️ 仍需维护规则

**适用场景**: 需要精细控制时

---

### 方案4: 动态阈值调整 ⭐️⭐️⭐️

**核心思想**: 根据各类别分别设置阈值

```python
THRESHOLDS = {
    "code_simple": 0.6,    # 代码类要求较高置信度
    "code_complex": 0.7,   # 复杂代码更严格
    "reasoning": 0.5,      # 推理类可以宽松些
    "research": 0.8,       # 研究类很严格
}

decision = route_layer(text)

if decision:
    threshold = THRESHOLDS.get(decision.name, 0.5)
    if decision.confidence >= threshold:
        return decision.name

return "casual"  # fallback
```

**优点**:
- ✅ 细的控制
- ✅ 可以针对不同类别调优

**缺点**:
- ⚠️ 需要为每个类别调参

---

### 方案5: 从配置文件加载 ⭐️⭐️⭐️⭐️

**核心思想**: utterances 从配置文件/数据库加载，支持热更新

```yaml
# routes.yaml
routes:
  code_simple:
    utteran      - write a function
      - 写一个函数
      # ... 可以随时添加

  casual:
    strategy: fallback
    threshold: 0.4
```

```python
# 热加载
@app.post("/admin/reload")
def reload_routes():
    routes = load_from_yaml("routes.yaml")
    route_layer.update(routes)
    return {"success": True}
```

**优点**:
- ✅ 无需改代码即可更新
- ✅ 支持A/B测试
- ✅ 易于维护

**缺点**:
- ⚠️ 需要额外的配置管理

---

## 🎯 推荐方案: 方案1 + 方案4 混合

```python
# 1. Casual as Fallback (基础)
ROUTES = [code, reasoning, research]  # 无 casual

# 2. 分类别阈值 (精细控制)
THRESHOLDS = {
    "code_simple": 0.6,
    "code_complex": 0.7,
    "reasoning": 0.5,
    "research": 0.8,
}

# 3. 分类逻辑
def classify(text):
    decision = route_layer(text)
    
    if decision is None:
        return {"route": "casual", "reason": "no_match"}
    
    threshold = THRESHOLDS.get(decision.name, 0.5)
    
    if decision.confidence >= threshold:
        return {
            "route": decision.name,
            "confidence": decision.confidence
        }
    else:
        return {
            "route": "casual",
            "reason": f"low_confidence_{decision.name}",
            "original_match": decision.name,
            "confidence": decision.confidence
        }
```

---

## 📊 实际效果预测

| 输入 | 当前v2(有casual utterances) | v3(fallback策略) | 准确性 |
|------|---------------------------|-----------------|--------|
| "写一个排序算法" | code_simple ✅ | code_simple ✅ | ✅ |
| "今天天气怎么样" | casual(运气好) ❓ | casual ✅ | ✅ |
| "推荐个餐厅" | code_simple ❌ | casual ✅ | ✅ |
| "讲个笑话" | casual(运气好) ❓ | casual ✅ | ✅ |
| "逐步分析" | reasoning ✅ | reasoning ✅ | ✅ |
| "hello" | casual ✅ | casual ✅ | ✅ |

**v3 准确率更高！因为不需要穷举 casual 的所有可能**

---

## 🔧 实施建议

### 立即可3)
1. 移除 casual 的 utterances
2. 实现 fallback 逻辑
3. 设置置信度阈值 0.5

### 1周后 (优化)
1. 收集100个 casual 样本测试
2. 调整阈值
3. 观察误判率

### 1个月后 (进阶)
1. 实现分类别阈值
2. 从配置文件加载
3. 考虑负样本训练

---

## 💡 关键洞察

**Casual 的本质**: 它不是一个"类别"，而是"其他所有"的集合。

就像垃圾分类:
- 纸类 ← 明确
- 塑料 ← 明确
- 金属 ← 明确
- **其他垃圾** ← 兜底

Casual 就是 LLM 路由中的"其他垃圾"，应该用 fallback 策略，而不是试图定义它。

---

## 下一步

我已经创建了 v3 实现: `main_v3_fallback.py`

要测试吗？
```bash
# 启动 v3
uv run python -m embedding_service.main_v3_fallback

# 测试各种 casual 场景
curl -X POST http://localhost:8001/classify \
  -d '{"text": "今天天气怎么样"}'
```
