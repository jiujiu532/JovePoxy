package quota_test

import (
	"testing"
	"time"

	"jovepoxy/internal/quota"
)

func TestParseQuotaHTML_extracts_three_windows(t *testing.T) {
	// Given
	html := `
	rollingUsage: $R[0] = { usagePercent: 12.5, resetInSec: 3600 }
	weeklyUsage: $R[0] = { usagePercent: 40, resetInSec: 86400 }
	monthlyUsage: $R[0] = { usagePercent: 75.2, resetInSec: 1209600 }
	`
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	// When
	windows := quota.ParseQuotaHTML(html, now)

	// Then
	if len(windows) != 3 {
		t.Fatalf("windows = %d, want 3", len(windows))
	}
	if windows[0].Label != quota.LabelRolling || windows[0].Used != 12.5 || windows[0].Remaining != 87.5 {
		t.Fatalf("rolling = %+v", windows[0])
	}
	if windows[1].Used != 40 || windows[2].Used != 75.2 {
		t.Fatalf("weekly/monthly = %+v %+v", windows[1], windows[2])
	}
	if !windows[0].ResetAt.Equal(now.Add(3600 * time.Second)) {
		t.Fatalf("reset_at = %v", windows[0].ResetAt)
	}
}

func TestParseQuotaHTML_handles_reset_first_field_order(t *testing.T) {
	// Given
	html := `rollingUsage: $R[1] = { resetInSec: 100, usagePercent: 8 }`
	now := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	// When
	windows := quota.ParseQuotaHTML(html, now)

	// Then
	if len(windows) != 1 || windows[0].Used != 8 || windows[0].ResetInSec != 100 {
		t.Fatalf("windows = %+v", windows)
	}
}

func TestFilterWindows_respects_visibility_flags(t *testing.T) {
	// Given
	windows := []quota.Window{
		{Label: quota.LabelRolling, Used: 1},
		{Label: quota.LabelWeekly, Used: 2},
		{Label: quota.LabelMonthly, Used: 3},
	}
	account := quota.Account{ShowRolling: true, ShowWeekly: false, ShowMonthly: true}

	// When
	filtered := quota.FilterWindows(windows, account)

	// Then
	if len(filtered) != 2 || filtered[0].Label != quota.LabelRolling || filtered[1].Label != quota.LabelMonthly {
		t.Fatalf("filtered = %+v", filtered)
	}
}
