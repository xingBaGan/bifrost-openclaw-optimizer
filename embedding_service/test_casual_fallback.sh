#!/bin/bash

# Casual Fallback 策略测试脚本
# 验证 v3 版本的 casual fallback 是否工作正常

BASE_URL="${BASE_URL:-http://localhost:8001}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo "=========================================="
echo "Casual Fallback Strategy Test (v3)"
echo "=========================================="
echo ""

# 检查服务
echo -e "${YELLOW}0. Checking Service...${NC}"
response=$(curl -s "$BASE_URL/")
version=$(echo "$response" | jq -r '.version')
strategy=$(echo "$response" | jq -r '.strategy')

if [ "$version" = "3.0.0" ]; then
    echo -e "${GREEN}✅ v3.0.0 running${NC}"
    echo -e "${GREEN}✅ Strategy: $strategy${NC}"
    echo "$response" | jq '{service, version, strategy, routes, confidence_threshold}'
else
    echo -e "${RED}❌ Not running v3 (found: $version)${NC}"
    exit 1
fi
echo ""

# 查看路由配置
echo -e "${YELLOW}1. Routes Configuration...${NC}"
curl -s "$BASE_URL/routes" | jq '.routes[] | {name, utterances_count, note}'
echo ""

# 测试专业类别（应该匹配）
echo -e "${YELLOW}2. Professional Categories (should match)...${NC}"
echo ""

declare -aONAL=(
    "写一个Python快排算法|code_simple"
    "refactor this distributed system|code_complex"
    "请逐步分析时间复杂度|reasoning"
    "综述最新的Transformer研究|research"
)

for test in "${PROFESSIONAL[@]}"; do
    IFS='|' read -r text expected <<< "$test"
    printf "%-45s " "$text"

    result=$(curl -s -X POST "$BASE_URL/classify" \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"$text\"}")

    route=$(echo "$result" | jq -r '.route_name')
    confidence=$(echo "$result" | jq -r '.confidence // 0')
    fallback=$(echo "$result" | jq -r '.fallback_reason // "none"')

    if [ "$route" = "$expected" ]; then
        printf "${GREEN}→ %-15s${NC} conf=%.2f %s\n" "$route" "$confidence" "✅"
    else
        printf "${RED}→ %-15s${NC} (expected $expected) ❌\n" "$route"
    fi
done

echo ""

# 测试 Casual 场景（应该 fallback）
echo -e "${YELLOW}3. Casual Scenarios (should fallback to casual)...${NC}"
echo ""

declare -a CASUAL=(
    "今天天气怎么样"
    "推荐个好吃的餐厅"
    "讲个笑话给我听"
    "明天股市会涨吗"
    "你觉得哪个球队会赢"
    "帮我算一下1+1等于几"
    "what's the weather like today"
    "recommend a restaurant"
    "tell me a joke"
)

for text in "${CASUAL[@]}"; do
    printf "%-45s " "$text"

    result=$(curl -s -X POST "$BASE_URL/classify" \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"$text\"}")

    route=$(echo "$result" | jq -r '.route_name')
    confidence=$(echo "$result" | jq -r '.confidence // 0')
    fallback=$(echo "$result" | jq -r '.fallback_reason // "none"')

    if [ "$route" = "casual" ]; then
        printf "${GREEN}→ casual${NC} (fallback: $fallback) ✅\n"
    else
        printf "${RED}→ $route${NC} (expected casual) ❌\n"
    fi
done

echo ""

# 测试边界case
echo -e "${YELLOW}4. Edge Cases...${NC}"
echo ""

declare -a EDGE=(
    "important message|casual"  # "important" 不再误判为 code
    "classroom management|casual"  # "classroom" 不再误判为 code
    "decode this text|casual"  # "decode" 如果不是代码上下文应该是casual
)

for test in "${EDGE[@]}"; do
    IFS='|' read -r text expected <<< "$test"
    printf "%-45s " "$text"

    result=$(curl -s -X POST "$BASE_URL/classify" \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"$text\"}")

    route=$(echo "$result" | jq -r '.route_name')
    confidence=$(echo "$result" | jq -r '.confidence // 0')
    fallback=$(echo "$result" | jq -r '.fallback_reason // "none"')

    if [ "$route" = "$expected" ]; then
        printf "${GREEN}→ $route${NC} ✅\n"
    else
        printf "${YELLOW}→ $route${NC} (expected $expected, conf=%.2f) ⚠️\n" "$confidence"
    fi
done

echo ""

# 批量测试
echo -e "${YELLOW}5. Batch Test...${NC}"

batch_result=$(curl -s -X POST "$BASE_URL/classify_batch" \
    -H "Content-Type: application/json" \
    -d '["写代码", "今天天气", "逐步分析", "推荐餐厅", "研究论文"]')

echo "$batch_result" | jq -r '.results[] | "\(.text) → \(.route_name) (fallback: \(.fallback_reason // "none"))"'

echo ""

# 统计
total_casual=$(echo "$batch_result" | jq '[.results[] | select(.route_name == "casual")] | length')
echo -e "${BLUE}Casual count: $total_casual / 5${NC}"

echo ""

# 测试阈值调整
echo -e "${YELLOW}6. Testing Threshold Adjustment...${NC}"

# 获取当前阈值
current=$(curl -s "$BASE_URL/" | jq -r '.confidence_threshold')
echo "Current threshold: $current"

# 调整阈值到 0.7 (更严格)
echo "Adjusting to 0.7..."
curl -s -X PUT "$BASE_URL/config/threshold?threshold=0.7" | jq '{success, new_threshold}'

# 重新测试一个边界case
echo ""
echo "Retesting with threshold=0.7:"
result=$(curl -s -X POST "$BASE_URL/classify" \
    -H "Content-Type: application/json" \
    -d '{"text": "帮我优化代码性能"}')

echo "$result" | jq '{route_name, confidence, fallback_reason}'

# 恢复阈值
echo ""
echo "Restoring threshold to 0.5..."
curl -s -X PUT "$BASE_URL/config/threshold?threshold=0.5" | jq '{success, new_threshold}'

echo ""
echo "=========================================="
echo -e "${GREEN}✅ All Tests Complete!${NC}"
echo "=========================================="
echo ""
echo "Key Improvements in v3:"
echo "  1. ✅ Casual不需要定义utterances"
echo "  2. ✅ 所有未匹配或低置信度→casual"
echo "  3. ✅ 支持动态调整阈值"
echo "  4. ✅ fallback_reason 说明为什么是casual"
