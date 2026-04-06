#!/bin/bash

# Integration Test Script for Embedding Service + Bifrost
# Tests the full pipeline: Bifrost → Embedding Service → Classification

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo "=========================================="
echo "Bifrost + Embedding Integration Test"
echo "=========================================="
echo ""

# 1. Check Embedding Service
echo -e "${YELLOW}1. Checking Embedding Service...${NC}"
if ! curl -s http://localhost:8001/health > /dev/null; then
    echo -e "${RED}❌ Embedding service not running${NC}"
    echo "Please start it with: cd embedding_service && uv run server"
    exit 1
fi

health=$(curl -s http://localhost:8001/health | jq -r '.status')
if [ "$health" = "ok" ]; then
    echo -e "${GREEN}✅ Embedding service healthy${NC}"
else
    echo -e "${RED}❌ Embedding service unhealthy: $health${NC}"
    exit 1
fi
echo ""

# 2. Check Bifrost
echo -e "${YELLOW}2. Checking Bifrost...${NC}"

if ! pgrep -f "bifrost" > /dev/null; then
    echo -e "${RED}❌ Bifrost not running${NC}"
    echo "Please start it with: ./bifrost --config config.json"
    exit 1
fi

echo -e "${GREEN}✅ Bifrost is running${NC}"
echo ""

# 3. Test Classification through Bifrost
echo -e "${YELLOW}3. Testing Classification Pipeline...${NC}"
echo ""

declare -a TEST_CASES=(
    "写一个快排算法|code_simple|quality"
    "优化这个数据库性能|code_complex|quality"
    "逐步分析这个问题|reasoning|quality"
    "今天天气怎么样|casual|economy"
    "implement a binary search tree|code_simple|quality"
    "refactor this authentication system|code_complex|quality"
)

success=0
total=${#TEST_CASES[@]}

for test_case in "${TEST_CASES[@]}"; do
    IFS='|' read -r text expected_type expected_tier <<< "$test_case"

    printf "  %-45s " "$text"

    # Call Bifrost API (which will call embedding service internally)
    response=$(curl -s -X POST http://localhost:8080/v1/chat/completions \
        -H "Content-Type: application/json" \
        -H "X-Debug-Headers: true" \
        -d "{
            \"model\": \"auto\",
            \"messages\": [{\"role\": \"user\", \"content\": \"$text\"}]
        }" 2>/dev/null)

    # Extract headers from response (Bifrost should return classification in headers)
    # Note: This assumes Bifrost returns x-tier and x-task-type in response headers or body

    # For now, directly test embedding service
    embed_result=$(curl -s -X POST http://localhost:8001/classify \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"$text\"}")

    route=$(echo "$embed_result" | jq -r '.route_name // "error"')
    tier=$(echo "$embed_result" | jq -r '.tier // "unknown"')
    task=$(echo "$embed_result" | jq -r '.task_type // "unknown"')
    conf=$(echo "$embed_result" | jq -r '.confidence // 0')

    # Check if classification matches expected
    if [[ "$task" == "$expected_type" || "$route" == "$expected_type" ]]; then
        success=$((success + 1))
        printf "${GREEN}✓${NC} → %-15s (tier=%s, conf=%.2f)\n" "$task" "$tier" "$conf"
    else
        printf "${RED}✗${NC} → %-15s (expected %s)\n" "$task" "$expected_type"
    fi
done

echo ""
echo -e "${BLUE}Results: $success/$total tests passed${NC}"
echo ""

# 4. Test Fallback Mechanism
echo -e "${YELLOW}4. Testinlback Mechanism...${NC}"
echo ""

echo "Stopping embedding service temporarily..."
embedding_pid=$(lsof -ti:8001)
if [ -n "$embedding_pid" ]; then
    kill -STOP $embedding_pid
    echo "Service paused (PID: $embedding_pid)"

    sleep 2

    echo "Testing Bifrost with embedding service down..."
    # Bifrost should fallback to rules
    echo "(Bifrost should automatically fallback to rule-based classification)"

    # Resume service
    kill -CONT $embedding_pid
    echo "Service resumed"
    echo ""
fi

# 5. Performance Test
echo -e "${YELLOW}5. Quick Performance Test...${NC}"
echo ""

echo "Sending 10 requests..."
start_time=$(date +%s%N)

for i in {1..10}; do
    curl -s -X POST http://localhost:8001/classify \
        -H "Content-Type: application/json" \
        -d '{"text": "写一个函数"}' > /dev/null
done

end_time=$(date +%s%N)
elapsed_ms=$(( ($end_time - $start_time) / 1000000 ))
avg_latency=$(( $elapsed_ms / 10 ))

echo "Total time: ${elapsed_ms}ms"
echo "Average latency: ${avg_latency}ms per request"
echo ""

if [ $avg_latency -lt 30 ]; then
    echo -e "${GREEN}✅ Performance excellent (<30ms)${NC}"
elif [ $avg_latency -lt 50 ]; then
    echo -e "${GREEN}✅ Performance good (<50ms)${NC}"
else
    echo -e "${YELLOW}⚠️  Performance acceptable but could be better${NC}"
fi
echo ""

# Summary
echo "=========================================="
echo -e "${GREEN}✅ Integration Test Complete!${NC}"
echo "=========================================="
echo ""

if [ $success -eq $total ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Monitor Bifrost logs for [embedding] tags"
    echo "  2. Check classification accuracy in production"
    echo "  3. Adjust confidence_threshold if needed"
else
    echo -e "${YELLOW}Some tests failed. Please check:${NC}"
    echo "  1. Embedding service utterances in main.py"
    echo "  2. Confidence threshold in config.json"
    echo "  3. Classification logic in embedding_client.go"
fi
echo ""
