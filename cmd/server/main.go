package main

import (
	"crypto/ecdsa"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/server"
)

var (
	version = "dev" // is set during build process

	// defaults
	defaultDebug               = os.Getenv("DEBUG") == "1"
	defaultLogJSON             = os.Getenv("LOG_JSON") == "1"
	defaultListenAddress       = "127.0.0.1:9000"
	defaultProxyUrl            = "http://127.0.0.1:8545"
	defaultProxyTimeoutSeconds = 10
	defaultRelayUrl            = "https://relay.flashbots.net"
	defaultRedisUrl            = "localhost:6379"
	defaultServiceName         = getEnvAsStrOrDefault("SERVICE_NAME", "rpc-endpoint")

	// cli flags
	versionPtr          = flag.Bool("version", false, "just print the program version")
	listenAddress       = flag.String("listen", getEnvAsStrOrDefault("LISTEN_ADDR", defaultListenAddress), "Listen address")
	proxyUrl            = flag.String("proxy", getEnvAsStrOrDefault("PROXY_URL", defaultProxyUrl), "URL for default JSON-RPC proxy target (eth node, Infura, etc.)")
	proxyTimeoutSeconds = flag.Int("proxyTimeoutSeconds", getEnvAsIntOrDefault("PROXY_TIMEOUT_SECONDS", defaultProxyTimeoutSeconds), "proxy client timeout in seconds")
	redisUrl            = flag.String("redis", getEnvAsStrOrDefault("REDIS_URL", defaultRedisUrl), "URL for Redis (use 'dev' to use integrated in-memory redis)")
	relayUrl            = flag.String("relayUrl", getEnvAsStrOrDefault("RELAY_URL", defaultRelayUrl), "URL for relay")
	relaySigningKey     = flag.String("signingKey", os.Getenv("RELAY_SIGNING_KEY"), "Signing key for relay requests")
	psqlDsn             = flag.String("psql", os.Getenv("POSTGRES_DSN"), "Postgres DSN")
	debugPtr            = flag.Bool("debug", defaultDebug, "print debug output")
	logJSONPtr          = flag.Bool("log-json", defaultLogJSON, "log in JSON")
	serviceName         = flag.String("serviceName", defaultServiceName, "name of the service which will be used in the logs")
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
	logger := log.New(log.Ctx{"service": *serviceName})
	// Perhaps print only the version
	if *versionPtr {
		logger.Info("rpc-endpoint", "version", version)
		return
	}

	logger.Info("Init rpc-endpoint", "version", version)

	if *relaySigningKey == "" {
		logger.Crit("Cannot use the relay without a signing key.")
	}

	pkHex := strings.Replace(*relaySigningKey, "0x", "", 1)
	if pkHex == "dev" {
		logger.Info("Creating a new dev signing key...")
		key, err = crypto.GenerateKey()
	} else {
		key, err = crypto.HexToECDSA(pkHex)
	}

	if err != nil {
		logger.Crit("Error with relay signing key", "error", err)
	}

	// Setup database
	var db database.Store
	if *psqlDsn == "" {
		db = database.NewMockStore()
	} else {
		db = database.NewPostgresStore(*psqlDsn)
	}
	// Start the endpoint
	s, err := server.NewRpcEndPointServer(logger, version, *listenAddress, *relayUrl, *proxyUrl, *proxyTimeoutSeconds, key, *redisUrl, db)
	if err != nil {
		logger.Crit("Server init error", "error", err)
	}
	logger.Info("Starting rpc-endpoint...", "relayUrl", *relayUrl, "proxyUrl", *proxyUrl)
	s.Start()
}

func getEnvAsStrOrDefault(key string, defaultValue string) string {
	ret := os.Getenv(key)
	if ret == "" {
		ret = defaultValue
	}
	return ret
}

func getEnvAsIntOrDefault(name string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
