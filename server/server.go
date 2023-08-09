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
	"github.com/pkg/errors"
)

var Now = time.Now // used to mock time in tests

var DebugDontSendTx = os.Getenv("DEBUG_DONT_SEND_RAWTX") != ""

// Metamask fix helper
var RState *RedisState

type RpcEndPointServer struct {
	server *http.Server
	drain  *http.Server

	drainAddress        string
	drainSeconds        int
	db                  database.Store
	isHealthy           bool
	isHealthyMx         sync.RWMutex
	listenAddress       string
	logger              log.Logger
	proxyTimeoutSeconds int
	proxyUrl            string
	relaySigningKey     *ecdsa.PrivateKey
	relayUrl            string
	startTime           time.Time
	version             string
}

func NewRpcEndPointServer(cfg Configuration) (*RpcEndPointServer, error) {
	var err error
	if DebugDontSendTx {
		cfg.Logger.Info("DEBUG MODE: raw transactions will not be sent out!", "redisUrl", cfg.RedisUrl)
	}

	if cfg.RedisUrl == "dev" {
		cfg.Logger.Info("Using integrated in-memory Redis instance", "redisUrl", cfg.RedisUrl)
		redisServer, err := miniredis.Run()
		if err != nil {
			return nil, err
		}
		cfg.RedisUrl = redisServer.Addr()
	}
	// Setup redis connection
	cfg.Logger.Info("Connecting to redis...", "redisUrl", cfg.RedisUrl)
	RState, err = NewRedisState(cfg.RedisUrl)
	if err != nil {
		return nil, errors.Wrap(err, "Redis init error")
	}

	return &RpcEndPointServer{
		db:                  cfg.DB,
		drainAddress:        cfg.DrainAddress,
		drainSeconds:        cfg.DrainSeconds,
		isHealthy:           true,
		listenAddress:       cfg.ListenAddress,
		logger:              cfg.Logger,
		proxyTimeoutSeconds: cfg.ProxyTimeoutSeconds,
		proxyUrl:            cfg.ProxyUrl,
		relaySigningKey:     cfg.RelaySigningKey,
		relayUrl:            cfg.RelayUrl,
		startTime:           Now(),
		version:             cfg.Version,
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

	s.startMainServer()
	s.startDrainServer()

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, os.Interrupt, syscall.SIGTERM)

	<-notifier

	s.stopDrainServer()
	s.stopMainServer()
}

func (s *RpcEndPointServer) startMainServer() {
	if s.server != nil {
		panic("http server is already running")
	}
	// Handler for root URL (JSON-RPC on POST, public/index.html on GET)
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.HandleHttpRequest)
	mux.HandleFunc("/health", s.handleHealthRequest)
	mux.HandleFunc("/bundle", s.HandleBundleRequest)
	s.server = &http.Server{
		Addr:         s.listenAddress,
		Handler:      mux,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("http server failed", "error", err)
		}
	}()
}

func (s *RpcEndPointServer) stopMainServer() {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); err != nil {
			s.logger.Error("http server shutdown failed", "error", err)
		}
		s.logger.Info("http server stopped")
		s.server = nil
	}
}

func (s *RpcEndPointServer) startDrainServer() {
	if s.drain != nil {
		panic("drain http server is already running")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDrain)
	s.drain = &http.Server{
		Addr:    s.drainAddress,
		Handler: mux,
	}
	go func() {
		if err := s.drain.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("drain http server failed", "error", err)
		}
	}()
}

func (s *RpcEndPointServer) stopDrainServer() {
	if s.drain != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.drain.Shutdown(ctx); err != nil {
			s.logger.Error("drain http server shutdown failed", "error", err)
		}
		s.logger.Info("drain http server stopped")
		s.drain = nil
	}
}

func (s *RpcEndPointServer) HandleHttpRequest(respw http.ResponseWriter, req *http.Request) {
	setCorsHeaders(respw)

	if req.Method == http.MethodGet {
		if strings.Trim(req.URL.Path, "/") == "fast" {
			http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/overview", http.StatusFound)
		} else {
			http.Redirect(respw, req, "https://docs.flashbots.net/flashbots-protect/overview", http.StatusFound)
		}
		return
	}

	if req.Method == http.MethodOptions {
		respw.WriteHeader(http.StatusOK)
		return
	}

	request := NewRpcRequestHandler(s.logger, &respw, req, s.proxyUrl, s.proxyTimeoutSeconds, s.relaySigningKey, s.relayUrl, s.db)
	request.process()
}

func (s *RpcEndPointServer) handleDrain(respw http.ResponseWriter, req *http.Request) {
	s.isHealthyMx.Lock()
	if !s.isHealthy {
		s.isHealthyMx.Unlock()
		return
	}

	s.isHealthy = false
	s.logger.Info("Server marked as unhealthy")

	// Let's not hold onto the lock in our sleep
	s.isHealthyMx.Unlock()

	// Give LB enough time to detect us unhealthy
	time.Sleep(
		time.Duration(s.drainSeconds) * time.Second,
	)
}

func (s *RpcEndPointServer) handleHealthRequest(respw http.ResponseWriter, req *http.Request) {
	s.isHealthyMx.RLock()
	defer s.isHealthyMx.RUnlock()
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
	if s.isHealthy {
		respw.WriteHeader(http.StatusOK)
	} else {
		respw.WriteHeader(http.StatusInternalServerError)
	}
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
