package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/flashbots/rpc-endpoint/utils"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// RPC request handler for a single/ batch JSON-RPC request
type RpcRequestHandler struct {
	respw           *http.ResponseWriter
	req             *http.Request
	logger          Logger
	timeStarted     time.Time
	defaultProxyUrl string
	relaySigningKey *ecdsa.PrivateKey
	uid             string
}

func NewRpcRequestHandler(respw *http.ResponseWriter, req *http.Request, proxyUrl string, relaySigningKey *ecdsa.PrivateKey) *RpcRequestHandler {
	return &RpcRequestHandler{
		respw:           respw,
		req:             req,
		timeStarted:     Now(),
		defaultProxyUrl: proxyUrl,
		relaySigningKey: relaySigningKey,
	}
}

func (r *RpcRequestHandler) process() {

	// At end of request, log the time it needed
	defer func() {
		timeRequestNeeded := time.Since(r.timeStarted)
		r.logger.log("request took %.6f sec", timeRequestNeeded.Seconds())
	}()

	ip := utils.GetIP(r.req)             // Fetch ip
	origin := r.req.Header.Get("Origin") // Fetch origin
	// Logger setup
	r.uid = uuid.New().String()
	r.logger = NewLogger(r.uid)
	r.logger.log("POST request received")

	// Validate if ip blacklisted
	if IsBlacklisted(ip) {
		r.logger.log("Blocked IP: %s", ip)
		r._writeHeaderStatus(http.StatusUnauthorized)
		return
	}

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	customProxyUrl, ok := r.req.URL.Query()["url"]
	if ok && len(customProxyUrl[0]) > 1 {
		r.defaultProxyUrl = customProxyUrl[0]
		r.logger.log("Using custom url: %s", r.defaultProxyUrl)
	}

	// Decode request JSON RPC
	defer r.req.Body.Close()
	body, err := ioutil.ReadAll(r.req.Body)
	if err != nil {
		r.logger.logError("failed to read request body: %v", err)
		r._writeHeaderStatus(http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		r._writeHeaderStatus(http.StatusBadRequest)
		return
	}

	// Parse JSON RPC payload
	var jsonReq *types.JsonRpcRequest
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		var jsonBatchReq []*types.JsonRpcRequest
		if err = json.Unmarshal(body, &jsonBatchReq); err != nil {
			r.logger.logError("Parse payload %v", err)
			r._writeHeaderStatus(http.StatusBadRequest)
			return
		}
		// Process batch request
		r.processBatchRequest(jsonBatchReq, ip, origin)
		return
	}
	// Process single request
	r.processRequest(jsonReq, ip, origin)

}

// processRequest handles single request
func (r *RpcRequestHandler) processRequest(jsonReq *types.JsonRpcRequest, ip, origin string) {
	// Handle single request
	rpcReq := NewRpcRequest(r.logger, jsonReq, r.defaultProxyUrl, r.relaySigningKey, ip, origin)
	res := rpcReq.ProcessRequest()
	// Write response
	r._writeRpcResponse(res)
}

// processBatchRequest handles multiple batch request
func (r *RpcRequestHandler) processBatchRequest(jsonBatchReq []*types.JsonRpcRequest, ip, origin string) {
	resCh := make(chan *types.JsonRpcResponse, len(jsonBatchReq)) // Chan to hold response from each go routine
	for i := 0; i < cap(resCh); i++ {
		// Process each individual request
		// Scatter worker
		go func(count int, rpcReq *types.JsonRpcRequest) {
			// Create child logger
			l := r.logger.CreateChildLogger(strconv.FormatInt(int64(count), 10))
			// Create rpc request
			req := NewRpcRequest(l, rpcReq, r.defaultProxyUrl, r.relaySigningKey, ip, origin) // Set each individual request
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
