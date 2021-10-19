/*
 * RPC endpoint tests.
 */
package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flashbots/rpc-endpoint/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var RpcBackendServerUrl string

func init() {
	rpcBackendServer := httptest.NewServer(http.HandlerFunc(RpcBackendHandler))
	fmt.Println("rpc backend:", rpcBackendServer.URL)
	RpcBackendServerUrl = rpcBackendServer.URL

	s := server.NewRpcEndPointServer("", rpcBackendServer.URL, rpcBackendServer.URL)
	rpcEndpointServer := httptest.NewServer(http.HandlerFunc(s.HandleHttpRequest))
	fmt.Println("rpc endpoint:", rpcEndpointServer.URL)
	RpcEndpointUrl = rpcEndpointServer.URL
}

// Test intercepting eth_call for Flashbots RPC contract
func TestMetamaskEthGetTransactionCount(t *testing.T) {
	req_getTransactionCount := newRpcRequest("eth_getTransactionCount", []interface{}{"0xc84edF69E78C0E9dE5ccFE4fB9017F6F7566787f", "latest"})
	res := sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	txCountBefore := res.Result
	assert.NotNil(t, txCountBefore, "getTxCount #0")

	// first sendRawTransaction call: rawTx that triggers the error (creates MM cache entry)
	req_sendRawTransaction := newRpcRequest("eth_sendRawTransaction", []interface{}{RawTxBundleFailedTooManyTimes})
	r1 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.NotNil(t, r1.Error)
	require.Equal(t, "Bundle submitted has already failed too many times", r1.Error.Message)

	// second sendRawTransaction call: is blocked because it's in MM cache
	r2 := sendRpcAndParseResponseOrFailNowAllowRpcError(t, req_sendRawTransaction)
	require.NotNil(t, r2.Error)
	require.Equal(t, "rawTx blocked because bundle failed too many times", r2.Error.Message)

	// Next 4 getTransactionCount calls should return wrong result (to make MM fail the tx)
	res = sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	valueAfter1 := res.Result
	assert.NotEqual(t, txCountBefore, valueAfter1, "getTxCount #1")

	res = sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	valueAfter2 := res.Result
	assert.Equal(t, valueAfter1, valueAfter2, "getTxCount #2")

	res = sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	valueAfter3 := res.Result
	assert.Equal(t, valueAfter1, valueAfter3, "getTxCount #3")

	res = sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	valueAfter4 := res.Result
	assert.Equal(t, valueAfter1, valueAfter4, "getTxCount #4")

	// 5th getTransactionCount should be correct again
	res = sendRpcAndParseResponseOrFailNow(t, req_getTransactionCount)
	valueAfter5 := res.Result
	assert.Equal(t, txCountBefore, valueAfter5, "call #5")
}

// Test intercepting eth_call for Flashbots RPC contract
func TestEthCallIntercept(t *testing.T) {
	// eth_call intercept
	req := newRpcRequest("eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1a",
	}})
	res := sendRpcAndParseResponseOrFailNow(t, req)
	require.Nil(t, res.Error)
	require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", res.Result, "FlashRPC contract - eth_call intercept")

	// eth_call passthrough
	req2 := newRpcRequest("eth_call", []interface{}{map[string]string{
		"from": "0xb60e8dd61c5d32be8058bb8eb970870f07233155",
		"to":   "0xf1a54b0759b58661cea17cff19dd37940a9b5f1b",
	}})
	res2 := sendRpcAndParseResponseOrFailNow(t, req2)
	require.Equal(t, "0x12345", res2.Result, "FlashRPC contract - eth_call passthrough")
}

func TestNetVersionIntercept(t *testing.T) {
	// eth_call intercept
	req := newRpcRequest("net_version", []interface{}{})
	res, err := sendRpcAndParseResponseTo(RpcBackendServerUrl, req)
	require.Nil(t, err, err)
	require.Equal(t, "3", res.Result, "net_version from backend")

	res = sendRpcAndParseResponseOrFailNow(t, req)
	require.Nil(t, res.Error)
	require.Equal(t, "1", res.Result, "net_version intercept")
}

func TestSendBundleResponse(t *testing.T) {
	// should be tx hash
	req_sendRawTransaction := newRpcRequest("eth_sendRawTransaction", []interface{}{"0xf8ac8201018527d064ee00830197f594269616d549d7e8eaa82dfb17028d0b212d11232a80b844a9059cbb000000000000000000000000c5daad04f42f923ed03a4e1e192e9ca9f46a14d50000000000000000000000000000000000000000000000000e92596fd629000025a013838b4bc34c2c3bf77f635cfa8d910e19092f38a8d7326077dbcc05f1f3fab1a06740cde8bdd8c27df60b5dd260f671b2f560e5387a83618a18d0793e17a17e02"})
	r1 := sendRpcAndParseResponseOrFailNow(t, req_sendRawTransaction)
	require.Nil(t, r1.Error, r1.Error)
	require.Equal(t, "0xfc211edc6cfe4de65c8aa654d2bf5fec366486729b5b0867d4a7595f0bb5b6d5", r1.Result)
}
