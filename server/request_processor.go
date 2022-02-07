/*
Request represents an incoming client request
*/
package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"reflect"
	"runtime"
	"strings"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/flashbots/rpc-endpoint/utils"
	"github.com/metachris/flashbotsrpc"
)

type RpcRequest struct {
	logger                     Logger
	jsonReq                    *types.JsonRpcRequest
	jsonRes                    *types.JsonRpcResponse
	rawTxHex                   string
	tx                         *ethtypes.Transaction
	txFrom                     string
	defaultProxyUrl            string
	relaySigningKey            *ecdsa.PrivateKey
	ip                         string
	origin                     string
	isWhitehatBundleCollection bool
	whitehatBundleId           string
}

func NewRpcRequest(logger Logger, jsonReq *types.JsonRpcRequest, defaultProxyUrl string, relaySigningKey *ecdsa.PrivateKey, ip, origin string, isWhitehatBundleCollection bool, whitehatBundleId string) *RpcRequest {
	return &RpcRequest{
		logger:                     logger,
		jsonReq:                    jsonReq,
		defaultProxyUrl:            defaultProxyUrl,
		relaySigningKey:            relaySigningKey,
		ip:                         ip,
		origin:                     origin,
		isWhitehatBundleCollection: isWhitehatBundleCollection,
		whitehatBundleId:           whitehatBundleId,
	}
}

func (r *RpcRequest) ProcessRequest() *types.JsonRpcResponse {
	r.logger.log("JSON-RPC request from ip: %s - method: %s / goroutines: %d", r.ip, r.jsonReq.Method, runtime.NumGoroutine())

	switch {
	case r.jsonReq.Method == "eth_sendRawTransaction":
		r.handle_sendRawTransaction()
	case r.jsonReq.Method == "eth_getTransactionCount" && r.intercept_mm_eth_getTransactionCount(): // intercept if MM needs to show an error to user
	case r.jsonReq.Method == "eth_call" && r.intercept_eth_call_to_FlashRPC_Contract(): // intercept if Flashbots isRPC contract
	case r.jsonReq.Method == "net_version": // don't need to proxy to node, it's always 1 (mainnet)
		r.writeRpcResult("1")
	case r.isWhitehatBundleCollection && r.jsonReq.Method == "eth_getBalance":
		r.writeRpcResult("0x56bc75e2d63100000") // 100 ETH, same as the eth_call SC call above returns
	default:
		if r.isWhitehatBundleCollection && r.jsonReq.Method == "eth_call" {
			r.WhitehatBalanceCheckerRewrite()
		}
		// Proxy the request to a node
		readJsonRpcSuccess := r.proxyRequestRead(r.defaultProxyUrl)
		if !readJsonRpcSuccess {
			r.logger.log("Proxy to node failed: %s", r.jsonReq.Method)
			r.writeRpcError("internal server error", types.JsonRpcInternalError)
			return r.jsonRes
		}

		// After proxy, perhaps check backend [MM fix #3 step 2]
		if r.jsonReq.Method == "eth_getTransactionReceipt" {
			requestCompleted := r.check_post_getTransactionReceipt(r.jsonRes)
			if requestCompleted {
				return r.jsonRes
			}
		}
		r.logger.log("Proxy to node successful: %s", r.jsonReq.Method)
	}
	return r.jsonRes
}

// Proxies the incoming request to the target URL, and tries to parse JSON-RPC response (and check for specific)
func (r *RpcRequest) proxyRequestRead(proxyUrl string) (readJsonRpsResponseSuccess bool) {
	timeProxyStart := Now() // for measuring execution time
	r.logger.log("proxyRequest to: %s", proxyUrl)

	body, err := json.Marshal(r.jsonReq)
	if err != nil {
		r.logger.logError("failed to marshal request before making proxy request: %v", err)
		return false
	}

	// Proxy request
	proxyResp, err := ProxyRequest(proxyUrl, body)
	if err != nil {
		r.logger.logError("failed to make proxy request: %v / resp: %v", err, proxyResp)
		if proxyResp == nil {
			return false
		} else {
			return false
		}
	}

	// Afterwards, check time and result
	timeProxyNeeded := time.Since(timeProxyStart)
	r.logger.log("proxy response %d after %.6f sec", proxyResp.StatusCode, timeProxyNeeded.Seconds())
	// r.logger.log("proxy response %d after %.6f: %v", proxyResp.StatusCode, timeProxyNeeded.Seconds(), proxyResp)

	// Read body
	defer proxyResp.Body.Close()
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		r.logger.logError("failed to read proxy request body: %v", err)
		return false
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(types.JsonRpcResponse)
	if err = json.Unmarshal(proxyRespBody, jsonRpcResp); err != nil {
		r.logger.logError("failed decoding proxy json-rpc response: %v - data: %s", err, proxyRespBody)
		return false
	}
	r.jsonRes = jsonRpcResp
	return true
}

