package analytics_test

import (
	"testing"

	"jovepoxy/internal/analytics"
	"jovepoxy/internal/quota"
)

func TestApplyOpenCodeCascade_monthly_full_blocks_lower(t *testing.T) {
	// Given
	windows := []quota.Window{
		{Label: quota.LabelRolling, Used: 10, Remaining: 90},
		{Label: quota.LabelWeekly, Used: 20, Remaining: 80},
		{Label: quota.LabelMonthly, Used: 100, Remaining: 0},
	}

	// When
	cascaded := analytics.ApplyOpenCodeCascade(windows)

	// Then
	if analytics.EffectiveRemaining(windows) != 0 {
		t.Fatalf("effective = %v", analytics.EffectiveRemaining(windows))
	}
	for _, window := range cascaded {
		if window.Label == quota.LabelMonthly {
			if window.Blocked {
				t.Fatalf("monthly should not be blocked: %+v", window)
			}
			continue
		}
		if !window.Blocked || window.EffectiveRemaining != 0 || window.BlockedBy != quota.LabelMonthly {
			t.Fatalf("window = %+v", window)
		}
	}
}

func TestApplyOpenCodeCascade_weekly_full_blocks_rolling_only(t *testing.T) {
	// Given
	windows := []quota.Window{
		{Label: quota.LabelRolling, Used: 5, Remaining: 95},
		{Label: quota.LabelWeekly, Used: 100, Remaining: 0},
		{Label: quota.LabelMonthly, Used: 40, Remaining: 60},
	}

	// When
	cascaded := analytics.ApplyOpenCodeCascade(windows)

	// Then
	for _, window := range cascaded {
		switch window.Label {
		case quota.LabelRolling:
			if !window.Blocked || window.BlockedBy != quota.LabelWeekly {
				t.Fatalf("rolling = %+v", window)
			}
		case quota.LabelWeekly, quota.LabelMonthly:
			if window.Blocked {
				t.Fatalf("unexpected block: %+v", window)
			}
		}
	}
}
