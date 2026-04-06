# Embedding Classifier 快速启动指南

## 🚀 5分钟MVP

### Step 1: 安装依赖 (Python端)

```bash
# 创建embedding服务目录
mkdir -p embedding_service && cd embedding_service

# 创建虚拟环境
python3 -m venv venv
source venv/bin/activate  # Windows: venv\Scripts\activate

# 安装依赖
pip install fastapi uvicorn sentence-transformers numpy
```

### Step 2: 创建服务 (embedding_service/app.py)

```python
from fastapi import FastAPI
from sentence_transformers import SentenceTransformer
from pydantic import BaseModel
import numpy as np

app = FastAPI()

# 加载模型 (首次会下载，约420MB)
print("Loading model...")
model = SentenceTransformer('paraphrase-multilingual-MiniLM-L12-v2')
print("Model loaded!")

# 意图示例 (简化版，3个类别)
EXAMPLES = {
    "code": ["写代码", "debug", "函数", "write code", "fix bug", "算法"],
    "think": ["逐步分析", "为什么", "推理", "step by step", "explain why"],
    "simple": ["你好", "天气", "hello", "how are you"],
}

# 预计算意图向量
print("Computing intent vectors...")
INTENT_VECTORS = {}
for intent, examples in EXAMPLES.items():
    vectors = model.encode(examples)
    INTENT_VECTORS[intent] = np.mean(vectors, axis=0)
print("Ready!")

class Request(BaseModel):
    text: str

@app.post("/classify")
def classify(req: Request):
    # 编码查询
    query_vec = model.encode([req.text])[0]
    
    # 计算相似度
    sims = {}
    for intent, vec in INTENT_VECTORS.items():
        sim = np.dot(query_vec, vec) / (np.linalg.norm(query_vec) * np.linalg.norm(vec))
        sims[intent] = float(sim)
    
    best = max(sims, key=sims.get)
    
    # 映射到路由
    routing = {
        "code": {"tier": "quality", "reasoning": "fast"},
     "think": {"tier": "quality", "reasoning": "think"},
        "simple": {"tier": "economy", "reasoning": "fast"},
    }
    
    return {
        "intent": best,
        "confidenc sims[best],
        "similarities": sims,
        "routing": routing[best],
    }

@app.get("/health")
def health():
    return {"status": "ok"}
```

### Step 3: 启动服务

```bash
python app.py
# 或
uvicorn app:app --host 0.0.0.0 --port 8001
```

### Step 4: 测试

```bash
# 测试1: 代码请求
curl -X POST http://localhost:8001/classify \
  -H "Content-Type: application/json" \
  -d '{"text": "帮我写一个快排算法"}'

# 期望输出: {"intent": "code", "confidence": 0.85, "routing": {"tier": "quality", ...}}

# 测试2: 推理请求
curl -X POST http://localhost:8001/classify \
  -H "Content-Type: application/json" \
  -d '{"text": "请逐步分析这个问题"}'

# 期望输出: {"intent": "think", ...}

# 测试3: 简单对话
curl -X POST http://localhost:8001/classify \
  -H "Content-Type: application/json" \
  -d '{"text": "你好，今天天气不错"}'

# 期望输出: {"intent": "simple", "routing": {"tier": "economy", ...}}
```

---

## 🔗 集成到Bifrost (Go端)

### Option A: 简单HTTP调用

在 `plugins/classifier/in_package.go` 添加:

```go
import (
    "bytes"
    "encoding/json"
    "net/http"
    "time"
)

type embeddingClient struct {
    url    string
    client *http.Client
}

func newEmbeddingClient(url string) *embeddingClient {
    return &embeddingClient{
        url: url,
        client: &http.Client{Timeout: 50 * time.Millisecond},
    }
}

func (ec *embeddingClient) classify(text string) (tier, reasoning string, err error) {
    reqData, _ := json.Marshal(map[string]string{"text": text})
    resp, err := ec.client.Post(ec.url+"/classify", "application/json", bytes.NewBuffer(reqData))
    if err != nil {
        return "", "", err
    }
    defer resp.Body.Close()
    
    var result struct {
        Routing struct {
            Tier      string `json:"tier"`
            Reasoning string `json:"reasoning"`
        } `json:"routing"`
        Confidence float64 `json:"confidence"`
    }
    
    json.NewDecoder(resp.Body).Decode(&result)
    
    if result.Confidence < 0.5 {
        return "", "", fmt.Errorf("low confidence")
    }
    
    return result.Routing.Tier, result.Routing.Reasoning, nil
}
```

