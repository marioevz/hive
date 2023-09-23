package helper

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	exec_client "github.com/marioevz/eth-clients/clients/execution"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
)

type TransactionSpammer struct {
	*hivesim.T
	ExecutionClients         []*exec_client.ExecutionClient
	Accounts                 []*globals.TestAccount
	Recipient                *common.Address
	TransactionType          helper.TestTransactionType
	TransactionsPerIteration int
	SecondsBetweenIterations int
}

func (t *TransactionSpammer) Run(ctx context.Context) error {
	// Send some transactions constantly in the bg
	nonceMap := make(map[common.Address]uint64)
	secondsBetweenIterations := time.Duration(t.SecondsBetweenIterations)
	txCreator := helper.BaseTransactionCreator{
		Recipient: t.Recipient,
		GasLimit:  500000,
		Amount:    common.Big1,
		TxType:    t.TransactionType,
	}
	txsSent := 0
	iteration := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * secondsBetweenIterations):
			currentClient := t.ExecutionClients[iteration%len(t.ExecutionClients)]
			h, err := currentClient.HeaderByNumber(ctx, nil)
			if err != nil {
				panic(err)
			}
			for i := 0; i < t.TransactionsPerIteration; i++ {
				sender := t.Accounts[txsSent%len(t.Accounts)]
				nonce := nonceMap[sender.GetAddress()]
				tx, err := txCreator.MakeTransaction(sender, nonce, h.Time)
				if err != nil {
					panic(err)
				}
				if err := currentClient.SendTransaction(
					ctx,
					tx,
				); err != nil {
					t.Logf("INFO: Error sending tx: %v, sender: %s, nonce=%d", err, sender.GetAddress().String(), nonce)
				}
				nonceMap[sender.GetAddress()] = nonce + 1
				txsSent += 1
			}
			iteration += 1
		}
	}
}

func WithdrawalsContainValidator(
	ws []*types.Withdrawal,
	vId beacon.ValidatorIndex,
) bool {
	for _, w := range ws {
		if w.Validator == uint64(vId) {
			return true
		}
	}
	return false
}

type BeaconBlockState struct {
	*beacon_client.VersionedBeaconStateResponse
	*beacon_client.VersionedSignedBeaconBlock
}

type BeaconCache map[tree.Root]BeaconBlockState

// Clear the cache for when there was a known/expected re-org to query everything again
func (c BeaconCache) Clear() error {
	roots := make([]tree.Root, len(c))
	i := 0
	for s := range c {
		roots[i] = s
		i++
	}
	for _, s := range roots {
		delete(c, s)
	}
	return nil
}

func (c BeaconCache) GetBlockStateByRoot(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	blockroot tree.Root,
) (BeaconBlockState, error) {
	if s, ok := c[blockroot]; ok {
		return s, nil
	}
	b, err := bc.BlockV2(ctx, eth2api.BlockIdRoot(blockroot))
	if err != nil {
		return BeaconBlockState{}, err
	}
	s, err := bc.BeaconStateV2(ctx, eth2api.StateIdSlot(b.Slot()))
	if err != nil {
		return BeaconBlockState{}, err
	}
	blockStateRoot := b.StateRoot()
	stateRoot := s.Root()
	if !bytes.Equal(blockStateRoot[:], stateRoot[:]) {
		return BeaconBlockState{}, fmt.Errorf(
			"state root missmatch while fetching state",
		)
	}
	both := BeaconBlockState{
		VersionedBeaconStateResponse: s,
		VersionedSignedBeaconBlock:   b,
	}
	c[blockroot] = both
	return both, nil
}

func (c BeaconCache) GetBlockStateBySlotFromHeadRoot(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	headblockroot tree.Root,
	slot beacon.Slot,
) (*BeaconBlockState, error) {
	current, err := c.GetBlockStateByRoot(ctx, bc, headblockroot)
	if err != nil {
		return nil, err
	}
	if current.Slot() < slot {
		return nil, fmt.Errorf("requested for slot above head")
	}
	for {
		if current.Slot() == slot {
			return &current, nil
		}
		if current.Slot() < slot || current.Slot() == 0 {
			// Skipped slot probably, not a fatal error
			return nil, nil
		}
		current, err = c.GetBlockStateByRoot(ctx, bc, current.ParentRoot())
		if err != nil {
			return nil, err
		}
	}
}

