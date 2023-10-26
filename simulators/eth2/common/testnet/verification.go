package testnet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"

	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/eth2/common/utils"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	"github.com/protolambda/eth2api"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
)

// Interface to specify on which slot the verification will be performed
type VerificationSlot interface {
	Slot(
		ctx context.Context,
		t *Testnet,
		bn *beacon_client.BeaconClient,
	) (common.Slot, error)
}

// Return the slot at the start of the checkpoint's following epoch
type FirstSlotAfterCheckpoint struct {
	*common.Checkpoint
}

var _ VerificationSlot = FirstSlotAfterCheckpoint{}

func (c FirstSlotAfterCheckpoint) Slot(
	ctx context.Context,
	t *Testnet,
	_ *beacon_client.BeaconClient,
) (common.Slot, error) {
	return t.Spec().EpochStartSlot(c.Checkpoint.Epoch + 1)
}

// Return the slot at the end of a checkpoint
type LastSlotAtCheckpoint struct {
	*common.Checkpoint
}

var _ VerificationSlot = LastSlotAtCheckpoint{}

func (c LastSlotAtCheckpoint) Slot(
	ctx context.Context,
	t *Testnet,
	_ *beacon_client.BeaconClient,
) (common.Slot, error) {
	return t.Spec().SLOTS_PER_EPOCH * common.Slot(c.Checkpoint.Epoch), nil
}

// Get last slot according to current time
type LastestSlotByTime struct{}

var _ VerificationSlot = LastestSlotByTime{}

func (l LastestSlotByTime) Slot(
	ctx context.Context,
	t *Testnet,
	_ *beacon_client.BeaconClient,
) (common.Slot, error) {
	return t.Spec().
			TimeToSlot(common.Timestamp(time.Now().Unix()), t.GenesisTime()),
		nil
}

// Get last slot according to current head of a beacon node
type LastestSlotByHead struct{}

var _ VerificationSlot = LastestSlotByHead{}

func (l LastestSlotByHead) Slot(
	ctx context.Context,
	t *Testnet,
	bn *beacon_client.BeaconClient,
) (common.Slot, error) {
	headInfo, err := bn.BlockHeader(ctx, eth2api.BlockHead)
	if err != nil {
		return common.Slot(0), fmt.Errorf("failed to poll head: %v", err)
	}
	return headInfo.Header.Message.Slot, nil
}

