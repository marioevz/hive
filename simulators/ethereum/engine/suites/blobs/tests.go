// # Test suite for blob tests
package suite_blobs

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
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

	MAX_BLOBS_PER_BLOCK = 4
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

// List of all withdrawals tests
var Tests = []test.SpecInterface{
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "Blob Transactions On Genesis",
			About: `
			Tests the sharding fork on genesis (e.g. on a
			testnet).
			`,
		},
		BlobsForkHeight: 0,
		TestBlobPayloads: []TestBlobPayload{
			{
				// We send a blob transaction on genesis, and this same
				// transaction should be included in the first block.
				BlobTransactionSendCount:      2,
				BlobTransactionMaxDataGasCost: big.NewInt(1),
				NormalTransactionSendCount:    0,
				ExpectedIncludedBlobCount:     2,
			},
		},
	},
}

// Test Blob Payload Descriptor
type TestBlobPayload struct {
	// Number of blob transactions to send before this block's GetPayload request
	BlobTransactionSendCount uint64
	// Max Data Gas Cost for every blob transaction
	BlobTransactionMaxDataGasCost *big.Int
	// Number of normal transactions (type 0-2) to send before this block's GetPayload request
	NormalTransactionSendCount uint64
	// Number of blob transactions that are expected to be included in the payload
	ExpectedIncludedBlobCount uint64
}

// Blobs base spec
// This struct contains the base spec for all blob tests. It contains the
// timestamp increments per block, the withdrawals fork height, and the list of
// payloads to produce during the test.
type BlobsBaseSpec struct {
	test.Spec
	TimeIncrements   uint64 // Timestamp increments per block throughout the test
	BlobsForkHeight  uint64 // Withdrawals activation fork height
	TestBlobPayloads []TestBlobPayload
}

// Generates the fork config, including sharding fork timestamp.
func (bs *BlobsBaseSpec) GetForkConfig() globals.ForkConfig {
	return globals.ForkConfig{
		ShanghaiTimestamp:     big.NewInt(0),
		ShardingForkTimestamp: big.NewInt(int64(bs.BlobsForkHeight) * int64(bs.GetBlockTimeIncrements())),
	}
}

// Get the per-block timestamp increments configured for this test
func (bs *BlobsBaseSpec) GetBlockTimeIncrements() uint64 {
	return 1
}

// Timestamp delta between genesis and the withdrawals fork
func (bs *BlobsBaseSpec) GetBlobsGenesisTimeDelta() uint64 {
	return bs.BlobsForkHeight * bs.GetBlockTimeIncrements()
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (bs *BlobsBaseSpec) GetBlobsForkTime() uint64 {
	return uint64(globals.GenesisTimestamp) + bs.GetBlobsGenesisTimeDelta()
}

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (bs *BlobsBaseSpec) GetGenesis() *core.Genesis {
	genesis := bs.Spec.GetGenesis()

	// Remove PoW altogether
	genesis.Difficulty = common.Big0
	genesis.Config.TerminalTotalDifficulty = common.Big0
	genesis.Config.Clique = nil
	genesis.ExtraData = []byte{}

	// Add accounts that use the DATAHASH opcode
	datahashCode := []byte{
		0x5F, // PUSH0
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x01)
		0x01,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x02)
		0x02,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
		0x60, // PUSH1(0x03)
		0x03,
		0x80, // DUP1
		0x49, // DATAHASH
		0x55, // SSTORE
	}

	for i := 0; i < DATAHASH_ADDRESS_COUNT; i++ {
		address := big.NewInt(0).Add(DATAHASH_START_ADDRESS, big.NewInt(int64(i)))
		genesis.Alloc[common.BigToAddress(address)] = core.GenesisAccount{
			Code:    datahashCode,
			Balance: common.Big0,
		}
	}

	return genesis
}

// Changes the CL Mocker default time increments of 1 to the value specified
// in the test spec.
func (bs *BlobsBaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	cl.BlockTimestampIncrement = big.NewInt(int64(bs.GetBlockTimeIncrements()))
}

type TestBlobTxPool struct {
	Transactions map[common.Hash]*types.Transaction
}