// Check whether to block resending this tx. Send only if (a) not sent before, (b) sent and status=failed, (c) sent, status=unknown and sent at least 5 min ago
func (r *RpcRequest) blockResendingTxToRelay(txHash string) bool {
	timeSent, txWasSentToRelay, err := RState.GetTxSentToRelay(txHash)
	if err != nil {
		r.logger.logError("[shouldSendTxToRelay] redis:GetTxSentToRelay error: %v", err)
		return false // don't block on redis error
	}

	if !txWasSentToRelay {
		return false // don't block if not sent before
	}

	// was sent before. check status and time
	txStatusApiResponse, err := GetTxStatus(txHash)
	if err != nil {
		r.logger.logError("[shouldSendTxToRelay] GetTxStatus error: %v", err)
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

// Send tx to relay and finish request (write response)
func (r *RpcRequest) sendTxToRelay() {
	txHash := strings.ToLower(r.tx.Hash().Hex())

	// Check if tx was already forwarded and should be blocked now
	if r.blockResendingTxToRelay(txHash) {
		r.logger.log("[sendTxToRelay] blocked %s", txHash)
		r.writeRpcResult(txHash)
		return
	}

	r.logger.log("[sendTxToRelay] sending %s -- from ip: %s / address: %s / to: %s", txHash, r.ip, r.txFrom, r.tx.To())

	// mark tx as sent to relay
	err := RState.SetTxSentToRelay(txHash)
	if err != nil {
		r.logger.logError("[sendTxToRelay] redis:SetTxSentToRelay failed: %v", err)
	}

	txTo := r.tx.To()
	if txTo == nil {
		r.writeRpcError("invalid target", types.JsonRpcInternalError)
		return
	}

	minNonce, maxNonce := r.GetAddressNonceRange(r.txFrom)
	if r.tx.Nonce() < minNonce || r.tx.Nonce() > maxNonce+1 {
		r.logger.logError("[sendTxToRelay] invalid nonce for %s from %s - want: [%d, %d], got: %d", txHash, r.txFrom, minNonce, maxNonce+1, r.tx.Nonce())
		r.writeRpcError("invalid nonce", types.JsonRpcInternalError)
		return
	}

	go RState.SetSenderMaxNonce(r.txFrom, r.tx.Nonce())

	// only allow large transactions to certain addresses - default max tx size is 128KB
	// https://github.com/ethereum/go-ethereum/blob/master/core/tx_pool.go#L53
	if r.tx.Size() > 131072 {
		if _, found := allowedLargeTxTargets[txTo.Hex()]; !found {
			r.logger.logError("sendTxToRelay] large tx to not allowed target - hash: %s - target: %s", txHash, txTo)
			r.writeRpcError("invalid target for large tx", types.JsonRpcInternalError)
			return
		}
		r.logger.log("sendTxToRelay] allowed large tx - hash: %s - target: %s", txHash, txTo)
	}

	// remember this tx based on from+nonce (for cancel-tx)
	err = RState.SetTxHashForSenderAndNonce(r.txFrom, r.tx.Nonce(), txHash)
	if err != nil {
		r.logger.logError("[sendTxToRelay] redis:SetTxHashForSenderAndNonce failed: %v", err)
	}

	// err = RState.SetLastPrivTxHashOfAccount(r.txFrom, txHash)
	// if err != nil {
	// 	r.logError("[sendTxToRelay] redis:SetLastTxHashOfAccount failed: %v", err)
	// }

	if DebugDontSendTx {
		r.logger.log("faked sending tx to relay, did nothing")
		r.writeRpcResult(txHash)
		return
	}

	sendPrivTxArgs := flashbotsrpc.FlashbotsSendPrivateTransactionRequest{Tx: r.rawTxHex}
	_, err = FlashbotsRPC.FlashbotsSendPrivateTransaction(r.relaySigningKey, sendPrivTxArgs)
	if err != nil {
		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
			r.logger.log("[sendTxToRelay] %v - rawTx: %s", err, r.rawTxHex)
			r.writeRpcError(err.Error(), types.JsonRpcInternalError)
		} else {
			r.logger.logError("[sendTxToRelay] relay call failed: %v - rawTx: %s", err, r.rawTxHex)
			r.writeRpcError(err.Error(), types.JsonRpcInternalError)
		}
		return
	}

	r.writeRpcResult(txHash)
	r.logger.log("[sendTxToRelay] sent %s", txHash)
}

// Sends cancel-tx to relay as cancelPrivateTransaction, if initial tx was sent there too.
func (r *RpcRequest) handleCancelTx() (requestCompleted bool) {
	cancelTxHash := strings.ToLower(r.tx.Hash().Hex())
	txFromLower := strings.ToLower(r.txFrom)
	r.logger.log("[cancel-tx] %s - check %s/%d", cancelTxHash, txFromLower, r.tx.Nonce())

	// Get initial txHash by sender+nonce
	initialTxHash, txHashFound, err := RState.GetTxHashForSenderAndNonce(txFromLower, r.tx.Nonce())
	if err != nil {
		r.logger.logError("[cancel-tx] redis:GetTxHashForSenderAndNonce failed %v", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return true
	}

	if !txHashFound { // not found, send to mempool
		return false
	}

	// Check if initial tx was sent to relay
	_, txWasSentToRelay, err := RState.GetTxSentToRelay(initialTxHash)
	if err != nil {
		r.logger.logError("[cancel-tx] redis:GetTxSentToRelay failed: %s", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return true
	}

	if !txWasSentToRelay { // was not sent to relay, send to mempool
		return false
	}

	// Should send cancel-tx to relay. Check if cancel-tx was already sent before
	_, cancelTxAlreadySentToRelay, err := RState.GetTxSentToRelay(cancelTxHash)
	if err != nil {
		r.logger.logError("[cancel-tx] redis:GetTxSentToRelay error: %v", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return true
	}

	if cancelTxAlreadySentToRelay { // already sent
		r.writeRpcResult(cancelTxHash)
		return true
	}

	r.logger.log("[cancel-tx] sending to relay: %s for %s/%d", initialTxHash, txFromLower, r.tx.Nonce())

	if DebugDontSendTx {
		r.logger.log("faked sending cancel-tx to relay, did nothing")
		r.writeRpcResult(initialTxHash)
		return true
	}

	cancelPrivTxArgs := flashbotsrpc.FlashbotsCancelPrivateTransactionRequest{TxHash: initialTxHash}
	_, err = FlashbotsRPC.FlashbotsCancelPrivateTransaction(r.relaySigningKey, cancelPrivTxArgs)
	if err != nil {
		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
			// errors could be: 'tx not found', 'tx was already cancelled', 'tx has already expired'
			r.logger.log("[cancel-tx] %v - rawTx: %s", err, r.rawTxHex)
			r.writeRpcError(err.Error(), types.JsonRpcInternalError)
		} else {
			r.logger.logError("[cancel-tx] relay call failed: %v - rawTx: %s", err, r.rawTxHex)
			r.writeRpcError("internal server error", types.JsonRpcInternalError)
		}
		return true
	}

	r.writeRpcResult(cancelTxHash)
	return true
}

func (r *RpcRequest) GetAddressNonceRange(address string) (minNonce, maxNonce uint64) {
	// Get minimum nonce by asking the eth node for the current transaction count
	_req := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{r.txFrom, "latest"})
	_res, err := utils.SendRpcAndParseResponseTo(r.defaultProxyUrl, _req)
	if err != nil {
		r.logger.logError("[sendTxToRelay] eth_getTransactionCount failed: %v", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}
	_userNonceStr := ""
	err = json.Unmarshal(_res.Result, &_userNonceStr)
	if err != nil {
		r.logger.logError("[sendTxToRelay] eth_getTransactionCount unmarshall failed: %v - result: %s", err, _res.Result)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}
	_userNonceStr = strings.Replace(_userNonceStr, "0x", "", 1)
	_userNonceBigInt := new(big.Int)
	_userNonceBigInt.SetString(_userNonceStr, 16)
	minNonce = _userNonceBigInt.Uint64()

	// Get maximum nonce by looking at redis, which has current pending transactions
	_redisMaxNonce, _, _ := RState.GetSenderMaxNonce(r.txFrom)
	maxNonce = Max(minNonce, _redisMaxNonce)
	return minNonce, maxNonce
}

func (r *RpcRequest) WhitehatBalanceCheckerRewrite() {
	var err error

	if len(r.jsonReq.Params) == 0 {
		return
	}

	// Ensure param is of type map
	t := reflect.TypeOf(r.jsonReq.Params[0])
	if t.Kind() != reflect.Map {
		return
	}

	p := r.jsonReq.Params[0].(map[string]interface{})
	if to := p["to"]; to == "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39" {
		r.jsonReq.Params[0].(map[string]interface{})["to"] = "0x268F7Cd7A396BCE178f0937095772C7fb83a9104"
		if err != nil {
			r.logger.logError("isWhitehatBundleCollection json marshal failed:", err)
		} else {
			r.logger.log("BalanceChecker contract was rewritten to new version")
		}
	}
}

func (r *RpcRequest) writeRpcError(msg string, errCode int) {
	r.jsonRes = &types.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Error: &types.JsonRpcError{
			Code:    errCode,
			Message: msg,
		},
	}

}

func (r *RpcRequest) writeRpcResult(result interface{}) {
	resBytes, err := json.Marshal(result)
	if err != nil {
		r.logger.logError("writeRpcResult error marshalling %s: %s", result, err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}
	r.jsonRes = &types.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Result:  resBytes,
	}
}
