// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package omegavm

import (
	"context"
	"time"

	"github.com/DioneProtocol/odysseygo/api"
	"github.com/DioneProtocol/odysseygo/ids"
	"github.com/DioneProtocol/odysseygo/utils/constants"
	"github.com/DioneProtocol/odysseygo/utils/crypto/secp256k1"
	"github.com/DioneProtocol/odysseygo/utils/formatting"
	"github.com/DioneProtocol/odysseygo/utils/formatting/address"
	"github.com/DioneProtocol/odysseygo/utils/json"
	"github.com/DioneProtocol/odysseygo/utils/rpc"
	"github.com/DioneProtocol/odysseygo/vms/omegavm/status"

	platformapi "github.com/DioneProtocol/odysseygo/vms/omegavm/api"
)

var _ Client = (*client)(nil)

// Client interface for interacting with the O Chain endpoint
type Client interface {
	// GetHeight returns the current block height of the O Chain
	GetHeight(ctx context.Context, options ...rpc.Option) (uint64, error)
	// ExportKey returns the private key corresponding to [address] from [user]'s account
	//
	// Deprecated: Keys should no longer be stored on the node.
	ExportKey(ctx context.Context, user api.UserPass, address ids.ShortID, options ...rpc.Option) (*secp256k1.PrivateKey, error)
	// ImportKey imports the specified [privateKey] to [user]'s keystore
	//
	// Deprecated: Keys should no longer be stored on the node.
	ImportKey(ctx context.Context, user api.UserPass, privateKey *secp256k1.PrivateKey, options ...rpc.Option) (ids.ShortID, error)
	// GetBalance returns the balance of [addrs] on the O Chain
	//
	// Deprecated: GetUTXOs should be used instead.
	GetBalance(ctx context.Context, addrs []ids.ShortID, options ...rpc.Option) (*GetBalanceResponse, error)
	// CreateAddress creates a new address for [user]
	//
	// Deprecated: Keys should no longer be stored on the node.
	CreateAddress(ctx context.Context, user api.UserPass, options ...rpc.Option) (ids.ShortID, error)
	// ListAddresses returns an array of omega addresses controlled by [user]
	//
	// Deprecated: Keys should no longer be stored on the node.
	ListAddresses(ctx context.Context, user api.UserPass, options ...rpc.Option) ([]ids.ShortID, error)
	// GetUTXOs returns the byte representation of the UTXOs controlled by [addrs]
	GetUTXOs(
		ctx context.Context,
		addrs []ids.ShortID,
		limit uint32,
		startAddress ids.ShortID,
		startUTXOID ids.ID,
		options ...rpc.Option,
	) ([][]byte, ids.ShortID, ids.ID, error)
	// GetAtomicUTXOs returns the byte representation of the atomic UTXOs controlled by [addrs]
	// from [sourceChain]
	GetAtomicUTXOs(
		ctx context.Context,
		addrs []ids.ShortID,
		sourceChain string,
		limit uint32,
		startAddress ids.ShortID,
		startUTXOID ids.ID,
		options ...rpc.Option,
	) ([][]byte, ids.ShortID, ids.ID, error)
	// GetSubnets returns information about the specified subnets
	//
	// Deprecated: Subnets should be fetched from a dedicated indexer.
	GetSubnets(ctx context.Context, subnetIDs []ids.ID, options ...rpc.Option) ([]ClientSubnet, error)
	// GetStakingAssetID returns the assetID of the asset used for staking on
	// subnet corresponding to [subnetID]
	GetStakingAssetID(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (ids.ID, error)
	// GetCurrentValidators returns the list of current validators for subnet with ID [subnetID]
	GetCurrentValidators(ctx context.Context, subnetID ids.ID, nodeIDs []ids.NodeID, options ...rpc.Option) ([]ClientPermissionlessValidator, error)
	// GetPendingValidators returns the list of pending validators for subnet with ID [subnetID]
	GetPendingValidators(ctx context.Context, subnetID ids.ID, nodeIDs []ids.NodeID, options ...rpc.Option) ([]interface{}, error)
	// GetCurrentSupply returns an upper bound on the supply of DIONE in the system
	GetCurrentSupply(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (uint64, error)
	// SampleValidators returns the nodeIDs of a sample of [sampleSize] validators from the current validator set for subnet with ID [subnetID]
	SampleValidators(ctx context.Context, subnetID ids.ID, sampleSize uint16, options ...rpc.Option) ([]ids.NodeID, error)
	// AddValidator issues a transaction to add a validator to the primary network
	// and returns the txID
	//
	// Deprecated: Transactions should be issued using the
	// `odysseygo/wallet/chain/p.Wallet` utility.
	AddValidator(
		ctx context.Context,
		user api.UserPass,
		from []ids.ShortID,
		changeAddr ids.ShortID,
		rewardAddress ids.ShortID,
		nodeID ids.NodeID,
		stakeAmount,
		startTime,
		endTime uint64,
		options ...rpc.Option,
	) (ids.ID, error)

	// AddSubnetValidator issues a transaction to add validator [nodeID] to subnet
	// with ID [subnetID] and returns the txID
	//
	// Deprecated: Transactions should be issued using the
	// `odysseygo/wallet/chain/p.Wallet` utility.
	AddSubnetValidator(
		ctx context.Context,
		user api.UserPass,
		from []ids.ShortID,
		changeAddr ids.ShortID,
		subnetID ids.ID,
		nodeID ids.NodeID,
		stakeAmount,
		startTime,
		endTime uint64,
		options ...rpc.Option,
	) (ids.ID, error)
	// CreateSubnet issues a transaction to create [subnet] and returns the txID
	//
	// Deprecated: Transactions should be issued using the
	// `odysseygo/wallet/chain/p.Wallet` utility.
	CreateSubnet(
		ctx context.Context,
		user api.UserPass,
		from []ids.ShortID,
		changeAddr ids.ShortID,
		controlKeys []ids.ShortID,
		threshold uint32,
		options ...rpc.Option,
	) (ids.ID, error)
	// ExportDIONE issues an ExportTx transaction and returns the txID
	//
	// Deprecated: Transactions should be issued using the
	// `odysseygo/wallet/chain/p.Wallet` utility.
	ExportDIONE(
		ctx context.Context,
		user api.UserPass,
		from []ids.ShortID,
		changeAddr ids.ShortID,
		to ids.ShortID,
		toChainIDAlias string,
		amount uint64,
		options ...rpc.Option,
	) (ids.ID, error)
	// ImportDIONE issues an ImportTx transaction and returns the txID
	//
	// Deprecated: Transactions should be issued using the
	// `odysseygo/wallet/chain/p.Wallet` utility.
	ImportDIONE(
		ctx context.Context,
		user api.UserPass,
		from []ids.ShortID,
		changeAddr ids.ShortID,
		to ids.ShortID,
		sourceChain string,
		options ...rpc.Option,
	) (ids.ID, error)
	// CreateBlockchain issues a CreateBlockchain transaction and returns the txID
	//
	// Deprecated: Transactions should be issued using the
	// `odysseygo/wallet/chain/p.Wallet` utility.
	CreateBlockchain(
		ctx context.Context,
		user api.UserPass,
		from []ids.ShortID,
		changeAddr ids.ShortID,
		subnetID ids.ID,
		vmID string,
		fxIDs []string,
		name string,
		genesisData []byte,
		options ...rpc.Option,
	) (ids.ID, error)
	// GetBlockchainStatus returns the current status of blockchain with ID: [blockchainID]
	GetBlockchainStatus(ctx context.Context, blockchainID string, options ...rpc.Option) (status.BlockchainStatus, error)
	// ValidatedBy returns the ID of the Subnet that validates [blockchainID]
	ValidatedBy(ctx context.Context, blockchainID ids.ID, options ...rpc.Option) (ids.ID, error)
	// Validates returns the list of blockchains that are validated by the subnet with ID [subnetID]
	Validates(ctx context.Context, subnetID ids.ID, options ...rpc.Option) ([]ids.ID, error)
	// GetBlockchains returns the list of blockchains on the omega
	//
	// Deprecated: Blockchains should be fetched from a dedicated indexer.
	GetBlockchains(ctx context.Context, options ...rpc.Option) ([]APIBlockchain, error)
	// IssueTx issues the transaction and returns its txID
	IssueTx(ctx context.Context, tx []byte, options ...rpc.Option) (ids.ID, error)
	// GetTx returns the byte representation of the transaction corresponding to [txID]
	GetTx(ctx context.Context, txID ids.ID, options ...rpc.Option) ([]byte, error)
	// GetTxStatus returns the status of the transaction corresponding to [txID]
	GetTxStatus(ctx context.Context, txID ids.ID, options ...rpc.Option) (*GetTxStatusResponse, error)
	// AwaitTxDecided polls [GetTxStatus] until a status is returned that
	// implies the tx may be decided.
	// TODO: Move this function off of the Client interface into a utility
	// function.
	AwaitTxDecided(
		ctx context.Context,
		txID ids.ID,
		freq time.Duration,
		options ...rpc.Option,
	) (*GetTxStatusResponse, error)
	// GetStake returns the amount of nDIONE that [addrs] have cumulatively
	// staked on the Primary Network.
	//
	// Deprecated: Stake should be calculated using GetTx, GetCurrentValidators,
	// and GetPendingValidators.
	GetStake(
		ctx context.Context,
		addrs []ids.ShortID,
		validatorsOnly bool,
		options ...rpc.Option,
	) (map[ids.ID]uint64, [][]byte, error)
	// GetMinStake returns the minimum staking amount in nDIONE for validators
	GetMinStake(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (uint64, error)
	// GetTotalStake returns the total amount (in nDIONE) staked on the network
	GetTotalStake(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (uint64, error)
	// GetMaxStakeAmount returns the maximum amount of nDIONE staking to the named
	// node during the time period.
	//
	// Deprecated: The MaxStakeAmount should be calculated using
	// GetCurrentValidators, and GetPendingValidators.
	GetMaxStakeAmount(
		ctx context.Context,
		subnetID ids.ID,
		nodeID ids.NodeID,
		startTime uint64,
		endTime uint64,
		options ...rpc.Option,
	) (uint64, error)
	// GetRewardUTXOs returns the reward UTXOs for a transaction
	//
	// Deprecated: GetRewardUTXOs should be fetched from a dedicated indexer.
	GetRewardUTXOs(context.Context, *api.GetTxArgs, ...rpc.Option) ([][]byte, error)
	// GetTimestamp returns the current chain timestamp
	GetTimestamp(ctx context.Context, options ...rpc.Option) (time.Time, error)
	// GetValidatorsAt returns the weights of the validator set of a provided subnet
	// at the specified height.
	GetValidatorsAt(ctx context.Context, subnetID ids.ID, height uint64, options ...rpc.Option) (map[ids.NodeID]uint64, error)
	// GetBlock returns the block with the given id.
	GetBlock(ctx context.Context, blockID ids.ID, options ...rpc.Option) ([]byte, error)
}

// Client implementation for interacting with the O Chain endpoint
type client struct {
	requester rpc.EndpointRequester
}

// NewClient returns a Client for interacting with the O Chain endpoint
func NewClient(uri string) Client {
	return &client{requester: rpc.NewEndpointRequester(
		uri + "/ext/O",
	)}
}

func (c *client) GetHeight(ctx context.Context, options ...rpc.Option) (uint64, error) {
	res := &api.GetHeightResponse{}
	err := c.requester.SendRequest(ctx, "omega.getHeight", struct{}{}, res, options...)
	return uint64(res.Height), err
}

func (c *client) ExportKey(ctx context.Context, user api.UserPass, address ids.ShortID, options ...rpc.Option) (*secp256k1.PrivateKey, error) {
	res := &ExportKeyReply{}
	err := c.requester.SendRequest(ctx, "omega.exportKey", &ExportKeyArgs{
		UserPass: user,
		Address:  address.String(),
	}, res, options...)
	return res.PrivateKey, err
}

func (c *client) ImportKey(ctx context.Context, user api.UserPass, privateKey *secp256k1.PrivateKey, options ...rpc.Option) (ids.ShortID, error) {
	res := &api.JSONAddress{}
	err := c.requester.SendRequest(ctx, "omega.importKey", &ImportKeyArgs{
		UserPass:   user,
		PrivateKey: privateKey,
	}, res, options...)
	if err != nil {
		return ids.ShortID{}, err
	}
	return address.ParseToID(res.Address)
}

func (c *client) GetBalance(ctx context.Context, addrs []ids.ShortID, options ...rpc.Option) (*GetBalanceResponse, error) {
	res := &GetBalanceResponse{}
	err := c.requester.SendRequest(ctx, "omega.getBalance", &GetBalanceRequest{
		Addresses: ids.ShortIDsToStrings(addrs),
	}, res, options...)
	return res, err
}

func (c *client) CreateAddress(ctx context.Context, user api.UserPass, options ...rpc.Option) (ids.ShortID, error) {
	res := &api.JSONAddress{}
	err := c.requester.SendRequest(ctx, "omega.createAddress", &user, res, options...)
	if err != nil {
		return ids.ShortID{}, err
	}
	return address.ParseToID(res.Address)
}

func (c *client) ListAddresses(ctx context.Context, user api.UserPass, options ...rpc.Option) ([]ids.ShortID, error) {
	res := &api.JSONAddresses{}
	err := c.requester.SendRequest(ctx, "omega.listAddresses", &user, res, options...)
	if err != nil {
		return nil, err
	}
	return address.ParseToIDs(res.Addresses)
}

func (c *client) GetUTXOs(
	ctx context.Context,
	addrs []ids.ShortID,
	limit uint32,
	startAddress ids.ShortID,
	startUTXOID ids.ID,
	options ...rpc.Option,
) ([][]byte, ids.ShortID, ids.ID, error) {
	return c.GetAtomicUTXOs(ctx, addrs, "", limit, startAddress, startUTXOID, options...)
}

func (c *client) GetAtomicUTXOs(
	ctx context.Context,
	addrs []ids.ShortID,
	sourceChain string,
	limit uint32,
	startAddress ids.ShortID,
	startUTXOID ids.ID,
	options ...rpc.Option,
) ([][]byte, ids.ShortID, ids.ID, error) {
	res := &api.GetUTXOsReply{}
	err := c.requester.SendRequest(ctx, "omega.getUTXOs", &api.GetUTXOsArgs{
		Addresses:   ids.ShortIDsToStrings(addrs),
		SourceChain: sourceChain,
		Limit:       json.Uint32(limit),
		StartIndex: api.Index{
			Address: startAddress.String(),
			UTXO:    startUTXOID.String(),
		},
		Encoding: formatting.Hex,
	}, res, options...)
	if err != nil {
		return nil, ids.ShortID{}, ids.Empty, err
	}

	utxos := make([][]byte, len(res.UTXOs))
	for i, utxo := range res.UTXOs {
		utxoBytes, err := formatting.Decode(res.Encoding, utxo)
		if err != nil {
			return nil, ids.ShortID{}, ids.Empty, err
		}
		utxos[i] = utxoBytes
	}
	endAddr, err := address.ParseToID(res.EndIndex.Address)
	if err != nil {
		return nil, ids.ShortID{}, ids.Empty, err
	}
	endUTXOID, err := ids.FromString(res.EndIndex.UTXO)
	return utxos, endAddr, endUTXOID, err
}

// ClientSubnet is a representation of a subnet used in client methods
type ClientSubnet struct {
	// ID of the subnet
	ID ids.ID
	// Each element of [ControlKeys] the address of a public key.
	// A transaction to add a validator to this subnet requires
	// signatures from [Threshold] of these keys to be valid.
	ControlKeys []ids.ShortID
	Threshold   uint32
}

func (c *client) GetSubnets(ctx context.Context, ids []ids.ID, options ...rpc.Option) ([]ClientSubnet, error) {
	res := &GetSubnetsResponse{}
	err := c.requester.SendRequest(ctx, "omega.getSubnets", &GetSubnetsArgs{
		IDs: ids,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	subnets := make([]ClientSubnet, len(res.Subnets))
	for i, apiSubnet := range res.Subnets {
		controlKeys, err := address.ParseToIDs(apiSubnet.ControlKeys)
		if err != nil {
			return nil, err
		}

		subnets[i] = ClientSubnet{
			ID:          apiSubnet.ID,
			ControlKeys: controlKeys,
			Threshold:   uint32(apiSubnet.Threshold),
		}
	}
	return subnets, nil
}

func (c *client) GetStakingAssetID(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (ids.ID, error) {
	res := &GetStakingAssetIDResponse{}
	err := c.requester.SendRequest(ctx, "omega.getStakingAssetID", &GetStakingAssetIDArgs{
		SubnetID: subnetID,
	}, res, options...)
	return res.AssetID, err
}

func (c *client) GetCurrentValidators(
	ctx context.Context,
	subnetID ids.ID,
	nodeIDs []ids.NodeID,
	options ...rpc.Option,
) ([]ClientPermissionlessValidator, error) {
	res := &GetCurrentValidatorsReply{}
	err := c.requester.SendRequest(ctx, "omega.getCurrentValidators", &GetCurrentValidatorsArgs{
		SubnetID: subnetID,
		NodeIDs:  nodeIDs,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	return getClientPermissionlessValidators(res.Validators)
}

func (c *client) GetPendingValidators(
	ctx context.Context,
	subnetID ids.ID,
	nodeIDs []ids.NodeID,
	options ...rpc.Option,
) ([]interface{}, error) {
	res := &GetPendingValidatorsReply{}
	err := c.requester.SendRequest(ctx, "omega.getPendingValidators", &GetPendingValidatorsArgs{
		SubnetID: subnetID,
		NodeIDs:  nodeIDs,
	}, res, options...)
	return res.Validators, err
}

func (c *client) GetCurrentSupply(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (uint64, error) {
	res := &GetCurrentSupplyReply{}
	err := c.requester.SendRequest(ctx, "omega.getCurrentSupply", &GetCurrentSupplyArgs{
		SubnetID: subnetID,
	}, res, options...)
	return uint64(res.Supply), err
}

func (c *client) SampleValidators(ctx context.Context, subnetID ids.ID, sampleSize uint16, options ...rpc.Option) ([]ids.NodeID, error) {
	res := &SampleValidatorsReply{}
	err := c.requester.SendRequest(ctx, "omega.sampleValidators", &SampleValidatorsArgs{
		SubnetID: subnetID,
		Size:     json.Uint16(sampleSize),
	}, res, options...)
	return res.Validators, err
}

func (c *client) AddValidator(
	ctx context.Context,
	user api.UserPass,
	from []ids.ShortID,
	changeAddr ids.ShortID,
	rewardAddress ids.ShortID,
	nodeID ids.NodeID,
	stakeAmount,
	startTime,
	endTime uint64,
	options ...rpc.Option,
) (ids.ID, error) {
	res := &api.JSONTxID{}
	jsonStakeAmount := json.Uint64(stakeAmount)
	err := c.requester.SendRequest(ctx, "omega.addValidator", &AddValidatorArgs{
		JSONSpendHeader: api.JSONSpendHeader{
			UserPass:       user,
			JSONFromAddrs:  api.JSONFromAddrs{From: ids.ShortIDsToStrings(from)},
			JSONChangeAddr: api.JSONChangeAddr{ChangeAddr: changeAddr.String()},
		},
		Staker: platformapi.Staker{
			NodeID:      nodeID,
			Weight:      jsonStakeAmount,
			StakeAmount: &jsonStakeAmount,
			StartTime:   json.Uint64(startTime),
			EndTime:     json.Uint64(endTime),
		},
		RewardAddress: rewardAddress.String(),
	}, res, options...)
	return res.TxID, err
}

func (c *client) AddSubnetValidator(
	ctx context.Context,
	user api.UserPass,
	from []ids.ShortID,
	changeAddr ids.ShortID,
	subnetID ids.ID,
	nodeID ids.NodeID,
	stakeAmount,
	startTime,
	endTime uint64,
	options ...rpc.Option,
) (ids.ID, error) {
	res := &api.JSONTxID{}
	jsonStakeAmount := json.Uint64(stakeAmount)
	err := c.requester.SendRequest(ctx, "omega.addSubnetValidator", &AddSubnetValidatorArgs{
		JSONSpendHeader: api.JSONSpendHeader{
			UserPass:       user,
			JSONFromAddrs:  api.JSONFromAddrs{From: ids.ShortIDsToStrings(from)},
			JSONChangeAddr: api.JSONChangeAddr{ChangeAddr: changeAddr.String()},
		},
		Staker: platformapi.Staker{
			NodeID:      nodeID,
			Weight:      jsonStakeAmount,
			StakeAmount: &jsonStakeAmount,
			StartTime:   json.Uint64(startTime),
			EndTime:     json.Uint64(endTime),
		},
		SubnetID: subnetID.String(),
	}, res, options...)
	return res.TxID, err
}

func (c *client) CreateSubnet(
	ctx context.Context,
	user api.UserPass,
	from []ids.ShortID,
	changeAddr ids.ShortID,
	controlKeys []ids.ShortID,
	threshold uint32,
	options ...rpc.Option,
) (ids.ID, error) {
	res := &api.JSONTxID{}
	err := c.requester.SendRequest(ctx, "omega.createSubnet", &CreateSubnetArgs{
		JSONSpendHeader: api.JSONSpendHeader{
			UserPass:       user,
			JSONFromAddrs:  api.JSONFromAddrs{From: ids.ShortIDsToStrings(from)},
			JSONChangeAddr: api.JSONChangeAddr{ChangeAddr: changeAddr.String()},
		},
		APISubnet: APISubnet{
			ControlKeys: ids.ShortIDsToStrings(controlKeys),
			Threshold:   json.Uint32(threshold),
		},
	}, res, options...)
	return res.TxID, err
}

func (c *client) ExportDIONE(
	ctx context.Context,
	user api.UserPass,
	from []ids.ShortID,
	changeAddr ids.ShortID,
	to ids.ShortID,
	targetChain string,
	amount uint64,
	options ...rpc.Option,
) (ids.ID, error) {
	res := &api.JSONTxID{}
	err := c.requester.SendRequest(ctx, "omega.exportDIONE", &ExportDIONEArgs{
		JSONSpendHeader: api.JSONSpendHeader{
			UserPass:       user,
			JSONFromAddrs:  api.JSONFromAddrs{From: ids.ShortIDsToStrings(from)},
			JSONChangeAddr: api.JSONChangeAddr{ChangeAddr: changeAddr.String()},
		},
		TargetChain: targetChain,
		To:          to.String(),
		Amount:      json.Uint64(amount),
	}, res, options...)
	return res.TxID, err
}

func (c *client) ImportDIONE(
	ctx context.Context,
	user api.UserPass,
	from []ids.ShortID,
	changeAddr ids.ShortID,
	to ids.ShortID,
	sourceChain string,
	options ...rpc.Option,
) (ids.ID, error) {
	res := &api.JSONTxID{}
	err := c.requester.SendRequest(ctx, "omega.importDIONE", &ImportDIONEArgs{
		JSONSpendHeader: api.JSONSpendHeader{
			UserPass:       user,
			JSONFromAddrs:  api.JSONFromAddrs{From: ids.ShortIDsToStrings(from)},
			JSONChangeAddr: api.JSONChangeAddr{ChangeAddr: changeAddr.String()},
		},
		To:          to.String(),
		SourceChain: sourceChain,
	}, res, options...)
	return res.TxID, err
}

func (c *client) CreateBlockchain(
	ctx context.Context,
	user api.UserPass,
	from []ids.ShortID,
	changeAddr ids.ShortID,
	subnetID ids.ID,
	vmID string,
	fxIDs []string,
	name string,
	genesisData []byte,
	options ...rpc.Option,
) (ids.ID, error) {
	genesisDataStr, err := formatting.Encode(formatting.Hex, genesisData)
	if err != nil {
		return ids.ID{}, err
	}

	res := &api.JSONTxID{}
	err = c.requester.SendRequest(ctx, "omega.createBlockchain", &CreateBlockchainArgs{
		JSONSpendHeader: api.JSONSpendHeader{
			UserPass:       user,
			JSONFromAddrs:  api.JSONFromAddrs{From: ids.ShortIDsToStrings(from)},
			JSONChangeAddr: api.JSONChangeAddr{ChangeAddr: changeAddr.String()},
		},
		SubnetID:    subnetID,
		VMID:        vmID,
		FxIDs:       fxIDs,
		Name:        name,
		GenesisData: genesisDataStr,
		Encoding:    formatting.Hex,
	}, res, options...)
	return res.TxID, err
}

func (c *client) GetBlockchainStatus(ctx context.Context, blockchainID string, options ...rpc.Option) (status.BlockchainStatus, error) {
	res := &GetBlockchainStatusReply{}
	err := c.requester.SendRequest(ctx, "omega.getBlockchainStatus", &GetBlockchainStatusArgs{
		BlockchainID: blockchainID,
	}, res, options...)
	return res.Status, err
}

func (c *client) ValidatedBy(ctx context.Context, blockchainID ids.ID, options ...rpc.Option) (ids.ID, error) {
	res := &ValidatedByResponse{}
	err := c.requester.SendRequest(ctx, "omega.validatedBy", &ValidatedByArgs{
		BlockchainID: blockchainID,
	}, res, options...)
	return res.SubnetID, err
}

func (c *client) Validates(ctx context.Context, subnetID ids.ID, options ...rpc.Option) ([]ids.ID, error) {
	res := &ValidatesResponse{}
	err := c.requester.SendRequest(ctx, "omega.validates", &ValidatesArgs{
		SubnetID: subnetID,
	}, res, options...)
	return res.BlockchainIDs, err
}

func (c *client) GetBlockchains(ctx context.Context, options ...rpc.Option) ([]APIBlockchain, error) {
	res := &GetBlockchainsResponse{}
	err := c.requester.SendRequest(ctx, "omega.getBlockchains", struct{}{}, res, options...)
	return res.Blockchains, err
}

func (c *client) IssueTx(ctx context.Context, txBytes []byte, options ...rpc.Option) (ids.ID, error) {
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return ids.ID{}, err
	}

	res := &api.JSONTxID{}
	err = c.requester.SendRequest(ctx, "omega.issueTx", &api.FormattedTx{
		Tx:       txStr,
		Encoding: formatting.Hex,
	}, res, options...)
	return res.TxID, err
}

func (c *client) GetTx(ctx context.Context, txID ids.ID, options ...rpc.Option) ([]byte, error) {
	res := &api.FormattedTx{}
	err := c.requester.SendRequest(ctx, "omega.getTx", &api.GetTxArgs{
		TxID:     txID,
		Encoding: formatting.Hex,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	return formatting.Decode(res.Encoding, res.Tx)
}

func (c *client) GetTxStatus(ctx context.Context, txID ids.ID, options ...rpc.Option) (*GetTxStatusResponse, error) {
	res := &GetTxStatusResponse{}
	err := c.requester.SendRequest(
		ctx,
		"omega.getTxStatus",
		&GetTxStatusArgs{
			TxID: txID,
		},
		res,
		options...,
	)
	return res, err
}

func (c *client) AwaitTxDecided(ctx context.Context, txID ids.ID, freq time.Duration, options ...rpc.Option) (*GetTxStatusResponse, error) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()

	for {
		res, err := c.GetTxStatus(ctx, txID, options...)
		if err == nil {
			switch res.Status {
			case status.Committed, status.Aborted, status.Dropped:
				return res, nil
			}
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *client) GetStake(
	ctx context.Context,
	addrs []ids.ShortID,
	validatorsOnly bool,
	options ...rpc.Option,
) (map[ids.ID]uint64, [][]byte, error) {
	res := &GetStakeReply{}
	err := c.requester.SendRequest(ctx, "omega.getStake", &GetStakeArgs{
		JSONAddresses: api.JSONAddresses{
			Addresses: ids.ShortIDsToStrings(addrs),
		},
		ValidatorsOnly: validatorsOnly,
		Encoding:       formatting.Hex,
	}, res, options...)
	if err != nil {
		return nil, nil, err
	}

	staked := make(map[ids.ID]uint64, len(res.Stakeds))
	for assetID, amount := range res.Stakeds {
		staked[assetID] = uint64(amount)
	}

	outputs := make([][]byte, len(res.Outputs))
	for i, outputStr := range res.Outputs {
		output, err := formatting.Decode(res.Encoding, outputStr)
		if err != nil {
			return nil, nil, err
		}
		outputs[i] = output
	}
	return staked, outputs, err
}

func (c *client) GetMinStake(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (uint64, error) {
	res := &GetMinStakeReply{}
	err := c.requester.SendRequest(ctx, "omega.getMinStake", &GetMinStakeArgs{
		SubnetID: subnetID,
	}, res, options...)
	return uint64(res.MinValidatorStake), err
}

func (c *client) GetTotalStake(ctx context.Context, subnetID ids.ID, options ...rpc.Option) (uint64, error) {
	res := &GetTotalStakeReply{}
	err := c.requester.SendRequest(ctx, "omega.getTotalStake", &GetTotalStakeArgs{
		SubnetID: subnetID,
	}, res, options...)
	var amount json.Uint64
	if subnetID == constants.PrimaryNetworkID {
		amount = res.Stake
	} else {
		amount = res.Weight
	}
	return uint64(amount), err
}

func (c *client) GetMaxStakeAmount(ctx context.Context, subnetID ids.ID, nodeID ids.NodeID, startTime, endTime uint64, options ...rpc.Option) (uint64, error) {
	res := &GetMaxStakeAmountReply{}
	err := c.requester.SendRequest(ctx, "omega.getMaxStakeAmount", &GetMaxStakeAmountArgs{
		SubnetID:  subnetID,
		NodeID:    nodeID,
		StartTime: json.Uint64(startTime),
		EndTime:   json.Uint64(endTime),
	}, res, options...)
	return uint64(res.Amount), err
}

func (c *client) GetRewardUTXOs(ctx context.Context, args *api.GetTxArgs, options ...rpc.Option) ([][]byte, error) {
	res := &GetRewardUTXOsReply{}
	err := c.requester.SendRequest(ctx, "omega.getRewardUTXOs", args, res, options...)
	if err != nil {
		return nil, err
	}
	utxos := make([][]byte, len(res.UTXOs))
	for i, utxoStr := range res.UTXOs {
		utxoBytes, err := formatting.Decode(res.Encoding, utxoStr)
		if err != nil {
			return nil, err
		}
		utxos[i] = utxoBytes
	}
	return utxos, err
}

func (c *client) GetTimestamp(ctx context.Context, options ...rpc.Option) (time.Time, error) {
	res := &GetTimestampReply{}
	err := c.requester.SendRequest(ctx, "omega.getTimestamp", struct{}{}, res, options...)
	return res.Timestamp, err
}

func (c *client) GetValidatorsAt(ctx context.Context, subnetID ids.ID, height uint64, options ...rpc.Option) (map[ids.NodeID]uint64, error) {
	res := &GetValidatorsAtReply{}
	err := c.requester.SendRequest(ctx, "omega.getValidatorsAt", &GetValidatorsAtArgs{
		SubnetID: subnetID,
		Height:   json.Uint64(height),
	}, res, options...)
	return res.Validators, err
}

func (c *client) GetBlock(ctx context.Context, blockID ids.ID, options ...rpc.Option) ([]byte, error) {
	res := &api.FormattedBlock{}
	if err := c.requester.SendRequest(ctx, "omega.getBlock", &api.GetBlockArgs{
		BlockID:  blockID,
		Encoding: formatting.Hex,
	}, res, options...); err != nil {
		return nil, err
	}
	return formatting.Decode(res.Encoding, res.Block)
}