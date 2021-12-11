/*
Request represents an incoming client request
*/
package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/flashbots/rpc-endpoint/utils"
	"github.com/google/uuid"
	"github.com/metachris/flashbotsrpc"
)

// RPC request for a single client JSON-RPC request
type RpcRequest struct {
	respw *http.ResponseWriter
	req   *http.Request

	uid             string
	timeStarted     time.Time
	defaultProxyUrl string
	relaySigningKey *ecdsa.PrivateKey

	// extracted during request lifecycle:
	origin   string
	ip       string
	body     []byte
	jsonReq  *types.JsonRpcRequest
	rawTxHex string
	tx       *ethtypes.Transaction
	txFrom   string

	// response flags
	respHeaderContentTypeWritten bool
	respHeaderStatusCodeWritten  bool
	respBodyWritten              bool
}

func NewRpcRequest(respw *http.ResponseWriter, req *http.Request, proxyUrl string, relaySigningKey *ecdsa.PrivateKey) *RpcRequest {
	return &RpcRequest{
		respw:           respw,
		req:             req,
		uid:             uuid.New().String(),
		timeStarted:     Now(),
		defaultProxyUrl: proxyUrl,
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

func (r *RpcRequest) process() {
	var err error

	// At end of request, log the time it needed
	defer func() {
		timeRequestNeeded := time.Since(r.timeStarted)
		r.log("request took %.6f sec", timeRequestNeeded.Seconds())
	}()

	r.ip = utils.GetIP(r.req)
	r.origin = r.req.Header.Get("Origin")
	r.log("POST request from ip: %s - origin: %s", r.ip, r.origin)

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
		if !readJsonRpcSuccess {
			r.log("Proxy to node failed: %s", r.jsonReq.Method)
			r.writeHeaderStatus(http.StatusInternalServerError)
			return
		}

		// After proxy, perhaps check backend [MM fix #3 step 2]
		if r.jsonReq.Method == "eth_getTransactionReceipt" {
			requestCompleted := r.check_post_getTransactionReceipt(jsonResp)
			if requestCompleted {
				return
			}
		}

		// Write the response to user
		r.writeHeaderContentTypeJson()
		r.writeHeaderStatus(proxyHttpStatus)
		r._writeRpcResponse(jsonResp)
		r.log("Proxy to node successful: %s", r.jsonReq.Method)
	}
}

// Proxies the incoming request to the target URL, and tries to parse JSON-RPC response (and check for specific)
func (r *RpcRequest) proxyRequestRead(proxyUrl string) (readJsonRpsResponseSuccess bool, httpStatusCode int, jsonResp *types.JsonRpcResponse) {
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
		r.logError("failed to read proxy request body: %v", err)
		return false, proxyResp.StatusCode, jsonResp
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(types.JsonRpcResponse)
	if err := json.Unmarshal(proxyRespBody, jsonRpcResp); err != nil {
		r.logError("failed decoding proxy json-rpc response: %v - data: %s", err, proxyRespBody)
		return false, proxyResp.StatusCode, jsonResp
	}

	return true, proxyResp.StatusCode, jsonRpcResp
}

// Check whether to block resending this tx. Send only if (a) not sent before, (b) sent and status=failed, (c) sent, status=unknown and sent at least 5 min ago
func (r *RpcRequest) blockResendingTxToRelay(txHash string) bool {
	timeSent, txWasSentToRelay, err := RState.GetTxSentToRelay(txHash)
	if err != nil {
		r.logError("[shouldSendTxToRelay] redis:GetTxSentToRelay error: %v", err)
		return false // don't block on redis error
	}

	if !txWasSentToRelay {
		return false // don't block if not sent before
	}

	// was sent before. check status and time
	txStatusApiResponse, err := GetTxStatus(txHash)
	if err != nil {
		r.logError("[shouldSendTxToRelay] GetTxStatus error: %v", err)
		return false // don't block on redis error
	}

	// Allow sending to relay if tx has failed, or if it's still unknown after a while
	txStatus := types.PrivateTxStatus(txStatusApiResponse.Status)
	if txStatus == types.TxStatusFailed {
		return false // don't block if tx failed
	} else if txStatus == types.TxStatusUnknown && time.Since(timeSent).Minutes() >= 5 {
		return false // don't block if unknown and sent at least 5 min ago
	} else {
		// block tx if pending or already included
		return true
	}
}

// Cache tx for later bundling
func (r *RpcRequest) cacheTx() {
	bundleId, ok := r.req.URL.Query()["bundle-id"]
	if ok {
		txHash := strings.ToLower(r.tx.Hash().Hex())
		txHex := r.rawTxHex
		r.log("caching tx to bundle %s txData: %s", bundleId[0], txHex)
		RState.AddTxToBundle(bundleId[0], txHex)
		r.writeRpcResult(txHash)
		return
	}
}

// Send tx to relay and finish request (write response)
func (r *RpcRequest) sendTxToRelay() {
	txHash := strings.ToLower(r.tx.Hash().Hex())

	// Check if tx was already forwarded and should be blocked now
	if r.blockResendingTxToRelay(txHash) {
		r.log("[sendTxToRelay] blocked %s", txHash)
		r.writeRpcResult(txHash)
		return
	}

	r.log("[sendTxToRelay] sending %s ... -- from ip: %s / address: %s", txHash, r.ip, r.txFrom)

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
		r.writeRpcResult(txHash)
		return
	}

	sendPrivTxArgs := flashbotsrpc.FlashbotsSendPrivateTransactionRequest{Tx: r.rawTxHex}
	_, err = FlashbotsRPC.FlashbotsSendPrivateTransaction(r.relaySigningKey, sendPrivTxArgs)
	if err != nil {
		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
			r.log("[sendTxToRelay] %v - rawTx: %s", err, r.rawTxHex)
			r.writeRpcError(err.Error())
		} else {
			r.logError("[sendTxToRelay] relay call failed: %v - rawTx: %s", err, r.rawTxHex)
			r.writeHeaderStatus(http.StatusInternalServerError)
		}
		return
	}

	r.writeRpcResult(txHash)
	r.log("[sendTxToRelay] sent %s", txHash)
}

