/*
 * RPC endpoint E2E tests.
 */
package tests

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flashbots/rpc-endpoint/database"

	"github.com/alicebob/miniredis"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/server"
	"github.com/flashbots/rpc-endpoint/testutils"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var RpcBackendServerUrl string

var relaySigningKey *ecdsa.PrivateKey

func init() {
	var err error
	relaySigningKey, err = crypto.HexToECDSA("7bdeed70a07d5a45546e83a88dd430f71348592e747d2d3eb23f32db003eb0e1")
	if err != nil {
		log.Crit("failed to create signing key", "err", err)
	}
}

// func setServerTimeNowOffset(td time.Duration) {
// 	server.Now = func() time.Time {
// 		return time.Now().Add(td)
// 	}
// }

var bundleJsonApi *httptest.Server

// Setup RPC endpoint and mock backend servers
func testServerSetupWithMockStore() {
	db := database.NewMockStore()
	testServerSetup(db)
}

func testServerSetup(db database.Store) {
	redisServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	// Create a fresh mock backend server (covers for both eth node and relay)
	rpcBackendServer := httptest.NewServer(http.HandlerFunc(testutils.RpcBackendHandler))
	RpcBackendServerUrl = rpcBackendServer.URL
	testutils.MockRpcBackendReset()
	testutils.MockTxApiReset()

	txApiServer := httptest.NewServer(http.HandlerFunc(testutils.MockTxApiHandler))
	server.ProtectTxApiHost = txApiServer.URL

	// Create a fresh RPC endpoint server
	rpcServer, err := server.NewRpcEndPointServer(server.Configuration{
		DB:                  db,
		Logger:              log.New("testlogger"),
		ProxyTimeoutSeconds: 10,
		ProxyUrl:            RpcBackendServerUrl,
		RedisUrl:            redisServer.Addr(),
		RelaySigningKey:     relaySigningKey,
		RelayUrl:            RpcBackendServerUrl,
		Version:             "test",
		DefaultMempoolRPC:   RpcBackendServerUrl,
	})
	if err != nil {
		panic(err)
	}
	rpcEndpointServer := httptest.NewServer(http.HandlerFunc(rpcServer.HandleHttpRequest))
	bundleJsonApi = httptest.NewServer(http.HandlerFunc(rpcServer.HandleBundleRequest))
	testutils.RpcEndpointUrl = rpcEndpointServer.URL
}

/*
 * HTTP TESTS
 */
// Check headers: status and content-type
func TestStandardHeaders(t *testing.T) {
	testServerSetupWithMockStore()

	rpcRequest := types.NewJsonRpcRequest(1, "null", nil)
	jsonData, err := json.Marshal(rpcRequest)
	require.Nil(t, err, err)

	resp, err := http.Post(RpcBackendServerUrl, "application/json", bytes.NewBuffer(jsonData))
	require.Nil(t, err, err)

	// Test for http status-code 200
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test for content-type: application/json
	contentTypeHeader := resp.Header.Get("content-type")
	assert.Equal(t, "application/json", strings.ToLower(contentTypeHeader))
}

// Check json-rpc id and version
func TestJsonRpc(t *testing.T) {
	testServerSetupWithMockStore()

	_id1 := float64(84363)
	rpcRequest := types.NewJsonRpcRequest(_id1, "null", nil)
	rpcResult := testutils.SendRpcAndParseResponseOrFailNow(t, rpcRequest)
	assert.Equal(t, _id1, rpcResult.Id)

	_id2 := "84363"
	rpcRequest2 := types.NewJsonRpcRequest(_id2, "null", nil)
	rpcResult2 := testutils.SendRpcAndParseResponseOrFailNow(t, rpcRequest2)
	assert.Equal(t, _id2, rpcResult2.Id)
	assert.Equal(t, "2.0", rpcResult2.Version)
}

/*
 * REQUEST TESTS
 */

// Test intercepting eth_call for Flashbots RPC contract
func TestEthCallIntercept(t *testing.T) {
	testServerSetupWithMockStore()
	var rpcResult string

	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	rpcResult = testutils.SendRpcAndParseResponseOrFailNowString(t, req)
	require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", rpcResult, "FlashRPC contract - eth_call intercept")

	// eth_call passthrough
	req2 := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	rpcResult = testutils.SendRpcAndParseResponseOrFailNowString(t, req2)
	require.Equal(t, "0x12345", rpcResult, "FlashRPC contract - eth_call passthrough")
}

