package protocol_test

import (
	"testing"
	"time"

	"github.com/xtls/xray-core/common/buf"
	. "github.com/xtls/xray-core/common/protocol"
	"golang.org/x/time/rate"
)

func TestRuntimeRateLimiterSharedPerUser(t *testing.T) {
	user := &MemoryUser{
		Email:        "alice@example.test",
		BandwidthBps: uint64(buf.Size),
	}
	defer user.ResetRuntimeLimiter()

	limiter1, burst1 := user.RuntimeRateLimiter(buf.NewRateLimiter)
	limiter2, burst2 := user.RuntimeRateLimiter(buf.NewRateLimiter)
	if limiter1 == nil || limiter2 == nil {
		t.Fatal("expected limiter for bandwidth-limited user")
	}
	if limiter1 != limiter2 {
		t.Fatal("expected one shared limiter per MemoryUser")
	}
	if burst1 != burst2 {
		t.Fatalf("expected stable burst, got %d and %d", burst1, burst2)
	}
	if !limiter1.AllowN(time.Now(), burst1) {
		t.Fatal("expected initial burst to be available")
	}
	if limiter2.Allow() {
		t.Fatal("second access should consume the same exhausted bucket")
	}
}

func TestRuntimeRateLimiterIsolatedBetweenUsers(t *testing.T) {
	alice := &MemoryUser{
		Email:        "alice@example.test",
		BandwidthBps: uint64(buf.Size),
	}
	bob := &MemoryUser{
		Email:        "bob@example.test",
		BandwidthBps: uint64(buf.Size),
	}
	defer alice.ResetRuntimeLimiter()
	defer bob.ResetRuntimeLimiter()

	aliceLimiter, aliceBurst := alice.RuntimeRateLimiter(buf.NewRateLimiter)
	bobLimiter, bobBurst := bob.RuntimeRateLimiter(buf.NewRateLimiter)
	if aliceLimiter == nil || bobLimiter == nil {
		t.Fatal("expected limiters for both users")
	}
	if aliceLimiter == bobLimiter {
		t.Fatal("different users must not share limiter state")
	}
	if !aliceLimiter.AllowN(time.Now(), aliceBurst) {
		t.Fatal("expected alice initial burst to be available")
	}
	if !bobLimiter.AllowN(time.Now(), bobBurst) {
		t.Fatal("bob bucket should be independent from alice")
	}
}

func TestRuntimeRateLimitersSharedAcrossMemoryUsersForSameAccount(t *testing.T) {
	first := &MemoryUser{
		Email:            "alice@example.test",
		UplinkLimitBps:   uint64(buf.Size * 8),
		DownlinkLimitBps: uint64(buf.Size * 16),
	}
	second := &MemoryUser{
		Email:            "alice@example.test",
		UplinkLimitBps:   uint64(buf.Size * 8),
		DownlinkLimitBps: uint64(buf.Size * 16),
	}
	defer first.ResetRuntimeLimiter()

	firstLimits := first.RuntimeRateLimiters(buf.NewRateLimiter)
	secondLimits := second.RuntimeRateLimiters(buf.NewRateLimiter)
	if firstLimits.Uplink != secondLimits.Uplink || firstLimits.Downlink != secondLimits.Downlink {
		t.Fatal("same account across inbounds must share direction buckets")
	}
	if !firstLimits.Uplink.AllowN(time.Now(), firstLimits.UplinkBurst) {
		t.Fatal("expected initial account uplink burst to be available")
	}
	if secondLimits.Uplink.Allow() {
		t.Fatal("second MemoryUser for the same account must consume the shared uplink bucket")
	}
}

func TestRuntimeRateLimiterConvertsBitsToBytes(t *testing.T) {
	user := &MemoryUser{
		Email:        "alice@example.test",
		BandwidthBps: 8_000_000,
	}
	var got uint64
	_, _ = user.RuntimeRateLimiter(func(bytesPerSecond uint64) (*rate.Limiter, int) {
		got = bytesPerSecond
		return buf.NewRateLimiter(bytesPerSecond)
	})
	if got != 1_000_000 {
		t.Fatalf("runtime limiter bytes/sec = %d, want 1000000", got)
	}
}

func TestRuntimeRateLimitersSeparateDirectionsAndSharePerUser(t *testing.T) {
	user := &MemoryUser{
		Email:            "alice@example.test",
		UplinkLimitBps:   8_000_000,
		DownlinkLimitBps: 16_000_000,
	}
	defer user.ResetRuntimeLimiter()

	first := user.RuntimeRateLimiters(buf.NewRateLimiter)
	second := user.RuntimeRateLimiters(buf.NewRateLimiter)
	if first.Uplink == nil || first.Downlink == nil {
		t.Fatal("expected both direction limiters")
	}
	if first.Uplink == first.Downlink {
		t.Fatal("uplink and downlink must use independent buckets")
	}
	if first.Uplink != second.Uplink || first.Downlink != second.Downlink {
		t.Fatal("concurrent links for one user must share each direction bucket")
	}
	if !first.Uplink.AllowN(time.Now(), first.UplinkBurst) {
		t.Fatal("expected uplink burst to be available")
	}
	if !first.Downlink.AllowN(time.Now(), first.DownBurst) {
		t.Fatal("downlink must not consume uplink budget")
	}

	bob := &MemoryUser{Email: "bob@example.test", UplinkLimitBps: 8_000_000, DownlinkLimitBps: 16_000_000}
	defer bob.ResetRuntimeLimiter()
	bobLimits := bob.RuntimeRateLimiters(buf.NewRateLimiter)
	if bobLimits.Uplink == first.Uplink || bobLimits.Downlink == first.Downlink {
		t.Fatal("different users must not share direction buckets")
	}
}

func TestResetRuntimeLimiter(t *testing.T) {
	user := &MemoryUser{
		Email:        "alice@example.test",
		BandwidthBps: uint64(buf.Size),
	}

	limiter1, _ := user.RuntimeRateLimiter(buf.NewRateLimiter)
	user.ResetRuntimeLimiter()
	limiter2, _ := user.RuntimeRateLimiter(buf.NewRateLimiter)
	if limiter1 == nil || limiter2 == nil {
		t.Fatal("expected limiter before and after reset")
	}
	if limiter1 == limiter2 {
		t.Fatal("expected reset to drop old limiter state")
	}
}
