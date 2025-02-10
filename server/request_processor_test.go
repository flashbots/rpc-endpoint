package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/testutils"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/stretchr/testify/require"
)

func setupRedis() {
	redisServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	RState, err = NewRedisState(redisServer.Addr())
	if err != nil {
		panic(err)
	}
}

func setupMockTxApi() {
	txApiServer := httptest.NewServer(http.HandlerFunc(testutils.MockTxApiHandler))
	ProtectTxApiHost = txApiServer.URL
	testutils.MockTxApiReset()
}

func setServerTimeNowOffset(td time.Duration) {
	Now = func() time.Time {
		return time.Now().Add(td)
	}
}

func TestRequestshouldSendTxToRelay(t *testing.T) {
	setupRedis()
	setupMockTxApi()

	request := RpcRequest{}
	txHash := "0x0Foo"

	// SEND when not seen before
	shouldSend := !request.blockResendingTxToRelay(txHash)
	require.True(t, shouldSend)

	// Fake a previous send
	err := RState.SetTxSentToRelay(txHash)
	require.Nil(t, err, err)

	// Ensure tx status is UNKNOWN
	txStatusApiResponse, err := GetTxStatus(txHash)
	require.Nil(t, err, err)
	require.Equal(t, types.TxStatusUnknown, txStatusApiResponse.Status)

	// NOT SEND when unknown and time since sent < 5 min
	shouldSend = !request.blockResendingTxToRelay(txHash)
	require.False(t, shouldSend)

	// Set tx status to Failed
	testutils.MockTxApiStatusForHash[txHash] = types.TxStatusFailed
	txStatusApiResponse, err = GetTxStatus(txHash)
	require.Nil(t, err, err)
	require.Equal(t, types.TxStatusFailed, txStatusApiResponse.Status)

	// SEND if failed
	shouldSend = !request.blockResendingTxToRelay(txHash)
	require.True(t, shouldSend)

	// Set tx status to pending
	testutils.MockTxApiStatusForHash[txHash] = types.TxStatusPending
	txStatusApiResponse, err = GetTxStatus(txHash)
	require.Nil(t, err, err)
	require.Equal(t, types.TxStatusPending, txStatusApiResponse.Status)

	// NOT SEND if pending
	shouldSend = !request.blockResendingTxToRelay(txHash)
	require.False(t, shouldSend)

	//
	// SEND if UNKNOWN and 5 minutes have passed
	//
	txHash = "0x0DeadBeef"
	setServerTimeNowOffset(time.Minute * -6)
	defer setServerTimeNowOffset(0)

	err = RState.SetTxSentToRelay(txHash)
	require.Nil(t, err, err)

	timeSent, found, err := RState.GetTxSentToRelay(txHash)
	require.Nil(t, err, err)
	require.True(t, found)
	require.True(t, time.Since(timeSent) > time.Minute*4)

	// Ensure tx status is UNKNOWN
	txStatusApiResponse, err = GetTxStatus(txHash)
	require.Nil(t, err, err)
	require.Equal(t, types.TxStatusUnknown, txStatusApiResponse.Status)

	shouldSend = !request.blockResendingTxToRelay(txHash)
	require.True(t, shouldSend)
}

type mockClient struct {
	err          error
	nextResponse *http.Response
}

func (m mockClient) ProxyRequest(body []byte) (*http.Response, error) {
	return m.nextResponse, m.err
}

var _ RPCProxyClient = &mockClient{}

func TestFailedTxShouldResetMaxNonce(t *testing.T) {
	setupRedis()
	setupMockTxApi()

	sender := "0x6bc84f6a0fabbd7102be338c048fe0ae54948c2e"
	txHash := "0x58e5a0fc7fbc849eddc100d44e86276168a8c7baaa5604e44ba6f5eb8ba1b7eb"

	mockClient := &mockClient{}
	privKey, _ := crypto.GenerateKey()
	require.NotNil(t, privKey)

	t.Run("setup", func(t *testing.T) {
		err := RState.SetSenderMaxNonce(sender, 4, 10)
		require.NoError(t, err)

		status, err := GetTxStatus(txHash)
		require.NoError(t, err)
		require.Equal(t, types.TxStatusUnknown, status.Status)
	})

	// Send a tx
	// NOTE: this portion is somewhat brittle and possibly prone to breakage
	// if we see this test failing then we should invest in proper mock tooling
	// around RpcRequest.
	t.Run("send tx", func(t *testing.T) {
		r := RpcRequest{}
		r.jsonReq = &types.JsonRpcRequest{
			Id:     1,
			Method: "eth_sendRawTransaction",
			Params: []any{
				"0xf86c258502540be40083035b609482e041e84074fc5f5947d4d27e3c44f824b7a1a187b1a2bc2ec500008078a04a7db627266fa9a4116e3f6b33f5d245db40983234eb356261f36808909d2848a0166fa098a2ce3bda87af6000ed0083e3bf7cc31c6686b670bd85cbc6da2d6e85",
			},
			Version: "2.0",
		}
		r.logger = log.New()
		r.client = mockClient
		r.ethSendRawTxEntry = &database.EthSendRawTxEntry{}
		r.relaySigningKey = privKey
		mockClient.nextResponse = &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"results":"0x58e5a0fc7fbc849eddc100d44e86276168a8c7baaa5604e44ba6f5eb8ba1b7eb"}`)),
		}

		// we expect handle_sendRawTransaction to set the
		// nonce in the RState cache to the txs nonce (0x25)
		r.handle_sendRawTransaction()

		// but actually the nonce is set asynchronously so we need to first
		// sleep until we see it in the cache.
		for i := 0; i < 5; i++ {
			_, found, _ := RState.GetSenderMaxNonce(sender)
			if !found {
				time.Sleep(time.Millisecond * time.Duration(i*10))
			}
		}

		// once found we can check the results
		require.Equal(t, r.tx.Nonce(), uint64(0x25))
		maxNonce, found, err := RState.GetSenderMaxNonce(sender)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, r.tx.Nonce(), maxNonce)
	})

	// mark tx failed in mock status API
	t.Run("fail tx", func(t *testing.T) {
		testutils.MockTxApiStatusForHash[txHash] = types.TxStatusFailed
	})

	// and now simulate the user sending a eth_getTransactionReceipt request
	t.Run("eth_getTransactionReceipt", func(t *testing.T) {
		r := RpcRequest{}
		r.logger = log.New()
		r.jsonReq = &types.JsonRpcRequest{
			Id:      1,
			Method:  "eth_getTransactionReceipt",
			Params:  []any{txHash},
			Version: "2.0",
		}
		response := &types.JsonRpcResponse{
			Id:      1,
			Result:  json.RawMessage(`null`),
			Error:   nil,
			Version: "2.0",
		}
		r.check_post_getTransactionReceipt(response)
	})

	// ensure that the max nonce is cleared
	t.Run("check nonce", func(t *testing.T) {
		maxNonce, found, err := RState.GetSenderMaxNonce(sender)
		require.NoError(t, err)
		require.Equal(t, uint64(0x0), maxNonce)
		require.False(t, found)
	})
}
