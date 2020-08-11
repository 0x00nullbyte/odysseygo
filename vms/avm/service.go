// (c) 2019-2020, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package avm

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/ava-labs/gecko/api"
	"github.com/ava-labs/gecko/ids"
	"github.com/ava-labs/gecko/snow/choices"
	"github.com/ava-labs/gecko/utils/constants"
	"github.com/ava-labs/gecko/utils/crypto"
	"github.com/ava-labs/gecko/utils/formatting"
	"github.com/ava-labs/gecko/utils/hashing"
	"github.com/ava-labs/gecko/utils/json"
	"github.com/ava-labs/gecko/vms/components/avax"
	"github.com/ava-labs/gecko/vms/components/verify"
	"github.com/ava-labs/gecko/vms/nftfx"
	"github.com/ava-labs/gecko/vms/secp256k1fx"

	safemath "github.com/ava-labs/gecko/utils/math"
)

var (
	errUnknownAssetID         = errors.New("unknown asset ID")
	errTxNotCreateAsset       = errors.New("transaction doesn't create an asset")
	errNoHolders              = errors.New("initialHolders must not be empty")
	errNoMinters              = errors.New("no minters provided")
	errInvalidAmount          = errors.New("amount must be positive")
	errSpendOverflow          = errors.New("spent amount overflows uint64")
	errInvalidMintAmount      = errors.New("amount minted must be positive")
	errAddressesCantMintAsset = errors.New("provided addresses don't have the authority to mint the provided asset")
	errInvalidUTXO            = errors.New("invalid utxo")
	errNilTxID                = errors.New("nil transaction ID")
)

// Service defines the base service for the asset vm
type Service struct{ vm *VM }

// FormattedTx defines a JSON formatted struct containing a Tx in CB58 format
type FormattedTx struct {
	Tx formatting.CB58 `json:"tx"`
}

// FormattedUTXOs defines a JSON formatted struct containing UTXOs in CB58 format
type FormattedUTXOs struct {
	UTXOs []formatting.CB58 `json:"utxos"`
}

// FormattedAssetID defines a JSON formatted struct containing an assetID as a string
type FormattedAssetID struct {
	AssetID ids.ID `json:"assetID"`
}

// IssueTx attempts to issue a transaction into consensus
func (service *Service) IssueTx(r *http.Request, args *FormattedTx, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: IssueTx called with %s", args.Tx)

	txID, err := service.vm.IssueTx(args.Tx.Bytes, nil)
	if err != nil {
		return err
	}

	reply.TxID = txID
	return nil
}

// GetTxStatusReply defines the GetTxStatus replies returned from the API
type GetTxStatusReply struct {
	Status choices.Status `json:"status"`
}

// GetTxStatus returns the status of the specified transaction
func (service *Service) GetTxStatus(r *http.Request, args *api.JsonTxID, reply *GetTxStatusReply) error {
	service.vm.ctx.Log.Info("AVM: GetTxStatus called with %s", args.TxID)

	if args.TxID.IsZero() {
		return errNilTxID
	}

	tx := UniqueTx{
		vm:   service.vm,
		txID: args.TxID,
	}

	reply.Status = tx.Status()
	return nil
}

// GetTx returns the specified transaction
func (service *Service) GetTx(r *http.Request, args *api.JsonTxID, reply *FormattedTx) error {
	service.vm.ctx.Log.Info("AVM: GetTx called with %s", args.TxID)

	if args.TxID.IsZero() {
		return errNilTxID
	}

	tx := UniqueTx{
		vm:   service.vm,
		txID: args.TxID,
	}
	if status := tx.Status(); !status.Fetched() {
		return errUnknownTx
	}

	reply.Tx.Bytes = tx.Bytes()
	return nil
}

// GetUTXOs gets all utxos for passed in addresses
func (service *Service) GetUTXOs(r *http.Request, args *api.JsonAddresses, reply *FormattedUTXOs) error {
	service.vm.ctx.Log.Info("AVM: GetUTXOs called with %s", args.Addresses)

	addrSet := ids.Set{}
	for _, addr := range args.Addresses {
		addrBytes, err := service.vm.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("problem parsing address '%s': %w", addr, err)
		}
		addrSet.Add(ids.NewID(hashing.ComputeHash256Array(addrBytes)))
	}

	utxos, err := service.vm.GetUTXOs(addrSet)
	if err != nil {
		return fmt.Errorf("problem retrieving UTXOs: %w", err)
	}

	reply.UTXOs = []formatting.CB58{}
	for _, utxo := range utxos {
		b, err := service.vm.codec.Marshal(utxo)
		if err != nil {
			return fmt.Errorf("problem marshalling UTXO: %w", err)
		}
		reply.UTXOs = append(reply.UTXOs, formatting.CB58{Bytes: b})
	}
	return nil
}

