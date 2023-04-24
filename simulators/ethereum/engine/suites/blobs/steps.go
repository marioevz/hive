package suite_blobs

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type BlobTestContext struct {
	*test.Env
	*TestBlobTxPool
	CurrentBlobID helper.BlobID
}

// Interface to represent a single step in a blob test
type BlobTestStep interface {
	// Executes the step
	Execute(testCtx *BlobTestContext) error
	Description() string
}

type BlobTestSequence []BlobTestStep

// A step that runs two or more steps in parallel
type ParallelSteps struct {
	Steps []BlobTestStep
}

func (step ParallelSteps) Execute(t *BlobTestContext) error {
	// Run the steps in parallel
	wg := sync.WaitGroup{}
	errs := make(chan error, len(step.Steps))
	for _, s := range step.Steps {
		wg.Add(1)
		go func(s BlobTestStep) {
			defer wg.Done()
			if err := s.Execute(t); err != nil {
				errs <- err
			}
		}(s)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

// A step that launches a new client
type LaunchClient struct {
	client.EngineStarter
	SkipConnectingToBootnode bool
	SkipAddingToCLMock       bool
}

func (step LaunchClient) Execute(t *BlobTestContext) error {
	// Launch a new client
	var (
		client client.EngineClient
		err    error
	)
	if !step.SkipConnectingToBootnode {
		client, err = step.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engines[0])
	} else {
		client, err = step.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles)
	}
	if err != nil {
		return err
	}
	t.Engines = append(t.Engines, client)
	if !step.SkipAddingToCLMock {
		t.CLMock.AddEngineClient(client)
	}
	return nil
}

func (step LaunchClient) Description() string {
	return "Launch new engine client"
}

// A step that sends a new payload to the client
type NewPayloads struct {
	// Payload Count
	PayloadCount uint64
	// Number of blob transactions that are expected to be included in the payload
	ExpectedIncludedBlobCount uint64
	// Blob IDs expected to be found in the payload
	ExpectedBlobs []helper.BlobID
}

func (step NewPayloads) GetPayloadCount() uint64 {
	payloadCount := step.PayloadCount
	if payloadCount == 0 {
		payloadCount = 1
	}
	return payloadCount
}

func (step NewPayloads) VerifyBlobBundle(pool *TestBlobTxPool, payload *engine.ExecutableData, blobBundle *engine.BlobsBundle) error {
	if len(blobBundle.Blobs) != int(step.ExpectedIncludedBlobCount) {
		return fmt.Errorf("expected %d blob, got %d", step.ExpectedIncludedBlobCount, len(blobBundle.Blobs))
	}
	if len(blobBundle.Commitments) != int(step.ExpectedIncludedBlobCount) {
		return fmt.Errorf("expected %d KZG, got %d", step.ExpectedIncludedBlobCount, len(blobBundle.Commitments))
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
		bundleCommitment := blobBundle.Commitments[i]
		bundleBlob := blobBundle.Blobs[i]
		bundleProof := blobBundle.Proofs[i]
		if !bytes.Equal(bundleCommitment[:], blobData.KZG[:]) {
			return fmt.Errorf("KZG mismatch at index %d of the bundle", i)
		}
		if !bytes.Equal(bundleBlob[:], blobData.Blob[:]) {
			return fmt.Errorf("blob mismatch at index %d of the bundle", i)
		}
		if !bytes.Equal(bundleProof[:], blobData.Proof[:]) {
			return fmt.Errorf("proof mismatch at index %d of the bundle", i)
		}
	}

	if len(step.ExpectedBlobs) != 0 {
		// Verify that the blobs in the payload match the expected blobs
		for _, expectedBlob := range step.ExpectedBlobs {
			found := false
			for _, blobData := range blobDataInPayload {
				if ok, err := expectedBlob.VerifyBlob(&blobData.Blob); err != nil {
					return err
				} else if ok {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("could not find expected blob %d", expectedBlob)
			}
		}
	}

	return nil
}

func (step NewPayloads) Execute(t *BlobTestContext) error {
	// Create a new payload
	// Produce the payload
	payloadCount := step.GetPayloadCount()
	for p := uint64(0); p < payloadCount; p++ {
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnGetPayload: func() {
				// Get the latest blob bundle
				blobBundle := t.CLMock.LatestBlobBundle
				if blobBundle == nil {
					t.Fatalf("FAIL: Error getting blobs bundle: %v", blobBundle)
				}

				payload := &t.CLMock.LatestPayloadBuilt

				if err := step.VerifyBlobBundle(t.TestBlobTxPool, payload, blobBundle); err != nil {
					t.Fatalf("FAIL: Error verifying blob bundle: %v", err)
				}
			},
		})
	}
	return nil
}

