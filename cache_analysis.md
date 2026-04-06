# Provider 缓存机制与切换成本分析

## 核心事实：AI 模型是无状态的

**每次 API 调用都需要发送完整上下文：**

```json
Request 1:
{
  "messages": [
    {"role": "user", "content": "帮我写个函数"}
  ]
}

Request 2:
{
  "messages": [
    {"role": "user", "content": "帮我写个函数"},
    {"role": "assistant", "content": "def foo(): ..."},
    {"role": "user", "content": "加个参数"}
  ]
}

Request 3:
{
  "messages": [
    {"role": "user", "content": "帮我写个函数"},
    {"role": "assistant", "content": "def foo(): ..."},
    {"role": "user", "content": "加个参数"},
    {"role": "assistant", "content": "def foo(param): ..."},
    {"role": "user", "content": "加上错误处理"}
  ]
}
```

**关键洞察：**
- 模型本身没有记忆
- 客户端（OpenClaw/Cursor）维护完整对话历史
- 每次请求都发送所有 messages
- **对话越长，输入 tokens 越多，成本越高**

## Provider 缓存机制详解

### 1. Anthropic Prompt Caching

**文档：** https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching

**机制：**
```javascript
{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {
      "role": "user",
      "content": "系统提示词（5000 tokens）...",
      "cache_control": {"type": "ephemeral"}  // 标记为可缓存
    },
    {
      "role": "user",
      "content": "对话历史 1"
    },
    {
      "role": "assistant",
      "content": "回复 1"
    },
    // ... 更多历史 40k tokens ...
    {
      "role": "user",
      "content": "对话历史 10",
      "cache_control": {"type": "ephemeral"}  // 再次缓存断点
    },
    {
      "role": "user",
      "content": "新问题（500 tokens）"
    }
  ]
}
```

**缓存规则：**
- 必须手动标记 `cache_control` 的位置
- 可缓存内容必须 ≥ 1024 tokens（2048 tokens for Claude 3.5 Haiku）
- 最多 4 个缓存断点
- 缓存有效期：5 分钟（活跃使用时自动延长）
- 前缀必须完全匹配才能命中

**价格（Claude 3.5 Sonnet）：**
```
正常输入: $3.00/M tokens
缓存写入: $3.75/M tokens (首次，+25%)
缓存读取: $0.30/M tokens (命中，-90%)
输出:     $15.00/M tokens
```

**示例计算：**
```
对话历史: 50k tokens (已缓存)
新问题: 500 tokens
输出: 1k tokens

成本:
- 缓存读取 50k: 50,000 × $0.30/M = $0.015
- 新问题 500: 500 × $3.00/M = $0.0015
- 输出 1k: 1,000 × $15.00/M = $0.015
总计: $0.0315

如果没有缓存:
- 输入 50.5k: 50,500 × $3.00/M = $0.1515
- 输出 1k: $0.015
总计: $0.1665

缓存节省: $0.135 (81%)
```

### 2. OpenAI Prompt Caching

**文档：** https://platform.openai.com/docs/guides/prompt-caching

**机制：**
```javascript
{
  "model": "gpt-4o",
  "messages": [
    // 自动缓存，无需标记
  ]
}
```

**缓存规则：**
- **自动启用**，无需配置
- 缓存前缀 ≥ 1024 tokens
- 缓存有效期：5-10 分钟
- 前缀完全匹配即命中
- 只有某些模型支持：gpt-4o, gpt-4o-mini, o1-preview, o1-mini

**价格（GPT-4o）：**
```
正常输入: $2.50/M tokens
缓存读取: $1.25/M tokens (50% 折扣)
输出:     $10.00/M tokens
```

**示例计算：**
```
对话历史: 50k tokens (已缓存)
新问题: 500 tokens
输出: 1k tokens

成本:
- 缓存读取 50k: 50,000 × $1.25/M = $0.0625
- 新问题 500: 500 × $2.50/M = $0.00125
- 输出 1k: 1,000 × $10.00/M = $0.01
总计: $0.0738

如果没有缓存:
- 输入 50.5k: 50,500 × $2.50/M = $0.1263
- 输出 1k: $0.01
总计: $0.1363

缓存节省: $0.0625 (46%)
```

