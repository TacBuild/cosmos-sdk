package keeper_test

import (
	"fmt"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// makeRecord builds a deterministic TokenizeShareRecord for tests.
// id is also used to derive owner/validator addresses so each record is unique.
func makeRecord(id uint64) types.TokenizeShareRecord {
	// 20-byte addresses with id encoded into the bytes so each record is unique.
	owner := sdk.AccAddress(fmt.Sprintf("o%019d", id))
	valoper := sdk.ValAddress(fmt.Sprintf("v%019d", id))
	return types.TokenizeShareRecord{
		Id:            id,
		Owner:         owner.String(),
		ModuleAccount: fmt.Sprintf("%s%d", types.TokenizeShareModuleAccountPrefix, id),
		Validator:     valoper.String(),
	}
}

// TestGetSetLastTokenizeShareRecordID verifies round-trip of the last-record-ID counter
// and that an unset counter reads back as zero.
func (s *KeeperTestSuite) TestGetSetLastTokenizeShareRecordID() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	// fresh store: must be 0
	require.Equal(uint64(0), keeper.GetLastTokenizeShareRecordID(ctx))

	keeper.SetLastTokenizeShareRecordID(ctx, 42)
	require.Equal(uint64(42), keeper.GetLastTokenizeShareRecordID(ctx))

	keeper.SetLastTokenizeShareRecordID(ctx, 1<<40)
	require.Equal(uint64(1<<40), keeper.GetLastTokenizeShareRecordID(ctx))
}

// TestAddTokenizeShareRecord verifies that AddTokenizeShareRecord populates all three
// indexes (by ID, by owner, by denom) and that the record is retrievable through each.
func (s *KeeperTestSuite) TestAddTokenizeShareRecord() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	rec := makeRecord(1)
	require.NoError(keeper.AddTokenizeShareRecord(ctx, rec))

	// by ID
	got, err := keeper.GetTokenizeShareRecord(ctx, rec.Id)
	require.NoError(err)
	require.Equal(rec, got)

	// by denom
	gotByDenom, err := keeper.GetTokenizeShareRecordByDenom(ctx, rec.GetShareTokenDenom())
	require.NoError(err)
	require.Equal(rec, gotByDenom)

	// by owner
	owner, err := sdk.AccAddressFromBech32(rec.Owner)
	require.NoError(err)
	byOwner := keeper.GetTokenizeShareRecordsByOwner(ctx, owner)
	require.Len(byOwner, 1)
	require.Equal(rec, byOwner[0])

	// in the global iterator
	all := keeper.GetAllTokenizeShareRecords(ctx)
	require.Len(all, 1)
	require.Equal(rec, all[0])
}

// TestAddTokenizeShareRecord_Multiple verifies multiple records co-exist and
// owner-keyed lookup returns only that owner's records.
func (s *KeeperTestSuite) TestAddTokenizeShareRecord_Multiple() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	rec1 := makeRecord(1)
	rec2 := makeRecord(2)
	rec3SameOwnerAsRec1 := makeRecord(3)
	rec3SameOwnerAsRec1.Owner = rec1.Owner // share owner with rec1

	require.NoError(keeper.AddTokenizeShareRecord(ctx, rec1))
	require.NoError(keeper.AddTokenizeShareRecord(ctx, rec2))
	require.NoError(keeper.AddTokenizeShareRecord(ctx, rec3SameOwnerAsRec1))

	all := keeper.GetAllTokenizeShareRecords(ctx)
	require.Len(all, 3)

	owner1, err := sdk.AccAddressFromBech32(rec1.Owner)
	require.NoError(err)
	byOwner1 := keeper.GetTokenizeShareRecordsByOwner(ctx, owner1)
	require.Len(byOwner1, 2, "rec1 and rec3 share owner1")

	owner2, err := sdk.AccAddressFromBech32(rec2.Owner)
	require.NoError(err)
	byOwner2 := keeper.GetTokenizeShareRecordsByOwner(ctx, owner2)
	require.Len(byOwner2, 1)
	require.Equal(rec2, byOwner2[0])
}

// TestAddTokenizeShareRecord_Duplicate verifies adding a record with an existing ID
// returns ErrTokenizeShareRecordAlreadyExists.
func (s *KeeperTestSuite) TestAddTokenizeShareRecord_Duplicate() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	rec := makeRecord(7)
	require.NoError(keeper.AddTokenizeShareRecord(ctx, rec))

	err := keeper.AddTokenizeShareRecord(ctx, rec)
	require.Error(err)
	require.ErrorIs(err, types.ErrTokenizeShareRecordAlreadyExists)
}

