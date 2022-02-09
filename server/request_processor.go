/*
Request represents an incoming client request
*/
package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/log"
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
	logger                     log.Logger
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

func NewRpcRequest(logger log.Logger, jsonReq *types.JsonRpcRequest, defaultProxyUrl string, relaySigningKey *ecdsa.PrivateKey, ip, origin string, isWhitehatBundleCollection bool, whitehatBundleId string) *RpcRequest {
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
	r.logger.Info("JSON-RPC request", "ip", r.ip, "method", r.jsonReq.Method, "goroutines", runtime.NumGoroutine())

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
			r.logger.Info("[ProcessRequest] Proxy to node failed", "method", r.jsonReq.Method)
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
		r.logger.Info("[ProcessRequest] Proxy to node successful", "method", r.jsonReq.Method)
	}
	return r.jsonRes
}

// Proxies the incoming request to the target URL, and tries to parse JSON-RPC response (and check for specific)
func (r *RpcRequest) proxyRequestRead(proxyUrl string) (readJsonRpsResponseSuccess bool) {
	timeProxyStart := Now() // for measuring execution time
	r.logger.Info("[proxyRequestRead] ProxyRequest to", "url", proxyUrl)

	body, err := json.Marshal(r.jsonReq)
	if err != nil {
		r.logger.Error("[proxyRequestRead] Failed to marshal request before making proxy request", "error", err)
		return false
	}

	// Proxy request
	proxyResp, err := ProxyRequest(proxyUrl, body)
	if err != nil {
		r.logger.Error("[proxyRequestRead] Failed to make proxy request", "error", err, "response", proxyResp)
		if proxyResp == nil {
			return false
		} else {
			return false
		}
	}

	// Afterwards, check time and result
	timeProxyNeeded := time.Since(timeProxyStart)
	r.logger.Info("[proxyRequestRead] proxied response", "statusCode", proxyResp.StatusCode, "secNeeded", timeProxyNeeded.Seconds())

	// Read body
	defer proxyResp.Body.Close()
	proxyRespBody, err := ioutil.ReadAll(proxyResp.Body)
	if err != nil {
		r.logger.Error("[proxyRequestRead] Failed to read proxy request body", "error", err)
		return false
	}

	// Unmarshall JSON-RPC response and check for error inside
	jsonRpcResp := new(types.JsonRpcResponse)
	if err = json.Unmarshal(proxyRespBody, jsonRpcResp); err != nil {
		r.logger.Error("[proxyRequestRead] Failed decoding proxy json-rpc response", "error", err, "response", proxyRespBody)
		return false
	}
	r.jsonRes = jsonRpcResp
	return true
}

// Check whether to block resending this tx. Send only if (a) not sent before, (b) sent and status=failed, (c) sent, status=unknown and sent at least 5 min ago
func (r *RpcRequest) blockResendingTxToRelay(txHash string) bool {
	timeSent, txWasSentToRelay, err := RState.GetTxSentToRelay(txHash)
	if err != nil {
		r.logger.Error("[blockResendingTxToRelay] Redis:GetTxSentToRelay error", "error", err)
		return false // don't block on redis error
	}

	if !txWasSentToRelay {
		return false // don't block if not sent before
	}

	// was sent before. check status and time
	txStatusApiResponse, err := GetTxStatus(txHash)
	if err != nil {
		r.logger.Error("[blockResendingTxToRelay] GetTxStatus error", "error", err)
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
		r.logger.Info("[sendTxToRelay] Blocked", "tx", txHash)
		r.writeRpcResult(txHash)
		return
	}

	r.logger.Info("[sendTxToRelay] sending transaction to relay", "tx", txHash, "ip", r.ip, "fromAddress", r.txFrom, "toAddress", r.tx.To())

	// mark tx as sent to relay
	err := RState.SetTxSentToRelay(txHash)
	if err != nil {
		r.logger.Error("[sendTxToRelay] Redis:SetTxSentToRelay failed", "error", err)
	}

	txTo := r.tx.To()
	if txTo == nil {
		r.writeRpcError("invalid target", types.JsonRpcInternalError)
		return
	}

	minNonce, maxNonce, err := r.GetAddressNonceRange(r.txFrom)
	if err != nil {
		r.logger.Error("[sendTxToRelay] GetAddressNonceRange error", "error", err)
	} else {
		if r.tx.Nonce() < minNonce || r.tx.Nonce() > maxNonce+1 {
			r.logger.Error("[sendTxToRelay] invalid nonce", "tx", txHash, "txFrom", r.txFrom, "minNonce", minNonce, "maxNonce", maxNonce+1, "txNonce", r.tx.Nonce())
			r.writeRpcError("invalid nonce", types.JsonRpcInternalError)
			return
		}
	}

	go RState.SetSenderMaxNonce(r.txFrom, r.tx.Nonce())

	// only allow large transactions to certain addresses - default max tx size is 128KB
	// https://github.com/ethereum/go-ethereum/blob/master/core/tx_pool.go#L53
	if r.tx.Size() > 131072 {
		if _, found := allowedLargeTxTargets[txTo.Hex()]; !found {
			r.logger.Error("[sendTxToRelay] large tx not allowed to target", "tx", txHash, "target", txTo)
			r.writeRpcError("invalid target for large tx", types.JsonRpcInternalError)
			return
		}
		r.logger.Info("sendTxToRelay] allowed large tx", "tx", txHash, "target", txTo)
	}

	// remember this tx based on from+nonce (for cancel-tx)
	err = RState.SetTxHashForSenderAndNonce(r.txFrom, r.tx.Nonce(), txHash)
	if err != nil {
		r.logger.Error("[sendTxToRelay] Redis:SetTxHashForSenderAndNonce failed", "error", err)
	}

	// err = RState.SetLastPrivTxHashOfAccount(r.txFrom, txHash)
	// if err != nil {
	// 	r.Error("[sendTxToRelay] redis:SetLastTxHashOfAccount failed: %v", err)
	// }

	if DebugDontSendTx {
		r.logger.Info("[sendTxToRelay] Faked sending tx to relay, did nothing", "tx", txHash)
		r.writeRpcResult(txHash)
		return
	}

	sendPrivTxArgs := flashbotsrpc.FlashbotsSendPrivateTransactionRequest{Tx: r.rawTxHex}
	_, err = FlashbotsRPC.FlashbotsSendPrivateTransaction(r.relaySigningKey, sendPrivTxArgs)
	if err != nil {
		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
			r.logger.Info("[sendTxToRelay] Relay error response", "error", err, "rawTx", r.rawTxHex)
			r.writeRpcError(err.Error(), types.JsonRpcInternalError)
		} else {
			r.logger.Error("[sendTxToRelay] Relay call failed", "error", err, "rawTx", r.rawTxHex)
			r.writeRpcError(err.Error(), types.JsonRpcInternalError)
		}
		return
	}

	r.writeRpcResult(txHash)
	r.logger.Info("[sendTxToRelay] Sent", "tx", txHash)
}

