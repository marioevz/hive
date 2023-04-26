// # Test suite for blob tests
package suite_blobs

import (
	"math/big"

	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	Head               *big.Int // Nil
	Pending            = big.NewInt(-2)
	Finalized          = big.NewInt(-3)
	Safe               = big.NewInt(-4)
	InvalidParamsError = -32602
	MAX_INITCODE_SIZE  = 49152

	DATAHASH_START_ADDRESS = big.NewInt(0x100)
	DATAHASH_ADDRESS_COUNT = 1000

	TARGET_BLOBS_PER_BLOCK = uint64(2)
	MAX_BLOBS_PER_BLOCK    = uint64(4)

	DATA_GAS_COST_INCREMENT_EXCEED_BLOBS = uint64(12)
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

// List of all blob tests
var Tests = []test.SpecInterface{
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transactions On Block 1, Cancun Genesis",
			About: `
			Tests the Cancun fork since genesis.
			`,
		},

		// We fork on genesis
		BlobsForkHeight: 0,

		BlobTestSequence: BlobTestSequence{
			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: 2,
				ExpectedBlobs:             []helper.BlobID{0, 1},
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				BlobTransactionSendCount:      DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(MAX_BLOBS_PER_BLOCK-TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
			},

			// Next payloads will have 4 data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (MAX_BLOBS_PER_BLOCK - TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Single Account",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send N blob transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions.
			 - The last payloads must include the last single-blob transactions.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		BlobsForkHeight: 0,

		BlobTestSequence: BlobTestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxDataGasCost: big.NewInt(100),
			},
			// Then send the single-blob transactions
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxDataGasCost: big.NewInt(100),
			},

			// First four payloads have MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              4,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              2,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Multiple Accounts",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Send N blob transactions with 1 blob each from account B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		BlobsForkHeight: 0,

		BlobTestSequence: BlobTestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxDataGasCost: big.NewInt(100),
				AccountIndex:                  0,
			},
			// Then send the single-blob transactions from account B
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxDataGasCost: big.NewInt(100),
				AccountIndex:                  1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Multiple Clients",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A, to client A.
			Send N blob transactions with 1 blob each from account B, to client
			B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		BlobsForkHeight: 0,

		BlobTestSequence: BlobTestSequence{
			// Start a secondary client to also receive blob transactions
			LaunchClient{
				EngineStarter: hive_rpc.HiveRPCEngineStarter{},
			},

			// Create a few blocks without any blobs
			NewPayloads{
				PayloadCount:              10,
				ExpectedIncludedBlobCount: 0,
			},

			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A, to client A.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxDataGasCost: big.NewInt(100),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			// Then send the single-blob transactions from account B, to client
			// B.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxDataGasCost: big.NewInt(100),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Replace Blob Transactions",
			About: `
			Test sending multiple blob transactions with the same nonce, but
			higher gas tip so the transaction is replaced.
			`,
		},

		// We fork on genesis
		BlobsForkHeight: 0,

		BlobTestSequence: BlobTestSequence{
			// Send multiple blob transactions with the same nonce.
			SendBlobTransactions{ // Blob ID 0
				BlobTransactionSendCount:      1,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e9),
				BlobTransactionGasTipCap:      big.NewInt(1e9),
			},
			SendBlobTransactions{ // Blob ID 1
				BlobTransactionSendCount:      1,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e10),
				BlobTransactionGasTipCap:      big.NewInt(1e10),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 2
				BlobTransactionSendCount:      1,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e11),
				BlobTransactionGasTipCap:      big.NewInt(1e11),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 3
				BlobTransactionSendCount:      1,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e12),
				BlobTransactionGasTipCap:      big.NewInt(1e12),
				ReplaceTransactions:           true,
			},

			// We create the first payload, which must contain the blob tx
			// with the higher tip.
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				ExpectedBlobs:             []helper.BlobID{3},
			},
		},
	},
}

// Blobs base spec
// This struct contains the base spec for all blob tests. It contains the
// timestamp increments per block, the withdrawals fork height, and the list of
// payloads to produce during the test.
type BlobsBaseSpec struct {
	test.Spec
	TimeIncrements  uint64 // Timestamp increments per block throughout the test
	BlobsForkHeight uint64 // Withdrawals activation fork height
	BlobTestSequence
}

// Base test case execution procedure for blobs tests.
func (bs *BlobsBaseSpec) Execute(t *test.Env) {

	t.CLMock.WaitForTTD()

	blobTestCtx := &BlobTestContext{
		Env:            t,
		TestBlobTxPool: new(TestBlobTxPool),
	}

	for stepId, step := range bs.BlobTestSequence {
		t.Logf("INFO: Executing step %d: %s", stepId+1, step.Description())
		if err := step.Execute(blobTestCtx); err != nil {
			t.Fatalf("FAIL: Error executing step %d: %v", stepId+1, err)
		}
	}

}
