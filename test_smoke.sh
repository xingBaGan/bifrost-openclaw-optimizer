#!/bin/bash

# Quick Smoke Test for Embedding Service Integration
# Tests basic functionality without running full integration test

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "Embedding Service Integration Smoke Test"
echo "=========================================="
echo ""

# Test 1: Check if embedding service responds
echo -e "${YELLOW}1. Testing Embedding Service Health...${NC}"
if curl -sf http://localhost:8001/health > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Embedding service is healthy${NC}"
else
    echo -e "${RED}❌ Embedding service not responding${NC}"
    echo "Try: cd embedding_service && uv run server"
    exit 1
fi

# Test 2: Check if embedding service can classify
echo -e "${YELLOW}2. Testing Embedding Classification...${NC}"
result=$(curl -sf -X POST http://localhost:8001/classify \
    -H "Content-Type: application/json" \
    -d '{"text": "写一个排序算法"}' 2>/dev/null)

if [ $? -eq 0 ]; then
    route=$(echo "$result" | jq -r '.route_name // "error"')
    tier=$(echo "$result" | jq -r '.tier // "unknown"')
    conf=$(echo "$result" | jq -r '.confidence // 0')
    echo -e "${GREEN}✅ Classification works: route=$route, tier=$tier, confidence=$conf${NC}"
else
    echo -e "${RED}❌ Classification failed${NC}"
    exit 1
fi

# Test 3: Check if docker-compose config is valid
echo -e "${YELLOW}3. Validating Docker Compose Config...${NC}"
if docker-compose config > /dev/null 2>&1; then
    echo -e "${GREEN}✅ docker-compose.yml is valid${NC}"
else
    echo -e "${RED}❌ docker-compose.yml has errors${NC}"
    exit 1
fi

# Test 4: Check if config.json has embedding service configured
echo -e "${YELLOW}4. Checking Bifrost Configuration...${NC}"
if grep -q '"embedding_service"' config.json && grep -q '"enabled": true' config.json; then
    echo -e "${GREEN}✅ config.json has embedding service enabled${NC}"
else
    echo -e "${RED}❌ config.json missing embedding service config${NC}"
    exit 1
fi

# Test 5: Check if key files exist
echo -e "${YELLOW}5. Checking Integration Files...${NC}"
files=(
    "plugins/classifier/embedding_client.go"
    "plugins/classifier/config.go"
    "embedding_service/Dockerfile"
    "test_integration.sh"
)

all_exist=true
for file in "${files[@]}"; do
    if [ -f "$file" ]; then
        echo -e "  ${GREEN}✓${NC} $file"
    else
        echo -e "  ${RED}✗${NC} $file (missing)"
        all_exist=false
    fi
done

if [ "$all_exist" = false ]; then
    echo -e "${RED}❌ Some integration files are missing${NC}"
    exit 1
fi

echo ""
echo "=========================================="
echo -e "${GREEN}✅ All Smoke Tests Passed!${NC}"
echo "=========================================="
echo ""
echo "Integration is ready. Next steps:"
echo "  1. Build: docker-compose build"
echo "  2. Start: docker-compose up -d"
echo "  3. Test:  ./test_integration.sh"
echo ""
