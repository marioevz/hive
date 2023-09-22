package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	mock_builder "github.com/marioevz/mock-builder/mock"

	"github.com/ethereum/hive/simulators/eth2/common/clients"
	beacon_verification "github.com/ethereum/hive/simulators/eth2/common/spoofing/beacon"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var Deneb string = "deneb"

// Generic Deneb test routine, capable of running most of the test
// scenarios.
func (ts BaseDencunTestSpec) Execute(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
	n []clients.NodeDefinition,
) {
	var (
		normalTxAccounts = globals.TestAccounts[:len(globals.TestAccounts)/2]
		blobTxAccounts   = globals.TestAccounts[len(globals.TestAccounts)/2:]
		err              error
	)

	// Setup the transaction spammers, both normal and blob transactions
	var (
		normalTxSpammer = TransactionSpammer{
			T:                        t,
			ExecutionClients:         testnet.ExecutionClients().Running(),
			Accounts:                 normalTxAccounts,
			TransactionType:          helper.DynamicFeeTxOnly,
			TransactionsPerIteration: 40,
			SecondsBetweenIterations: int(testnet.Spec().SECONDS_PER_SLOT),
		}
		blobTxSpammer = TransactionSpammer{
			T:                        t,
			ExecutionClients:         testnet.ExecutionClients().Running(),
			Accounts:                 blobTxAccounts,
			TransactionType:          helper.BlobTxOnly,
			TransactionsPerIteration: 2,
			SecondsBetweenIterations: int(testnet.Spec().SECONDS_PER_SLOT),
		}
	)

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

	// Wait for Deneb
	if config.DenebForkEpoch.Uint64() > 0 {
		slotsUntilDeneb := beacon.Slot(
			config.DenebForkEpoch.Uint64(),
		)*testnet.Spec().SLOTS_PER_EPOCH + 4
		timeoutCtx, cancel := testnet.Spec().SlotTimeoutContext(ctx, slotsUntilDeneb)
		defer cancel()
		if err = testnet.WaitForFork(timeoutCtx, Deneb); err != nil {
			t.Fatalf("FAIL: error waiting for deneb: %v", err)
		}
	}

	// Start sending blob transactions from dedicated accounts
	go blobTxSpammer.Run(ctx)

	// Check that the builder was working properly until now
	if config.EnableBuilders {
		for i, b := range testnet.BeaconClients().Running() {
			builder, ok := b.Builder.(*mock_builder.MockBuilder)
			if !ok {
				t.Fatalf(
					"FAIL: client %d (%s) is not a mock builder",
					i,
					b.ClientName(),
				)
			}
			if builder.GetBuiltPayloadsCount() == 0 {
				t.Fatalf("FAIL: builder %d did not build any payloads", i)
			}
			if builder.GetSignedBeaconBlockCount() == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce any signed beacon blocks",
					i,
				)
			}
		}
	}

	// Send BLSToExecutionChanges messages during Deneb for all validators on BLS credentials
	allValidators, err := ValidatorsFromBeaconState(
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
				ComputeBLSToExecutionDomain(testnet),
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

	// Lastly check all clients are on the same head
	if err = testnet.VerifyELHeads(ctx); err != nil {
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

	if ts.WaitForFinality {
		finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 5)
		defer cancel()
		if _, err := testnet.WaitForCurrentEpochFinalization(finalityCtx); err != nil {
			t.Fatalf("FAIL: error waiting for epoch finalization: %v", err)
		}
	}
}

