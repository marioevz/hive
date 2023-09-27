package suite_reorg

import (
	"context"

	"github.com/ethereum/hive/hivesim"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

// Re-org testnet.
func (ts ReorgTestSpec) ExecutePostFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Run the base test spec execute function, this sends blobs and constructs the chain
	ts.BaseTestSpec.ExecutePostFork(t, ctx, testnet, env, config)

	// Wait the specified number of epochs to build separate chains before starting the last client
	t.Logf("INFO: Waiting %d epochs for running clients to build different chains for re-orgs", ts.EpochsToSync)
	testnet.WaitSlots(ctx, beacon.Slot(ts.EpochsToSync)*testnet.Spec().SLOTS_PER_EPOCH)

	t.Logf("INFO: Starting secondary clients")
	// Start the other clients
	for _, n := range testnet.Nodes {
		if !n.IsRunning() {
			if err := n.Start(); err != nil {
				t.Fatalf("FAIL: error starting node %s: %v", n.ClientNames(), err)
			}
		}
	}

	// Wait for all other clients to sync with a timeout of 1 epoch
	syncCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 1)
	defer cancel()
	if h, err := testnet.WaitForSync(syncCtx); err != nil {
		t.Fatalf("FAIL: error waiting for canonincal chain: %v", err)
	} else {
		t.Logf("INFO: all clients synced at head %s", h)
	}
}