package suite_transition

import (
	"context"
	"math/big"
	"time"

	api "github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type SecondaryClientSpec struct {
	ClientStarter client.EngineStarter

	// Whether the PoS chain should be built on top of this secondary client
	BuildPoSChainOnTop bool

	// Whether the main client shall sync to this secondary client or not.
	MainClientShallSync bool

	// TODO: Expected FcU outcome, could be "SYNCING", "VALID", etc..
}

type SecondaryClientSpecs []SecondaryClientSpec

type MergeTestSpec struct {
	// Name of the test
	Name string

	// Brief description of the test
	About string

	// TerminalTotalDifficulty delta value.
	// Actual TTD is genesis.Difficulty + this value
	// Default: 0
	TTD int64

	// Test maximum execution time until a timeout is raised.
	// Default: 60 seconds
	TimeoutSeconds int

	// Amount of seconds to keep checking that the main client does not switch chains.
	// Default: 0 seconds
	KeepCheckingUntilTimeout bool

	// Genesis file to be used for all clients launched during test
	// Default: genesis.json (init/genesis.json)
	GenesisFile string

	// Chain file to initialize the main client.
	MainChainFile string

	// Introduce PREVRANDAO transactions on the PoS blocks, including transition,
	// which could overwrite an existing transaction in the PoW chain (if re-org
	// occurred to a lower-height chain)
	PrevRandaoTransactions bool

	// Whether or not to send a forkchoiceUpdated directive on the main client before any attempts to re-org
	// to secondary clients happen.
	SkipMainClientFcU bool

	// Whether or not to wait for TTD to be reached by the main client
	SkipMainClientTTDWait bool

	// If set, the main client will be polled with `newPayload` until status!=`SYNCING` is returned.
	// If `VALID`, `latestValidHash` is also checked to be the hash of the transition block.
	// If `INVALID`, {status: INVALID, latestValidHash: 0x00..00, payloadId: null} is expected.
	TransitionPayloadStatus test.PayloadStatus

	// Number of PoS blocks to build on top of the MainClient.
	// Blocks will be built before any of the other clients is started, leading to a potential Post-PoS re-org.
	// Requires SkipMainClientFcU==false
	MainClientPoSBlocks int

	// Slot Safe/Finalized Delays
	SlotsToSafe      *big.Int
	SlotsToFinalized *big.Int

	// All secondary clients to be started during the tests with their respective chain files
	SecondaryClientSpecs SecondaryClientSpecs
}