func (pool *TestBlobTxPool) VerifyBlobBundle(payload *engine.ExecutableData, blobBundle *engine.BlobsBundle, expectedBlobCount int) error {
	if payload.BlockHash != blobBundle.BlockHash {
		return fmt.Errorf("block hash mismatch: %s != %s", payload.BlockHash.String(), blobBundle.BlockHash.String())
	}
	if len(blobBundle.Blobs) != expectedBlobCount {
		return fmt.Errorf("expected %d blob, got %d", expectedBlobCount, len(blobBundle.Blobs))
	}
	if len(blobBundle.KZGs) != expectedBlobCount {
		return fmt.Errorf("expected %d KZG, got %d", expectedBlobCount, len(blobBundle.KZGs))
	}
	// Find all blob transactions included in the payload
	type BlobWrapData struct {
		VersionedHash common.Hash
		KZG           types.KZGCommitment
		Blob          types.Blob
		Proof         types.KZGProof
	}
	var blobDataInPayload = make([]*BlobWrapData, 0)

	for _, binaryTx := range payload.Transactions {
		// Unmarshal the tx from the payload, which should be the minimal version
		// of the blob transaction
		txData := new(types.Transaction)
		if err := txData.UnmarshalMinimal(binaryTx); err != nil {
			return err
		}

		if txData.Type() != types.BlobTxType {
			continue
		}

		// Find the transaction in the current pool of known transactions
		if tx, ok := pool.Transactions[txData.Hash()]; ok {
			versionedHashes, kzgs, blobs, proofs := tx.BlobWrapData()
			if len(versionedHashes) != len(kzgs) || len(kzgs) != len(blobs) || len(blobs) != len(proofs) {
				return fmt.Errorf("invalid blob wrap data")
			}
			for i := 0; i < len(versionedHashes); i++ {
				blobDataInPayload = append(blobDataInPayload, &BlobWrapData{
					VersionedHash: versionedHashes[i],
					KZG:           kzgs[i],
					Blob:          blobs[i],
					Proof:         proofs[i],
				})
			}
		} else {
			return fmt.Errorf("could not find transaction %s in the pool", txData.Hash().String())
		}
	}

	// Verify that the calculated amount of blobs in the payload matches the
	// amount of blobs in the bundle
	if len(blobDataInPayload) != len(blobBundle.Blobs) {
		return fmt.Errorf("expected %d blobs in the bundle, got %d", len(blobDataInPayload), len(blobBundle.Blobs))
	}

	for i, blobData := range blobDataInPayload {
		bundleKzg := blobBundle.KZGs[i]
		bundleBlob := blobBundle.Blobs[i]
		if !bytes.Equal(bundleKzg[:], blobData.KZG[:]) {
			return fmt.Errorf("KZG mismatch at index %d", i)
		}
		if !bytes.Equal(bundleBlob[:], blobData.Blob[:]) {
			return fmt.Errorf("blob mismatch at index %d", i)
		}
	}

	return nil
}

func (pool *TestBlobTxPool) AddBlobTransaction(tx *types.Transaction) {
	if pool.Transactions == nil {
		pool.Transactions = make(map[common.Hash]*types.Transaction)
	}
	pool.Transactions[tx.Hash()] = tx
}

// Test two different transactions with the same blob, and check the blob bundle.

