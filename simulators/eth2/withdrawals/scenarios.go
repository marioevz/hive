package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/hivesim"
	mock_builder "github.com/ethereum/hive/simulators/eth2/common/builder/mock"
	"github.com/ethereum/hive/simulators/eth2/common/clients"

	beacon_verification "github.com/ethereum/hive/simulators/eth2/common/spoofing/beacon"
	tn "github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var ConsensusClientsSupportingBLSChangesBeforeCapella = []string{
	"prysm",
	"lodestar",
}

// Generic withdrawals test routine, capable of running most of the test
// scenarios.
func (ts BaseWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *tn.Environment,
	n []clients.NodeDefinition,
) {
	config := ts.GetTestnetConfig(n)
	ctx := context.Background()

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	// Add verification of Beacon->Execution Engine API calls to the proxies

	// NewPayloadV1 expires at ShanghaiTime
	newPayloadV1ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineNewPayloadV1,
		testnet.ExecutionGenesis().Config.ShanghaiTime.Uint64(),
	)
	// ForkchoiceUpdatedV1 expires at ShanghaiTime
	forkchoiceUpdatedV1ExpireVerifier := beacon_verification.NewEngineMaxTimestampVerifier(
		t,
		beacon_verification.EngineForkchoiceUpdatedV1,
		testnet.ExecutionGenesis().Config.ShanghaiTime.Uint64(),
	)
	for _, e := range testnet.ExecutionClients() {
		newPayloadV1ExpireVerifier.AddToProxy(e.Proxy())
		forkchoiceUpdatedV1ExpireVerifier.AddToProxy(e.Proxy())
	}

	blsDomain := ComputeBLSToExecutionDomain(testnet)

	// Get all validators info
	allValidators, err := ValidatorsFromBeaconState(
		testnet.GenesisBeaconState(),
		testnet.Spec().Spec,
		env.Keys,
		&blsDomain,
	)
	if err != nil {
		t.Fatalf("FAIL: Error parsing validators from beacon state")
	}
	genesisNonWithdrawable := allValidators.NonWithdrawable()

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)

	// Wait for 3 slots to pass
	<-time.After(
		3 * time.Second * time.Duration(testnet.Spec().SECONDS_PER_SLOT),
	)

	if beaconClients := testnet.FilterByCL(ConsensusClientsSupportingBLSChangesBeforeCapella).
		BeaconClients(); len(
		beaconClients,
	) > 0 &&
		ts.SubmitBLSChangesOnBellatrix {
		// If there are clients that support sending BLS to execution
		// changes in bellatrix, we send half of the changes here
		if len(genesisNonWithdrawable) > 0 {
			nonWithdrawableValidators := genesisNonWithdrawable.Chunks(2)[0]

			if len(nonWithdrawableValidators) > 0 {
				t.Logf(
					"INFO: Sending %d validators' BLS-to-exec-change on bellatrix",
					len(nonWithdrawableValidators),
				)
				for i := 0; i < len(nonWithdrawableValidators); i++ {
					b := beaconClients[i%len(beaconClients)]
					v := nonWithdrawableValidators[i]
					if err := v.SignSendBLSToExecutionChange(
						ctx,
						b,
						common.Address{byte(v.Index + 0x100)},
					); err != nil {
						t.Fatalf(
							"FAIL: Unable to submit bls-to-execution changes: %v",
							err,
						)
					} else {
						t.Logf("INFO: Sent validator %d BLS-To-Exec-Change on Bellatrix (%s)", v.Index, b.ClientName())
					}
				}

			}

		} else {
			t.Logf("INFO: no validators left on BLS credentials")
		}
	} else {
		t.Logf("INFO: No beacon clients support BLS-To-Execution-Changes on bellatrix, skipping")
	}

	// Wait for Capella
	if config.CapellaForkEpoch.Uint64() > 0 {
		slotsUntilCapella := beacon.Slot(
			config.CapellaForkEpoch.Uint64(),
		) * testnet.Spec().SLOTS_PER_EPOCH
		testnet.WaitSlots(ctx, slotsUntilCapella)
	}

	// If there are any remaining validators that cannot withdraw yet, send
	// them now
	nonWithdrawableValidators := allValidators.NonWithdrawable()
	if len(nonWithdrawableValidators) > 0 {
		beaconClients := testnet.BeaconClients()
		for i := 0; i < len(nonWithdrawableValidators); i++ {
			b := beaconClients[i%len(beaconClients)]
			v := nonWithdrawableValidators[i]
			if err := v.SignSendBLSToExecutionChange(
				ctx,
				b,
				common.Address{byte(v.Index + 0x100)},
			); err != nil {
				t.Fatalf(
					"FAIL: Unable to submit bls-to-execution changes: %v",
					err,
				)
			} else {
				t.Logf("INFO: Sent validator %d BLS-To-Exec-Change on Capella (%s)", v.Index, b.ClientName())
			}
		}

		// Wait for all BLS to execution to be included
		slotsForAllBlsInclusion := beacon.Slot(
			len(genesisNonWithdrawable)/int(
				testnet.Spec().MAX_BLS_TO_EXECUTION_CHANGES,
			) + 1,
		)
		testnet.WaitSlots(ctx, slotsForAllBlsInclusion)
	} else {
		t.Logf("INFO: no validators left on BLS credentials")
	}

	// Get the beacon state and verify the credentials were updated
	var versionedBeaconState *clients.VersionedBeaconStateResponse
	for _, bn := range testnet.BeaconClients().Running() {
		versionedBeaconState, err = bn.BeaconStateV2(
			ctx,
			eth2api.StateHead,
		)
		if err != nil || versionedBeaconState == nil {
			t.Logf("WARN: Unable to get latest beacon state: %v", err)
		} else {
			break
		}
	}
	if versionedBeaconState == nil {
		t.Fatalf(
			"FAIL: Unable to get latest beacon state from any client: %v",
			err,
		)
	}

	validators := versionedBeaconState.Validators()

	if len(genesisNonWithdrawable) > 0 {
		t.Logf("INFO: Checking validator updates on slot %d",
			versionedBeaconState.StateSlot())

		for _, v := range genesisNonWithdrawable {
			validator := validators[v.Index]
			credentials := validator.WithdrawalCredentials
			if !bytes.Equal(
				credentials[:1],
				[]byte{beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX},
			) {
				t.Fatalf(
					"FAIL: Withdrawal credential not updated for validator %d: %v",
					v.Index,
					credentials,
				)
			}
			if v.WithdrawAddress == nil {
				t.Fatalf(
					"FAIL: BLS-to-execution change was not sent for validator %d",
					v.Index,
				)
			}
			if !bytes.Equal(v.WithdrawAddress[:], credentials[12:]) {
				t.Fatalf(
					"FAIL: Incorrect withdrawal credential for validator %d: want=%x, got=%x",
					v.Index,
					v.WithdrawAddress,
					credentials[12:],
				)
			}
			t.Logf("INFO: Successful BLS to execution change: %s", credentials)
		}
	}

	// Wait for all validators to withdraw
	waitSlotsForAllWithdrawals := beacon.Slot(
		(len(validators)/int(testnet.Spec().MAX_WITHDRAWALS_PER_PAYLOAD) +
			5), // Wiggle room
	)
	slotCtx, cancel := testnet.Spec().
		SlotTimeoutContext(ctx, waitSlotsForAllWithdrawals)
	defer cancel()