func (step NewPayloads) Description() string {
	return fmt.Sprintf("NewPayloads: %d payloads, %d blobs expected", step.GetPayloadCount(), step.ExpectedIncludedBlobCount)
}

// A step that sends multiple new blobs to the client
type SendBlobTransactions struct {
	// Number of blob transactions to send before this block's GetPayload request
	BlobTransactionSendCount uint64
	// Blobs per transaction
	BlobsPerTransaction uint64
	// Max Data Gas Cost for every blob transaction
	BlobTransactionMaxDataGasCost *big.Int
	// Gas Fee Cap for every blob transaction
	BlobTransactionGasFeeCap *big.Int
	// Gas Tip Cap for every blob transaction
	BlobTransactionGasTipCap *big.Int
	// Replace transactions
	ReplaceTransactions bool
	// Account index to send the blob transactions from
	AccountIndex uint64
	// Client index to send the blob transactions to
	ClientIndex uint64
}

func (step SendBlobTransactions) GetBlobsPerTransaction() uint64 {
	blobCountPerTx := step.BlobsPerTransaction
	if blobCountPerTx == 0 {
		blobCountPerTx = 1
	}
	return blobCountPerTx
}

func (step SendBlobTransactions) Execute(t *BlobTestContext) error {
	// Send a blob transaction
	addr := common.BigToAddress(DATAHASH_START_ADDRESS)
	blobCountPerTx := step.GetBlobsPerTransaction()
	var engine client.EngineClient
	if step.ClientIndex >= uint64(len(t.Engines)) {
		return fmt.Errorf("invalid client index %d", step.ClientIndex)
	}
	engine = t.Engines[step.ClientIndex]
	//  Send the blob transactions
	for bTx := uint64(0); bTx < step.BlobTransactionSendCount; bTx++ {
		blobTxCreator := &helper.BlobTransactionCreator{
			To:         &addr,
			GasLimit:   100000,
			GasTip:     step.BlobTransactionGasTipCap,
			DataGasFee: step.BlobTransactionMaxDataGasCost,
			BlobCount:  blobCountPerTx,
			BlobID:     t.CurrentBlobID,
		}
		if step.AccountIndex != 0 {
			if step.AccountIndex >= uint64(len(globals.TestAccounts)) {
				return fmt.Errorf("invalid account index %d", step.AccountIndex)
			}
			key := globals.TestAccounts[step.AccountIndex].GetKey()
			blobTxCreator.PrivateKey = key
		}
		var (
			blobTx *types.Transaction
			err    error
		)
		if step.ReplaceTransactions {
			blobTx, err = helper.ReplaceLastTransaction(t.TestContext, engine,
				blobTxCreator,
			)
		} else {
			blobTx, err = helper.SendNextTransaction(t.TestContext, engine,
				blobTxCreator,
			)
		}
		if err != nil {
			t.Fatalf("FAIL: Error sending blob transaction: %v", err)
		}
		VerifyTransactionFromNode(t.TestContext, engine, blobTx)
		t.AddBlobTransaction(blobTx)
		t.Logf("INFO: Sent blob transaction: %s", blobTx.Hash().String())
		t.CurrentBlobID += helper.BlobID(blobCountPerTx)
	}
	return nil
}

func (step SendBlobTransactions) Description() string {
	return fmt.Sprintf("SendBlobTransactions: %d Transactions, %d blobs each, %d max data gas fee", step.BlobTransactionSendCount, step.GetBlobsPerTransaction(), step.BlobTransactionMaxDataGasCost.Uint64())
}