// Helper struct to keep track of current status of a validator withdrawal state
type Validator struct {
	Index                    beacon.ValidatorIndex
	PubKey                   *beacon.BLSPubkey
	WithdrawAddress          *common.Address
	Exited                   bool
	ExitCondition            string
	ExactWithdrawableBalance *big.Int
	Keys                     *cl.ValidatorDetails
	Verified                 bool
	InitialBalance           beacon.Gwei
	Spec                     beacon.Spec
	BlockStateCache          BeaconCache
}

// Signs the BLS-to-execution-change for the given address
func (v *Validator) SignBLSToExecutionChange(
	executionAddress common.Address,
	blsToExecutionChangeDomain beacon.BLSDomain,
) (*beacon.SignedBLSToExecutionChange, error) {
	if v.Keys == nil {
		return nil, fmt.Errorf("no key to sign")
	}
	if v.WithdrawAddress != nil {
		return nil, fmt.Errorf("execution address already set")
	}
	kdPubKey := beacon.BLSPubkey{}
	copy(kdPubKey[:], v.Keys.WithdrawalPubkey[:])
	eth1Address := beacon.Eth1Address{}
	copy(eth1Address[:], executionAddress[:])
	blsToExecChange := beacon.BLSToExecutionChange{
		ValidatorIndex:     v.Index,
		FromBLSPubKey:      kdPubKey,
		ToExecutionAddress: eth1Address,
	}
	sigRoot := beacon.ComputeSigningRoot(
		blsToExecChange.HashTreeRoot(tree.GetHashFn()),
		blsToExecutionChangeDomain,
	)
	sk := new(blsu.SecretKey)
	sk.Deserialize(&v.Keys.WithdrawalSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	return &beacon.SignedBLSToExecutionChange{
		BLSToExecutionChange: blsToExecChange,
		Signature:            beacon.BLSSignature(signature),
	}, nil
}

// Sign and send the BLS-to-execution-change.
// Also internally update the withdraw address.
func (v *Validator) SignSendBLSToExecutionChange(
	ctx context.Context,
	bc *beacon_client.BeaconClient,
	executionAddress common.Address,
	blsToExecutionChangeDomain beacon.BLSDomain,
) error {
	signedBLS, err := v.SignBLSToExecutionChange(executionAddress, blsToExecutionChangeDomain)
	if err != nil {
		return err
	}
	if err := bc.SubmitPoolBLSToExecutionChange(ctx, beacon.SignedBLSToExecutionChanges{
		*signedBLS,
	}); err != nil {
		return err
	}

	v.WithdrawAddress = &executionAddress
	return nil
}

type Validators []*Validator

func (vs Validators) GetValidatorByIndex(i beacon.ValidatorIndex) *Validator {
	for _, v := range vs {
		if v.Index == i {
			return v
		}
	}
	return nil
}

func (vs Validators) NonWithdrawable() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.WithdrawAddress == nil {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) Withdrawable() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.WithdrawAddress != nil {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) FullyWithdrawable() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.WithdrawAddress != nil && v.Exited {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) Exited() Validators {
	ret := make(Validators, 0)
	for _, v := range vs {
		v := v
		if v.Exited {
			ret = append(ret, v)
		}
	}
	return ret
}

func (vs Validators) Chunks(totalShares int) []Validators {
	ret := make([]Validators, totalShares)
	countPerChunk := len(vs) / totalShares
	for i := range ret {
		ret[i] = vs[i*countPerChunk : (i*countPerChunk)+countPerChunk]
	}
	return ret
}

func ValidatorFromBeaconValidator(
	spec beacon.Spec,
	index beacon.ValidatorIndex,
	source beacon.Validator,
	balance beacon.Gwei,
	keys *cl.ValidatorDetails,
	beaconCache BeaconCache,
) (*Validator, error) {
	// Assume genesis state
	currentEpoch := beacon.Epoch(0)

	v := new(Validator)

	v.Spec = spec
	v.Index = index
	v.Keys = keys
	v.BlockStateCache = beaconCache

	pk, err := source.Pubkey()
	if err != nil {
		return nil, err
	}
	v.PubKey = &pk

	wc, err := source.WithdrawalCredentials()
	if err != nil {
		return nil, err
	}
	if wc[0] == beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX {
		withdrawAddress := common.Address{}
		copy(withdrawAddress[:], wc[12:])
		v.WithdrawAddress = &withdrawAddress
	}

	exitEpoch, err := source.ExitEpoch()
	if err != nil {
		return nil, err
	}

	slashed, err := source.Slashed()
	if err != nil {
		return nil, err
	}

	// Assuming this is the genesis beacon state
	if exitEpoch <= currentEpoch || slashed {
		v.Exited = true
		if slashed {
			v.ExitCondition = "Slashed"
		} else {
			v.ExitCondition = "Voluntary Exited"
		}
		v.ExactWithdrawableBalance = big.NewInt(int64(balance))
		v.ExactWithdrawableBalance.Mul(
			v.ExactWithdrawableBalance,
			big.NewInt(1e9),
		)
	}
	v.InitialBalance = balance
	return v, nil
}

func ValidatorFromBeaconState(
	spec beacon.Spec,
	state beacon.BeaconState,
	index beacon.ValidatorIndex,
	keys *cl.ValidatorDetails,
	beaconCache BeaconCache,
) (*Validator, error) {
	stateVals, err := state.Validators()
	if err != nil {
		return nil, err
	}
	balances, err := state.Balances()
	if err != nil {
		return nil, err
	}
	beaconVal, err := stateVals.Validator(index)
	if err != nil {
		return nil, err
	}
	balance, err := balances.GetBalance(index)
	if err != nil {
		return nil, err
	}
	return ValidatorFromBeaconValidator(
		spec,
		index,
		beaconVal,
		balance,
		keys,
		beaconCache,
	)
}

func ValidatorsFromBeaconState(
	state beacon.BeaconState,
	spec beacon.Spec,
	keys []*cl.ValidatorDetails,
) (Validators, error) {
	stateVals, err := state.Validators()
	if err != nil {
		return nil, err
	}
	balances, err := state.Balances()
	if err != nil {
		return nil, err
	}
	validatorCount, err := stateVals.ValidatorCount()
	if err != nil {
		return nil, err
	} else if validatorCount == 0 {
		return nil, fmt.Errorf("got zero validators")
	} else if validatorCount != uint64(len(keys)) {
		return nil, fmt.Errorf("incorrect amount of keys: want=%d, got=%d", validatorCount, len(keys))
	}
	beaconCache := make(BeaconCache)
	validators := make(Validators, 0)
	for i := beacon.ValidatorIndex(0); i < beacon.ValidatorIndex(validatorCount); i++ {
		beaconVal, err := stateVals.Validator(beacon.ValidatorIndex(i))
		if err != nil {
			return nil, err
		}
		balance, err := balances.GetBalance(i)
		if err != nil {
			return nil, err
		}
		validator, err := ValidatorFromBeaconValidator(
			spec,
			i,
			beaconVal,
			balance,
			keys[i],
			beaconCache,
		)
		if err != nil {
			return nil, err
		}
		validators = append(validators, validator)

	}
	return validators, nil
}

func ComputeBLSToExecutionDomain(
	t *testnet.Testnet,
) beacon.BLSDomain {
	return beacon.ComputeDomain(
		beacon.DOMAIN_BLS_TO_EXECUTION_CHANGE,
		t.Spec().GENESIS_FORK_VERSION,
		t.GenesisValidatorsRoot(),
	)
}
