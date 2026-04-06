package server

import (
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// ClassifierPlugin classifies requests by multi-dimensional scoring
// to inject routing headers for governance rules.
type ClassifierPlugin struct {
	config any
	logger schemas.Logger
}

// chatMessage represents a single message in OpenAI-compatible format.
type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// chatRequest represents the top-level request body.
type chatRequest struct {
	Messages []chatMessage `json:"messages"`
}

func InitClassifier(config any, logger schemas.Logger) (schemas.BasePlugin, error) {
	if logger != nil {
		logger.Info("Classifier Plugin (In-package) Init")
	}
	return &ClassifierPlugin{config: config, logger: logger}, nil
}

func (p *ClassifierPlugin) GetName() string { return "classifier" }
func (p *ClassifierPlugin) Cleanup() error  { return nil }

func (p *ClassifierPlugin) HTTPTransportPostHook(
	ctx *schemas.BifrostContext, req *schemas.HTTPRequest, resp *schemas.HTTPResponse,
) error {
	return nil
}

func (p *ClassifierPlugin) HTTPTransportStreamChunkHook(
	ctx *schemas.BifrostContext, req *schemas.HTTPRequest, chunk *schemas.BifrostStreamChunk,
) (*schemas.BifrostStreamChunk, error) {
	return chunk, nil
}

func (p *ClassifierPlugin) HTTPTransportPreHook(
	ctx *schemas.BifrostContext, req *schemas.HTTPRequest,
) (*schemas.HTTPResponse, error) {
	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("ClassifierPlugin PREHOOK TRACE: body_len=%d", len(req.Body)))
	}
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}

	// Step 0: Explicit header override (highest priority)
	m := req.Headers["X-Route-Modality"]
	if m == "" { m = req.Headers["x-route-modality"] }
	if m == "" { m = req.Headers["x-modality"] }

	if m != "" {
		tier := req.Headers["X-Route-Tier"]
		if tier == "" { tier = req.Headers["x-route-tier"] }
		if tier == "" { tier = req.Headers["x-tier"] }
		tier = defaultStr(tier, "quality")

		reasoning := req.Headers["X-Route-Reasoning"]
		if reasoning == "" { reasoning = req.Headers["x-route-reasoning"] }
		if reasoning == "" { reasoning = req.Headers["x-reasoning"] }
		reasoning = defaultStr(reasoning, "fast")

		ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule,
			"Classifier: explicit header override")
		p.injectHeaders(ctx, req.Headers, m, tier, reasoning, "explicit_override", "en", "small", false, false)
		return nil, nil
	}

	// Step 1: Parse messages structure and get raw body for modality check
	rawBody := req.Body
	msgs := parseMessages(rawBody)
	systemText := extractByRole(msgs, "system")
	userText := extractByRole(msgs, "user")
	allText := systemText + "\n" + userText

	// Step 2: Phase 2 capability & context detection
	lang := detectLanguage(allText)
	ctxSize := estimateContextSize(allText)
	hasTool := detectToolCalling(rawBody)
	hasJSON := detectJSONOutput(rawBody)

	// Step 3: Modality detection from raw body
	bodyLower := strings.ToLower(string(rawBody))
	if hasVisionContent(bodyLower) {
		tier, reasoning := p.scoreTierAndReasoning(systemText, userText, len(msgs))
		// For vision, we default to quality tier as economy vision is rare/low-perf
		if tier == "economy" {
			tier = "quality"
		}
		ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule,
			fmt.Sprintf("Classifier: vision/%s/%s lang=%s ctx=%s", tier, reasoning, lang, ctxSize))
		p.injectHeaders(ctx, req.Headers, "vision", tier, reasoning, "multimodal", lang, ctxSize, hasTool, hasJSON)
		return nil, nil
	}

	// Step 4: Score text request across multiple dimensions
	tier, reasoning := p.scoreTierAndReasoning(systemText, userText, len(msgs))
	// Tool calling boosts tier to at least quality
	if hasTool && tier == "economy" {
		tier = "quality"
	}
	taskType := inferTaskType(tier, reasoning)
	ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule,
		fmt.Sprintf("Classifier: text/%s/%s (%s) lang=%s ctx=%s tools=%v json=%v",
			tier, reasoning, taskType, lang, ctxSize, hasTool, hasJSON))
	p.injectHeaders(ctx, req.Headers, "text", tier, reasoning, taskType, lang, ctxSize, hasTool, hasJSON)
	return nil, nil
}

func (p *ClassifierPlugin) injectHeaders(
	ctx *schemas.BifrostContext,
	headers map[string]string, modality, tier, reasoning, taskType,
	lang, ctxSize string, hasTool, hasJSON bool,
) {
	// 1. Inject into request headers (for downstream hooks)
	headers["x-modality"] = modality
	headers["x-tier"] = tier
	headers["x-reasoning"] = reasoning
	headers["x-task-type"] = taskType
	headers["x-language"] = lang
	headers["x-context-size"] = ctxSize
	if hasTool {
		headers["x-has-tools"] = "true"
	}
	if hasJSON {
		headers["x-has-json-output"] = "true"
	}

	// 2. Inject into context headers (CRITICAL for governance CEL engine)
	ctxHeaders, ok := ctx.Value(schemas.BifrostContextKeyRequestHeaders).(map[string]string)
	if !ok || ctxHeaders == nil {
		ctxHeaders = make(map[string]string)
		ctx.SetValue(schemas.BifrostContextKeyRequestHeaders, ctxHeaders)
	}

	p.logger.Info(fmt.Sprintf("ClassifierPlugin TRACE: Injecting context headers: x-modality=%s, x-tier=%s, x-reasoning=%s", modality, tier, reasoning))
	
	ctxHeaders["x-modality"] = modality
	ctxHeaders["x-tier"] = tier
	ctxHeaders["x-reasoning"] = reasoning
	ctxHeaders["x-task-type"] = taskType
	ctxHeaders["x-language"] = lang
	ctxHeaders["x-context-size"] = ctxSize
	if hasTool {
		ctxHeaders["x-has-tools"] = "true"
	}
	if hasJSON {
		ctxHeaders["x-has-json-output"] = "true"
	}
}

// defaultStr returns fallback if s is empty.
func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
