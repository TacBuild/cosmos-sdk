package types_test

import (
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	"cosmossdk.io/api/amino"
	cosmos_proto "github.com/cosmos/cosmos-proto"
	gogoproto "github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	googleproto "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	_ "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestLSMProtoAddressScalars(t *testing.T) {
	file := stakingTxFileDescriptorProto(t)

	tests := []struct {
		msg      protoreflect.Name
		field    protoreflect.Name
		expected string
	}{
		{"MsgUnbondValidator", "validator_address", "cosmos.ValidatorAddressString"},
		{"MsgTokenizeShares", "validator_address", "cosmos.ValidatorAddressString"},
		{"MsgTransferTokenizeShareRecord", "sender", "cosmos.AddressString"},
		{"MsgTransferTokenizeShareRecord", "new_owner", "cosmos.AddressString"},
		{"MsgValidatorBond", "delegator_address", "cosmos.AddressString"},
		{"MsgValidatorBond", "validator_address", "cosmos.ValidatorAddressString"},
	}

	for _, tc := range tests {
		t.Run(string(tc.msg)+"."+string(tc.field), func(t *testing.T) {
			field := findProtoField(t, file, tc.msg, tc.field)
			opts := field.GetOptions()
			require.NotNil(t, opts)
			require.True(t, googleproto.HasExtension(opts, cosmos_proto.E_Scalar))
			require.Equal(t, tc.expected, googleproto.GetExtension(opts, cosmos_proto.E_Scalar))
		})
	}
}

func TestLSMProtoAminoNames(t *testing.T) {
	file := stakingTxFileDescriptorProto(t)

	tests := []struct {
		msg      protoreflect.Name
		expected string
	}{
		{"MsgValidatorBond", "cosmos-sdk/MsgValidatorBond"},
		{"MsgUnbondValidator", "cosmos-sdk/MsgUnbondValidator"},
		{"MsgTokenizeShares", "cosmos-sdk/MsgTokenizeShares"},
		{"MsgRedeemTokensForShares", "cosmos-sdk/MsgRedeemTokensForShares"},
		{"MsgTransferTokenizeShareRecord", "cosmos-sdk/MsgTransferTokenizeRecord"},
		{"MsgDisableTokenizeShares", "cosmos-sdk/MsgDisableTokenizeShares"},
		{"MsgEnableTokenizeShares", "cosmos-sdk/MsgEnableTokenizeShares"},
	}

	for _, tc := range tests {
		t.Run(string(tc.msg), func(t *testing.T) {
			msg := findProtoMessage(t, file, tc.msg)
			opts := msg.GetOptions()
			require.NotNil(t, opts)
			require.True(t, googleproto.HasExtension(opts, amino.E_Name))
			require.Equal(t, tc.expected, googleproto.GetExtension(opts, amino.E_Name))
		})
	}
}

func stakingTxFileDescriptorProto(t *testing.T) *descriptorpb.FileDescriptorProto {
	t.Helper()

	compressed := gogoproto.FileDescriptor("cosmos/staking/v1beta1/tx.proto")
	require.NotNil(t, compressed)

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(t, err)
	defer reader.Close()

	bz, err := io.ReadAll(reader)
	require.NoError(t, err)

	fd := &descriptorpb.FileDescriptorProto{}
	require.NoError(t, googleproto.Unmarshal(bz, fd))

	return fd
}

func findProtoMessage(t *testing.T, file *descriptorpb.FileDescriptorProto, name protoreflect.Name) *descriptorpb.DescriptorProto {
	t.Helper()

	for _, msg := range file.GetMessageType() {
		if msg.GetName() == string(name) {
			return msg
		}
	}

	require.FailNowf(t, "missing message", "missing message %s", name)
	return nil
}

func findProtoField(t *testing.T, file *descriptorpb.FileDescriptorProto, msgName, fieldName protoreflect.Name) *descriptorpb.FieldDescriptorProto {
	t.Helper()

	msg := findProtoMessage(t, file, msgName)
	for _, field := range msg.GetField() {
		if field.GetName() == string(fieldName) {
			return field
		}
	}

	require.FailNowf(t, "missing field", "missing field %s.%s", msgName, fieldName)
	return nil
}