loop:
	for {
		// Print all info
		testnet.BeaconClients().Running().PrintStatus(slotCtx)

		// Check all accounts
		for _, n := range testnet.Nodes.Running() {
			ec := n.ExecutionClient
			bc := n.BeaconClient
			if allAccountsWithdrawn, err := allValidators.Withdrawable().VerifyWithdrawnBalance(ctx, bc, ec, eth2api.BlockHead); err != nil {
				t.Logf("INFO: error getting withdrawals balances: %v", err)
				continue
			} else if allAccountsWithdrawn {
				t.Logf("INFO: All accounts have successfully withdrawn")
				break loop
			}
		}

		select {
		case <-slotCtx.Done():
			PrintWithdrawalHistory(ctx, testnet.BeaconClients().Running()[0], eth2api.BlockHead)
			t.Fatalf("FAIL: Timeout waiting on all accounts to withdraw")
		case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
		}
	}

	PrintWithdrawalHistory(
		ctx,
		testnet.BeaconClients().Running()[0],
		eth2api.BlockHead,
	)

	// Lastly check all clients are on the same head
	testnet.VerifyELHeads(ctx)

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
		testnet.WaitForFinality(ctx)
	}
}

// Withdrawals re-org test routine:
// - Launch two node pairs initially, disconnected from each other
// - Beacon chain starts at capella
// - Send BLS-to-execution changes on both forks, but the addresses are different
// - Let both forks of the chain progress separately, wait for withdrawals on every account
// - Spawn a fifth node that connects to all clients
// - Wait until one of the forks becomes canonical to all clients
// - Verify withdrawal accounts and balances are correct
func (ts ReOrgWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *tn.Environment,
	n []clients.NodeDefinition,
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := ts.GetTestnetConfig(n)

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	if len(testnet.Nodes) != 5 {
		t.Fatalf(
			"FAIL: Expected five nodes, node count = %d",
			len(testnet.Nodes),
		)
	} else if len(testnet.Nodes.Running()) != 4 {
		t.Fatalf("FAIL: Expected two nodes running, node count = %d", len(testnet.Nodes.Running()))
	}

	// Get all validators info
	blsDomain := ComputeBLSToExecutionDomain(testnet)
	allValidators, err := ValidatorsFromBeaconState(
		testnet.GenesisBeaconState(),
		testnet.Spec().Spec,
		env.Keys,
		&blsDomain,
	)
	if err != nil {
		t.Fatalf("FAIL: Error parsing validators from beacon state")
	}
	allValidatorsPerBeacon := []Validators{
		allValidators,
		allValidators.Copy(),
	}

	beaconPairs := []clients.BeaconClients{
		testnet.BeaconClients().Running().Subnet("A"),
		testnet.BeaconClients().Running().Subnet("B"),
	}

	// Wait for beacon chain genesis to happen
	testnet.WaitForGenesis(ctx)

	// Wait a couple of slots for connection between clients
	testnet.WaitSlots(ctx, 3)

	// Send different address to each running beacon client
	for i, bns := range beaconPairs {
		for _, v := range allValidatorsPerBeacon[i] {
			addr := common.Address{}
			addr[0] = byte(i)
			addr[1] = byte(v.Index)

			if _, err := v.SignBLSToExecutionChange(addr); err != nil {
				t.Fatalf(
					"FAIL: Unable to sign bls-to-execution change: %v",
					err,
				)
			} else {
				v.WithdrawAddress = &addr
			}
		}
		for _, b := range bns {
			if err := allValidatorsPerBeacon[i].SendSignedBLSToExecutionChanges(ctx, b); err != nil {
				t.Fatalf(
					"FAIL: Unable to submit bls-to-execution changes to beacon %d: %v",
					i,
					err,
				)
			} else {
				t.Logf("INFO: Sent validator BLS changes to beacon %d", b.Config.ClientIndex)
			}
		}
	}

	// Wait for the credentials to be updated for all validators.
	// It could take a while since the validators are missing a lot of slots
	var (
		wg   sync.WaitGroup
		errs = make(chan error, len(testnet.BeaconClients().Running()))
	)
	timeoutCtx, cancel := testnet.Spec().
		EpochTimeoutContext(ctx, 2)
	defer cancel()
	for i, b := range testnet.BeaconClients().Running() {
		wg.Add(1)
		go func(ctx context.Context, bn *clients.BeaconClient, bIdx int, vs Validators) {
			defer wg.Done()
			for {
				if versionedBeaconState, err := bn.BeaconStateV2(
					ctx,
					eth2api.StateHead,
				); err != nil || versionedBeaconState == nil {
					t.Logf(
						"WARN: Unable to get latest beacon state: %v",
						err,
					)
				} else {
					t.Logf(
						"INFO: beacon %d: slot=%d, root=%s",
						bIdx, versionedBeaconState.StateSlot(),
						versionedBeaconState.Root(),
					)
					validators := versionedBeaconState.Validators()
					validatorsUpdatedCount := 0
					for _, v := range vs {
						validator := validators[v.Index]
						credentials := validator.WithdrawalCredentials
						if v.WithdrawAddress != nil {
							expectedAddress := *v.WithdrawAddress
							if bytes.Equal(
								credentials[:1],
								[]byte{beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX},
							) {
								if !bytes.Equal(expectedAddress[:], credentials[12:]) {
									errs <- fmt.Errorf("validator updated to incorrect address: want %s, got %s", expectedAddress.String(), common.BytesToAddress(credentials[12:]).String())
									return
								}
								validatorsUpdatedCount += 1
							}
						} else {
							t.Logf(
								"WARN: validator has nil expected exec address: %d",
								v.Index,
							)
						}
					}
					if validatorsUpdatedCount == len(vs) {
						t.Logf(
							"INFO: all %d validators updated",
							validatorsUpdatedCount,
						)
						return
					} else {
						t.Logf(
							"INFO: %d validators out of %d updated",
							validatorsUpdatedCount,
							len(vs),
						)
					}
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
				}
			}
		}(
			timeoutCtx,
			b,
			i,
			allValidatorsPerBeacon[i/2],
		)
	}

	wg.Wait()

	select {
	case <-ctx.Done():
		t.Fatalf("FAIL: Timeout while waiting for BLS changes inclusion")
	case err := <-errs:
		t.Fatalf(
			"FAIL: Error while trying to fetch BLS changes from client: %v",
			err,
		)
	default:
		t.Logf("INFO: All validators updated on both clients")
		testnet.BeaconClients().Running().PrintStatus(ctx)
	}

	// Start third client to connect all clients together
	if err := testnet.Nodes[4].Start(); err != nil {
		t.Fatalf(
			"FAIL: Unable to start third client: %v",
			err,
		)
	}

	if head, err := testnet.WaitForCanonicalChain(ctx); err != nil {
		t.Fatalf(
			"FAIL: Failure while waiting for clients to converge: %v",
			err,
		)
	} else {
		t.Logf("INFO: All beacon clients converged into the same head: slot=%d, root=%s", head.Header.Message.Slot, head.Root.String())
	}

	// Wait for all nodes to leave optimistic sync state
	wg = sync.WaitGroup{}
	errs = make(chan error, len(testnet.BeaconClients().Running()))
	for _, bn := range testnet.BeaconClients().Running() {
		wg.Add(1)
		go func(bn *clients.BeaconClient) {
			defer wg.Done()
			if _, err := bn.WaitForOptimisticState(ctx, eth2api.BlockHead, false); err != nil {
				errs <- err
			}
		}(bn)
	}
	wg.Wait()
	select {
	case err := <-errs:
		t.Fatalf(
			"FAIL: Failure while waiting for clients to leave optimistic sync: %v",
			err,
		)
	default:
		t.Logf("INFO: All beacon clients left optimistic sync mode")
	}

	// Verify which address list ended up as canonical
	var (
		canonicalValidators Validators
		orphanedValidators  Validators
	)
	for i := range allValidatorsPerBeacon {
		v := allValidatorsPerBeacon[i][0]
		if set, err := v.CheckExecutionAddressApplied(ctx, testnet.BeaconClients().Running()[0], eth2api.StateHead); err != nil {
			if errors.Is(err, UnexpectedExecutionWithdrawalAddress) {
				orphanedValidators = allValidatorsPerBeacon[i]
			} else {
				t.Fatalf(
					"FAIL: Failure while checking for canonical list of validator addresses: %v",
					err,
				)
			}
		} else if set {
			canonicalValidators = allValidatorsPerBeacon[i]
		}
	}

	// Verify withdrawal addresses and balances on all validators
loop:
	for {
		// Print all info
		testnet.BeaconClients().Running().PrintStatus(ctx)

		// Check all accounts
		for _, n := range testnet.Nodes.Running() {
			ec := n.ExecutionClient
			bc := n.BeaconClient
			if allAccountsWithdrawn, err := canonicalValidators.Withdrawable().VerifyWithdrawnBalance(ctx, bc, ec, eth2api.BlockHead); err != nil {
				t.Fatalf("FAIL: error during withdrawal balance verification: %v", err)
			} else if allAccountsWithdrawn {
				t.Logf("INFO: All accounts have successfully withdrawn")
				break loop
			}
		}

		select {
		case <-ctx.Done():
			PrintWithdrawalHistory(ctx, testnet.BeaconClients().Running()[0], eth2api.BlockHead)
			t.Fatalf("FAIL: Timeout waiting on all accounts to withdraw")
		case <-time.After(time.Duration(testnet.Spec().SECONDS_PER_SLOT) * time.Second):
		}
	}

	for _, v := range orphanedValidators {
		executionAddress := *v.WithdrawAddress
		for _, ec := range testnet.ExecutionClients().Running() {
			balance, err := ec.BalanceAt(ctx, executionAddress, nil)
			if err != nil {
				t.Fatalf(
					"FAIL: Error getting balance of orphaned withdraw account: %v",
					err,
				)
			}
			if balance.Cmp(common.Big0) != 0 {
				t.Fatalf(
					"FAIL: Orphaned withdraw account not empty: %d",
					balance,
				)
			}
		}
		t.Logf(
			"INFO: Balance correctly empty for orphaned validator withdraw account %s (validator %d)",
			executionAddress,
			v.Index,
		)
	}

	PrintWithdrawalHistory(
		ctx,
		testnet.BeaconClients().Running()[0],
		eth2api.BlockHead,
	)
}

