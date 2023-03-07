package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	cl "github.com/ethereum/hive/simulators/eth2/common/config/consensus"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	blsu "github.com/protolambda/bls12-381-util"
	"github.com/protolambda/eth2api"
	beacon "github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/ztyp/tree"
)

// API call names
const (
	EngineForkchoiceUpdatedV1 = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1        = "engine_getPayloadV1"
	EngineNewPayloadV1        = "engine_newPayloadV1"
	EthGetBlockByHash         = "eth_getBlockByHash"
	EthGetBlockByNumber       = "eth_getBlockByNumber"
)

// Engine API Types

type PayloadStatus string

const (
	Unknown          = ""
	Valid            = "VALID"
	Invalid          = "INVALID"
	Accepted         = "ACCEPTED"
	Syncing          = "SYNCING"
	InvalidBlockHash = "INVALID_BLOCK_HASH"
)

// Signer for all txs
type Signer struct {
	ChainID    *big.Int
	PrivateKey *ecdsa.PrivateKey
}

func (vs Signer) SignTx(
	baseTx *types.Transaction,
) (*types.Transaction, error) {
	signer := types.NewEIP155Signer(vs.ChainID)
	return types.SignTx(baseTx, signer, vs.PrivateKey)
}

var VaultSigner = Signer{
	ChainID:    CHAIN_ID,
	PrivateKey: VAULT_KEY,
}

func WithdrawalsContainValidator(
	ws beacon.Withdrawals,
	vId beacon.ValidatorIndex,
) bool {
	for _, w := range ws {
		if w.ValidatorIndex == vId {
			return true
		}
	}
	return false
}

func PrintWithdrawalHistory(
	ctx context.Context,
	bn *clients.BeaconClient,
	headId eth2api.BlockId,
) error {
	// Get the state history of the given block
	stateHistory, err := bn.BeaconStateV2HistoryFromHead(ctx, headId)
	if err != nil || len(stateHistory) == 0 {
		return err
	}

	var prevState *clients.VersionedBeaconStateResponse
	for _, s := range stateHistory {
		nextWithdrawalIndex, _ := s.NextWithdrawalIndex()
		nextWithdrawalValidatorIndex, _ := s.NextWithdrawalValidatorIndex()
		fmt.Printf(
			"Slot=%d, NextWithdrawalIndex=%d, NextWithdrawalValidatorIndex=%d\n",
			s.StateSlot(),
			nextWithdrawalIndex,
			nextWithdrawalValidatorIndex,
		)
		if prevState != nil {
			fmt.Printf("Withdrawals:\n")
			ws, _ := s.Withdrawals(prevState)
			for i, w := range ws {
				fmt.Printf(
					"%d: Validator Index: %s, Amount: %d\n",
					i,
					w.ValidatorIndex,
					w.Amount,
				)
			}
		}
		prevState = s
	}
	return nil
}

// Helper struct to keep track of current status of a validator withdrawal state
type Validator struct {
	Index                      beacon.ValidatorIndex
	WithdrawAddress            *common.Address
	Exited                     bool
	ExitCondition              string
	ExactWithdrawableBalance   *big.Int
	Keys                       *cl.KeyDetails
	BLSToExecutionChangeDomain *beacon.BLSDomain
	SignedBLSToExecutionChange *beacon.SignedBLSToExecutionChange
	Verified                   bool
	InitialBalance             beacon.Gwei
	Spec                       *beacon.Spec
}

