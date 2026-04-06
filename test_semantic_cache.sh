#!/bin/bash

# Semantic Cache 测试脚本
# 测试 Bifrost 的语义缓存功能

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo "=========================================="
echo "Bifrost Semantic Cache 功能测试"
echo "=========================================="
echo ""

# 检查服务状态
echo -e "${YELLOW}检查服务状态...${NC}"
if ! curl -sf http://localhost:6379 > /dev/null 2>&1; then
    if docker exec bifrost-redis redis-cli ping > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Redis 服务正常${NC}"
    else
        echo -e "${RED}❌ Redis 服务未运行${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✅ Redis 服务正常${NC}"
fi

if ! curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${RED}❌ Bifrost 服务未运行${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Bifrost 服务正常${NC}"
echo ""

# 测试 1: 第一次请求（无缓存）
echo "=========================================="
echo -e "${BLUE}Test 1: 第一次请求（无缓存）${NC}"
echo "=========================================="
echo "请求: '写一个快速排序算法'"
echo "预期: ~2-5 秒（调用 LLM）"
echo ""

START=$(date +%s%N)
RESPONSE1=$(curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-bf-cache-key: test-session-quicksort" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "写一个快速排序算法"}]
  }')
END=$(date +%s%N)
ELAPSED1=$((($END - $START) / 1000000))

echo "$RESPONSE1" | jq -r '.choices[0].message.content' | head -10
echo ""
echo -e "${GREEN}耗时: ${ELAPSED1}ms${NC}"
echo ""

# 等待一下确保缓存已存储
sleep 1

# 测试 2: 相同请求（应该命中精确缓存）
echo "=========================================="
echo -e "${BLUE}Test 2: 相同请求（精确缓存）${NC}"
echo "=========================================="
echo "请求: '写一个快速排序算法'"
echo "预期: ~50-100ms（从缓存返回）"
echo ""

START=$(date +%s%N)
RESPONSE2=$(curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-bf-cache-key: test-session-quicksort" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "写一个快速排序算法"}]
  }')
END=$(date +%s%N)
ELAPSED2=$((($END - $START) / 1000000))

echo "$RESPONSE2" | jq -r '.choices[0].message.content' | head -10
echo ""
echo -e "${GREEN}耗时: ${ELAPSED2}ms${NC}"

# 检查是否命中缓存
if [ $ELAPSED2 -lt 1000 ]; then
    echo -e "${GREEN}✅ 缓存命中！延迟降低 $(( ($ELAPSED1 - $ELAPSED2) * 100 / $ELAPSED1 ))%${NC}"
else
    echo -e "${YELLOW}⚠️  可能未命中缓存，延迟仍然较高${NC}"
fi
echo ""

# 测试 3: 语义相似请求（应该命中语义缓存）
echo "=========================================="
echo -e "${BLUE}Test 3: 语义相似请求（语义缓存）${NC}"
echo "=========================================="
echo "请求: '实现快排代码'（语义相似）"
echo "预期: ~100-200ms（语义相似匹配）"
echo ""

START=$(date +%s%N)
RESPONSE3=$(curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-bf-cache-key: test-session-quicksort" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "实现快排代码"}]
  }')
END=$(date +%s%N)
ELAPSED3=$((($END - $START) / 1000000))

echo "$RESPONSE3" | jq -r '.choices[0].message.content' | head -10
echo ""
echo -e "${GREEN}耗时: ${ELAPSED3}ms${NC}"

if [ $ELAPSED3 -lt 1000 ]; then
    echo -e "${GREEN}✅ 语义缓存命中！延迟降低 $(( ($ELAPSED1 - $ELAPSED3) * 100 / $ELAPSED1 ))%${NC}"
else
    echo -e "${YELLOW}⚠️  可能未命中语义缓存${NC}"
fi
echo ""

# 测试 4: 完全不同的请求（不应该命中缓存）
echo "=========================================="
echo -e "${BLUE}Test 4: 完全不同请求（无缓存）${NC}"
echo "=========================================="
echo "请求: '今天天气怎么样'（完全不同）"
echo "预期: ~2-5 秒（重新调用 LLM）"
echo ""

START=$(date +%s%N)
RESPONSE4=$(curl -s -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "x-bf-cache-key: test-session-weather" \
  -d '{
    "model": "auto",
    "messages": [{"role": "user", "content": "今天天气怎么样"}]
  }')
END=$(date +%s%N)
ELAPSED4=$((($END - $START) / 1000000))

echo "$RESPONSE4" | jq -r '.choices[0].message.content' | head -10
echo ""
echo -e "${GREEN}耗时: ${ELAPSED4}ms${NC}"

if [ $ELAPSED4 -gt 1000 ]; then
    echo -e "${GREEN}✅ 正确未命中缓存，调用了 LLM${NC}"
else
    echo -e "${YELLOW}⚠️  延迟意外地短，可能配置有问题${NC}"
fi
echo ""

# 查看 Redis 缓存统计
echo "=========================================="
echo -e "${BLUE}Redis 缓存统计${NC}"
echo "=========================================="
CACHE_SIZE=$(docker exec bifrost-redis redis-cli DBSIZE | grep -oE '[0-9]+')
CACHE_KEYS=$(docker exec bifrost-redis redis-cli KEYS "bifrost:cache:*" | wc -l)

echo "缓存数据库大小: $CACHE_SIZE 条目"
echo "Bifrost 缓存 key 数量: $CACHE_KEYS"
echo ""

# 查看部分缓存 keys
echo "缓存 key 示例:"
docker exec bifrost-redis redis-cli KEYS "bifrost:cache:*" | head -5
echo ""

# 总结
echo "=========================================="
echo -e "${GREEN}测试总结${NC}"
echo "=========================================="
echo ""
printf "%-30s %10s %15s\n" "测试场景" "耗时" "缓存状态"
echo "----------------------------------------------------"
printf "%-30s %10dms %15s\n" "第一次请求（无缓存）" $ELAPSED1 "Miss"
printf "%-30s %10dms %15s\n" "相同请求（精确缓存）" $ELAPSED2 "Hit (exact)"
printf "%-30s %10dms %15s\n" "相似请求（语义缓存）" $ELAPSED3 "Hit (semantic)"
printf "%-30s %10dms %15s\n" "不同请求（无缓存）" $ELAPSED4 "Miss"
echo ""

# 计算缓存效果
SPEEDUP2=$(echo "scale=1; $ELAPSED1 / $ELAPSED2" | bc)
SPEEDUP3=$(echo "scale=1; $ELAPSED1 / $ELAPSED3" | bc)

echo -e "${GREEN}缓存加速效果:${NC}"
echo "  精确缓存: ${SPEEDUP2}x 加速"
echo "  语义缓存: ${SPEEDUP3}x 加速"
echo ""

if (( $(echo "$SPEEDUP2 > 10" | bc -l) )); then
    echo -e "${GREEN}🎉 缓存功能工作正常！${NC}"
else
    echo -e "${YELLOW}⚠️  缓存效果不明显，请检查配置${NC}"
fi
echo ""
