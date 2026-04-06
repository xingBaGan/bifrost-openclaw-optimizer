import os
# 使用国内镜像源加速 Hugging Face 模型下载
os.environ["HF_ENDPOINT"] = "https://hf-mirror.com"

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from semantic_router import Route
from semantic_router.routers import SemanticRouter
from semantic_router.encoders import HuggingFaceEncoder
from semantic_router.index import LocalIndex
from typing import List, Optional
import uvicorn
import logging
import time

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="Bifrost Semantic Router v3",
    description="Intent classification with casual fallback strategy",
    version="3.0.0"
)

# 全局变量
route_layer: Optional[SemanticRouter] = None

# 配置
CONFIG = {
    "confidence_threshold": 0.5,  # 低于此阈值视为casual
    "casual_metadata": {
        "tier": "economy",
        "reasoning": "fast",
        "task_type": "casual",
        "modality": "text"
    }
}

# 定义路由规则
# 注意: 只定义明确的专业类别，casual作为fallback处理
# code_simple: 直接实现类任务 - 有明确答案，直接编码即可 (reasoning: fast)
# code_complex: 架构/优化类任务 - 需要权衡设计，深度思考 (reasoning: think)
ROUTES = [
    Route(
        name="code_simple",
        utterances=[
            # 直接实现特征
            "write a function", "create a class", "implement a feature",
            "add a method", "write code for", "how to code",
            "implement an algorithm", "code example", "sample code",
            "fix this bug", "debug this", "syntax error",
            "how do I write", "show me code", "give me code",
            "implement data structure", "write a program",
            "build a binary tree", "implement a linked list", "create a hash map",
            "code a stack", "implement a queue", "write a tree traversal",
            # 中文 - 直接实现
            "写一个函数", "创建一个类", "实现一个功能",
            "添加一个方法", "写个代码", "怎么写代码",
            "实现一个算法", "代码示例", "示例代码",
            "写一个XX算法", "帮我写", "给我代码",
            "修复bug", "debug代码", "语法错误",
            "实现一个数据结构", "写一个程序",
            "实现二叉树", "实现链表", "创建哈希表",
            "写个栈", "实现队列", "树的遍历代码",
        ],
        metadata={"tier": "quality", "reasoning": "fast", "task_type": "code", "modality": "text"}
    ),
    Route(
        name="code_complex",
        utterances=[
            # 架构/优化特征
            "refactor architecture", "redesign system", "improve architecture",
            "refactor the module", "refactor authentication", "refactor this code",
            "optimize performance", "improve efficiency", "reduce latency",
            "design pattern", "code review", "best practices",
            "scalability", "maintainability", "trade-offs",
            "compare approaches", "which is better", "pros and cons",
            # 中文 - 架构/优化
            "重构架构", "重新设计", "改进架构",
            "重构模块", "重构认证", "重构代码",
            "优化性能", "提升效率", "降低延迟",
            "设计模式", "代码审查", "最佳实践",
            "可扩展性", "可维护性", "权衡分析",
            "对比方案", "哪个更好", "优缺点",
        ],
        metadata={"tier": "quality", "reasoning": "think", "task_type": "code_complex", "modality": "text"}
    ),
    Route(
        name="reasoning",
        utterances=[
            # 逐步分析类
            "step by step", "analyze step by step", "break down step by step",
            "walk me through", "explain step by step", "show me the steps",
            "detailed analysis", "in-depth analysis", "thorough analysis",
            "逐步分析", "一步一步分析", "分步骤说明",
            "详细分析", "深入分析", "逐步讲解",

            # 解释原因类
            "explain why", "why does", "what causes", "reason behind",
            "how does this work", "what's the reasoning", "explain the logic",
            "explain in detail", "elaborate on", "clarify the reason",
            "解释为什么", "为什么会", "什么原因导致", "原因是什么",
            "详细解释", "解释一下原理", "说明原因", "阐述原因",

            # 证明推导类
            "prove that", "demonstrate that", "show that", "verify that",
            "derive the formula", "derive this equation", "proof of",
            "mathematical proof", "formal proof", "prove mathematically",
            "证明", "推导公式", "推导过程", "证明过程",
            "数学证明", "形式化证明", "如何推导",

            # 逻辑推理类
            "reasoning process", "logical reasoning", "deduction", "inference",
            "thought process", "chain of reasoning", "line of reasoning",
            "reason about", "think through", "reason through",
            "推理过程", "逻辑推理", "演绎推理", "推断",
            "思维过程", "推理链", "逻辑链",

            # 复杂度分析类（修复测试失败的场景）
            "analyze complexity", "complexity analysis", "time complexity",
            "space complexity", "analyze the algorithm", "algorithm analysis",
            "analyze performance", "computational complexity",
            "分析复杂度", "复杂度分析", "时间复杂度分析",
            "空间复杂度", "算法分析", "性能分析",
            "计算复杂度", "分析算法的复杂度",

            # 比较分析类
            "compare and contrast", "analyze the difference", "comparoaches",
            "analyze trade-offs", "which approach is better and why",
            "对比分析", "比较分析", "分析差异", "权衡分析",

            # 根因分析类
            "root cause", "underlying reason", "fundamental reason",
            "analyze the root cause", "identify the cause", "find the reason",
            "根本原因", "根因分析", "深层原因", "找出原因",

            # 决策分析类
            "should I choose", "help me decide", "analyze options",
            "decision analysis", "evaluate alternatives",
            "帮我分析选择", "决策分析", "评估方案", "分析利弊",
        ],
        metadata={"tier": "quality", "reasoning": "think", "task_type": "reasoning", "modality": "text"}
    ),
    Route(
        name="research",
        utterances=[
            # 文献综述类
            "survey literature", "literature review", "state of art", "SOTA",
            "recent research", "latest research", "research trends",
            "survey the field", "review the literature",
            "文献综述", "文献回顾", "最新研究", "研究趋势",
            "前沿研究", "研究现状", "领域综述",

            # 学术类
            "academic paper", "research paper", "scholarly article",
            "scientific paper", "conference paper", "journal article",
            "学术论文", "研究论文", "会议论文", "期刊论文",

            # 研究方法类
            "methodology", "research methodology", "research method",
            "experimental design", "research design", "study design",
            "研究方法", "研究方法论", "实验设计", "研究设计",

            # 评审类
            "peer-reviewed", "peer review", "academic review",
            "scholarly review", "citation analysis",
            "同行评审", "学术评审", "引用分析",

            # 实证研究类
            "empirical research", "empirical study", "empirical analysis",
            "experimental results", "research findings", "study findings",
            "实证研究", "实验研究", "研究发现", "实验结果",

            # 前沿类
            "cutting-edge", "frontier research", "emerging research",
            "breakthrough research", "innovative research",
     "前沿研究", "突破性研究", "创新性研究", "新兴研究",
        ],
        metadata={"tier": "research", "reasoning": "think", "task_type": "research", "modality": "text"}
    ),
]


