package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"
	"golang.org/x/exp/rand"

	"github.com/flashbots/rpc-endpoint/application"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/types"
)

var seed uint64 = uint64(rand.Int63())

// RPC request handler for a single/ batch JSON-RPC request
type RpcRequestHandler struct {
	respw               *http.ResponseWriter
	req                 *http.Request
	logger              log.Logger
	timeStarted         time.Time
	defaultProxyUrl     string
	proxyTimeoutSeconds int
	relaySigningKey     *ecdsa.PrivateKey
	relayUrl            string
	uid                 uuid.UUID
	requestRecord       *requestRecord
	builderNames        []string
	chainID             []byte
	rpcCache            *application.RpcCache
}

func NewRpcRequestHandler(
	logger log.Logger,
	respw *http.ResponseWriter,
	req *http.Request,
	proxyUrl string,
	proxyTimeoutSeconds int,
	relaySigningKey *ecdsa.PrivateKey,
	relayUrl string,
	db database.Store,
	builderNames []string,
	chainID []byte,
	rpcCache *application.RpcCache,
) *RpcRequestHandler {
	return &RpcRequestHandler{
		logger:              logger,
		respw:               respw,
		req:                 req,
		timeStarted:         Now(),
		defaultProxyUrl:     proxyUrl,
		proxyTimeoutSeconds: proxyTimeoutSeconds,
		relaySigningKey:     relaySigningKey,
		relayUrl:            relayUrl,
		uid:                 uuid.New(),
		requestRecord:       NewRequestRecord(db),
		builderNames:        builderNames,
		chainID:             chainID,
		rpcCache:            rpcCache,
	}
}

// nolint
func (r *RpcRequestHandler) process() {
	r.logger = r.logger.New("uid", r.uid)
	r.logger.Info("[process] POST request received")

	defer r.finishRequest()
	r.requestRecord.requestEntry.ReceivedAt = r.timeStarted
	r.requestRecord.requestEntry.Id = r.uid
	r.requestRecord.UpdateRequestEntry(r.req, http.StatusOK, "")

	whitehatBundleId := r.req.URL.Query().Get("bundle")
	isWhitehatBundleCollection := whitehatBundleId != ""

	origin := r.req.Header.Get("Origin")
	referer := r.req.Header.Get("Referer")

	// If users specify a proxy url in their rpc endpoint they can have their requests proxied to that endpoint instead of Infura
	// e.g. https://rpc.flashbots.net?url=http://RPC-ENDPOINT.COM
	customProxyUrl, ok := r.req.URL.Query()["url"]
	if ok && len(customProxyUrl[0]) > 1 {
		r.defaultProxyUrl = customProxyUrl[0]
		r.logger.Info("[process] Using custom url", "url", r.defaultProxyUrl)
	}

	// Decode request JSON RPC
	defer r.req.Body.Close()
	body, err := io.ReadAll(r.req.Body)
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

	fingerprint, _ := FingerprintFromRequest(r.req, time.Now(), seed)
	if fingerprint != 0 {
		r.logger = r.logger.New("fingerprint", fingerprint.ToIPv6().String())
	}

	// create rpc proxy client for making proxy request
	client := NewRPCProxyClient(r.logger, r.defaultProxyUrl, r.proxyTimeoutSeconds, fingerprint)

	r.requestRecord.UpdateRequestEntry(r.req, http.StatusOK, "") // Data analytics

	// Parse JSON RPC payload
	var jsonReq *types.JsonRpcRequest
	if err = json.Unmarshal(body, &jsonReq); err != nil {
		r.logger.Warn("[process] Parse payload", "error", err)
		(*r.respw).WriteHeader(http.StatusBadRequest)
		return
	}

	// mev-share parameters
	urlParams, err := ExtractParametersFromUrl(r.req.URL, r.builderNames)
	if err != nil {
		r.logger.Warn("[process] Invalid auction preference", "error", err)
		res := AuctionPreferenceErrorToJSONRPCResponse(jsonReq, err)
		r._writeRpcResponse(res)
		return
	}
	r.logger = r.logger.New("rpc_method", jsonReq.Method)

	// Process single request
	r.processRequest(client, jsonReq, origin, referer, isWhitehatBundleCollection, whitehatBundleId, urlParams, r.req.URL.String(), body)
}

// processRequest handles single request
func (r *RpcRequestHandler) processRequest(client RPCProxyClient, jsonReq *types.JsonRpcRequest, origin, referer string, isWhitehatBundleCollection bool, whitehatBundleId string, urlParams URLParameters, reqURL string, body []byte) {
	var entry *database.EthSendRawTxEntry
	if jsonReq.Method == "eth_sendRawTransaction" {
		entry = r.requestRecord.AddEthSendRawTxEntry(uuid.New())
		// log the full url for debugging
		r.logger.Info("[processRequest] eth_sendRawTransaction request URL", "url", reqURL)
	}
	// Handle single request
	rpcReq := NewRpcRequest(r.logger, client, jsonReq, r.relaySigningKey, r.relayUrl, origin, referer, isWhitehatBundleCollection, whitehatBundleId, entry, urlParams, r.chainID, r.rpcCache)

	if r.req.Header.Get("X-Flashbots-Signature") != "" {
		rpcReq.flashbotsSignature = r.req.Header.Get("X-Flashbots-Signature")
		rpcReq.flashbotsSignatureBody = body
	}
	res := rpcReq.ProcessRequest()
	// Write response
	r._writeRpcResponse(res)
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
