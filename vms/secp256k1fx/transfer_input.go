// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package secp256k1fx

import (
	"errors"

	"github.com/dioneprotocol/dionego/snow"
)

var errNoValueInput = errors.New("input has no value")

type TransferInput struct {
	Amt   uint64 `serialize:"true" json:"amount"`
	Input `serialize:"true"`
}

func (*TransferInput) InitCtx(*snow.Context) {}

// Amount returns the quantity of the asset this input produces
func (in *TransferInput) Amount() uint64 {
	return in.Amt
}

// Verify this input is syntactically valid
func (in *TransferInput) Verify() error {
	switch {
	case in == nil:
		return errNilInput
	case in.Amt == 0:
		return errNoValueInput
	default:
		return in.Input.Verify()
	}
}
