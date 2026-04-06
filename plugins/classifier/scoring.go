package server

import (
	"encoding/json"
	"strings"
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
	raw, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	var req chatRequest
	json.Unmarshal(raw, &req)
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
func scoreTierAndReasoning(systemText, userText string, msgCount int) (string, string) {
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
	if tierScore >= 3 {
		tier = "research"
	} else if tierScore >= 1 {
		tier = "quality"
	}

	reasoning := "fast"
	if reasonScore >= 1 {
		reasoning = "think"
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
