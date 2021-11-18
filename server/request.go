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
	"github.com/google/uuid"
)

// RPC request for a single client JSON-RPC request
type RpcRequest struct {
	respw *http.ResponseWriter
	req   *http.Request

	uid             string
	timeStarted     time.Time
	defaultProxyUrl string
	txManagerUrl    string
	relayUrl        string
	useRelay        bool
	relaySigningKey *ecdsa.PrivateKey

	// extracted during request lifecycle:
	body     []byte
	jsonReq  *JsonRpcRequest
	ip       string
	rawTxHex string
	tx       *types.Transaction
	txFrom   string

	// response flags
	respHeaderContentTypeWritten bool
	respHeaderStatusCodeWritten  bool
	respBodyWritten              bool
}

func NewRpcRequest(respw *http.ResponseWriter, req *http.Request, proxyUrl string, txManagerUrl string, relayUrl string, useRelay bool, relaySigningKey *ecdsa.PrivateKey) *RpcRequest {
	return &RpcRequest{
		respw:           respw,
		req:             req,
		uid:             uuid.New().String(),
		timeStarted:     Now(),
		defaultProxyUrl: proxyUrl,
		txManagerUrl:    txManagerUrl,
		relayUrl:        relayUrl,
		useRelay:        useRelay,
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

	// Parse JSON RPC
	if err = json.Unmarshal(r.body, &r.jsonReq); err != nil {
		r.logError("failed to parse JSON RPC request: %v - body: %s", err, r.body)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	r.log("JSON-RPC method: %s ip: %s", r.jsonReq.Method, r.ip)
	MetaMaskFix.CleanupStaleEntries()

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
			r.check_post_getTransactionReceipt(jsonResp)
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

	txHashLower := strings.ToLower(r.tx.Hash().Hex())

	if _, isBlacklistedTx := MetaMaskFix.blacklistedRawTx[txHashLower]; isBlacklistedTx {
		r.log("tx blocked - is on metamask-fix-blacklist")
		r.writeRpcError("rawTx blocked")
		return
	}

	// Get tx from address
	r.txFrom, err = GetSenderFromRawTx(r.tx)
	if err != nil {
		r.logError("couldn't get address from rawTx: %v", err)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}
	r.log("txHash: %s - from: %s", r.tx.Hash(), r.txFrom)

	if r.tx.Nonce() >= 1e9 {
		r.log("tx blocked - nonce too high: %d", r.tx.Nonce())
		r.writeRpcError("tx rejected - nonce too high")
		return
	}

	// Remember time when tx was received
	if _, found := MetaMaskFix.rawTransactionSubmission[txHashLower]; !found {
		MetaMaskFix.rawTransactionSubmission[txHashLower] = &mmRawTxTracker{
			submittedAt: Now(),
			tx:          r.tx,
			txFrom:      r.txFrom,
		}
	}

	if isOnOFACList(r.txFrom) {
		r.log("BLOCKED TX FROM OFAC SANCTIONED ADDRESS")
		r.writeHeaderStatus(http.StatusUnauthorized)
		return
	}

	needsProtection, err := r.doesTxNeedFrontrunningProtection(r.tx)
	if err != nil {
		r.logError("failed to evaluate transaction: %v", err)
		r.writeHeaderStatus(http.StatusBadRequest)
		return
	}

	target := "mempool"
	url := r.defaultProxyUrl
	if needsProtection {
		if r.useRelay {
			r.sendTxToRelay()
			return
		}

		target = "TxManager"
		url = r.txManagerUrl
	}

	// Proxy now!
	readJsonRpcSuccess, proxyHttpStatus, jsonResp := r.proxyRequestRead(url)

	// Log after proxying
	if !readJsonRpcSuccess {
		r.log("Proxy to %s failed: eth_sendRawTransaction", target)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return
	}

	// Write JSON-RPC response now
	r.writeHeaderContentTypeJson()
	r.writeHeaderStatus(proxyHttpStatus)
	if jsonResp.Error != nil {
		r.log("Proxy to %s successful: eth_sendRawTransaction - with JSON-RPC Error %s", target, jsonResp.Error.Message)

		// write the original response to the user
		r._writeRpcResponse(jsonResp)
		return
	} else {
		// TxManager returns bundle hash, but this call needs to return tx hash
		txHash := r.tx.Hash().Hex()
		r.writeRpcResult(txHash)
		r.log("Proxy to %s successful: eth_sendRawTransaction - tx-hash: %s", target, txHash)
		return
	}
}

// Proxies the incoming request to the target URL, and tries to parse JSON-RPC response (and check for specific)
func (r *RpcRequest) proxyRequestRead(proxyUrl string) (readJsonRpsResponseSuccess bool, httpStatusCode int, jsonResp *JsonRpcResponse) {
	timeProxyStart := Now() // for measuring execution time
	r.log("proxyRequest to: %s", proxyUrl)

	// Proxy request
	proxyResp, err := ProxyRequest(proxyUrl, r.body)
	if err != nil {
		r.logError("failed to make proxy request: %v", err)
		return false, proxyResp.StatusCode, jsonResp
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
	jsonRpcResp := new(JsonRpcResponse)
	if err := json.Unmarshal(proxyRespBody, jsonRpcResp); err != nil {
		r.logError("failed decoding proxy json-rpc response: %v", err)
		return false, proxyResp.StatusCode, jsonResp
	}

	// If JSON-RPC had an error response, parse but still pass back to user
	if jsonRpcResp.Error != nil {
		r.handleProxyError(jsonRpcResp.Error)
	}

	return true, proxyResp.StatusCode, jsonRpcResp
}

func (r *RpcRequest) handleProxyError(rpcError *JsonRpcError) {
	r.log("proxy response json-rpc error: %s", rpcError.Message)

	if rpcError.Message == "Bundle submitted has already failed too many times" {
		MetaMaskFix.blacklistedRawTx[strings.ToLower(r.tx.Hash().Hex())] = Now()
		r.log("rawTx with hash %s added to blocklist. entries: %d", r.tx.Hash().Hex(), len(MetaMaskFix.blacklistedRawTx))

		// fmt.Println("NONCE", nonce, "for", r.txFrom)
		MetaMaskFix.accountAndNonce[strings.ToLower(r.txFrom)] = &mmNonceHelper{
			Nonce: 1e9,
		}
	}
}

// Check if a request needs frontrunning protection. There are many transactions that don't need frontrunning protection,
// for example simple ERC20 transfers.
func (r *RpcRequest) doesTxNeedFrontrunningProtection(tx *types.Transaction) (bool, error) {
	gas := tx.Gas()
	r.log("[protect-check] gas: %v", gas)

	// Flashbots Relay will reject anything less than 42000 gas, so we just send those to the mempool
	// Anyway things with that low of gas probably don't need frontrunning protection regardless
	if gas < 42000 {
		return false, nil
	}

	data := hex.EncodeToString(tx.Data())
	r.log("[protect-check] tx-data: %v", data)
	if len(data) == 0 {
		r.log("[protect-check] data had a length of 0, but a gas greater than 21000. Sending cancellation tx to mempool.")
		return false, nil
	}

	if isOnFunctionWhiteList(data[0:8]) {
		return false, nil // function being called is on our whitelist and no protection needed
	} else {
		return true, nil // needs protection if not on whitelist
	}
}

func (r *RpcRequest) writeRpcError(msg string) {
	res := JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Error: &JsonRpcError{
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
	res := JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Result:  resBytes,
	}
	r._writeRpcResponse(&res)
}

func (r *RpcRequest) _writeRpcResponse(res *JsonRpcResponse) {
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

// Send tx to relay and finish request (write response)
func (r *RpcRequest) sendTxToRelay() {
	cleanupOldRelayForwardings() // forwards should only be blocked for a specific time

	// Check if tx was already forwarded and should be blocked now
	txHash := r.tx.Hash().Hex()
	if _, wasAlreadyForwarded := txForwardedToRelay[txHash]; wasAlreadyForwarded {
		r.log("[sendTxToRelay] already sent %s", txHash)
		r.writeRpcResult(txHash)
		return
	}

	r.log("[sendTxToRelay] sending %s", txHash)
	txForwardedToRelay[txHash] = Now()

	param := make(map[string]string)
	param["tx"] = r.rawTxHex
	jsonRpcReq := NewJsonRpcRequest1(1, "eth_sendPrivateTransaction", param)
	backendResp, respBytes, err := SendRpcWithSignatureAndParseResponse(r.relayUrl, r.relaySigningKey, jsonRpcReq)
	if err != nil {
		r.logError("[sendTxToRelay] failed for %s: %s - data: %s", txHash, err, *respBytes)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return
	}

	if backendResp.Error != nil {
		r.logError("[sendTxToRelay] failed for %s (BE error): %s", txHash, backendResp.Error.Message)
		r.handleProxyError(backendResp.Error)
	}

	r._writeRpcResponse(backendResp)
	r.log("[sendTxToRelay] done")
}
