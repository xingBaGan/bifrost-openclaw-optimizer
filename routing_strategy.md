# Bifrost 路由策略设计

## 核心原则

**同一会话，同一模型** - 保证上下文连续性和输出一致性

## 路由粒度

### 1. 会话级路由（Session-level Routing）

**适用场景:** 多轮对话、复杂任务、代码重构

```
规则:
- 客户端发送 x-session-id header
- Bifrost 记录 session → model 的映射
- 同一 session 的后续请求，强制使用相同模型
- 会话结束后，映射记录过期（1小时）

示例:
Request 1: x-session-id=abc123, x-task-type=heavy_coding
→ 路由到 Claude Opus
→ 记录: abc123 → claude-opus

Request 2: x-session-id=abc123, x-task-type=light_coding
→ 忽略 task-type，强制使用 Claude Opus（保持一致性）
→ 返回 X-Bifrost-Session-Model: claude-opus header

Request 3: x-session-id=abc123, ...
→ 继续用 Claude Opus，直到会话结束
```

**实现:**
```go
type SessionStore struct {
    sessions map[string]SessionInfo
    mu       sync.RWMutex
}

type SessionInfo struct {
    Model     string
    Provider  string
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (r *Router) RouteWithSession(req *Request) (*Target, error) {
    sessionID := req.Headers.Get("x-session-id")

    // 如果有 session ID，检查是否已存在
    if sessionID != "" {
        if existing := r.sessions.Get(sessionID); existing != nil {
            // 强制使用已绑定的模型
            return &Target{
                Provider: existing.Provider,
                Model:    existing.Model,
                Reason:   "session-locked",
            }, nil
        }
    }

    // 新会话或无 session ID，正常路由
    target := r.routeByRules(req)

    // 如果有 session ID，记录绑定关系
    if sessionID != "" {
        r.sessions.Set(sessionID, SessionInfo{
            Model:     target.Model,
            Provider:  target.Provider,
            CreatedAt: time.Now(),
        })
    }

    return target, nil
}
```

### 2. 任务级路由（Task-level Routing）

**适用场景:** 一次性查询、独立任务、无需多轮对话

```
规则:
- 不发送 x-session-id（或 session-id 为空）
- Bifrost 根据 x-task-type、x-tier 等规则路由
- 每个请求独立决策，不记录状态

示例:
Request 1: x-task-type=quick_query (无 session-id)
→ 路由到 DeepSeek
→ 完成，不记录状态

Request 2: x-task-type=quick_query (无 session-id)
→ 再次路由到 DeepSeek
→ 与上一个请求无关

Request 3: x-task-type=reasoning_complex (无 session-id)
→ 路由到 Claude Opus
→ 独立任务
```

**适用工具:**
- Cursor CMD+K（快速编辑）
- Claude Code 的单次查询
- API 调用中的独立请求

### 3. 容灾级路由（Fallback Routing）

**适用场景:** 主模型故障、超时、限流

```
规则:
- 主模型失败时，自动尝试 fallback 模型
- 即使在会话中，也允许降级（因为没有选择）
- 记录降级事件，供后续分析

示例:
Request: x-session-id=xyz, 绑定模型=Claude Opus
→ 请求 Claude Opus
→ 503 Service Unavailable
→ 自动 fallback 到 GPT-4
→ 更新会话绑定: xyz → gpt-4（记录原因: claude-fallback）
→ 返回 X-Bifrost-Fallback: claude-opus->gpt-4 header

后续请求: x-session-id=xyz
→ 继续用 gpt-4（保持一致性）
```

**注意:**
- Fallback 后，会话绑定更新为新模型
- 避免在一个会话中反复切换
- 客户端可通过响应 header 了解实际使用的模型

### 4. 预算级路由（Budget-based Routing）

**适用场景:** 成本控制、自动降级

```
规则:
- 设置每月/每周/每天的成本上限
- 降级到经济模型
- 已有会话不受影响（保持一致性）

示例:
月预算: $100
当前消费: $95

新请求: x-task-type=heavy_coding (无 session-id)
→ 正常应该用 Claude Opus
→ 检测到预算即将耗尽
→ 降级到 DeepSeek
→ 返回 X-Bifrost-Budget-Override: true header

正在进行的会话: x-session-id=abc (绑定 Claude Opus)
→ 继续用 Claude Opus（不中断用户体验）
→ 但发出预算警告
```

## 客户端集成指南

### Cursor 集成示例

