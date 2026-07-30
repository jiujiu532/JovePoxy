package zenpool_test

import (
	"math"
	"testing"
	"time"

	"jovepoxy/internal/zenpool"
)

func TestTrafficShares_singleKey(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	list := []zenpool.Metadata{
		{ID: "a", Weight: 1, Enabled: true, Provider: zenpool.ProviderOpenCode},
	}
	shares := zenpool.TrafficShares(list, now)
	if len(shares) != 1 || math.Abs(shares[0]-100) > 0.001 {
		t.Fatalf("shares = %v, want [100]", shares)
	}
}

func TestTrafficShares_weighted1to3(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	list := []zenpool.Metadata{
		{ID: "a", Weight: 1, Enabled: true},
		{ID: "b", Weight: 3, Enabled: true},
	}
	shares := zenpool.TrafficShares(list, now)
	if math.Abs(shares[0]-25) > 0.001 || math.Abs(shares[1]-75) > 0.001 {
		t.Fatalf("shares = %v, want [25, 75]", shares)
	}
	if math.Abs(shares[0]+shares[1]-100) > 0.001 {
		t.Fatalf("sum = %v, want 100", shares[0]+shares[1])
	}
}

func TestTrafficShares_allCooling(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Minute)
	list := []zenpool.Metadata{
		{ID: "a", Weight: 1, Enabled: true, CooldownUntil: &until},
		{ID: "b", Weight: 2, Enabled: true, CooldownUntil: &until},
	}
	shares := zenpool.TrafficShares(list, now)
	if shares[0] != 0 || shares[1] != 0 {
		t.Fatalf("shares = %v, want zeros when all cooling", shares)
	}
}

func TestTrafficShares_allDisabled(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	list := []zenpool.Metadata{
		{ID: "a", Weight: 1, Enabled: false},
		{ID: "b", Weight: 5, Enabled: false},
	}
	shares := zenpool.TrafficShares(list, now)
	if shares[0] != 0 || shares[1] != 0 {
		t.Fatalf("shares = %v, want zeros when all disabled", shares)
	}
}

func TestTrafficShares_weightZeroExcluded(t *testing.T) {
	// Align with Acquire: weight <= 0 is never selected.
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	list := []zenpool.Metadata{
		{ID: "a", Weight: 0, Enabled: true},
		{ID: "b", Weight: 2, Enabled: true},
	}
	shares := zenpool.TrafficShares(list, now)
	if shares[0] != 0 || math.Abs(shares[1]-100) > 0.001 {
		t.Fatalf("shares = %v, want [0, 100]", shares)
	}
}

func TestTrafficShares_mixedIneligible(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	until := now.Add(30 * time.Second)
	past := now.Add(-time.Minute)
	list := []zenpool.Metadata{
		{ID: "disabled", Weight: 10, Enabled: false},
		{ID: "cooling", Weight: 10, Enabled: true, CooldownUntil: &until},
		{ID: "past", Weight: 1, Enabled: true, CooldownUntil: &past},
		{ID: "active", Weight: 3, Enabled: true},
	}
	shares := zenpool.TrafficShares(list, now)
	if shares[0] != 0 || shares[1] != 0 {
		t.Fatalf("ineligible must be 0: %v", shares)
	}
	if math.Abs(shares[2]-25) > 0.001 || math.Abs(shares[3]-75) > 0.001 {
		t.Fatalf("shares = %v, want past=25 active=75", shares)
	}
}

func TestDeriveStatus_andRemaining(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	until := now.Add(90 * time.Second)
	past := now.Add(-10 * time.Second)

	if got := zenpool.DeriveStatus(zenpool.Metadata{Enabled: false}, now); got != zenpool.StatusDisabled {
		t.Fatalf("disabled status = %s", got)
	}
	if got := zenpool.DeriveStatus(zenpool.Metadata{Enabled: true, CooldownUntil: &until}, now); got != zenpool.StatusCooling {
		t.Fatalf("cooling status = %s", got)
	}
	if got := zenpool.DeriveStatus(zenpool.Metadata{Enabled: true, CooldownUntil: &past}, now); got != zenpool.StatusActive {
		t.Fatalf("past cooldown should be active, got %s", got)
	}
	if got := zenpool.DeriveStatus(zenpool.Metadata{Enabled: true}, now); got != zenpool.StatusActive {
		t.Fatalf("active status = %s", got)
	}

	if sec := zenpool.CooldownRemainingSec(zenpool.Metadata{CooldownUntil: &until}, now); sec != 90 {
		t.Fatalf("remaining = %d, want 90", sec)
	}
	if sec := zenpool.CooldownRemainingSec(zenpool.Metadata{CooldownUntil: &past}, now); sec != 0 {
		t.Fatalf("past remaining = %d, want 0", sec)
	}
	if sec := zenpool.CooldownRemainingSec(zenpool.Metadata{}, now); sec != 0 {
		t.Fatalf("nil remaining = %d, want 0", sec)
	}
}

func TestSummarize_byProvider(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	until := now.Add(time.Minute)
	list := []zenpool.Metadata{
		{ID: "1", Enabled: true, Provider: zenpool.ProviderOpenCode},
		{ID: "2", Enabled: true, Provider: zenpool.ProviderOpenCode, CooldownUntil: &until},
		{ID: "3", Enabled: false, Provider: zenpool.ProviderOpenCode},
		{ID: "4", Enabled: true, Provider: zenpool.ProviderOllama},
		{ID: "5", Enabled: false, Provider: zenpool.ProviderOllama},
	}
	sum := zenpool.Summarize(list, now)
	if sum.Total != 5 || sum.Enabled != 3 || sum.Healthy != 2 || sum.Cooled != 1 || sum.Disabled != 2 {
		t.Fatalf("summary = %+v", sum)
	}
	oc := sum.ByProvider[string(zenpool.ProviderOpenCode)]
	if oc.Total != 3 || oc.Healthy != 1 || oc.Cooled != 1 || oc.Disabled != 1 {
		t.Fatalf("opencode = %+v", oc)
	}
	ol := sum.ByProvider[string(zenpool.ProviderOllama)]
	if ol.Total != 2 || ol.Healthy != 1 || ol.Disabled != 1 || ol.Cooled != 0 {
		t.Fatalf("ollama = %+v", ol)
	}
}

func TestDeriveViews_trafficPerProvider(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	list := []zenpool.Metadata{
		{ID: "oc1", Weight: 1, Enabled: true, Provider: zenpool.ProviderOpenCode},
		{ID: "oc2", Weight: 1, Enabled: true, Provider: zenpool.ProviderOpenCode},
		{ID: "ol1", Weight: 1, Enabled: true, Provider: zenpool.ProviderOllama},
	}
	views := zenpool.DeriveViews(list, now)
	if math.Abs(views[0].TrafficPct-50) > 0.001 || math.Abs(views[1].TrafficPct-50) > 0.001 {
		t.Fatalf("opencode shares = %v %v, want 50/50", views[0].TrafficPct, views[1].TrafficPct)
	}
	if math.Abs(views[2].TrafficPct-100) > 0.001 {
		t.Fatalf("ollama share = %v, want 100", views[2].TrafficPct)
	}
	if views[0].Status != zenpool.StatusActive {
		t.Fatalf("status = %s", views[0].Status)
	}
}
