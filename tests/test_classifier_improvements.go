package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Copy of matchKeyword function from scoring.go
func matchKeyword(text, keyword string) bool {
	if strings.Contains(keyword, " ") {
		return strings.Contains(text, keyword)
	}
	hasAlphaNum := false
	for _, r := range keyword {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			hasAlphaNum = true
			break
		}
	}
	if !hasAlphaNum {
		return strings.Contains(text, keyword)
	}
	trimmed := strings.TrimSpace(keyword)
	pattern := `\b` + regexp.QuoteMeta(trimmed) + `\b`
	matched, _ := regexp.MatchString(pattern, text)
	return matched
}

func countMatches(text string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if matchKeyword(text, kw) {
			count++
		}
	}
	return count
}

func estimateContextSize(text string) string {
	cjkCount, nonCJK := 0, 0
	codeChars := 0

	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			cjkCount++
		} else {
			nonCJK++
			if r == '{' || r == '}' || r == '[' || r == ']' ||
				r == '(' || r == ')' || r == '<' || r == '>' ||
				r == ';' || r == ':' || r == '=' || r == '|' {
				codeChars++
			}
		}
	}

	tokens := 0
	codeRatio := float64(codeChars) / float64(nonCJK+1)
	if codeRatio > 0.15 || strings.Contains(text, "```") {
		tokens = nonCJK*10/35 + cjkCount*2
	} else {
		tokens = nonCJK/4 + cjkCount*2
	}

	if tokens > 32000 {
		return "large"
	}
	if tokens > 4000 {
		return "medium"
	}
	return "small"
}

type TestCase struct {
	Name     string
	Text     string
	Keyword  string
	Expected bool
}

type TokenTestCase struct {
	Name     string
	Text     string
	Expected string
}

func main() {
	fmt.Println("=== Testing Keyword Matching Improvements ===\n")

	// Test 1: Word boundary matching
	matchTests := []TestCase{
		{
			Name:     "import should NOT match 'important'",
			Text:     "this is important information",
			Keyword:  "import ",
			Expected: false,
		},
		{
			Name:     "import should match 'import os'",
			Text:     "import os",
			Keyword:  "import ",
			Expected: true,
		},
		{
			Name:     "class should NOT match 'classroom'",
			Text:     "welcome to the classroom",
			Keyword:  "class ",
			Expected: false,
		},
		{
			Name:     "class should match 'class MyClass'",
			Text:     "class MyClass:",
			Keyword:  "class ",
			Expected: true,
		},
		{
			Name:     "Multi-word phrase 'step by step'",
			Text:     "explain step by step how this works",
			Keyword:  "step by step",
			Expected: true,
		},
		{
			Name:     "Symbol matching ' => '",
			Text:     "const fn = (x) => x * 2",
			Keyword:  " => ",
			Expected: true,
		},
		{
			Name:     "Code block marker '```'",
			Text:     "here is code:\n```python\nprint('hello')\n```",
			Keyword:  "```",
			Expected: true,
		},
	}

	passed := 0
	for _, tc := range matchTests {
		result := matchKeyword(tc.Text, tc.Keyword)
		status := "✓ PASS"
		if result != tc.Expected {
			status = "✗ FAIL"
		} else {
			passed++
		}
		fmt.Printf("%s: %s\n", status, tc.Name)
		fmt.Printf("   Text: %q, Keyword: %q\n", tc.Text, tc.Keyword)
		fmt.Printf("   Expected: %v, Got: %v\n\n", tc.Expected, result)
	}

	fmt.Printf("Keyword Matching: %d/%d tests passed\n\n", passed, len(matchTests))

	// Test 2: Token estimation
	fmt.Println("=== Testing Token Estimation ===\n")

	tokenTests := []TokenTestCase{
		{
			Name:     "Short plain text",
			Text:     "Hello, how are you?",
			Expected: "small",
		},
		{
			Name:     "Medium plain text",
			Text:     strings.Repeat("Hello world. ", 500), // ~6500 chars
			Expected: "medium",
		},
		{
			Name:     "Code with high symbol density",
			Text:     strings.Repeat("func main() { fmt.Println([]string{\"a\", \"b\"}) }\n", 100),
			Expected: "medium",
		},
		{
			Name:     "Code block marker triggers code path",
			Text:     "```python\n" + strings.Repeat("print('hello')\n", 500) + "```",
			Expected: "medium",
		},
		{
			Name:     "Chinese text (high token density)",
			Text:     strings.Repeat("这是一段中文文本。", 500), // ~5000 chars CJK
			Expected: "large", // CJK: 2 tokens/char => ~10k tokens
		},
	}

	tokenPassed := 0
	for _, tc := range tokenTests {
		result := estimateContextSize(tc.Text)
		status := "✓ PASS"
		if result != tc.Expected {
			status = "✗ FAIL"
		} else {
			tokenPassed++
		}
		fmt.Printf("%s: %s\n", status, tc.Name)
		fmt.Printf("   Text length: %d chars\n", len(tc.Text))
		fmt.Printf("   Expected: %s, Got: %s\n\n", tc.Expected, result)
	}

	fmt.Printf("Token Estimation: %d/%d tests passed\n\n", tokenPassed, len(tokenTests))

	// Test 3: Expanded keyword coverage
	fmt.Println("=== Testing Expanded Keyword Coverage ===\n")

	codeKeywords := []string{
		"```", "func ", "def ", "class ", "async ", "await ",
		" => ", " -> ", "pub fn", "impl ", "trait ",
	}

	codeTexts := map[string]string{
		"Python":     "def calculate(x): return x * 2",
		"JavaScript": "const double = (x) => x * 2",
		"Go":         "func Double(x int) int { return x * 2 }",
		"Rust":       "pub fn double(x: i32) -> i32 { x * 2 }",
		"TypeScript": "async function fetchData() { await fetch(url) }",
	}

	for lang, text := range codeTexts {
		count := countMatches(strings.ToLower(text), codeKeywords)
		fmt.Printf("✓ %s: matched %d code keywords\n", lang, count)
	}

	fmt.Println("\n=== All Tests Complete ===")

	// Summary JSON output
	summary := map[string]interface{}{
		"keyword_matching_passed": passed,
		"keyword_matching_total":  len(matchTests),
		"token_estimation_passed": tokenPassed,
		"token_estimation_total":  len(tokenTests),
	}
	jsonOut, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Printf("\nSummary:\n%s\n", jsonOut)
}