// Builder testnet.
func (ts BuilderDenebTestSpec) Execute(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
	n []clients.NodeDefinition,
) {
	// Run the base test spec execute function
	ts.BaseDencunTestSpec.Execute(t, ctx, testnet, env, config, n)

	// Verify any modified payloads did not make it into the
	// canonical chain
	if !ts.ErrorOnHeaderRequest && !ts.ErrorOnPayloadReveal &&
		!ts.InvalidPayloadVersion &&
		ts.InvalidatePayload == "" &&
		ts.InvalidatePayloadAttributes == "" {
		// Simply verify that builder's deneb payloads were included in the
		// canonical chain
		t.Logf("INFO: Verifying builder payloads were included in the canonical chain")
		for i, n := range testnet.Nodes.Running() {
			b, ok := n.BeaconClient.Builder.(*mock_builder.MockBuilder)
			if !ok {
				t.Fatalf(
					"FAIL: client %d (%s) is not a mock builder",
					i,
					n.BeaconClient.ClientName(),
				)
			}
			ec := n.ExecutionClient
			includedPayloads := 0
			includedPayloadsWithBlobs := 0
			for _, p := range b.GetBuiltPayloads() {
				p, _, err := p.FullPayload().ToExecutableData()
				if err != nil {
					t.Fatalf(
						"FAIL: error converting payload to executable data: %v",
						err,
					)
				}
				if testnet.ExecutionGenesis().Config.CancunTime == nil {
					t.Fatalf("FAIL: Cancun time is nil")
				}
				if p.Timestamp >= *testnet.ExecutionGenesis().Config.CancunTime {
					if h, err := ec.HeaderByNumber(ctx, big.NewInt(int64(p.Number))); err != nil {
						t.Fatalf(
							"FAIL: error getting execution header from node %d: %v",
							i,
							err,
						)
					} else if h != nil {
						hash := h.Hash()
						if bytes.Equal(hash[:], p.BlockHash[:]) {
							includedPayloads++
							// On deneb we also need to make sure at least one payload has blobs
							if p.BlobGasUsed != nil && *p.BlobGasUsed > 0 {
								includedPayloadsWithBlobs++
							}
						}
					}
				}
			}
			if includedPayloads == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce deneb payloads included in the canonical chain",
					i,
				)
			}
			if includedPayloadsWithBlobs == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce deneb payloads with blobs included in the canonical chain",
					i,
				)
			}
		}
	} else if ts.InvalidatePayloadAttributes != "" {
		// TODO: INVALIDATE_ATTR_BEACON_ROOT cannot be detected by the consensus client
		//       on a blinded payload
		t.Logf("INFO: Verifying builder payloads were NOT included in the canonical chain")
		for i, n := range testnet.VerificationNodes().Running() {
			b, ok := n.BeaconClient.Builder.(*mock_builder.MockBuilder)
			if !ok {
				t.Fatalf(
					"FAIL: client %d (%s) is not a mock builder",
					i,
					n.BeaconClient.ClientName(),
				)
			}
			modifiedPayloads := b.GetModifiedPayloads()
			if len(modifiedPayloads) == 0 {
				t.Fatalf("FAIL: No payloads were modified by builder %d", i)
			}
			for _, p := range modifiedPayloads {
				p, _, err := p.ToExecutableData()
				if err != nil {
					t.Fatalf(
						"FAIL: error converting payload to executable data: %v",
						err,
					)
				}
				for _, ec := range testnet.ExecutionClients().Running() {
					b, err := ec.BlockByNumber(
						ctx,
						big.NewInt(int64(p.Number)),
					)
					if err != nil {
						t.Fatalf(
							"FAIL: Error getting execution block %d: %v",
							p.Number,
							err,
						)
					}
					h := b.Hash()
					if bytes.Equal(h[:], p.BlockHash[:]) {
						t.Fatalf(
							"FAIL: Modified payload included in canonical chain: %d (%s)",
							p.Number,
							p.BlockHash,
						)
					}
				}
			}
			t.Logf(
				"INFO: No modified payloads were included in canonical chain of node %d",
				i,
			)
		}
	}

	// Count, print and verify missed slots
	if count, err := testnet.BeaconClients().Running()[0].GetFilledSlotsCountPerEpoch(ctx); err != nil {
		t.Fatalf("FAIL: unable to obtain slot count per epoch: %v", err)
	} else {
		for ep, slots := range count {
			t.Logf("INFO: Epoch %d, filled slots=%d", ep, slots)
		}

		var max_missed_slots uint64 = 0
		if ts.ErrorOnHeaderRequest || ts.InvalidPayloadVersion || ts.InvalidatePayloadAttributes != "" {
			// These errors should be caught by the CL client when the built blinded
			// payload is received. Hence, a low number of missed slots is expected.
			max_missed_slots = 1
		} else {
			// All other errors cannot be caught by the CL client until the
			// payload is revealed, and the beacon block had been signed.
			// Hence, a high number of missed slots is expected because the
			// circuit breaker is a mechanism that only kicks in after many
			// missed slots.
			max_missed_slots = 10
		}

		denebEpoch := beacon.Epoch(config.DenebForkEpoch.Uint64())

		if count[denebEpoch] < uint64(testnet.Spec().SLOTS_PER_EPOCH)-max_missed_slots {
			t.Fatalf(
				"FAIL: Epoch %d should have at least %d filled slots, but has %d",
				denebEpoch,
				uint64(testnet.Spec().SLOTS_PER_EPOCH)-max_missed_slots,
				count[denebEpoch],
			)
		}

	}

	// Verify all submited blinded beacon blocks have correct signatures
	for i, n := range testnet.Nodes.Running() {
		b, ok := n.BeaconClient.Builder.(*mock_builder.MockBuilder)
		if !ok {
			t.Fatalf(
				"FAIL: client %d (%s) is not a mock builder",
				i,
				n.BeaconClient.ClientName(),
			)
		}

		if b.GetValidationErrorsCount() > 0 {
			// Validation errors should never happen, this means the submited blinded
			// beacon response received from the consensus client was incorrect.
			validationErrorsMap := b.GetValidationErrors()
			for slot, validationError := range validationErrorsMap {
				signedBeaconResponse, ok := b.GetSignedBeaconBlock(slot)
				if ok {
					signedBeaconResponseJson, _ := json.MarshalIndent(signedBeaconResponse, "", "  ")
					t.Logf(
						"INFO: builder %d encountered a validation error on slot %d: %v\n%s",
						i,
						slot,
						validationError,
						signedBeaconResponseJson,
					)
				}
				t.Fatalf(
					"FAIL: builder %d encountered a validation error on slot %d: %v",
					i,
					slot,
					validationError,
				)
			}
		}
	}
	t.Logf(
		"INFO: Validated all signatures of beacon blocks received by builders",
	)
}

// Sync testnet.
func (ts SyncDenebTestSpec) Execute(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
	n []clients.NodeDefinition,
) {
	// Run the base test spec execute function, this sends blobs and constructs the chain
	ts.BaseDencunTestSpec.Execute(t, ctx, testnet, env, config, n)

	// Wait the specified number of epochs to sync before starting the second client
	t.Logf("INFO: Waiting %d epochs for running clients to build a chain for last client to sync", ts.EpochsToSync)
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
		t.Fatalf("FAIL: error waiting for sync: %v", err)
	} else {
		t.Logf("INFO: all clients synced at head %s", h)
	}

	// Verify all clients agree on blobs for each slot
	if err := testnet.VerifyBlobs(ctx); err != nil {
		t.Fatalf("FAIL: error verifying blobs: %v", err)
	}
}