func (v *Validator) VerifyWithdrawnBalance(
	ctx context.Context,
	bn *clients.BeaconClient,
	ec *clients.ExecutionClient,
	headId eth2api.BlockId,
) (bool, error) {
	// Check the withdrawal address, if unset this is an error
	if v.WithdrawAddress == nil {
		return false, fmt.Errorf(
			"checked balance for validator without a withdrawal address",
		)
	}
	execAddress := *v.WithdrawAddress

	fmt.Printf(
		"INFO: Verifying withdrawal for validator %d\n",
		v.Index,
	)

	// Get the state history of the given block
	stateHistory, err := bn.BeaconStateV2HistoryFromHead(ctx, headId)
	if err != nil {
		return false, err
	} else if len(stateHistory) == 0 {
		return false, fmt.Errorf("state history is empty")
	}

	var (
		previousState       *clients.VersionedBeaconStateResponse
		currentWithdrawals  beacon.Withdrawals
		expectedGweiBalance beacon.Gwei
	)

	for _, state := range stateHistory {
		if previousState != nil {
			currentWithdrawals, err = state.Withdrawals(previousState)
			if err != nil {
				return false, err
			}
			previousBalance := previousState.Balance(v.Index)
			currentBalance := state.Balance(v.Index)
			if WithdrawalsContainValidator(currentWithdrawals, v.Index) {
				if currentBalance == 0 {
					// Full withdrawal
					expectedGweiBalance += previousBalance
				} else {
					expectedGweiBalance += previousBalance - (v.Spec.MAX_EFFECTIVE_BALANCE)
				}
			}

		}
		previousState = state
	}

	// If balance is zero, there have not been any withdrawals yet,
	// but this is not an error
	if expectedGweiBalance == 0 {
		fmt.Printf(
			"INFO: Expected balance of validator %d is zero\n",
			v.Index,
		)
		return false, nil
	}

	// Then get the balance
	headPayloadNumber := stateHistory[len(stateHistory)-1].LatestExecutionPayloadHeaderNumber()
	balance, err := ec.BalanceAt(
		ctx,
		execAddress,
		big.NewInt(
			int64(
				headPayloadNumber,
			),
		),
	)
	if err != nil {
		return false, err
	}

	// Check the balance
	expectedBalance := big.NewInt(int64(expectedGweiBalance))
	expectedBalance.Mul(expectedBalance, big.NewInt(1e9))
	if balance.Cmp(expectedBalance) != 0 {
		return true, fmt.Errorf(
			"unexepected balance: want=%d, got=%d",
			expectedBalance,
			balance,
		)
	}

	fmt.Printf(
		"INFO: Validator %d correctly withdrew: block=%d, balance=%d, account=%s\n",
		v.Index,
		headPayloadNumber,
		balance,
		execAddress,
	)
	v.Verified = true
	return true, nil
}

// Signs the BLS-to-execution-change for the given address
func (v *Validator) SignBLSToExecutionChange(
	executionAddress common.Address,
) (*beacon.SignedBLSToExecutionChange, error) {
	if v.Keys == nil {
		return nil, fmt.Errorf("no key to sign")
	}
	if v.BLSToExecutionChangeDomain == nil {
		return nil, fmt.Errorf("no domain to sign")
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
		*v.BLSToExecutionChangeDomain,
	)
	sk := new(blsu.SecretKey)
	sk.Deserialize(&v.Keys.WithdrawalSecretKey)
	signature := blsu.Sign(sk, sigRoot[:]).Serialize()
	v.SignedBLSToExecutionChange = &beacon.SignedBLSToExecutionChange{
		BLSToExecutionChange: blsToExecChange,
		Signature:            beacon.BLSSignature(signature),
	}
	return v.SignedBLSToExecutionChange, nil
}

