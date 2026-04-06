package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// Classifier constants mapping to xheader.json enums
const (
	XModalityText   = "text"
	XModalityVision = "vision"

	XTierEconomy  = "economy"
	XTierQuality  = "quality"
	XTierResearch = "research"

	XReasoningFast  = "fast"
	XReasoningThink = "think"

	XTaskTypeQuickChat   = "quick_query"
	XTaskTypeDeepReason  = "reasoning_complex"
	XTaskTypeVision      = "multimodal_analyze"
	XTaskTypeHeavyCoding = "heavy_coding"
)

// Init is called when the plugin is loaded
func Init(config any) error {
	return nil
}

// GetName returns the plugin's unique identifier
func GetName() string {
	return "SmartClassifier"
}

// HTTPTransportPreHook intercepts requests BEFORE they enter Bifrost core.
// It parses the request payload and injects routing hints into headers.
func HTTPTransportPreHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest) (*schemas.HTTPResponse, error) {
	// Initialize headers if nil to avoid panic
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}

	// Parse the request body to extract keywords.
	// req.Body is a map[string]any representing the parsed JSON payload.
	bodyStr := ""
	if b, err := json.Marshal(req.Body); err == nil {
		bodyStr = strings.ToLower(string(b))
	}

	// 1. Detect Vision tasks (searching for image triggers)
	if strings.Contains(bodyStr, "image_url") || strings.Contains(bodyStr, "base64") {
		ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule, "SmartClassifier: Classified as vision task")
		injectHeaders(req.Headers, XModalityVision, XTierQuality, XReasoningFast, XTaskTypeVision)
		return nil, nil
	}

	// 2. Detect Heavy Coding (looking for code structure patterns)
	codeKeywords := []string{"```", "func ", "def ", "class ", "import ", "function"}
	for _, kw := range codeKeywords {
		if strings.Contains(bodyStr, kw) {
			ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule, fmt.Sprintf("SmartClassifier: Classified as heavy coding task (keyword: %s)", kw))
			injectHeaders(req.Headers, XModalityText, XTierQuality, XReasoningFast, XTaskTypeHeavyCoding)
			return nil, nil
		}
	}

	// 3. Detect Deep Reasoning (based on keywords or intent)
	reasonKeywords := []string{"think", "reason", "analyze", "explain", "complex", "step by step"}
	isLongRequest := len(bodyStr) > 2000
	hasReasonKeywords := false
	for _, kw := range reasonKeywords {
		if strings.Contains(bodyStr, kw) {
			hasReasonKeywords = true
			break
		}
	}

	if hasReasonKeywords || isLongRequest {
		ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule, "SmartClassifier: Classified as deep reasoning task")
		injectHeaders(req.Headers, XModalityText, XTierResearch, XReasoningThink, XTaskTypeDeepReason)
		return nil, nil
	}

	// 4. Default: Simple Chat (economy tier)
	ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule, "SmartClassifier: Classified as simple chat")
	injectHeaders(req.Headers, XModalityText, XTierEconomy, XReasoningFast, XTaskTypeQuickChat)
	return nil, nil
}

// injectHeaders helper to set multiple routing-related headers.
func injectHeaders(headers map[string]string, modality, tier, reasoning, taskType string) {
	headers["x-modality"] = modality
	headers["x-tier"] = tier
	headers["x-reasoning"] = reasoning
	headers["x-task-type"] = taskType
}

// HTTPTransportPostHook intercepts responses AFTER they exit Bifrost core.
func HTTPTransportPostHook(ctx *schemas.BifrostContext, req *schemas.HTTPRequest, resp *schemas.HTTPResponse) error {
	return nil
}

// PreLLMHook is called before the request is sent to the provider.
func PreLLMHook(ctx *schemas.BifrostContext, req *schemas.BifrostRequest) (*schemas.BifrostRequest, *schemas.LLMPluginShortCircuit, error) {
	return req, nil, nil
}

// PostLLMHook is called after receiving a response from the provider.
func PostLLMHook(ctx *schemas.BifrostContext, resp *schemas.BifrostResponse, bifrostErr *schemas.BifrostError) (*schemas.BifrostResponse, *schemas.BifrostError, error) {
	return resp, bifrostErr, nil
}

// Cleanup is called when Bifrost shuts down.
func Cleanup() error { return nil }