func VerifyTransactionFromNode(ctx context.Context, eth client.Eth, tx *types.Transaction) error {
	returnedTx, _, err := eth.TransactionByHash(ctx, tx.Hash())
	if err != nil {
		return err
	}

	// Verify that the tx fields are all the same
	if returnedTx.Nonce() != tx.Nonce() {
		return fmt.Errorf("nonce mismatch: %d != %d", returnedTx.Nonce(), tx.Nonce())
	}
	if returnedTx.Gas() != tx.Gas() {
		return fmt.Errorf("gas mismatch: %d != %d", returnedTx.Gas(), tx.Gas())
	}
	if returnedTx.GasPrice().Cmp(tx.GasPrice()) != 0 {
		return fmt.Errorf("gas price mismatch: %d != %d", returnedTx.GasPrice(), tx.GasPrice())
	}
	if returnedTx.Value().Cmp(tx.Value()) != 0 {
		return fmt.Errorf("value mismatch: %d != %d", returnedTx.Value(), tx.Value())
	}
	if returnedTx.To() != nil && tx.To() != nil && returnedTx.To().Hex() != tx.To().Hex() {
		return fmt.Errorf("to mismatch: %s != %s", returnedTx.To().Hex(), tx.To().Hex())
	}
	if returnedTx.Data() != nil && tx.Data() != nil && !bytes.Equal(returnedTx.Data(), tx.Data()) {
		return fmt.Errorf("data mismatch: %s != %s", hex.EncodeToString(returnedTx.Data()), hex.EncodeToString(tx.Data()))
	}
	if returnedTx.AccessList() != nil && tx.AccessList() != nil && !reflect.DeepEqual(returnedTx.AccessList(), tx.AccessList()) {
		return fmt.Errorf("access list mismatch: %v != %v", returnedTx.AccessList(), tx.AccessList())
	}
	if returnedTx.ChainId().Cmp(tx.ChainId()) != 0 {
		return fmt.Errorf("chain id mismatch: %d != %d", returnedTx.ChainId(), tx.ChainId())
	}
	if returnedTx.DataGas().Cmp(tx.DataGas()) != 0 {
		return fmt.Errorf("data gas mismatch: %d != %d", returnedTx.DataGas(), tx.DataGas())
	}
	if returnedTx.GasFeeCapCmp(tx) != 0 {
		return fmt.Errorf("max fee per gas mismatch: %d != %d", returnedTx.GasFeeCap(), tx.GasFeeCap())
	}
	if returnedTx.GasTipCapCmp(tx) != 0 {
		return fmt.Errorf("max priority fee per gas mismatch: %d != %d", returnedTx.GasTipCap(), tx.GasTipCap())
	}
	if returnedTx.MaxFeePerDataGas().Cmp(tx.MaxFeePerDataGas()) != 0 {
		return fmt.Errorf("max fee per data gas mismatch: %d != %d", returnedTx.MaxFeePerDataGas(), tx.MaxFeePerDataGas())
	}
	if returnedTx.DataHashes() != nil && tx.DataHashes() != nil && !reflect.DeepEqual(returnedTx.DataHashes(), tx.DataHashes()) {
		return fmt.Errorf("blob versioned hashes mismatch: %v != %v", returnedTx.DataHashes(), tx.DataHashes())
	}
	if returnedTx.Type() != tx.Type() {
		return fmt.Errorf("type mismatch: %d != %d", returnedTx.Type(), tx.Type())
	}

	return nil
}

// Base test case execution procedure for blobs tests.
func (bs *BlobsBaseSpec) Execute(t *test.Env) {

	t.CLMock.WaitForTTD()
	addr := common.BigToAddress(DATAHASH_START_ADDRESS)

	// Create the internal test blob transaction pool
	testBlobTxPool := new(TestBlobTxPool)

	for payloadId, testBlobBlock := range bs.TestBlobPayloads {
		t.Logf("INFO: Preparing blob payload %d", payloadId+1)

		//  Send the blob transactions
		for bTx := uint64(0); bTx < testBlobBlock.BlobTransactionSendCount; bTx++ {
			// Send the blob transaction
			blobTx, err := helper.SendNextTransaction(t.TestContext, t.Engine,
				&helper.BlobTransactionCreator{
					To:         &addr,
					GasLimit:   100000,
					DataGasFee: testBlobBlock.BlobTransactionMaxDataGasCost,
				},
			)
			if err != nil {
				t.Fatalf("FAIL: Error sending blob transaction: %v", err)
			}
			VerifyTransactionFromNode(t.TestContext, t.Engine, blobTx)
			testBlobTxPool.AddBlobTransaction(blobTx)
			t.Logf("INFO: Sent blob transaction: %s", blobTx.Hash().String())
		}

		// Produce the payload
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnGetPayload: func() {
				// Get the blobs bundle from the node too
				ctx, cancel := context.WithTimeout(t.TestContext, 10*time.Second)
				defer cancel()

				blobBundle, err := t.Engine.GetBlobsBundleV1(ctx, t.CLMock.NextPayloadID)
				if err != nil {
					t.Fatalf("FAIL: Error getting blobs bundle: %v", err)
				}

				payload := &t.CLMock.LatestPayloadBuilt

				if err := testBlobTxPool.VerifyBlobBundle(payload, blobBundle, int(testBlobBlock.ExpectedIncludedBlobCount)); err != nil {
					t.Fatalf("FAIL: Error verifying blob bundle: %v", err)
				}
			},
		})
	}

}
