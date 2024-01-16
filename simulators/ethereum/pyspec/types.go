package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"

	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

type testcase struct {
	// test meta data
	name       string
	filepath   string
	clientType string
	failedErr  error
	// test fixture data
	fixture           fixtureTest
	genesis           *core.Genesis
	postAlloc         *core.GenesisAlloc
	engineNewPayloads []engineNewPayload
}

func (t *testcase) Name() string {
	return t.name
}

func (t *testcase) Description() string {
	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Test Link: %s\n", t.RepoLink()))
	if validationError := t.ValidationError(); validationError != nil {
		sb.WriteString(fmt.Sprintf("Validation Error: %s\n", *validationError))
	}
	return sb.String()
}

func (t *testcase) ValidationError() *string {
	for _, engineNewPayload := range t.engineNewPayloads {
		if engineNewPayload.ValidationError != nil {
			return engineNewPayload.ValidationError
		}
	}
	return nil
}

// repoLink coverts a pyspec test path into a github repository link.
func (t *testcase) RepoLink() string {
	// Example: Converts '/fixtures/cancun/eip4844_blobs/blob_txs/invalid_normal_gas.json'
	// into 'tests/cancun/eip4844_blobs/test_blob_txs.py', and appends onto main branch repo link.
	filePath := strings.Replace(t.filepath, "/fixtures", "tests", -1)
	fileDir := filepath.Dir(filePath)
	fileBase := filepath.Base(fileDir)
	fileName := filepath.Join(filepath.Dir(fileDir), "test_"+fileBase+".py")
	repoLink := fmt.Sprintf("https://github.com/ethereum/execution-spec-tests/tree/main/%v", fileName)
	return repoLink
}

type fixtureTest struct {
	json fixtureJSON
}

func (t *fixtureTest) UnmarshalJSON(in []byte) error {
	if err := json.Unmarshal(in, &t.json); err != nil {
		return err
	}
	return nil
}

type fixtureJSON struct {
	Fork              string              `json:"network"`
	Genesis           genesisBlock        `json:"genesisBlockHeader"`
	EngineNewPayloads []engineNewPayload  `json:"engineNewPayloads"`
	EngineFcuVersion  math.HexOrDecimal64 `json:"engineFcuVersion"`
	Pre               core.GenesisAlloc   `json:"pre"`
	Post              core.GenesisAlloc   `json:"postState"`
}

//go:generate go run github.com/fjl/gencodec -type genesisBlock -field-override genesisBlockUnmarshaling -out gen_gb.go
type genesisBlock struct {
	Coinbase      common.Address   `json:"coinbase"`
	Difficulty    *big.Int         `json:"difficulty"`
	GasLimit      uint64           `json:"gasLimit"`
	Timestamp     *big.Int         `json:"timestamp"`
	ExtraData     []byte           `json:"extraData"`
	MixHash       common.Hash      `json:"mixHash"`
	Nonce         types.BlockNonce `json:"nonce"`
	BaseFee       *big.Int         `json:"baseFeePerGas"`
	BlobGasUsed   *uint64          `json:"blobGasUsed"`
	ExcessBlobGas *uint64          `json:"excessBlobGas"`

	Hash common.Hash `json:"hash"`
}

type genesisBlockUnmarshaling struct {
	Difficulty    *math.HexOrDecimal256 `json:"difficulty"`
	GasLimit      math.HexOrDecimal64   `json:"gasLimit"`
	Timestamp     *math.HexOrDecimal256 `json:"timestamp"`
	ExtraData     hexutil.Bytes         `json:"extraData"`
	BaseFee       *math.HexOrDecimal256 `json:"baseFeePerGas"`
	BlobGasUsed   *math.HexOrDecimal64  `json:"dataGasUsed"`
	ExcessBlobGas *math.HexOrDecimal64  `json:"excessDataGas"`
}

type engineNewPayload struct {
	ExecutionPayload      *api.ExecutableData `json:"executionPayload"`
	BlobVersionedHashes   []common.Hash       `json:"expectedBlobVersionedHashes"`
	ParentBeaconBlockRoot *common.Hash        `json:"parentBeaconBlockRoot"`
	Version               math.HexOrDecimal64 `json:"version"`
	ValidationError       *string             `json:"validationError"`
	ErrorCode             int64               `json:"errorCode,string"`

	HiveExecutionPayload *typ.ExecutableData
}
