#!/bin/bash

# Semantic Router Performance Benchmark
# 测试吞吐量、延迟、并发性能

BASE_URL="${BASE_URL:-http://localhost:8001}"
REQUESTS="${REQUESTS:-100}"
CONCURRENCY="${CONCURRENCY:-10}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo "=========================================="
echo "Semantic Router Benchmark"
echo "=========================================="
echo ""

# 检查服务
echo -e "${YELLOW}Checking service...${NC}"
health=$(curl -s "$BASE_URL/health")
status=$(echo "$health" | jq -r '.status')

if [ "$status" != "ok" ]; then
    echo -e "${RED}❌ Service not ready${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Service ready${NC}"
echo "$health" | jq
echo ""

# 准备测试数据
declare -a TEST_QUERIES=(
    '{"text": "写一个快排算法"}'
    '{"text": "优化这个数据库查询性能"}'
    '{"text": "逐步分析这个算法的时间复杂度"}'
    '{"text": "今天天气怎么样"}'
    '{"text": "implement a binary search tree"}'
    '{"text": "refactor this authentication system"}'
    '{"text": "explain step by step"}'
    '{"text": "推荐个餐厅"}'
    '{"text": "帮我写一个递归函数"}'
    '{"text": "重构这个模块的架构"}'
)

# 1. 单请求延迟测试
echo -e "${YELLOW}1. Single Request Latency Test${NC}"
echo "Testing ${#TEST_QUERIES[@]} different queries..."
echo ""

total_time=0
success_count=0
error_count=0

for query in "${TEST_QUERIES[@]}"; do
    start=$(date +%s%N)
    response=$(curl -s -X POST "$BASE_URL/classify" \
        -H "Content-Type: application/json" \
        -d "$query")
    end=$(date +%s%N)

    elapsed=$((($end - $start) / 1000000))  # Convert to ms
    total_time=$(($total_time + $elapsed))

    route=$(echo "$response" | jq -r '.route_name // "error"')
    conf=$(echo "$response" | jq -r '.confidence // 0')

    if [ "$route" != "error" ]; then
        success_count=$(($success_count + 1))
        printf "  ${GREEN}✓${NC} %-50s → %-15s %4d ms (conf=%.2f)\n" \
            "$(echo $query | jq -r .text | cut -c1-40)" "$route" "$elapsed" "$conf"
    else
        error_count=$(($error_count + 1))
        printf "  ${RED}✗${NC} %-50s → ERROR %4d ms\n" \
            "$(echo $query | jq -r .text | cut -c1-40)" "$elapsed"
    fi
done

avg_latency=$(($total_time / ${#TEST_QUERIES[@]}))

echo ""
echo -e "${BLUE}Results:${NC}"
echo "  Success: $success_count"
echo "  Errors:  $error_count"
echo "  Average Latency: ${avg_latency} ms"
echo ""

# 2. 吞吐量测试 (Sequential)
echo -e "${YELLOW}2. Sequential Throughput Test${NC}"
echo "Sending $REQUESTS requests sequentially..."
echo ""

start_time=$(date +%s%N)

for i in $(seq 1 $REQUESTS); do
    query_idx=$(($i % ${#TEST_QUERIES[@]}))
    query="${TEST_QUERIES[$query_idx]}"

    curl -s -X POST "$BASE_URL/classify" \
        -H "Content-Type: application/json" \
        -d "$query" > /dev/null

    if [ $(($i % 20)) -eq 0 ]; then
        printf "."
    fi
done

end_time=$(date +%s%N)
elapsed_sec=$(( ($end_time - $start_time) / 1000000000 ))
elapsed_ms=$(( ($end_time - $start_time) / 1000000 ))

throughput=$(echo "scale=2; $REQUESTS / $elapsed_sec" | bc)
avg_latency=$(echo "scale=2; $elapsed_ms / $REQUESTS" | bc)

echo ""
echo ""
echo -e "${BLUE}Results:${NC}"
echo "  Total Requests: $REQUESTS"
echo "  Total Time:     ${elapsed_sec}s (${elapsed_ms}ms)"
echo "  Throughput:     ${throughput} req/s"
echo "  Avg Latency:    ${avg_latency} ms/req"
echo ""

# 3. 并发测试 (使用 xargs)
echo -e "${YELLOW}3. Concurrent Requests Test${NC}"
echo "Sending $REQUESTS requests with concurrency=$CONCURRENCY..."
echo ""

# 创建临时文件存放请求
temp_file=$(mktemp)
for i in $(seq 1 $REQUESTS); do
    query_idx=$(($i % ${#TEST_QUERIES[@]}))
    echo "${TEST_QUERIES[$query_idx]}" >> "$temp_file"
done

start_time=$(date +%s%N)

# 使用 xargs 并发执行
cat "$temp_file" | xargs -I {} -P $CONCURRENCY curl -s -X POST "$BASE_URL/classify" \
    -H "Content-Type: application/json" \
    -d {} > /dev/null

end_time=$(date +%s%N)
elapsed_sec=$(( ($end_time - $start_time) / 1000000000 ))
elapsed_ms=$(( ($end_time - $start_time) / 1000000 ))

throughput=$(echo "scale=2; $REQUESTS / $elapsed_sec" | bc)
avg_latency=$(echo "scale=2; $elapsed_ms / $REQUESTS" | bc)

rm "$temp_file"

echo ""
echo -e "${BLUE}Results:${NC}"
echo "  Total Requests: $REQUESTS"
echo "  Concurrency:    $CONCURRENCY"
echo "  Total Time:     ${elapsed_sec}s (${elapsed_ms}ms)"
echo "  Throughput:     ${throughput} req/s"
echo "  Avg Latency:    ${avg_latency} ms/req"
echo ""

# 4. 批量接口测试
echo -e "${YELLOW}4. Batch API Test${NC}"
echo "Testing /classify_batch with 20 queries..."
echo ""

batch_queries=$(printf '%s\n' "${TEST_QUERIES[@]}" | jq -s 'map(.text)')

start_time=$(date +%s%N)

batch_response=$(curl -s -X POST "$BASE_URL/classify_batch" \
    -H "Content-Type: application/json" \
    -d "$batch_queries")

end_time=$(date +%s%N)
elapsed_ms=$(( ($end_time - $start_time) / 1000000 ))

count=$(echo "$batch_response" | jq -r '.count')
server_latency=$(echo "$batch_response" | jq -r '.latency_ms')

echo -e "${BLUE}Results:${NC}"
echo "  Queries:        $count"
echo "  Total Latency:  ${elapsed_ms} ms (client-measured)"
echo "  Server Latency: ${server_latency} ms"
echo "  Avg per query:  $(echo "scale=2; $server_latency / $count" | bc) ms"
echo ""

# 5. 分类准确性统计
echo -e "${YELLOW}5. Classification Distribution${NC}"
echo ""

routes=$(echo "$batch_response" | jq -r '.results[] | .route_name' | sort | uniq -c)
echo "$routes"
echo ""

# 总结
echo "=========================================="
echo -e "${GREEN}✅ Benchmark Complete!${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  • Single request latency: ~${avg_latency} ms"
echo "  • Sequential throughput:  ${throughput} req/s"
echo "  • Batch API efficient for multiple queries"
echo ""
echo "💡 Tips:"
echo "  • Use batch API for multiple classifications"
echo "  • First request includes model loading time"
echo "  • Subsequent requests benefit from warm cache"
echo ""
