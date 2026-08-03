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

func TestMapForZen(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"auto", ""},
		{"AUTO", ""},
		{"off", "none"},
		{"max", "xhigh"},
		{"MAX", "xhigh"},
		{"high", "high"},
		{"xhigh", "xhigh"},
		{" medium ", "medium"},
	}
	for _, tc := range cases {
		if got := MapForZen(tc.in); got != tc.want {
			t.Fatalf("MapForZen(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