### 3. 其他 Provider

| Provider | 缓存支持 | 备注 |
|----------|---------|------|
| **DeepSeek** | ❌ 不支持 | 每次全量计费 |
| **Kimi** | ❌ 不支持 | 每次全量计费 |
| **OpenRouter** | ⚠️ 取决于后端 | 如果是 Claude/GPT-4o 可能支持 |
| **Gemini** | ✅ 支持 | Context Caching，类似 Anthropic |

## 切换模型的真实成本

### 场景设定

```
对话已进行 10 轮
对话历史: 50k tokens
新问题: 500 tokens
期望输出: 1k tokens
```

### 方案 A: 继续用 Claude（有缓存）

```
成本:
- 历史 50k (缓存读): $0.015
- 新问题 500: $0.0015
- 输出 1k: $0.015
总计: $0.0315
```

### 方案 B: 切换到 DeepSeek（无缓存）

```
成本:
- 历史 50k (全量): 50,000 × $0.14/M = $0.007
- 新问题 500: 500 × $0.14/M = $0.00007
- 输出 1k: 1,000 × $0.28/M = $0.00028
总计: $0.0073

对比: 比 Claude 便宜 77%
```

**结论：** ✅ 即使没有缓存，DeepSeek 还是便宜很多

### 方案 C: 切换到 GPT-4o（无缓存）

```
成本:
- 历史 50k (全量): 50,000 × $2.50/M = $0.125
- 新问题 500: $0.00125
- 输出 1k: $0.01
总计: $0.136

对比: 比 Claude (有缓存) 贵 4.3 倍！
```

**结论：** ❌ 切换到 GPT-4o 反而更贵

### 方案 D: 切换到 Kimi K2.5（无缓存）

```
价格假设: $1.00/M input, $2.00/M output

成本:
- 历史 50k (全量): 50,000 × $1.00/M = $0.05
- 新问题 500: $0.0005
- 输出 1k: 1,000 × $2.00/M = $0.002
总计: $0.0525

对比: 比 Claude (有缓存) 贵 67%
```

**结论：** ❌ 切换到中档模型也不划算

## 关键洞察：何时切换才值得？

### 公式推导

设：
- `C_current` = 当前模型成本（有缓存）
- `C_new` = 新模型成本（无缓存）
- `H` = 历史 tokens
- `N` = 新问题 tokens
- `O` = 输出 tokens

当前模型（Claude，有缓存）：
```
C_current = H × $0.30/M + N × $3.00/M + O × $15.00/M
```

新模型（DeepSeek，无缓存）：
```
C_new = (H + N) × $0.14/M + O × $0.28/M
```

**切换有利条件：**
```
C_new < C_current

(H + N) × 0.14 + O × 0.28 < H × 0.30 + N × 3.00 + O × 15.00

简化：
H × (0.14 - 0.30) < N × (3.00 - 0.14) + O × (15.00 - 0.28)
H × (-0.16) < N × 2.86 + O × 14.72

关键：负数，所以切换总是有利！
```

**结论：** 切换到 DeepSeek 几乎总是更便宜，即使对方没有缓存

### 不同历史长度的切换阈值

| 历史长度 | Claude (缓存) | DeepSeek (无缓存) | 节省 |
|---------|--------------|------------------|-----|
| 10k | $0.0105 | $0.0029 | 72% |
| 50k | $0.0315 | $0.0073 | 77% |
| 100k | $0.0615 | $0.0143 | 77% |
| 200k | $0.1215 | $0.0283 | 77% |

**观察：**
- 节省比例稳定在 77% 左右
- 历史越长，绝对节省越多
- DeepSeek 便宜到即使没缓存也碾压

### 什么时候切换会亏？

**只有一种情况：切换到其他贵模型**

```
切换到 GPT-4o:
- Claude (缓存) 50k: $0.0315
- GPT-4o (无缓存) 50k: $0.136
亏损: 4.3x

切换到 Kimi:
- Claude (缓存) 50k: $0.0315
- Kimi (无缓存) 50k: $0.0525
亏损: 1.67x
```

