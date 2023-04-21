// # Test suite for blob tests
package suite_blobs

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type BlobTestContext struct {
	*test.Env
	*TestBlobTxPool
	CurrentBlobID uint64
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

// A step that sends a new payload to the client
type NewPayloads struct {
	// Number of blob transactions that are expected to be included in the payload
	ExpectedIncludedBlobCount uint64
	// Payload Count
	PayloadCount uint64
}

func (step NewPayloads) GetPayloadCount() uint64 {
	payloadCount := step.PayloadCount
	if payloadCount == 0 {
		payloadCount = 1
	}
	return payloadCount
}

func (step NewPayloads) Execute(t *BlobTestContext) error {
	// Create a new payload
	// Produce the payload
	payloadCount := step.GetPayloadCount()
	for p := uint64(0); p < payloadCount; p++ {
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

				if err := t.VerifyBlobBundle(payload, blobBundle, int(step.ExpectedIncludedBlobCount)); err != nil {
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
	// Gas Tip Cap for every blob transaction
	BlobTransactionGasTipCap *big.Int
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
	//  Send the blob transactions
	for bTx := uint64(0); bTx < step.BlobTransactionSendCount; bTx++ {
		blobTx, err := helper.SendNextTransaction(t.TestContext, t.Engine,
			&helper.BlobTransactionCreator{
				To:         &addr,
				GasLimit:   100000,
				GasTip:     step.BlobTransactionGasTipCap,
				DataGasFee: step.BlobTransactionMaxDataGasCost,
				BlobCount:  blobCountPerTx,
				BlobId:     t.CurrentBlobID,
			},
		)
		if err != nil {
			t.Fatalf("FAIL: Error sending blob transaction: %v", err)
		}
		VerifyTransactionFromNode(t.TestContext, t.Engine, blobTx)
		t.AddBlobTransaction(blobTx)
		t.Logf("INFO: Sent blob transaction: %s", blobTx.Hash().String())
		t.CurrentBlobID += blobCountPerTx
	}
	return nil
}

func (step SendBlobTransactions) Description() string {
	return fmt.Sprintf("SendBlobTransactions: %d Transactions, %d blobs each, %d max data gas fee", step.BlobTransactionSendCount, step.GetBlobsPerTransaction(), step.BlobTransactionMaxDataGasCost.Uint64())
}
