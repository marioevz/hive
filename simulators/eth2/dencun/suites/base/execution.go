package suite_base

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/dencun/helper"

	beacon_verification "github.com/ethereum/hive/simulators/eth2/common/spoofing/beacon"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	engine_helper "github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var Deneb string = "deneb"

var (
	normalTxAccounts = globals.TestAccounts[:len(globals.TestAccounts)/2]
	blobTxAccounts   = globals.TestAccounts[len(globals.TestAccounts)/2:]
)

// Generic Deneb test routine, capable of running most of the test
// scenarios.
func (ts BaseTestSpec) ExecutePreFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Setup the transaction spammers, both normal and blob transactions
	normalTxSpammer := helper.TransactionSpammer{
		T:                        t,
		Name:                     "normal",
		Recipient:                &CodeContractAddress,
		ExecutionClients:         testnet.ExecutionClients().Running(),
		Accounts:                 normalTxAccounts,
		TransactionType:          engine_helper.DynamicFeeTxOnly,
		TransactionsPerIteration: 40,
		SecondsBetweenIterations: int(testnet.Spec().SECONDS_PER_SLOT),
	}

	// Start sending normal transactions from dedicated accounts
	go normalTxSpammer.Run(ctx)

	// Add verification of Beacon->Execution Engine API calls to the proxies
	chainconfig := testnet.ExecutionGenesis().Config
	// NewPayloadV2 expires at CancunTime: if a client sends a payload with
	// a timestamp greater than CancunTime, and it's using NewPayloadV2, it
	// must result in test failure.
	newPayloadV2ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineNewPayloadV2,
		*chainconfig.CancunTime,
	)
	// ForkchoiceUpdatedV2 expires at CancunTime: if a client sends a payload with
	// a timestamp greater than CancunTime, and it's using ForkchoiceUpdatedV2, it
	// must result in test failure.
	forkchoiceUpdatedV2ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineForkchoiceUpdatedV2,
		*chainconfig.CancunTime,
	)
	for _, e := range testnet.ExecutionClients() {
		newPayloadV2ExpireVerifier.AddToProxy(e.Proxy())
		forkchoiceUpdatedV2ExpireVerifier.AddToProxy(e.Proxy())
	}

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)
}

func (ts BaseTestSpec) ExecutePostFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Wait one more slot before continuing
	testnet.WaitSlots(ctx, 1)

	// Start sending blob transactions from dedicated accounts
	blobTxSpammer := helper.TransactionSpammer{
		T:                        t,
		Name:                     "blobs",
		Recipient:                &CodeContractAddress,
		ExecutionClients:         testnet.ExecutionClients().Running(),
		Accounts:                 blobTxAccounts,
		TransactionType:          engine_helper.BlobTxOnly,
		TransactionsPerIteration: 2,
		SecondsBetweenIterations: int(testnet.Spec().SECONDS_PER_SLOT),
	}

	go blobTxSpammer.Run(ctx)

	// Send BLSToExecutionChanges messages during Deneb for all validators on BLS credentials
	allValidators, err := helper.ValidatorsFromBeaconState(
		testnet.GenesisBeaconState(),
		*testnet.Spec().Spec,
		env.Keys,
	)
	if err != nil {
		t.Fatalf("FAIL: Error parsing validators from beacon state")
	}
	nonWithdrawableValidators := allValidators.NonWithdrawable()
	if len(nonWithdrawableValidators) > 0 {
		beaconClients := testnet.BeaconClients().Running()
		for i := 0; i < len(nonWithdrawableValidators); i++ {
			b := beaconClients[i%len(beaconClients)]
			v := nonWithdrawableValidators[i]
			if err := v.SignSendBLSToExecutionChange(
				ctx,
				b,
				common.Address{byte(v.Index + 0x100)},
				helper.ComputeBLSToExecutionDomain(testnet),
			); err != nil {
				t.Fatalf(
					"FAIL: Unable to submit bls-to-execution changes: %v",
					err,
				)
			}
		}
	} else {
		t.Logf("INFO: no validators left on BLS credentials")
	}
}

func (ts BaseTestSpec) ExecutePostForkWait(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	if ts.EpochsAfterFork != 0 {
		// Wait for the specified number of epochs after fork
		var err error
		if ts.MaxMissedSlots > 0 {
			err = testnet.WaitSlotsWithMaxMissedSlots(ctx, beacon.Slot(ts.EpochsAfterFork)*testnet.Spec().SLOTS_PER_EPOCH, ts.MaxMissedSlots)
		} else {
			err = testnet.WaitSlots(ctx, beacon.Slot(ts.EpochsAfterFork)*testnet.Spec().SLOTS_PER_EPOCH)
		}
		if err != nil {
			t.Fatalf("FAIL: error waiting for %d epochs after fork: %v", ts.EpochsAfterFork, err)
		}
	}
	if ts.WaitForFinality {
		finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 5)
		defer cancel()
		if _, err := testnet.WaitForCurrentEpochFinalization(finalityCtx); err != nil {
			t.Fatalf("FAIL: error waiting for epoch finalization: %v", err)
		}
	}
}

func (ts BaseTestSpec) Verify(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Check all clients are on the same head
	if err := testnet.VerifyELHeads(ctx); err != nil {
		t.Fatalf("FAIL: error verifying execution layer heads: %v", err)
	}

	// Check for optimistic sync
	for i, n := range testnet.Nodes.Running() {
		bc := n.BeaconClient
		if op, err := bc.BlockIsOptimistic(ctx, eth2api.BlockHead); op {
			t.Fatalf(
				"FAIL: client %d (%s) is optimistic, it should be synced.",
				i,
				n.ClientNames(),
			)
		} else if err != nil {
			t.Fatalf("FAIL: error querying optimistic state on client %d (%s): %v", i, n.ClientNames(), err)
		}
	}

	// Verify all clients agree on blobs for each slot
	if blobCount, err := testnet.VerifyBlobs(ctx, tn.LastestSlotByHead{}); err != nil {
		t.Fatalf("FAIL: error verifying blobs: %v", err)
	} else if blobCount == 0 {
		t.Fatalf("FAIL: no blobs were included in the chain")
	} else {
		t.Logf("INFO: %d blobs were included in the chain", blobCount)
	}
}
