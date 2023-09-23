package suite_builder

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	mock_builder "github.com/marioevz/mock-builder/mock"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-builder",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet and builder API for Cancun+Deneb.`,
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-builders-sanity",
				Description: `
				Test canonical chain includes deneb payloads built by the builder api.
				`,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
				WaitForFinality: true,
			},
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-builders-invalid-payload-attributes-beacon-root",
				Description: `
				Test canonical chain can still finalize if the builders start
				building payloads with invalid withdrawals list.
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			InvalidatePayloadAttributes: mock_builder.INVALIDATE_ATTR_BEACON_ROOT,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-builders-error-on-deneb-header-request",
				Description: `
				Test canonical chain can still finalize if the builders start
				returning error on header request after deneb transition.
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			ErrorOnHeaderRequest: true,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-builders-error-on-deneb-unblind-payload-requestr",
				Description: `
				Test canonical chain can still finalize if the builders start
				returning error on unblinded payload request after deneb transition.
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			ErrorOnPayloadReveal: true,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-builders-invalid-payload-version",
				Description: `
				Test consensus clients correctly reject a built payload if the
				version is outdated (bellatrix instead of deneb).
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			InvalidPayloadVersion: true,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-builders-invalid-payload-beacon-root",
				Description: `
				Test consensus clients correctly circuit break builder after a
				period of empty blocks due to invalid unblinded blocks.
				The payloads are built using an invalid parent beacon block root, which can only
				be caught after unblinding the entire payload and running it in the
				local execution client, at which point another payload cannot be
				produced locally and results in an empty slot.
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			InvalidatePayload: mock_builder.INVALIDATE_PAYLOAD_BEACON_ROOT,
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
