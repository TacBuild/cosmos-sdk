package keeper_test

import (
	"go.uber.org/mock/gomock"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// withBondedPoolBalance arranges the mocked bank keeper so that
// TotalBondedTokens(...) returns the given amount of bond-denom tokens for
// any number of calls in the test. It also wires the account-keeper mock so
// that GetBondedPool(ctx) (which is invoked under the hood) succeeds.
func (s *KeeperTestSuite) withBondedPoolBalance(amount math.Int) {
	bondDenom, err := s.stakingKeeper.BondDenom(s.ctx)
	s.Require().NoError(err)
	s.accountKeeper.EXPECT().
		GetModuleAccount(gomock.Any(), types.BondedPoolName).
		Return(bondedAcc).
		AnyTimes()
	s.bankKeeper.EXPECT().
		GetBalance(gomock.Any(), bondedAcc.GetAddress(), bondDenom).
		Return(sdk.NewCoin(bondDenom, amount)).
		AnyTimes()
}

// makeValidatorWithShares creates a validator and stores it with explicit
// DelegatorShares / LiquidShares / ValidatorBondShares so that cap math can be
// driven precisely without going through (un)delegation paths.
func (s *KeeperTestSuite) makeValidatorWithShares(
	idx int,
	delegatorShares, liquidShares, validatorBondShares math.LegacyDec,
) types.Validator {
	_, valAddrs := createValAddrs(idx + 1)
	val := testutil.NewValidator(s.T(), valAddrs[idx], PKs[idx])
	val.DelegatorShares = delegatorShares
	val.LiquidShares = liquidShares
	val.ValidatorBondShares = validatorBondShares
	val.Tokens = delegatorShares.TruncateInt()
	s.Require().NoError(s.stakingKeeper.SetValidator(s.ctx, val))
	return val
}

// withCaps overrides the LSM-related staking params for the current test.
func (s *KeeperTestSuite) withCaps(globalCap, valCap, valBondFactor math.LegacyDec) {
	params, err := s.stakingKeeper.GetParams(s.ctx)
	s.Require().NoError(err)
	params.GlobalLiquidStakingCap = globalCap
	params.ValidatorLiquidStakingCap = valCap
	params.ValidatorBondFactor = valBondFactor
	s.Require().NoError(s.stakingKeeper.SetParams(s.ctx, params))
}

// ---------------------------------------------------------------------------
// CheckExceedsValidatorBondCap (pure: validator + factor only)
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestCheckExceedsValidatorBondCap() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	// validator with 100 bond shares, 50 liquid shares already
	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000), // delegator shares (irrelevant here)
		math.LegacyNewDec(50),   // liquid shares
		math.LegacyNewDec(100),  // bond shares
	)

	// disabled factor (-1) -> never exceeded
	s.withCaps(math.LegacyOneDec(), math.LegacyOneDec(), types.ValidatorBondCapDisabled)
	require.False(keeper.CheckExceedsValidatorBondCap(ctx, val, math.LegacyNewDec(1_000_000)))

	// factor=2 -> max liquid shares = 200; 50 + 149 = 199 -> ok
	s.withCaps(math.LegacyOneDec(), math.LegacyOneDec(), math.LegacyNewDec(2))
	require.False(keeper.CheckExceedsValidatorBondCap(ctx, val, math.LegacyNewDec(149)))
	// 50 + 151 = 201 -> exceeded
	require.True(keeper.CheckExceedsValidatorBondCap(ctx, val, math.LegacyNewDec(151)))
}

// ---------------------------------------------------------------------------
// CheckExceedsValidatorLiquidStakingCap (pure)
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestCheckExceedsValidatorLiquidStakingCap() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	// 1000 delegator shares, 100 already liquid -> 10% liquid currently
	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000),
		math.LegacyNewDec(100),
		math.LegacyNewDec(0),
	)

	// cap=25%
	s.withCaps(math.LegacyOneDec(), math.LegacyNewDecWithPrec(25, 2), types.ValidatorBondCapDisabled)

	// not bonded yet: adding 100 -> liquid=200, total=1100, 200/1100 ≈ 18% < 25%
	require.False(keeper.CheckExceedsValidatorLiquidStakingCap(ctx, val, math.LegacyNewDec(100), false))

	// not bonded yet: adding 400 -> liquid=500, total=1400, 500/1400 ≈ 35.7% > 25%
	require.True(keeper.CheckExceedsValidatorLiquidStakingCap(ctx, val, math.LegacyNewDec(400), false))

	// already bonded (total unchanged): adding 200 -> liquid=300/1000 = 30% > 25%
	require.True(keeper.CheckExceedsValidatorLiquidStakingCap(ctx, val, math.LegacyNewDec(200), true))

	// already bonded: adding 100 -> liquid=200/1000 = 20% < 25%
	require.False(keeper.CheckExceedsValidatorLiquidStakingCap(ctx, val, math.LegacyNewDec(100), true))
}