var mergeTestSpecs = []MergeTestSpec{
	{
		Name:          "Single Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height",
		TTD:           196608,
		MainChainFile: "blocks_1_td_196608.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196704.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                    "Single Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height (Transition Payload)",
		TTD:                     196608,
		MainChainFile:           "blocks_1_td_196608.rlp",
		TransitionPayloadStatus: test.Valid,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196704.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Single Block PoW Re-org to Lower-Total-Difficulty Chain, Equal Height",
		TTD:           196608,
		MainChainFile: "blocks_1_td_196704.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196608.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block PoW Re-org to Higher-Total-Difficulty Chain, Equal Height",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_2_td_393504.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block PoW Re-org to Lower-Total-Difficulty Chain, Equal Height",
		TTD:           393120,
		MainChainFile: "blocks_2_td_393504.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_2_td_393120.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block PoW Re-org to Higher-Height Chain",
		TTD:           196704,
		MainChainFile: "blocks_1_td_196704.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_2_td_393120.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:          "Two Block PoW Re-org to Lower-Height Chain",
		TTD:           196704,
		MainChainFile: "blocks_2_td_393120.rlp",
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196704.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                     "Two Block PoW Re-org to Lower-Height Chain, Transaction Overwrite",
		TTD:                      196704,
		MainChainFile:            "blocks_2_td_393120.rlp",
		KeepCheckingUntilTimeout: true,
		PrevRandaoTransactions:   true,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196704.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                "Two Block Post-PoS Re-org to Higher-Total-Difficulty PoW Chain",
		TTD:                 196608,
		MainChainFile:       "blocks_1_td_196608.rlp",
		MainClientPoSBlocks: 2,
		SlotsToFinalized:    big.NewInt(5),
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196704.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                "Two Block Post-PoS Re-org to Lower-Total-Difficulty PoW Chain",
		TTD:                 196608,
		MainChainFile:       "blocks_1_td_196704.rlp",
		MainClientPoSBlocks: 2,
		SlotsToFinalized:    big.NewInt(5),
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196608.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                "Two Block Post-PoS Re-org to Higher-Height PoW Chain",
		TTD:                 196704,
		MainChainFile:       "blocks_1_td_196704.rlp",
		MainClientPoSBlocks: 2,
		SlotsToFinalized:    big.NewInt(5),
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_2_td_393120.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                "Two Block Post-PoS Re-org to Lower-Height PoW Chain",
		TTD:                 196704,
		MainChainFile:       "blocks_2_td_393120.rlp",
		MainClientPoSBlocks: 2,
		SlotsToFinalized:    big.NewInt(5),
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1_td_196704.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                     "Halt following PoW chain",
		TTD:                      196608,
		MainChainFile:            "blocks_1_td_196608.rlp",
		SkipMainClientFcU:        true,
		TimeoutSeconds:           120,
		KeepCheckingUntilTimeout: true,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					TerminalTotalDifficulty: big.NewInt(393120),
					ChainFile:               "blocks_2_td_393120.rlp",
				},
				BuildPoSChainOnTop:  false,
				MainClientShallSync: false,
			},
		},
	},
	{
		Name:                  "Long PoW Chain Sync",
		TTD:                   135112316,
		MainChainFile:         "blocks_1_td_196416.rlp",
		SkipMainClientFcU:     true,
		SkipMainClientTTDWait: true,
		TimeoutSeconds:        300,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					ChainFile: "blocks_1024_td_135112316.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: true,
			},
		},
	},
	{
		Name:                     "Transition to a Chain with Invalid Terminal Block, Higher Configured Total Difficulty",
		TTD:                      196608,
		MainChainFile:            "blocks_1_td_196608.rlp",
		MainClientPoSBlocks:      1,
		KeepCheckingUntilTimeout: true,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					TerminalTotalDifficulty: big.NewInt(393120),
					ChainFile:               "blocks_2_td_393120.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: false,
			},
		},
	},
	{
		Name:                    "Transition to a Chain with Invalid Terminal Block, Higher Configured Total Difficulty (Transition Payload)",
		TTD:                     196608,
		MainChainFile:           "blocks_1_td_196608.rlp",
		MainClientPoSBlocks:     1,
		TransitionPayloadStatus: test.Invalid,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					TerminalTotalDifficulty: big.NewInt(393120),
					ChainFile:               "blocks_2_td_393120.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: false,
			},
		},
	},
	{
		Name:                     "Transition to a Chain with Invalid Terminal Block, Lower Configured Total Difficulty",
		TTD:                      393120,
		MainChainFile:            "blocks_2_td_393120.rlp",
		MainClientPoSBlocks:      1,
		KeepCheckingUntilTimeout: true,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					TerminalTotalDifficulty: big.NewInt(196608),
					ChainFile:               "blocks_1_td_196608.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: false,
			},
		},
	},
	{
		Name:                    "Transition to a Chain with Invalid Terminal Block, Lower Configured Total Difficulty (Transition Payload)",
		TTD:                     393120,
		MainChainFile:           "blocks_2_td_393120.rlp",
		MainClientPoSBlocks:     1,
		TransitionPayloadStatus: test.Invalid,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					TerminalTotalDifficulty: big.NewInt(196608),
					ChainFile:               "blocks_1_td_196608.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: false,
			},
		},
	},
	{
		Name:                    "Stop processing gossiped PoW blocks",
		TTD:                     393120,
		MainChainFile:           "blocks_2_td_393120.rlp",
		MainClientPoSBlocks:     1,
		TransitionPayloadStatus: test.Invalid,
		SecondaryClientSpecs: []SecondaryClientSpec{
			{
				ClientStarter: hive_rpc.HiveRPCEngineStarter{
					TerminalTotalDifficulty: big.NewInt(196608),
					ChainFile:               "blocks_1_td_196608.rlp",
				},
				BuildPoSChainOnTop:  true,
				MainClientShallSync: false,
			},
		},
	},

	/* TODOs:
	- reorg to a block with uncles
	- reorg to invalid block (bad state root or similar)
	- reorg multiple times to multiple client chains (5+)
	-
	*/
}

var Tests = func() []test.Spec {
	testSpecs := make([]test.Spec, 0)
	for _, mergeTest := range mergeTestSpecs {
		testSpecs = append(testSpecs, GenerateMergeTestSpec(mergeTest))
	}
	return testSpecs
}()

