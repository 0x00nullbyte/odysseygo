// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"math"

	"github.com/DioneProtocol/odysseygo/codec"
	"github.com/DioneProtocol/odysseygo/codec/linearcodec"
	"github.com/DioneProtocol/odysseygo/utils/wrappers"
	"github.com/DioneProtocol/odysseygo/vms/omegavm/txs"
)

// Version is the current default codec version
const Version = txs.Version

// GenesisCode allows blocks of larger than usual size to be parsed.
// While this gives flexibility in accommodating large genesis blocks
// it must not be used to parse new, unverified blocks which instead
// must be processed by Codec
var (
	Codec        codec.Manager
	GenesisCodec codec.Manager
)

func init() {
	c := linearcodec.NewDefault()
	Codec = codec.NewDefaultManager()
	gc := linearcodec.NewCustomMaxLength(math.MaxInt32)
	GenesisCodec = codec.NewManager(math.MaxInt32)

	errs := wrappers.Errs{}
	for _, c := range []linearcodec.Codec{c, gc} {
		errs.Add(
			RegisterOdysseyBlockTypes(c),
			txs.RegisterUnsignedTxsTypes(c),
			RegisterBanffBlockTypes(c),
		)
	}
	errs.Add(
		Codec.RegisterCodec(Version, c),
		GenesisCodec.RegisterCodec(Version, gc),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}

// RegisterOdysseyBlockTypes allows registering relevant type of blocks package
// in the right sequence. Following repackaging of omegavm package, a few
// subpackage-level codecs were introduced, each handling serialization of
// specific types.
func RegisterOdysseyBlockTypes(targetCodec codec.Registry) error {
	errs := wrappers.Errs{}
	errs.Add(
		targetCodec.RegisterType(&OdysseyProposalBlock{}),
		targetCodec.RegisterType(&OdysseyAbortBlock{}),
		targetCodec.RegisterType(&OdysseyCommitBlock{}),
		targetCodec.RegisterType(&OdysseyStandardBlock{}),
		targetCodec.RegisterType(&OdysseyAtomicBlock{}),
	)
	return errs.Err
}

func RegisterBanffBlockTypes(targetCodec codec.Registry) error {
	errs := wrappers.Errs{}
	errs.Add(
		targetCodec.RegisterType(&BanffProposalBlock{}),
		targetCodec.RegisterType(&BanffAbortBlock{}),
		targetCodec.RegisterType(&BanffCommitBlock{}),
		targetCodec.RegisterType(&BanffStandardBlock{}),
	)
	return errs.Err
}