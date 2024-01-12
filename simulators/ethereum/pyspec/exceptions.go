package main

import (
	"fmt"
	"regexp"
	"strings"
)

var exceptionClientRegexMap = map[string]map[string]string{
	"TransactionException.INSUFFICIENT_ACCOUNT_FUNDS": {
		"besu":        `transaction invalid transaction up-front cost .* exceeds transaction sender account balance`,
		"ethereumjs":  `sender doesn't have enough funds to send tx`,
		"go-ethereum": `insufficient funds for gas \* price \+ value`,
		"reth":        `Transaction error: LackOfFundForMaxFee`,
	},
	"TransactionException.INSUFFICIENT_MAX_FEE_PER_GAS": {
		"besu":        `transaction invalid gasPrice is less than the current BaseFee`,
		"ethereumjs":  `tx unable to pay base fee`,
		"go-ethereum": `max fee per gas less than block base fee`,
		"reth":        `Transaction error: GasPriceLessThanBasefee`,
	},
	"TransactionException.PRIORITY_GREATER_THAN_MAX_FEE_PER_GAS": {
		"besu":        ``,
		"ethereumjs":  ``,
		"go-ethereum": ``,
		"reth":        ``,
	},
	"TransactionException.INSUFFICIENT_MAX_FEE_PER_BLOB_GAS": {
		"besu":        ``,
		"ethereumjs":  `blob transaction maxFeePerBlobGas \d+ < than block blob gas price \d+`,
		"go-ethereum": `max fee per blob gas less than block blob gas fee`,
		"reth":        `Transaction error: BlobGasPriceGreaterThanMax`,
	},
	"TransactionException.INTRINSIC_GAS_TOO_LOW": {
		"besu":        `transaction invalid intrinsic gas cost \d+ exceeds gas limit \d+`,
		"ethereumjs":  `invalid transactions: errors at tx \d+: gasLimit is too low. given \d+, need at least \d+`,
		"go-ethereum": `intrinsic gas too low: have \d+, want \d+`,
		"reth":        `Transaction error: CallGasCostMoreThanGasLimit`,
	},
	"TransactionException.INITCODE_SIZE_EXCEEDED": {
		"besu":        `transaction invalid Initcode size of \d+ exceeds maximum size of \d+`,
		"ethereumjs":  `Invalid tx at index \d+: Error: the initcode size of this transaction is too large: it is \d+ while the max is \d+`,
		"go-ethereum": `max initcode size exceeded: code size \d+ limit \d+`,
		"reth":        `Transaction error: CreateInitcodeSizeLimit`,
	},
	"TransactionException.TYPE_3_TX_PRE_FORK": {
		"besu":        `transaction invalid Transaction type BLOB is invalid, accepted transaction types are .*`,
		"ethereumjs":  `EIP-4844 not enabled on Common`,
		"go-ethereum": `transaction type not supported`,
		"reth":        `pre-Cancun payload has blob transactions`,
	},
	"TransactionException.TYPE_3_TX_ZERO_BLOBS_PRE_FORK": {
		"besu":        `Failed to decode transactions from block parameter`,
		"ethereumjs":  `EIP-4844 not enabled on Common`,
		"go-ethereum": `invalid number of versionedHashes`,
		"reth":        `pre-Cancun payload has blob transactions`,
	},
	"TransactionException.TYPE_3_TX_INVALID_BLOB_VERSIONED_HASH": {
		"besu":        `Invalid versionedHash`,
		"ethereumjs":  `Invalid tx at index \d+: Error: versioned hash does not start with KZG commitment version`,
		"go-ethereum": `blob \d+ hash version mismatch`,
		"reth":        `Transaction error: BlobVersionNotSupported`,
	},
	"TransactionException.TYPE_3_TX_WITH_FULL_BLOBS": {
		"besu":        `Failed to decode transactions from block parameter`,
		"ethereumjs":  `Invalid tx at index \d+: Error: Invalid EIP-4844 transaction`,
		"go-ethereum": `unexpected blob sidecar in transaction at index \d+`,
		"reth":        `unexpected list`,
	},
	"TransactionException.TYPE_3_TX_BLOB_COUNT_EXCEEDED": {
		"besu":        `Invalid Blob Count: \d+`,
		"ethereumjs":  `Invalid tx at index \d+: Error: tx can contain at most \d+ blobs`,
		"go-ethereum": `blob gas used \d+ exceeds maximum allowance \d+`,
		"reth":        `blob gas used \d+ exceeds maximum allowance \d+`,
	},
	"TransactionException.TYPE_3_TX_CONTRACT_CREATION": {
		"besu":        `transaction invalid transaction blob transactions cannot have a to address`,
		"ethereumjs":  `Invalid tx at index \d+: Error: tx should have a \"to\" field and cannot be used to create contracts`,
		"go-ethereum": `invalid transaction \d+: rlp: input string too short for common.Address, decoding into \(types\.BlobTx\)\.To`,
		"reth":        `Transaction error: BlobCreateTransaction`,
	},
	"TransactionException.TYPE_3_TX_MAX_BLOB_GAS_ALLOWANCE_EXCEEDED": {
		"besu":        `Invalid Blob Count: \d+`,
		"ethereumjs":  `invalid transactions: errors at tx \d+: tx causes total blob gas of \d+ to exceed maximum blob gas per block of \d+`,
		"go-ethereum": `blob gas used \d+ exceeds maximum allowance \d+`,
		"reth":        `blob gas used \d+ exceeds maximum allowance \d+`,
	},
	"TransactionException.TYPE_3_TX_ZERO_BLOBS": {
		"besu":        `Failed to decode transactions from block parameter`,
		"ethereumjs":  `Invalid tx at index \d+: Error: tx should contain at least one blob`,
		"go-ethereum": `blob transaction missing blob hashes`,
		"reth":        `Transaction error: EmptyBlobs`,
	},
	// Block Exceptions
	"BlockException.INCORRECT_BLOCK_FORMAT": {
		"besu":        ``,
		"ethereumjs":  ``,
		"go-ethereum": ``,
		"reth":        ``,
	},
	"BlockException.INCORRECT_BLOB_GAS_USED": {
		"besu":        `Payload BlobGasUsed does not match calculated BlobGasUsed`,
		"ethereumjs":  `invalid transactions: invalid blobGasUsed expected=\d+ actual=\d+`,
		"go-ethereum": `blob gas used mismatch \(header \d+, calculated \d+\)`,
		"reth":        `blob gas used mismatch: got \d+, expected \d+`,
	},
	"BlockException.BLOB_GAS_USED_ABOVE_LIMIT": {
		"besu":        `Payload BlobGasUsed does not match calculated BlobGasUsed`,
		"ethereumjs":  `invalid transactions: invalid blobGasUsed expected=\d+ actual=\d+`,
		"go-ethereum": `blob gas used \d+ exceeds maximum allowance \d+`,
		"reth":        `blob gas used mismatch: got \d+, expected \d+`,
	},
	"BlockException.INCORRECT_EXCESS_BLOB_GAS": {
		"besu":        `Payload excessBlobGas does not match calculated excessBlobGas`,
		"ethereumjs":  `block excessBlobGas mismatch: have \d+, want \d+`,
		"go-ethereum": `invalid excessBlobGas: have \d+, want \d+`,
		"reth":        `invalid excess blob gas: got \d+, expected \d+`,
	},
}

