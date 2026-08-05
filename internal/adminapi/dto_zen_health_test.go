package adminapi

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"jovepoxy/internal/zenpool"
)

func TestMapZenKeysAt_healthColdStartAndDynamicShare(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	until := now.Add(2 * time.Minute)
	list := []zenpool.Metadata{
		{ID: "zk_a", Label: "A", Prefix: "sk-aaa…", Weight: 9, Enabled: true, Provider: zenpool.ProviderOpenCode, CreatedAt: now},
		{ID: "zk_b", Label: "B", Prefix: "sk-bbb…", Weight: 1, Enabled: true, Provider: zenpool.ProviderOpenCode, CreatedAt: now},
		{ID: "zk_c", Label: "C", Prefix: "sk-ccc…", Weight: 5, Enabled: true, Provider: zenpool.ProviderOpenCode, CooldownUntil: &until, CreatedAt: now},
		{ID: "zk_d", Label: "D", Prefix: "sk-ddd…", Weight: 1, Enabled: false, Provider: zenpool.ProviderOpenCode, CreatedAt: now},
		{ID: "zk_o", Label: "O", Prefix: "sk-ooo…", Weight: 1, Enabled: true, Provider: zenpool.ProviderOllama, CreatedAt: now},
	}
	benched := map[zenpool.KeyID]time.Time{"zk_b": now.Add(time.Minute)}

	resp := mapZenKeysAt(list, now, benched)
	if len(resp.Keys) != 5 {
		t.Fatalf("keys len = %d", len(resp.Keys))
	}
	if resp.Summary == nil {
		t.Fatal("expected summary")
	}

	byID := map[string]zenKeyDTO{}
	for _, key := range resp.Keys {
		byID[key.ID] = key
	}

	// Cold-start health for missing domain; weight must not drive traffic share.
	activeA := byID["zk_a"]
	if activeA.HealthScore != coldStartHealthScore || activeA.SelectionScore != coldStartHealthScore {
		t.Fatalf("A health/selection = %v/%v, want cold-start %v", activeA.HealthScore, activeA.SelectionScore, coldStartHealthScore)
	}
	if activeA.SuccessCount != 0 || activeA.FailureCount != 0 || activeA.ConsecutiveFailures != 0 {
		t.Fatalf("A counters should be zero: %+v", activeA)
	}
	if activeA.LastErrorClass != "" || activeA.CooldownReason != "" {
		t.Fatalf("A error/reason must be empty: %+v", activeA)
	}
	if activeA.LastSuccessAt != nil || activeA.LastFailureAt != nil || activeA.HealthUpdatedAt != nil {
		t.Fatalf("A times must be nil without domain: %+v", activeA)
	}
	if activeA.Status != string(zenpool.StatusActive) {
		t.Fatalf("A status = %s", activeA.Status)
	}
	// Two active OpenCode keys would be A only (B benched, C cooling, D disabled) → 100%.
	// Weight was 9 vs 1 historically; with equal selection_score only A is eligible → 100.
	if math.Abs(activeA.TrafficPct-100) > 0.01 {
		t.Fatalf("A traffic_pct = %v, want 100 (dynamic, weight ignored)", activeA.TrafficPct)
	}

	benchedB := byID["zk_b"]
	if benchedB.Status != string(zenpool.StatusBenched) || benchedB.TrafficPct != 0 {
		t.Fatalf("B should be benched with 0 share: %+v", benchedB)
	}
	coolingC := byID["zk_c"]
	if coolingC.Status != string(zenpool.StatusCooling) || coolingC.TrafficPct != 0 {
		t.Fatalf("C should be cooling with 0 share: %+v", coolingC)
	}
	if coolingC.CooldownRemainingSec <= 0 {
		t.Fatalf("C remaining = %d", coolingC.CooldownRemainingSec)
	}
	disabledD := byID["zk_d"]
	if disabledD.Status != string(zenpool.StatusDisabled) || disabledD.TrafficPct != 0 {
		t.Fatalf("D should be disabled with 0 share: %+v", disabledD)
	}

	// Ollama active alone in its provider → 100%, independent of OpenCode.
	ollama := byID["zk_o"]
	if ollama.Provider != string(zenpool.ProviderOllama) || math.Abs(ollama.TrafficPct-100) > 0.01 {
		t.Fatalf("ollama share = %+v", ollama)
	}

	// Summary counters + share mode + attention (cooled+benched+probing).
	if resp.Summary.ShareMode != "dynamic_health_estimate" {
		t.Fatalf("share_mode = %q", resp.Summary.ShareMode)
	}
	if resp.Summary.Probing != 0 {
		t.Fatalf("probing = %d without domain", resp.Summary.Probing)
	}
	// cooled=1 (C), benched=1 (B) → attention=2
	if resp.Summary.Attention != 2 {
		t.Fatalf("attention = %d, want 2", resp.Summary.Attention)
	}
	if resp.Summary.Healthy < 1 || resp.Summary.Cooled != 1 || resp.Summary.Benched != 1 {
		t.Fatalf("summary counts = %+v", resp.Summary)
	}
}