// GetAtomicUTXOs gets all atomic utxos for passed in addresses
func (service *Service) GetAtomicUTXOs(r *http.Request, args *api.JsonAddresses, reply *FormattedUTXOs) error {
	service.vm.ctx.Log.Info("GetAtomicUTXOs called with %s", args.Addresses)

	addrSet := ids.Set{}
	for _, addr := range args.Addresses {
		addrBytes, err := service.vm.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("problem parsing address '%s': %w", addr, err)
		}
		addrSet.Add(ids.NewID(hashing.ComputeHash256Array(addrBytes)))
	}

	utxos, err := service.vm.GetAtomicUTXOs(addrSet)
	if err != nil {
		return fmt.Errorf("problem retrieving atomic UTXOs: %w", err)
	}

	reply.UTXOs = []formatting.CB58{}
	for _, utxo := range utxos {
		b, err := service.vm.codec.Marshal(utxo)
		if err != nil {
			return fmt.Errorf("problem marshalling atomic UTXO: %w", err)
		}
		reply.UTXOs = append(reply.UTXOs, formatting.CB58{Bytes: b})
	}
	return nil
}

// GetAssetDescriptionArgs are arguments for passing into GetAssetDescription requests
type GetAssetDescriptionArgs struct {
	AssetID string `json:"assetID"`
}

// GetAssetDescriptionReply defines the GetAssetDescription replies returned from the API
type GetAssetDescriptionReply struct {
	AssetID      ids.ID     `json:"assetID"`
	Name         string     `json:"name"`
	Symbol       string     `json:"symbol"`
	Denomination json.Uint8 `json:"denomination"`
}

// GetAssetDescription creates an empty account with the name passed in
func (service *Service) GetAssetDescription(_ *http.Request, args *GetAssetDescriptionArgs, reply *GetAssetDescriptionReply) error {
	service.vm.ctx.Log.Info("AVM: GetAssetDescription called with %s", args.AssetID)

	assetID, err := service.vm.Lookup(args.AssetID)
	if err != nil {
		assetID, err = ids.FromString(args.AssetID)
		if err != nil {
			return fmt.Errorf("couldn't find asset with ID: %s", args.AssetID)
		}
	}

	tx := &UniqueTx{
		vm:   service.vm,
		txID: assetID,
	}
	if status := tx.Status(); !status.Fetched() {
		return errUnknownAssetID
	}
	createAssetTx, ok := tx.UnsignedTx.(*CreateAssetTx)
	if !ok {
		return errTxNotCreateAsset
	}

	reply.AssetID = assetID
	reply.Name = createAssetTx.Name
	reply.Symbol = createAssetTx.Symbol
	reply.Denomination = json.Uint8(createAssetTx.Denomination)

	return nil
}

// GetBalanceArgs are arguments for passing into GetBalance requests
type GetBalanceArgs struct {
	Address string `json:"address"`
	AssetID string `json:"assetID"`
}

// GetBalanceReply defines the GetBalance replies returned from the API
type GetBalanceReply struct {
	Balance json.Uint64   `json:"balance"`
	UTXOIDs []avax.UTXOID `json:"utxoIDs"`
}

// GetBalance returns the amount of an asset that an address at least partially owns
func (service *Service) GetBalance(r *http.Request, args *GetBalanceArgs, reply *GetBalanceReply) error {
	service.vm.ctx.Log.Info("AVM: GetBalance called with address: %s assetID: %s", args.Address, args.AssetID)

	address, err := service.vm.ParseAddress(args.Address)
	if err != nil {
		return fmt.Errorf("problem parsing address '%s': %w", args.Address, err)
	}

	assetID, err := service.vm.Lookup(args.AssetID)
	if err != nil {
		assetID, err = ids.FromString(args.AssetID)
		if err != nil {
			return fmt.Errorf("problem parsing assetID '%s': %w", args.AssetID, err)
		}
	}

	addrSet := ids.Set{}
	addrSet.Add(ids.NewID(hashing.ComputeHash256Array(address)))

	utxos, err := service.vm.GetUTXOs(addrSet)
	if err != nil {
		return fmt.Errorf("problem retrieving UTXOs: %w", err)
	}

	reply.UTXOIDs = make([]avax.UTXOID, 0, len(utxos))
	for _, utxo := range utxos {
		if !utxo.AssetID().Equals(assetID) {
			continue
		}
		transferable, ok := utxo.Out.(avax.TransferableOut)
		if !ok {
			continue
		}
		amt, err := safemath.Add64(transferable.Amount(), uint64(reply.Balance))
		if err != nil {
			return err
		}
		reply.Balance = json.Uint64(amt)
		reply.UTXOIDs = append(reply.UTXOIDs, utxo.UTXOID)
	}

	return nil
}

