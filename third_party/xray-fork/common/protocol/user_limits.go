package protocol

import (
	"sync"

	"golang.org/x/time/rate"
)

type runtimeLimiterState struct {
	mu          sync.Mutex
	uplinkBps   uint64
	downlinkBps uint64
	uplink      *rate.Limiter
	downlink    *rate.Limiter
	uplinkBurst int
	downBurst   int
}

// runtimeLimiters is scoped to one managed Xray process and keyed by the
// account email. A single account can be present in multiple inbounds (or be
// represented by more than one MemoryUser after a config update), so pointer
// identity would incorrectly create one bucket per inbound/connection. A
// process-local map deliberately does not aggregate the same account across
// separate XrayTool processes.
var runtimeLimiters sync.Map // string -> *runtimeLimiterState

type RuntimeRateLimiters struct {
	Uplink      *rate.Limiter
	Downlink    *rate.Limiter
	UplinkBurst int
	DownBurst   int
}

// RuntimeLimits returns the LayerX per-user runtime limits carried in memory.
// Zero values mean unlimited and preserve upstream behavior.
func (u *MemoryUser) RuntimeLimits() (bandwidthBps uint64, connLimit uint32) {
	if u == nil {
		return 0, 0
	}
	uplink := u.EffectiveUplinkLimitBps()
	downlink := u.EffectiveDownlinkLimitBps()
	if downlink > uplink {
		uplink = downlink
	}
	return uplink, u.ConnLimit
}

// RuntimeRateLimiters returns shared direction-specific buckets. All concurrent
// links for one account in this Xray process share a direction bucket, while
// uplink and downlink never consume each other's budget.
func (u *MemoryUser) RuntimeRateLimiters(newLimiter func(uint64) (*rate.Limiter, int)) RuntimeRateLimiters {
	if u == nil || u.Email == "" {
		return RuntimeRateLimiters{}
	}
	raw, ok := runtimeLimiters.Load(u.Email)
	if !ok {
		raw, _ = runtimeLimiters.LoadOrStore(u.Email, new(runtimeLimiterState))
	}
	state := raw.(*runtimeLimiterState)
	uplinkBps := u.EffectiveUplinkLimitBps()
	downlinkBps := u.EffectiveDownlinkLimitBps()
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.uplink == nil || state.uplinkBps != uplinkBps {
		state.uplink, state.uplinkBurst = newRuntimeLimiter(newLimiter, uplinkBps)
		state.uplinkBps = uplinkBps
	}
	if state.downlink == nil || state.downlinkBps != downlinkBps {
		state.downlink, state.downBurst = newRuntimeLimiter(newLimiter, downlinkBps)
		state.downlinkBps = downlinkBps
	}
	return RuntimeRateLimiters{
		Uplink:      state.uplink,
		Downlink:    state.downlink,
		UplinkBurst: state.uplinkBurst,
		DownBurst:   state.downBurst,
	}
}

// RuntimeRateLimiter is retained for older fork callers. New data paths must
// use RuntimeRateLimiters so the two directions stay independent.
func (u *MemoryUser) RuntimeRateLimiter(newLimiter func(uint64) (*rate.Limiter, int)) (*rate.Limiter, int) {
	limits := u.RuntimeRateLimiters(newLimiter)
	return limits.Uplink, limits.UplinkBurst
}

func newRuntimeLimiter(newLimiter func(uint64) (*rate.Limiter, int), bitsPerSecond uint64) (*rate.Limiter, int) {
	if bitsPerSecond == 0 {
		return nil, 0
	}
	return newLimiter(bitsPerSecondToRuntimeBytesPerSecond(bitsPerSecond))
}

func bitsPerSecondToRuntimeBytesPerSecond(bitsPerSecond uint64) uint64 {
	if bitsPerSecond == 0 {
		return 0
	}
	return (bitsPerSecond + 7) / 8
}

func (u *MemoryUser) ResetRuntimeLimiter() {
	if u == nil || u.Email == "" {
		return
	}
	runtimeLimiters.Delete(u.Email)
}
