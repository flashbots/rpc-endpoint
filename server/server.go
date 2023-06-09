package server

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/flashbots/rpc-endpoint/database"

	"github.com/ethereum/go-ethereum/log"

	"github.com/alicebob/miniredis"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/metachris/flashbotsrpc"
	"github.com/pkg/errors"
)

var Now = time.Now // used to mock time in tests

var DebugDontSendTx = os.Getenv("DEBUG_DONT_SEND_RAWTX") != ""

// Metamask fix helper
var RState *RedisState

var FlashbotsRPC *flashbotsrpc.FlashbotsRPC

type RpcEndPointServer struct {
	logger              log.Logger
	version             string
	startTime           time.Time
	listenAddress       string
	proxyUrl            string
	proxyTimeoutSeconds int
	relaySigningKey     *ecdsa.PrivateKey
	db                  database.Store
	healthy             bool
	healthyMu           sync.Mutex
}

func NewRpcEndPointServer(logger log.Logger, version, listenAddress, relayUrl, proxyUrl string, proxyTimeoutSeconds int, relaySigningKey *ecdsa.PrivateKey, redisUrl string, db database.Store) (*RpcEndPointServer, error) {
	var err error
	if DebugDontSendTx {
		logger.Info("DEBUG MODE: raw transactions will not be sent out!", "redisUrl", redisUrl)
	}

	if redisUrl == "dev" {
		logger.Info("Using integrated in-memory Redis instance", "redisUrl", redisUrl)
		redisServer, err := miniredis.Run()
		if err != nil {
			return nil, err
		}
		redisUrl = redisServer.Addr()
	}
	// Setup redis connection
	logger.Info("Connecting to redis...", "redisUrl", redisUrl)
	RState, err = NewRedisState(redisUrl)
	if err != nil {
		return nil, errors.Wrap(err, "Redis init error")
	}

	FlashbotsRPC = flashbotsrpc.New(relayUrl)
	// FlashbotsRPC.Debug = true

	return &RpcEndPointServer{
		logger:              logger,
		startTime:           Now(),
		version:             version,
		listenAddress:       listenAddress,
		proxyUrl:            proxyUrl,
		proxyTimeoutSeconds: proxyTimeoutSeconds,
		relaySigningKey:     relaySigningKey,
		db:                  db,
		healthy:             true,
		healthyMu:           sync.Mutex{},
	}, nil
}

func (s *RpcEndPointServer) Start() {
	s.logger.Info("Starting rpc endpoint...", "version", s.version, "listenAddress", s.listenAddress)

	// Regularly log debug info
	go func() {
		for {
			s.logger.Info("[stats] num-goroutines", "count", runtime.NumGoroutine())
			time.Sleep(10 * time.Second)
		}
	}()

	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	http.HandleFunc("/", s.HandleHttpRequest)
	http.HandleFunc("/health", s.handleHealthRequest)
	http.HandleFunc("/bundle", s.HandleBundleRequest)

	server := &http.Server{
		Addr:         s.listenAddress,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("http server failed", "error", err)
		}
	}()

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, os.Interrupt, syscall.SIGTERM)

	<-notifier

	s.SetUnhealthy()
	s.logger.Info("http server stopping")

	// Wait for 60 seconds for load balancer to pick up the unhealthy state or for SIGTERM to be sent again
	select {
	case <-time.After(60 * time.Second):
	case <-notifier:
		s.logger.Info("http server stopping immediately")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	go func() {
		<-notifier
		s.logger.Info("http server shutdown interrupted by signal")
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		s.logger.Error("http server shutdown failed", "error", err)
	}
	s.logger.Info("http server stopped")
}

func (s *RpcEndPointServer) HandleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	setCorsHeaders(respw)

	if req.Method == http.MethodGet {
		if strings.Trim(req.URL.Path, "/") == "fast" {
			http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/rpc/fast-mode/", http.StatusFound)
		} else {
			http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/rpc/quick-start/", http.StatusFound)
		}
		return
	}

	if req.Method == http.MethodOptions {
		respw.WriteHeader(http.StatusOK)
		return
	}

	request := NewRpcRequestHandler(s.logger, &respw, req, s.proxyUrl, s.proxyTimeoutSeconds, s.relaySigningKey, s.db)
	request.process()
}

func (s *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	s.healthyMu.Lock()
	defer s.healthyMu.Unlock()
	if !s.healthy {
		respw.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	res := types.HealthResponse{
		Now:       Now(),
		StartTime: s.startTime,
		Version:   s.version,
	}

	jsonResp, err := json.Marshal(res)
	if err != nil {
		s.logger.Info("[healthCheck] Json error", "error", err)
		respw.WriteHeader(http.StatusInternalServerError)
		return
	}

	respw.Header().Set("Content-Type", "application/json")
	respw.WriteHeader(http.StatusOK)
	respw.Write(jsonResp)
}

func (s *RpcEndPointServer) HandleBundleRequest(respw http.ResponseWriter, req *http.Request) {
	setCorsHeaders(respw)
	bundleId := req.URL.Query().Get("id")
	if bundleId == "" {
		http.Error(respw, "no bundle id", http.StatusBadRequest)
		return
	}

	if req.Method == http.MethodGet {
		txs, err := RState.GetWhitehatBundleTx(bundleId)
		if err != nil {
			s.logger.Info("[handleBundleRequest] GetWhitehatBundleTx failed", "bundleId", bundleId, "error", err)
			respw.WriteHeader(http.StatusInternalServerError)
			return
		}

		res := types.BundleResponse{
			BundleId: bundleId,
			RawTxs:   txs,
		}

		jsonResp, err := json.Marshal(res)
		if err != nil {
			s.logger.Info("[handleBundleRequest] Json marshal failed", "error", err)
			respw.WriteHeader(http.StatusInternalServerError)
			return
		}
		respw.Header().Set("Content-Type", "application/json")
		respw.WriteHeader(http.StatusOK)
		respw.Write(jsonResp)

	} else if req.Method == http.MethodDelete {
		RState.DelWhitehatBundleTx(bundleId)
		respw.WriteHeader(http.StatusOK)

	} else {
		respw.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func setCorsHeaders(respw http.ResponseWriter) {
	respw.Header().Set("Access-Control-Allow-Origin", "*")
	respw.Header().Set("Access-Control-Allow-Headers", "Accept,Content-Type")
}

func (s *RpcEndPointServer) SetUnhealthy() {
	s.healthyMu.Lock()
	defer s.healthyMu.Unlock()
	s.healthy = false
}
