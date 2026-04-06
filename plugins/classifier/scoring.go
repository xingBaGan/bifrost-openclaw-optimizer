package server

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Keyword groups for multi-dimensional classification.
// Density thresholds prevent single-keyword false positives.
var (
	codeKeywords = []string{
		// Code block markers
		"```", "```python", "```javascript", "```go", "```rust", "```java",
		"```typescript", "```cpp", "```c++",
		// Function/class definitions (multi-language)
		"func ", "def ", "class ", "function ", "fn ", "impl ", "struct ",
		"interface ", "enum ", "trait ", "type ",
		// Import/package management
		"import ", "require(", "use ", "package ", "module ", "from ", "export ",
		// Control flow
		"return ", "yield", "await ", "async ", "for ", "while ", "if ", "else ",
		// Variable declarations
		"const ", "var ", "let ", "mut ", "pub ",
		// Common symbols and patterns
		" => ", " -> ", "::", "pub fn", "async fn", "fn main",
		// OOP patterns
		"extends ", "implements ", "override ", "constructor",
		// Error handling
		"try ", "catch ", "throw ", "panic", "unwrap", "Result<",
		// Comments and docs// ", "/* ", "* @", "/// ", "#[", "TODO:", "FIXME:",
	}
	reasonKeywords = []string{
		// English
		"step by step", "analyze", "explain why", "reason about",
		"think through", "break down", "consider", "evaluate",
		"derive", "prove", "demonstrate", "justify",
		"reasoning", "logical", "inference", "deduce",
		// Chinese
		"逐步", "深入分析", "论证", "推理", "思考",
		"分析原因", "解释为什么", "推导", "证明",
		"详细说明", "仔细考虑", "评估", "判断",
	}
	researchKeywords = []string{
		// English
		"research", "academic", "survey", "expert",
		"comprehensive", "systematic", "peer-reviewed",
		"literature review", "state-of-the-art", "SOTA",
		"benchmark", "empirical", "methodology",
		"hypothesis", "experiment", "publication",
		"journal", "conference", "proceedings",
		// Chinese
		"论文", "学术", "研究", "综述",
		"实证", "方法论", "假设", "实验",
		"文献综述", "最新进展", "前沿",
	}
	codingSystemKeywords = []string{
		// English
		"code", "programming", "debug", "developer", "engineer",
		"software", "refactor", "optimize", "implement",
		"algorithm", "data structure", "API", "backend", "frontend",
		"database", "architecture", "design pattern",
		"unit test", "integration", "deployment",
		"repository", "version control", "git",
		// Chinese
		"编程", "代码", "调试", "开发", "工程师",
		"软件", "重构", "优化", "实现",
		"算法", "数据结构", "后端", "前端",
		"数据库", "架构", "设计模式",
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
// Uses word boundary matching for alphanumeric keywords to avoid false positives.
func countMatches(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if matchKeyword(text, kw) {
			count++
		}
	}
	return count
}

// matchKeyword checks if a keyword exists in text with smart matching.
// For symbols and special patterns, uses direct string matching.
// For words, uses word boundary matching to avoid false positives.
func matchKeyword(text, keyword string) bool {
	// Special case: multi-word phrases (contains space)
	if strings.Contains(keyword, " ") {
		return strings.Contains(text, keyword)
	}

	// Check if keyword is purely symbolic (no alphanumeric chars)
	hasAlphaNum := false
	for _, r := range keyword {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAlphaNum = true
			break
		}
	}

	// For symbols/operators, use direct matching
	if !hasAlphaNum {
		return strings.Contains(text, keyword)
	}

	// For alphanumeric keywords, use word boundary matching
	// to avoid "import" matching "important"
	trimmed := strings.TrimSpace(keyword)

	// Build regex with word boundaries
	// \b works for ASCII, but for Unicode we need custom logic
	pattern := `\b` + regexp.QuoteMeta(trimmed) + `\b`
	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		// Fallback to simple contains if regex fails
		return strings.Contains(text, keyword)
	}
	return matched
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
// Uses conservative coefficients to account for tokenizer differences across models.
// Code and structured content have higher token density than plain text.
func estimateContextSize(text string) string {
	cjkCount, nonCJK := 0, 0
	codeChars := 0 // Count of code-like characters (brackets, operators, etc.)

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			cjkCount++
		} else {
			nonCJK++
			// Detect code-like patterns
			if r == '{' || r == '}' || r == '[' || r == ']' ||
				r == '(' || r == ')' || r == '<' || r == '>' ||
				r == ';' || r == ':' || r == '=' || r == '|' {
				codeChars++
			}
		}
	}

	// Base token estimation:
	// - English plain text: ~4 chars/token
	// - CJK characters: ~2 tokens/char (conservative, some models ~1.5)
	// - Code content: ~3.5 chars/token (higher density due to symbols)
	tokens := 0

	// If text has high code density, use code coefficient
	codeRatio := float64(codeChars) / float64(nonCJK+1) // +1 to avoid division by zero
	if codeRatio > 0.15 || strings.Contains(text, "```") {
		// Code-heavy content: use more conservative estimate
		tokens = nonCJK*10/35 + cjkCount*2 // 3.5 chars/token for code, 2 tokens/char for CJK
	} else {
		// Plain text: standard estimate
		tokens = nonCJK/4 + cjkCount*2 // 4 chars/token for English, 2 tokens/char for CJK
	}

	// Thresholds based on common model context windows:
	// - small: < 4K tokens (fits in all models)
	// - medium: 4K-32K tokens (needs models like GPT-4, Claude)
	// - large: > 32K tokens (needs specialized models like Kimi, Claude with extended context)
	if tokens > 32000 {
		return "large"
	}
	if tokens > 4000 {
		return "medium"
	}
	return "small"
}