class ClassifyRequest(BaseModel):
    text: str


class ClassifyResponse(BaseModel):
    route_name: str
    tier: str
    reasoning: str
    task_type: str
    modality: str
    confidence: Optional[float] = None
    fallback_reason: Optional[str] = None


@app.on_event("startup")
async def initialize():
    """启动时初始化 Semantic Router"""
    global route_layer

    logger.info("🚀 Initializing Semantic Router v3...")
    start_time = time.time()

    try:
        # 使用多语言编码器
        encoder = HuggingFaceEncoder(
            name="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
        )

        # 显式创建 LocalIndex
        index = LocalIndex()

        # 创建路由层，启用 auto_sync 来自动填充 index
        route_layer = SemanticRouter(
            encoder=encoder,
            routes=ROUTES,
            index=index,
            auto_sync="local",  # 自动同步 routes 到 index
        )

        # 触发索引初始化：调用一次测试查询
        logger.info("⏳ Initializing index...")
        route_layer("initialization test")
        logger.info("✅ Index initialized!")

        elapsed = time.time() - start_time
        logger.info(f"✅ Semantic Router ready! ({elapsed:.2f}s)")
        logger.info(f"📋 Loaded {len(ROUTES)} routes")
        logger.info(f"🎯 Confidence threshold: {CONFIG['confidence_threshold']}")

    except Exception as e:
        logger.error(f"❌ Failed to initialize: {e}")
        raise


@app.get("/")
def root():
    return {
        "service": "Bifrost Semantic Router",
        "version": "3.0.0",
        "strategy": "casual_as_fallback",
        "routes": [r.name for r in ROUTES] + ["casual (fallback)"],
        "confidence_threshold": CONFIG['confidence_threshold'],
        "status": "ready" if route_layer else "loading",
    }


@app.get("/health")
def health():
    """健康检查，确保路由器已初始化"""
    return {
        "status": "ok" if route_layer is not None else "loading",
        "model_ready": route_layer is not None,
        "routes_count": len(ROUTES),
    }