// Balance ...
type Balance struct {
	AssetID string      `json:"asset"`
	Balance json.Uint64 `json:"balance"`
}

// GetAllBalancesReply is the response from a call to GetAllBalances
type GetAllBalancesReply struct {
	Balances []Balance `json:"balances"`
}

// GetAllBalances returns a map where:
//   Key: ID of an asset such that [args.Address] has a non-zero balance of the asset
//   Value: The balance of the asset held by the address
// Note that balances include assets that the address only _partially_ owns
// (ie is one of several addresses specified in a multi-sig)
func (service *Service) GetAllBalances(r *http.Request, args *api.JsonAddress, reply *GetAllBalancesReply) error {
	service.vm.ctx.Log.Info("AVM: GetAllBalances called with address: %s", args.Address)

	address, err := service.vm.ParseAddress(args.Address)
	if err != nil {
		return fmt.Errorf("problem parsing address '%s': %w", args.Address, err)
	}
	addrAsSet := ids.Set{}
	addrAsSet.Add(ids.NewID(hashing.ComputeHash256Array(address)))

	utxos, err := service.vm.GetUTXOs(addrAsSet)
	if err != nil {
		return fmt.Errorf("couldn't get address's UTXOs: %s", err)
	}

	assetIDs := ids.Set{}                    // IDs of assets the address has a non-zero balance of
	balances := make(map[[32]byte]uint64, 0) // key: ID (as bytes). value: balance of that asset
	for _, utxo := range utxos {
		transferable, ok := utxo.Out.(avax.TransferableOut)
		if !ok {
			continue
		}
		assetID := utxo.AssetID()
		assetIDs.Add(assetID)
		balance := balances[assetID.Key()] // 0 if key doesn't exist
		balance, err := safemath.Add64(transferable.Amount(), balance)
		if err != nil {
			balances[assetID.Key()] = math.MaxUint64
		} else {
			balances[assetID.Key()] = balance
		}
	}

	reply.Balances = make([]Balance, assetIDs.Len())
	for i, assetID := range assetIDs.List() {
		if alias, err := service.vm.PrimaryAlias(assetID); err == nil {
			reply.Balances[i] = Balance{
				AssetID: alias,
				Balance: json.Uint64(balances[assetID.Key()]),
			}
		} else {
			reply.Balances[i] = Balance{
				AssetID: assetID.String(),
				Balance: json.Uint64(balances[assetID.Key()]),
			}
		}
	}

	return nil
}

// CreateFixedCapAssetArgs are arguments for passing into CreateFixedCapAsset requests
type CreateFixedCapAssetArgs struct {
	api.UserPass
	Name           string    `json:"name"`
	Symbol         string    `json:"symbol"`
	Denomination   byte      `json:"denomination"`
	InitialHolders []*Holder `json:"initialHolders"`
}

// Holder describes how much an address owns of an asset
type Holder struct {
	Amount  json.Uint64 `json:"amount"`
	Address string      `json:"address"`
}

// CreateFixedCapAsset returns ID of the newly created asset
func (service *Service) CreateFixedCapAsset(r *http.Request, args *CreateFixedCapAssetArgs, reply *FormattedAssetID) error {
	service.vm.ctx.Log.Info("AVM: CreateFixedCapAsset called with name: %s symbol: %s number of holders: %d",
		args.Name,
		args.Symbol,
		len(args.InitialHolders),
	)

	if len(args.InitialHolders) == 0 {
		return errNoHolders
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, keys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: service.vm.txFee,
		},
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	if amountSpent := amountsSpent[avaxKey]; amountSpent > service.vm.txFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - service.vm.txFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	initialState := &InitialState{
		FxID: 0, // TODO: Should lookup secp256k1fx FxID
		Outs: make([]verify.State, 0, len(args.InitialHolders)),
	}
	for _, holder := range args.InitialHolders {
		address, err := service.vm.ParseAddress(holder.Address)
		if err != nil {
			return err
		}
		addr, err := ids.ToShortID(address)
		if err != nil {
			return err
		}
		initialState.Outs = append(initialState.Outs, &secp256k1fx.TransferOutput{
			Amt: uint64(holder.Amount),
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{addr},
			},
		})
	}
	initialState.Sort(service.vm.codec)

	tx := Tx{UnsignedTx: &CreateAssetTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Name:         args.Name,
		Symbol:       args.Symbol,
		Denomination: args.Denomination,
		States:       []*InitialState{initialState},
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	assetID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.AssetID = assetID
	return nil
}