## Bifrost 的缓存策略

### 问题：Bifrost 如何传递缓存？

**Bifrost 是个网关，不修改请求内容**

```javascript
// 客户端发送
{
  "messages": [
    {
      "role": "user",
      "content": "历史...",
      "cache_control": {"type": "ephemeral"}  // Claude 专用
    },
    ...
  ]
}

// Bifrost 转发给 Claude → 缓存生效 ✅
// Bifrost 转发给 OpenAI → 自动缓存 ✅
// Bifrost 转发给 DeepSeek → 无缓存 ✅ (但价格低)
```

**好消息：** Bifrost 不需要特殊处理，直接透传即可

### 客户端需要做的：正确标记缓存点

**OpenClaw/Cursor 需要：**

```typescript
function buildMessages(history: Message[], newPrompt: string) {
    const messages = [];

    // 1. 系统提示（如果有）
    if (systemPrompt) {
        messages.push({
            role: "user",
            content: systemPrompt,
            cache_control: {type: "ephemeral"}  // 缓存系统提示
        });
    }

    // 2. 对话历史
    for (let i = 0; i < history.length; i++) {
        messages.push(history[i]);

        // 每 5 轮对话设置一个缓存点
        if (i > 0 && i % 10 === 0) {
            messages[messages.length - 1].cache_control = {type: "ephemeral"};
        }
    }

    // 3. 最新一轮对话之前的最后一条，设置缓存点
    if (messages.length > 0) {
        messages[messages.length - 1].cache_control = {type: "ephemeral"};
    }

    // 4. 新问题（不缓存，因为每次都不同）
    messages.push({
        role: "user",
        content: newPrompt
    });

    return messages;
}
```

**缓存点策略：**
- 系统提示：总是缓存（很少变）
- 对话历史：每 10 轮设置一个缓存点
- 最近一轮：缓存（下次请求会复用）
- 新问题：不缓存（每次都不同）

### Bifrost 的优化：缓存感知路由

**策略：优先使用已有缓存的模型**

```python
class CacheAwareRouter:
    def __init__(self):
        # 记录每个物理会话使用的模型
        # session_id -> model_name
        self.session_models = {}

    def route(self, request):
        session_id = request.headers.get("x-physical-session-id")

        # 判断是否新任务
        if self.is_new_task(request):
            # 新任务，根据任务类型选择模型
            model = self.select_by_task_type(request)
        else:
            # 延续任务，优先复用上次的模型（缓存）
            last_model = self.session_models.get(session_id)

            if last_model:
                # 检查：切换模型是否真的更划算？
                current_cost = self.estimate_cost_with_cache(
                    model=last_model,
                    history_tokens=request.history_size
                )

                best_model = self.select_by_task_type(request)
                new_cost = self.estimate_cost_without_cache(
                    model=best_model,
                    history_tokens=request.history_size
                )

      f new_cost < current_cost * 0.5:  # 只有便宜 50% 以上才切
                    model = best_model
                    # 记录日志：因为成本优势，切换了模型
                else:
                    model = last_model
                    # 记录日志：虽然新任务，但保持模型以利用缓存
            else:
                model = self.select_by_task_type(request)

        # 更新会话模型记录
        self.session_models[session_id] = model
        return model
```

**决策矩阵：**

| 情况 | 当前模型 | 建议模型 | 历史长度 | 决策 |
|------|---------|---------|---------|-----|
| 新任务 | Claude | DeepSeek | 50k | ✅ 切换（节省 77%） |
| 新任务 | Claude | GPT-4o | 50k | ❌ 不切（会贵 4x） |
| 新任务 | DeepSeek | Claude | 50k | ⚠️ 看需求（质量 vs 成本） |
| 延续 | Claude | DeepSeek | 任何 | ❌ 不切（保持一致性） |
| 延续 | DeepSeek | Claude | 任何 | ❌ 不切（保持一致性） |

## 实际成本模拟

### 场景：一天的 OpenClaw 使用

