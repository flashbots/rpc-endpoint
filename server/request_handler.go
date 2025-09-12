package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"
	"golang.org/x/exp/rand"

	"github.com/flashbots/rpc-endpoint/application"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/metrics"
	"github.com/flashbots/rpc-endpoint/types"
)

var seed uint64 = uint64(rand.Int63())

// RPC request handler for a single/ batch JSON-RPC request
type RpcRequestHandler struct {
	respw                *http.ResponseWriter
	req                  *http.Request
	logger               log.Logger
	timeStarted          time.Time
	defaultProxyUrl      string
	proxyTimeoutSeconds  int
	relaySigningKey      *ecdsa.PrivateKey
	relayUrl             string
	uid                  uuid.UUID
	requestRecord        *requestRecord
	builderNames         []string
	chainID              []byte
	rpcCache             *application.RpcCache
	defaultEthClient     *ethclient.Client
	configurationWatcher *ConfigurationWatcher
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
	defaultEthClient *ethclient.Client,
	configurationWatcher *ConfigurationWatcher,
) *RpcRequestHandler {
	return &RpcRequestHandler{
		logger:               logger,
		respw:                respw,
		req:                  req,
		timeStarted:          Now(),
		defaultProxyUrl:      proxyUrl,
		proxyTimeoutSeconds:  proxyTimeoutSeconds,
		relaySigningKey:      relaySigningKey,
		relayUrl:             relayUrl,
		uid:                  uuid.New(),
		requestRecord:        NewRequestRecord(db),
		builderNames:         builderNames,
		chainID:              chainID,
		rpcCache:             rpcCache,
		defaultEthClient:     defaultEthClient,
		configurationWatcher: configurationWatcher,
	}
}

// getEffectiveParameters determines the URL parameters to use for this request.
// It checks for header-based preset override first, then falls back to URL parsing.
func (r *RpcRequestHandler) getEffectiveParameters() (URLParameters, error) {
	extracted, err := ExtractParametersFromUrl(r.req.URL, r.builderNames)
	if err != nil {
		return extracted, err
	}
	if r.configurationWatcher == nil {
		return extracted, nil
	}
	originID := extracted.originId
	if headerOriginID := r.req.Header.Get("X-Flashbots-Origin"); headerOriginID != "" {
		originID = headerOriginID
	}
	if preset, exists := r.configurationWatcher.ParsedPresets[originID]; exists {
		r.logger.Info("Using preset configuration", "originID", originID)
		return preset, nil
	}

	return extracted, nil
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
		metrics.UrlParamUsageInc()
		r.defaultProxyUrl = customProxyUrl[0]
		originID := r.req.Header.Get("X-Flashbots-Origin")
		r.logger.Info("[process] Using custom url", "url", r.defaultProxyUrl, "originID", originID)
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
	urlParams, err := r.getEffectiveParameters()
	if err != nil {
		r.logger.Warn("[process] Invalid auction preference", "error", err, "url", r.req.URL)
		res := AuctionPreferenceErrorToJSONRPCResponse(jsonReq, err)
		r._writeRpcResponse(res)
		return
	}
	r.logger = r.logger.New("rpc_method", jsonReq.Method)

	if r.configurationWatcher != nil && jsonReq.Method == "eth_sendRawTransaction" {
		origin := urlParams.originId
		r.logger.Info("configuration_watcher_check", "url", r.req.RequestURI, "origin", origin)
		updated := r.configurationWatcher.IsConfigurationUpdated(origin, urlParams)
		if updated {
			r.logger.Info("Configuration change detected", "origin", origin, "url", r.req.RequestURI)
			metrics.ReportCustomerConfigWasUpdated(origin)
		}
	}

	// Process single request
	r.processRequest(client, jsonReq, origin, referer, isWhitehatBundleCollection, whitehatBundleId, urlParams, r.req.URL.String(), body)
}

// processRequest handles single request
func (r *RpcRequestHandler) processRequest(client RPCProxyClient, jsonReq *types.JsonRpcRequest, origin, referer string, isWhitehatBundleCollection bool, whitehatBundleId string, urlParams URLParameters, reqURL string, body []byte) {
	var entry *database.EthSendRawTxEntry
	if jsonReq.Method == "eth_sendRawTransaction" || jsonReq.Method == "eth_sendPrivateTransaction" {
		entry = r.requestRecord.AddEthSendRawTxEntry(uuid.New())
		// log the full url for debugging
		r.logger.Info("[processRequest] ", jsonReq.Method, " request URL", "url", reqURL)
	}
	// Handle single request
	rpcReq := NewRpcRequest(r.logger, client, jsonReq, r.relaySigningKey, r.relayUrl, origin, referer, isWhitehatBundleCollection, whitehatBundleId, entry, urlParams, r.chainID, r.rpcCache, r.defaultEthClient)

	if err := rpcReq.CheckFlashbotsSignature(r.req.Header.Get("X-Flashbots-Signature"), body); err != nil {
		r.logger.Warn("[processRequest] CheckFlashbotsSignature", "error", err)
		rpcReq.writeRpcError(err.Error(), types.JsonRpcInvalidRequest)
		r._writeRpcResponse(rpcReq.jsonRes)
		return
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
			log.Error("saveRecord failed", "requestId", r.requestRecord.requestEntry.Id, "error", err)
		}
	}()
	r.logger.Info("Request finished", "duration", reqDuration.Seconds())
}
