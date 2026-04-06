#!/bin/bash

# 快速验证 v3 部署是否成功

echo "🚀 验证 v3 部署..."
echo ""

# 1. 检查服务是否运行
echo "1️⃣ 检查服务状态..."
health_resp=$(curl -s http://localhost:8001/health)
if [ $? -eq 0 ]; then
    status=$(echo "$health_resp" | jq -r '.status')
    if [ "$status" = "ok" ]; then
        echo "   ✅ 服务已就绪 (模型已加载)"
    else
        echo "   ⏳ 服务正在启动中 (模型加载中: $status)..."
        echo "   请稍等几秒后再运行此脚本。"
        exit 1
    fi
else
    echo "   ❌ 服务未运行"
    echo "   请运行: uv run server"
    exit 1
fi

# 2. 检查版本
echo ""
echo "2️⃣ 检查版本..."
version=$(curl -s http://localhost:8001/ | jq -r '.version')
strategy=$(curl -s http://localhost:8001/ | jq -r '.strategy')

if [ "$version" = "3.0.0" ]; then
    echo "   ✅ 版本: v3.0.0"
    echo "   ✅ 策略: $strategy"
else
    echo "   ⚠️  版本: $version (期望 3.0.0)"
fi

# 3. 测试专业类别
echo ""
echo "3️⃣ 测试专业分类..."
result=$(curl -s -X POST http://localhost:8001/classify \
    -H "Content-Type: application/json" \
    -d '{"text": "写一个快排算法"}')

route=$(echo "$result" | jq -r '.route_name')
if [ "$route" = "code_simple" ]; then
    echo "   ✅ 代码类别: $route"
else
    echo "   ❌ 代码类别错误: $route"
fi

# 4. 测试 casual fallback
echo ""
echo "4️⃣ 测试 Casual Fallback..."
result=$(curl -s -X POST http://localhost:8001/classify \
    -H "Content-Type: application/json" \
    -d '{"text": "今天天气怎么样"}')

route=$(echo "$result" | jq -r '.route_name')
fallback=$(echo "$result" | jq -r '.fallback_reason')

if [ "$route" = "casual" ]; then
    echo "   ✅ Casual fallback: $route (原因: $fallback)"
else
    echo "   ❌ Casual fallback 失败: $route"
fi

# 5. 测试动态阈值
echo ""
echo "5️⃣ 测试动态阈值..."
result=$(curl -s -X PUT "http://localhost:8001/config/threshold?threshold=0.6")
success=$(echo "$result" | jq -r '.success')

if [ "$success" = "true" ]; then
    echo "   ✅ 阈值调整成功"
    # 恢复默认
    curl -s -X PUT "http://localhost:8001/config/threshold?threshold=0.5" > /dev/null
else
    echo "   ❌ 阈值调整失败"
fi

# 总结
echo ""
echo "=========================================="
if [ "$version" = "3.0.0" ] && [ "$route" = "casual" ]; then
    echo "✅ v3 部署成功！"
    echo ""
    echo "📝 下一步:"
    echo "   1. 运行完整测试: ./test_casual_fallback.sh"
    echo "   2. 查看 API 文档: open http://localhost:8001/docs"
    echo "   3. 开始集成到 Bifrost"
else
    echo "⚠️  部署可能有问题，请检查日志"
fi
echo "=========================================="
