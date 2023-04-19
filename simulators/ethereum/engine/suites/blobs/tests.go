// # Test suite for blob tests
package suite_blobs

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	"github.com/holiman/uint256"
	"github.com/protolambda/ztyp/view"
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
		BlobsBlockCount: 2, // Genesis is a withdrawals block
		BlobTxsPerBlock: 4,
	},
}

// Blobs base spec
type BlobsBaseSpec struct {
	test.Spec
	TimeIncrements  uint64 // Timestamp increments per block throughout the test
	BlobsForkHeight uint64 // Withdrawals activation fork height
	BlobsBlockCount uint64 // Number of blocks on and after blobs fork activation
	BlobTxsPerBlock uint64 // Number of blob txs per block
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

/*
func (bs *BlobsBaseSpec) VerifyContractsStorage(t *test.Env) {
	if bs.GetTransactionCountPerPayload() < uint64(len(TX_CONTRACT_ADDRESSES)) {
		return
	}
	// Assume that forkchoice updated has been already sent
	latestPayloadNumber := t.CLMock.LatestExecutedPayload.Number
	latestPayloadNumberBig := big.NewInt(int64(latestPayloadNumber))

	r := t.TestEngine.TestStorageAt(WARM_COINBASE_ADDRESS, common.BigToHash(latestPayloadNumberBig), latestPayloadNumberBig)
	p := t.TestEngine.TestStorageAt(PUSH0_ADDRESS, common.Hash{}, latestPayloadNumberBig)
	if latestPayloadNumber >= bs.WithdrawalsForkHeight {
		// Shanghai
		r.ExpectBigIntStorageEqual(big.NewInt(100))        // WARM_STORAGE_READ_COST
		p.ExpectBigIntStorageEqual(latestPayloadNumberBig) // tx succeeded
	} else {
		// Pre-Shanghai
		r.ExpectBigIntStorageEqual(big.NewInt(2600)) // COLD_ACCOUNT_ACCESS_COST
		p.ExpectBigIntStorageEqual(big.NewInt(0))    // tx must've failed
	}
}
*/

// Changes the CL Mocker default time increments of 1 to the value specified
// in the test spec.
func (bs *BlobsBaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	cl.BlockTimestampIncrement = big.NewInt(int64(bs.GetBlockTimeIncrements()))
}

func SendBlobTransaction(parentCtx context.Context, eth client.Eth, to *common.Address, nonce uint64, gaslimit uint64, gasFee uint64, tip uint64, dataGasFee uint64, key *ecdsa.PrivateKey) error {
	// Need tx wrap data that will pass blob verification
	blobData := &types.BlobTxWrapData{
		BlobKzgs: []types.KZGCommitment{
			{0xc0},
		},
		Blobs: []types.Blob{
			{},
		},
	}
	var hashes []common.Hash
	for i := 0; i < len(blobData.BlobKzgs); i++ {
		hashes = append(hashes, blobData.BlobKzgs[i].ComputeVersionedHash())
	}
	_, _, proofs, err := blobData.Blobs.ComputeCommitmentsAndProofs()
	if err != nil {
		return err
	}
	blobData.Proofs = proofs

	var address *types.AddressSSZ
	if to != nil {
		to_ssz := types.AddressSSZ(*to)
		address = &to_ssz
	}
	sbtx := &types.SignedBlobTx{
		Message: types.BlobTxMessage{
			Nonce:               view.Uint64View(nonce),
			GasTipCap:           view.Uint256View(*uint256.NewInt(tip)),
			GasFeeCap:           view.Uint256View(*uint256.NewInt(gasFee)),
			Gas:                 view.Uint64View(gaslimit),
			To:                  types.AddressOptionalSSZ{address},
			Value:               view.Uint256View(*uint256.NewInt(100)),
			Data:                nil,
			AccessList:          nil,
			MaxFeePerDataGas:    view.Uint256View(*uint256.NewInt(dataGasFee)),
			BlobVersionedHashes: hashes,
		},
	}
	sbtx.Message.ChainID.SetFromBig(globals.ChainID)

	if key == nil {
		key = globals.VaultKey
	}

	tx, err := types.SignNewTx(key, types.NewDankSigner(globals.ChainID), sbtx, types.WithTxWrapData(blobData))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel()
	return eth.SendTransaction(ctx, tx)
}

// Base test case execution procedure for blobs tests.
func (bs *BlobsBaseSpec) Execute(t *test.Env) {

	t.CLMock.WaitForTTD()
	addr := common.BigToAddress(DATAHASH_START_ADDRESS)
	SendBlobTransaction(t.TestContext, t.Eth, &addr, 0, 100000, 1, 1, 1, nil)

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
}
