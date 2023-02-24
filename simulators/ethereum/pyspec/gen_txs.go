// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package main

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
)

var _ = (*transactions)(nil)

// MarshalJSON marshals as JSON.
func (t transactions) MarshalJSON() ([]byte, error) {
	type transactions struct {
		Nonce     *math.HexOrDecimal256 `json:"nonce"`
		To        *common.Address       `json:"to.omitempty"`
		Value     *math.HexOrDecimal256 `json:"value"`
		Data      *hexutil.Bytes        `json:"data"`
		GasLimit  *math.HexOrDecimal64  `json:"gasLimit"`
		GasUsed   *math.HexOrDecimal64  `json:"gasUsed"`
		SecretKey *common.Hash          `json:"secretKey"`
	}
	var enc transactions
	enc.Nonce = t.Nonce
	enc.To = t.To
	enc.Value = t.Value
	enc.Data = t.Data
	enc.GasLimit = t.GasLimit
	enc.GasUsed = t.GasUsed
	enc.SecretKey = t.SecretKey
	return json.Marshal(&enc)
}

// UnmarshalJSON unmarshals from JSON.
func (t *transactions) UnmarshalJSON(input []byte) error {
	type transactions struct {
		Nonce     *math.HexOrDecimal256 `json:"nonce"`
		To        *common.Address       `json:"to.omitempty"`
		Value     *math.HexOrDecimal256 `json:"value"`
		Data      *hexutil.Bytes        `json:"data"`
		GasLimit  *math.HexOrDecimal64  `json:"gasLimit"`
		GasUsed   *math.HexOrDecimal64  `json:"gasUsed"`
		SecretKey *common.Hash          `json:"secretKey"`
	}
	var dec transactions
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Nonce != nil {
		t.Nonce = dec.Nonce
	}
	if dec.To != nil {
		t.To = dec.To
	}
	if dec.Value != nil {
		t.Value = dec.Value
	}
	if dec.Data != nil {
		t.Data = dec.Data
	}
	if dec.GasLimit != nil {
		t.GasLimit = dec.GasLimit
	}
	if dec.GasUsed != nil {
		t.GasUsed = dec.GasUsed
	}
	if dec.SecretKey != nil {
		t.SecretKey = dec.SecretKey
	}
	return nil
}