// CreateVariableCapAssetArgs are arguments for passing into CreateVariableCapAsset requests
type CreateVariableCapAssetArgs struct {
	api.UserPass
	Name         string   `json:"name"`
	Symbol       string   `json:"symbol"`
	Denomination byte     `json:"denomination"`
	MinterSets   []Owners `json:"minterSets"`
}

// Owners describes who can perform an action
type Owners struct {
	Threshold json.Uint32 `json:"threshold"`
	Minters   []string    `json:"minters"`
}

// CreateVariableCapAsset returns ID of the newly created asset
func (service *Service) CreateVariableCapAsset(r *http.Request, args *CreateVariableCapAssetArgs, reply *FormattedAssetID) error {
	service.vm.ctx.Log.Info("AVM: CreateVariableCapAsset called with name: %s symbol: %s number of minters: %d",
		args.Name,
		args.Symbol,
		len(args.MinterSets),
	)

	if len(args.MinterSets) == 0 {
		return errNoMinters
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, keys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: service.vm.txFee,
		},
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	if amountSpent := amountsSpent[avaxKey]; amountSpent > service.vm.txFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - service.vm.txFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	initialState := &InitialState{
		FxID: 0, // TODO: Should lookup secp256k1fx FxID
		Outs: make([]verify.State, 0, len(args.MinterSets)),
	}
	for _, owner := range args.MinterSets {
		minter := &secp256k1fx.MintOutput{
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: uint32(owner.Threshold),
				Addrs:     make([]ids.ShortID, 0, len(owner.Minters)),
			},
		}
		for _, address := range owner.Minters {
			addrBytes, err := service.vm.ParseAddress(address)
			if err != nil {
				return err
			}
			addr, err := ids.ToShortID(addrBytes)
			if err != nil {
				return err
			}
			minter.Addrs = append(minter.Addrs, addr)
		}
		ids.SortShortIDs(minter.Addrs)
		initialState.Outs = append(initialState.Outs, minter)
	}
	initialState.Sort(service.vm.codec)

	tx := Tx{UnsignedTx: &CreateAssetTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Name:         args.Name,
		Symbol:       args.Symbol,
		Denomination: args.Denomination,
		States:       []*InitialState{initialState},
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	assetID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.AssetID = assetID
	return nil
}

// CreateNFTAssetArgs are arguments for passing into CreateNFTAsset requests
type CreateNFTAssetArgs struct {
	api.UserPass
	Name       string   `json:"name"`
	Symbol     string   `json:"symbol"`
	MinterSets []Owners `json:"minterSets"`
}

// CreateNFTAsset returns ID of the newly created asset
func (service *Service) CreateNFTAsset(r *http.Request, args *CreateNFTAssetArgs, reply *FormattedAssetID) error {
	service.vm.ctx.Log.Info("AVM: CreateNFTAsset called with name: %s symbol: %s number of minters: %d",
		args.Name,
		args.Symbol,
		len(args.MinterSets),
	)

	if len(args.MinterSets) == 0 {
		return errNoMinters
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, keys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: service.vm.txFee,
		},
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	if amountSpent := amountsSpent[avaxKey]; amountSpent > service.vm.txFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - service.vm.txFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	initialState := &InitialState{
		FxID: 1, // TODO: Should lookup nftfx FxID
		Outs: make([]verify.State, 0, len(args.MinterSets)),
	}
	for i, owner := range args.MinterSets {
		minter := &nftfx.MintOutput{
			GroupID: uint32(i),
			OutputOwners: secp256k1fx.OutputOwners{
				Threshold: uint32(owner.Threshold),
			},
		}
		for _, address := range owner.Minters {
			addrBytes, err := service.vm.ParseAddress(address)
			if err != nil {
				return err
			}
			addr, err := ids.ToShortID(addrBytes)
			if err != nil {
				return err
			}
			minter.Addrs = append(minter.Addrs, addr)
		}
		ids.SortShortIDs(minter.Addrs)
		initialState.Outs = append(initialState.Outs, minter)
	}
	initialState.Sort(service.vm.codec)

	tx := Tx{UnsignedTx: &CreateAssetTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Name:         args.Name,
		Symbol:       args.Symbol,
		Denomination: 0, // NFTs are non-fungible
		States:       []*InitialState{initialState},
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	assetID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.AssetID = assetID
	return nil
}

// CreateAddress creates an address for the user [args.Username]
func (service *Service) CreateAddress(r *http.Request, args *api.UserPass, reply *api.JsonAddress) error {
	service.vm.ctx.Log.Info("AVM: CreateAddress called for user '%s'", args.Username)

	db, err := service.vm.ctx.Keystore.GetDatabase(args.Username, args.Password)
	if err != nil {
		return fmt.Errorf("problem retrieving user '%s': %w", args.Username, err)
	}

	user := userState{vm: service.vm}

	factory := crypto.FactorySECP256K1R{}
	skIntf, err := factory.NewPrivateKey()
	if err != nil {
		return fmt.Errorf("problem generating private key: %w", err)
	}
	sk := skIntf.(*crypto.PrivateKeySECP256K1R)

	if err := user.SetKey(db, sk); err != nil {
		return fmt.Errorf("problem saving private key: %w", err)
	}

	addresses, _ := user.Addresses(db)
	addresses = append(addresses, sk.PublicKey().Address())

	if err := user.SetAddresses(db, addresses); err != nil {
		return fmt.Errorf("problem saving address: %w", err)
	}
	reply.Address, err = service.vm.FormatAddress(sk.PublicKey().Address().Bytes())
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}
	return nil
}

