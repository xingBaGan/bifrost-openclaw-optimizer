# Bifrost 语义会话管理设计

## 问题定义

**用户真实行为：**
- 在**一个长物理会话**中，做很多不同的事情
- 不会主动创建多个会话（他们不知道这个概念）
- 但这个长会话中，包含了多个**逻辑上独立的任务**

**示例：OpenClaw 的真实使用场景**
```
一个会话，从早到晚：

10:00  "帮我看看这个认证bug"          → 复杂调试，应该用 Claude Opus
10:15  "给这个函数加个注释"            → 简单任务，可以用 DeepSeek
10:20  "解释一下 JWT 的原理"           → 知识查询，可以用 DeepSeek
10:30  "重构整个用户模块的架构"        → 复杂重构，应该用 Claude Opus
10:45  "生成单元测试"                  → 简单生成，可以用 DeepSeek
11:00  "这个测试失败了，帮我看看"      → 延续上一个任务，还用 DeepSeek
```

**核心挑战：**
- 10:15 的请求 → 新任务，可以换成 DeepSeek
- 11:00 的请求 → 延续 10:45 的任务，必须保持 DeepSeek

**Bifrost 需要自动判断：这个请求是新任务，还是延续上一个任务？**

## 解决方案：语义会话管理

### 概念模型

```
物理会话（Physical Session）
├─ 逻辑子会话 1（Sub-session 1）- Claude Opus
│  ├─ Request 1: "看看这个bug"
│  └─ Request 2: "试试这个修复方案"
├─ 逻辑子会话 2（Sub-session 2）- DeepSeek
│  └─ Request 3: "解释 JWT 原理"
├─ 逻辑子会话 3（Sub-session 3）- Claude Opus
│  ├─ Request 4: "重构用户模块"
│  └─ Request 5: "这里的依赖注入怎么处理？"
└─ 逻辑子会话 4（Sub-session 4）- DeepSeek
   ├─ Request 6: "生成单元测试"
   └─ Request 7: "这个测试失败了，看看"
```

**规则：**
- 同一个**逻辑子会话**内，必须使用同一个模型
- 不同**逻辑子会话**之间，可以切换模型

### 核心算法：任务分割

#### 1. 基于规则的快速判断（第一版）

**新任务的强信号：**
```python
NEW_TASK_PATTERNS = [
    # 明确的切换词
    r"^(现在|接下来|换一个|另外)",
    r"^(帮我|请|麻烦).*(另一个|新的)",

 通常是新对话开始）
    r"^(你好|嗨|hi|hello)",

    # 完全不同的动词
    # 上一轮是"调试"，这一轮是"解释"
    # 上一轮是"重构"，这一轮是"生成测试"
]

CONTINUATION_PATTERNS = [
    # 明确的指代
    r"(这个|这里|刚才|上面|之前)",

    # 追问
    r"(为什么|怎么|如何)",

    # 修改指令
    r"^(再|还|也|改成|加上|去掉)",

    # 错误反馈
    r"(不对|错了|失败|报错)",
]
```

**判断流程：**
```python
def is_new_task(current_request, last_request, last_response):
    # 1. 检查强信号
    if matches_any(current_request, NEW_TASK_PATTERNS):
        return True

    if matches_any(current_request, CONTINUATION_PATTERNS):
        return False

    # 2. 检查时间间隔
    if time_since_last_request > 30_minutes:
        return True  # 可能是新话题

    # 3. 检查话题相似度
    topic_similarity = calculate_topic_similarity(
        current_request,
        last_request + last_response
    )
    if topic_similarity < 0.3:
        return True  # 话题差异大，可能是新任务

    # 4. 检查任务类型切换
    current_task_type = classify_task_type(current_request)
    last_task_type = classify_task_type(last_request)

    if current_task_type != last_task_type:
        # 任务类型切换，但需要确认是否有指代关系
        if has_reference(current_request, last_response):
            return False  # 有指代，是延续
        else:
            return True  # 无指代，是新任务

    # 默认：延续
    return False
```

