package quota_test

import (
	"testing"

	"jovepoxy/internal/quota"
)

func TestDeriveWindowNarrative(t *testing.T) {
	t.Parallel()

	ptr := func(v float64) *float64 { return &v }

	cases := []struct {
		name      string
		used      float64
		remaining float64
		total     float64
		wantUsed  *float64
		wantHead  *float64
	}{
		{
			name: "total_zero",
			used: 10, remaining: 0, total: 0,
			wantUsed: nil, wantHead: nil,
		},
		{
			name: "total_negative",
			used: 10, remaining: 5, total: -1,
			wantUsed: nil, wantHead: nil,
		},
		{
			name: "normal_half",
			used: 50, remaining: 50, total: 100,
			wantUsed: ptr(50), wantHead: ptr(50),
		},
		{
			name: "normal_fraction",
			used: 12.5, remaining: 7.5, total: 20,
			wantUsed: ptr(62.5), wantHead: ptr(37.5),
		},
		{
			name: "remaining_zero",
			used: 100, remaining: 0, total: 100,
			wantUsed: ptr(100), wantHead: ptr(0),
		},
		{
			name: "used_zero",
			used: 0, remaining: 100, total: 100,
			wantUsed: ptr(0), wantHead: ptr(100),
		},
		{
			name: "used_over_total_clamps",
			used: 150, remaining: -10, total: 100,
			wantUsed: ptr(100), wantHead: ptr(0),
		},
		{
			name: "negative_used_clamps",
			used: -5, remaining: 100, total: 100,
			wantUsed: ptr(0), wantHead: ptr(100),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := quota.DeriveWindowNarrative(tc.used, tc.remaining, tc.total)
			if got.BurnPerDay != nil || got.DaysToEmpty != nil {
				t.Fatalf("burn/days must be nil without rate: %+v", got)
			}
			assertPct(t, "used_pct", got.UsedPct, tc.wantUsed)
			assertPct(t, "headroom_pct", got.HeadroomPct, tc.wantHead)
		})
	}
}

func TestDaysToEmptyFromBurn(t *testing.T) {
	t.Parallel()

	if got := quota.DaysToEmptyFromBurn(10, 0); got != nil {
		t.Fatalf("zero burn should be nil, got %v", *got)
	}
	if got := quota.DaysToEmptyFromBurn(10, -1); got != nil {
		t.Fatalf("negative burn should be nil, got %v", *got)
	}
	if got := quota.DaysToEmptyFromBurn(-1, 2); got != nil {
		t.Fatalf("negative remaining should be nil, got %v", *got)
	}
	got := quota.DaysToEmptyFromBurn(7.5, 2.5)
	if got == nil || *got != 3 {
		t.Fatalf("days = %v want 3", got)
	}
}

func assertPct(t *testing.T, label string, got, want *float64) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("%s: got %v want %v", label, got, want)
	}
	if *got != *want {
		t.Fatalf("%s: got %v want %v", label, *got, *want)
	}
}
