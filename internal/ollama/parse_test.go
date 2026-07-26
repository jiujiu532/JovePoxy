package ollama_test

import (
	"testing"
	"time"

	"jovepoxy/internal/ollama"
)

func TestNormalizeSessionCookie(t *testing.T) {
	got, err := ollama.NormalizeSessionCookie("abc123")
	if err != nil || got != "__Secure-session=abc123" {
		t.Fatalf("raw = %q err=%v", got, err)
	}
	full := "aid=x; __Secure-session=token123"
	got, err = ollama.NormalizeSessionCookie(full)
	if err != nil || got != full {
		t.Fatalf("full = %q err=%v", got, err)
	}
}

func TestParseQuotaHTML_fixture(t *testing.T) {
	html := `
    <span>Cloud usage</span>
    <span class="text-xs font-normal px-2 py-0.5 rounded-full bg-neutral-100 text-neutral-600 capitalize">pro</span>
    <div class="flex justify-between mb-2">
      <span class="text-sm text-neutral-400">Session usage</span>
      <span class="text-sm text-neutral-500">Weekly limit reached</span>
    </div>
    <div data-usage-track aria-label="Session usage 12.5% used"></div>
    <div class="local-time" data-time="2026-06-29T00:00:00Z">Sessions resume in 3 days.</div>
    <div class="flex justify-between mb-2">
      <span class="text-sm">Weekly usage</span>
      <span class="text-sm text-red-500">80% used</span>
    </div>
    <div data-usage-track aria-label="Weekly usage 80% used">
      <button data-usage-segment data-model="glm-5.2" data-requests="100" style="width: 80%; background: #3b82f6"></button>
    </div>
    <div class="local-time" data-time="2026-06-30T00:00:00Z">Resets in 4 days.</div>
    <script>
    `
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	plan, windows, err := ollama.ParseQuotaHTML(html, now)
	if err != nil {
		t.Fatalf("ParseQuotaHTML: %v", err)
	}
	if plan != "pro" {
		t.Fatalf("plan = %q", plan)
	}
	if len(windows) != 2 {
		t.Fatalf("windows = %d", len(windows))
	}
	if windows[0].Label != ollama.LabelSession || windows[0].Used != 12.5 {
		t.Fatalf("session = %+v", windows[0])
	}
	if windows[0].StatusText != "Weekly limit reached" {
		t.Fatalf("status = %q", windows[0].StatusText)
	}
	if windows[1].Used != 80 || len(windows[1].Models) == 0 || windows[1].Models[0].Model != "glm-5.2" {
		t.Fatalf("weekly = %+v", windows[1])
	}
}
