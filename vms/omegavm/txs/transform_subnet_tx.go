// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"fmt"

	"github.com/DioneProtocol/odysseygo/ids"
	"github.com/DioneProtocol/odysseygo/snow"
	"github.com/DioneProtocol/odysseygo/utils/constants"
	"github.com/DioneProtocol/odysseygo/vms/components/verify"
	"github.com/DioneProtocol/odysseygo/vms/omegavm/reward"
)

var (
	_ UnsignedTx = (*TransformSubnetTx)(nil)

	errCantTransformPrimaryNetwork       = errors.New("cannot transform primary network")
	errEmptyAssetID                      = errors.New("empty asset ID is not valid")
	errAssetIDCantBeDIONE                = errors.New("asset ID can't be DIONE")
	errInitialSupplyZero                 = errors.New("initial supply must be non-0")
	errInitialSupplyGreaterThanMaxSupply = errors.New("initial supply can't be greater than maximum supply")
	errMinConsumptionRateTooLarge        = errors.New("min consumption rate must be less than or equal to max consumption rate")
	errMaxConsumptionRateTooLarge        = fmt.Errorf("max consumption rate must be less than or equal to %d", reward.PercentDenominator)
	errMinValidatorStakeZero             = errors.New("min validator stake must be non-0")
	errMinValidatorStakeAboveSupply      = errors.New("min validator stake must be less than or equal to initial supply")
	errMinStakeDurationZero              = errors.New("min stake duration must be non-0")
	errMinStakeDurationTooLarge          = errors.New("min stake duration must be less than or equal to max stake duration")
	errMaxValidatorWeightFactorZero      = errors.New("max validator weight factor must be non-0")
	errUptimeRequirementTooLarge         = fmt.Errorf("uptime requirement must be less than or equal to %d", reward.PercentDenominator)
)

// TransformSubnetTx is an unsigned transformSubnetTx
type TransformSubnetTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`
	// ID of the Subnet to transform
	// Restrictions:
	// - Must not be the Primary Network ID
	Subnet ids.ID `serialize:"true" json:"subnetID"`
	// Asset to use when staking on the Subnet
	// Restrictions:
	// - Must not be the Empty ID
	// - Must not be the DIONE ID
	AssetID ids.ID `serialize:"true" json:"assetID"`
	// Amount to initially specify as the current supply
	// Restrictions:
	// - Must be > 0
	InitialSupply uint64 `serialize:"true" json:"initialSupply"`
	// Amount to specify as the maximum token supply
	// Restrictions:
	// - Must be >= [InitialSupply]
	MaximumSupply uint64 `serialize:"true" json:"maximumSupply"`
	// MinConsumptionRate is the rate to allocate funds if the validator's stake
	// duration is 0
	MinConsumptionRate uint64 `serialize:"true" json:"minConsumptionRate"`
	// MaxConsumptionRate is the rate to allocate funds if the validator's stake
	// duration is equal to the minting period
	// Restrictions:
	// - Must be >= [MinConsumptionRate]
	// - Must be <= [reward.PercentDenominator]
	MaxConsumptionRate uint64 `serialize:"true" json:"maxConsumptionRate"`
	// MinValidatorStake is the minimum amount of funds required to become a
	// validator.
	// Restrictions:
	// - Must be > 0
	// - Must be <= [InitialSupply]
	MinValidatorStake uint64 `serialize:"true" json:"minValidatorStake"`
	// MinStakeDuration is the minimum number of seconds a staker can stake for.
	// Restrictions:
	// - Must be > 0
	MinStakeDuration uint32 `serialize:"true" json:"minStakeDuration"`
	// MaxStakeDuration is the maximum number of seconds a staker can stake for.
	// Restrictions:
	// - Must be >= [MinStakeDuration]
	// - Must be <= [GlobalMaxStakeDuration]
	MaxStakeDuration uint32 `serialize:"true" json:"maxStakeDuration"`
	// MaxValidatorWeightFactor is the factor which calculates the maximum
	// amount a validator can receive.
	// Restrictions:
	// - Must be > 0
	MaxValidatorWeightFactor byte `serialize:"true" json:"maxValidatorWeightFactor"`
	// UptimeRequirement is the minimum percentage a validator must be online
	// and responsive to receive a reward.
	// Restrictions:
	// - Must be <= [reward.PercentDenominator]
	UptimeRequirement uint32 `serialize:"true" json:"uptimeRequirement"`
	// Authorizes this transformation
	SubnetAuth verify.Verifiable `serialize:"true" json:"subnetAuthorization"`
}

func (tx *TransformSubnetTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.Subnet == constants.PrimaryNetworkID:
		return errCantTransformPrimaryNetwork
	case tx.AssetID == ids.Empty:
		return errEmptyAssetID
	case tx.AssetID == ctx.DIONEAssetID:
		return errAssetIDCantBeDIONE
	case tx.InitialSupply == 0:
		return errInitialSupplyZero
	case tx.InitialSupply > tx.MaximumSupply:
		return errInitialSupplyGreaterThanMaxSupply
	case tx.MinConsumptionRate > tx.MaxConsumptionRate:
		return errMinConsumptionRateTooLarge
	case tx.MaxConsumptionRate > reward.PercentDenominator:
		return errMaxConsumptionRateTooLarge
	case tx.MinValidatorStake == 0:
		return errMinValidatorStakeZero
	case tx.MinValidatorStake > tx.InitialSupply:
		return errMinValidatorStakeAboveSupply
	case tx.MinStakeDuration == 0:
		return errMinStakeDurationZero
	case tx.MinStakeDuration > tx.MaxStakeDuration:
		return errMinStakeDurationTooLarge
	case tx.MaxValidatorWeightFactor == 0:
		return errMaxValidatorWeightFactorZero
	case tx.UptimeRequirement > reward.PercentDenominator:
		return errUptimeRequirementTooLarge
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := tx.SubnetAuth.Verify(); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *TransformSubnetTx) Visit(visitor Visitor) error {
	return visitor.TransformSubnetTx(tx)
}