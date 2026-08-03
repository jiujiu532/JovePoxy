package effort

import "strings"

// BudgetToLevel maps Anthropic budget_tokens to OpenAI reasoning_effort levels.
// Aligns with CLIProxy convert thresholds.
//
//	-1        → auto
//	0         → none
//	1–512     → minimal
//	513–1024  → low
//	1025–8192 → medium
//	8193–24576 → high
//	≥24577    → xhigh
//
// Budgets < -1 return empty string (caller should omit the field).
func BudgetToLevel(budget int) string {
	switch {
	case budget < -1:
		return ""
	case budget == -1:
		return "auto"
	case budget == 0:
		return "none"
	case budget <= 512:
		return "minimal"
	case budget <= 1024:
		return "low"
	case budget <= 8192:
		return "medium"
	case budget <= 24576:
		return "high"
	default:
		return "xhigh"
	}
}

// NormalizeLevel trims and lowercases an effort string. Empty input stays empty.
func NormalizeLevel(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// MapForZen normalizes client effort labels for Zen OpenAI-compatible upstream.
// Empty / auto → "" (caller chooses fallback). max → xhigh (Zen has no "max").
func MapForZen(s string) string {
	level := NormalizeLevel(s)
	switch level {
	case "", "auto":
		return ""
	case "off":
		return "none"
	case "max":
		return "xhigh"
	default:
		return level
	}
}
