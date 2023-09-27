package suite_builder

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"

	"github.com/ethereum/hive/hivesim"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	mock_builder "github.com/marioevz/mock-builder/mock"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

// Builder testnet.
func (ts BuilderTestSpec) ExecutePostFork(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Run the base test spec execute function
	ts.BaseTestSpec.ExecutePostFork(t, ctx, testnet, env, config)

	// Check that the builder was working properly until now
	if !ts.DenebGenesis {
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
}

func (ts BuilderTestSpec) Verify(
	t *hivesim.T,
	ctx context.Context,
	testnet *tn.Testnet,
	env *tn.Environment,
	config *tn.Config,
) {
	// Run the base test spec verify function
	ts.BaseTestSpec.Verify(t, ctx, testnet, env, config)

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
