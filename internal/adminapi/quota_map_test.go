package adminapi

import (
	"encoding/json"
	"testing"
	"time"

	"jovepoxy/internal/quota"
)

func TestMapQuotas_includesUsedPct(t *testing.T) {
	t.Parallel()
	list := []quota.AccountQuota{
		{
			AccountID: "acct_1", Name: "main", WorkspaceID: "wrk_1",
			Success: true, UpdatedAt: time.Unix(0, 0).UTC(),
			Windows: []quota.Window{
				{Label: "Weekly", Used: 25, Remaining: 75, Total: 100, Unit: "%", ResetInSec: 3600},
			},
		},
		{
			AccountID: "acct_fail", Name: "bad", WorkspaceID: "wrk_2",
			Success: false, UpdatedAt: time.Unix(0, 0).UTC(), Error: "scrape failed",
		},
	}
	mapped := mapQuotas(list)
	if len(mapped.Quotas) != 2 {
		t.Fatalf("len = %d", len(mapped.Quotas))
	}
	ok := mapped.Quotas[0]
	if ok.Narrative == nil || ok.Narrative.UsedPct == nil || *ok.Narrative.UsedPct != 25 {
		t.Fatalf("narrative = %+v", ok.Narrative)
	}
	if len(ok.Windows) != 1 || ok.Windows[0].UsedPct == nil || *ok.Windows[0].UsedPct != 25 {
		t.Fatalf("window = %+v", ok.Windows)
	}
	if ok.Windows[0].HeadroomPct == nil || *ok.Windows[0].HeadroomPct != 75 {
		t.Fatalf("headroom = %+v", ok.Windows[0].HeadroomPct)
	}
	if ok.Windows[0].BurnPerDay != nil || ok.Windows[0].DaysToEmpty != nil {
		t.Fatalf("burn must be omitted when unknown: %+v", ok.Windows[0])
	}
	fail := mapped.Quotas[1]
	if fail.Narrative != nil {
		t.Fatalf("failed scrape must not invent narrative: %+v", fail.Narrative)
	}

	raw, err := json.Marshal(mapped)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	if !containsAll(body, `"used_pct":25`, `"headroom_pct":75`) {
		t.Fatalf("json missing pct fields: %s", body)
	}
	if containsAll(body, `"burn_per_day"`) {
		t.Fatalf("json should omit burn_per_day: %s", body)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !contains(s, part) {
			return false
		}
	}
	return true
}

func contains(s, part string) bool {
	return len(part) == 0 || (len(s) >= len(part) && indexOf(s, part) >= 0)
}

func indexOf(s, part string) int {
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}