// ListAddresses returns all of the addresses controlled by user [args.Username]
func (service *Service) ListAddresses(_ *http.Request, args *api.UserPass, response *api.JsonAddresses) error {
	service.vm.ctx.Log.Info("AVM: ListAddresses called for user '%s'", args.Username)

	db, err := service.vm.ctx.Keystore.GetDatabase(args.Username, args.Password)
	if err != nil {
		return fmt.Errorf("problem retrieving user '%s': %w", args.Username, err)
	}

	response.Addresses = []string{}

	user := userState{vm: service.vm}
	addresses, err := user.Addresses(db)
	if err != nil {
		return nil
	}

	for _, address := range addresses {
		addr, err := service.vm.FormatAddress(address.Bytes())
		if err != nil {
			return fmt.Errorf("problem formatting address: %w", err)
		}
		response.Addresses = append(response.Addresses, addr)
	}
	return nil
}

// ExportKeyArgs are arguments for ExportKey
type ExportKeyArgs struct {
	api.UserPass
	Address string `json:"address"`
}

// ExportKeyReply is the response for ExportKey
type ExportKeyReply struct {
	// The decrypted PrivateKey for the Address provided in the arguments
	PrivateKey string `json:"privateKey"`
}

// ExportKey returns a private key from the provided user
func (service *Service) ExportKey(r *http.Request, args *ExportKeyArgs, reply *ExportKeyReply) error {
	service.vm.ctx.Log.Info("AVM: ExportKey called for user '%s'", args.Username)

	address, err := service.vm.ParseAddress(args.Address)
	if err != nil {
		return fmt.Errorf("problem parsing address '%s': %w", args.Address, err)
	}
	addr, err := ids.ToShortID(address)
	if err != nil {
		return fmt.Errorf("problem parsing address '%s': %w", args.Address, err)
	}

	db, err := service.vm.ctx.Keystore.GetDatabase(args.Username, args.Password)
	if err != nil {
		return fmt.Errorf("problem retrieving user '%s': %w", args.Username, err)
	}

	user := userState{vm: service.vm}

	sk, err := user.Key(db, addr)
	if err != nil {
		return fmt.Errorf("problem retrieving private key: %w", err)
	}

	reply.PrivateKey = constants.SecretKeyPrefix + formatting.CB58{Bytes: sk.Bytes()}.String()
	return nil
}

// ImportKeyArgs are arguments for ImportKey
type ImportKeyArgs struct {
	api.UserPass
	PrivateKey string `json:"privateKey"`
}

// ImportKeyReply is the response for ImportKey
type ImportKeyReply struct {
	// The address controlled by the PrivateKey provided in the arguments
	Address string `json:"address"`
}

// ImportKey adds a private key to the provided user
func (service *Service) ImportKey(r *http.Request, args *ImportKeyArgs, reply *api.JsonAddress) error {
	service.vm.ctx.Log.Info("AVM: ImportKey called for user '%s'", args.Username)

	db, err := service.vm.ctx.Keystore.GetDatabase(args.Username, args.Password)
	if err != nil {
		return fmt.Errorf("problem retrieving data: %w", err)
	}

	user := userState{vm: service.vm}

	if !strings.HasPrefix(args.PrivateKey, constants.SecretKeyPrefix) {
		return fmt.Errorf("private key missing %s prefix", constants.SecretKeyPrefix)
	}
	trimmedPrivateKey := strings.TrimPrefix(args.PrivateKey, constants.SecretKeyPrefix)
	formattedPrivateKey := formatting.CB58{}
	if err := formattedPrivateKey.FromString(trimmedPrivateKey); err != nil {
		return fmt.Errorf("problem parsing private key: %w", err)
	}

	factory := crypto.FactorySECP256K1R{}
	skIntf, err := factory.ToPrivateKey(formattedPrivateKey.Bytes)
	if err != nil {
		return fmt.Errorf("problem parsing private key: %w", err)
	}
	sk := skIntf.(*crypto.PrivateKeySECP256K1R)

	if err := user.SetKey(db, sk); err != nil {
		return fmt.Errorf("problem saving key %w", err)
	}

	addresses, _ := user.Addresses(db)

	newAddress := sk.PublicKey().Address()
	reply.Address, err = service.vm.FormatAddress(newAddress.Bytes())
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}
	for _, address := range addresses {
		if newAddress.Equals(address) {
			return nil
		}
	}

	addresses = append(addresses, newAddress)
	if err := user.SetAddresses(db, addresses); err != nil {
		return fmt.Errorf("problem saving addresses: %w", err)
	}

	return nil
}