func TestNetVersionIntercept(t *testing.T) {
	testServerSetupWithMockStore()
	var rpcResult string

	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "net_version", nil)
	res, err := testutils.SendRpcAndParseResponseTo(RpcBackendServerUrl, req)
	require.Nil(t, err, err)
	json.Unmarshal(res.Result, &rpcResult)
	require.Equal(t, "3", rpcResult, "net_version from backend")

	rpcResult = testutils.SendRpcAndParseResponseOrFailNowString(t, req)
	require.Nil(t, res.Error)
	require.Equal(t, "3", rpcResult, "net_version intercept")
}

// Ensure bundle response is the tx hash, not the bundle id
func TestSendBundleResponse(t *testing.T) {
	testServerSetupWithMockStore()

	// should be tx hash
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_RawTx})
	rpcResult := testutils.SendRpcAndParseResponseOrFailNowString(t, req_sendRawTransaction)
	require.Equal(t, testutils.TestTx_BundleFailedTooManyTimes_Hash, rpcResult)
}

func TestNull(t *testing.T) {
	testServerSetupWithMockStore()
	expectedResultRaw := `{"id":1,"result":null,"jsonrpc":"2.0"}` + "\n"

	// Build and do RPC call: "null"
	rpcRequest := types.NewJsonRpcRequest(1, "null", nil)
	jsonData, err := json.Marshal(rpcRequest)
	require.Nil(t, err, err)
	resp, err := http.Post(RpcBackendServerUrl, "application/json", bytes.NewBuffer(jsonData))
	require.Nil(t, err, err)
	respData, err := io.ReadAll(resp.Body)
	require.Nil(t, err, err)

	// Check that raw result is expected
	require.Equal(t, expectedResultRaw, string(respData))

	// Parsing null results in "null":
	var jsonRpcResp types.JsonRpcResponse
	err = json.Unmarshal(respData, &jsonRpcResp)
	require.Nil(t, err, err)

	require.Equal(t, 4, len(jsonRpcResp.Result))
	require.Equal(t, json.RawMessage{110, 117, 108, 108}, jsonRpcResp.Result) // the bytes for null

	// Double-check that plain bytes are 'null'
	resultStr := string(jsonRpcResp.Result)
	require.Equal(t, "null", resultStr)
}

func TestGetTxReceiptNull(t *testing.T) {
	testServerSetupWithMockStore()

	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_Hash})
	jsonResp := testutils.SendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	// fmt.Println(jsonResp)
	require.Equal(t, "null", string(jsonResp.Result))

	jsonResp, err := testutils.SendRpcAndParseResponseTo(RpcBackendServerUrl, req_getTransactionCount)
	require.Nil(t, err, err)

	fmt.Println(jsonResp)
	require.Equal(t, "null", string(jsonResp.Result))
}

func TestMetamaskFix(t *testing.T) {
	testServerSetupWithMockStore()
	testutils.MockTxApiStatusForHash[testutils.TestTx_MM2_Hash] = types.TxStatusFailed

	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{testutils.TestTx_MM2_From, "latest"})
	txCountBefore := testutils.SendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)

	//first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_MM2_RawTx})
	r1 := testutils.SendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.NotNil(t, r1.Error, r1.Error) // tx will actually fail due to incorrect nonce but it's fine for this test, since we mark it as sent to relay
	//fmt.Printf("\n\n\n\n\n")

	// call getTxReceipt to trigger query to Tx API
	req_getTransactionReceipt := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_MM2_Hash})
	jsonResp := testutils.SendRpcAndParseResponseOrFailNow(t, req_getTransactionReceipt)
	require.Nil(t, jsonResp.Error)
	require.Equal(t, "null", string(jsonResp.Result))
	// require.Equal(t, "Transaction failed", jsonResp.Error.Message)

	// At this point, the tx hash should be blacklisted and too high a nonce is returned
	valueAfter1 := testutils.SendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.NotEqual(t, txCountBefore, valueAfter1)
	require.Equal(t, "0x3b9aca01", valueAfter1)

	// getTransactionCount 2/4 should return the same (fixed) value
	valueAfter2 := testutils.SendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, valueAfter1, valueAfter2)

	// getTransactionCount 3/4 should return the same (fixed) value
	valueAfter3 := testutils.SendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, valueAfter1, valueAfter3)

	// getTransactionCount 4/4 should return the same (fixed) value
	valueAfter4 := testutils.SendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, valueAfter1, valueAfter4)

	// getTransactionCount 5 should return the initial value
	valueAfter5 := testutils.SendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, txCountBefore, valueAfter5)
}

