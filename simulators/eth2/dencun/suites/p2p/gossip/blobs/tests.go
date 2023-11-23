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
	DisplayName: "Deneb P2P Blobs Gossip",
	Description: `Collection of test vectors that verify client behavior under different blob gossiping scenarios.`,
	Location:    "suites/p2p/gossip/blobs",
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.Default{},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "blob-gossiping-sanity",
				DisplayName: "Blob Gossiping Sanity",
				Description: `
		Sanity test where the blobber is verified to be working correctly
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.Default{
				BroadcastBlobsFirst: true,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "blob-gossiping-before-block",
				DisplayName: "Blob Gossiping Before Block",
				Description: `
		Test chain health where the blobs are gossiped before the block
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.BlobGossipDelay{
				DelayMilliseconds: 500,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "blob-gossiping-delay",
				DisplayName: "Blob Gossiping Delay",
				Description: `
		Test chain health where the blobs are gossiped after the block with a 500ms delay
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.BlobGossipDelay{
				DelayMilliseconds: 6000,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "blob-gossiping-one-slot-delay",
				DisplayName: "Blob Gossiping One-Slot Delay",
				Description: `
		Test chain health where the blobs are gossiped after the block with a 6s delay
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
			// A slot might be missed due to blobs arriving late
			BlobberActionCausesMissedSlot: true,
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.EquivocatingBlock{
				CorrectBlockDelayMilliseconds: 500,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "equivocating-block",
				DisplayName: "Equivocating Block",
				Description: `
		Test chain health a proposer sends an equivocating block before the correct block.
		Blob sidecars contain the correct block header.
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
			// Action should not cause missed slots since the blob sidecars and the block are available
			BlobberActionCausesMissedSlot: false,
		},
		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.EquivocatingBlockAndBlobs{
				BroadcastBlobsFirst: true,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "equivocating-block-and-blobs",
				DisplayName: "Equivocating Block and Blobs",
				Description: `
		Test chain health a proposer sends equivocating blobs and block to different peers
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
			// A slot might be missed due to re-orgs
			BlobberActionCausesMissedSlot: true,
		},

		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.EquivocatingBlockHeaderInBlobs{
				BroadcastBlobsFirst: false,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "equivocating-block-header-in-blob-sidecars",
				DisplayName: "Equivocating Block Header in Blob Sidecars",
				Description: `
		Test chain health a proposer sends equivocating blob sidecars (equivocating block header), but the correct full block is sent first.
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
			// Slot is missed because the blob with the correct header are never sent
			BlobberActionCausesMissedSlot: true,
		},

		P2PBlobsGossipTestSpec{
			BlobberSlotAction: blobber_slot_actions.EquivocatingBlockHeaderInBlobs{
				BroadcastBlobsFirst: true,
			},
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "equivocating-block-header-in-blob-sidecars-2",
				DisplayName: "Equivocating Block Header in Blob Sidecars 2",
				Description: `
		Test chain health a proposer sends equivocating blob sidecars (equivocating block header), and the correct full block is sent afterwards.
		`,
				DenebGenesis: true,
				GenesisExecutionWithdrawalCredentialsShares: 1,
			},
			// Slot is missed because the blob with the correct header are never sent
			BlobberActionCausesMissedSlot: true,
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