// TestGetTokenizeShareRecord_NotFound verifies a missing ID returns ErrTokenizeShareRecordNotExists.
func (s *KeeperTestSuite) TestGetTokenizeShareRecord_NotFound() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	_, err := keeper.GetTokenizeShareRecord(ctx, 999)
	require.Error(err)
	require.ErrorIs(err, types.ErrTokenizeShareRecordNotExists)
}

// TestDeleteTokenizeShareRecord verifies that all three indexes are cleared on delete.
func (s *KeeperTestSuite) TestDeleteTokenizeShareRecord() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	rec := makeRecord(11)
	require.NoError(keeper.AddTokenizeShareRecord(ctx, rec))

	require.NoError(keeper.DeleteTokenizeShareRecord(ctx, rec.Id))

	// by ID -> not found
	_, err := keeper.GetTokenizeShareRecord(ctx, rec.Id)
	require.ErrorIs(err, types.ErrTokenizeShareRecordNotExists)

	// by denom -> error
	_, err = keeper.GetTokenizeShareRecordByDenom(ctx, rec.GetShareTokenDenom())
	require.Error(err)

	// by owner -> empty
	owner, err := sdk.AccAddressFromBech32(rec.Owner)
	require.NoError(err)
	require.Empty(keeper.GetTokenizeShareRecordsByOwner(ctx, owner))

	// global iterator -> empty
	require.Empty(keeper.GetAllTokenizeShareRecords(ctx))
}

// TestDeleteTokenizeShareRecord_NotFound verifies deleting a missing ID errors out
// (because the underlying GetTokenizeShareRecord call fails).
func (s *KeeperTestSuite) TestDeleteTokenizeShareRecord_NotFound() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	err := keeper.DeleteTokenizeShareRecord(ctx, 12345)
	require.Error(err)
	require.ErrorIs(err, types.ErrTokenizeShareRecordNotExists)
}

// TestSetGetTotalLiquidStakedTokens covers Set/Get round-trip plus the
// fresh-store-returns-zero contract.
func (s *KeeperTestSuite) TestSetGetTotalLiquidStakedTokens() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	// fresh store
	require.True(keeper.GetTotalLiquidStakedTokens(ctx).IsZero())

	keeper.SetTotalLiquidStakedTokens(ctx, math.NewInt(1_000_000))
	require.Equal(math.NewInt(1_000_000), keeper.GetTotalLiquidStakedTokens(ctx))

	// overwrite
	keeper.SetTotalLiquidStakedTokens(ctx, math.NewInt(42))
	require.Equal(math.NewInt(42), keeper.GetTotalLiquidStakedTokens(ctx))
}

// TestDecreaseTotalLiquidStakedTokens_Underflow verifies the underflow guard:
// trying to decrease below zero returns ErrTotalLiquidStakedUnderflow and the
// stored value is left untouched.
func (s *KeeperTestSuite) TestDecreaseTotalLiquidStakedTokens_Underflow() {
	ctx, keeper := s.ctx, s.stakingKeeper
	require := s.Require()

	keeper.SetTotalLiquidStakedTokens(ctx, math.NewInt(100))

	err := keeper.DecreaseTotalLiquidStakedTokens(ctx, math.NewInt(101))
	require.ErrorIs(err, types.ErrTotalLiquidStakedUnderflow)
	require.Equal(math.NewInt(100), keeper.GetTotalLiquidStakedTokens(ctx),
		"stored value must remain unchanged after a failed decrease")

	// exact decrease to zero is fine
	require.NoError(keeper.DecreaseTotalLiquidStakedTokens(ctx, math.NewInt(100)))
	require.True(keeper.GetTotalLiquidStakedTokens(ctx).IsZero())
}

// TestDelegatorIsLiquidStaker verifies the 32-byte address heuristic used to
// classify a delegator as a liquid-staking provider (ICA or LSM module account).
func (s *KeeperTestSuite) TestDelegatorIsLiquidStaker() {
	keeper := s.stakingKeeper
	require := s.Require()

	addr20 := sdk.AccAddress(make([]byte, 20))
	addr32 := sdk.AccAddress(make([]byte, 32))

	require.False(keeper.DelegatorIsLiquidStaker(addr20))
	require.True(keeper.DelegatorIsLiquidStaker(addr32))

	// a real LSM module account address must satisfy the predicate
	rec := makeRecord(1)
	require.True(keeper.DelegatorIsLiquidStaker(rec.GetModuleAddress()))
}