#### 2. 基于 LLM 的智能判断（第二版）

使用一个**轻量级模型**（DeepSeek/Haiku）来判断：

```python
CLASSIFIER_PROMPT = """
你是一个任务分割助手。判断用户的新请求是【新任务】还是【延续上一个任务】。

上一轮对话：
用户: {last_request}
助手: {last_response}

这一轮请求：
用户: {current_request}

判断规则：
- 如果新请求明确引用上一轮的内容（"这个"、"刚才的"），是【延续】
- 如果新请求的话题/任务类型完全不同，是【新任务】
- 如果新请求是追问/修正/深入，是【延续】
- 如果新请求是全新的问题/指令，是【新任务】

只回答：NEW_TASK 或 CONTINUATION
"""

async def is_new_task_llm(current, last_req, last_resp):
    prompt = CLASSIFIER_PROMPT.format(
        current_request=current,
        last_request=last_req,
        last_response=last_resp[:500]  # 只看前 500 字符
    )

    # 用便宜的模型判断（成本 ~$0.0001）
    result = await call_llm(model="deepseek-chat", prompt=prompt)

    return result.strip() == "NEW_TASK"
```

**优点：**
- 更准确，理解语义
- 能处理复杂的边界情况

**成本控制：**
- 使用最便宜的模型（DeepSeek: $0.14/M tokens）
- 只发送摘要上下文（500 字符）
- 每次分类成本 < $0.0001

#### 3. 混合方案（推荐）

```python
async def is_new_task_hybrid(current, last_req, last_resp, history):
    # 1. 快速规则检查（免费，毫秒级）
    if matches_any(current, CONTINUATION_PATTERNS):
        return False  # 明确是延续，不需要 LLM

    if matches_any(current, NEW_TASK_PATTERNS):
        # 可能是新任务，但用 LLM 再确认一下
        pass

    # 2. 时间启发式
    if time_since_last > 30_minutes:
        return True  # 很可能是新任务

    # 3. 任务类型快速检查
    current_type = classify_task_type(current)
    last_type = classify_task_type(last_req)

    if current_type == last_type:
        return False  # 同类型，可能是延续

    # 4. 不确定的情况，用 LLM 判断
    return await is_new_task_llm(current, last_req, last_resp)
```

**性能：**
- 80% 的情况通过规则快速判断
- 20% 的边界情况用 LLM（成本可控）

### 数据结构

```go
// 物理会话
type PhysicalSession struct {
    ID           string
    ClientID     string
    CreatedAt    time.Time
    LastActiveAt time.Time
    SubSessions  []*SubSession
}

// 逻辑子会话
type SubSession struct {
    ID        string
    ParentID  string  // 物理会话ID
    Model     string
    Provider  string
    StartedAt time.Time
    EndedAt   *time.Time  // nil 表示当前活跃

    // 用于判断延续的上下文
    FirstRequest  string
    LastRequest   string
    LastResponse  string
    TaskType      string
    TopicKeywords []string
}

// 路由决策
type RoutingDecision struct {
    IsNewTask     bool
    SubSessionID  string
    SelectedModel string
    Reason        string  // "new-task" | "continuation" | "fallback"
}
```

### 路由流程

```
请求到达
    ↓
提取物理会话 ID
    ↓
加载物理会话历史
    ↓
获取最后一个子会话
    ↓
判断：新任务 or 延续？
    ↓
    ├─ 新任务 ─────────→ 应用路由规则 ─→ 选择最优模型
    │                                       ↓
    │                                   创建新子会话
    │                                       ↓
    └─ 延续 ───────────→ 使用上一个子会话的模型
                              ↓
                          更新子会话
                              ↓
                        请求目标模型
                              ↓
                          返回结果
```

### API 设计

#### 请求 Headers

