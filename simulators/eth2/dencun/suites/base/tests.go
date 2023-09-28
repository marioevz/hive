package suite_base

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-testnet",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb.`,
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		BaseTestSpec{
			Name: "test-deneb-fork",
			Description: `
			Sanity test to check the fork transition to deneb.
			`,
			DenebGenesis: false,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			EpochsAfterDeneb: 1,
		},
		BaseTestSpec{
			Name: "test-deneb-genesis",
			Description: `
		Sanity test to check the beacon clients can start with deneb genesis.
		`,
			DenebGenesis: true,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			EpochsAfterDeneb: 1,
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
