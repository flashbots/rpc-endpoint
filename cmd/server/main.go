package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/server"
	"os"
	"strings"
)

var (
	version = "dev" // is set during build process

	// defaults
	defaultDebug         = os.Getenv("DEBUG") == "1"
	defaultLogJSON       = os.Getenv("LOG_JSON") == "1"
	defaultListenAddress = "127.0.0.1:9000"
	defaultProxyUrl      = "http://127.0.0.1:8545"
	defaultRelayUrl      = "https://relay.flashbots.net"
	defaultRedisUrl      = "localhost:6379"
	defaultPostgresDSN   = fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		"127.0.0.1", 5432, "test", "postgres", "postgres")

	// cli flags
	versionPtr      = flag.Bool("version", false, "just print the program version")
	listenAddress   = flag.String("listen", getEnvOrDefault("LISTEN_ADDR", defaultListenAddress), "Listen address")
	proxyUrl        = flag.String("proxy", getEnvOrDefault("PROXY_URL", defaultProxyUrl), "URL for default JSON-RPC proxy target (eth node, Infura, etc.)")
	redisUrl        = flag.String("redis", getEnvOrDefault("REDIS_URL", defaultRedisUrl), "URL for Redis (use 'dev' to use integrated in-memory redis)")
	relayUrl        = flag.String("relayUrl", getEnvOrDefault("RELAY_URL", defaultRelayUrl), "URL for relay")
	relaySigningKey = flag.String("signingKey", os.Getenv("RELAY_SIGNING_KEY"), "Signing key for relay requests")
	psqlDsn         = flag.String("psqlDsn", getEnvOrDefault("POSTGRES_DSN", defaultPostgresDSN), "Postgres DSN")
	debugPtr        = flag.Bool("debug", defaultDebug, "print debug output")
	logJSONPtr      = flag.Bool("log-json", defaultLogJSON, "log in JSON")
)

func main() {
	var key *ecdsa.PrivateKey
	var err error

	flag.Parse()
	logFormat := log.TerminalFormat(true)
	if *logJSONPtr {
		logFormat = log.JSONFormat()
	}

	logLevel := log.LvlInfo
	if *debugPtr {
		logLevel = log.LvlDebug
	}

	log.Root().SetHandler(log.LvlFilterHandler(logLevel, log.StreamHandler(os.Stderr, logFormat)))
	// Perhaps print only the version
	if *versionPtr {
		log.Info("rpc-endpoint", "version", version)
		return
	}

	log.Info("Init rpc-endpoint", "version", version)

	if *relaySigningKey == "" {
		log.Error("Cannot use the relay without a signing key.")
		return
	}

	pkHex := strings.Replace(*relaySigningKey, "0x", "", 1)
	if pkHex == "dev" {
		log.Info("Creating a new dev signing key...")
		key, err = crypto.GenerateKey()
	} else {
		key, err = crypto.HexToECDSA(pkHex)
	}

	if err != nil {
		log.Error("Error with relay signing key", "error", err)
		return
	}

	// Setup database
	db := database.NewPostgresStore(*psqlDsn)

	// Start the endpoint
	s, err := server.NewRpcEndPointServer(version, *listenAddress, *proxyUrl, *relayUrl, key, *redisUrl, db)
	if err != nil {
		log.Error("Server init error", "error", err)
		return
	}
	s.Start()
}

func getEnvOrDefault(key string, defaultValue string) string {
	ret := os.Getenv(key)
	if ret == "" {
		ret = defaultValue
	}
	return ret
}
