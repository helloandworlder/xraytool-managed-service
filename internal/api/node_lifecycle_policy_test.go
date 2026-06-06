package api

import "testing"

func TestNodeLifecycleAllowsDeactivateDuringDraining(t *testing.T) {
	if !nodeLifecycleAllowsMutation("DRAINING", "POST", "/api/orders/42/deactivate") {
		t.Fatalf("expected deactivate to be allowed during DRAINING")
	}
}

func TestNodeLifecycleBlocksCreateOrderDuringDraining(t *testing.T) {
	if nodeLifecycleAllowsMutation("DRAINING", "POST", "/api/orders") {
		t.Fatalf("expected create order to be blocked during DRAINING")
	}
}

func TestNodeLifecycleAllowsLifecycleUpdateDuringDecommissioned(t *testing.T) {
	if !nodeLifecycleAllowsMutation("DECOMMISSIONED", "PUT", "/api/node-lifecycle") {
		t.Fatalf("expected lifecycle update route to stay allowed during DECOMMISSIONED")
	}
}

func TestNodeLifecycleAllowsBackupRestoreDuringRetired(t *testing.T) {
	if !nodeLifecycleAllowsMutation("RETIRED", "POST", "/api/db/restore") {
		t.Fatalf("expected admin backup restore to stay allowed during RETIRED")
	}
}
