// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/DioneProtocol/odysseygo/database"
	"github.com/DioneProtocol/odysseygo/ids"
	"github.com/DioneProtocol/odysseygo/snow"
	"github.com/DioneProtocol/odysseygo/utils"
	"github.com/DioneProtocol/odysseygo/utils/constants"
	"github.com/DioneProtocol/odysseygo/utils/timer/mockable"
	"github.com/DioneProtocol/odysseygo/vms/components/dione"
	"github.com/DioneProtocol/odysseygo/vms/components/verify"
	"github.com/DioneProtocol/odysseygo/vms/platformvm/config"
	"github.com/DioneProtocol/odysseygo/vms/platformvm/state"
	"github.com/DioneProtocol/odysseygo/vms/platformvm/txs"
	"github.com/DioneProtocol/odysseygo/vms/platformvm/utxo"
	"github.com/DioneProtocol/odysseygo/vms/secp256k1fx"
)

func TestVerifyAddPermissionlessValidatorTx(t *testing.T) {
	type test struct {
		name        string
		backendF    func(*gomock.Controller) *Backend
		stateF      func(*gomock.Controller) state.Chain
		sTxF        func() *txs.Tx
		txF         func() *txs.AddPermissionlessValidatorTx
		expectedErr error
	}

	var (
		subnetID            = ids.GenerateTestID()
		customAssetID       = ids.GenerateTestID()
		unsignedTransformTx = &txs.TransformSubnetTx{
			AssetID:           customAssetID,
			MinValidatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  3,
			MaxStakeDuration:  4,
		}
		transformTx = txs.Tx{
			Unsigned: unsignedTransformTx,
			Creds:    []verify.Verifiable{},
		}
		// This tx already passed syntactic verification.
		verifiedTx = txs.AddPermissionlessValidatorTx{
			BaseTx: txs.BaseTx{
				SyntacticallyVerified: true,
				BaseTx: dione.BaseTx{
					NetworkID:    1,
					BlockchainID: ids.GenerateTestID(),
					Outs:         []*dione.TransferableOutput{},
					Ins:          []*dione.TransferableInput{},
				},
			},
			Validator: txs.Validator{
				NodeID: ids.GenerateTestNodeID(),
				Start:  1,
				End:    1 + uint64(unsignedTransformTx.MinStakeDuration),
				Wght:   unsignedTransformTx.MinValidatorStake,
			},
			Subnet: subnetID,
			StakeOuts: []*dione.TransferableOutput{
				{
					Asset: dione.Asset{
						ID: customAssetID,
					},
				},
			},
			ValidatorRewardsOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
				Threshold: 1,
			},
		}
		verifiedSignedTx = txs.Tx{
			Unsigned: &verifiedTx,
			Creds:    []verify.Verifiable{},
		}
	)
	verifiedSignedTx.SetBytes([]byte{1}, []byte{2})

	tests := []test{
		{
			name: "fail syntactic verification",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
				}
			},
			stateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return nil
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return nil
			},
			expectedErr: txs.ErrNilSignedTx,
		},
		{
			name: "not bootstrapped",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: &utils.Atomic[bool]{},
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return nil
			},
			expectedErr: nil,
		},
		{
			name: "start time too early",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(verifiedTx.StartTime())
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrTimestampNotBeforeStartTime,
		},
		{
			name: "weight too low",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MinValidatorStake - 1
				return &tx
			},
			expectedErr: ErrWeightTooSmall,
		},
		{
			name: "weight too high",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake + 1
				return &tx
			},
			expectedErr: ErrWeightTooLarge,
		},
		{
			name: "duration too short",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				// Note the duration is 1 less than the minimum
				tx.Validator.Start = 1
				tx.Validator.End = uint64(unsignedTransformTx.MinStakeDuration)
				return &tx
			},
			expectedErr: ErrStakeTooShort,
		},
		{
			name: "duration too long",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				// Note the duration is more than the maximum
				tx.Validator.Start = 1
				tx.Validator.End = 2 + uint64(unsignedTransformTx.MaxStakeDuration)
				return &tx
			},
			expectedErr: ErrStakeTooLong,
		},
		{
			name: "wrong assetID",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.StakeOuts = []*dione.TransferableOutput{
					{
						Asset: dione.Asset{
							ID: ids.GenerateTestID(),
						},
					},
				}
				return &tx
			},
			expectedErr: ErrWrongStakedAssetID,
		},
		{
			name: "duplicate validator",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				// State says validator exists
				state.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrDuplicateValidator,
		},
		{
			name: "validator not subset of primary network validator",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				// Validator time isn't subset of primary network validator time
				primaryNetworkVdr := &state.Staker{
					StartTime: verifiedTx.StartTime().Add(time.Second),
					EndTime:   verifiedTx.EndTime(),
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrValidatorSubset,
		},
		{
			name: "flow check fails",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(ErrFlowCheckFailed)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSubnetValidatorFee: 1,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime: verifiedTx.StartTime(),
					EndTime:   verifiedTx.EndTime(),
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrFlowCheckFailed,
		},
		{
			name: "starts too far in the future",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSubnetValidatorFee: 1,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime: time.Unix(0, 0),
					EndTime:   mockable.MaxTime,
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				// Note this copies [verifiedTx]
				tx := verifiedTx
				tx.Validator.Start = uint64(MaxFutureStartTime.Seconds()) + 1
				tx.Validator.End = tx.Validator.Start + uint64(unsignedTransformTx.MinStakeDuration)
				return &tx
			},
			expectedErr: ErrFutureStakeTime,
		},
		{
			name: "success",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSubnetValidatorFee: 1,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime: time.Unix(0, 0),
					EndTime:   mockable.MaxTime,
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var (
				backend = tt.backendF(ctrl)
				state   = tt.stateF(ctrl)
				sTx     = tt.sTxF()
				tx      = tt.txF()
			)

			err := verifyAddPermissionlessValidatorTx(backend, state, sTx, tx)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestGetValidatorRules(t *testing.T) {
	type test struct {
		name          string
		subnetID      ids.ID
		backend       *Backend
		chainStateF   func(*gomock.Controller) state.Chain
		expectedRules *addValidatorRules
		expectedErr   error
	}

	var (
		config = &config.Config{
			MinValidatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  time.Second,
			MaxStakeDuration:  2 * time.Second,
		}
		dioneAssetID   = ids.GenerateTestID()
		customAssetID = ids.GenerateTestID()
		subnetID      = ids.GenerateTestID()
	)

	tests := []test{
		{
			name:     "primary network",
			subnetID: constants.PrimaryNetworkID,
			backend: &Backend{
				Config: config,
				Ctx: &snow.Context{
					DIONEAssetID: dioneAssetID,
				},
			},
			chainStateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			expectedRules: &addValidatorRules{
				assetID:           dioneAssetID,
				minValidatorStake: config.MinValidatorStake,
				maxValidatorStake: config.MaxValidatorStake,
				minStakeDuration:  config.MinStakeDuration,
				maxStakeDuration:  config.MaxStakeDuration,
			},
		},
		{
			name:     "can't get subnet transformation",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetSubnetTransformation(subnetID).Return(nil, errTest)
				return state
			},
			expectedRules: &addValidatorRules{},
			expectedErr:   errTest,
		},
		{
			name:     "subnet",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.TransformSubnetTx{
						AssetID:           customAssetID,
						MinValidatorStake: config.MinValidatorStake,
						MaxValidatorStake: config.MaxValidatorStake,
						MinStakeDuration:  1337,
						MaxStakeDuration:  42,
					},
				}
				state.EXPECT().GetSubnetTransformation(subnetID).Return(tx, nil)
				return state
			},
			expectedRules: &addValidatorRules{
				assetID:           customAssetID,
				minValidatorStake: config.MinValidatorStake,
				maxValidatorStake: config.MaxValidatorStake,
				minStakeDuration:  time.Duration(1337) * time.Second,
				maxStakeDuration:  time.Duration(42) * time.Second,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			chainState := tt.chainStateF(ctrl)
			rules, err := getValidatorRules(tt.backend, chainState, tt.subnetID)
			if tt.expectedErr != nil {
				require.ErrorIs(err, tt.expectedErr)
				return
			}
			require.NoError(err)
			require.Equal(tt.expectedRules, rules)
		})
	}
}