```typescript
// Cursor 的两种模式

// 模式 1: CMD+K 快速编辑（任务级路由）
async function quickEdit(prompt: string) {
    const response = await fetch('http://localhost:8000/v1/chat/completions', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'x-task-type': 'quick_query',  // 让 Bifrost 选择经济模型
            // 不发送 x-session-id
        },
        body: JSON.stringify({
            messages: [{ role: 'user', content: prompt }]
        })
    });
    // 每次请求都是独立的，Bifrost 可能会用不同模型（但用户不关心）
}

// 模式 2: CMD+L 对话（会话级路由）
class ChatSession {
    sessionId = generateUUID();

    async sendMessage(prompt: string) {
        const response = await fetch('http://localhost:8000/v1/chat/completions', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'x-session-id': this.sessionId,  // 绑定会话
                'x-task-type': 'heavy_coding',   // 首次请求时决策，后续忽略
            },
            body: JSON.stringify({
                messages: this.history  // 包含历史消息
            })
        });

        // Bifrost 保证同一 session-id 使用同一模型
        // 用户体验一致，代码风格统一
    }
}
```

### OpenClaw 集成示例

```typescript
// OpenClaw 的智能路由

async function executeCommand(command: string, context: Context) {
    // 根据命令类型决定是否需要会话
    const needsSession = analyzeCommand(command);

    if (needsSession) {
        // 复杂任务，创建会话
        const session = new Session({
            sessionId: generateUUID(),
            taskType: 'heavy_coding',
            tier: 'quality'
        });
        return await session.execute(command, context);
    } else {
        // 简单查询，任务级路由
        return await oneOffRequest(command, {
            taskType: 'quick_query',
            tier: 'economy'
        });
    }
}
```

## 响应 Headers

Bifrost 应该返回这些 header，帮助客户端理解路由决策：

```
X-Bifrost-Model: claude-3-5-sonnet-20241022
X-Bifrost-Provider: anthropic
X-Bifrost-Routing-Reason: session-locked | rule-based | fallback | budget-override
X-Bifrost-Session-Model: claude-3-5-sonnet-20241022  (如果是会话级路由)
X-Bifrost-Fallback: original-model->fallback-model  (如果发生了降级)
X-Bifrost-Cost: 0.0025  (本次请求的成本，美元)
X-Bifrost-Budget-Remaining: 45.30  (剩余预算，美元)
```

## 路由决策流程图

```
请求到达
    ↓
检查 x-session-id
    ↓
    ├─ 有 session-id ───→ 查询 session 绑定 ─→ 找到 ─→ 使用绑定的模型
    │                                        ↓
    │                                      未找到
    │                                        ↓
    └─ 无 session-id ───────────────────────┴───→ 应用路由规则
                                                      ↓
                                                  选择目标模型
                                                      ↓
                                              检查预算/配额
                                                      ↓
                                              ├─ 在预算内 ─→ 使用选定模型
                                              │                   ↓
                                              └─ 超预算 ─→ 降级到经济模型
                                                                  ↓
                                                          请求目标 provider
                                                                  ↓
                                                          ├─ 成功 ─→ 返回结果
                                                          │             ↓
                                                          └─ 失败 ─→ Fallback
                                                                        ↓
                                                                   更新会话绑定
                                                                        ↓
                                                                   返回结果
```

## 优先级

**会话锁定 > 容灾降级 > 预算控制 > 路由规则**

```
1. 会话锁定（最高优先级）
   - 如果请求属于已有会话，强制使用会话绑定的模型
   - 保证上下文连续性

2. 容灾降级
   - 如果会话绑定的模型不可用，fallback 到备用模型
   - 更新会话绑定

3. 预算控制
   - 如果预算不足，新会话降级到经济模型
   - 已有会话不受影响

4. 路由规则（最低优先级）
   - 对于新会话/独立任务，应用 CEL 规则
```

## 总结

**Bifrost 的价值不是频繁切换，而是智能选择：**

1. **一次性任务** → 自动用便宜模型（用户无感知）
2. **持续对话** → 自动锁定一个高质量模型（保证一致性）
3. **故障容灾** → 自动 fallback（提高可用性）
4. **成本控制** → 自动降级新会话（保护预算）

**用户得到的:**
- 💰 成本降低 70-80%（通过任务级优化）
- 🎯 体验一致（会话内不切换）
- 🛡️ 更高可用性（自动 fallback）
- 📊 透明可控（清晰的 headers 反馈）

---

*文档版本: 2.0*
*更新时间: 2026-04-02*
