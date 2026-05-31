package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestParamsEqual(t *testing.T) {
	p1 := types.DefaultParams()
	p2 := types.DefaultParams()

	ok := p1.Equal(p2)
	require.True(t, ok)

	p2.UnbondingTime = 60 * 60 * 24 * 2
	p2.BondDenom = "soup"

	ok = p1.Equal(p2)
	require.False(t, ok)
}

func TestValidateParams(t *testing.T) {
	params := types.DefaultParams()

	// default params have no error
	require.NoError(t, params.Validate())

	// validate mincommission
	params.MinCommissionRate = math.LegacyNewDec(-1)
	require.Error(t, params.Validate())

	params.MinCommissionRate = math.LegacyNewDec(2)
	require.Error(t, params.Validate())
}

func TestValidateLSMParams(t *testing.T) {
	testCases := []struct {
		name      string
		mutate    func(*types.Params)
		expErrMsg string
	}{
		{
			name: "disabled validator bond factor",
			mutate: func(params *types.Params) {
				params.ValidatorBondFactor = types.ValidatorBondCapDisabled
			},
		},
		{
			name: "zero validator bond factor",
			mutate: func(params *types.Params) {
				params.ValidatorBondFactor = math.LegacyZeroDec()
			},
		},
		{
			name: "negative validator bond factor",
			mutate: func(params *types.Params) {
				params.ValidatorBondFactor = math.LegacyNewDecWithPrec(-5, 1)
			},
			expErrMsg: "invalid validator bond factor",
		},
		{
			name: "global liquid staking cap negative",
			mutate: func(params *types.Params) {
				params.GlobalLiquidStakingCap = math.LegacyNewDecWithPrec(-1, 1)
			},
			expErrMsg: "global liquid staking cap cannot be negative",
		},
		{
			name: "global liquid staking cap over 100%",
			mutate: func(params *types.Params) {
				params.GlobalLiquidStakingCap = math.LegacyNewDecWithPrec(11, 1)
			},
			expErrMsg: "global liquid staking cap cannot be greater than 100%",
		},
		{
			name: "validator liquid staking cap negative",
			mutate: func(params *types.Params) {
				params.ValidatorLiquidStakingCap = math.LegacyNewDecWithPrec(-1, 1)
			},
			expErrMsg: "validator liquid staking cap cannot be negative",
		},
		{
			name: "validator liquid staking cap over 100%",
			mutate: func(params *types.Params) {
				params.ValidatorLiquidStakingCap = math.LegacyNewDecWithPrec(11, 1)
			},
			expErrMsg: "validator liquid staking cap cannot be greater than 100%",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.mutate(&params)

			err := params.Validate()
			if tc.expErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expErrMsg)
			}
		})
	}
}