```http
POST /v1/chat/completions

Headers:
x-physical-session-id: abc-123-xyz     # 物理会话ID（客户端维护）
x-task-type: heavy_coding              # 任务类型提示（可选）
x-tier: quality                        # 质量层级（可选）
x-force-new-task: true                 # 强制开启新任务（可选）
```

**注意：**
- 客户端只需要维护**一个**物理会话ID
- Bifrost 自动管理逻辑子会话
- 用户无感知

#### 响应 Headers

```http
X-Bifrost-Model: claude-3-5-sonnet
X-Bifrost-Provider: anthropic
X-Bifrost-Sub-Session-Id: sub-456
X-Bifrost-Task-Decision: new-task | continuation
X-Bifrost-Task-Decision-Reason: topic-switch | explicit-reference | time-gap
X-Bifrost-Cost: 0.0025
```

### 客户端集成

#### OpenClaw 集成示例

```typescript
// OpenClaw 客户端
class BifrostClient {
    // 只维护一个物理会话ID
    private physicalSessionId = generateUUID();

    async sendMessage(prompt: string, context: Context) {
        const response = await fetch('http://localhost:8000/v1/chat/completions', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',

                // 只发送物理会话ID
                'x-physical-session-id': this.physicalSessionId,

                // 可选：提供任务类型提示（帮助 Bifrost 更准确）
                'x-task-type': this.inferTaskType(prompt),
                'x-tier': this.inferTier(prompt),
            },
            body: JSON.stringify({
                messages: this.buildMessages(prompt, context)
            })
        });

        // 从响应 header 了解 Bifrost 的决策
        const decision = response.headers.get('X-Bifrost-Task-Decision');
        const model = response.headers.get('X-Bifrost-Model');

        console.log(`Bifrost decision: ${decision}, using model: ${model}`);

        return await response.json();
    }

    // OpenClaw 可以提供任务类型提示，但不是必须的
    private inferTaskType(prompt: string): string {
        if (prompt.includes('bug') || prompt.includes('错误')) {
            return 'debugging';
        }
        if (prompt.includes('重构') || prompt.includes('refactor')) {
            return 'heavy_coding';
        }
        if (prompt.includes('解释') || prompt.includes('是什么')) {
            return 'quick_query';
        }
        return 'general';
    }
}
```

**关键：**
- OpenClaw 只需要维护一个会话ID
- 不需要手动管理子会话
- Bifrost 自动判断和切换

#### Cursor 集成示例

```typescript
// Cursor 可以更简单，甚至不需要会话ID
async function sendToBifrost(prompt: string, history: Message[]) {
    const response = await fetch('http://localhost:8000/v1/chat/completions', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
       // 不发送会话ID，Bifrost 根据历史上下文判断
        },
        body: JSON.stringify({
            messages: [...history, { role: 'user', content: prompt }]
        })
    });

    return await response.json();
}
```

**Bifrost 会：**
- 分析 messages 中的历史对话
- 自动判断是否是新任务
- 自动选择最优模型

### 高级功能

#### 1. 手动任务标记

用户可以显式标记新任务：

```typescript
// 用户输入："/new 现在帮我重构代码"
// OpenClaw 识别 /new 指令，添加 header

fetch('/v1/chat', {
    headers: {
        'x-force-new-task': 'true'  // 强制开启新任务
    }
})
```

#### 2. 任务分组可视化

Bifrost Dashboard 可以展示任务分组：

```
物理会话: session-abc-123
├─ [10:00-10:15] 调试认证 bug (Claude Opus) - $0.05
├─ [10:15-10:20] 知识查询 (DeepSeek) - $0.001
├─ [10:30-10:50] 重构用户模块 (Claude Opus) - $0.08
└─ [10:45-11:00] 生成测试 (DeepSeek) - $0.002

总成本: $0.133
如果全用 Claude: $0.45
节省: 70.4%
```

#### 3. 学习用户习惯

