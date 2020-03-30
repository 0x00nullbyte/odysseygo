package nftfx

import (
	"errors"

	"github.com/ava-labs/gecko/vms/secp256k1fx"
)

const (
	// MaxPayloadSize is the maximum size that can be placed into a payload
	MaxPayloadSize = 1 << 10
)

var (
	errNilTransferOutput = errors.New("nil transfer output")
	errPayloadTooLarge   = errors.New("payload too large")
)

// TransferOutput ...
type TransferOutput struct {
	GroupID                  uint32 `serialize:"true"`
	Payload                  []byte `serialize:"true"`
	secp256k1fx.OutputOwners `serialize:"true"`
}

// Verify ...
func (out *TransferOutput) Verify() error {
	switch {
	case out == nil:
		return errNilTransferOutput
	case len(out.Payload) > MaxPayloadSize:
		return errPayloadTooLarge
	default:
		return out.OutputOwners.Verify()
	}
}