// SendArgs are arguments for passing into Send requests
type SendArgs struct {
	api.UserPass
	Amount  json.Uint64 `json:"amount"`
	AssetID string      `json:"assetID"`
	To      string      `json:"to"`
}

// Send returns the ID of the newly created transaction
func (service *Service) Send(r *http.Request, args *SendArgs, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: Send called with username: %s", args.Username)

	if args.Amount == 0 {
		return errInvalidAmount
	}

	assetID, err := service.vm.Lookup(args.AssetID)
	if err != nil {
		assetID, err = ids.FromString(args.AssetID)
		if err != nil {
			return fmt.Errorf("asset '%s' not found", args.AssetID)
		}
	}

	toBytes, err := service.vm.ParseAddress(args.To)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}
	to, err := ids.ToShortID(toBytes)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	amounts := map[[32]byte]uint64{
		assetID.Key(): uint64(args.Amount),
	}
	amountsWithFee := make(map[[32]byte]uint64, len(amounts)+1)
	for k, v := range amounts {
		amountsWithFee[k] = v
	}

	avaxKey := service.vm.avax.Key()
	amountWithFee, err := safemath.Add64(amountsWithFee[avaxKey], service.vm.txFee)
	if err != nil {
		return fmt.Errorf("problem calculating required spend amount: %w", err)
	}
	amountsWithFee[avaxKey] = amountWithFee

	amountsSpent, ins, keys, err := service.vm.Spend(
		utxos,
		kc,
		amountsWithFee,
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	for asset, amountWithFee := range amountsWithFee {
		assetID := ids.NewID(asset)
		amount := amounts[asset]
		amountSpent := amountsSpent[asset]

		if amount > 0 {
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: assetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: amount,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{to},
					},
				},
			})
		}
		if amountSpent > amountWithFee {
			changeAddr := kc.Keys[0].PublicKey().Address()
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: assetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: amountSpent - amountWithFee,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{changeAddr},
					},
				},
			})
		}
	}
	avax.SortTransferableOutputs(outs, service.vm.codec)

	tx := Tx{UnsignedTx: &BaseTx{
		NetID: service.vm.ctx.NetworkID,
		BCID:  service.vm.ctx.ChainID,
		Outs:  outs,
		Ins:   ins,
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	txID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.TxID = txID
	return nil
}

// MintArgs are arguments for passing into Mint requests
type MintArgs struct {
	api.UserPass
	Amount  json.Uint64 `json:"amount"`
	AssetID string      `json:"assetID"`
	To      string      `json:"to"`
}

// Mint issues a transaction that mints more of the asset
func (service *Service) Mint(r *http.Request, args *MintArgs, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: Mint called with username: %s", args.Username)

	if args.Amount == 0 {
		return errInvalidMintAmount
	}

	assetID, err := service.vm.Lookup(args.AssetID)
	if err != nil {
		assetID, err = ids.FromString(args.AssetID)
		if err != nil {
			return fmt.Errorf("asset '%s' not found", args.AssetID)
		}
	}

	toBytes, err := service.vm.ParseAddress(args.To)
	if err != nil {
		return fmt.Errorf("problem parsing to address '%s': %w", args.To, err)
	}
	to, err := ids.ToShortID(toBytes)
	if err != nil {
		return fmt.Errorf("problem parsing to address '%s': %w", args.To, err)
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, keys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: service.vm.txFee,
		},
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	if amountSpent := amountsSpent[avaxKey]; amountSpent > service.vm.txFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - service.vm.txFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	ops, opKeys, err := service.vm.Mint(
		utxos,
		kc,
		map[[32]byte]uint64{
			assetID.Key(): uint64(args.Amount),
		},
		to,
	)
	if err != nil {
		return err
	}
	keys = append(keys, opKeys...)

	tx := Tx{UnsignedTx: &OperationTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Ops: ops,
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	txID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.TxID = txID
	return nil
}

// SendNFTArgs are arguments for passing into SendNFT requests
type SendNFTArgs struct {
	api.UserPass
	AssetID string      `json:"assetID"`
	GroupID json.Uint32 `json:"groupID"`
	To      string      `json:"to"`
}

// SendNFT sends an NFT
func (service *Service) SendNFT(r *http.Request, args *SendNFTArgs, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: SendNFT called with username: %s", args.Username)

	assetID, err := service.vm.Lookup(args.AssetID)
	if err != nil {
		assetID, err = ids.FromString(args.AssetID)
		if err != nil {
			return fmt.Errorf("asset '%s' not found", args.AssetID)
		}
	}

	toBytes, err := service.vm.ParseAddress(args.To)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}
	to, err := ids.ToShortID(toBytes)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, secpKeys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: service.vm.txFee,
		},
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	if amountSpent := amountsSpent[avaxKey]; amountSpent > service.vm.txFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - service.vm.txFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	ops, nftKeys, err := service.vm.SpendNFT(
		utxos,
		kc,
		assetID,
		uint32(args.GroupID),
		to,
	)
	if err != nil {
		return err
	}

	tx := Tx{UnsignedTx: &OperationTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Ops: ops,
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, secpKeys); err != nil {
		return err
	}
	if err := service.vm.SignNFTFx(&tx, nftKeys); err != nil {
		return err
	}

	txID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.TxID = txID
	return nil
}

