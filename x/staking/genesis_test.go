package staking_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func testTokenizeShareRecord(t *testing.T, id uint64) types.TokenizeShareRecord {
	t.Helper()

	owner := sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address())
	valAddr := sdk.ValAddress(ed25519.GenPrivKey().PubKey().Address())

	return types.TokenizeShareRecord{
		Id:            id,
		Owner:         owner.String(),
		ModuleAccount: fmt.Sprintf("%s%d", types.TokenizeShareModuleAccountPrefix, id),
		Validator:     valAddr.String(),
	}
}

func testLockAddress() string {
	return sdk.AccAddress(ed25519.GenPrivKey().PubKey().Address()).String()
}

func TestValidateGenesis(t *testing.T) {
	genValidators1 := make([]types.Validator, 1, 5)
	pk := ed25519.GenPrivKey().PubKey()
	genValidators1[0] = testutil.NewValidator(t, sdk.ValAddress(pk.Address()), pk)
	genValidators1[0].Tokens = math.OneInt()
	genValidators1[0].DelegatorShares = math.LegacyOneDec()

	tests := []struct {
		name    string
		mutate  func(*types.GenesisState)
		wantErr bool
	}{
		{"default", func(*types.GenesisState) {}, false},
		// validate genesis validators
		{"duplicate validator", func(data *types.GenesisState) {
			data.Validators = genValidators1
			data.Validators = append(data.Validators, genValidators1[0])
		}, true},
		{"no delegator shares", func(data *types.GenesisState) {
			data.Validators = genValidators1
			data.Validators[0].DelegatorShares = math.LegacyZeroDec()
		}, true},
		{"jailed and bonded validator", func(data *types.GenesisState) {
			data.Validators = genValidators1
			data.Validators[0].Jailed = true
			data.Validators[0].Status = types.Bonded
		}, true},
		{"valid lsm state", func(data *types.GenesisState) {
			record := testTokenizeShareRecord(t, 1)
			data.TokenizeShareRecords = []types.TokenizeShareRecord{record}
			data.LastTokenizeShareRecordId = record.Id
			data.TotalLiquidStakedTokens = math.OneInt()
			data.TokenizeShareLocks = []types.TokenizeShareLock{
				{
					Address: testLockAddress(),
					Status:  types.ShareLockStatusLocked.String(),
				},
				{
					Address:        testLockAddress(),
					Status:         types.ShareLockStatusLockExpiring.String(),
					CompletionTime: time.Unix(1_715_000_000, 0).UTC(),
				},
			}
		}, false},
		{"invalid tokenize share record owner", func(data *types.GenesisState) {
			record := testTokenizeShareRecord(t, 1)
			record.Owner = "not-a-bech32-address"
			data.TokenizeShareRecords = []types.TokenizeShareRecord{record}
			data.LastTokenizeShareRecordId = record.Id
		}, true},
		{"duplicate tokenize share record id", func(data *types.GenesisState) {
			record1 := testTokenizeShareRecord(t, 1)
			record2 := testTokenizeShareRecord(t, 1)
			data.TokenizeShareRecords = []types.TokenizeShareRecord{record1, record2}
			data.LastTokenizeShareRecordId = record1.Id
		}, true},
		{"invalid tokenize share record validator", func(data *types.GenesisState) {
			record := testTokenizeShareRecord(t, 1)
			record.Validator = "not-a-validator-address"
			data.TokenizeShareRecords = []types.TokenizeShareRecord{record}
			data.LastTokenizeShareRecordId = record.Id
		}, true},
		{"invalid tokenize share record module account", func(data *types.GenesisState) {
			record := testTokenizeShareRecord(t, 1)
			record.ModuleAccount = "wrong-module-account"
			data.TokenizeShareRecords = []types.TokenizeShareRecord{record}
			data.LastTokenizeShareRecordId = record.Id
		}, true},
		{"last tokenize share record id below max record id", func(data *types.GenesisState) {
			record := testTokenizeShareRecord(t, 2)
			data.TokenizeShareRecords = []types.TokenizeShareRecord{record}
			data.LastTokenizeShareRecordId = 1
		}, true},
		{"negative total liquid staked tokens", func(data *types.GenesisState) {
			data.TotalLiquidStakedTokens = math.NewInt(-1)
		}, true},
		{"duplicate tokenize share lock address", func(data *types.GenesisState) {
			addr := testLockAddress()
			data.TokenizeShareLocks = []types.TokenizeShareLock{
				{Address: addr, Status: types.ShareLockStatusLocked.String()},
				{Address: addr, Status: types.ShareLockStatusLocked.String()},
			}
		}, true},
		{"lock expiring without completion time", func(data *types.GenesisState) {
			data.TokenizeShareLocks = []types.TokenizeShareLock{
				{Address: testLockAddress(), Status: types.ShareLockStatusLockExpiring.String()},
			}
		}, true},
		{"locked lock with completion time", func(data *types.GenesisState) {
			data.TokenizeShareLocks = []types.TokenizeShareLock{
				{
					Address:        testLockAddress(),
					Status:         types.ShareLockStatusLocked.String(),
					CompletionTime: time.Unix(1_715_000_000, 0).UTC(),
				},
			}
		}, true},
		{"invalid tokenize share lock status", func(data *types.GenesisState) {
			data.TokenizeShareLocks = []types.TokenizeShareLock{
				{Address: testLockAddress(), Status: "BROKEN"},
			}
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			genesisState := types.DefaultGenesisState()
			tt.mutate(genesisState)

			if tt.wantErr {
				assert.Error(t, staking.ValidateGenesis(genesisState))
			} else {
				assert.NoError(t, staking.ValidateGenesis(genesisState))
			}
		})
	}
}