// Sign and send the BLS-to-execution-change.
// Also internally update the withdraw address.
func (v *Validator) SignSendBLSToExecutionChange(
	ctx context.Context,
	bc *clients.BeaconClient,
	executionAddress common.Address,
) error {
	signedBLS, err := v.SignBLSToExecutionChange(executionAddress)
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

// Sign and send the BLS-to-execution-change.
// Also internally update the withdraw address.
var UnexpectedExecutionWithdrawalAddress = errors.New(
	"validator updated to incorrect address",
)

var NilExpectedAddress = errors.New(
	"validator has nil expected exec address",
)

func (v *Validator) CheckExecutionAddressApplied(
	ctx context.Context,
	bn *clients.BeaconClient,
	stateID eth2api.StateId,
) (bool, error) {
	var expectedAddress common.Address
	if v.WithdrawAddress == nil {
		return false, NilExpectedAddress
	} else {
		expectedAddress = *v.WithdrawAddress
	}
	if versionedBeaconState, err := bn.BeaconStateV2(
		ctx,
		eth2api.StateHead,
	); err != nil {
		return false, err
	} else if versionedBeaconState == nil {
		return false, fmt.Errorf("beacon state does not exist")
	} else {
		validators := versionedBeaconState.Validators()
		validator := validators[v.Index]
		credentials := validator.WithdrawalCredentials
		if !bytes.Equal(
			credentials[:1],
			[]byte{beacon.ETH1_ADDRESS_WITHDRAWAL_PREFIX},
		) {
			return false, nil
		}
		if bytes.Equal(expectedAddress[:], credentials[12:]) {
			return true, nil
		} else {
			return false, UnexpectedExecutionWithdrawalAddress
		}
	}
}

type Validators []*Validator

func (vs Validators) Copy() Validators {
	cpy := make(Validators, len(vs))
	for i, v := range vs {
		if v != nil {
			vCpy := *v
			cpy[i] = &vCpy
		}
	}
	return cpy
}

// Verify all validators have withdrawn
func (vs Validators) VerifyWithdrawnBalance(
	ctx context.Context,
	bc *clients.BeaconClient,
	ec *clients.ExecutionClient,
	headId eth2api.BlockId,
) (bool, error) {
	for i, v := range vs {
		if withdrawn, err := v.VerifyWithdrawnBalance(ctx, bc, ec, headId); err != nil {
			return withdrawn, fmt.Errorf(
				"error verifying validator %d balance: %v",
				i,
				err,
			)
		} else if !withdrawn {
			return false, nil
		}
	}
	return true, nil
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

func (vs Validators) SendSignedBLSToExecutionChanges(
	ctx context.Context,
	bc *clients.BeaconClient,
) error {
	signedChanges := make(beacon.SignedBLSToExecutionChanges, 0)
	for _, v := range vs {
		if v.SignedBLSToExecutionChange != nil {
			signedChanges = append(
				signedChanges,
				*v.SignedBLSToExecutionChange,
			)
		}
	}
	return bc.SubmitPoolBLSToExecutionChange(ctx, signedChanges)
}

func ValidatorFromBeaconValidator(
	spec *beacon.Spec,
	index beacon.ValidatorIndex,
	source beacon.Validator,
	balance beacon.Gwei,
	keys *cl.KeyDetails,
	domain *beacon.BLSDomain,
) (*Validator, error) {
	// Assume genesis state
	currentEpoch := beacon.Epoch(0)

	v := new(Validator)

	v.Spec = spec
	v.Index = index
	v.Keys = keys
	v.BLSToExecutionChangeDomain = domain

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
	spec *beacon.Spec,
	state beacon.BeaconState,
	index beacon.ValidatorIndex,
	keys *cl.KeyDetails,
	domain *beacon.BLSDomain,
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
		domain,
	)
}

func ValidatorsFromBeaconState(
	state beacon.BeaconState,
	spec *beacon.Spec,
	keys []*cl.KeyDetails,
	domain *beacon.BLSDomain,
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
			domain,
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

type BaseTransactionCreator struct {
	Recipient  *common.Address
	GasLimit   uint64
	Amount     *big.Int
	Payload    []byte
	PrivateKey *ecdsa.PrivateKey
}

func (tc *BaseTransactionCreator) MakeTransaction(
	nonce uint64,
) (*types.Transaction, error) {
	var newTxData types.TxData

	gasFeeCap := new(big.Int).Set(GasPrice)
	gasTipCap := new(big.Int).Set(GasTipPrice)
	newTxData = &types.DynamicFeeTx{
		Nonce:     nonce,
		Gas:       tc.GasLimit,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		To:        tc.Recipient,
		Value:     tc.Amount,
		Data:      tc.Payload,
	}

	tx := types.NewTx(newTxData)
	key := tc.PrivateKey
	if key == nil {
		key = VaultKey
	}
	signedTx, err := types.SignTx(
		tx,
		types.NewLondonSigner(ChainID),
		key,
	)
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}
