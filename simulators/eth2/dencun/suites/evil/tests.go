package suite_evil

import (
	"math/big"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	"github.com/ethereum/hive/simulators/ethereum/engine/config/cancun"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-evil",
	Description: `Test suite that uses a modified node that sends incorrect blobs on each proposal.`,
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		suite_base.BaseTestSpec{
			Name: "test-invalid-extra-blob",
			Description: `
		Spawns testnet with one validating node that first publishes a modified blobSidecar with a random KZGProof, after 500ms publishes block and all valid blobs, and a second importing node that imports the block and all blobs.
		`,
			DenebGenesis:        true,
			NodeCount:           2,
			ValidatingNodeCount: 1,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			WaitForFinality: true,
			BlobCount:       new(big.Int).SetUint64(cancun.MAX_BLOBS_PER_BLOCK - 1), // Evil teku requires 1 less blob than the max
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
