package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/google/uuid"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"
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
	reqRecord       *RequestRecord
	jsonReq         *types.JsonRpcRequest
}

func NewRpcRequestHandler(respw *http.ResponseWriter, req *http.Request, proxyUrl string, relaySigningKey *ecdsa.PrivateKey, reqRecord *RequestRecord) *RpcRequestHandler {
	return &RpcRequestHandler{
		respw:           respw,
		req:             req,
		timeStarted:     Now(),
		defaultProxyUrl: proxyUrl,
		relaySigningKey: relaySigningKey,
		uid:             uuid.New(),
		reqRecord:       reqRecord,
	}
}

func (r *RpcRequestHandler) process() {
	r.logger = log.New(log.Ctx{"uid": r.uid})
	r.logger.Info("[process] POST request received")
	defer r.requestRecord()
	r.reqRecord.requestEntry.ReceivedAt = r.timeStarted
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
		r.reqRecord.UpdateRequestEntry(r.req, http.StatusBadRequest, err.Error())
		r.logger.Error("[process] Failed to read request body", "error", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		r.reqRecord.UpdateRequestEntry(r.req, http.StatusBadRequest, "empty request body")
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	// create rpc proxy client for making proxy request
	client := NewRPCProxyClient(r.defaultProxyUrl)

	r.reqRecord.UpdateRequestEntry(r.req, http.StatusOK, "")
	// Parse JSON RPC payload
	var jsonReq *types.JsonRpcRequest
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		var jsonBatchReq []*types.JsonRpcRequest
		if err = json.Unmarshal(body, &jsonBatchReq); err != nil {
			r.reqRecord.UpdateRequestEntry(r.req, http.StatusBadRequest, err.Error())
			r.logger.Error("[process] Parse payload", "error", err)
			(*r.respw).WriteHeader(http.StatusBadRequest)
			return
		}
		r.jsonReq = jsonReq
		r.reqRecord.requestEntry.RequestType = "batch"
		// Process batch request
		r.processBatchRequest(client, jsonBatchReq, ip, origin, isWhitehatBundleCollection, whitehatBundleId)
		return
	}

	r.jsonReq = jsonReq
	r.reqRecord.requestEntry.RequestType = "single"
	// Process single request
	r.processRequest(client, jsonReq, ip, origin, isWhitehatBundleCollection, whitehatBundleId)
}

// processRequest handles single request
func (r *RpcRequestHandler) processRequest(client RPCProxyClient, jsonReq *types.JsonRpcRequest, ip, origin string, isWhitehatBundleCollection bool, whitehatBundleId string) {
	// Handle single request
	rpcReq := NewRpcRequest(r.logger, client, jsonReq, r.relaySigningKey, ip, origin, isWhitehatBundleCollection, whitehatBundleId, r.reqRecord, r.uid)
	res := rpcReq.ProcessRequest()
	// Write response
	r._writeRpcResponse(res)
}

// processBatchRequest handles multiple batch request
func (r *RpcRequestHandler) processBatchRequest(client RPCProxyClient, jsonBatchReq []*types.JsonRpcRequest, ip, origin string, isWhitehatBundleCollection bool, whitehatBundleId string) {
	resCh := make(chan *types.JsonRpcResponse, len(jsonBatchReq)) // Chan to hold response from each go routine
	for i := 0; i < cap(resCh); i++ {
		// Process each individual request
		// Scatter worker
		go func(count int, rpcReq *types.JsonRpcRequest) {
			// Create child logger
			id := uuid.NewMD5(r.uid, big.NewInt(int64(count)).Bytes())
			logger := log.New(log.Ctx{"uid": r.uid, "id": id, "count": count})
			// Create rpc request
			req := NewRpcRequest(logger, client, rpcReq, r.relaySigningKey, ip, origin, isWhitehatBundleCollection, whitehatBundleId, r.reqRecord, id) // Set each individual request
			res := req.ProcessRequest()
			resCh <- res
		}(i, jsonBatchReq[i])
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

func (r *RpcRequestHandler) requestRecord() {
	timeRequestNeeded := time.Since(r.timeStarted) // At end of request, log the time it needed
	r.reqRecord.requestEntry.RequestDuration = timeRequestNeeded
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		r.reqRecord.SaveRequestEntryToDB(ctx)
	}()
	r.logger.Info("Request finished", "timeTakenInSec", timeRequestNeeded.Seconds())
}