func TestRelayTx(t *testing.T) {
	testServerSetupWithMockStore()

	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_RawTx})
	r1 := testutils.SendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error)

	// Ensure that request called eth_sendPrivateTransaction with correct param
	require.Equal(t, "eth_sendPrivateTransaction", testutils.MockBackendLastJsonRpcRequest.Method)

	resp := testutils.MockBackendLastJsonRpcRequest.Params[0].(map[string]interface{})
	require.Equal(t, testutils.TestTx_BundleFailedTooManyTimes_RawTx, resp["tx"])

	// Ensure that request was signed properly
	pubkey := crypto.PubkeyToAddress(relaySigningKey.PublicKey).Hex()
	require.Equal(t, pubkey+":0x06e1ea66c5fc1017787369beffc9c9acd82570b4ec4ea075c708f2351a26fdff4abbf601037884d0785ff88985b590f7b865852a4100d5670605a56b9118804900", testutils.MockBackendLastRawRequest.Header.Get("X-Flashbots-Signature"))

	// Check result - should be the tx hash
	var res string
	json.Unmarshal(r1.Result, &res)
	require.Equal(t, testutils.TestTx_BundleFailedTooManyTimes_Hash, res)

	timeStampFirstRequest := testutils.MockBackendLastJsonRpcRequestTimestamp

	// Send tx again, should not arrive at backend
	testutils.SendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error)
	require.Equal(t, timeStampFirstRequest, testutils.MockBackendLastJsonRpcRequestTimestamp)

	// Ensure nonce is saved to redis
	nonce, found, err := server.RState.GetSenderMaxNonce(testutils.TestTx_BundleFailedTooManyTimes_From)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(30), nonce)
}

func TestRelayTxWithAuctionPreference(t *testing.T) {
	// Store setup
	memStore := database.NewMemStore()

	// Server setup
	testServerSetup(memStore)

	tx := testutils.TestTx_BundleFailedTooManyTimes_RawTx
	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	reqSendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{tx})
	// call rpc with auction preference
	r1 := testutils.SendRpcWithAuctionPreferenceAndParseResponse(t, reqSendRawTransaction, "/?hint=calldata&hint=contract_address")
	require.Nil(t, r1.Error)

	// Ensure that request called eth_sendPrivateTransaction with correct param
	require.Equal(t, "eth_sendPrivateTransaction", testutils.MockBackendLastJsonRpcRequest.Method)

	resp := testutils.MockBackendLastJsonRpcRequest.Params[0].(map[string]interface{})
	require.Equal(t, tx, resp["tx"])
	// Ensure fast endpoint is called and fast preference is set
	auctionPref := resp["preferences"].(map[string]interface{})["privacy"].(map[string]interface{})
	require.NotNil(t, auctionPref)

	expectedHints := []string{"calldata", "contract_address", "special_logs"}
	hintPref := auctionPref["hints"].([]interface{})
	for i, hint := range hintPref {
		strHint := hint.(string)
		require.Equal(t, expectedHints[i], strHint)
	}

	require.Equal(t, 1, len(memStore.EthSendRawTxs))
}

func TestRelayTxWithIncorrectAuctionPreference(t *testing.T) {
	// Store setup
	memStore := database.NewMemStore()

	// Server setup
	testServerSetup(memStore)

	tx := testutils.TestTx_BundleFailedTooManyTimes_RawTx
	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	reqSendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{tx})
	// call rpc with auction preference
	r1 := testutils.SendRpcWithAuctionPreferenceAndParseResponse(t, reqSendRawTransaction, "/?hint=incorrect")
	require.Contains(t, r1.Error.Message, "Incorrect auction hint")
}

func TestRelayCancelTx(t *testing.T) {
	testServerSetupWithMockStore()

	// sendRawTransaction of the initial TX
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_CancelAtRelay_Initial_RawTx})
	testutils.SendRpcAndParseResponseOrFailNow(t, req_sendRawTransaction)

	// Ensure that request called eth_sendPrivateTransaction on the Relay
	require.Equal(t, "eth_sendPrivateTransaction", testutils.MockBackendLastJsonRpcRequest.Method)

	// Ensure that the RPC backend sent the rawTx to the relay
	resp := testutils.MockBackendLastJsonRpcRequest.Params[0].(map[string]interface{})
	require.Equal(t, testutils.TestTx_CancelAtRelay_Initial_RawTx, resp["tx"])

	// Send cancel-tx to the RPC backend
	req_cancelTx := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_CancelAtRelay_Cancel_RawTx})
	cancelResp := testutils.SendRpcAndParseResponseOrFailNow(t, req_cancelTx)

	// Ensure that request called eth_sendPrivateTransaction on the Relay
	require.Equal(t, "eth_cancelPrivateTransaction", testutils.MockBackendLastJsonRpcRequest.Method)
	var res string
	json.Unmarshal(cancelResp.Result, &res)

	// Ensure the response is the tx hash
	require.Equal(t, testutils.TestTx_CancelAtRelay_Cancel_Hash, res)
}

