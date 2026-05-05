package server

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"

	gethcore "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/flashbots/rpc-endpoint/database"
	rpctypes "github.com/flashbots/rpc-endpoint/types"
	"github.com/stretchr/testify/require"
)

func TestValidateTxLimits(t *testing.T) {
	mkTx := func(gas uint64) *types.Transaction {
		return types.NewTx(&types.LegacyTx{Nonce: 0, GasPrice: big.NewInt(1), Gas: gas})
	}

	t.Run("nil tx is rejected", func(t *testing.T) {
		err := validateTxLimits(nil)
		require.Error(t, err)
	})

	t.Run("normal transfer (21000) accepted", func(t *testing.T) {
		require.NoError(t, validateTxLimits(mkTx(21000)))
	})

	t.Run("exactly at cap accepted", func(t *testing.T) {
		require.NoError(t, validateTxLimits(mkTx(params.MaxTxGas)))
	})

	t.Run("cap+1 rejected", func(t *testing.T) {
		err := validateTxLimits(mkTx(params.MaxTxGas + 1))
		require.Error(t, err)
		require.True(t, errors.Is(err, gethcore.ErrGasLimitTooHigh),
			"error should wrap core.ErrGasLimitTooHigh")
		require.Contains(t, err.Error(), "16777216")
		require.Contains(t, err.Error(), fmt.Sprint(params.MaxTxGas+1))
	})

	t.Run("20M rejected", func(t *testing.T) {
		err := validateTxLimits(mkTx(20_000_000))
		require.Error(t, err)
		require.True(t, errors.Is(err, gethcore.ErrGasLimitTooHigh))
		require.Contains(t, err.Error(), "20000000")
	})
}

// signedRawTx returns a hex-encoded (no 0x prefix) signed legacy tx with the
// given gas limit, using a freshly generated key.
func signedRawTx(t *testing.T, gas uint64) string {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	chainID := big.NewInt(1)
	tx := types.MustSignNewTx(key, types.LatestSignerForChainID(chainID), &types.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      gas,
		To:       nil,
		Value:    big.NewInt(0),
	})
	raw, err := tx.MarshalBinary()
	require.NoError(t, err)
	return "0x" + hex.EncodeToString(raw)
}

// newSendRawTxRequest builds an RpcRequest configured with a mock relay client
// that records whether it was called.
func newSendRawTxRequest(t *testing.T, rawTx string) (*RpcRequest, *recordingClient) {
	t.Helper()
	relayKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	rc := &recordingClient{}
	r := &RpcRequest{
		jsonReq: &rpctypes.JsonRpcRequest{
			Id:      1,
			Method:  "eth_sendRawTransaction",
			Params:  []any{rawTx},
			Version: "2.0",
		},
		logger:            log.New(),
		client:            rc,
		ethSendRawTxEntry: &database.EthSendRawTxEntry{},
		relaySigningKey:   relayKey,
	}
	return r, rc
}

type recordingClient struct {
	called bool
}

func (c *recordingClient) ProxyRequest(body []byte) (*http.Response, error) {
	c.called = true
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(`{"result":"0x0"}`)),
	}, nil
}

var _ RPCProxyClient = &recordingClient{}

func TestSendRawTransaction_GasLimitCap(t *testing.T) {
	setupRedis()
	setupMockTxApi()

	t.Run("over-cap (20M) rejected with -32602 and not forwarded", func(t *testing.T) {
		r, rc := newSendRawTxRequest(t, signedRawTx(t, 20_000_000))
		r.handle_sendRawTransaction()

		require.False(t, rc.called, "tx must not be forwarded to relay")
		require.NotNil(t, r.jsonRes)
		require.NotNil(t, r.jsonRes.Error)
		require.Equal(t, rpctypes.JsonRpcInvalidParams, r.jsonRes.Error.Code)
		require.True(t, strings.Contains(r.jsonRes.Error.Message, "transaction gas limit too high"),
			"got: %s", r.jsonRes.Error.Message)
		require.Contains(t, r.jsonRes.Error.Message, "16777216")
	})

	t.Run("cap+1 rejected", func(t *testing.T) {
		r, rc := newSendRawTxRequest(t, signedRawTx(t, params.MaxTxGas+1))
		r.handle_sendRawTransaction()

		require.False(t, rc.called)
		require.NotNil(t, r.jsonRes)
		require.NotNil(t, r.jsonRes.Error)
		require.Equal(t, rpctypes.JsonRpcInvalidParams, r.jsonRes.Error.Code)
	})

	// Note: the "exactly at cap" / "under cap" success paths are covered by
	// TestValidateTxLimits above. Asserting the full success path through
	// handle_sendRawTransaction would require mocking redis writes,
	// OFAC checks, and the relay round-trip — out of scope for this PR.
}