### Option B: 混合分类

```go
func (p *ClassifierPlugin) HTTPTransportPreHook(
    ctx *schemas.BifrostContext, req *schemas.HTTPRequest,
) (*schemas.HTTPResponse, error) {
    
    // 快速路径: 明确的header或vision
    if quickResult := p.tryQuickPath(req); quickResult != nil {
        p.injectHeaders(ctx, req.Headers, quickResult)
        return nil, nil
    }
    
    // Embedding路径 (如果服务可用)
    if p.embeddingClient != nil {
        msgs := parseMessages(req.Body)
        userText := extractByRole(msgs, "user")
        
        if len(userText) > 20 { // 短文本不值得调用
            tier, reasoning, err := p.embeddingClient.classify(userText)
            if err == nil {
                p.logger.Info("Using embedding classification")
                p.injectHeaders(ctx, req.Headers, "text", tier, reasoning, ...)
                return nil, nil
            }
        }
    }
    
    // Fallback: 规则
    p.logger.Info("Falling back to rule-based classification")
    return p.classifyByRules(ctx, req)
}
```

---

## 📊 效果验证

### 对比测试

```bash
# 测试脚本
cat > test_classifier.sh <<'EOF'
#!/bin/bash

# 测试用例
declare -a TESTS=(
    "帮我写一个Rust函数"
    "请逐步推导这个公式"
    "你好吗"
    "这个important message很重要"  # 规则会误判为code
    "optimize my algorithm performance"
)

echo "=== Embedding Classifier Results ==="
for text in "${TESTS[@]}"; do
    echo -n "$text -> "
    curl -s -X POST http://localhost:8001/classify \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"$text\"}" | jq -r '.intent'
done
EOF

chmod +x test_classifier.sh
./test_classifier.sh
```

期望输出:
```
帮我写一个Rust函数 -> code
请逐步推导这个公式 -> think
你好吗 -> simple
这个important message很重要 -> simple  ✅ (规则会误判)
optimize my algorithm performance -> code
```

---

## ⚡ 性能优化

### 1. 添加缓存 (Redis)

```python
# app.py
import redis
import hashlib

redis_client = redis.Redis(host='localhost', port=6379, db=0)

@app.post("/classify")
def classify(req: Request):
    # 缓存key
    cache_key = f"classify:{hashlib.md5(req.text.encode()).hexdigest()}"
    
    # 检查缓存
    cached = redis_client.get(cache_key)
    if cached:
        return json.loads(cached)
    
    # 计算
    result = compute_classification(req.text)
    
    # 缓存1小时
    redis_client.setex(cache_key, 3600, json.dumps(result))
    
    return result
```

### 2. 批处理

```python
@app.post("/classify_batch")
def classify_batch(texts: list[str]):
    # 一次编码多个文本
    query_vecs = model.encode(texts)
    
    results = []
    for vec in query_vecs:
        # ... 计算相似度
        results.append(result)
    
    return results
```

Go端:
```go
// 收集100ms内的请求，批量处理
type batcher struct {
    requests chan *classifyRequest
}

func (b *batcher) classify(text string) <-chan *classifyResult {
    req := &classifyRequest{text: text, result: make(chan *classifyResult)}
    b.requests <- req
    return req.result
}

func (b *batcher) run() {
    batch := []*classifyRequest{}
    ticker := time.NewTicker(100 * time.Millisecond)
    
    for {
        select {
        case req := <-b.requests:
            batch = append(batch, req)
            if len(batch) >= 10 { // 满10个或100ms，发送
                b.processBatch(batch)
                batch = nil
            }
        case <-ticker.C:
            if len(batch) > 0 {
                b.processBatch(batch)
                batch = nil
            }
        }
    }
}
```

---

## 🐳 Docker部署

### Dockerfile (embedding_service/)

