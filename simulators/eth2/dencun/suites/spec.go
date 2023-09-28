package suites

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	consensus_config "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var Deneb string = "deneb"

type TestSpec interface {
	GetName() string
	GetTestnetConfig(clients.NodeDefinitions) *testnet.Config
	GetDescription() string
	ExecutePreFork(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	ExecutePostFork(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	ExecutePostForkWait(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	Verify(*hivesim.T, context.Context, *testnet.Testnet, *testnet.Environment, *testnet.Config)
	GetValidatorKeys(string) []*consensus_config.ValidatorDetails
}

// Add all tests to the suite
func SuiteHydrate(
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
				testnet := testnet.StartTestnet(ctx, t, env, config)
				if testnet == nil {
					t.Fatalf("failed to start testnet")
				}
				defer testnet.Stop()

				// Execute pre-fork
				test.ExecutePreFork(t, ctx, testnet, env, config)

				// Wait for the fork
				slotsUntilFork := beacon.Slot(
					config.DenebForkEpoch.Uint64(),
				)*testnet.Spec().SLOTS_PER_EPOCH + 4
				timeoutCtx, cancel := testnet.Spec().SlotTimeoutContext(ctx, slotsUntilFork)
				defer cancel()
				if err := testnet.WaitForFork(timeoutCtx, Deneb); err != nil {
					t.Fatalf("FAIL: error waiting for deneb: %v", err)
				}

				// Execute post-fork
				test.ExecutePostFork(t, ctx, testnet, env, config)

				// Execute post-fork wait
				test.ExecutePostForkWait(t, ctx, testnet, env, config)

				// Verify
				test.Verify(t, ctx, testnet, env, config)
			},
		},
		)
	}
}
