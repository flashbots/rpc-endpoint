package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"

	"github.com/flashbots/rpc-endpoint/application"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/types"
)

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

	fingerprint, err := r.getFingerprint()
	if fingerprint != "" {
		r.logger = r.logger.New("fingerprint", fingerprint)
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
	r.processRequest(client, jsonReq, origin, referer, isWhitehatBundleCollection, whitehatBundleId, urlParams, r.req.URL.String())
}

// processRequest handles single request
func (r *RpcRequestHandler) processRequest(client RPCProxyClient, jsonReq *types.JsonRpcRequest, origin, referer string, isWhitehatBundleCollection bool, whitehatBundleId string, urlParams URLParameters, reqURL string) {
	var entry *database.EthSendRawTxEntry
	if jsonReq.Method == "eth_sendRawTransaction" {
		entry = r.requestRecord.AddEthSendRawTxEntry(uuid.New())
		// log the full url for debugging
		r.logger.Info("[processRequest] eth_sendRawTransaction request URL", "url", reqURL)
	}
	// Handle single request
	rpcReq := NewRpcRequest(r.logger, client, jsonReq, r.relaySigningKey, r.relayUrl, origin, referer, isWhitehatBundleCollection, whitehatBundleId, entry, urlParams, r.chainID, r.rpcCache)
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

func (r *RpcRequestHandler) getFingerprint() (string, error) {
	// X-Forwarded-For: 2600:8802:4700:bee:d13c:c7fb:8e0f:84ff, 172.70.210.100
	xff, err := getXForwardedForIP(r.req)
	if err != nil {
		return "", err
	}
	fingerprintPreimage := fmt.Sprintf("XFF:%s|UA:%s", xff, r.req.Header.Get("User-Agent"))
	sum := xxhash.Sum64String(fingerprintPreimage)
	fingerprint := fmt.Sprintf("%x", sum)
	return fingerprint, nil
}

func getXForwardedForIP(r *http.Request) (string, error) {
	// gets the left-most non-private IP in the X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return "", fmt.Errorf("no X-Forwarded-For header")
	}
	ips := strings.Split(xff, ",")
	for _, ip := range ips {
		if !isPrivateIP(ip) {
			return ip, nil
		}
	}
	return "", fmt.Errorf("no non-private IP in X-Forwarded-For header")
}

func isPrivateIP(ip string) bool {
	// compare ip to RFC-1918 known private IP ranges
	// https://en.wikipedia.org/wiki/Private_network
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}

	for _, cidr := range cidrs {
		if cidr.Contains(ipAddr) {
			return true
		}
	}
	return false
}

// Taken from https://github.com/tomasen/realip/blob/master/realip.go
// MIT Licensed, Copyright (c) 2018 SHEN SHENG
var cidrs []*net.IPNet

func init() {
	maxCidrBlocks := []string{
		"127.0.0.1/8",    // localhost
		"10.0.0.0/8",     // 24-bit block
		"172.16.0.0/12",  // 20-bit block
		"192.168.0.0/16", // 16-bit block
		"169.254.0.0/16", // link local address
		"::1/128",        // localhost IPv6
		"fc00::/7",       // unique local address IPv6
		"fe80::/10",      // link local address IPv6
	}

	cidrs = make([]*net.IPNet, len(maxCidrBlocks))
	for i, maxCidrBlock := range maxCidrBlocks {
		_, cidr, _ := net.ParseCIDR(maxCidrBlock)
		cidrs[i] = cidr
	}
}
