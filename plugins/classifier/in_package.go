package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// ClassifierPlugin classifies requests by multi-dimensional scoring
// to inject routing headers for governance rules.
type ClassifierPlugin struct {
	config any
	logger *schemas.Logger
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
	logger.Info("Classifier Plugin (In-package) Init")
	return &ClassifierPlugin{config: config, logger: &logger}, nil
}

func (p *ClassifierPlugin) GetName() string { return "classifier" }
func (p *ClassifierPlugin) Cleanup() error  { return nil }

func (p *ClassifierPlugin) HTTPTransportPostHook(
	ctx *schemas.BifrostContext, req *schemas.HTTPRequest, resp *schemas.HTTPResponse,
) error {
	return nil
}

func (p *ClassifierPlugin) HTTPTransportPreHook(
	ctx *schemas.BifrostContext, req *schemas.HTTPRequest,
) (*schemas.HTTPResponse, error) {
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}

	// Step 0: Explicit header override (highest priority)
	if m := req.Headers["x-route-modality"]; m != "" {
		tier := defaultStr(req.Headers["x-route-tier"], "quality")
		reasoning := defaultStr(req.Headers["x-route-reasoning"], "fast")
		ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule,
			"Classifier: explicit header override")
		p.injectHeaders(req.Headers, m, tier, reasoning, "explicit_override")
		return nil, nil
	}

	// Step 1: Parse messages structure
	msgs := parseMessages(req.Body)
	systemText := extractByRole(msgs, "system")
	userText := extractByRole(msgs, "user")

	// Step 2: Modality detection from raw body
	rawBody, _ := json.Marshal(req.Body)
	bodyLower := strings.ToLower(string(rawBody))
	if hasVisionContent(bodyLower) {
		tier, reasoning := scoreTierAndReasoning(systemText, userText, len(msgs))
		ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule,
			fmt.Sprintf("Classifier: vision/%s/%s", tier, reasoning))
		p.injectHeaders(req.Headers, "vision", tier, reasoning, "multimodal")
		return nil, nil
	}

	// Step 3: Score text request across multiple dimensions
	tier, reasoning := scoreTierAndReasoning(systemText, userText, len(msgs))
	taskType := inferTaskType(tier, reasoning)
	ctx.AppendRoutingEngineLog(schemas.RoutingEngineRoutingRule,
		fmt.Sprintf("Classifier: text/%s/%s (%s)", tier, reasoning, taskType))
	p.injectHeaders(req.Headers, "text", tier, reasoning, taskType)
	return nil, nil
}

func (p *ClassifierPlugin) injectHeaders(
	headers map[string]string, modality, tier, reasoning, taskType string,
) {
	headers["x-modality"] = modality
	headers["x-tier"] = tier
	headers["x-reasoning"] = reasoning
	headers["x-task-type"] = taskType
}

// defaultStr returns fallback if s is empty.
func defaultStr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