@app.post("/classify", response_model=ClassifyResponse)
def classify(req: ClassifyRequest):
    if route_layer is None:
        raise HTTPException(status_code=503, detail="Router not initialized")

    if not req.text or len(req.text.strip()) == 0:
        raise HTTPException(status_code=400, detail="Text cannot be empty")

    try:
        start_time = time.time()
        choice = route_layer(req.text)
        latency = (time.time() - start_time) * 1000
        
        # 记录原始推理结果以便 Debug
        logger.info(f"🔍 Inference input: '{req.text[:30]}' -> Result: name={choice.name}, score={choice.similarity_score}")

        # 情况 1: 没匹配到任何已知路由
        if choice.name is None or choice.name == "":
            return ClassifyResponse(
                route_name="casual",
                **CONFIG['casual_metadata'],
                confidence=0.0,
                fallback_reason="no_route_matched"
            )

        # 情况 2: 匹配到了，但置信度低于阈值
        confidence = choice.similarity_score if choice.similarity_score is not None else 0.0
        
        if confidence < CONFIG['confidence_threshold']:
            logger.info(f"🔄 Lower than threshold ({confidence:.2f}) -> falling back to casual")
            return ClassifyResponse(
                route_name="casual",
                **CONFIG['casual_metadata'],
                confidence=confidence,
                fallback_reason=f"low_confidence_matched_{choice.name}"
            )

        # 情况 3: 匹配成功
        # 查找原始路由对应的 metadata (保持之前的逻辑)
        route_obj = next((r for r in ROUTES if r.name == choice.name), None)
        metadata = route_obj.metadata if route_obj else {}

        logger.info(f"✅ Matched: {choice.name} (score={confidence:.2f})")

        return ClassifyResponse(
            route_name=str(choice.name), # 确保是字符串
            tier=metadata.get("tier", "quality"),
            reasoning=metadata.get("reasoning", "fast"),
            task_type=metadata.get("task_type", "code"),
            modality=metadata.get("modality", "text"),
            confidence=float(confidence),
            fallback_reason=None,
        )

    except Exception as e:
        logger.error(f"❌ Error during classification: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


@app.post("/classify_batch")
def classify_batch(texts: List[str]):
    if route_layer is None:
        raise HTTPException(status_code=503, detail="Router not initialized")

    try:
        start_time = time.time()
        results = []
        for text in texts:
            choice = route_layer(text)
            
            if choice.name is None:
                results.append({
                    "text": text, "route_name": "casual", **CONFIG['casual_metadata'],
                    "confidence": 0.0, "fallback_reason": "no_route_matched"
                })
            else:
                confidence = choice.similarity_score if choice.similarity_score is not None else None
                if confidence is not None and confidence < CONFIG['confidence_threshold']:
                    results.append({
                        "text": text, "route_name": "casual", **CONFIG['casual_metadata'],
                        "confidence": confidence, "fallback_reason": f"low_confidence_matched_{choice.name}"
                    })
                else:
                    route_obj = next((r for r in ROUTES if r.name == choice.name), None)
                    metadata = route_obj.metadata if route_obj else {}
                    results.append({
                        "text": text, "route_name": choice.name,
                        "tier": metadata.get("tier", "quality"),
                        "reasoning": metadata.get("reasoning", "fast"),
                        "task_type": metadata.get("task_type", "code"),
                        "modality": metadata.get("modality", "text"),
                        "confidence": confidence, "fallback_reason": None,
                    })

        latency = (time.time() - start_time) * 1000
        return {"results": results, "count": len(results), "latency_ms": latency}

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@app.get("/routes")
def list_routes():
    return {
        "routes": [r.name for r in ROUTES] + ["casual"],
        "confidence_threshold": CONFIG['confidence_threshold']
    }


@app.put("/config/threshold")
def update_threshold(threshold: float):
    """动态调整置信度阈值"""
    if not 0.0 <= threshold <= 1.0:
        raise HTTPException(status_code=400, detail="Threshold must be between 0 and 1")

    CONFIG['confidence_threshold'] = threshold
    logger.info(f"🔧 Threshold updated to {threshold}")

    return {
        "success": True,
        "new_threshold": threshold,
        "message": "Threshold updated successfully"
    }


def start():
    uvicorn.run("embedding_service.main:app", host="0.0.0.0", port=8001, reload=True)


if __name__ == "__main__":
    start()
