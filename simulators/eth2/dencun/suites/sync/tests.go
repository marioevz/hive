package suite_sync

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-sync",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb and test syncing of the beacon chain.`,
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		SyncTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-sync-sanity-from-capella",
				Description: `
				Test syncing of the beacon chain by a secondary non-validating client, sync from capella.
				`,
				NodeCount: 3,
				// Wait for 1 epoch after the fork to start the syncing client
				EpochsAfterFork: 1,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
			},
		},
		SyncTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-sync-sanity-from-deneb",
				Description: `
				Test syncing of the beacon chain by a secondary non-validating client, sync from deneb.
				`,
				NodeCount: 3,
				// Wait for 1 epoch after the fork to start the syncing client
				EpochsAfterFork: 1,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
				DenebGenesis: true,
			},
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
