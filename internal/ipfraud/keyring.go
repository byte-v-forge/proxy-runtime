package ipfraud

import (
	"errors"
	"sync"
	"time"
)

var errQuotaExhausted = errors.New("IP fraud provider quota exhausted")
var errProviderUnavailable = errors.New("IP fraud provider unavailable")

type quotaError struct {
	retryAfter time.Duration
	cause      error
}

func (e quotaError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return errQuotaExhausted.Error()
}

func (e quotaError) Unwrap() error {
	return errQuotaExhausted
}

type keyState struct {
	value            string
	unavailableUntil time.Time
}

type keyRing struct {
	mu       sync.Mutex
	keys     []keyState
	next     int
	cooldown time.Duration
}

func newKeyRing(keys []string, cooldown time.Duration) *keyRing {
	if cooldown <= 0 {
		cooldown = 24 * time.Hour
	}
	states := make([]keyState, 0, len(keys))
	seen := map[string]bool{}
	for _, key := range keys {
		if seen[key] {
			continue
		}
		seen[key] = true
		states = append(states, keyState{value: key})
	}
	if len(states) == 0 {
		states = append(states, keyState{})
	}
	return &keyRing{keys: states, cooldown: cooldown}
}

func (r *keyRing) nextAvailable(now time.Time) (int, string, bool) {
	if r == nil || len(r.keys) == 0 {
		return 0, "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for attempt := 0; attempt < len(r.keys); attempt++ {
		index := (r.next + attempt) % len(r.keys)
		if r.keys[index].unavailableUntil.After(now) {
			continue
		}
		r.next = (index + 1) % len(r.keys)
		return index, r.keys[index].value, true
	}
	return 0, "", false
}

func (r *keyRing) size() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.keys)
}

func (r *keyRing) markUnavailable(index int, duration time.Duration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < 0 || index >= len(r.keys) {
		return
	}
	if duration <= 0 {
		duration = r.cooldown
	}
	r.keys[index].unavailableUntil = time.Now().Add(duration)
}
