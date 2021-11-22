/*
Request represents an incoming client request
*/
package server

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/rpc-endpoint/rpctypes"
	"github.com/google/uuid"
)

// RPC request for a single client JSON-RPC request
type RpcRequest struct {
	respw *http.ResponseWriter
	req   *http.Request

	uid             string
	timeStarted     time.Time
	defaultProxyUrl string
	relayUrl        string
	relaySigningKey *ecdsa.PrivateKey

	// extracted during request lifecycle:
	body     []byte
	jsonReq  *rpctypes.JsonRpcRequest
	ip       string
	rawTxHex string
	tx       *types.Transaction
	txFrom   string

	// response flags
	respHeaderContentTypeWritten bool
	respHeaderStatusCodeWritten  bool
	respBodyWritten              bool
}

func NewRpcRequest(respw *http.ResponseWriter, req *http.Request, proxyUrl string, relayUrl string, relaySigningKey *ecdsa.PrivateKey) *RpcRequest {
	return &RpcRequest{
		respw:           respw,
		req:             req,
		uid:             uuid.New().String(),
		timeStarted:     Now(),
		defaultProxyUrl: proxyUrl,
		relayUrl:        relayUrl,
		relaySigningKey: relaySigningKey,
	}
}

func (r *RpcRequest) log(format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ", r.uid)
	log.Printf(prefix+format, v...)
}

func (r *RpcRequest) logError(format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ERROR: ", r.uid)
	log.Printf(prefix+format, v...)
}

func (r *RpcRequest) writeHeaderStatus(statusCode int) {
	if r.respHeaderStatusCodeWritten {
		return
	}
	r.respHeaderStatusCodeWritten = true
	(*r.respw).WriteHeader(statusCode)
}

func (r *RpcRequest) writeHeaderContentType(contentType string) {
	if r.respHeaderStatusCodeWritten {
		r.logError("writeHeaderContentType failed because status code was already written")
	}
	if r.respHeaderContentTypeWritten {
		return
	}
	r.respHeaderContentTypeWritten = true
	(*r.respw).Header().Set("Content-Type", contentType)
}

func (r *RpcRequest) writeHeaderContentTypeJson() {
	r.writeHeaderContentType("application/json")
}

