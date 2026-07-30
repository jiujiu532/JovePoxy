package analytics_test

import (
	"testing"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/quota"
)

func TestBuildQuotaNarrative(t *testing.T) {
	t.Parallel()

	t.Run("empty_windows", func(t *testing.T) {
		t.Parallel()
		got := analytics.BuildQuotaNarrative(nil, 0)
		if got == nil || got.Note != "sample_insufficient" {
			t.Fatalf("got %+v", got)
		}
		if got.WorstUsedPct != nil || got.BurnPerDay != nil || got.DaysToEmpty != nil {
			t.Fatalf("must not invent burn fields: %+v", got)
		}
	})

	t.Run("picks_worst_used", func(t *testing.T) {
		t.Parallel()
		windows := []quota.Window{
			{Label: "Weekly", Used: 20, Remaining: 80, Total: 100, Unit: "%"},
			{Label: "Monthly", Used: 75, Remaining: 25, Total: 100, Unit: "%"},
		}
		got := analytics.BuildQuotaNarrative(windows, 20)
		if got == nil || got.WorstUsedPct == nil || *got.WorstUsedPct != 75 {
			t.Fatalf("worst = %+v want 75", got)
		}
		if got.HeadroomPct == nil || *got.HeadroomPct != 25 {
			t.Fatalf("headroom = %+v want 25", got)
		}
		if got.EffectiveRemaining != 20 {
			t.Fatalf("effective = %v", got.EffectiveRemaining)
		}
		if got.BurnPerDay != nil || got.DaysToEmpty != nil {
			t.Fatalf("burn must stay nil: %+v", got)
		}
	})

	t.Run("total_zero_sample_insufficient", func(t *testing.T) {
		t.Parallel()
		windows := []quota.Window{
			{Label: "Weekly", Used: 1, Remaining: 0, Total: 0, Unit: "%"},
		}
		got := analytics.BuildQuotaNarrative(windows, 0)
		if got == nil || got.Note != "sample_insufficient" || got.WorstUsedPct != nil {
			t.Fatalf("got %+v", got)
		}
	})
}
