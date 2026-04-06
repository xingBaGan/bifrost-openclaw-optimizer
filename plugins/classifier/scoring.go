package server

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Keyword groups for multi-dimensional classification.
// Density thresholds prevent single-keyword false positives.
var (
	codeKeywords = []string{
		"```", "func ", "def ", "class ", "import ", "return ",
		"const ", "var ", "package ", "module ", "interface ",
	}
	reasonKeywords = []string{
		"step by step", "analyze", "explain why", "reason about",
		"逐步", "深入分析", "论证", "推理", "思考",
	}
	researchKeywords = []string{
		"research", "academic", "论文", "survey", "expert",
		"comprehensive", "systematic", "peer-reviewed",
	}
	codingSystemKeywords = []string{
		"code", "programming", "debug", "developer", "engineer",
		"编程", "代码", "software", "refactor",
	}
)

// parseMessages extracts messages from request body.
func parseMessages(body any) []chatMessage {
	var raw []byte
	switch v := body.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		raw, _ = json.Marshal(v)
	}

	var req chatRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil
	}
	return req.Messages
}

// extractByRole concatenates all message content for a given role.
func extractByRole(msgs []chatMessage, role string) string {
	var parts []string
	for _, m := range msgs {
		if m.Role != role {
			continue
		}
		switch v := m.Content.(type) {
		case string:
			parts = append(parts, v)
		default:
			b, _ := json.Marshal(v)
			parts = append(parts, string(b))
		}
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

// hasVisionContent checks for image data indicators in raw body.
func hasVisionContent(body string) bool {
	return strings.Contains(body, "image_url") ||
		strings.Contains(body, "\"type\":\"image")
}

// countMatches returns number of keywords found in text.
func countMatches(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			count++
		}
	}
	return count
}

// scoreTierAndReasoning calculates tier and reasoning via weighted scoring.
// tier: economy(0) / quality(1-2) / research(3+)
// reasoning: fast(0) / think(1+)
func (p *ClassifierPlugin) scoreTierAndReasoning(systemText, userText string, msgCount int) (string, string) {
	tierScore := 0
	reasonScore := 0

	// System prompt signals (high weight: domain expertise indicators)
	tierScore += countMatches(systemText, researchKeywords) * 2
	tierScore += countMatches(systemText, codingSystemKeywords)
	reasonScore += countMatches(systemText, reasonKeywords)

	// User message: code density (threshold=3 for strong signal)
	codeHits := countMatches(userText, codeKeywords)
	if codeHits >= 3 {
		tierScore += 2
	} else if codeHits >= 1 {
		tierScore += 1
	}

	// User message: reasoning density (threshold=2)
	reasonHits := countMatches(userText, reasonKeywords)
	if reasonHits >= 2 {
		tierScore++
		reasonScore++
	}

	// Complexity heuristics
	if len(userText) > 3000 {
		tierScore++
	}
	if msgCount > 4 {
		tierScore++ // multi-turn conversation
	}

	// Map scores to categories
	tier := "economy"
	if tierScore >= 5 {
		tier = "research"
	} else if tierScore >= 1 {
		tier = "quality"
	}

	reasoning := "fast"
	if reasonScore >= 1 {
		reasoning = "think"
	}
	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("ClassifierPlugin SCORING TRACE: tierScore=%d, reasonScore=%d, content_len=%d", tierScore, reasonScore, len(userText)))
	}
	return tier, reasoning
}

// inferTaskType generates a descriptive task type label.
func inferTaskType(tier, reasoning string) string {
	switch {
	case tier == "research":
		return "deep_analysis"
	case tier == "quality" && reasoning == "think":
		return "complex_task"
	case tier == "quality":
		return "skilled_task"
	default:
		return "quick_query"
	}
}

// --- Phase 2: Capability & Context Detection ---

// extendedRequest captures tool calling and structured output fields.
type extendedRequest struct {
	Tools          any `json:"tools"`
	Functions      any `json:"functions"`
	ResponseFormat any `json:"response_format"`
}

// detectToolCalling checks if the request expects tool/function calling.
func detectToolCalling(body any) bool {
	var bodyStr string
	switch v := body.(type) {
	case []byte:
		bodyStr = string(v)
	case string:
		bodyStr = v
	default:
		raw, _ := json.Marshal(v)
		bodyStr = string(raw)
	}
	return strings.Contains(bodyStr, "\"tools\"") ||
		strings.Contains(bodyStr, "\"functions\"")
}

// detectJSONOutput checks if the request expects structured JSON output.
func detectJSONOutput(body any) bool {
	var bodyStr string
	switch v := body.(type) {
	case []byte:
		bodyStr = string(v)
	case string:
		bodyStr = v
	default:
		raw, _ := json.Marshal(v)
		bodyStr = string(raw)
	}
	return strings.Contains(bodyStr, "\"json_object\"") ||
		strings.Contains(bodyStr, "\"json_schema\"")
}

// detectLanguage returns "zh" if CJK rune ratio > 30%, otherwise "en".
func detectLanguage(text string) string {
	if len(text) == 0 {
		return "en"
	}
	cjkCount, total := 0, 0
	for _, r := range text {
		total++
		if unicode.Is(unicode.Han, r) {
			cjkCount++
		}
	}
	if total > 0 && float64(cjkCount)/float64(total) > 0.3 {
		return "zh"
	}
	return "en"
}

// estimateContextSize returns "small"/"medium"/"large" based on token estimation.
// English: ~4 chars/token, CJK: ~1.5 tokens/char.
func estimateContextSize(text string) string {
	cjkCount, nonCJK := 0, 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			cjkCount++
		} else {
			nonCJK++
		}
	}
	tokens := nonCJK/4 + cjkCount*3/2
	if tokens > 32000 {
		return "large"
	}
	if tokens > 4000 {
		return "medium"
	}
	return "small"
}
