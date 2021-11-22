/*
 * RPC endpoint tests.
 */
package test

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/ethereum/go-ethereum/crypto"
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
		log.Fatal(err)
	}
}

// func setServerTimeNowOffset(td time.Duration) {
// 	server.Now = func() time.Time {
// 		return time.Now().Add(td)
// 	}
// }

// Reset the RPC endpoint and mock backend servers
func resetTestServers() {
	redisServer, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	// Create a fresh mock backend server (covers for both eth node and relay)
	rpcBackendServer := httptest.NewServer(http.HandlerFunc(RpcBackendHandler))
	RpcBackendServerUrl = rpcBackendServer.URL
	MockBackendLastRawRequest = nil
	MockBackendLastJsonRpcRequest = nil
	MockBackendLastJsonRpcRequestTimestamp = time.Time{}

	testutils.MockTxApiReset()
	txApiServer := httptest.NewServer(http.HandlerFunc(testutils.MockTxApiHandler))
	server.ProtectTxApiHost = txApiServer.URL

	// Create a fresh RPC endpoint server
	rpcServer, err := server.NewRpcEndPointServer("test", "", rpcBackendServer.URL, rpcBackendServer.URL, relaySigningKey, redisServer.Addr())
	if err != nil {
		panic(err)
	}
	rpcEndpointServer := httptest.NewServer(http.HandlerFunc(rpcServer.HandleHttpRequest))
	RpcEndpointUrl = rpcEndpointServer.URL
}

func init() {
	resetTestServers()
}

/*
 * HTTP TESTS
 */
// Check headers: status and content-type
func TestStandardHeaders(t *testing.T) {
	resetTestServers()

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
	resetTestServers()

	_id1 := float64(84363)
	rpcRequest := types.NewJsonRpcRequest(_id1, "null", nil)
	rpcResult := sendRpcAndParseResponseOrFailNow(t, rpcRequest)
	assert.Equal(t, _id1, rpcResult.Id)

	_id2 := "84363"
	rpcRequest2 := types.NewJsonRpcRequest(_id2, "null", nil)
	rpcResult2 := sendRpcAndParseResponseOrFailNow(t, rpcRequest2)
	assert.Equal(t, _id2, rpcResult2.Id)
	assert.Equal(t, "2.0", rpcResult2.Version)
}

/*
 * REQUEST TESTS
 */

// Test intercepting eth_call for Flashbots RPC contract
func TestEthCallIntercept(t *testing.T) {
	resetTestServers()
	var rpcResult string

	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	rpcResult = sendRpcAndParseResponseOrFailNowString(t, req)
	require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", rpcResult, "FlashRPC contract - eth_call intercept")

	// eth_call passthrough
	req2 := types.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	rpcResult = sendRpcAndParseResponseOrFailNowString(t, req2)
	require.Equal(t, "0x12345", rpcResult, "FlashRPC contract - eth_call passthrough")
}

func TestNetVersionIntercept(t *testing.T) {
	resetTestServers()
	var rpcResult string

	// eth_call intercept
	req := types.NewJsonRpcRequest(1, "net_version", nil)
	res, err := server.SendRpcAndParseResponseTo(RpcBackendServerUrl, req)
	require.Nil(t, err, err)
	json.Unmarshal(res.Result, &rpcResult)
	require.Equal(t, "3", rpcResult, "net_version from backend")

	rpcResult = sendRpcAndParseResponseOrFailNowString(t, req)
	require.Nil(t, res.Error)
	require.Equal(t, "1", rpcResult, "net_version intercept")
}

// Ensure bundle response is the tx hash, not the bundle id
func TestSendBundleResponse(t *testing.T) {
	resetTestServers()

	// should be tx hash
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{TestTx_BundleFailedTooManyTimes_RawTx})
	rpcResult := sendRpcAndParseResponseOrFailNowString(t, req_sendRawTransaction)
	require.Equal(t, TestTx_BundleFailedTooManyTimes_Hash, rpcResult)
}

