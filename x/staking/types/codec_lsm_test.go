package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestRegisterLegacyAminoCodecIncludesLSMMessages(t *testing.T) {
	cdc := codec.NewLegacyAmino()
	types.RegisterLegacyAminoCodec(cdc)

	tests := []struct {
		name      string
		msg       sdk.Msg
		aminoName string
	}{
		{
			name:      "validator bond",
			msg:       &types.MsgValidatorBond{DelegatorAddress: "cosmos1delegator", ValidatorAddress: "cosmosvaloper1validator"},
			aminoName: "cosmos-sdk/MsgValidatorBond",
		},
		{
			name:      "unbond validator",
			msg:       &types.MsgUnbondValidator{ValidatorAddress: "cosmosvaloper1validator"},
			aminoName: "cosmos-sdk/MsgUnbondValidator",
		},
		{
			name:      "tokenize shares",
			msg:       &types.MsgTokenizeShares{DelegatorAddress: "cosmos1delegator", ValidatorAddress: "cosmosvaloper1validator", Amount: sdk.NewCoin("stake", math.NewInt(1)), TokenizedShareOwner: "cosmos1owner"},
			aminoName: "cosmos-sdk/MsgTokenizeShares",
		},
		{
			name:      "redeem tokens for shares",
			msg:       &types.MsgRedeemTokensForShares{DelegatorAddress: "cosmos1delegator", Amount: sdk.NewCoin("stake", math.NewInt(1))},
			aminoName: "cosmos-sdk/MsgRedeemTokensForShares",
		},
		{
			name:      "transfer tokenize share record",
			msg:       &types.MsgTransferTokenizeShareRecord{TokenizeShareRecordId: 1, Sender: "cosmos1sender", NewOwner: "cosmos1owner"},
			aminoName: "cosmos-sdk/MsgTransferTokenizeRecord",
		},
		{
			name:      "disable tokenize shares",
			msg:       &types.MsgDisableTokenizeShares{DelegatorAddress: "cosmos1delegator"},
			aminoName: "cosmos-sdk/MsgDisableTokenizeShares",
		},
		{
			name:      "enable tokenize shares",
			msg:       &types.MsgEnableTokenizeShares{DelegatorAddress: "cosmos1delegator"},
			aminoName: "cosmos-sdk/MsgEnableTokenizeShares",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bz, err := cdc.MarshalJSON(tc.msg)
			require.NoError(t, err)
			require.Contains(t, string(bz), `"type":"`+tc.aminoName+`"`)
		})
	}
}