// Sends cancel-tx to relay as cancelPrivateTransaction, if initial tx was sent there too.
func (r *RpcRequest) handleCancelTx() (requestCompleted bool) {
	cancelTxHash := strings.ToLower(r.tx.Hash().Hex())
	txFromLower := strings.ToLower(r.txFrom)
	r.logger.Info("[cancel-tx] cancelling transaction", "cancelTxHash", cancelTxHash, "txFromLower", txFromLower, "txNonce", r.tx.Nonce())

	// Get initial txHash by sender+nonce
	initialTxHash, txHashFound, err := RState.GetTxHashForSenderAndNonce(txFromLower, r.tx.Nonce())
	if err != nil {
		r.logger.Error("[cancelTx] Redis:GetTxHashForSenderAndNonce failed", "error", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return true
	}

	if !txHashFound { // not found, send to mempool
		return false
	}

	// Check if initial tx was sent to relay
	_, txWasSentToRelay, err := RState.GetTxSentToRelay(initialTxHash)
	if err != nil {
		r.logger.Error("[cancelTx] Redis:GetTxSentToRelay failed", "error", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return true
	}

	if !txWasSentToRelay { // was not sent to relay, send to mempool
		return false
	}

	// Should send cancel-tx to relay. Check if cancel-tx was already sent before
	_, cancelTxAlreadySentToRelay, err := RState.GetTxSentToRelay(cancelTxHash)
	if err != nil {
		r.logger.Error("[cancelTx] Redis:GetTxSentToRelay error", "error", err)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return true
	}

	if cancelTxAlreadySentToRelay { // already sent
		r.writeRpcResult(cancelTxHash)
		return true
	}

	r.logger.Info("[cancel-tx] sending to relay", "initialTxHash", initialTxHash, "txFromLower", txFromLower, "txNonce", r.tx.Nonce())

	if DebugDontSendTx {
		r.logger.Info("[cancelTx] Faked sending cancel-tx to relay, did nothing", "tx", initialTxHash)
		r.writeRpcResult(initialTxHash)
		return true
	}

	cancelPrivTxArgs := flashbotsrpc.FlashbotsCancelPrivateTransactionRequest{TxHash: initialTxHash}
	_, err = FlashbotsRPC.FlashbotsCancelPrivateTransaction(r.relaySigningKey, cancelPrivTxArgs)
	if err != nil {
		if errors.Is(err, flashbotsrpc.ErrRelayErrorResponse) {
			// errors could be: 'tx not found', 'tx was already cancelled', 'tx has already expired'
			r.logger.Info("[cancelTx] Relay error response", "err", err, "rawTx", r.rawTxHex)
			r.writeRpcError(err.Error(), types.JsonRpcInternalError)
		} else {
			r.logger.Error("[cancelTx] Relay call failed", "error", err, "rawTx", r.rawTxHex)
			r.writeRpcError("internal server error", types.JsonRpcInternalError)
		}
		return true
	}

	r.writeRpcResult(cancelTxHash)
	return true
}

func (r *RpcRequest) GetAddressNonceRange(address string) (minNonce, maxNonce uint64, err error) {
	// Get minimum nonce by asking the eth node for the current transaction count
	_req := types.NewJsonRpcRequest(1, "eth_getTransactionCount", []interface{}{r.txFrom, "latest"})
	_res, err := utils.SendRpcAndParseResponseTo(r.defaultProxyUrl, _req)
	if err != nil {
		return 0, 0, err
	}
	_userNonceStr := ""
	err = json.Unmarshal(_res.Result, &_userNonceStr)
	if err != nil {
		r.logger.Error("[sendTxToRelay] eth_getTransactionCount unmarshall failed", "error", err, "result", _res.Result)
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
	return minNonce, maxNonce, nil
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
			r.logger.Error("[WhitehatBalanceCheckerRewrite] isWhitehatBundleCollection json marshal failed:", "error", err)
		} else {
			r.logger.Info("[WhitehatBalanceCheckerRewrite] BalanceChecker contract was rewritten to new version")
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
		r.logger.Error("[writeRpcResult] writeRpcResult error marshalling", "error", err, "result", result)
		r.writeRpcError("internal server error", types.JsonRpcInternalError)
		return
	}
	r.jsonRes = &types.JsonRpcResponse{
		Id:      r.jsonReq.Id,
		Version: "2.0",
		Result:  resBytes,
	}
}