```python
# 随着时间推移，Bifrost 学习用户的切换习惯
class UserPreference:
    def __init__(self, user_id):
        self.user_id = user_id
        self.switch_history = []

    def record_switch(self, context, user_feedback):
        # 用户手动切换模型 → 记录这个决策
        # 下次类似情况，自动应用
        pass

    def predict_should_switch(self, context):
        # 基于历史习惯预测
        pass
```

## 实现优先级

### Phase 1: 基础能力（MVP）

- [ ] 基于规则的快速判断
  - [ ] 识别明确的延续信号（"这个"、"刚才"）
  - [ ] 识别明确的切换信号（"现在"、"接下来"）
  - [ ] 时间间隔启发式（>30分钟 = 新任务）

- [ ] 子会话管理
  - [ ] 数据结构定义
  - [ ] 子会话创建/更新逻辑
  - [ ] 子会话绑定模型锁定

- [ ] 响应 Headers
  - [ ] 返回决策信息
  - [ ] 返回子会话ID

**目标:** 70% 的情况能正确判断，成本 = $0（纯规则）

### Phase 2: LLM 增强

- [ ] 集成轻量级分类模型
  - [ ] 使用 DeepSeek 做任务分割判断
  - [ ] 成本控制（只发送摘要上下文）

- [ ] 混合策略
  - [ ] 规则快速判断 + LLM 兜底
  - [ ] 动态阈值调整

**目标:** 9平均成本 < $0.0001/请求

### Phase 3: 高级功能

- [ ] 手动任务标记（x-force-new-task）
- [ ] Dashboard 可视化
- [ ] 用户习惯学习
- [ ] A/B 测试和优化

**目标:** 95%+ 准确率，用户满意度高

## 评估指标

### 1. 准确率

```
正确判断率 = 正确决策数 / 总决策数

人工标注 100 个真实对话，计算准确率
目标: Phase 1 > 70%, Phase 2 > 90%
```

### 2. 成本节省

```
节省率 = (全用贵模型成本 - 实际成本) / 全用贵模型成本

目标: 节省 60-80%
```

### 3. 用户满意度

```
- 输出质量是否下降？
- 是否有明显的不一致体验？
- 切换是否合理？

目标: 满意度 > 4/5
```

### 4. 误判成本

```
误判类型：
1. 假阳性（新任务判成延续）→ 用了不合适的便宜模型
2. 假阴性（延续判成新任务）→ 切换模型导致不一致

误判成本 = 假阳性损失 + 假阴性损失

目标: 误判率 < 10%
```

## 与现有方案对比

| 方案 | 用户体验 | 成本节省 | 实现复杂度 |
|------|---------|---------|-----------|
| **手动切换模型** | 需要用户主动操作 | 取决于用户是否记得切 | 低 |
| **会话级路由** | 一个会话一个模型 | 无法在会话内优化 | 低 |
| **语义会话管理** | 自动化，用户无感知 | 60-80% | 中 |

**Bifrost 的独特价值：**
- 用户完全无感知（不需要手动管理会话）
- 自动在长会话中优化成本
- 保持逻辑一致性（同一任务用同一模型）

## 总结

**问题：** 用户在一个长会话中做很多不同的事，如何自动切换模型？

**解决方案：** 语义会话管理
1. 自动判断新请求是新任务还是延续
2. 创建逻辑子会话，绑定模型
3. 同一子会话用同一模型，不同子会话可以切换

**技术路径：**
- Phase 1: 规则判断（70% 准确率，成本 $0）
- Phase 2: LLM 增强（90% 准确率，成本 < $0.0001）
- Phase 3: 学习优化（95%+ 准确率）

**预期效果：**
- 成本节省: 60-80%
- 用户体验: 无感知，自动优化
- 一致性: 同任务保持一致

**这是 Bifrost 的核心差异化能力！**

---

*文档版本: 1.0*
*创建时间: 2026-04-02*