// VerifyParticipation ensures that the participation of the finialized epoch
// of a given checkpoint is above the expected threshold.
func (t *Testnet) VerifyParticipation(
	parentCtx context.Context,
	vs VerificationSlot,
	expected float64,
) error {
	runningNodes := t.VerificationNodes().Running()
	slot, err := vs.Slot(parentCtx, t, runningNodes[0].BeaconClient)
	if err != nil {
		return err
	}
	if t.Spec().BELLATRIX_FORK_EPOCH <= t.Spec().SlotToEpoch(slot) {
		// slot-1 to target last slot in finalized epoch
		slot = slot - 1
	}
	for i, n := range runningNodes {
		health, err := GetHealth(
			parentCtx,
			n.BeaconClient,
			t.Spec().Spec,
			slot,
		)
		if err != nil {
			return err
		}
		if health < expected {
			return fmt.Errorf(
				"node %d (%s): participation not healthy (got:%.2f, want:%.2f)",
				i,
				n.ClientNames(),
				health,
				expected,
			)
		}
		t.Logf(
			"node %d (%s): epoch=%d participation=%.2f",
			i,
			n.ClientNames(),
			t.Spec().SlotToEpoch(slot),
			health,
		)
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadIsCanonical(
	parentCtx context.Context,
	vs VerificationSlot,
) error {
	runningNodes := t.VerificationNodes().Running()
	b := runningNodes[0].BeaconClient
	slot, err := vs.Slot(parentCtx, t, b)
	if err != nil {
		return err
	}

	versionedBlock, err := b.BlockV2(
		parentCtx,
		eth2api.BlockIdSlot(slot),
	)
	if err != nil {
		return fmt.Errorf(
			"node %d (%s): failed to retrieve block: %v",
			0,
			runningNodes[0].ClientNames(),
			err,
		)
	}

	payload, _, _, err := versionedBlock.ExecutionPayload()
	if err != nil {
		return err
	}

	for i, n := range runningNodes {
		ec := n.ExecutionClient
		if block, err := ec.BlockByNumber(
			parentCtx,
			big.NewInt(int64(payload.Number)),
		); err != nil {
			return fmt.Errorf("eth1 %d: %s", 0, err)
		} else {
			blockHash := block.Hash()
			if !bytes.Equal(blockHash[:], payload.BlockHash[:]) {
				return fmt.Errorf(
					"node %d (%s): execution blocks don't match (got=%s, expected=%s)",
					i,
					n.ClientNames(),
					utils.Shorten(blockHash.String()),
					utils.Shorten(payload.BlockHash.String()),
				)
			}
		}
	}
	return nil
}

// VerifyExecutionPayloadIsCanonical retrieves the execution payload from the
// finalized block and verifies that is in the execution client's canonical
// chain.
func (t *Testnet) VerifyExecutionPayloadHashInclusion(
	parentCtx context.Context,
	vs VerificationSlot,
	hash ethcommon.Hash,
) (*beacon_client.VersionedSignedBeaconBlock, error) {
	for _, bn := range t.VerificationNodes().BeaconClients().Running() {
		b, err := t.VerifyExecutionPayloadHashInclusionNode(
			parentCtx,
			vs,
			bn,
			hash,
		)
		if err != nil || b != nil {
			return b, err
		}
	}
	return nil, nil
}

func (t *Testnet) VerifyExecutionPayloadHashInclusionNode(
	parentCtx context.Context,
	vs VerificationSlot,
	bn *beacon_client.BeaconClient,
	hash ethcommon.Hash,
) (*beacon_client.VersionedSignedBeaconBlock, error) {
	lastSlot, err := vs.Slot(parentCtx, t, bn)
	if err != nil {
		return nil, err
	}
	for slot := lastSlot; slot > 0; slot -= 1 {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			continue
		}
		if executionPayload, _, _, err := versionedBlock.ExecutionPayload(); err != nil {
			// Block can't contain an executable payload
			break
		} else if bytes.Equal(executionPayload.BlockHash[:], hash[:]) {
			return versionedBlock, nil
		}

	}
	return nil, nil
}

// VerifyProposers checks that all validator clients have proposed a block on
// the finalized beacon chain that includes an execution payload.
func (t *Testnet) VerifyProposers(
	parentCtx context.Context,
	vs VerificationSlot,
	allow_empty_blocks bool,
) error {
	runningNodes := t.VerificationNodes().Running()
	bn := runningNodes[0].BeaconClient
	lastSlot, err := vs.Slot(parentCtx, t, bn)
	if err != nil {
		return err
	}
	proposers := make([]bool, len(runningNodes))
	for slot := common.Slot(0); slot <= lastSlot; slot += 1 {
		versionedBlock, err := bn.BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			if allow_empty_blocks {
				continue
			}
			return fmt.Errorf(
				"node %d (%s): failed to retrieve beacon block: %v",
				0,
				runningNodes[0].ClientNames(),
				err,
			)
		}

		validator, err := bn.StateValidator(
			parentCtx,
			eth2api.StateIdSlot(slot),
			eth2api.ValidatorIdIndex(versionedBlock.ProposerIndex()),
		)
		if err != nil {
			return fmt.Errorf(
				"node %d (%s): failed to retrieve validator: %v",
				0,
				runningNodes[0].ClientNames(),
				err,
			)
		}
		idx, err := t.ValidatorClientIndex(
			[48]byte(validator.Validator.Pubkey),
		)
		if err != nil {
			return fmt.Errorf("pub key not found on any validator client")
		}
		proposers[idx] = true
	}
	for i, proposed := range proposers {
		if !proposed {
			return fmt.Errorf(
				"node %d (%s): did not propose a block",
				i,
				runningNodes[i].ClientNames(),
			)
		}
	}
	return nil
}

func (t *Testnet) VerifyELBlockLabels(parentCtx context.Context) error {
	runningNodes := t.VerificationNodes().Running()
	for i := 0; i < len(runningNodes); i++ {
		n := runningNodes[i]
		el := n.ExecutionClient
		bn := n.BeaconClient
		// Get the head
		headInfo, err := bn.BlockHeader(parentCtx, eth2api.BlockHead)
		if err != nil {
			return err
		}

		// Get the checkpoints, first try querying state root, then slot number
		checkpoints, err := bn.BlockFinalityCheckpoints(
			parentCtx,
			eth2api.BlockHead,
		)
		if err != nil {
			return err
		}
		blockLabels := map[string]tree.Root{
			"latest":    headInfo.Root,
			"finalized": checkpoints.Finalized.Root,
			"safe":      checkpoints.CurrentJustified.Root,
		}

		for label, root := range blockLabels {
			// Get the beacon block
			versionedBlock, err := bn.BlockV2(
				parentCtx,
				eth2api.BlockIdRoot(root),
			)
			if err != nil {
				return err
			}
			if executionPayload, _, _, err := versionedBlock.ExecutionPayload(); err != nil {
				// Get the el block and compare
				h, err := el.HeaderByLabel(parentCtx, label)
				if err != nil {
					if executionPayload.BlockHash != (ethcommon.Hash{}) {
						return err
					}
				} else {
					if h.Hash() != executionPayload.BlockHash {
						return fmt.Errorf(
							"node %d (%s): Execution hash found in checkpoint block "+
								"(%s) does not match what the el returns: %v != %v",
							i,
							n.ClientNames(),
							label,
							executionPayload.BlockHash,
							h.Hash(),
						)
					}
					fmt.Printf(
						"node %d (%s): Execution hash matches beacon "+
							"checkpoint block (%s) information: %v\n",
						i,
						n.ClientNames(),
						label,
						h.Hash(),
					)
				}
			}

		}
	}
	return nil
}