// MintNFTArgs are arguments for passing into MintNFT requests
type MintNFTArgs struct {
	api.UserPass
	AssetID string          `json:"assetID"`
	Payload formatting.CB58 `json:"payload"`
	To      string          `json:"to"`
}

// MintNFT issues a MintNFT transaction and returns the ID of the newly created transaction
func (service *Service) MintNFT(r *http.Request, args *MintNFTArgs, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: MintNFT called with username: %s", args.Username)

	assetID, err := service.vm.Lookup(args.AssetID)
	if err != nil {
		assetID, err = ids.FromString(args.AssetID)
		if err != nil {
			return fmt.Errorf("asset '%s' not found", args.AssetID)
		}
	}

	toBytes, err := service.vm.ParseAddress(args.To)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}
	to, err := ids.ToShortID(toBytes)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, secpKeys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: service.vm.txFee,
		},
	)
	if err != nil {
		return err
	}

	outs := []*avax.TransferableOutput{}
	if amountSpent := amountsSpent[avaxKey]; amountSpent > service.vm.txFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - service.vm.txFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	ops, nftKeys, err := service.vm.MintNFT(
		utxos,
		kc,
		assetID,
		args.Payload.Bytes,
		to,
	)
	if err != nil {
		return err
	}

	tx := Tx{UnsignedTx: &OperationTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Ops: ops,
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, secpKeys); err != nil {
		return err
	}
	if err := service.vm.SignNFTFx(&tx, nftKeys); err != nil {
		return err
	}

	txID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.TxID = txID
	return nil
}

// ImportAVAXArgs are arguments for passing into ImportAVAX requests
type ImportAVAXArgs struct {
	// User that controls To
	api.UserPass

	// Address receiving the imported $AVAX
	To string `json:"to"`
}

