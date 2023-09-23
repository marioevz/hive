package suite_sync

import (
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

type SyncTestSpec struct {
	suite_base.BaseTestSpec

	EpochsToSync beacon.Epoch
}

func (ts SyncTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	// By default we only have one validating client, and the other clients must sync to it
	if ts.BaseTestSpec.ValidatingNodeCount == 0 {
		ts.BaseTestSpec.ValidatingNodeCount = ts.BaseTestSpec.NodeCount - 1
	}

	tc := ts.BaseTestSpec.GetTestnetConfig(allNodeDefinitions)

	// We disable the start of the last node
	tc.NodeDefinitions[len(tc.NodeDefinitions)-1].DisableStartup = true

	return tc
}
