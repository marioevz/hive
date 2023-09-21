package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	mock_builder "github.com/marioevz/mock-builder/mock"
)

var CHAIN_ID = big.NewInt(1)

type TestSpec interface {
	GetName() string
	GetTestnetConfig(clients.NodeDefinitions) *testnet.Config
	GetDescription() string
	Execute(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config, []clients.NodeDefinition)
	GetValidatorKeys(string) []*consensus_config.ValidatorDetails
}

var tests = []TestSpec{
	BaseDencunTestSpec{
		Name: "test-deneb-fork",
		Description: `
		Sanity test to check the fork transition to deneb.
		`,
		DenebGenesis: false,
		GenesisExecutionWithdrawalCredentialsShares: 1,
	},

	BaseDencunTestSpec{
		Name: "test-deneb-genesis",
		Description: `
		Sanity test to check the beacon clients can start with deneb genesis.
		`,
		DenebGenesis: true,
		GenesisExecutionWithdrawalCredentialsShares: 1,
	},
}

var builderTests = []TestSpec{
	BuilderDenebTestSpec{
		BaseDencunTestSpec: BaseDencunTestSpec{
			Name: "test-builders-sanity",
			Description: `
			Test canonical chain includes deneb payloads built by the builder api.
			`,
			// All validators start with BLS withdrawal credentials
			GenesisExecutionWithdrawalCredentialsShares: 0,
			WaitForFinality: true,
		},
	},
	BuilderDenebTestSpec{
		BaseDencunTestSpec: BaseDencunTestSpec{
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
	BuilderDenebTestSpec{
		BaseDencunTestSpec: BaseDencunTestSpec{
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
	BuilderDenebTestSpec{
		BaseDencunTestSpec: BaseDencunTestSpec{
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
	BuilderDenebTestSpec{
		BaseDencunTestSpec: BaseDencunTestSpec{
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
	BuilderDenebTestSpec{
		BaseDencunTestSpec: BaseDencunTestSpec{
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
}

func main() {
	// Create simulator that runs all tests
	sim := hivesim.New()
	// From the simulator we can get all client types provided
	clientTypes, err := sim.ClientTypes()
	if err != nil {
		panic(err)
	}
	c := clients.ClientsByRole(clientTypes)

	// Create the test suites
	denebSuite := hivesim.Suite{
		Name:        "eth2-deneb",
		Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb.`,
	}
	builderSuite := hivesim.Suite{
		Name:        "eth2-deneb-builder",
		Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet and builder API for Cancun+Deneb.`,
	}

	// Add all tests to the suites
	addAllTests(&denebSuite, c, tests)
	addAllTests(&builderSuite, c, builderTests)

	// Mark suites for execution
	hivesim.MustRunSuite(sim, denebSuite)
	hivesim.MustRunSuite(sim, builderSuite)
}

func addAllTests(
	suite *hivesim.Suite,
	c *clients.ClientDefinitionsByRole,
	tests []TestSpec,
) {
	mnemonic := "couple kiwi radio river setup fortune hunt grief buddy forward perfect empty slim wear bounce drift execute nation tobacco dutch chapter festival ice fog"

	clientCombinations := c.Combinations()
	for _, test := range tests {
		test := test
		suite.Add(hivesim.TestSpec{
			Name: fmt.Sprintf(
				"%s-%s",
				test.GetName(),
				strings.Join(clientCombinations.ClientTypes(), "-"),
			),
			Description: test.GetDescription(),
			Run: func(t *hivesim.T) {
				keys := test.GetValidatorKeys(mnemonic)
				secrets, err := consensus_config.SecretKeys(keys)
				if err != nil {
					panic(err)
				}
				env := &testnet.Environment{
					Clients: c,
					Keys:    keys,
					Secrets: secrets,
				}
				config := test.GetTestnetConfig(clientCombinations)
				// Create the testnet
				ctx := context.Background()
				tn := testnet.StartTestnet(ctx, t, env, config)
				defer tn.Stop()
				test.Execute(t, ctx, tn, env, config, clientCombinations)
			},
		},
		)
	}
}