// ---------------------------------------------------------------------------
// CheckExceedsGlobalLiquidStakingCap (uses bank.GetBalance for TotalBondedTokens)
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestCheckExceedsGlobalLiquidStakingCap() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	// 1000 bonded in pool, 100 already liquid-staked, cap=25%
	s.withBondedPoolBalance(math.NewInt(1000))
	keeper.SetTotalLiquidStakedTokens(ctx, math.NewInt(100))
	s.withCaps(math.LegacyNewDecWithPrec(25, 2), math.LegacyOneDec(), types.ValidatorBondCapDisabled)

	// not bonded yet: liquid=100+100=200, total=1000+100=1100, 200/1100≈18% < 25%
	require.False(keeper.CheckExceedsGlobalLiquidStakingCap(ctx, math.NewInt(100), false))

	// not bonded yet: liquid=600, total=1500, 600/1500=40% > 25%
	require.True(keeper.CheckExceedsGlobalLiquidStakingCap(ctx, math.NewInt(500), false))

	// already bonded (total unchanged): liquid=300/1000 = 30% > 25%
	require.True(keeper.CheckExceedsGlobalLiquidStakingCap(ctx, math.NewInt(200), true))

	// already bonded: liquid=200/1000 = 20% < 25%
	require.False(keeper.CheckExceedsGlobalLiquidStakingCap(ctx, math.NewInt(100), true))
}

// ---------------------------------------------------------------------------
// SafelyIncreaseTotalLiquidStakedTokens (errors + storage)
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestSafelyIncreaseTotalLiquidStakedTokens() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	s.withBondedPoolBalance(math.NewInt(1000))
	keeper.SetTotalLiquidStakedTokens(ctx, math.NewInt(0))
	// 25% cap
	s.withCaps(math.LegacyNewDecWithPrec(25, 2), math.LegacyOneDec(), types.ValidatorBondCapDisabled)

	// 100/1100 ≈ 9% -> ok
	require.NoError(keeper.SafelyIncreaseTotalLiquidStakedTokens(ctx, math.NewInt(100), false))
	require.Equal(math.NewInt(100), keeper.GetTotalLiquidStakedTokens(ctx))

	// 500 more would push to 600/1500 = 40% > 25%
	err := keeper.SafelyIncreaseTotalLiquidStakedTokens(ctx, math.NewInt(500), false)
	require.ErrorIs(err, types.ErrGlobalLiquidStakingCapExceeded)
	require.Equal(math.NewInt(100), keeper.GetTotalLiquidStakedTokens(ctx),
		"counter must not move on a rejected increase")
}

// ---------------------------------------------------------------------------
// SafelyIncreaseValidatorLiquidShares: bond cap + liquid cap + happy path
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestSafelyIncreaseValidatorLiquidShares_BondCapExceeded() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000),
		math.LegacyNewDec(50),
		math.LegacyNewDec(100), // bond shares
	)
	// factor=2 -> max liquid = 200; 50 + 200 = 250 > 200 -> bond cap exceeded
	s.withCaps(math.LegacyOneDec(), math.LegacyOneDec(), math.LegacyNewDec(2))

	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	s.Require().NoError(err)

	_, err = keeper.SafelyIncreaseValidatorLiquidShares(ctx, valAddr, math.LegacyNewDec(200), false)
	require.Error(err)
	require.ErrorIs(err, types.ErrInsufficientValidatorBondShares)

	stored, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.LiquidShares.Equal(math.LegacyNewDec(50)),
		"liquid shares must remain unchanged on rejection")
}