func (r *RpcRequest) process() {
	var err error

	// At end of request, log the time it needed
	defer func() {
		timeRequestNeeded := time.Since(r.timeStarted)
		r.log("request took %.6f sec", timeRequestNeeded.Seconds())
	}()

	r.ip = GetIP(r.req)
	r.log("POST request from ip: %s - goroutines: %d", r.ip, runtime.NumGoroutine())

	if IsBlacklisted(r.ip) {
		r.log("Blocked IP: %s", r.ip)
		r.writeHeaderStatus(http.StatusUnauthorized)
		return
	}

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	customProxyUrl, ok := r.req.URL.Query()["url"]
	if ok && len(customProxyUrl[0]) > 1 {
		r.defaultProxyUrl = customProxyUrl[0]
		r.log("Using custom url: %s", r.defaultProxyUrl)
	}

	// Decode request JSON RPC
	defer r.req.Body.Close()
	r.body, err = ioutil.ReadAll(r.req.Body)
	if err != nil {
		r.logError("failed to read request body: %v", err)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	if len(r.body) == 0 {
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	// Parse JSON RPC
	if err = json.Unmarshal(r.body, &r.jsonReq); err != nil {
		r.log("failed to parse JSON RPC request: %v - body: %s", err, r.body)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.log("JSON-RPC method: %s ip: %s", r.jsonReq.Method, r.ip)

	if r.jsonReq.Method == "eth_sendRawTransaction" {
		r.handle_sendRawTransaction()

	} else {
		// Normal proxy mode. Check for intercepts
		if r.jsonReq.Method == "eth_getTransactionCount" && r.intercept_mm_eth_getTransactionCount() { // intercept if MM needs to show an error to user
			return
		} else if r.jsonReq.Method == "eth_call" && r.intercept_eth_call_to_FlashRPC_Contract() { // intercept if Flashbots isRPC contract
			return
		} else if r.jsonReq.Method == "net_version" { // don't need to proxy to node, it's always 1 (mainnet)
			r.writeRpcResult("1")
			return
		}

		// Proxy the request to a node
		readJsonRpcSuccess, proxyHttpStatus, jsonResp := r.proxyRequestRead(r.defaultProxyUrl)

		// After proxy, perhaps check backend [MM fix #3 step 2]
		if r.jsonReq.Method == "eth_getTransactionReceipt" {
			requestCompleted := r.check_post_getTransactionReceipt(jsonResp)
			if requestCompleted {
				return
			}
		}

		// Write the response to user
		if readJsonRpcSuccess {
			r.writeHeaderContentTypeJson()
			r.writeHeaderStatus(proxyHttpStatus)
			r._writeRpcResponse(jsonResp)
			r.log("Proxy to node successful: %s", r.jsonReq.Method)
		} else {
			r.writeHeaderStatus(http.StatusInternalServerError)
			r.log("Proxy to node failed: %s", r.jsonReq.Method)
		}
	}
}

func (r *RpcRequest) handle_sendRawTransaction() {
	var err error

	// JSON-RPC sanity checks
	if len(r.jsonReq.Params) < 1 {
		r.logError("no params for eth_sendRawTransaction")
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.rawTxHex = r.jsonReq.Params[0].(string)
	if len(r.rawTxHex) < 2 {
		r.logError("invalid raw transaction (wrong length)")
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.log("rawTx: %s", r.rawTxHex)

	r.tx, err = GetTx(r.rawTxHex)
	if err != nil {
		r.logError("getting transaction object failed")
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	// Get tx from address
	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {
		r.logError("couldn't get address from rawTx: %v", err)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.log("txHash: %s - from: %s / to: %s / nonce: %d / gasPrice: %s", r.tx.Hash(), r.txFrom, AddressPtrToStr(r.tx.To()), r.tx.Nonce(), BigIntPtrToStr(r.tx.GasPrice()))
	txFromLower := strings.ToLower(r.txFrom)

	if r.tx.Nonce() >= 1e9 {
		r.log("tx rejected - nonce too high: %d - %s from %s", r.tx.Nonce(), r.tx.Hash(), txFromLower)
		r.writeRpcError("tx rejected - nonce too high")
		err = RState.DelNonceFixForAccount(txFromLower)
		if err != nil {
			r.logError("redis:DelAccountWithNonceFix failed: %v", err)
		}
		return
	}

	// Remember time when tx was received
	txHashLower := strings.ToLower(r.tx.Hash().Hex())
	err = RState.SetSenderOfTxHash(txHashLower, txFromLower)
	if err != nil {
		r.logError("redis:SetSenderOfTxHash failed: %v", err)
	}

	if isOnOFACList(r.txFrom) {
		r.log("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
		r.writeHeaderStatus(http.StatusUnauthorized)
		return
	}

	// Check if transaction needs protection
	needsProtection := r.doesTxNeedFrontrunningProtection(r.tx)

	// Check for cancellation-tx
	if len(r.tx.Data()) <= 2 && txFromLower == strings.ToLower(r.tx.To().Hex()) {
		requestDone := r.handleCancelTx() // returns true if tx was cancelled at the relay and response has been sent to the user
		if requestDone {
			return
		}

		// It's a cancel-tx for the mempool
		needsProtection = false
		r.log("[cancel-tx] sending to mempool for %s/%d", txFromLower, r.tx.Nonce())
	}

	if needsProtection {
		r.sendTxToRelay()
		return
	}

	if DebugDontSendTx {
		r.log("faked sending tx to mempool, did nothing")
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	// Proxy to public node now
	readJsonRpcSuccess, proxyHttpStatus, jsonResp := r.proxyRequestRead(r.defaultProxyUrl)

	// Log after proxying
	if !readJsonRpcSuccess {
		r.logError("Proxy to mempool failed: eth_sendRawTransaction")
		r.writeHeaderStatus(http.StatusInternalServerError)
		return
	}

	// Write JSON-RPC response now
	r.writeHeaderContentTypeJson()
	r.writeHeaderStatus(proxyHttpStatus)
	r._writeRpcResponse(jsonResp)

	if jsonResp.Error != nil {
		r.log("Proxied eth_sendRawTransaction to mempool - with JSON-RPC Error %s", jsonResp.Error.Message)
	} else {
		r.log("Proxied eth_sendRawTransaction to mempool")
	}
}

// Proxies the incoming request to the target URL, and tries to parse JSON-RPC response (and check for specific)
func (r *RpcRequest) proxyRequestRead(proxyUrl string) (readJsonRpsResponseSuccess bool, httpStatusCode int, jsonResp *rpctypes.JsonRpcResponse) {
	timeProxyStart := Now() // for measuring execution time
	r.log("proxyRequest to: %s", proxyUrl)

	// Proxy request
	proxyResp, err := ProxyRequest(proxyUrl, r.body)
	if err != nil {
		r.logError("failed to make proxy request: %v / resp: %v", err, proxyResp)
		if proxyResp == nil {
			return false, http.StatusInternalServerError, jsonResp
		} else {
			return false, proxyResp.StatusCode, jsonResp
		}
	}

	// Afterwards, check time and result
	timeProxyNeeded := time.Since(timeProxyStart)
	r.log("proxy response %d after %.6f sec", proxyResp.StatusCode, timeProxyNeeded.Seconds())
	// r.log("proxy response %d after %.6f: %v", proxyResp.StatusCode, timeProxyNeeded.Seconds(), proxyResp)

	// Read body
	defer proxyResp.Body.Close()
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		r.logError("failed to decode proxy request body: %v", err)
		return false, proxyResp.StatusCode, jsonResp
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(rpctypes.JsonRpcResponse)
	if err := json.Unmarshal(proxyRespBody, jsonRpcResp); err != nil {
		r.logError("failed decoding proxy json-rpc response: %v", err)
		return false, proxyResp.StatusCode, jsonResp
	}

	return true, proxyResp.StatusCode, jsonRpcResp
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func (r *RpcRequest) doesTxNeedFrontrunningProtection(tx *types.Transaction) bool {
	gas := tx.Gas()
	r.log("[protect-check] gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false
	}

	data := hex.EncodeToString(tx.Data())
	r.log("[protect-check] tx-data: %v", data)

	if len(data) < 8 {
		return false
	}

	if isOnFunctionWhiteList(data[0:8]) {
		return false // function being called is on our whitelist and no protection needed
	} else {
		return true // needs protection if not on whitelist
	}
}

func (r *RpcRequest) writeRpcError(msg string) {
	res := rpctypes.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Error: &rpctypes.JsonRpcError{
			Code:    -32603,
			Message: msg,
		},
	}
	r._writeRpcResponse(&res)
}

func (r *RpcRequest) writeRpcResult(result interface{}) {
	resBytes, err := json.Marshal(result)
	if err != nil {
		r.logError("writeRpcResult error marshalling %s: %s", result, err)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return
	}
	res := rpctypes.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Result:  resBytes,
	}
	r._writeRpcResponse(&res)
}

func (r *RpcRequest) _writeRpcResponse(res *rpctypes.JsonRpcResponse) {
	if r.respBodyWritten {
		r.logError("_writeRpcResponse: response already written")
		return
	}

	if !r.respHeaderContentTypeWritten {
		r.writeHeaderContentTypeJson() // set content type to json, if not yet set
	}

	if !r.respHeaderStatusCodeWritten {
		r.writeHeaderStatus(http.StatusOK) // set status header to 200, if not yet set
	}

	if err := json.NewEncoder(*r.respw).Encode(res); err != nil {
		r.logError("failed writing rpc response: %v", err)
		r.writeHeaderStatus(http.StatusInternalServerError)
	}

	r.respBodyWritten = true
}

// - if sent before, then check API and resend only if not pending
// - if not sent before then send now
func (r *RpcRequest) shouldSendTxToRelay(txHash string) bool {
	timeSent, txWasSentToRelay, err := RState.GetTxSentToRelay(txHash)
	if err != nil {
		r.logError("[shouldSendTxToRelay] redis:GetTxSentToRelay error: %v", err)
		return true
	}

	if !txWasSentToRelay {
		return true
	}

	// was sent before. check status and time
	txStatusApiResponse, err := GetTxStatus(txHash)
	if err != nil {
		r.logError("[shouldSendTxToRelay] GetTxStatus error: %v", err)
		return true
	}

	// Allow sending to relay if tx has failed, or if it's still unknown after a while
	txStatus := rpctypes.PrivateTxStatus(txStatusApiResponse.Status)
	if txStatus == rpctypes.TxStatusFailed {
		return true
	} else if txStatus == rpctypes.TxStatusUnknown && time.Since(timeSent).Minutes() >= 5 {
		return true
	} else {
		// If tx is still pending, or included then don't send it again
		return false
	}
}

// Send tx to relay and finish request (write response)
func (r *RpcRequest) sendTxToRelay() {
	// Improve should-send check:
	txHash := strings.ToLower(r.tx.Hash().Hex())

	// Check if tx was already forwarded and should be blocked now
	if !r.shouldSendTxToRelay(txHash) {
		r.log("[sendTxToRelay] shouldn't send %s", txHash)
		r.writeRpcResult(txHash)
		return
	}

	r.log("[sendTxToRelay] sending %s ...", txHash)

	// mark tx as sent to relay
	err := RState.SetTxSentToRelay(txHash)
	if err != nil {
		r.logError("[sendTxToRelay] redis:SetTxSentToRelay failed: %v", err)
	}

	// remember that this tx based on from+nonce (for cancel-tx)
	err = RState.SetTxHashForSenderAndNonce(r.txFrom, r.tx.Nonce(), txHash)
	if err != nil {
		r.logError("[sendTxToRelay] redis:SetTxHashForSenderAndNonce failed: %v", err)
	}

	// err = RState.SetLastPrivTxHashOfAccount(r.txFrom, txHash)
	// if err != nil {
	// 	r.logError("[sendTxToRelay] redis:SetLastTxHashOfAccount failed: %v", err)
	// }

	if DebugDontSendTx {
		r.log("faked sending tx to relay, did nothing")
		r.writeRpcResult(r.tx.Hash().Hex())
		return
	}

	param := make(map[string]string)
	param["tx"] = r.rawTxHex
	jsonRpcReq := rpctypes.NewJsonRpcRequest1(1, "eth_sendPrivateTransaction", param)
	backendResp, respBytes, err := SendRpcWithSignatureAndParseResponse(r.relayUrl, r.relaySigningKey, jsonRpcReq)
	if err != nil {
		r.logError("[sendTxToRelay] relay call failed for %s: %s - data: %s", txHash, err, *respBytes)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return
	}

	if backendResp.Error != nil {
		r.logError("[sendTxToRelay] relay returned an error for %s: %s", txHash, backendResp.Error.Message)
	}

	r._writeRpcResponse(backendResp)
	r.log("[sendTxToRelay] sent %s", txHash)
}

func (r *RpcRequest) handleCancelTx() (requestCompleted bool) {
	txFromLower := strings.ToLower(r.txFrom)

	// Get original tx hash by sender+nonce
	txHash, txHashFound, err := RState.GetTxHashForSenderAndNonce(txFromLower, r.tx.Nonce())
	if err != nil {
		r.logError("Redis error on isCancelTx: %s", err)
		return false
	}

	if !txHashFound {
		return false
	}

	// Check if tx was sent to relay
	_, txWasSentToRelay, err := RState.GetTxSentToRelay(txHash)
	if err != nil {
		r.logError("Redis error on isCancelTx: %s", err)
		return false
	}

	if !txWasSentToRelay {
		return false
	}

	r.log("[cancel-tx] sending to relay for %s/%d", txFromLower, r.tx.Nonce())

	// TODO: convert cancel-tx to cancelPrivateTransaction
	panic("not implemented")

	// All done, write response
	// r.writeRpcResult(txHash)
	// return true
}
