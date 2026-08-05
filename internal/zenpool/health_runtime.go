package zenpool

import (
	"context"
	"time"
)

type healthRuntime struct {
	requestTimes []time.Time
	inflight     int
	probing      bool
	lastPersist  time.Time
}

func (service *Service) runtimeSnapshot(id KeyID, now time.Time) (requests, inflight int, probing bool) {
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	runtime := service.healthRuntime[id]
	if runtime == nil {
		return 0, 0, false
	}
	cutoff := now.Add(-healthLoadWindow)
	index := 0
	for index < len(runtime.requestTimes) && runtime.requestTimes[index].Before(cutoff) {
		index++
	}
	if index > 0 {
		runtime.requestTimes = append([]time.Time(nil), runtime.requestTimes[index:]...)
	}
	return len(runtime.requestTimes), runtime.inflight, runtime.probing
}

func (service *Service) noteAcquire(id KeyID, probing bool, now time.Time) {
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	if service.healthRuntime == nil {
		service.healthRuntime = make(map[KeyID]*healthRuntime)
	}
	runtime := service.healthRuntime[id]
	if runtime == nil {
		runtime = &healthRuntime{}
		service.healthRuntime[id] = runtime
	}
	runtime.requestTimes = append(runtime.requestTimes, now)
	runtime.inflight++
	if probing {
		runtime.probing = true
	}
}

func (service *Service) noteRelease(id KeyID) {
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	if runtime := service.healthRuntime[id]; runtime != nil {
		if runtime.inflight > 0 {
			runtime.inflight--
		}
		runtime.probing = false
	}
}

func (service *Service) shouldPersistSuccess(id KeyID, now time.Time) bool {
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	runtime := service.healthRuntime[id]
	if runtime == nil {
		runtime = &healthRuntime{}
		service.healthRuntime[id] = runtime
	}
	if runtime.lastPersist.IsZero() || now.Sub(runtime.lastPersist) >= 30*time.Second {
		runtime.lastPersist = now
		return true
	}
	return false
}

func (service *Service) shareCapExceeded(id KeyID, candidates []KeyID, now time.Time) bool {
	if len(candidates) <= 1 {
		return false
	}
	total, selected := 0, 0
	for _, candidate := range candidates {
		requests, _, _ := service.runtimeSnapshot(candidate, now)
		total += requests
		if candidate == id {
			selected = requests
		}
	}
	if total < 10 {
		return false
	}
	return float64(selected+1)/float64(total+1) > maxSelectionShare
}

// overlayDirtyHealth returns an in-memory unflushed health snapshot when present.
// Callers still Normalize/decay as needed; this never invents secrets or bodies.
func (service *Service) overlayDirtyHealth(id KeyID, health Health) Health {
	if service == nil || id == "" {
		return health
	}
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	if dirty, ok := service.healthDirty[id]; ok {
		return dirty
	}
	return health
}

func (service *Service) rememberDirtyHealth(id KeyID, health Health) {
	if service == nil || id == "" {
		return
	}
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	if service.healthDirty == nil {
		service.healthDirty = make(map[KeyID]Health)
	}
	service.healthDirty[id] = health
}

func (service *Service) clearDirtyHealth(id KeyID) {
	if service == nil || id == "" {
		return
	}
	service.healthMu.Lock()
	defer service.healthMu.Unlock()
	delete(service.healthDirty, id)
}

// persistHealth writes secret-free health, clearing dirty on success and keeping dirty on failure.
func (service *Service) persistHealth(ctx context.Context, health Health) {
	if service == nil || health.KeyID == "" {
		return
	}
	if err := service.store.UpsertHealth(ctx, health); err != nil {
		service.rememberDirtyHealth(health.KeyID, health)
		return
	}
	service.clearDirtyHealth(health.KeyID)
}