func TestMapZenKeysAt_equalDynamicShareIgnoresWeight(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	list := []zenpool.Metadata{
		{ID: "zk_a", Label: "A", Prefix: "a…", Weight: 9, Enabled: true, Provider: zenpool.ProviderOpenCode, CreatedAt: now},
		{ID: "zk_b", Label: "B", Prefix: "b…", Weight: 1, Enabled: true, Provider: zenpool.ProviderOpenCode, CreatedAt: now},
	}
	resp := mapZenKeysAt(list, now, nil)
	if len(resp.Keys) != 2 {
		t.Fatalf("len = %d", len(resp.Keys))
	}
	// Both cold-start selection_score=70 → 50/50, not 90/10 from weight.
	if math.Abs(resp.Keys[0].TrafficPct-50) > 0.01 || math.Abs(resp.Keys[1].TrafficPct-50) > 0.01 {
		t.Fatalf("shares = %v %v, want 50/50 from health not weight", resp.Keys[0].TrafficPct, resp.Keys[1].TrafficPct)
	}
}

func TestApplyHealthView_preservesExplicitZero(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	updated := now.Add(-time.Minute)
	list := []zenpool.Metadata{
		{
			ID: "zk_zero", Label: "Z", Prefix: "sk-zzz…", Weight: 1, Enabled: true,
			Provider: zenpool.ProviderOpenCode, CreatedAt: now,
			HealthScore: 0, SelectionScore: 1, SuccessCount: 0, FailureCount: 5,
			ConsecutiveFailures: 5, LastErrorClass: "upstream_5xx",
			HealthUpdatedAt: &updated, CooldownReason: "upstream_5xx",
		},
	}
	resp := mapZenKeysAt(list, now, nil)
	if len(resp.Keys) != 1 {
		t.Fatalf("len = %d", len(resp.Keys))
	}
	if resp.Keys[0].HealthScore != 0 {
		t.Fatalf("health_score = %v, want explicit 0 (not rewritten to 70)", resp.Keys[0].HealthScore)
	}
	if resp.Keys[0].SelectionScore != 1 {
		t.Fatalf("selection_score = %v, want 1", resp.Keys[0].SelectionScore)
	}
}

func TestZenKeyDTO_jsonContractSecretFree(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	successAt := now.Add(-time.Hour)
	dto := zenKeyDTO{
		ID: "zk_x", Label: "demo", Prefix: "sk-dem…", Weight: 1, Enabled: true,
		Provider: "opencode", CreatedAt: now, Status: "active",
		TrafficPct: 100, CooldownRemainingSec: 0,
		HealthScore: 70, SelectionScore: 70,
		SuccessCount: 3, FailureCount: 1, ConsecutiveFailures: 0,
		LastErrorClass: "rate_limited", LastSuccessAt: &successAt,
		CooldownReason: "",
	}
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	// Required health keys present.
	for _, key := range []string{
		`"health_score"`, `"selection_score"`, `"success_count"`, `"failure_count"`,
		`"consecutive_failures"`, `"last_error_class"`, `"last_success_at"`,
		`"cooldown_reason"`, `"weight"`, `"traffic_pct"`, `"status"`,
	} {
		if !strings.Contains(body, key) {
			t.Fatalf("missing %s in %s", key, body)
		}
	}
	// Never serialize secrets / bodies / cookies.
	for _, banned := range []string{
		`"secret"`, `"key_ciphertext"`, `"auth_cookie"`, `"cookie"`,
		`"password"`, `"prompt"`, `"completion"`, `"authorization"`,
	} {
		if strings.Contains(strings.ToLower(body), banned) {
			t.Fatalf("banned field %s leaked in %s", banned, body)
		}
	}
	// omitempty: nil last_failure_at / health_updated_at absent
	if strings.Contains(body, `"last_failure_at"`) || strings.Contains(body, `"health_updated_at"`) {
		t.Fatalf("unexpected nil time fields: %s", body)
	}
}


func TestMapZenPoolSummary_usesDomainProbingWithoutKeyDTOs(t *testing.T) {
	sum := zenpool.PoolSummary{
		Total: 3, Enabled: 3, Healthy: 1, Cooled: 1, Benched: 0, Probing: 1, Disabled: 0,
		ByProvider: map[string]zenpool.ProviderSummary{
			"opencode": {Total: 3, Enabled: 3, Healthy: 1, Cooled: 1, Benched: 0, Probing: 1},
		},
	}
	// Overview path calls mapZenPoolSummary without key DTOs.
	dto := mapZenPoolSummary(sum)
	if dto.Probing != 1 {
		t.Fatalf("probing = %d, want 1 from domain summary", dto.Probing)
	}
	if dto.Attention != 2 { // cooled + probing
		t.Fatalf("attention = %d, want 2", dto.Attention)
	}
	if dto.ByProvider == nil || dto.ByProvider["opencode"].Probing != 1 {
		t.Fatalf("by_provider probing missing: %+v", dto.ByProvider)
	}
	if dto.ByProvider["opencode"].Attention != 2 {
		t.Fatalf("provider attention = %d, want 2", dto.ByProvider["opencode"].Attention)
	}
}
