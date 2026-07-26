package quota

import (
	"regexp"
	"strconv"
	"time"
)

// Window labels match QuotaHub dashboard sections.
const (
	LabelRolling = "5h Rolling"
	LabelWeekly  = "Weekly"
	LabelMonthly = "Monthly"
)

// Window is one normalized usage window expressed as percent of quota.
type Window struct {
	Label      string    `json:"label"`
	Used       float64   `json:"used"`
	Remaining  float64   `json:"remaining"`
	Total      float64   `json:"total"`
	Unit       string    `json:"unit"`
	ResetAt    time.Time `json:"reset_at"`
	ResetInSec int       `json:"reset_in_sec"`
}

var (
	reRollingPctFirst   = regexp.MustCompile(`rollingUsage:\s*\$R\[\d+\]\s*=\s*\{[^}]*usagePercent\s*:\s*(-?\d+(?:\.\d+)?)[^}]*resetInSec\s*:\s*(-?\d+(?:\.\d+)?)[^}]*\}`)
	reRollingResetFirst = regexp.MustCompile(`rollingUsage:\s*\$R\[\d+\]\s*=\s*\{[^}]*resetInSec\s*:\s*(-?\d+(?:\.\d+)?)[^}]*usagePercent\s*:\s*(-?\d+(?:\.\d+)?)[^}]*\}`)
	reWeeklyPctFirst    = regexp.MustCompile(`weeklyUsage:\s*\$R\[\d+\]\s*=\s*\{[^}]*usagePercent\s*:\s*(-?\d+(?:\.\d+)?)[^}]*resetInSec\s*:\s*(-?\d+(?:\.\d+)?)[^}]*\}`)
	reWeeklyResetFirst  = regexp.MustCompile(`weeklyUsage:\s*\$R\[\d+\]\s*=\s*\{[^}]*resetInSec\s*:\s*(-?\d+(?:\.\d+)?)[^}]*usagePercent\s*:\s*(-?\d+(?:\.\d+)?)[^}]*\}`)
	reMonthlyPctFirst   = regexp.MustCompile(`monthlyUsage:\s*\$R\[\d+\]\s*=\s*\{[^}]*usagePercent\s*:\s*(-?\d+(?:\.\d+)?)[^}]*resetInSec\s*:\s*(-?\d+(?:\.\d+)?)[^}]*\}`)
	reMonthlyResetFirst = regexp.MustCompile(`monthlyUsage:\s*\$R\[\d+\]\s*=\s*\{[^}]*resetInSec\s*:\s*(-?\d+(?:\.\d+)?)[^}]*usagePercent\s*:\s*(-?\d+(?:\.\d+)?)[^}]*\}`)
)

// ParseQuotaHTML extracts rolling/weekly/monthly windows from dashboard HTML/JS.
func ParseQuotaHTML(html string, now time.Time) []Window {
	pairs := []struct {
		label      string
		pctFirst   *regexp.Regexp
		resetFirst *regexp.Regexp
	}{
		{LabelRolling, reRollingPctFirst, reRollingResetFirst},
		{LabelWeekly, reWeeklyPctFirst, reWeeklyResetFirst},
		{LabelMonthly, reMonthlyPctFirst, reMonthlyResetFirst},
	}
	windows := make([]Window, 0, 3)
	for _, pair := range pairs {
		usagePercent, resetInSec, ok := parseWindow(pair.pctFirst, pair.resetFirst, html)
		if !ok {
			continue
		}
		windows = append(windows, normalizeWindow(pair.label, usagePercent, resetInSec, now))
	}
	return windows
}

// FilterWindows keeps only windows enabled on the account visibility flags.
func FilterWindows(windows []Window, account Account) []Window {
	out := make([]Window, 0, len(windows))
	for _, window := range windows {
		switch window.Label {
		case LabelRolling:
			if !account.ShowRolling {
				continue
			}
		case LabelWeekly:
			if !account.ShowWeekly {
				continue
			}
		case LabelMonthly:
			if !account.ShowMonthly {
				continue
			}
		}
		out = append(out, window)
	}
	return out
}

func parseWindow(pctFirst, resetFirst *regexp.Regexp, html string) (float64, int, bool) {
	if match := pctFirst.FindStringSubmatch(html); len(match) == 3 {
		return mustFloat(match[1]), mustInt(match[2]), true
	}
	if match := resetFirst.FindStringSubmatch(html); len(match) == 3 {
		return mustFloat(match[2]), mustInt(match[1]), true
	}
	return 0, 0, false
}

func normalizeWindow(label string, usagePercent float64, resetInSec int, now time.Time) Window {
	used := clampPercent(usagePercent)
	return Window{
		Label: label, Used: used, Remaining: 100 - used, Total: 100, Unit: "%",
		ResetAt: now.UTC().Add(time.Duration(resetInSec) * time.Second), ResetInSec: resetInSec,
	}
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func mustFloat(raw string) float64 {
	value, _ := strconv.ParseFloat(raw, 64)
	return value
}

func mustInt(raw string) int {
	value, _ := strconv.ParseFloat(raw, 64)
	return int(value)
}