func TestNull(t *testing.T) {
	resetTestServers()
	expectedResultRaw := `{"id":1,"result":null,"jsonrpc":"2.0"}` + "\n"

	// Build and do RPC call: "null"
	rpcRequest := types.NewJsonRpcRequest(1, "null", nil)
	jsonData, err := json.Marshal(rpcRequest)
	require.Nil(t, err, err)
	resp, err := http.Post(RpcBackendServerUrl, "application/json", bytes.NewBuffer(jsonData))
	require.Nil(t, err, err)
	respData, err := ioutil.ReadAll(resp.Body)
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
	resetTestServers()

	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{TestTx_BundleFailedTooManyTimes_Hash})
	jsonResp := sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	fmt.Println(jsonResp)
	require.Equal(t, "null", string(jsonResp.Result))

	jsonResp, err := server.SendRpcAndParseResponseTo(RpcBackendServerUrl, req_getTransactionCount)
	require.Nil(t, err, err)

	fmt.Println(jsonResp)
	require.Equal(t, "null", string(jsonResp.Result))
}

func TestMetamaskFix(t *testing.T) {
	resetTestServers()
	testutils.MockTxApiStatusForHash[TestTx_MM2_Hash] = types.TxStatusFailed

	req_getTransactionCount := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{TestTx_MM2_From, "latest"})
	txCountBefore := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)

	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{TestTx_MM2_RawTx})
	r1 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error, r1.Error)
	fmt.Printf("\n\n\n\n\n")

	// call getTxReceipt to trigger query to Tx API
	req_getTransactionReceipt := types.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{TestTx_MM2_Hash})
	jsonResp := sendRpcAndParseResponseOrFailNow(t, req_getTransactionReceipt)
	require.NotNil(t, jsonResp.Error)
	require.Equal(t, "Transaction failed", jsonResp.Error.Message)

	// At this point, the tx hash should be blacklisted and too high a nonce is returned
	valueAfter1 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.NotEqual(t, txCountBefore, valueAfter1)
	require.Equal(t, "0x3b9aca01", valueAfter1)

	// getTransactionCount 2/4 should return the same (fixed) value
	valueAfter2 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, valueAfter1, valueAfter2)

	// getTransactionCount 3/4 should return the same (fixed) value
	valueAfter3 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, valueAfter1, valueAfter3)

	// getTransactionCount 4/4 should return the same (fixed) value
	valueAfter4 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, valueAfter1, valueAfter4)

	// getTransactionCount 5 should return the initial value
	valueAfter5 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, txCountBefore, valueAfter5)
}

func TestRelayTx(t *testing.T) {
	resetTestServers()

	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	req_sendRawTransaction := types.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{TestTx_BundleFailedTooManyTimes_RawTx})
	r1 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error)

	// Ensure that request called eth_sendPrivateTransaction with correct param
	require.Equal(t, "eth_sendPrivateTransaction", MockBackendLastJsonRpcRequest.Method)

	resp := MockBackendLastJsonRpcRequest.Params[0].(map[string]interface{})
	require.Equal(t, TestTx_BundleFailedTooManyTimes_RawTx, resp["tx"])

	// Ensure that request was signed properly
	pubkey := crypto.PubkeyToAddress(relaySigningKey.PublicKey).Hex()
	require.Equal(t, pubkey+":0x5adece0bfbb877bf59e8793ce49cee8c9a772e831f57b20409c98d1ac4e3dc800702567414f4045b660c68ca381e369730a19c0b4729491dd43fa5c05247e5a701", MockBackendLastRawRequest.Header.Get("X-Flashbots-Signature"))

	// Check result - should be the tx hash
	var res string
	json.Unmarshal(r1.Result, &res)
	require.Equal(t, TestTx_BundleFailedTooManyTimes_Hash, res)

	timeStampFirstRequest := MockBackendLastJsonRpcRequestTimestamp

	// Send tx again, should not arrive at backend
	sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error)
	require.Equal(t, timeStampFirstRequest, MockBackendLastJsonRpcRequestTimestamp)
}
