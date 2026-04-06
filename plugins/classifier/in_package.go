package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// ClassifierPlugin classifies requests by multi-dimensional scoring
// to inject routing headers for governance rules.
type ClassifierPlugin struct {
	config          *ClassifierConfig
	logger          schemas.Logger
	embeddingClient *EmbeddingClient
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

	// Parse config
	var pluginConfig ClassifierConfig
	if config != nil {
		configBytes, err := json.Marshal(config)
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
		if err := json.Unmarshal(configBytes, &pluginConfig); err != nil {
			return nil, fmt.Errorf("unmarshal config: %w", err)
		}
	}

	plugin := &ClassifierPlugin{
		config: &pluginConfig,
		logger: logger,
	}

	// Initialize embedding client if configured
	if pluginConfig.EmbeddingService != nil && pluginConfig.EmbeddingService.Enabled {
		timeout := time.Duration(pluginConfig.EmbeddingService.TimeoutMs) * time.Millisecond
		plugin.embeddingClient = NewEmbeddingClient(pluginConfig.EmbeddingService.URL, timeout)

		// Health check
		if err := plugin.embeddingClient.HealthCheck(); err != nil {
			logger.Warn(fmt.Sprintf("Embedding service unhealthy, will fallback to rules: %v", err))
		} else {
			logger.Info(fmt.Sprintf("Embedding service ready at %s", pluginConfig.EmbeddingService.URL))
		}
	}

	return plugin, nil
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

	// Step 4: Try embedding classification first (if enabled)
	tier, reasoning, taskType, embeddingUsed := p.tryEmbeddingClassify(userText)

	// If embedding failed or not configured, fall back to rule-based classification
	if !embeddingUsed {
		tier, reasoning = p.scoreTierAndReasoning(systemText, userText, len(msgs))
		// Tool calling boosts tier to at least quality
		if hasTool && tier == "economy" {
			tier = "quality"
		}
		taskType = inferTaskType(tier, reasoning)
	}

	logMsg := fmt.Sprintf("Classifier: text/%s/%s (%s) lang=%s ctx=%s tools=%v json=%v",
		tier, reasoning, taskType, lang, ctxSize, hasTool, hasJSON)
	if embeddingUsed {
		logMsg += " [embedding]"
	} else {
		logMsg += " [rules]"
	}
	ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule, logMsg)

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

// tryEmbeddingClassify attempts to classify using the embedding service.
// Returns (tier, reasoning, taskType, used) where used indicates if embedding was actually used.
func (p *ClassifierPlugin) tryEmbeddingClassify(text string) (tier, reasoning, taskType string, used bool) {
	// Check if embedding service is configured and enabled
	if p.embeddingClient == nil {
		return "", "", "", false
	}

	if p.config == nil || p.config.EmbeddingService == nil || !p.config.EmbeddingService.Enabled {
		return "", "", "", false
	}

	// Call embedding service
	result, err := p.embeddingClient.Classify(text)
	if err != nil {
		if p.logger != nil {
			p.logger.Warn(fmt.Sprintf("Embedding classification failed: %v", err))
		}
// Fallback to rules if configured
		if p.config.EmbeddingService.FallbackToRules {
			return "", "", "", false
		}
		// Otherwise return economy as safe default
		return "economy", "fast", "casual", true
	}

	// Check confidence threshold
	threshold := p.config.EmbeddingService.ConfidenceThreshold
	if threshold == 0 {
		threshold = 0.5 // Default threshold
	}

	// If confidence too low and fallback enabled, return false to use rules
	if result.Confidence < threshold && p.config.EmbeddingService.FallbackToRules {
		if p.logger != nil {
			p.logger.Info(fmt.Sprintf("Embedding confidence %.2f below threshold %.2f, falling back to rules",
				result.Confidence, threshold))
		}
		return "", "", "", false
	}

	// Use embedding result
	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("Embedding classified as %s/%s/%s (conf=%.2f)",
			result.TaskType, result.Tier, result.Reasoning, result.Confidence))
	}

	return result.Tier, result.Reasoning, result.TaskType, true
}

// defaultStr returns fallback if s is empty.
func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
