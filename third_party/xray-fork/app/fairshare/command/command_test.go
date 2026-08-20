package command

import (
	"context"
	"testing"

	"github.com/xtls/xray-core/common/protocol"
)

func TestSetNodeBandwidthDefaultsZeroDirectionalValuesTo30Mbps(t *testing.T) {
	scheduler := protocol.FairScheduler()
	oldUplink, oldDownlink := scheduler.UplinkBps(), scheduler.DownlinkBps()
	t.Cleanup(func() { scheduler.SetNodeBandwidthDirections(oldUplink, oldDownlink) })

	_, err := (&fairShareServer{}).SetNodeBandwidth(context.Background(), &SetNodeBandwidthRequest{})
	if err != nil {
		t.Fatalf("SetNodeBandwidth returned error: %v", err)
	}
	if got := scheduler.UplinkBps(); got != protocol.DefaultLimitBytesPerSecond {
		t.Fatalf("zero uplink request = %d bytes/s, want %d", got, protocol.DefaultLimitBytesPerSecond)
	}
	if got := scheduler.DownlinkBps(); got != protocol.DefaultLimitBytesPerSecond {
		t.Fatalf("zero downlink request = %d bytes/s, want %d", got, protocol.DefaultLimitBytesPerSecond)
	}
}