func (t *Testnet) VerifyELHeads(
	parentCtx context.Context,
) error {
	runningExecution := t.VerificationNodes().ExecutionClients().Running()
	head, err := runningExecution[0].HeaderByNumber(parentCtx, nil)
	if err != nil {
		return err
	}

	t.Logf("Verifying EL heads at %v", head.Hash())
	for i, node := range runningExecution {
		head2, err := node.HeaderByNumber(parentCtx, nil)
		if err != nil {
			return err
		}
		if head.Hash() != head2.Hash() && head.ParentHash != head2.Hash() && head.Hash() != head2.ParentHash {
			return fmt.Errorf(
				"different heads: %v: %v %v: %v",
				0,
				head,
				i,
				head2,
			)
		}
	}
	return nil
}

func (t *Testnet) VerifyBlobs(
	parentCtx context.Context,
	vs VerificationSlot,
) (uint64, error) {
	nodes := t.VerificationNodes().Running()
	beaconClients := nodes.BeaconClients()
	if len(nodes) == 0 {
		return 0, fmt.Errorf("no beacon clients running")
	}
	if len(nodes) == 1 {
		return 0, fmt.Errorf("only one beacon client running, can't verify blobs")
	}
	var blobCount uint64

	lastSlot, err := vs.Slot(parentCtx, t, beaconClients[0])
	if err != nil {
		return 0, err
	}
	for slot := lastSlot; slot > 0; slot -= 1 {

		versionedBlock, err := beaconClients[0].BlockV2(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			continue
		}
		if !versionedBlock.ContainsKZGCommitments() {
			// Block can't contain blobs before deneb
			break
		}

		// Get the execution block from the execution client
		executionPayload, _, _, err := versionedBlock.ExecutionPayload()
		if err != nil {
			panic(err)
		}
		executionBlock, err := nodes[0].ExecutionClient.BlockByHash(parentCtx, executionPayload.BlockHash)
		if err != nil {
			panic(err)
		}
		blockKzgCommitments := versionedBlock.KZGCommitments()

		refSidecars, err := beaconClients[0].BlobSidecars(parentCtx, eth2api.BlockIdSlot(slot))
		if err != nil {
			return 0, fmt.Errorf(
				"node %d (%s): failed to retrieve blobs for slot %d: %v",
				0,
				t.VerificationNodes().Running()[0].ClientNames(),
				slot,
				err,
			)
		}
		blobCount += uint64(len(refSidecars))

		if len(refSidecars) != len(blockKzgCommitments) {
			return 0, fmt.Errorf(
				"node %d (%s): block kzg commitments and sidecars length differ (sidecar count=%d, block kzg commitments=%d)",
				0,
				t.VerificationNodes().Running()[0].ClientNames(),
				len(refSidecars),
				len(blockKzgCommitments),
			)
		}

		// Verify against the execution block transactions
		executionBlockHashesCount := 0
		for _, tx := range executionBlock.Transactions() {
			blobHashes := tx.BlobHashes()
			versionedHashVersion := byte(1)
			if len(blobHashes) > 0 {
				for _, blobHash := range blobHashes {
					if executionBlockHashesCount < len(blockKzgCommitments) {
						// Sha256 the kzg commitment and modify the first byte to be the version
						hasher := sha256.New()
						hasher.Write(blockKzgCommitments[executionBlockHashesCount][:])
						kzgHash := hasher.Sum(nil)
						if !bytes.Equal(blobHash[1:], kzgHash[1:]) {
							return 0, fmt.Errorf(
								"node %d (%s): block kzg commitments and execution block hashes differ (block kzg commitment=%x, execution block hash=%x)",
								0,
								t.VerificationNodes().Running()[0].ClientNames(),
								blockKzgCommitments[executionBlockHashesCount][:],
								blobHash[:],
							)
						}
						if blobHash[0] != versionedHashVersion {
							return 0, fmt.Errorf(
								"node %d (%s): execution blob hash does not contain the correct version: %d",
								0,
								t.VerificationNodes().Running()[0].ClientNames(),
								blobHash[0],
							)
						}
					} // else: test will fail after the loop, but we need to keep counting the hashes
					executionBlockHashesCount++
				}
			}
		}
		if executionBlockHashesCount != len(blockKzgCommitments) {
			return 0, fmt.Errorf(
				"node %d (%s): block kzg commitments and execution block hashes length differ (block kzg commitment count=%d, execution block hash count=%d)",
				0,
				t.VerificationNodes().Running()[0].ClientNames(),
				len(blockKzgCommitments),
				executionBlockHashesCount,
			)
		}

		for i := 1; i < len(beaconClients); i++ {
			// Check the reference client against the other clients, and verify that all clients return the same blobs
			//  Also keep count of blobs per client
			bn := beaconClients[i]
			// Get the sidecars for this client
			sidecars, err := bn.BlobSidecars(parentCtx, eth2api.BlockIdSlot(slot))
			if err != nil {
				// We already got some blobs from the reference client, so we should not get an error here
				return 0, err
			}
			if len(sidecars) != len(refSidecars) {
				return 0, fmt.Errorf(
					"node %d (%s): different number of blobs (got=%d, expected=%d)",
					i,
					t.VerificationNodes().Running()[i].ClientNames(),
					len(sidecars),
					len(refSidecars),
				)
			}
			for j := 0; j < len(sidecars); j++ {
				if !bytes.Equal(sidecars[j].Blob[:], refSidecars[j].Blob[:]) {
					return 0, fmt.Errorf(
						"node %d (%s): different blobs\ngot=%x\nexpected=%x",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].Blob[:],
						refSidecars[j].Blob[:],
					)
				}
				// Verify the commitments
				if !bytes.Equal(sidecars[j].KZGCommitment[:], refSidecars[j].KZGCommitment[:]) {
					return 0, fmt.Errorf(
						"node %d (%s): different commitments\ngot=%x\nexpected=%x",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].KZGCommitment[:],
						refSidecars[j].KZGCommitment[:],
					)
				}
				// Verify the proofs
				if !bytes.Equal(sidecars[j].KZGProof[:], refSidecars[j].KZGProof[:]) {
					return 0, fmt.Errorf(
						"node %d (%s): different proofs\ngot=%x\nexpected=%x",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].KZGProof[:],
						refSidecars[j].KZGProof[:],
					)
				}
				// Verify the block roots
				if !bytes.Equal(sidecars[j].BlockRoot[:], refSidecars[j].BlockRoot[:]) {
					return 0, fmt.Errorf(
						"node %d (%s): different block roots\ngot=%x\nexpected=%x",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].BlockRoot[:],
						refSidecars[j].BlockRoot[:],
					)
				}
				// Verify the blob index
				if sidecars[j].Index != refSidecars[j].Index || uint64(sidecars[j].Index) != uint64(j) {
					return 0, fmt.Errorf(
						"node %d (%s): different blob indices\ngot=%d\nexpected=%d",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].Index,
						j,
					)
				}
				// Verify the slot number
				if sidecars[j].Slot != refSidecars[j].Slot || uint64(sidecars[j].Slot) != uint64(slot) {
					return 0, fmt.Errorf(
						"node %d (%s): different slot numbers\ngot=%d\nexpected=%d",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].Slot,
						slot,
					)
				}
				// Verify the block parent root
				if !bytes.Equal(sidecars[j].BlockParentRoot[:], refSidecars[j].BlockParentRoot[:]) {
					return 0, fmt.Errorf(
						"node %d (%s): different block parent roots\ngot=%x\nexpected=%x",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].BlockParentRoot[:],
						refSidecars[j].BlockParentRoot[:],
					)
				}
				// Verify the proposer index
				if sidecars[j].ProposerIndex != refSidecars[j].ProposerIndex {
					return 0, fmt.Errorf(
						"node %d (%s): different proposer indices\ngot=%d\nexpected=%d",
						i,
						t.VerificationNodes().Running()[i].ClientNames(),
						sidecars[j].ProposerIndex,
						refSidecars[j].ProposerIndex,
					)
				}
			}
		}
	}
	return blobCount, nil
}
