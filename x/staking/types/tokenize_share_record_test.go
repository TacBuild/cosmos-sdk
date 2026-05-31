package types_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// newValAddress generates a deterministic validator address for tests.
func newValAddress(t *testing.T, seed byte) sdk.ValAddress {
	t.Helper()
	priv := secp256k1.GenPrivKeyFromSecret([]byte{seed})
	return sdk.ValAddress(priv.PubKey().Address())
}

func TestTokenizeShareRecord_GetShareTokenDenom(t *testing.T) {
	valAddr := newValAddress(t, 1)
	record := types.TokenizeShareRecord{
		Id:        42,
		Validator: valAddr.String(),
	}

	denom := record.GetShareTokenDenom()

	// denom must be lowercased validator/recordID
	require.Equal(t, fmt.Sprintf("%s/42", strings.ToLower(valAddr.String())), denom)
	// denom must be lowercase only
	require.Equal(t, strings.ToLower(denom), denom)
}

func TestTokenizeShareRecord_GetModuleAddress(t *testing.T) {
	r1 := types.TokenizeShareRecord{
		Id:            1,
		ModuleAccount: fmt.Sprintf("%s%d", types.TokenizeShareModuleAccountPrefix, 1),
	}
	r2 := types.TokenizeShareRecord{
		Id:            2,
		ModuleAccount: fmt.Sprintf("%s%d", types.TokenizeShareModuleAccountPrefix, 2),
	}

	addr1a := r1.GetModuleAddress()
	addr1b := r1.GetModuleAddress()
	addr2 := r2.GetModuleAddress()

	// determinism
	require.Equal(t, addr1a, addr1b, "module address must be deterministic for the same record")

	// distinctness across record IDs
	require.NotEqual(t, addr1a, addr2, "module addresses must differ for different records")

	// 32-byte length is a load-bearing invariant: DelegatorIsLiquidStaker
	// and AccountIsLiquidStakingProvider rely on it to discriminate
	// LSM-owned delegations from regular accounts.
	require.Len(t, addr1a, 32, "tokenize-share module address must be 32 bytes")
	require.Len(t, addr2, 32, "tokenize-share module address must be 32 bytes")
}
