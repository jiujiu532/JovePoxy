package ollama

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	LabelSession = "Session"
	LabelWeekly  = "Weekly"
)

// ModelUsage is one model segment on a usage track.
type ModelUsage struct {
	Model        string   `json:"model"`
	Requests     int      `json:"requests"`
	SharePercent *float64 `json:"share_percent,omitempty"`
}

// Window is a Session/Weekly quota window.
type Window struct {
	Label      string       `json:"label"`
	Used       float64      `json:"used"`
	Remaining  float64      `json:"remaining"`
	Total      float64      `json:"total"`
	Unit       string       `json:"unit"`
	ResetAt    string       `json:"reset_at"`
	ResetInSec int          `json:"reset_in_sec"`
	StatusText string       `json:"status_text,omitempty"`
	Models     []ModelUsage `json:"models,omitempty"`
}

var (
	reCloudUsageBlock = regexp.MustCompile(`(?is)<span>Cloud usage</span>(.*?)</div>\s*<script>`)
	rePlan            = regexp.MustCompile(`(?i)rounded-full[^>]*capitalize[^>]*>\s*([^<]+?)\s*</span`)
	reUsageTrack      = regexp.MustCompile(`(?is)data-usage-track[^>]*aria-label="([^"]+)"[^>]*>(.*?)</div>`)
	reUsageSegment    = regexp.MustCompile(`(?i)<button\b[^>]*data-usage-segment[^>]*>`)
	reDataModel       = regexp.MustCompile(`data-model="([^"]+)"`)
	reDataRequests    = regexp.MustCompile(`data-requests="(\d+)"`)
	reWidthPercent    = regexp.MustCompile(`width:\s*([\d.]+)%`)
	rePercentUsed     = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*%\s*used`)
	rePeriodHeader    = regexp.MustCompile(`(?is)<div class="flex justify-between mb-2">(.*?)</div>`)
	reHeaderSpan      = regexp.MustCompile(`(?is)<span class="text-sm[^"]*"[^>]*>\s*([^<]+?)\s*</span`)
	reLocalTime       = regexp.MustCompile(`(?i)class="[^"]*local-time[^"]*"[^>]*data-time="([^"]+)"[^>]*>\s*([^<]+?)\s*</div>`)
	reSignIn          = regexp.MustCompile(`(?i)(sign in|log in|invalid credentials)`)
)

// ParseQuotaHTML extracts plan + Session/Weekly windows from ollama.com/settings HTML.
func ParseQuotaHTML(html string, now time.Time) (string, []Window, error) {
	if reSignIn.MatchString(html) && !strings.Contains(html, "Cloud usage") {
		return "", nil, fmt.Errorf("not signed in or cookie invalid")
	}
	match := reCloudUsageBlock.FindStringSubmatch(html)
	if len(match) < 2 {
		return "", nil, fmt.Errorf("cloud usage block not found")
	}
	block := match[1]
	plan := ""
	if planMatch := rePlan.FindStringSubmatch(block); len(planMatch) == 2 {
		plan = strings.TrimSpace(planMatch[1])
	}
	tracks := parseUsageTracks(block)
	statusTexts := parsePeriodHeaders(block)
	resets := parseResetInfo(block)

	windows := make([]Window, 0, 2)
	labels := []string{LabelSession, LabelWeekly}
	for index, label := range labels {
		var track usageTrack
		if index < len(tracks) {
			track = tracks[index]
		}
		var resetAt string
		if index < len(resets) {
			resetAt = resets[index]
		}
		var status string
		if index < len(statusTexts) {
			status = statusTexts[index]
		}
		window := buildWindow(label, track.usedPercent, status, resetAt, track.models, now)
		if window != nil {
			windows = append(windows, *window)
		}
	}
	if len(windows) == 0 {
		return "", nil, fmt.Errorf("unable to parse cloud usage windows")
	}
	return plan, windows, nil
}

type usageTrack struct {
	usedPercent *float64
	models      []ModelUsage
}

func parseUsageTracks(block string) []usageTrack {
	tracks := make([]usageTrack, 0)
	for _, trackMatch := range reUsageTrack.FindAllStringSubmatch(block, -1) {
		if len(trackMatch) < 3 {
			continue
		}
		aria := trackMatch[1]
		inner := trackMatch[2]
		models := make([]ModelUsage, 0)
		for _, seg := range reUsageSegment.FindAllString(inner, -1) {
			modelMatch := reDataModel.FindStringSubmatch(seg)
			requestsMatch := reDataRequests.FindStringSubmatch(seg)
			if len(modelMatch) < 2 || len(requestsMatch) < 2 {
				continue
			}
			requests, _ := strconv.Atoi(requestsMatch[1])
			item := ModelUsage{Model: modelMatch[1], Requests: requests}
			if widthMatch := reWidthPercent.FindStringSubmatch(seg); len(widthMatch) == 2 {
				if width, err := strconv.ParseFloat(widthMatch[1], 64); err == nil {
					item.SharePercent = &width
				}
			}
			models = append(models, item)
		}
		tracks = append(tracks, usageTrack{usedPercent: parsePercentFromAria(aria), models: models})
	}
	return tracks
}

func parsePercentFromAria(aria string) *float64 {
	match := rePercentUsed.FindStringSubmatch(aria)
	if len(match) < 2 {
		return nil
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil
	}
	return &value
}

func parsePeriodHeaders(block string) []string {
	headers := make([]string, 0, 2)
	for _, section := range rePeriodHeader.FindAllStringSubmatch(block, -1) {
		if len(section) < 2 {
			continue
		}
		spans := reHeaderSpan.FindAllStringSubmatch(section[1], -1)
		if len(spans) >= 2 {
			headers = append(headers, strings.TrimSpace(spans[1][1]))
		}
		if len(headers) >= 2 {
			break
		}
	}
	for len(headers) < 2 {
		headers = append(headers, "")
	}
	return headers[:2]
}

func parseResetInfo(block string) []string {
	resets := make([]string, 0, 2)
	for _, item := range reLocalTime.FindAllStringSubmatch(block, -1) {
		if len(item) >= 2 {
			resets = append(resets, item[1])
		}
		if len(resets) >= 2 {
			break
		}
	}
	for len(resets) < 2 {
		resets = append(resets, "")
	}
	return resets[:2]
}

func buildWindow(label string, usedPercent *float64, statusText, resetAtRaw string, models []ModelUsage, now time.Time) *Window {
	if usedPercent == nil {
		return nil
	}
	used := clampPercent(*usedPercent)
	resetAt := ""
	resetInSec := 0
	if parsed := parseResetAt(resetAtRaw); parsed != nil {
		resetInSec = int(parsed.Sub(now).Seconds())
		if resetInSec < 0 {
			resetInSec = 0
		}
		resetAt = parsed.UTC().Format(time.RFC3339)
	}
	return &Window{
		Label: label, Used: used, Remaining: 100 - used, Total: 100, Unit: "%",
		ResetAt: resetAt, ResetInSec: resetInSec, StatusText: statusText, Models: models,
	}
}

func parseResetAt(value string) *time.Time {
	text := strings.TrimSpace(value)
	if text == "" {
		return nil
	}
	if strings.HasSuffix(text, "Z") {
		text = strings.TrimSuffix(text, "Z") + "+00:00"
	}
	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339Nano, text)
		if err != nil {
			return nil
		}
	}
	utc := parsed.UTC()
	return &utc
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
