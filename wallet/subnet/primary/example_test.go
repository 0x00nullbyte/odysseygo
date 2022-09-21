// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package primary

import (
	"context"
	"log"
	"time"

	"github.com/ava-labs/avalanchego/genesis"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	"github.com/ava-labs/avalanchego/vms/platformvm/validator"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

func ExampleWallet() {
	ctx := context.Background()
	kc := secp256k1fx.NewKeychain(genesis.EWOQKey)

	// NewWalletFromURI fetches the available UTXOs owned by [kc] on the network
	// that [LocalAPIURI] is hosting.
	walletSyncStartTime := time.Now()
	wallet, err := NewWalletFromURI(ctx, LocalAPIURI, kc)
	if err != nil {
		log.Fatalf("failed to initialize wallet with: %s\n", err)
		return
	}
	log.Printf("synced wallet in %s\n", time.Since(walletSyncStartTime))

	// Get the P-chain and the X-chain wallets
	pWallet := wallet.P()
	xWallet := wallet.X()

	// Pull out useful constants to use when issuing transactions.
	xChainID := xWallet.BlockchainID()
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}

	// Create a custom asset to send to the P-chain.
	createAssetStartTime := time.Now()
	createAssetTxID, err := xWallet.IssueCreateAssetTx(
		"RnM",
		"RNM",
		9,
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          100 * units.MegaAvax,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to create new X-chain asset with: %s\n", err)
		return
	}
	log.Printf("created X-chain asset %s in %s\n", createAssetTxID, time.Since(createAssetStartTime))

	// Send 100 schmeckles to the P-chain.
	exportStartTime := time.Now()
	exportTxID, err := xWallet.IssueExportTx(
		constants.PlatformChainID,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: createAssetTxID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          100 * units.MegaAvax,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to issue X->P export transaction with: %s\n", err)
		return
	}
	log.Printf("issued X->P export %s in %s\n", exportTxID, time.Since(exportStartTime))

	// Import the 100 schmeckles from the X-chain into the P-chain.
	importStartTime := time.Now()
	importTxID, err := pWallet.IssueImportTx(xChainID, owner)
	if err != nil {
		log.Fatalf("failed to issue X->P import transaction with: %s\n", err)
		return
	}
	log.Printf("issued X->P import %s in %s\n", importTxID, time.Since(importStartTime))

	createSubnetStartTime := time.Now()
	createSubnetTxID, err := pWallet.IssueCreateSubnetTx(owner)
	if err != nil {
		log.Fatalf("failed to issue create subnet transaction with: %s\n", err)
		return
	}
	log.Printf("issued create subnet transaction %s in %s\n", createSubnetTxID, time.Since(createSubnetStartTime))

	transformSubnetStartTime := time.Now()
	transformSubnetTxID, err := pWallet.IssueTransformSubnetTx(
		createSubnetTxID,
		createAssetTxID,
		50*units.MegaAvax,
		100*units.MegaAvax,
		reward.PercentDenominator,
		reward.PercentDenominator,
		1,
		100*units.MegaAvax,
		time.Second,
		365*24*time.Hour,
		0,
		1,
		5,
		.80*reward.PercentDenominator,
	)
	if err != nil {
		log.Fatalf("failed to issue transform subnet transaction with: %s\n", err)
		return
	}
	log.Printf("issued transform subnet transaction %s in %s\n", transformSubnetTxID, time.Since(transformSubnetStartTime))

	addPermissionlessValidatorStartTime := time.Now()
	startTime := time.Now().Add(time.Minute)
	addSubnetValidatorTxID, err := pWallet.IssueAddPermissionlessValidatorTx(
		&validator.SubnetValidator{
			Validator: validator.Validator{
				NodeID: genesis.LocalConfig.InitialStakers[0].NodeID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(5 * time.Second).Unix()),
				Wght:   25 * units.MegaAvax,
			},
			Subnet: createSubnetTxID,
		},
		&signer.Empty{},
		createAssetTxID,
		&secp256k1fx.OutputOwners{},
		&secp256k1fx.OutputOwners{},
		reward.PercentDenominator,
	)
	if err != nil {
		log.Fatalf("failed to issue add subnet validator with: %s\n", err)
		return
	}
	log.Printf("issued add subnet validator transaction %s in %s\n", addSubnetValidatorTxID, time.Since(addPermissionlessValidatorStartTime))

	addPermissionlessDelegatorStartTime := time.Now()
	addSubnetDelegatorTxID, err := pWallet.IssueAddPermissionlessDelegatorTx(
		&validator.SubnetValidator{
			Validator: validator.Validator{
				NodeID: genesis.LocalConfig.InitialStakers[0].NodeID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(5 * time.Second).Unix()),
				Wght:   25 * units.MegaAvax,
			},
			Subnet: createSubnetTxID,
		},
		createAssetTxID,
		&secp256k1fx.OutputOwners{},
	)
	if err != nil {
		log.Fatalf("failed to issue add subnet delegator with: %s\n", err)
		return
	}
	log.Printf("issued add subnet validator delegator %s in %s\n", addSubnetDelegatorTxID, time.Since(addPermissionlessDelegatorStartTime))
}
