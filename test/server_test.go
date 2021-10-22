/*
 * RPC endpoint tests.
 */
package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flashbots/rpc-endpoint/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var RpcBackendServerUrl string

func resetTestServers() {
	// Create a fresh mock backend server
	rpcBackendServer := httptest.NewServer(http.HandlerFunc(RpcBackendHandler))
	RpcBackendServerUrl = rpcBackendServer.URL

	// Create a fresh RPC endpoint server
	s := server.NewRpcEndPointServer("", rpcBackendServer.URL, rpcBackendServer.URL)
	rpcEndpointServer := httptest.NewServer(http.HandlerFunc(s.HandleHttpRequest))
	RpcEndpointUrl = rpcEndpointServer.URL

	// Reset the metamask fixer
	server.MetaMaskFix = server.NewMetaMaskFixer()
}

func init() {
	resetTestServers()
}

/*
 * HTTP TESTS
 */
// Check headers: status and content-type
func TestStandardHeaders(t *testing.T) {
	rpcRequest := server.NewJsonRpcRequest(1, "null", nil)
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
	_id1 := float64(84363)
	rpcRequest := server.NewJsonRpcRequest(_id1, "null", nil)
	rpcResult := sendRpcAndParseResponseOrFailNow(t, rpcRequest)
	assert.Equal(t, _id1, rpcResult.Id)

	_id2 := "84363"
	rpcRequest2 := server.NewJsonRpcRequest(_id2, "null", nil)
	rpcResult2 := sendRpcAndParseResponseOrFailNow(t, rpcRequest2)
	assert.Equal(t, _id2, rpcResult2.Id)
	assert.Equal(t, "2.0", rpcResult2.Version)

}

/*
 * REQUEST TESTS
 */
// Test intercepting eth_call for Flashbots RPC contract
func TestMetamaskEthGetTransactionCount(t *testing.T) {
	req_getTransactionCount := server.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{TestTx_BundleFailedTooManyTimes_From, "latest"})
	txCountBefore := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)

	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := server.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{TestTx_BundleFailedTooManyTimes_RawTx})
	r1 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.NotNil(t, r1.Error)
	require.Equal(t, "Bundle submitted has already failed too many times", r1.Error.Message)

	// second sendRawTransaction call: is blocked because it's in MM cache
	r2 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.NotNil(t, r2.Error)
	require.Equal(t, "rawTx blocked because bundle failed too many times", r2.Error.Message)

	// Next 4 getTransactionCount calls should return wrong result (to make MM fail the tx)
	valueAfter1 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.NotEqual(t, txCountBefore, valueAfter1, "getTxCount #1")
	require.Equal(t, "0x3b9aca01", valueAfter1, "getTxCount #1")

	valueAfter2 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	assert.Equal(t, valueAfter1, valueAfter2, "getTxCount #2")

	valueAfter3 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	assert.Equal(t, valueAfter1, valueAfter3, "getTxCount #3")

	valueAfter4 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	assert.Equal(t, valueAfter1, valueAfter4, "getTxCount #4")

	// 5th getTransactionCount should be correct again
	valueAfter5 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	assert.Equal(t, txCountBefore, valueAfter5, "call #5")
}

// Test intercepting eth_call for Flashbots RPC contract
func TestEthCallIntercept(t *testing.T) {
	var rpcResult string

	// eth_call intercept
	req := server.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	rpcResult = sendRpcAndParseResponseOrFailNowString(t, req)
	require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", rpcResult, "FlashRPC contract - eth_call intercept")

	// eth_call passthrough
	req2 := server.NewJsonRpcRequest(1, "eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	rpcResult = sendRpcAndParseResponseOrFailNowString(t, req2)
	require.Equal(t, "0x12345", rpcResult, "FlashRPC contract - eth_call passthrough")
}

func TestNetVersionIntercept(t *testing.T) {
	var rpcResult string

	// eth_call intercept
	req := server.NewJsonRpcRequest(1, "net_version", nil)
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
	// should be tx hash
	req_sendRawTransaction := server.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{"0xf8ac8201018527d064ee00830197f594269616d549d7e8eaa82dfb17028d0b212d11232a80b844a9059cbb000000000000000000000000c5daad04f42f923ed03a4e1e192e9ca9f46a14d50000000000000000000000000000000000000000000000000e92596fd629000025a013838b4bc34c2c3bf77f635cfa8d910e19092f38a8d7326077dbcc05f1f3fab1a06740cde8bdd8c27df60b5dd260f671b2f560e5387a83618a18d0793e17a17e02"})
	rpcResult := sendRpcAndParseResponseOrFailNowString(t, req_sendRawTransaction)
	require.Equal(t, "0xfc211edc6cfe4de65c8aa654d2bf5fec366486729b5b0867d4a7595f0bb5b6d5", rpcResult)
}

