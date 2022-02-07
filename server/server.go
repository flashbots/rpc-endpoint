package server

import (
	"crypto/ecdsa"
	"encoding/json"
	"github.com/ethereum/go-ethereum/log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/metachris/flashbotsrpc"
	"github.com/pkg/errors"
)

var Now = time.Now // used to mock time in tests

var DebugDontSendTx = os.Getenv("DEBUG_DONT_SEND_RAWTX") != ""

// No IPs blacklisted right now
var blacklistedIps = []string{"127.0.0.2"}

// Metamask fix helper
var RState *RedisState

var FlashbotsRPC *flashbotsrpc.FlashbotsRPC

type RpcEndPointServer struct {
	version         string
	startTime       time.Time
	listenAddress   string
	proxyUrl        string
	relaySigningKey *ecdsa.PrivateKey
}

func NewRpcEndPointServer(version string, listenAddress, proxyUrl, relayUrl string, relaySigningKey *ecdsa.PrivateKey, redisUrl string) (*RpcEndPointServer, error) {
	var err error

	if DebugDontSendTx {
		log.Info("DEBUG MODE: raw transactions will not be sent out!", "env", redisUrl)
	}

	if redisUrl == "dev" {
		log.Info("Using integrated in-memory Redis instance", "env", redisUrl)
		redisServer, err := miniredis.Run()
		if err != nil {
			return nil, err
		}
		redisUrl = redisServer.Addr()
	}

	// Setup redis connection
	log.Info("Connecting to redis...", "redisUrl", redisUrl)
	RState, err = NewRedisState(redisUrl)
	if err != nil {
		return nil, errors.Wrap(err, "Redis init error")
	}

	FlashbotsRPC = flashbotsrpc.New(relayUrl)
	FlashbotsRPC.Debug = true

	return &RpcEndPointServer{
		startTime:       Now(),
		version:         version,
		listenAddress:   listenAddress,
		proxyUrl:        proxyUrl,
		relaySigningKey: relaySigningKey,
	}, nil
}

func (s *RpcEndPointServer) Start() {
	log.Info("[Start] Starting rpc endpoint...", "version", s.version, "listenAddress", s.listenAddress)

	// Regularly log debug info
	go func() {
		for {
			log.Info("[Start] Num-goroutines", "count", runtime.NumGoroutine())
			time.Sleep(10 * time.Second)
		}
	}()

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", http.HandlerFunc(s.HandleHttpRequest))
	http.HandleFunc("/health", http.HandlerFunc(s.handleHealthRequest))
	http.HandleFunc("/bundle", http.HandlerFunc(s.HandleBundleRequest))

	// Start serving
	if err := http.ListenAndServe(s.listenAddress, nil); err != nil {
		log.Error("[Start] Failed to start rpc endpoint", "error", err)
	}
}

func (s *RpcEndPointServer) HandleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")

	if req.Method == "GET" {
		http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/rpc/quick-start/", http.StatusFound)
		return
	}

	if req.Method == "OPTIONS" {
		respw.WriteHeader(http.StatusOK)
		return
	}

	request := NewRpcRequestHandler(&respw, req, s.proxyUrl, s.relaySigningKey)
	request.process()
}

func (s *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	res := types.HealthResponse{
		Now:       Now(),
		StartTime: s.startTime,
		Version:   s.version,
	}

	jsonResp, err := json.Marshal(res)
	if err != nil {
		log.Info("[healthCheck] Json error", "error", err)
		respw.WriteHeader(http.StatusInternalServerError)
		return
	}

	respw.Header().Set("Content-Type", "application/json")
	respw.WriteHeader(http.StatusOK)
	respw.Write(jsonResp)
}

func (s *RpcEndPointServer) HandleBundleRequest(respw http.ResponseWriter, req *http.Request) {
	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")

	bundleId := req.URL.Query().Get("id")
	if bundleId == "" {
		http.Error(respw, "no bundle id", http.StatusBadRequest)
		return
	}

	if req.Method == "GET" {
		txs, err := RState.GetWhitehatBundleTx(bundleId)
		if err != nil {
			log.Info("[handleBundleRequest] GetWhitehatBundleTx failed", "bundleId", bundleId, "error", err)
			respw.WriteHeader(http.StatusInternalServerError)
			return
		}

		res := types.BundleResponse{
			BundleId: bundleId,
			RawTxs:   txs,
		}

		jsonResp, err := json.Marshal(res)
		if err != nil {
			log.Info("[handleBundleRequest] Json marshal failed", "error", err)
			respw.WriteHeader(http.StatusInternalServerError)
			return
		}

		respw.Header().Set("Content-Type", "application/json")
		respw.WriteHeader(http.StatusOK)
		respw.Write(jsonResp)

	} else if req.Method == "POST" {
		RState.DelWhitehatBundleTx(bundleId)
		respw.WriteHeader(http.StatusOK)

	} else {
		respw.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func IsBlacklisted(ip string) bool {
	for i := range blacklistedIps {
		if strings.HasPrefix(ip, blacklistedIps[i]) {
			return true
		}
	}
	return false
}