// cancel-tx without initial related tx would just go to mempool
func TestRelayCancelTxWithoutInitialTx(t *testing.T) {
	testServerSetupWithMockStore()

	// Send cancel-tx to the RPC backend
	req_cancelTx := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_CancelAtRelay_Cancel_RawTx})
	cancelResp := testutils.SendRpcAndParseResponseOrFailNow(t, req_cancelTx)

	// Ensure that request called eth_sendRawTransaction on the relay, cause we don't send txs to mempool now
	require.Equal(t, "eth_sendPrivateTransaction", testutils.MockBackendLastJsonRpcRequest.Method)
	var res string
	json.Unmarshal(cancelResp.Result, &res)

	// Ensure the response is the tx hash
	require.Equal(t, testutils.TestTx_CancelAtRelay_Cancel_Hash, res)
}

// tx with wrong nonce should be rejected
func TestRelayTxWithWrongNonce(t *testing.T) {
	testServerSetupWithMockStore()

	nonceOrig := testutils.TestTx_BundleFailedTooManyTimes_Nonce
	testutils.TestTx_BundleFailedTooManyTimes_Nonce = "0x1f"
	defer func() { testutils.TestTx_BundleFailedTooManyTimes_Nonce = nonceOrig }()

	// Send cancel-tx to the RPC backend
	req1 := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_RawTx})
	resp1 := testutils.SendRpcAndParseResponseOrFailNow(t, req1)

	// Ensure the response has an error
	require.NotNil(t, resp1.Error)
	require.Equal(t, "invalid nonce", resp1.Error.Message)
}

// Test batch request with multiple eth raw transaction
func TestBatch_eth_sendRawTransaction(t *testing.T) {
	t.Skip()
	testServerSetupWithMockStore()

	var batch []*types.JsonRpcRequest
	for i := range "testing" {
		rpcRequest := types.NewJsonRpcRequest(i, "eth_sendRawTransaction", []interface{}{testutils.TestTx_CancelAtRelay_Cancel_RawTx})
		batch = append(batch, rpcRequest)
	}
	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 7)
}

// Test batch request with different eth transaction
func TestBatch_eth_transaction(t *testing.T) {
	t.Skip()
	testServerSetupWithMockStore()

	var batch []*types.JsonRpcRequest
	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{testutils.TestTx_MM2_From, "latest"})
	batch = append(batch, req_getTransactionCount)
	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := types.NewJsonRpcRequest(2, "eth_sendRawTransaction", []interface{}{testutils.TestTx_MM2_RawTx})
	batch = append(batch, req_sendRawTransaction)
	// call getTxReceipt to trigger query to Tx API
	req_getTransactionReceipt := types.NewJsonRpcRequest(3, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_MM2_Hash})
	batch = append(batch, req_getTransactionReceipt)

	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 3)

	m := map[float64]*types.JsonRpcResponse{
		float64(1): {Id: float64(1), Result: []byte(`"0x22"`), Error: nil, Version: "2.0"},
		float64(2): {Id: float64(2), Result: []byte(`"tx-hash1"`), Error: nil, Version: "2.0"},
		float64(3): {Id: float64(3), Result: []byte(`null`), Error: nil, Version: "2.0"},
	}
	for _, j := range res {
		assert.Equal(t, m[j.Id.(float64)], j)
	}

}