func (clients SecondaryClientSpecs) AnySync() bool {
	for _, c := range clients {
		if c.MainClientShallSync {
			return true
		}
	}
	return false
}

func (clients SecondaryClientSpecs) AnyPoSChainOnTop() bool {
	for _, c := range clients {
		if c.BuildPoSChainOnTop {
			return true
		}
	}
	return false
}

func GenerateMergeTestSpec(mergeTestSpec MergeTestSpec) test.Spec {
	runFunc := func(t *test.Env) {
		// The first client waits for TTD, which ideally should be reached immediately using loaded chain
		if !mergeTestSpec.SkipMainClientTTDWait {
			if err := helper.WaitForTTDWithTimeout(t.Engine, t.TestContext); err != nil {
				t.Fatalf("FAIL (%s): Error while waiting for EngineClient (%s) to reach TTD: %v", t.TestName, t.Engine.ID(), err)
			}

			if !mergeTestSpec.SkipMainClientFcU {
				// Set the head of the CLMocker to the head of the main client
				t.CLMock.SetTTDBlockClient(t.Engine)
				if mergeTestSpec.MainClientPoSBlocks > 0 {
					// CL Mocker `ProduceBlocks` automatically checks that the PoS chain is followed by the client
					t.CLMock.ProduceBlocks(mergeTestSpec.MainClientPoSBlocks, clmock.BlockProcessCallbacks{})
				}
			}
		}

		// At this point, Head must be main client's HeadBlockHash, but this can change depending on the
		// secondary clients
		ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
		defer cancel()
		header, err := t.Eth.HeaderByNumber(ctx, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to obtain main client latest header: %v", t.TestName, err)
		}
		mustHeadHash := header.Hash()
		t.Logf("INFO (%s): Must head hash updated: %v", t.TestName, mustHeadHash)

		// Start a secondary clients with alternative PoW chains
		for _, secondaryClientSpec := range mergeTestSpec.SecondaryClientSpecs {
			// Start the secondary client with the alternative PoW chain
			t.Logf("INFO (%s): Running secondary client: %v", t.TestName, secondaryClientSpec)
			secondaryClient, err := secondaryClientSpec.ClientStarter.StartClient(t.T, t.ClientParams, t.ClientFiles, t.Engine)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to start secondary client: %v", t.TestName, err)
			}

			// Add this client to the CLMocker list of Engine clients
			t.CLMock.AddEngineClient(secondaryClient)

			if secondaryClientSpec.BuildPoSChainOnTop {
				if err := helper.WaitForTTDWithTimeout(secondaryClient, t.TestContext); err != nil {
					t.Fatalf("FAIL (%s): Error while waiting for EngineClient (%s) to reach TTD: %v", t.TestName, secondaryClient.ID(), err)
				}
				t.CLMock.SetTTDBlockClient(secondaryClient)
			}

			if secondaryClientSpec.MainClientShallSync {
				// The main client shall sync to this secondary client in order for the test to succeed.
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				if header, err := secondaryClient.HeaderByNumber(ctx, nil); err == nil {
					mustHeadHash = header.Hash()
					t.Logf("INFO (%s): Must head hash updated: %v", t.TestName, mustHeadHash)
				} else {
					t.Fatalf("FAIL (%s): Unable to obtain client [%s] latest header: %v", t.TestName, secondaryClient.ID, err)
				}
			}
		}

		// We are going to send PREVRANDAO transactions if the test requires so.
		// These transactions might overwrite some of the PoW chain transactions if we re-org'd into a lower height chain.
		prevRandaoTxs := make([]*types.Transaction, 0)
		prevRandaoFunc := func() {
			if mergeTestSpec.PrevRandaoTransactions {
				// Get the address nonce:
				// This is because we could have included transactions in the PoW chain of the block
				// producer, or re-orged.
				tx, err := helper.SendNextTransaction(t.TestContext, t.CLMock.NextBlockProducer, globals.PrevRandaoContractAddr, common.Big0, nil, t.TestTransactionType)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable create next transaction: %v", t.TestName, err)
				}
				prevRandaoTxs = append(prevRandaoTxs, tx)
			}
		}
		if mergeTestSpec.PrevRandaoTransactions {
			// At the end of the test we are going to verify that these transactions did produce the post-merge expected
			// outcome, even if they had been previously executed on the PoW chain.
			defer func() {
				for _, tx := range prevRandaoTxs {
					// Get the receipt of the transaction
					ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
					defer cancel()
					r, err := t.Eth.TransactionReceipt(ctx, tx.Hash())
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to obtain tx [%v] receipt: %v", t.TestName, tx.Hash(), err)
					}

					blockNumberAsStorageKey := common.BytesToHash(r.BlockNumber.Bytes())
					s := t.TestEngine.TestStorageAt(globals.PrevRandaoContractAddr, blockNumberAsStorageKey, nil)
					s.ExpectStorageEqual(t.CLMock.PrevRandaoHistory[r.BlockNumber.Uint64()])
				}
			}()
		}

		// Test end state of the main client
		for {
			if mergeTestSpec.SecondaryClientSpecs.AnyPoSChainOnTop() && (mergeTestSpec.TransitionPayloadStatus == test.Unknown ||
				t.CLMock.FirstPoSBlockNumber == nil) {
				// Build a block and check whether the main client switches
				t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
					OnPayloadProducerSelected: prevRandaoFunc,
				})

				// If the main client should follow the PoS chain, update the mustHeadHash
				if mustHeadHash == t.CLMock.LatestHeader.ParentHash {
					// Keep following the chain if that is what the test expects
					mustHeadHash = t.CLMock.LatestHeader.Hash()
					t.Logf("INFO (%s): Must head hash updated: %v", t.TestName, mustHeadHash)
				}
			}
			ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
			defer cancel()
			if mergeTestSpec.TransitionPayloadStatus != test.Unknown {
				// We are specifically checking the transition payload in this test case
				p := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestExecutedPayload)
				p.ExpectNoError()
				if p.Status.Status != api.SYNCING {
					p.ExpectStatus(mergeTestSpec.TransitionPayloadStatus)
					if mergeTestSpec.TransitionPayloadStatus == test.Valid {
						p.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
					} else if mergeTestSpec.TransitionPayloadStatus == test.Invalid {
						p.ExpectLatestValidHash(&(common.Hash{}))
					}
					break
				}

			} else if header, err := t.Eth.HeaderByNumber(ctx, nil); err == nil {
				// We are not checking the transition block, we are checking that the client sticks to the correct chain.
				if header.Hash() == mustHeadHash {
					t.Logf("INFO (%s): Main client is now synced to the expected head, %v", t.TestName, header.Hash())
					break
				}
			} else {
				t.Fatalf("FAIL (%s): Error getting latest header for main client: %v", t.TestName, err)
			}

			// Check for timeout.
			select {
			case <-time.After(time.Second):
			case <-t.TestContext.Done():
				t.Fatalf("FAIL (%s): Timeout while waiting for sync on the alternative PoW chain", t.TestName)
			}
		}

		// Test specified that we must keep checking the main client to sticks to mustHeadHash until timeout
		if mergeTestSpec.KeepCheckingUntilTimeout {
			for {
				if mergeTestSpec.SecondaryClientSpecs.AnyPoSChainOnTop() {
					// Build a block and check whether the main client switches
					t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
						OnPayloadProducerSelected: prevRandaoFunc,
					})

					// If the main client should follow the PoS chain, update the mustHeadHash
					if mustHeadHash == t.CLMock.LatestHeader.ParentHash {
						// Keep following the chain if that is what the test expects
						mustHeadHash = t.CLMock.LatestHeader.Hash()
						t.Logf("INFO (%s): Must head hash updated: %v", t.TestName, mustHeadHash)
					}

				}
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				if header, err := t.Eth.HeaderByNumber(ctx, nil); err == nil {
					if header.Hash() != mustHeadHash {
						t.Fatalf("FAIL (%s): Main client synced to incorrect chain: %v", t.TestName, header.Hash())
						break
					}
				} else {
					t.Fatalf("FAIL (%s): Error getting latest header for main client: %v", t.TestName, err)
				}

				// Wait here before checking the head again.
				select {
				case <-time.After(time.Second):
				case <-t.TestContext.Done():
					// This means the test is over but that is ok since the client did not switch to an incorrect chain.
					return
				}

			}
		}
	}

	return test.Spec{
		Name:             mergeTestSpec.Name,
		About:            mergeTestSpec.About,
		Run:              runFunc,
		TTD:              mergeTestSpec.TTD,
		TimeoutSeconds:   mergeTestSpec.TimeoutSeconds,
		SlotsToSafe:      mergeTestSpec.SlotsToSafe,
		SlotsToFinalized: mergeTestSpec.SlotsToFinalized,
		GenesisFile:      mergeTestSpec.GenesisFile,
		ChainFile:        mergeTestSpec.MainChainFile,
	}
}