```dockerfile
FROM python:3.10-slim

WORKDIR /app

# 安装依赖
RUN pip install --no-cache-dir fastapi uvicorn sentence-transformers numpy

# 预下载模型 (减少首次启动时间)
RUN python -c "from sentence_transformers import SentenceTransformer; SentenceTransformer('paraphrase-multilingual-MiniLM-L12-v2')"

COPY app.py .

EXPOSE 8001

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8001"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  bifrost:
    build: .
    ports:
      - "8080:8080"
    environment:
      - EMBEDDING_SERVICE_URL=http://embedding:8001
    depends_on:
      - embedding

  embedding:
    build: ./embedding_service
    ports:
      - "8001:8001"
    deploy:
      resources:
        limits:
          memory: 2G
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8001/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

启动:
```bash
docker-compose up -d
```

---

## 📈 监控指标

### Prometheus指标

```python
from prometheus_client import Counter, Histogram, make_asgi_app

classify_counter = Counter('classify_total', 'Total classifications', ['intent'])
classify_latency = Histogram('classify_latency_seconds', 'Classification latency')

@app.post("/classify")
@classify_latency.time()
def classify(req: Request):
    result = compute_classification(req.text)
    classify_counter.labels(intent=result['intent']).inc()
    return result

# 添加metrics endpoint
metrics_app = make_asgi_app()
app.mount("/metrics", metrics_app)
```

Grafana dashboard:
- 分类分布饼图
- 延迟P50/P95/P99
- QPS趋势
- 缓存命中率

---

## 🔄 迁移路径

### Phase 1: 并行运行 (1周)
- Embedding和规则并行
- 记录两者的分类结果差异
- 不影响线上路由

### Phase 2: A/B测试 (2周)
- 10%流量使用embedding
- 对比准确率、成本、延迟
- 收集badcase

### Phase 3: 全量 (1个月)
- 逐步提升到50% → 90% → 100%
- 规则作为fallback保留

---

## 💡 进阶玩法

### 1. 微调模型

收集1000+个据:
```python
from sentence_transformers import losses, InputExample
from torch.utils.data import DataLoader

# 准备训练数据
train_examples = [
    InputExample(texts=['帮我写代码', '实现一个函数'], label=1.0),  # 相似
    InputExample(texts=['帮我写代码', '你好'], label=0.0),  # 不相似
    # ...
]

train_dataloader = DataLoader(train_examples, shuffle=True, batch_size=16)

# 定义loss
train_loss = losses.CosineSimilarityLoss(model)

# 微调
model.fit(
    train_objectives=[(train_dataloader, train_loss)],
    epochs=3,
    warmup_steps=100,
)

# 保存
model.save('models/bifrost-classifier-v1')
```

### 2. 多模态embedding

Vision + Text统一向量空间 (CLIP):
```python
from transformers import CLIPProcessor, CLIPModel

model = CLIPModel.from_pretrained("openai/clip-vit-base-patch32")
processor = CLIPProcessor.from_pretrained("openai/clip-vit-base-patch32")

# 图片+文本一起编码
inputs = processor(text=["a photo of a cat"], images=image, return_tensors="pt")
outputs = model(**inputs)

text_embeds = outputs.text_embeds
image_embeds = outputs.image_embeds
```

### 3. 用户画像embedding

结合用户历史:
```python
# 用户最近10个请求的平均embedding
user_history_vec = np.mean([model.encode(req) for req in last_10_requests], axis=0)

# 当前请求 + 用户画像
combined_vec = 0.7 * current_vec + 0.3 * user_history_vec

# 用combined_vec做分类
```

---

## 常见问题

### Q: 延迟太高怎么办?
A: 
1. 使用ONNX量化版本 (10ms → 3ms)
2. 批处理 (100ms窗口攒批)
3. GPU加速 (2ms)
4. 短文本(<50字)直接用规则

### Q: 模型太大怎么办?
A: 
- MiniLM: 420MB → bge-small: 95MB
- ONNX + INT8量化: 95MB → 25MB
- Distillation: 自己蒸馏更小的模型

##么办?
A:
1. 增加意图示例 (每个类别20+个)
2. 微调模型 (收集真实数据)
3. 置信度阈值调优

### Q: 多实例如何共享缓存?
A: Redis集中缓存 + 一致性哈希

---

## 下一步

1. ✅ 启动embedding服务
2. ✅ 测试分类效果
3. ⬜ 集成到Bifrost (混合模式)
4. ⬜ A/B测试对比
5. ⬜ 收集数据优化

需要我帮你搭建初始服务吗? 还是有其他问题?