// ImportAVAX imports AVAX to this chain from the P-Chain.
// The AVAX must have already been exported from the P-Chain.
// Returns the ID of the newly created atomic transaction
func (service *Service) ImportAVAX(_ *http.Request, args *ImportAVAXArgs, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: ImportAVAX called with username: %s", args.Username)

	toBytes, err := service.vm.ParseAddress(args.To)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}
	to, err := ids.ToShortID(toBytes)
	if err != nil {
		return fmt.Errorf("problem parsing to address %q: %w", args.To, err)
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	addrs := ids.Set{}
	for _, addr := range kc.Addrs.List() {
		addrs.Add(ids.NewID(hashing.ComputeHash256Array(addr.Bytes())))
	}

	atomicUtxos, err := service.vm.GetAtomicUTXOs(addrs)
	if err != nil {
		return fmt.Errorf("problem retrieving user's atomic UTXOs: %w", err)
	}

	amountsSpent, importInputs, importKeys, err := service.vm.SpendAll(atomicUtxos, kc)
	if err != nil {
		return err
	}

	ins := []*avax.TransferableInput{}
	keys := [][]*crypto.PrivateKeySECP256K1R{}

	avaxKey := service.vm.avax.Key()
	if amountSpent := amountsSpent[avaxKey]; amountSpent < service.vm.txFee {
		var localAmountsSpent map[[32]byte]uint64
		localAmountsSpent, ins, keys, err = service.vm.Spend(
			utxos,
			kc,
			map[[32]byte]uint64{
				avaxKey: service.vm.txFee - amountSpent,
			},
		)
		if err != nil {
			return err
		}
		for asset, amount := range localAmountsSpent {
			newAmount, err := safemath.Add64(amountsSpent[asset], amount)
			if err != nil {
				return fmt.Errorf("problem calculating required spend amount: %w", err)
			}
			amountsSpent[asset] = newAmount
		}

		// Because we ensured that we had enough inputs for the fee, we can
		// safely just remove it without concern for underflow.
		amountsSpent[avaxKey] -= service.vm.txFee
	}
	keys = append(keys, importKeys...)

	outs := []*avax.TransferableOutput{}
	for asset, amount := range amountsSpent {
		assetID := ids.NewID(asset)
		if amount > 0 {
			outs = append(outs, &avax.TransferableOutput{
				Asset: avax.Asset{ID: assetID},
				Out: &secp256k1fx.TransferOutput{
					Amt: amount,
					OutputOwners: secp256k1fx.OutputOwners{
						Locktime:  0,
						Threshold: 1,
						Addrs:     []ids.ShortID{to},
					},
				},
			})
		}
	}
	avax.SortTransferableOutputs(outs, service.vm.codec)

	tx := Tx{UnsignedTx: &ImportTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Ins: importInputs,
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	txID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.TxID = txID
	return nil
}

// ExportAVAXArgs are arguments for passing into ExportAVA requests
type ExportAVAXArgs struct {
	api.UserPass // User providing exported AVAX

	// Amount of nAVAX to send
	Amount json.Uint64 `json:"amount"`

	// Address of P-Chain account that will receive the AVAX
	To string `json:"to"`
}

// ExportAVAX sends AVAX from this chain to the P-Chain.
// After this tx is accepted, the AVAX must be imported to the P-chain with an importTx.
// Returns the ID of the newly created atomic transaction
func (service *Service) ExportAVAX(_ *http.Request, args *ExportAVAXArgs, reply *api.JsonTxID) error {
	service.vm.ctx.Log.Info("AVM: ExportAVAX called with username: %s", args.Username)
	pchainID := service.vm.platform
	chainPrefixes := []string{pchainID.String()}
	if alias, err := service.vm.ctx.BCLookup.PrimaryAlias(pchainID); err == nil {
		chainPrefixes = append(chainPrefixes, alias)
	}

	ToBytes, err := formatting.ParseAddress(args.To, chainPrefixes, addressSep, service.vm.GetHRP())
	if err != nil {
		return err
	}

	ToID, err := ids.ToShortID(ToBytes)
	if err != nil {
		return err
	}

	if args.Amount == 0 {
		return errInvalidAmount
	}

	utxos, kc, err := service.vm.LoadUser(args.Username, args.Password)
	if err != nil {
		return err
	}

	amountWithFee, err := safemath.Add64(uint64(args.Amount), service.vm.txFee)
	if err != nil {
		return fmt.Errorf("problem calculating required spend amount: %w", err)
	}

	avaxKey := service.vm.avax.Key()
	amountsSpent, ins, keys, err := service.vm.Spend(
		utxos,
		kc,
		map[[32]byte]uint64{
			avaxKey: amountWithFee,
		},
	)
	if err != nil {
		return err
	}

	amountSpent := amountsSpent[avaxKey]

	exportOuts := []*avax.TransferableOutput{{
		Asset: avax.Asset{ID: service.vm.avax},
		Out: &secp256k1fx.TransferOutput{
			Amt: uint64(args.Amount),
			OutputOwners: secp256k1fx.OutputOwners{
				Locktime:  0,
				Threshold: 1,
				Addrs:     []ids.ShortID{ToID},
			},
		},
	}}

	outs := []*avax.TransferableOutput{}
	if amountSpent > amountWithFee {
		changeAddr := kc.Keys[0].PublicKey().Address()
		outs = append(outs, &avax.TransferableOutput{
			Asset: avax.Asset{ID: service.vm.avax},
			Out: &secp256k1fx.TransferOutput{
				Amt: amountSpent - amountWithFee,
				OutputOwners: secp256k1fx.OutputOwners{
					Locktime:  0,
					Threshold: 1,
					Addrs:     []ids.ShortID{changeAddr},
				},
			},
		})
	}

	tx := Tx{UnsignedTx: &ExportTx{
		BaseTx: BaseTx{
			NetID: service.vm.ctx.NetworkID,
			BCID:  service.vm.ctx.ChainID,
			Outs:  outs,
			Ins:   ins,
		},
		Outs: exportOuts,
	}}
	if err := service.vm.SignSECP256K1Fx(&tx, keys); err != nil {
		return err
	}

	txID, err := service.vm.IssueTx(tx.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("problem issuing transaction: %w", err)
	}

	reply.TxID = txID
	return nil
}
