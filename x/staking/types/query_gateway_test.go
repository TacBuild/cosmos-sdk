package types

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

type tokenizeShareRecordByDenomGatewayServer struct {
	UnimplementedQueryServer

	denom string
}

func (s *tokenizeShareRecordByDenomGatewayServer) TokenizeShareRecordByDenom(
	ctx context.Context,
	req *QueryTokenizeShareRecordByDenomRequest,
) (*QueryTokenizeShareRecordByDenomResponse, error) {
	s.denom = req.Denom
	return &QueryTokenizeShareRecordByDenomResponse{}, nil
}

func TestTokenizeShareRecordByDenomGatewayAcceptsSlashDenomQueryParam(t *testing.T) {
	denom := "cosmosvaloper1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqp5r57p/1"
	req := httptest.NewRequest(
		"GET",
		"/cosmos/staking/v1beta1/tokenize_share_record_by_denom?denom="+url.QueryEscape(denom),
		nil,
	)
	server := &tokenizeShareRecordByDenomGatewayServer{}

	res, _, err := local_request_Query_TokenizeShareRecordByDenom_0(
		context.Background(),
		nil,
		server,
		req,
		map[string]string{},
	)

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, denom, server.denom)
}
