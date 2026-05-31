package keeper_test

import (
	"time"

	sdkmath "cosmossdk.io/math"
	"go.uber.org/mock/gomock"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// tests Jail, Unjail
func (s *KeeperTestSuite) TestRevocation() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	valAddr := sdk.ValAddress(PKs[0].Address().Bytes())
	consAddr := sdk.ConsAddress(PKs[0].Address())
	validator := testutil.NewValidator(s.T(), valAddr, PKs[0])

	// initial state
	require.NoError(keeper.SetValidator(ctx, validator))
	require.NoError(keeper.SetValidatorByConsAddr(ctx, validator))
	val, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.False(val.IsJailed())

	// test jail
	require.NoError(keeper.Jail(ctx, consAddr))
	val, err = keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(val.IsJailed())

	// test unjail
	require.NoError(keeper.Unjail(ctx, consAddr))
	val, err = keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.False(val.IsJailed())
}

// tests Slash at a future height (must error)
func (s *KeeperTestSuite) TestSlashAtFutureHeight() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	consAddr := sdk.ConsAddress(PKs[0].Address())
	validator := testutil.NewValidator(s.T(), sdk.ValAddress(PKs[0].Address().Bytes()), PKs[0])
	require.NoError(keeper.SetValidator(ctx, validator))
	require.NoError(keeper.SetValidatorByConsAddr(ctx, validator))

	fraction := sdkmath.LegacyNewDecWithPrec(5, 1)
	_, err := keeper.Slash(ctx, consAddr, 1, 10, fraction)
	require.Error(err)
}

func (s *KeeperTestSuite) TestSlashDecreasesTotalLiquidStakedTokens() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	power := int64(1)
	tokens := keeper.TokensFromConsensusPower(ctx, power)
	shares := sdkmath.LegacyNewDecFromInt(tokens)
	liquidTokens := tokens.QuoRaw(2)
	liquidShares := sdkmath.LegacyNewDecFromInt(liquidTokens)

	valAddr := sdk.ValAddress(PKs[0].Address().Bytes())
	consAddr := sdk.ConsAddress(PKs[0].Address())
	validator := testutil.NewValidator(s.T(), valAddr, PKs[0])
	validator.Status = stakingtypes.Bonded
	validator = s.setValidatorState(validator, tokens, shares, liquidShares, sdkmath.LegacyZeroDec())
	require.NoError(keeper.SetValidatorByConsAddr(ctx, validator))
	keeper.SetTotalLiquidStakedTokens(ctx, liquidTokens)

	slashedTokens := tokens.QuoRaw(2)
	s.bankKeeper.EXPECT().
		BurnCoins(gomock.Any(), stakingtypes.BondedPoolName, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, slashedTokens))).
		Return(nil)

	burned, err := keeper.Slash(ctx, consAddr, ctx.BlockHeight(), power, sdkmath.LegacyNewDecWithPrec(5, 1))
	require.NoError(err)
	require.Equal(slashedTokens, burned)
	require.Equal(liquidTokens.QuoRaw(2), keeper.GetTotalLiquidStakedTokens(ctx))
}

func (s *KeeperTestSuite) TestSlashRedelegationLiquidStakerDecreasesLiquidAccounting() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	_, valAddrs := createValAddrs(2)
	srcValAddr := valAddrs[0]
	dstValAddr := valAddrs[1]
	lspAddr := liquidStakerAddress(4)

	shares := sdkmath.LegacyNewDec(100)
	dstValidator := testutil.NewValidator(s.T(), dstValAddr, PKs[1])
	s.setValidatorState(dstValidator, sdkmath.NewInt(100), shares, shares, sdkmath.LegacyZeroDec())

	require.NoError(keeper.SetDelegation(ctx, stakingtypes.NewDelegation(
		lspAddr.String(),
		dstValAddr.String(),
		shares,
	)))
	keeper.SetTotalLiquidStakedTokens(ctx, sdkmath.NewInt(100))

	redelegation := stakingtypes.NewRedelegation(
		lspAddr,
		srcValAddr,
		dstValAddr,
		ctx.BlockHeight(),
		ctx.BlockTime().Add(time.Hour),
		sdkmath.NewInt(100),
		shares,
		0,
		keeper.ValidatorAddressCodec(),
		s.accountKeeper.AddressCodec(),
	)

	s.bankKeeper.EXPECT().
		BurnCoins(gomock.Any(), stakingtypes.NotBondedPoolName, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(50)))).
		Return(nil)

	totalSlashed, err := keeper.SlashRedelegation(
		ctx,
		stakingtypes.Validator{},
		redelegation,
		ctx.BlockHeight(),
		sdkmath.LegacyNewDecWithPrec(5, 1),
	)
	require.NoError(err)
	require.Equal(sdkmath.NewInt(50), totalSlashed)
	require.Equal(sdkmath.NewInt(50), keeper.GetTotalLiquidStakedTokens(ctx))

	storedDst, err := keeper.GetValidator(ctx, dstValAddr)
	require.NoError(err)
	require.True(storedDst.LiquidShares.Equal(sdkmath.LegacyNewDec(50)))
}

func (s *KeeperTestSuite) TestSlashRedelegationValidatorBondDecreasesBondShares() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	delAddrs, valAddrs := createValAddrs(3)
	delegatorAddr := delAddrs[2]
	srcValAddr := valAddrs[0]
	dstValAddr := valAddrs[1]

	shares := sdkmath.LegacyNewDec(100)
	dstValidator := testutil.NewValidator(s.T(), dstValAddr, PKs[1])
	s.setValidatorState(dstValidator, sdkmath.NewInt(100), shares, sdkmath.LegacyZeroDec(), shares)

	delegation := stakingtypes.NewDelegation(delegatorAddr.String(), dstValAddr.String(), shares)
	delegation.ValidatorBond = true
	require.NoError(keeper.SetDelegation(ctx, delegation))

	redelegation := stakingtypes.NewRedelegation(
		delegatorAddr,
		srcValAddr,
		dstValAddr,
		ctx.BlockHeight(),
		ctx.BlockTime().Add(time.Hour),
		sdkmath.NewInt(100),
		shares,
		0,
		keeper.ValidatorAddressCodec(),
		s.accountKeeper.AddressCodec(),
	)

	s.bankKeeper.EXPECT().
		BurnCoins(gomock.Any(), stakingtypes.NotBondedPoolName, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(50)))).
		Return(nil)

	totalSlashed, err := keeper.SlashRedelegation(
		ctx,
		stakingtypes.Validator{},
		redelegation,
		ctx.BlockHeight(),
		sdkmath.LegacyNewDecWithPrec(5, 1),
	)
	require.NoError(err)
	require.Equal(sdkmath.NewInt(50), totalSlashed)

	storedDst, err := keeper.GetValidator(ctx, dstValAddr)
	require.NoError(err)
	require.True(storedDst.ValidatorBondShares.Equal(sdkmath.LegacyNewDec(50)))
}