// Test batch request with different eth transaction
func TestBatch_eth_call(t *testing.T) {
	t.Skip()
	testServerSetupWithMockStore()

	var batch []*types.JsonRpcRequest
	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	batch = append(batch, req)
	// eth_call passthrough
	req2 := types.NewJsonRpcRequest(2, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	batch = append(batch, req2)
	req_getTransactionCount := types.NewJsonRpcRequest(3, "eth_getTransactionCount", []interface{}{testutils.TestTx_MM2_From, "latest"})
	batch = append(batch, req_getTransactionCount)
	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := types.NewJsonRpcRequest(4, "eth_sendRawTransaction", []interface{}{testutils.TestTx_MM2_RawTx})
	batch = append(batch, req_sendRawTransaction)
	// call getTxReceipt to trigger query to Tx API
	req_getTransactionReceipt := types.NewJsonRpcRequest(5, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_MM2_Hash})
	batch = append(batch, req_getTransactionReceipt)

	m := map[float64]*types.JsonRpcResponse{
		float64(1): {Id: float64(1), Result: []byte(`"0x0000000000000000000000000000000000000000000000000000000000000001"`), Error: nil, Version: "2.0"},
		float64(2): {Id: float64(2), Result: []byte(`"0x12345"`), Error: nil, Version: "2.0"},
		float64(3): {Id: float64(3), Result: []byte(`"0x22"`), Error: nil, Version: "2.0"},
		float64(4): {Id: float64(4), Result: []byte(`"tx-hash1"`), Error: nil, Version: "2.0"},
		float64(5): {Id: float64(5), Result: []byte(`null`), Error: nil, Version: "2.0"},
	}
	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 5)
	for _, j := range res {
		assert.Equal(t, m[j.Id.(float64)], j)
	}

}

// Test batch request with different transaction
func TestBatch_CombinationOfSuccessAndFailure(t *testing.T) {
	t.Skip()
	testServerSetupWithMockStore()

	var batch []*types.JsonRpcRequest
	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	batch = append(batch, req)
	// eth_call passthrough
	req2 := types.NewJsonRpcRequest(1, "eth_callxvssfa", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	batch = append(batch, req2)
	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{testutils.TestTx_MM2_From, "latest"})
	batch = append(batch, req_getTransactionCount)
	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransactionxxx", []interface{}{testutils.TestTx_MM2_RawTx})
	batch = append(batch, req_sendRawTransaction)
	// call getTxReceipt to trigger query to Tx API
	req_getTransactionReceipt := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_MM2_Hash})
	batch = append(batch, req_getTransactionReceipt)

	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 5)
}

// Test batch request with multiple eth raw transaction
func TestBatch_Validate_eth_sendRawTransaction_Error(t *testing.T) {
	t.Skip()
	testServerSetupWithMockStore()
	// key=request-id, value=json-rpc error
	m := map[float64]int{
		1: types.JsonRpcInvalidParams,
		2: types.JsonRpcInvalidParams,
		3: types.JsonRpcInvalidParams,
		4: types.JsonRpcInvalidRequest,
	}
	var batch []*types.JsonRpcRequest

	r1 := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{})     // no params
	r2 := types.NewJsonRpcRequest(2, "eth_sendRawTransaction", nil)                 // nil params
	r3 := types.NewJsonRpcRequest(3, "eth_sendRawTransaction", []interface{}{"x"})  // invalid params
	r4 := types.NewJsonRpcRequest(4, "eth_sendRawTransaction", []interface{}{"xy"}) // invalid request
	batch = append(batch, r1, r2, r3, r4)

	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 4)
	for _, r := range res {
		assert.Equal(t, m[r.Id.(float64)], r.Error.Code)
	}
}

// Whitehat Tests
func TestWhitehatBundleCollection(t *testing.T) {
	testServerSetupWithMockStore()

	bundleId := "123"
	url := testutils.RpcEndpointUrl + "?bundle=" + bundleId

	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_RawTx})
	resp, err := testutils.SendRpcAndParseResponseTo(url, req_sendRawTransaction)
	require.Nil(t, err, err)
	require.Nil(t, resp.Error, resp.Error)

	// Last request should be network version (executed on start)
	require.Equal(t, &types.JsonRpcRequest{Id: float64(1), Method: "net_version", Params: []interface{}{}, Version: "2.0"}, testutils.MockBackendLastJsonRpcRequest)
	// Check redis
	txs, err := server.RState.GetWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)
	require.Equal(t, 1, len(txs))

	// Send again (#2)
	resp, err = testutils.SendRpcAndParseResponseTo(url, req_sendRawTransaction)
	require.Nil(t, err, err)
	require.Nil(t, resp.Error, resp.Error)

	// Check redis (#2)
	txs, err = server.RState.GetWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)
	require.Equal(t, 1, len(txs))

	// Check JSON API
	jsonApiUrl := bundleJsonApi.URL + "/bundle?id=" + bundleId
	fmt.Println("jsonApiUrl: ", jsonApiUrl)
	res, err := http.Get(jsonApiUrl)
	require.Nil(t, err, err)
	body, err := io.ReadAll(res.Body)
	require.Nil(t, err, err)
	fmt.Println(string(body))
	bundleResponse := new(types.BundleResponse)
	err = json.Unmarshal(body, bundleResponse)
	require.Nil(t, err, err)
	require.Equal(t, bundleId, bundleResponse.BundleId)
	require.Equal(t, 1, len(bundleResponse.RawTxs))
}

