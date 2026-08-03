package effort

import (
	"strings"
)

// BudgetToLevel maps Anthropic budget_tokens to OpenAI reasoning_effort levels.
// Aligns with CLIProxy convert thresholds. Call MapForModel afterward so the
// level is clamped to what the target model actually accepts.
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

// Profile describes the effort labels a model family accepts upstream.
// Levels is ordered from weakest non-none to strongest (used for clamp).
type Profile struct {
	// Levels accepted non-none labels, weakest → strongest.
	Levels []string
	// AllowNone keeps "none" / "off" as "none". When false, none/off omit the field.
	AllowNone bool
	// AllowAuto keeps "auto". When false, auto is omitted (never sent).
	AllowAuto bool
}

// Canonical order used for nearest-neighbor clamping across families.
var strengthOrder = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

func strengthIndex(level string) int {
	for i, v := range strengthOrder {
		if v == level {
			return i
		}
	}
	return -1
}

// ProfileForModel returns the effort profile for a catalog model ID.
// Safe for admin/UI exposure; same rules used by MapForModel.
func ProfileForModel(model string) Profile {
	return profileForModel(model)
}

// LevelsForDisplay returns ordered labels the model accepts, including none/auto
// when allowed. Used by admin model list so operators see the real clamp set.
func LevelsForDisplay(model string) []string {
	p := profileForModel(model)
	out := make([]string, 0, len(p.Levels)+2)
	if p.AllowNone {
		out = append(out, "none")
	}
	out = append(out, p.Levels...)
	if p.AllowAuto {
		out = append(out, "auto")
	}
	return out
}

// profileForModel returns the effort profile for a catalog model ID.
// Sources (priority): CLIProxy models.json thinking.levels, live Zen/Go/Ollama
// probes, vendor docs (OpenAI reasoning_effort, xAI grok, Moonshot Kimi,
// Ollama thinking). Unknown families fall back to the conservative Zen set.
func profileForModel(model string) Profile {
	id := strings.ToLower(strings.TrimSpace(model))
	// Strip Ollama tag suffix (name:tag) for family matching only.
	base := id
	if i := strings.IndexByte(base, ':'); i >= 0 {
		base = base[:i]
	}

	switch {
	// OpenAI gpt-oss on Ollama Pro: none|low|medium|high|max (no minimal/xhigh/auto).
	// Aligns with Ollama thinking models + live probes (xhigh→max).
	case strings.Contains(base, "gpt-oss") || strings.HasPrefix(base, "gpt-oss"):
		return Profile{Levels: []string{"low", "medium", "high", "max"}, AllowNone: true}

	// GPT-5.6 family (luna/terra/sol): low|medium|high|xhigh|max (CLIProxy codex-*).
	// No none — off/none is omitted rather than sent.
	case strings.HasPrefix(base, "gpt-5.6"):
		return Profile{Levels: []string{"low", "medium", "high", "xhigh", "max"}, AllowNone: false}

	// Other Codex GPT-5.x (5.3/5.4/5.5): low|medium|high|xhigh — no max in CLIProxy.
	case strings.HasPrefix(base, "gpt-5.5") || strings.HasPrefix(base, "gpt-5.4") ||
		strings.HasPrefix(base, "gpt-5.3") || strings.HasPrefix(base, "gpt-5"):
		return Profile{Levels: []string{"low", "medium", "high", "xhigh"}, AllowNone: false}

	// Kimi K3: low|high|max; none not allowed (CLIProxy zero_allowed=false).
	case strings.HasPrefix(base, "kimi-k3") || strings.HasPrefix(base, "kimi/k3"):
		return Profile{Levels: []string{"low", "high", "max"}, AllowNone: false}

	// Kimi K2.7 Code: low|high; none not allowed.
	case strings.Contains(base, "k2.7-code") || strings.Contains(base, "k2.7_code"):
		return Profile{Levels: []string{"low", "high"}, AllowNone: false}

	// Other Kimi K2.x / thinking: low|high; none allowed (CLIProxy k2.5/k2.6).
	case strings.HasPrefix(base, "kimi"):
		return Profile{Levels: []string{"low", "high"}, AllowNone: true}

	// xAI Grok 4.5: low|medium|high; none not allowed (CLIProxy zero_allowed=false).
	// Older grok-4.3 allows none — we still omit unsupported labels via clamp.
	case strings.HasPrefix(base, "grok-4.5") || strings.HasPrefix(base, "grok"):
		return Profile{Levels: []string{"low", "medium", "high"}, AllowNone: false}

	// Xiaomi MiMo: none|low|medium|high|max (minimal/xhigh rejected live).
	case strings.HasPrefix(base, "mimo"):
		return Profile{Levels: []string{"low", "medium", "high", "max"}, AllowNone: true}

	// Qwen: none|minimal|low|medium|high|xhigh (max rejected live → clamp to xhigh).
	case strings.HasPrefix(base, "qwen"):
		return Profile{Levels: []string{"minimal", "low", "medium", "high", "xhigh"}, AllowNone: true}

	// DeepSeek: broad level set; auto rejected. Official thinking_mode is on/off;
	// Zen/Go expose OpenAI-style reasoning_effort with this enum (live + probes).
	case strings.HasPrefix(base, "deepseek"):
		return Profile{Levels: []string{"minimal", "low", "medium", "high", "xhigh", "max"}, AllowNone: true}

	// GLM / Zhipu thinking: broad incl. max/xhigh (BigModel thinking guide + live).
	case strings.HasPrefix(base, "glm"):
		return Profile{Levels: []string{"minimal", "low", "medium", "high", "xhigh", "max"}, AllowNone: true}

	// MiniMax: broad incl. max/xhigh; auto accepted live.
	case strings.HasPrefix(base, "minimax"):
		return Profile{Levels: []string{"minimal", "low", "medium", "high", "xhigh", "max"}, AllowNone: true, AllowAuto: true}

	// Ollama Gemma / Nemotron / Mistral: OpenAI-compat level models (low|medium|high).
	case strings.HasPrefix(base, "gemma") || strings.HasPrefix(base, "nemotron") ||
		strings.HasPrefix(base, "mistral"):
		return Profile{Levels: []string{"low", "medium", "high"}, AllowNone: true}

	// Default (Zen free unknowns, hy3, laguna, ling, north, big-pickle…):
	// OpenCode2API conservative map — none|low|medium|high; minimal→none, xhigh/max→high.
	default:
		return Profile{Levels: []string{"low", "medium", "high"}, AllowNone: true}
	}
}

