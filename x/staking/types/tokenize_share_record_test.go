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

func TestParseShareTokenDenom_Valid(t *testing.T) {
	valAddr := newValAddress(t, 7)

	testCases := []struct {
		name string
		id   uint64
	}{
		{"id 1", 1},
		{"id 1000", 1000},
		{"id max-ish", 18446744073709551614}, // max uint64 - 1
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := types.TokenizeShareRecord{
				Id:        tc.id,
				Validator: valAddr.String(),
			}

			denom := original.GetShareTokenDenom()
			parsed, err := types.ParseShareTokenDenom(denom)
			require.NoError(t, err)

			// validator field must round-trip; bech32 normalizes case
			require.Equal(t, strings.ToLower(original.Validator), strings.ToLower(parsed.Validator))

			// module account must contain prefix and the original ID
			require.Equal(t,
				fmt.Sprintf("%s%d", types.TokenizeShareModuleAccountPrefix, tc.id),
				parsed.ModuleAccount,
			)
		})
	}
}

func TestParseShareTokenDenom_Invalid(t *testing.T) {
	valAddr := newValAddress(t, 9)

	testCases := []struct {
		name        string
		denom       string
		errContains string
	}{
		{
			name:        "empty",
			denom:       "",
			errContains: "wrong number of segments",
		},
		{
			name:        "no separator",
			denom:       "foobar",
			errContains: "wrong number of segments",
		},
		{
			name:        "three segments",
			denom:       "a/b/c",
			errContains: "wrong number of segments",
		},
		{
			name:        "invalid validator bech32",
			denom:       "notavalidator/1",
			errContains: "failed to parse val address part",
		},
		{
			name:        "negative recordID",
			denom:       fmt.Sprintf("%s/-1", strings.ToLower(valAddr.String())),
			errContains: "failed to parse recordId part",
		},
		{
			name:        "non-numeric recordID",
			denom:       fmt.Sprintf("%s/abc", strings.ToLower(valAddr.String())),
			errContains: "failed to parse recordId part",
		},
		{
			name:        "empty recordID",
			denom:       fmt.Sprintf("%s/", strings.ToLower(valAddr.String())),
			errContains: "failed to parse recordId part",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := types.ParseShareTokenDenom(tc.denom)
			require.Error(t, err, "expected error for denom %q", tc.denom)
			require.Contains(t, err.Error(), tc.errContains)
		})
	}
}
