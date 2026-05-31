package keeper_test

import (
	"time"

	"go.uber.org/mock/gomock"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *KeeperTestSuite) TestInitGenesisRestoresLSMState() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	s.accountKeeper.EXPECT().
		GetModuleAccount(gomock.Any(), stakingtypes.BondedPoolName).
		Return(bondedAcc).
		AnyTimes()
	s.accountKeeper.EXPECT().
		GetModuleAccount(gomock.Any(), stakingtypes.NotBondedPoolName).
		Return(notBondedAcc).
		AnyTimes()
	s.accountKeeper.EXPECT().
		SetModuleAccount(gomock.Any(), gomock.Any()).
		AnyTimes()
	s.bankKeeper.EXPECT().
		GetAllBalances(gomock.Any(), bondedAcc.GetAddress()).
		Return(sdk.NewCoins()).
		AnyTimes()
	s.bankKeeper.EXPECT().
		GetAllBalances(gomock.Any(), notBondedAcc.GetAddress()).
		Return(sdk.NewCoins()).
		AnyTimes()

	record1 := makeRecord(1)
	record2 := makeRecord(2)
	record2.Owner = record1.Owner

	lockedAddr := sdk.AccAddress([]byte("locked-address-00001")).String()
	expiringAddr := sdk.AccAddress([]byte("expiring-address000"))
	completionTime := time.Unix(1_715_000_000, 0).UTC()

	genesis := &stakingtypes.GenesisState{
		Params:                    stakingtypes.DefaultParams(),
		LastTotalPower:            math.ZeroInt(),
		Exported:                  true,
		TokenizeShareRecords:      []stakingtypes.TokenizeShareRecord{record1, record2},
		LastTokenizeShareRecordId: record2.Id,
		TotalLiquidStakedTokens:   math.NewInt(123456789),
		TokenizeShareLocks: []stakingtypes.TokenizeShareLock{
			{
				Address: lockedAddr,
				Status:  stakingtypes.ShareLockStatusLocked.String(),
			},
			{
				Address:        expiringAddr.String(),
				Status:         stakingtypes.ShareLockStatusLockExpiring.String(),
				CompletionTime: completionTime,
			},
		},
	}

	keeper.InitGenesis(ctx, genesis)

	got1, err := keeper.GetTokenizeShareRecord(ctx, record1.Id)
	require.NoError(err)
	require.Equal(record1, got1)

	got2, err := keeper.GetTokenizeShareRecord(ctx, record2.Id)
	require.NoError(err)
	require.Equal(record2, got2)

	owner1, err := sdk.AccAddressFromBech32(record1.Owner)
	require.NoError(err)
	require.Len(keeper.GetTokenizeShareRecordsByOwner(ctx, owner1), 2)

	require.Equal(record2.Id, keeper.GetLastTokenizeShareRecordID(ctx))
	require.Equal(math.NewInt(123456789), keeper.GetTotalLiquidStakedTokens(ctx))

	lockedStatus, lockedTime := keeper.GetTokenizeSharesLock(ctx, sdk.MustAccAddressFromBech32(lockedAddr))
	require.Equal(stakingtypes.ShareLockStatusLocked, lockedStatus)
	require.True(lockedTime.IsZero())

	expiringStatus, expiringTime := keeper.GetTokenizeSharesLock(ctx, expiringAddr)
	require.Equal(stakingtypes.ShareLockStatusLockExpiring, expiringStatus)
	require.True(expiringTime.Equal(completionTime))

	pending := keeper.GetPendingTokenizeShareAuthorizations(ctx, completionTime)
	require.Equal([]string{expiringAddr.String()}, pending.Addresses)

	exported := keeper.ExportGenesis(ctx)
	require.Equal(genesis.TokenizeShareRecords, exported.TokenizeShareRecords)
	require.Equal(genesis.LastTokenizeShareRecordId, exported.LastTokenizeShareRecordId)
	require.Equal(genesis.TotalLiquidStakedTokens, exported.TotalLiquidStakedTokens)
	require.Len(exported.TokenizeShareLocks, 2)
}

func (s *KeeperTestSuite) TestInitGenesisPanicsWhenLastTokenizeShareRecordIDIsTooLow() {
	record := makeRecord(2)
	genesis := &stakingtypes.GenesisState{
		Params:                    stakingtypes.DefaultParams(),
		LastTotalPower:            math.ZeroInt(),
		Exported:                  true,
		TokenizeShareRecords:      []stakingtypes.TokenizeShareRecord{record},
		LastTokenizeShareRecordId: 1,
	}

	require := s.Require()
	require.PanicsWithValue(
		"Tokenize share record specified with ID greater than the latest ID",
		func() { s.stakingKeeper.InitGenesis(s.ctx, genesis) },
	)
}