func matchRegex(regex string, actual string) bool {
	return regexp.MustCompile(regex).MatchString(actual)

}
func mapClientType(clientType string) string {
	if strings.HasPrefix(clientType, "besu") {
		return "besu"
	}
	if strings.HasPrefix(clientType, "erigon") {
		return "erigon"
	}
	if strings.HasPrefix(clientType, "ethereumjs") {
		return "ethereumjs"
	}
	if strings.HasPrefix(clientType, "go-ethereum") {
		return "go-ethereum"
	}
	if strings.HasPrefix(clientType, "nethermind") {
		return "nethermind"
	}
	if strings.HasPrefix(clientType, "reth") {
		return "reth"
	}
	return ""
}
func validateException(clientType string, expectedExceptions *string, actualException *string) error {
	if expectedExceptions == nil && actualException == nil {
		return nil
	}
	if expectedExceptions == nil && actualException != nil && *actualException != "" {
		return fmt.Errorf("unexpected exception: %s", *actualException)
	}
	if expectedExceptions != nil && actualException == nil {
		return fmt.Errorf("expected exception: %s", *expectedExceptions)
	}
	// Compare on a per-client basis
	if clientType == "" {
		return fmt.Errorf("unknown client type")
	}
	for _, expectedException := range strings.Split(*expectedExceptions, "|") {
		if clientRegexMap, ok := exceptionClientRegexMap[expectedException]; ok {
			if regex, ok := clientRegexMap[clientType]; ok {
				if regex == "" {
					return fmt.Errorf("client %s does not support exception: %s", clientType, expectedException)
				}
				if matchRegex(regex, *actualException) {
					return nil
				}
			} else {
				return fmt.Errorf("unknown client type: %s", clientType)
			}
		} else {
			return fmt.Errorf("unknown exception: %s", expectedException)
		}
	}
	return fmt.Errorf("unexpected exception: %s, expected: %s", *actualException, *expectedExceptions)
}
