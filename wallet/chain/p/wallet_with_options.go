// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p

import (
	"time"

	"github.com/DioneProtocol/odysseygo/ids"
	"github.com/DioneProtocol/odysseygo/vms/components/dione"
	"github.com/DioneProtocol/odysseygo/vms/platformvm/signer"
	"github.com/DioneProtocol/odysseygo/vms/platformvm/txs"
	"github.com/DioneProtocol/odysseygo/vms/secp256k1fx"
	"github.com/DioneProtocol/odysseygo/wallet/subnet/primary/common"
)

var _ Wallet = (*walletWithOptions)(nil)

func NewWalletWithOptions(
	wallet Wallet,
	options ...common.Option,
) Wallet {
	return &walletWithOptions{
		Wallet:  wallet,
		options: options,
	}
}

type walletWithOptions struct {
	Wallet
	options []common.Option
}

func (w *walletWithOptions) Builder() Builder {
	return NewBuilderWithOptions(
		w.Wallet.Builder(),
		w.options...,
	)
}

func (w *walletWithOptions) IssueBaseTx(
	outputs []*dione.TransferableOutput,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueBaseTx(
		outputs,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddValidatorTx(
	vdr *txs.Validator,
	rewardsOwner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueAddValidatorTx(
		vdr,
		rewardsOwner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddSubnetValidatorTx(
	vdr *txs.SubnetValidator,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueAddSubnetValidatorTx(
		vdr,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueRemoveSubnetValidatorTx(
	nodeID ids.NodeID,
	subnetID ids.ID,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueRemoveSubnetValidatorTx(
		nodeID,
		subnetID,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueCreateChainTx(
	subnetID ids.ID,
	genesis []byte,
	vmID ids.ID,
	fxIDs []ids.ID,
	chainName string,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueCreateChainTx(
		subnetID,
		genesis,
		vmID,
		fxIDs,
		chainName,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueCreateSubnetTx(
	owner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueCreateSubnetTx(
		owner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueImportTx(
	sourceChainID ids.ID,
	to *secp256k1fx.OutputOwners,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueImportTx(
		sourceChainID,
		to,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueExportTx(
	chainID ids.ID,
	outputs []*dione.TransferableOutput,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueExportTx(
		chainID,
		outputs,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueTransformSubnetTx(
	subnetID ids.ID,
	assetID ids.ID,
	initialSupply uint64,
	maxSupply uint64,
	minConsumptionRate uint64,
	maxConsumptionRate uint64,
	minValidatorStake uint64,
	maxValidatorStake uint64,
	minStakeDuration time.Duration,
	maxStakeDuration time.Duration,
	maxValidatorWeightFactor byte,
	uptimeRequirement uint32,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueTransformSubnetTx(
		subnetID,
		assetID,
		initialSupply,
		maxSupply,
		minConsumptionRate,
		maxConsumptionRate,
		minValidatorStake,
		maxValidatorStake,
		minStakeDuration,
		maxStakeDuration,
		maxValidatorWeightFactor,
		uptimeRequirement,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueAddPermissionlessValidatorTx(
	vdr *txs.SubnetValidator,
	signer signer.Signer,
	assetID ids.ID,
	validationRewardsOwner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueAddPermissionlessValidatorTx(
		vdr,
		signer,
		assetID,
		validationRewardsOwner,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueUnsignedTx(
	utx txs.UnsignedTx,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueUnsignedTx(
		utx,
		common.UnionOptions(w.options, options)...,
	)
}

func (w *walletWithOptions) IssueTx(
	tx *txs.Tx,
	options ...common.Option,
) (ids.ID, error) {
	return w.Wallet.IssueTx(
		tx,
		common.UnionOptions(w.options, options)...,
	)
}