// Builder testnet.
func (ts BuilderWithdrawalsTestSpec) Execute(
	t *hivesim.T,
	env *tn.Environment,
	n []clients.NodeDefinition,
) {
	config := ts.GetTestnetConfig(n)
	ctx := context.Background()

	// Configure the builder according to the error
	config.BuilderOptions = make([]mock_builder.Option, 0)

	// Bump the built payloads value
	config.BuilderOptions = append(
		config.BuilderOptions,
		mock_builder.WithPayloadWeiValueBump(big.NewInt(10000)),
		mock_builder.WithExtraDataWatermark("builder payload tst"),
	)

	// Inject test error
	capellaEpoch := beacon.Epoch(config.CapellaForkEpoch.Uint64())
	if ts.ErrorOnHeaderRequest {
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithErrorOnHeaderRequestAtEpoch(capellaEpoch),
		)
	}
	if ts.ErrorOnPayloadReveal {
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithErrorOnPayloadRevealAtEpoch(capellaEpoch),
		)
	}
	if ts.InvalidatePayload != "" {
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithPayloadInvalidatorAtEpoch(
				capellaEpoch,
				ts.InvalidatePayload,
			),
		)
	}
	if ts.InvalidatePayloadAttributes != "" {
		config.BuilderOptions = append(
			config.BuilderOptions,
			mock_builder.WithPayloadAttributesInvalidatorAtEpoch(
				capellaEpoch,
				ts.InvalidatePayloadAttributes,
			),
		)
	}

	testnet := tn.StartTestnet(ctx, t, env, config)
	defer testnet.Stop()

	go func() {
		lastNonce := uint64(0)
		txPerIteration := 5
		txCreator := BaseTransactionCreator{
			GasLimit:   500000,
			Amount:     common.Big1,
			PrivateKey: VaultKey,
		}
		// Send some transactions constantly in the bg
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				for i := 0; i < txPerIteration; i++ {
					txCreator.Recipient = &CodeContractAddress
					tx, err := txCreator.MakeTransaction(lastNonce)
					if err != nil {
						panic(err)
					}
					if err := testnet.ExecutionClients().Running()[0].SendTransaction(
						ctx,
						tx,
					); err != nil {
						t.Logf("INFO: Error sending tx: %v", err)
					}
					lastNonce++
				}
			}
		}
	}()

	// Wait for capella
	forkCtx, cancel := testnet.Spec().
		EpochTimeoutContext(ctx, beacon.Epoch(config.CapellaForkEpoch.Uint64())+1)
	defer cancel()
	if err := testnet.WaitForFork(forkCtx, "capella"); err != nil {
		t.Fatalf("FAIL: error while waiting for capella: %v", err)
	}

	// Check that the builder was working properly until now
	for i, b := range testnet.BeaconClients().Running() {
		builder := b.Builder
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

	// Wait for finalization, to verify that builder modifications
	// did not affect the network
	finalityCtx, cancel := testnet.Spec().EpochTimeoutContext(ctx, 5)
	defer cancel()
	if _, err := testnet.WaitForCurrentEpochFinalization(finalityCtx); err != nil {
		t.Fatalf("FAIL: error waiting for epoch finalization: %v", err)
	}

	// Verify any modified payloads did not make it into the
	// canonical chain
	if !ts.ErrorOnHeaderRequest && !ts.ErrorOnPayloadReveal &&
		ts.InvalidatePayload == "" &&
		ts.InvalidatePayloadAttributes == "" {
		// Simply verify that builder's capella payloads were included in the
		// canonical chain
		for i, n := range testnet.Nodes.Running() {
			b := n.BeaconClient.Builder
			ec := n.ExecutionClient
			includedPayloads := 0
			for _, p := range b.GetBuiltPayloads() {
				if p.Withdrawals != nil {
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
						}
					}
				}
			}
			if includedPayloads == 0 {
				t.Fatalf(
					"FAIL: builder %d did not produce capella payloads included in the canonical chain",
					i,
				)
			}
		}
	} else if ts.InvalidatePayloadAttributes != "" {
		for i, n := range testnet.VerificationNodes().Running() {
			modifiedPayloads := n.BeaconClient.Builder.GetModifiedPayloads()
			if len(modifiedPayloads) == 0 {
				t.Fatalf("FAIL: No payloads were modified by builder %d", i)
			}
			for _, p := range modifiedPayloads {
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

	// Count and print missed slots
	if count, err := testnet.BeaconClients().Running()[0].GetFilledSlotsCountPerEpoch(ctx); err != nil {
		t.Fatalf("FAIL: unable to obtain slot count per epoch: %v", err)
	} else {
		for ep, slots := range count {
			t.Logf("INFO: Epoch %d, filled slots=%d", ep, slots)
		}
	}
}
