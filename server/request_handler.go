package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/google/uuid"
)

// RPC request handler for a single/ batch JSON-RPC request
type RpcRequestHandler struct {
	respw           *http.ResponseWriter
	req             *http.Request
	logger          log.Logger
	timeStarted     time.Time
	defaultProxyUrl string
	relaySigningKey *ecdsa.PrivateKey
	uid             uuid.UUID
	requestRecord   *requestRecord
}

func NewRpcRequestHandler(respw *http.ResponseWriter, req *http.Request, proxyUrl string, relaySigningKey *ecdsa.PrivateKey, db database.Store) *RpcRequestHandler {
	return &RpcRequestHandler{
		respw:           respw,
		req:             req,
		timeStarted:     Now(),
		defaultProxyUrl: proxyUrl,
		relaySigningKey: relaySigningKey,
		uid:             uuid.New(),
		requestRecord:   NewRequestRecord(db),
	}
}

func (r *RpcRequestHandler) process() {
	r.logger = log.New(log.Ctx{"uid": r.uid})
	r.logger.Info("[process] POST request received")

	defer r.finishRequest()
	r.requestRecord.requestEntry.ReceivedAt = r.timeStarted
	r.requestRecord.requestEntry.Id = r.uid
	r.requestRecord.UpdateRequestEntry(r.req, http.StatusOK, "")

	whitehatBundleId := r.req.URL.Query().Get("bundle")
	isWhitehatBundleCollection := whitehatBundleId != ""

	ip := GetIP(r.req)
	origin := r.req.Header.Get("Origin")

	// Validate if ip blacklisted
	if IsBlacklisted(ip) {
		r.logger.Info("[process] Blocked IP", "ip", ip)
		(*r.respw).WriteHeader(http.StatusUnauthorized)
		return
	}

	var preferences *types.Preferences
	if strings.Trim(r.req.URL.Path, "/") == "fast" { // If fast called, do not include tx to bundle, directly send tx to miners
		isFast := true
		preferences = &types.Preferences{Fast: &isFast}
		r.logger.Info("[process] Setting fast preference", "isFast", isFast)
	}

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	customProxyUrl, ok := r.req.URL.Query()["url"]
	if ok && len(customProxyUrl[0]) > 1 {
		r.defaultProxyUrl = customProxyUrl[0]
		r.logger.Info("[process] Using custom url", "url", r.defaultProxyUrl)
	}

	// Decode request JSON RPC
	defer r.req.Body.Close()
	body, err := ioutil.ReadAll(r.req.Body)
	if err != nil {
		r.requestRecord.UpdateRequestEntry(r.req, http.StatusBadRequest, err.Error())
		r.logger.Error("[process] Failed to read request body", "error", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		r.requestRecord.UpdateRequestEntry(r.req, http.StatusBadRequest, "empty request body")
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	// create rpc proxy client for making proxy request
	client := NewRPCProxyClient(r.defaultProxyUrl)

	r.requestRecord.UpdateRequestEntry(r.req, http.StatusOK, "") // Data analytics
	// Parse JSON RPC payload
	var jsonReq *types.JsonRpcRequest
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		var jsonBatchReq []*types.JsonRpcRequest
		if err = json.Unmarshal(body, &jsonBatchReq); err != nil {
			r.requestRecord.UpdateRequestEntry(r.req, http.StatusBadRequest, err.Error())
			r.logger.Error("[process] Parse payload", "error", err)
			(*r.respw).WriteHeader(http.StatusBadRequest)
			return
		}
		r.requestRecord.requestEntry.IsBatchRequest = true
		r.requestRecord.requestEntry.NumRequestInBatch = len(jsonBatchReq)
		//r.ethSendRawTxEntries = make([]*database.EthSendRawTxEntry, 0, len(jsonBatchReq))
		// Process batch request
		r.processBatchRequest(client, jsonBatchReq, ip, origin, isWhitehatBundleCollection, whitehatBundleId, preferences)
		return
	}
	// Process single request
	//r.ethSendRawTxEntries = make([]*database.EthSendRawTxEntry, 1)
	r.processRequest(client, jsonReq, ip, origin, isWhitehatBundleCollection, whitehatBundleId, preferences)
}

// processRequest handles single request
func (r *RpcRequestHandler) processRequest(client RPCProxyClient, jsonReq *types.JsonRpcRequest, ip, origin string, isWhitehatBundleCollection bool, whitehatBundleId string, preferences *types.Preferences) {
	var entry *database.EthSendRawTxEntry
	if jsonReq.Method == "eth_sendRawTransaction" {
		entry = r.requestRecord.AddEthSendRawTxEntry(uuid.New())
	}
	// Handle single request
	rpcReq := NewRpcRequest(r.logger, client, jsonReq, r.relaySigningKey, ip, origin, isWhitehatBundleCollection, whitehatBundleId, entry, preferences)
	res := rpcReq.ProcessRequest()
	// Write response
	r._writeRpcResponse(res)
}

// processBatchRequest handles multiple batch request
func (r *RpcRequestHandler) processBatchRequest(client RPCProxyClient, jsonBatchReq []*types.JsonRpcRequest, ip, origin string, isWhitehatBundleCollection bool, whitehatBundleId string, preferences *types.Preferences) {
	resCh := make(chan *types.JsonRpcResponse, len(jsonBatchReq)) // Chan to hold response from each go routine
	for i := 0; i < cap(resCh); i++ {
		// Process each individual request
		// Scatter worker
		go func(count int, rpcReq *types.JsonRpcRequest, record *requestRecord) {
			id := uuid.New()
			// Create child logger
			logger := log.New(log.Ctx{"uid": r.uid, "id": id, "count": count})
			// If the request contains eth_sendRawTransaction method, update the request record
			// This rawTxEntry will be stored for protect analytics
			var entry *database.EthSendRawTxEntry
			if rpcReq.Method == "eth_sendRawTransaction" {
				entry = r.requestRecord.AddEthSendRawTxEntry(id)
			}
			// Create rpc request
			req := NewRpcRequest(logger, client, rpcReq, r.relaySigningKey, ip, origin, isWhitehatBundleCollection, whitehatBundleId, entry, preferences) // Set each individual request
			res := req.ProcessRequest()
			resCh <- res
		}(i, jsonBatchReq[i], r.requestRecord)
	}

	response := make([]*types.JsonRpcResponse, 0)
	// Gather responses
	for i := 0; i < cap(resCh); i++ {
		res := <-resCh
		response = append(response, res) // Add it to batch response list
	}
	close(resCh)
	// Write consolidated response
	r._writeRpcBatchResponse(response)
}

func (r *RpcRequestHandler) finishRequest() {
	reqDuration := time.Since(r.timeStarted) // At end of request, log the time it needed
	r.requestRecord.requestEntry.RequestDurationMs = reqDuration.Milliseconds()
	go func() {
		// Save both request entry and raw tx entries if present
		if err := r.requestRecord.SaveRecord(); err != nil {
			log.Error("saveRecord failed", "requestId", r.requestRecord.requestEntry.Id, "error", err, "requestId", r.uid)
		}

	}()
	r.logger.Info("Request finished", "duration", reqDuration.Seconds())
}