func TestNull(t *testing.T) {
	expectedResultRaw := `{"id":1,"result":null,"jsonrpc":"2.0"}` + "\n"

	// Build and do RPC call: "null"
	rpcRequest := server.NewJsonRpcRequest(1, "null", nil)
	jsonData, err := json.Marshal(rpcRequest)
	require.Nil(t, err, err)
	resp, err := http.Post(RpcBackendServerUrl, "application/json", bytes.NewBuffer(jsonData))
	require.Nil(t, err, err)
	respData, err := ioutil.ReadAll(resp.Body)
	require.Nil(t, err, err)

	// Check that raw result is expected
	require.Equal(t, expectedResultRaw, string(respData))

	// Parsing null results in "null":
	var jsonRpcResp server.JsonRpcResponse
	err = json.Unmarshal(respData, &jsonRpcResp)
	require.Nil(t, err, err)

	require.Equal(t, 4, len(jsonRpcResp.Result))
	require.Equal(t, json.RawMessage{110, 117, 108, 108}, jsonRpcResp.Result) // the bytes for null

	// Double-check that plain bytes are 'null'
	resultStr := string(jsonRpcResp.Result)
	require.Equal(t, "null", resultStr)
}

func TestGetTxReceiptNull(t *testing.T) {
	req_getTransactionCount := server.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{TestTx_BundleFailedTooManyTimes_Hash})
	jsonResp := sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	fmt.Println(jsonResp)
	require.Equal(t, "null", string(jsonResp.Result))

	jsonResp, err := server.SendRpcAndParseResponseTo(RpcBackendServerUrl, req_getTransactionCount)
	require.Nil(t, err, err)

	fmt.Println(jsonResp)
	require.Equal(t, "null", string(jsonResp.Result))
}

func TestMetamaskFix2WithBlacklist(t *testing.T) {
	resetTestServers()

	req_getTransactionCount := server.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{TestTx_MM2_From, "latest"})
	txCountBefore := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)

	// set the clock back 14 min, to predate entries
	setServerTimeOffset(-14 * time.Minute)

	// sendRawTransaction adds tx to MM cache entry, to be used at later eth_getTransactionReceipt call
	req_sendRawTransaction := server.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{TestTx_MM2_RawTx})
	r1 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error)
	fmt.Printf("\n\n\n")

	// Set the clock to normal, so that more than 16 minutes have passed and we can trigger the codepath for RPC to query the backend
	setServerTimeOffset(0)
	req_getTransactionReceipt := server.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{TestTx_MM2_Hash})
	jsonResp := sendRpcAndParseResponseOrFailNow(t, req_getTransactionReceipt)
	require.Equal(t, "null", string(jsonResp.Result))
	fmt.Printf("\n\n\n")

	// At this point, too high a nonce should be returned
	valueAfter1 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.NotEqual(t, txCountBefore, valueAfter1, "getTxCount #1")
}

func setServerTimeOffset(td time.Duration) {
	server.Now = func() time.Time {
		return time.Now().Add(td)
	}
}

// If getTxReceipt call is made within 16 minutes, no blacklisting occurs
func TestMetamaskFix2WithoutBlacklist(t *testing.T) {
	resetTestServers()

	req_getTransactionCount := server.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{TestTx_MM2_From, "latest"})
	txCountBefore := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)

	setServerTimeOffset(-13 * time.Minute)

	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := server.NewJsonRpcRequest(1, "eth_sendRawTransaction", []interface{}{TestTx_MM2_RawTx})
	r1 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.Nil(t, r1.Error, r1.Error)
	fmt.Printf("\n\n\n\n\n")

	// Set the clock to normal, so that more than 16 minutes have passed and we can trigger
	setServerTimeOffset(0)

	req_getTransactionReceipt := server.NewJsonRpcRequest(1, "eth_getTransactionReceipt", []interface{}{TestTx_MM2_Hash})
	jsonResp := sendRpcAndParseResponseOrFailNow(t, req_getTransactionReceipt)
	_ = jsonResp
	require.Equal(t, "null", string(jsonResp.Result))

	// At this point, the tx hash should be blacklisted and too high a nonce is returned
	valueAfter1 := sendRpcAndParseResponseOrFailNowString(t, req_getTransactionCount)
	require.Equal(t, txCountBefore, valueAfter1, "getTxCount #1")
}
