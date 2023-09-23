package suite_builder

import (
	"math/big"

	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	mock_builder "github.com/marioevz/mock-builder/mock"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
)

var REQUIRES_FINALIZATION_TO_ACTIVATE_BUILDER = []string{
	"lighthouse",
	"teku",
}

type BuilderTestSpec struct {
	suite_base.BaseTestSpec
	ErrorOnHeaderRequest        bool
	ErrorOnPayloadReveal        bool
	InvalidPayloadVersion       bool
	InvalidatePayload           mock_builder.PayloadInvalidation
	InvalidatePayloadAttributes mock_builder.PayloadAttributesInvalidation
}

func (ts BuilderTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	tc := ts.BaseTestSpec.GetTestnetConfig(allNodeDefinitions)

	tc.DenebForkEpoch = big.NewInt(1)

	if len(
		allNodeDefinitions.FilterByCL(
			REQUIRES_FINALIZATION_TO_ACTIVATE_BUILDER,
		),
	) > 0 {
		// At least one of the CLs require finalization to start requesting
		// headers from the builder
		tc.DenebForkEpoch = big.NewInt(5)
	}

	// Builders are always enabled for these tests
	tc.EnableBuilders = true

	// Builder config
	// Configure the builder according to the error
	tc.BuilderOptions = make([]mock_builder.Option, 0)

	// Bump the built payloads value
	tc.BuilderOptions = append(
		tc.BuilderOptions,
		mock_builder.WithPayloadWeiValueMultiplier(big.NewInt(10)),
		mock_builder.WithExtraDataWatermark("builder payload tst"),
	)

	// Inject test error
	denebEpoch := beacon.Epoch(tc.DenebForkEpoch.Uint64())
	if ts.ErrorOnHeaderRequest {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithErrorOnHeaderRequestAtEpoch(denebEpoch),
		)
	}
	if ts.ErrorOnPayloadReveal {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithErrorOnPayloadRevealAtEpoch(denebEpoch),
		)
	}
	if ts.InvalidatePayload != "" {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithPayloadInvalidatorAtEpoch(
				denebEpoch,
				ts.InvalidatePayload,
			),
		)
	}
	if ts.InvalidatePayloadAttributes != "" {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithPayloadAttributesInvalidatorAtEpoch(
				denebEpoch,
				ts.InvalidatePayloadAttributes,
			),
		)
	}
	if ts.InvalidPayloadVersion {
		tc.BuilderOptions = append(
			tc.BuilderOptions,
			mock_builder.WithInvalidBuilderBidVersionAtEpoch(denebEpoch),
		)
	}

	return tc
}
