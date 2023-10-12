package suite_blobs_gossip

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	blobber_slot_actions "github.com/marioevz/blobber/slot_actions"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-p2p-blobs-gossip",
	Description: `Collection of test vectors that verify client behavior under different blob gossiping scenarios.`,
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		P2PBlobsGossipTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-blob-gossiping-sanity",
				Description: `
		Sanity test where the blobber is verified to be working correctly
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
				EpochsAfterFork: 1,
				MaxMissedSlots:  3,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.BroadcastBlobsBeforeBlock{},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-blob-gossiping-before-block",
				Description: `
		Test chain health where the blobs are gossiped before the block
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
				EpochsAfterFork: 1,
				MaxMissedSlots:  3,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.BlobGossipDelay{
				DelayMilliseconds: 500,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-blob-gossiping-delay",
				Description: `
		Test chain health where the blobs are gossiped after the block with a 500ms delay
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
				EpochsAfterFork: 1,
				MaxMissedSlots:  3,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.ExtraBlobs{
				BroadcastBlockFirst:     true,
				BroadcastExtraBlobFirst: true,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-blob-gossiping-extra-blob-with-correct-kzg-commitment",
				Description: `
		Test chain health where there is always an extra blob with:
		 - Correct KZG commitment
		 - Correct block root
		 - Correct proposer signature
		 - Broadcasted after the block
		 - Broadcasted before the rest of the blobs
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
				EpochsAfterFork: 1,
				MaxMissedSlots:  3,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.ExtraBlobs{
				BroadcastBlockFirst:     true,
				BroadcastExtraBlobFirst: true,
				IncorrectKZGCommitment:  true,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-blob-gossiping-extra-blob-with-incorrect-kzg-commitment",
				Description: `
		Test chain health where there is always an extra blob with:
		 - Incorrect KZG commitment
		 - Correct block root
		 - Correct proposer signature
		 - Broadcasted after the block
		 - Broadcasted before the rest of the blobs
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
				EpochsAfterFork: 1,
				MaxMissedSlots:  3,
			},
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
