package suite_blobs_gossip

import (
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	blobber_config "github.com/marioevz/blobber/config"
	blobber_slot_actions "github.com/marioevz/blobber/slot_actions"
)

type P2PBlobsGossipTestSpec struct {
	suite_base.BaseTestSpec

	BlobberSlotAction          blobber_slot_actions.SlotAction
	BlobberSlotActionFrequency uint64
}

func (ts P2PBlobsGossipTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	config := ts.BaseTestSpec.GetTestnetConfig(allNodeDefinitions)

	config.EnableBlobber = true
	config.BlobberOptions = []blobber_config.Option{
		blobber_config.WithSlotAction(ts.BlobberSlotAction),
		blobber_config.WithSlotActionFrequency(ts.BlobberSlotActionFrequency),
	}

	return config
}