// MapForModel normalizes client effort for a specific upstream model.
// Returns "" when the field should be omitted (empty/auto-not-allowed/unknown).
// Never invents unsupported labels such as sending max to a model that only has high.
func MapForModel(model, input string) string {
	level := NormalizeLevel(input)
	if level == "" {
		return ""
	}
	// Aliases before profile checks.
	switch level {
	case "off", "disabled", "disable":
		level = "none"
	case "true", "on", "enabled", "enable", "yes":
		level = "high"
	case "false", "no":
		level = "none"
	}

	p := profileForModel(model)
	if level == "auto" {
		if p.AllowAuto {
			return "auto"
		}
		return ""
	}
	if level == "none" {
		if p.AllowNone {
			return "none"
		}
		return ""
	}

	// Exact match.
	for _, allowed := range p.Levels {
		if level == allowed {
			return level
		}
	}

	// Clamp to nearest allowed level by strength order.
	return clampToProfile(level, p)
}

func clampToProfile(level string, p Profile) string {
	if len(p.Levels) == 0 {
		return ""
	}
	want := strengthIndex(level)
	if want < 0 {
		// Unknown token: do not pass through.
		return ""
	}
	// If none requested but only non-none levels: already handled. For minimal→…
	best := p.Levels[0]
	bestDist := abs(strengthIndex(best) - want)
	for _, cand := range p.Levels[1:] {
		d := abs(strengthIndex(cand) - want)
		// Equal distance: prefer the stronger label (xhigh→max when both high/max exist).
		if d < bestDist || (d == bestDist && strengthIndex(cand) > strengthIndex(best)) {
			best, bestDist = cand, d
		}
	}
	// Prefer "none" when request is weaker than lowest allowed and AllowNone.
	if p.AllowNone && want < strengthIndex(p.Levels[0]) {
		// minimal often maps to none when minimal not in levels (OpenCode table).
		if level == "minimal" {
			return "none"
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// MapForZen is the model-agnostic fallback used when the model ID is unknown.
// Prefer MapForModel. Keeps historical aliases: max stays max (not forced to xhigh);
// auto/empty omit; off→none.
func MapForZen(s string) string {
	return MapForModel("", s)
}
