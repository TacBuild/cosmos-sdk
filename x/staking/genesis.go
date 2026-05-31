package staking

import (
	"fmt"

	cmttypes "github.com/cometbft/cometbft/types"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// WriteValidators returns a slice of bonded genesis validators.
func WriteValidators(ctx sdk.Context, keeper *keeper.Keeper) (vals []cmttypes.GenesisValidator, returnErr error) {
	err := keeper.IterateLastValidators(ctx, func(_ int64, validator types.ValidatorI) (stop bool) {
		pk, err := validator.ConsPubKey()
		if err != nil {
			returnErr = err
			return true
		}
		cmtPk, err := cryptocodec.ToCmtPubKeyInterface(pk)
		if err != nil {
			returnErr = err
			return true
		}

		vals = append(vals, cmttypes.GenesisValidator{
			Address: sdk.ConsAddress(cmtPk.Address()).Bytes(),
			PubKey:  cmtPk,
			Power:   validator.GetConsensusPower(keeper.PowerReduction(ctx)),
			Name:    validator.GetMoniker(),
		})

		return false
	})
	if err != nil {
		return nil, err
	}

	return vals, returnErr
}

// ValidateGenesis validates the provided staking genesis state to ensure the
// expected invariants holds. (i.e. params in correct bounds, no duplicate validators)
func ValidateGenesis(data *types.GenesisState) error {
	if err := validateGenesisStateValidators(data.Validators); err != nil {
		return err
	}

	if err := validateGenesisStateLSM(data); err != nil {
		return err
	}

	return data.Params.Validate()
}

func validateGenesisStateValidators(validators []types.Validator) error {
	addrMap := make(map[string]bool, len(validators))

	for i := range validators {
		val := validators[i]
		consPk, err := val.ConsPubKey()
		if err != nil {
			return err
		}

		strKey := string(consPk.Bytes())

		if _, ok := addrMap[strKey]; ok {
			consAddr, err := val.GetConsAddr()
			if err != nil {
				return err
			}
			return fmt.Errorf("duplicate validator in genesis state: moniker %v, address %v", val.Description.Moniker, consAddr)
		}

		if val.Jailed && val.IsBonded() {
			consAddr, err := val.GetConsAddr()
			if err != nil {
				return err
			}
			return fmt.Errorf("validator is bonded and jailed in genesis state: moniker %v, address %v", val.Description.Moniker, consAddr)
		}

		if val.DelegatorShares.IsZero() && !val.IsUnbonding() {
			return fmt.Errorf("bonded/unbonded genesis validator cannot have zero delegator shares, validator: %v", val)
		}

		addrMap[strKey] = true
	}

	return nil
}

func validateGenesisStateLSM(data *types.GenesisState) error {
	recordIDs := make(map[uint64]struct{}, len(data.TokenizeShareRecords))
	recordDenoms := make(map[string]struct{}, len(data.TokenizeShareRecords))
	maxRecordID := uint64(0)

	for _, record := range data.TokenizeShareRecords {
		if _, ok := recordIDs[record.Id]; ok {
			return fmt.Errorf("duplicate tokenize share record id in genesis state: %d", record.Id)
		}
		recordIDs[record.Id] = struct{}{}
		if record.Id > maxRecordID {
			maxRecordID = record.Id
		}

		if _, err := sdk.AccAddressFromBech32(record.Owner); err != nil {
			return fmt.Errorf("invalid tokenize share record owner %q: %w", record.Owner, err)
		}
		if _, err := sdk.ValAddressFromBech32(record.Validator); err != nil {
			return fmt.Errorf("invalid tokenize share record validator %q: %w", record.Validator, err)
		}

		expectedModuleAccount := fmt.Sprintf("%s%d", types.TokenizeShareModuleAccountPrefix, record.Id)
		if record.ModuleAccount != expectedModuleAccount {
			return fmt.Errorf(
				"invalid tokenize share record module account for id %d: got %q, expected %q",
				record.Id,
				record.ModuleAccount,
				expectedModuleAccount,
			)
		}

		denom := record.GetShareTokenDenom()
		if _, ok := recordDenoms[denom]; ok {
			return fmt.Errorf("duplicate tokenize share record denom in genesis state: %s", denom)
		}
		recordDenoms[denom] = struct{}{}
	}

	if data.LastTokenizeShareRecordId < maxRecordID {
		return fmt.Errorf(
			"last tokenize share record id %d is lower than max tokenize share record id %d",
			data.LastTokenizeShareRecordId,
			maxRecordID,
		)
	}

	if !data.TotalLiquidStakedTokens.IsNil() && data.TotalLiquidStakedTokens.IsNegative() {
		return fmt.Errorf("total liquid staked tokens cannot be negative: %s", data.TotalLiquidStakedTokens)
	}

	lockAddresses := make(map[string]struct{}, len(data.TokenizeShareLocks))
	for _, lock := range data.TokenizeShareLocks {
		address, err := sdk.AccAddressFromBech32(lock.Address)
		if err != nil {
			return fmt.Errorf("invalid tokenize share lock address %q: %w", lock.Address, err)
		}

		addressKey := address.String()
		if _, ok := lockAddresses[addressKey]; ok {
			return fmt.Errorf("duplicate tokenize share lock address in genesis state: %s", addressKey)
		}
		lockAddresses[addressKey] = struct{}{}

		switch lock.Status {
		case types.ShareLockStatusLocked.String():
			if !lock.CompletionTime.IsZero() {
				return fmt.Errorf("locked tokenize share lock %s cannot have completion time", addressKey)
			}
		case types.ShareLockStatusLockExpiring.String():
			if lock.CompletionTime.IsZero() {
				return fmt.Errorf("expiring tokenize share lock %s must have completion time", addressKey)
			}
		default:
			return fmt.Errorf("invalid tokenize share lock status: %s", lock.Status)
		}
	}

	return nil
}