```
09:00 会话开始
  Request 1: "看看这个 bug"
  → Claude Opus (质量任务)
  → History: 0, Cost: $0.015

09:15 延续调试
  Request 2: "试试这个方案"
  → Claude Opus (缓存命中)
  → History: 5k, Cost: $0.020

09:30 新任务：知识查询
  Request 3: "解释一下 Redis"
  → 判断：新任务，切换到 DeepSeek
  → History: 10k (无缓存，因为新模型)
  → Cost: $0.003

09:45 延续查询
  Request 4: "Redis 和 Memcached 对比"
  → DeepSeek (保持一致)
  → History: 15k, Cost: $0.004

10:00 新任务：复杂重构
  Request 5: "重构整个认证模块"
  → 判断：新任务，质量优先，切回 Claude
  → History: 20k (无缓存，因为切回来了)
  → Cost: $0.075

10:30 延续重构
  Request 6: "这个接口怎么设计？"
  → Claude (缓存命中)
  → History: 30k, Cost: $0.025: $0.142

如果全用 Claude (有缓存):
Request 1: $0.015
Request 2: $0.020
Request 3: $0.035 (缓存)
Request 4: $0.040 (缓存)
Request 5: $0.060 (缓存)
Request 6: $0.045 (缓存)
总成本: $0.215

节省: $0.073 (34%)
```

**观察：**
- 新任务切换到便宜模型，值得（即使失去缓存）
- 延续任务保持模型，利用缓存
- 综合节省 30-40%

## 最佳实践建议

### 对于 OpenClaw/Cursor（客户端）

1. **正确标记缓存点**
   ```typescript
   // 系统提示、工具定义等静态内容 → 总是缓存
   // 对话历史 → 每 10 轮缓存一次轮 → 缓存
   ```

2. **发送物理会话 ID**
   ```typescript
   headers: {
       "x-physical-session-id": "abc-123"  // Bifrost 用于跟踪
   }
   ```

3. **提供任务类型提示**
   ```typescript
   headers: {
       "x-task-type": "quick_query" | "heavy_coding"
   }
   ```

### 对于 Bifrost（网关）

1. **缓存感知路由**
   - 记录每个会话最后使用的模型
   - 延续任务优先复用（利用缓存）
   - 新任务评估切换成本

2. **成本估算**
   ```python
   def estimate_switching_cost(
       current_model,
       new_model,
       history_tokens,
       new_tokens,
       output_tokens
   ):
       # 当前模型（有缓存）
       current = calculate_with_cache(...)

       # 新模型（无缓存）
       new = calculate_without_cache(...)

       return new - current  # 正数 = 更贵，负数 = 更便宜
   ```

3. **智能切换策略**
   ```
   规则 1: 延续任务 → 永远不切换
   规则 2: 新任务 + 切换能省 50%+ → 切换
   规则 3: 新任务 + 切换会更贵 → 不切换
   规则 4: 用户强制指定 → 遵从用户
   ```

## 总结

### 关键发现

1. **AI 模型是无状态的**
   - 每次都发送完整 context
   - 对话越长，输入成本越高

2. **缓存是成本优化的关键**
   - Anthropic: 90% 折扣
   - OpenAI: 50% 折扣
   - DeepSeek/Kimi: 不支持

3. **切换模型会失去缓存**
   - 但如果新模型足够便宜（如 DeepSeek），依然划算
   - 切换到其他贵模型（如 GPT-4o）会亏

4. **最优策略：**
   - 新任务 → 根据任务类型选择，可以切换
   - 延续任务 → 保持模型，利用缓存
   - 优先切换到超便宜模型（DeepSeek）
   - 避免切换到中高价模型

### Bifrost 的价值重新定义

**不是单纯的成本优化，而是：**
- ✅ 任务级智能选择（新任务用最优模型）
- ✅ 缓存感知路由（延续任务复用缓存）
- ✅ 成本-质量平衡（综合评估）

**预期效果：**
- 成本节省：30-40%（而非之前估计的 70-80%）
- 质量保证：关键任务用高质量模型
- 体验一致：同任务保持模型不变

---

*文档版本: 1.0*
*创建时间: 2026-04-02*
*重要更新: 纠正了之前对缓存机制的忽视*
