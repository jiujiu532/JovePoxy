package effort

import "testing"

func TestBudgetToLevel(t *testing.T) {
	cases := []struct {
		budget int
		want   string
	}{
		{-2, ""},
		{-1, "auto"},
		{0, "none"},
		{1, "minimal"},
		{512, "minimal"},
		{513, "low"},
		{1024, "low"},
		{1025, "medium"},
		{8192, "medium"},
		{8193, "high"},
		{24576, "high"},
		{24577, "xhigh"},
		{100000, "xhigh"},
	}
	for _, tc := range cases {
		if got := BudgetToLevel(tc.budget); got != tc.want {
			t.Fatalf("BudgetToLevel(%d) = %q, want %q", tc.budget, got, tc.want)
		}
	}
}

func TestNormalizeLevel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"HIGH", "high"},
		{" Medium ", "medium"},
		{"xHigh", "xhigh"},
		{"weird-Level", "weird-level"},
	}
	for _, tc := range cases {
		if got := NormalizeLevel(tc.in); got != tc.want {
			t.Fatalf("NormalizeLevel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMapForModel_families(t *testing.T) {
	cases := []struct {
		model, in, want string
	}{
		// omit / aliases
		{"glm-5.2", "", ""},
		{"glm-5.2", "auto", ""},
		{"minimax-m2.7", "auto", "auto"},
		{"any", "off", "none"},
		{"gpt-5.6-luna", "off", ""}, // none not in codex set → omit

		// gpt-oss: no minimal/xhigh
		{"gpt-oss:20b", "minimal", "none"},
		{"gpt-oss:20b", "xhigh", "max"}, // tie high/max → stronger
		{"gpt-oss:20b", "max", "max"},
		{"gpt-oss:120b", "low", "low"},

		// kimi k2.7-code: low|high only; none not allowed
		{"kimi-k2.7-code", "minimal", "low"}, // no none → nearest low
		{"kimi-k2.7-code", "none", ""},
		{"kimi-k2.7-code", "medium", "high"}, // tie low/high → stronger
		{"kimi-k2.7-code", "xhigh", "high"},
		{"kimi-k2.7-code", "max", "high"},
		{"kimi-k2.7-code", "low", "low"},
		{"kimi-k2.7-code", "high", "high"},

		// kimi k2.5: low|high + none
		{"kimi-k2.5", "minimal", "none"},
		{"kimi-k2.5", "none", "none"},

		// kimi k3: low|high|max; none not allowed
		{"kimi-k3", "max", "max"},
		{"kimi-k3", "xhigh", "max"},
		{"kimi-k3", "medium", "high"},
		{"kimi-k3", "none", ""},

		// gpt-5.6-luna: has xhigh+max
		{"gpt-5.6-luna", "xhigh", "xhigh"},
		{"gpt-5.6-luna", "max", "max"},
		{"gpt-5.6-luna", "minimal", "low"},
		{"gpt-5.6-luna", "none", ""},

		// gpt-5.4: xhigh yes, max no
		{"gpt-5.4", "max", "xhigh"},
		{"gpt-5.4", "xhigh", "xhigh"},

		// qwen: no max
		{"qwen3.5-plus", "max", "xhigh"},
		{"qwen3.5-plus", "xhigh", "xhigh"},
		{"qwen3.5-plus", "auto", ""},

		// mimo: no minimal/xhigh
		{"mimo-v2.5", "minimal", "none"},
		{"mimo-v2.5", "xhigh", "max"},
		{"mimo-v2.5", "max", "max"},

		// grok-4.5: no none/xhigh/max
		{"grok-4.5", "xhigh", "high"},
		{"grok-4.5", "max", "high"},
		{"grok-4.5", "medium", "medium"},
		{"grok-4.5", "none", ""},

		// deepseek free/paid
		{"deepseek-v4-flash-free", "max", "max"},
		{"deepseek-v4-flash", "xhigh", "xhigh"},
		{"deepseek-v4-flash", "auto", ""},

		// default conservative
		{"big-pickle", "xhigh", "high"},
		{"big-pickle", "max", "high"},
		{"big-pickle", "minimal", "none"},
		{"hy3", "high", "high"},

		// unknown garbage not passed through
		{"glm-5.2", "super-ultra", ""},
	}
	for _, tc := range cases {
		if got := MapForModel(tc.model, tc.in); got != tc.want {
			t.Fatalf("MapForModel(%q,%q) = %q, want %q", tc.model, tc.in, got, tc.want)
		}
	}
}

func TestMapForZen_defaultFamily(t *testing.T) {
	// Empty model → default profile (conservative).
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"auto", ""},
		{"off", "none"},
		{"minimal", "none"},
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
		{"xhigh", "high"},
		{"max", "high"},
	}
	for _, tc := range cases {
		if got := MapForZen(tc.in); got != tc.want {
			t.Fatalf("MapForZen(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestClamp_mediumOnKimiPrefersHighOrLow(t *testing.T) {
	// Document actual clamp for medium with only low/high: equal distance picks first scanned = low.
	got := MapForModel("kimi-k2.7-code", "medium")
	if got != "low" && got != "high" {
		t.Fatalf("medium clamp = %q, want low or high", got)
	}
}

func TestLevelsForDisplay(t *testing.T) {
	cases := []struct {
		model string
		want  []string
	}{
		{"gpt-oss:20b", []string{"none", "low", "medium", "high", "max"}},
		{"gpt-5.6-luna", []string{"low", "medium", "high", "xhigh", "max"}},
		{"gpt-5.4", []string{"low", "medium", "high", "xhigh"}},
		{"kimi-k2.7-code", []string{"low", "high"}},
		{"kimi-k2.5", []string{"none", "low", "high"}},
		{"kimi-k3", []string{"low", "high", "max"}},
		{"grok-4.5", []string{"low", "medium", "high"}},
		{"qwen3.5-plus", []string{"none", "minimal", "low", "medium", "high", "xhigh"}},
		{"minimax-m2.7", []string{"none", "minimal", "low", "medium", "high", "xhigh", "max", "auto"}},
		{"big-pickle", []string{"none", "low", "medium", "high"}},
	}
	for _, tc := range cases {
		got := LevelsForDisplay(tc.model)
		if len(got) != len(tc.want) {
			t.Fatalf("LevelsForDisplay(%q)=%v want %v", tc.model, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("LevelsForDisplay(%q)=%v want %v", tc.model, got, tc.want)
			}
		}
	}
}