func TestWhitehatBundleCollectionGetBalance(t *testing.T) {
	testServerSetupWithMockStore()
	bundleId := "123"
	url := testutils.RpcEndpointUrl + "?bundle=" + bundleId

	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getBalance", []interface{}{testutils.TestTx_MM2_From, "latest"})
	resp, err := testutils.SendRpcAndParseResponseTo(url, req_getTransactionCount)
	require.Nil(t, err, err)
	require.Nil(t, resp.Error, resp.Error)
	val := ""
	err = json.Unmarshal(resp.Result, &val)
	require.Nil(t, err, err)
	require.Equal(t, "0x56bc75e2d63100000", val)
}

func Test_StoreRequests(t *testing.T) {
	// Store setup
	memStore := database.NewMemStore()

	// Server setup
	testServerSetup(memStore)

	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_Hash})
	_ = testutils.SendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	// sendRawTransaction of the initial TX
	reqSendRawTransaction1 := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_CancelAtRelay_Initial_RawTx})
	testutils.SendRpcAndParseResponseOrFailNow(t, reqSendRawTransaction1)

	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	reqSendRawTransaction2 := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_BundleFailedTooManyTimes_RawTx})
	r1 := testutils.SendRpcAndParseResponseOrFailNowAllowRpcError(t, reqSendRawTransaction2)
	require.Nil(t, r1.Error)

	require.Equal(t, 2, len(memStore.Requests))
	require.Equal(t, 2, len(memStore.EthSendRawTxs))
	for _, txs := range memStore.EthSendRawTxs {
		for _, tx := range txs {
			assert.Equal(t, true, tx.NeedsFrontRunningProtection)
		}
	}
}

func Test_StoreBatchRequests(t *testing.T) {
	t.Skip()
	// Store setup
	memStore := database.NewMemStore()
	// Server setup
	testServerSetup(memStore)

	var batch []*types.JsonRpcRequest
	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	batch = append(batch, req)
	// eth_call passthrough
	req2 := types.NewJsonRpcRequest(1, "eth_callxvssfa", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	batch = append(batch, req2)
	reqGetTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{testutils.TestTx_MM2_From, "latest"})
	batch = append(batch, reqGetTransactionCount)
	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	reqSendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_MM2_RawTx})
	batch = append(batch, reqSendRawTransaction)
	// call getTxReceipt to trigger query to Tx API
	reqGetTransactionReceipt := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{testutils.TestTx_MM2_Hash})
	batch = append(batch, reqGetTransactionReceipt)

	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 5)
	require.Equal(t, 1, len(memStore.Requests))
	require.Equal(t, 1, len(memStore.EthSendRawTxs))
}

func Test_StoreValidateTxs(t *testing.T) {
	t.Skip()
	// Store setup
	memStore := database.NewMemStore()

	// Server setup
	testServerSetup(memStore)

	var batch []*types.JsonRpcRequest

	// call sendRawTx
	reqSendRawTransactionInvalidNonce1 := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_Invalid_Nonce_1})
	batch = append(batch, reqSendRawTransactionInvalidNonce1)

	reqSendRawTransactionInvalidNonce2 := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{testutils.TestTx_Invalid_Nonce_2})
	batch = append(batch, reqSendRawTransactionInvalidNonce2)

	res, err := testutils.SendBatchRpcAndParseResponse(batch)
	require.Nil(t, err, err)
	assert.Equal(t, len(res), 2)
	require.Equal(t, 1, len(memStore.Requests))
	require.Equal(t, 1, len(memStore.EthSendRawTxs))

	for _, entries := range memStore.EthSendRawTxs {
		for _, entry := range entries {
			require.True(t, entry.NeedsFrontRunningProtection)
			require.Equal(t, "invalid nonce", entry.Error)
			require.Equal(t, -32603, entry.ErrorCode)
			require.Equal(t, 10, len(entry.TxSmartContractMethod))
			require.False(t, entry.Fast)
		}

	}

}