// Sends cancel-tx to relay as cancelPrivateTransaction, if initial tx was sent there too.
func (r *RpcRequest) handleCancelTx() (requestCompleted bool) {
	cancelTxHash := strings.ToLower(r.tx.Hash().Hex())
	txFromLower := strings.ToLower(r.txFrom)
	r.log("[cancel-tx] %s - check %s/%d", cancelTxHash, txFromLower, r.tx.Nonce())

	// Get initial txHash by sender+nonce
	initialTxHash, txHashFound, err := RState.GetTxHashForSenderAndNonce(txFromLower, r.tx.Nonce())
	if err != nil {
		r.logError("[cancel-tx] redis:GetTxHashForSenderAndNonce failed %v", err)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return true
	}

	if !txHashFound { // not found, send to mempool
		return false
	}

	// Check if initial tx was sent to relay
	_, txWasSentToRelay, err := RState.GetTxSentToRelay(initialTxHash)
	if err != nil {
		r.logError("[cancel-tx] redis:GetTxSentToRelay failed: %s", err)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return true
	}

	if !txWasSentToRelay { // was not sent to relay, send to mempool
		return false
	}

	// Should send cancel-tx to relay. Check if cancel-tx was already sent before
	_, cancelTxAlreadySentToRelay, err := RState.GetTxSentToRelay(cancelTxHash)
	if err != nil {
		r.logError("[cancel-tx] redis:GetTxSentToRelay error: %v", err)
		r.writeHeaderStatus(http.StatusInternalServerError)
		return true
	}

	if cancelTxAlreadySentToRelay { // already sent
		r.writeRpcResult(cancelTxHash)
		return true
	}

	r.log("[cancel-tx] sending to relay: %s for %s/%d", initialTxHash, txFromLower, r.tx.Nonce())

	if DebugDontSendTx {
		r.log("faked sending cancel-tx to relay, did nothing")
		r.writeRpcResult(initialTxHash)
		return true
	}

	cancelPrivTxArgs := flashbotsrpc.FlashbotsCancelPrivateTransactionRequest{TxHash: initialTxHash}
	_, err = FlashbotsRPC.FlashbotsCancelPrivateTransaction(r.relaySigningKey, cancelPrivTxArgs)
	if err != nil {
		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
			// errors could be: 'tx not found', 'tx was already cancelled', 'tx has already expired'
			r.log("[cancel-tx] %v - rawTx: %s", err, r.rawTxHex)
			r.writeRpcError(err.Error())
		} else {
			r.logError("[cancel-tx] relay call failed: %v - rawTx: %s", err, r.rawTxHex)
			r.writeHeaderStatus(http.StatusInternalServerError)
		}
		return true
	}

	r.writeRpcResult(cancelTxHash)
	return true
}
