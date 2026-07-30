package quota

// WindowNarrative is the derived burn/headroom story for one quota window.
// Burn/days fields stay nil unless the caller has a scientifically sound rate.
type WindowNarrative struct {
	UsedPct     *float64
	HeadroomPct *float64
	BurnPerDay  *float64
	DaysToEmpty *float64
}

// DeriveWindowNarrative computes used%/headroom% from a window snapshot.
// total<=0 → both pct nil (do not invent). used is clamped to [0, total];
// negative remaining falls back to total-used so used>total cannot invent headroom.
// Burn and days-to-empty stay nil: without elapsed window length we refuse fake burn.
func DeriveWindowNarrative(used, remaining, total float64) WindowNarrative {
	if total <= 0 {
		return WindowNarrative{}
	}
	clampedUsed := used
	if clampedUsed < 0 {
		clampedUsed = 0
	}
	if clampedUsed > total {
		clampedUsed = total
	}
	clampedRemaining := remaining
	if remaining < 0 {
		clampedRemaining = total - clampedUsed
	}
	if clampedRemaining < 0 {
		clampedRemaining = 0
	}
	if clampedRemaining > total {
		clampedRemaining = total
	}
	usedPct := round1(clampedUsed / total * 100)
	headroomPct := round1(clampedRemaining / total * 100)
	return WindowNarrative{
		UsedPct:     &usedPct,
		HeadroomPct: &headroomPct,
	}
}

// PickPrimaryNarrative selects the busiest successful window as account-level story.
// Empty windows → note "sample_insufficient"; never invents burn.
func PickPrimaryNarrative(windows []Window) (label string, n WindowNarrative, note string) {
	if len(windows) == 0 {
		return "", WindowNarrative{}, "sample_insufficient"
	}
	bestIdx := -1
	var bestUsed float64
	for i, window := range windows {
		derived := DeriveWindowNarrative(window.Used, window.Remaining, window.Total)
		if derived.UsedPct == nil {
			continue
		}
		if bestIdx < 0 || *derived.UsedPct > bestUsed {
			bestIdx = i
			bestUsed = *derived.UsedPct
		}
	}
	if bestIdx < 0 {
		return "", WindowNarrative{}, "sample_insufficient"
	}
	window := windows[bestIdx]
	return window.Label, DeriveWindowNarrative(window.Used, window.Remaining, window.Total), ""
}

// DaysToEmptyFromBurn returns remaining/burn when both are positive; otherwise nil.
func DaysToEmptyFromBurn(remaining, burnPerDay float64) *float64 {
	if remaining < 0 || burnPerDay <= 0 {
		return nil
	}
	days := round1(remaining / burnPerDay)
	return &days
}

func round1(value float64) float64 {
	return float64(int(value*10+0.5)) / 10
}
