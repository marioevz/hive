package suite_base

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-testnet",
	DisplayName: "Deneb Testnet",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb.`,
	Location:    "suites/base",
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		BaseTestSpec{
			Name:        "test-deneb-fork",
			DisplayName: "Deneb Fork",
			Description: `
			Sanity test to check the fork transition to deneb.
			- Start two validating nodes that begin on Capella/Shanghai genesis
			- Deneb/Cancun transition occurs on Epoch 1
			- Total of 128 Validators, 64 for each validating node
			- Wait for Deneb fork and start sending blob transactions to the Execution client
			- Verify on the execution client that:
			  - Blob (type-3) transactions are included in the blocks
			- Verify on the consensus client that:
			  - For each blob transaction on the execution chain, the blob sidecars are available for the
				beacon block at the same height
			  - The beacon block lists the correct commitments for each blob
			`,
			DenebGenesis: false,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			EpochsAfterFork: 1,
		},
		BaseTestSpec{
			Name:        "test-deneb-genesis",
			DisplayName: "Deneb Genesis",
			Description: `
			Sanity test to check the beacon clients can start with deneb genesis.
			
			- Start two validating nodes that begin on Deneb genesis
			- Total of 128 Validators, 64 for each validating node
			- From the beginning send blob transactions to the Execution client
			- Verify on the execution client that:
			  - Blob (type-3) transactions are included in the blocks
			- Verify on the consensus client that:
			  - For each blob transaction on the execution chain, the blob sidecars are available for the
				beacon block at the same height
			  - The beacon block lists the correct commitments for each blob
			`,
			DenebGenesis: true,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			WaitForFinality: true,
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
