package suite_builder

import (
	"strings"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	"github.com/lithammer/dedent"
	mock_builder "github.com/marioevz/mock-builder/mock"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-builder",
	DisplayName: "Deneb Builder",
	Description: `
	Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet and builder API for Cancun+Deneb.
	`,
	Location: "suites/builder",
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-builders-sanity",
				DisplayName: "Deneb Builder Workflow From Capella Transition",
				Description: `
				Test canonical chain includes deneb payloads built by the builder api.
				  
				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1 or 5
					- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
				- Both nodes have the mock-builder configured as builder endpoint from the start
				- Wait for Deneb fork
				- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
				- Start sending blob transactions to the Execution client
				- Wait one more epoch for the chain to progress and include blobs
				- Verify on the beacon chain that:
					- Builder was able to include blocks with blobs in the canonical chain, which implicitly verifies:
					- Consensus client is able to properly format header requests to the builder
					- Consensus client is able to properly format blinded signed requests to the builder
					- No signed block or blob sidecar contained an invalid format or signature
					- For each blob transaction on the execution chain, the blob sidecars are available for the
					beacon block at the same height
					- The beacon block lists the correct commitments for each blob
					- Chain is finalizing
					- No more than two missed slots on the latest epoch
				`,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
				WaitForFinality: true,
			},
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-builders-invalid-payload-attributes-beacon-root",
				DisplayName: "Deneb Builder Builds Block With Invalid Beacon Root, Correct State Root",
				Description: `
				Test canonical chain can still finalize if the builders start
				building payloads with invalid parent beacon block root.

				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1 or 5
				  - Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
				- Both nodes have the mock-builder configured as builder endpoint from the start
				- Total of 128 Validators, 64 for each validating node
				- Wait for Deneb fork
				- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
				- Start sending blob transactions to the Execution client
				- Starting from Deneb, Mock builder starts corrupting the payload attributes' parent beacon block root sent to the execution client to produce the payloads
				- Wait one more epoch for the chain to progress
				- Verify on the beacon chain that:
				  - Blocks with the corrupted beacon root are not included in the canonical chain
				  - Circuit breaker correctly kicks in and disables the builder workflow
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			InvalidatePayloadAttributes: mock_builder.INVALIDATE_ATTR_BEACON_ROOT,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-builders-error-on-deneb-header-request",
				DisplayName: "Deneb Builder Errors Out on Header Requests After Deneb Transition",
				Description: `
				Test canonical chain can still finalize if the builders start
				returning error on header request after deneb transition.

				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1 or 5
				  - Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
				- Both nodes have the mock-builder configured as builder endpoint from the start
				- Total of 128 Validators, 64 for each validating node
				- Wait for Deneb fork
				- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
				- Start sending blob transactions to the Execution client
				- Starting from Deneb, Mock builder starts returning error on the request for block headers
				- Wait one more epoch for the chain to progress
				- Verify on the beacon chain that:
				  - Consensus clients fallback to local block building
				  - No more than two missed slots on the latest epoch
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			ErrorOnHeaderRequest: true,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-builders-error-on-deneb-unblind-payload-request",
				DisplayName: "Deneb Builder Errors Out on Signed Blinded Beacon Block/Blob Sidecars Submission After Deneb Transition",
				Description: `
				Test canonical chain can still finalize if the builders start
				returning error on unblinded payload request after deneb transition.

				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1 or 5
				  - Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
				- Both nodes have the mock-builder configured as builder endpoint from the start
				- Total of 128 Validators, 64 for each validating node
				- Wait for Deneb fork
				- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
				- Start sending blob transactions to the Execution client
				- Starting from Deneb, Mock builder starts returning error on the submission of signed blinded beacon block/blob sidecars
				- Wait one more epoch for the chain to progress
				- Verify on the beacon chain that:
				  - Signed missed slots do not fallback to local block building
				  - Circuit breaker correctly kicks in and disables the builder workflow
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			ErrorOnPayloadReveal: true,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-builders-invalid-payload-version",
				DisplayName: "Deneb Builder Builds Block With Invalid Payload Version",
				Description: `
				Test consensus clients correctly reject a built payload if the
				version is outdated (capella instead of deneb).

				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1 or 5
				  - Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
				- Both nodes have the mock-builder configured as builder endpoint from the start
				- Total of 128 Validators, 64 for each validating node
				- Wait for Deneb fork
				- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
				- Start sending blob transactions to the Execution client
				- Starting from Deneb, Mock builder starts returning the invalid payload version (Capella instead of Deneb)
				- Wait one more epoch for the chain to progress
				- Verify on the beacon chain that:
				  - Blocks with the invalid payload version are not included in the canonical chain
				  - No more than two missed slots on the latest epoch
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			InvalidPayloadVersion: true,
		},
		BuilderTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-builders-invalid-payload-beacon-root",
				DisplayName: "Deneb Builder Builds Block With Invalid Beacon Root, Incorrect State Root",
				Description: `
				Test consensus clients correctly circuit break builder after a
				period of empty blocks due to invalid unblinded blocks.
				The payloads are built using an invalid parent beacon block root, which can only
				be caught after unblinding the entire payload and running it in the
				local execution client, at which point another payload cannot be
				produced locally and results in an empty slot.

				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1 or 5
					- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
				- Both nodes have the mock-builder configured as builder endpoint from the start
				- Total of 128 Validators, 64 for each validating node
				- Wait for Deneb fork
				- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
				- Start sending blob transactions to the Execution client
				- Starting from Deneb, Mock builder starts corrupting the parent beacon block root of the payload received from the execution client
				- Wait one more epoch for the chain to progress
				- Verify on the beacon chain that:
					- Blocks with the corrupted beacon root are not included in the canonical chain
					- Circuit breaker correctly kicks in and disables the builder workflow
				`,
				// All validators can withdraw from the start
				GenesisExecutionWithdrawalCredentialsShares: 1,
				WaitForFinality: true,
			},
			InvalidatePayload: mock_builder.INVALIDATE_PAYLOAD_BEACON_ROOT,
		},
	)

	// Add clients that require finalization to the description of the suite
	sb := strings.Builder{}
	sb.WriteString(dedent.Dedent(testSuite.Description))
	sb.WriteString("\n\n")
	sb.WriteString("### Clients that require finalization to enable builder\n")
	for _, client := range REQUIRES_FINALIZATION_TO_ACTIVATE_BUILDER {
		sb.WriteString("- ")
		sb.WriteString(strings.Title(client))
		sb.WriteString("\n")
	}
	testSuite.Description = sb.String()
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