func (s *KeeperTestSuite) TestSafelyIncreaseValidatorLiquidShares_LiquidCapExceeded() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000),
		math.LegacyNewDec(100),
		math.LegacyNewDec(0),
	)
	// bond factor disabled, liquid cap=25%
	s.withCaps(math.LegacyOneDec(), math.LegacyNewDecWithPrec(25, 2), types.ValidatorBondCapDisabled)

	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	require.NoError(err)

	// already-bonded: liquid=400/1000=40% > 25%
	_, err = keeper.SafelyIncreaseValidatorLiquidShares(ctx, valAddr, math.LegacyNewDec(300), true)
	require.ErrorIs(err, types.ErrValidatorLiquidStakingCapExceeded)

	stored, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.LiquidShares.Equal(math.LegacyNewDec(100)))
}

func (s *KeeperTestSuite) TestSafelyIncreaseValidatorLiquidShares_Happy() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000),
		math.LegacyNewDec(100),
		math.LegacyNewDec(0),
	)
	s.withCaps(math.LegacyOneDec(), math.LegacyOneDec(), types.ValidatorBondCapDisabled)

	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	require.NoError(err)

	updated, err := keeper.SafelyIncreaseValidatorLiquidShares(ctx, valAddr, math.LegacyNewDec(50), false)
	require.NoError(err)
	require.True(updated.LiquidShares.Equal(math.LegacyNewDec(150)))

	stored, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.LiquidShares.Equal(math.LegacyNewDec(150)))
}

// ---------------------------------------------------------------------------
// DecreaseValidatorLiquidShares: underflow + happy
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestDecreaseValidatorLiquidShares() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000),
		math.LegacyNewDec(100),
		math.LegacyNewDec(0),
	)
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	require.NoError(err)

	// underflow
	_, err = keeper.DecreaseValidatorLiquidShares(ctx, valAddr, math.LegacyNewDec(101))
	require.ErrorIs(err, types.ErrValidatorLiquidSharesUnderflow)

	stored, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.LiquidShares.Equal(math.LegacyNewDec(100)),
		"liquid shares must not change on underflow")

	// happy
	updated, err := keeper.DecreaseValidatorLiquidShares(ctx, valAddr, math.LegacyNewDec(40))
	require.NoError(err)
	require.True(updated.LiquidShares.Equal(math.LegacyNewDec(60)))
}

// ---------------------------------------------------------------------------
// IncreaseValidatorBondShares + SafelyDecreaseValidatorBond
// ---------------------------------------------------------------------------

func (s *KeeperTestSuite) TestValidatorBondShares() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	val := s.makeValidatorWithShares(0,
		math.LegacyNewDec(1000),
		math.LegacyNewDec(50),  // current liquid shares
		math.LegacyNewDec(100), // bond shares
	)
	valAddr, err := sdk.ValAddressFromBech32(val.GetOperator())
	require.NoError(err)

	// Increase: 100 -> 150
	require.NoError(keeper.IncreaseValidatorBondShares(ctx, valAddr, math.LegacyNewDec(50)))
	stored, err := keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.ValidatorBondShares.Equal(math.LegacyNewDec(150)))

	// SafelyDecrease with factor=2: max liquid after decrease = (150-100)*2 = 100; current liquid=50 -> ok
	s.withCaps(math.LegacyOneDec(), math.LegacyOneDec(), math.LegacyNewDec(2))
	require.NoError(keeper.SafelyDecreaseValidatorBond(ctx, valAddr, math.LegacyNewDec(100)))
	stored, err = keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.ValidatorBondShares.Equal(math.LegacyNewDec(50)))

	// Now bond=50, liquid=50, factor=2 -> after decrease by 1, max=(50-1)*2=98; 50<98 ok
	require.NoError(keeper.SafelyDecreaseValidatorBond(ctx, valAddr, math.LegacyNewDec(1)))

	// Decrease too much: bond=49, decrease by 25 -> after=(24)*2=48; liquid=50 > 48 -> reject
	err = keeper.SafelyDecreaseValidatorBond(ctx, valAddr, math.LegacyNewDec(25))
	require.ErrorIs(err, types.ErrInsufficientValidatorBondShares)

	stored, err = keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.ValidatorBondShares.Equal(math.LegacyNewDec(49)),
		"bond shares must not change on rejection")

	// Disabled factor: any decrease is fine
	s.withCaps(math.LegacyOneDec(), math.LegacyOneDec(), types.ValidatorBondCapDisabled)
	require.NoError(keeper.SafelyDecreaseValidatorBond(ctx, valAddr, math.LegacyNewDec(49)))
	stored, err = keeper.GetValidator(ctx, valAddr)
	require.NoError(err)
	require.True(stored.ValidatorBondShares.IsZero())
}
