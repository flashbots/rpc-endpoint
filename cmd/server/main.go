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
	"github.com/flashbots/rpc-endpoint/metrics"
	"github.com/flashbots/rpc-endpoint/server"
)

var (
	version = "dev" // is set during build process

	// defaults
	defaultDebug                    = os.Getenv("DEBUG") == "1"
	defaultLogJSON                  = os.Getenv("LOG_JSON") == "1"
	defaultListenAddress            = "127.0.0.1:9000"
	defaultDrainAddress             = "127.0.0.1:9001"
	defaultDrainSeconds             = 60
	defaultProxyUrl                 = "http://127.0.0.1:8545"
	defaultProxyTimeoutSeconds      = 10
	defaultRelayUrl                 = "https://relay.flashbots.net"
	defaultRedisUrl                 = "localhost:6379"
	defaultServiceName              = os.Getenv("SERVICE_NAME")
	defaultFetchInfoIntervalSeconds = 600
	defaultRpcTTLCacheSeconds       = 300
	defaultMempoolRPC               = os.Getenv("DEFAULT_MEMPOOL_RPC")
	defaultMetricsAddr              = os.Getenv("METRICS_ADDR")
	defaultCustomerConfigFile       = os.Getenv("CUSTOMER_CONFIG")

	// cli flags
	versionPtr           = flag.Bool("version", false, "just print the program version")
	listenAddress        = flag.String("listen", getEnvAsStrOrDefault("LISTEN_ADDR", defaultListenAddress), "Listen address")
	drainAddress         = flag.String("drain", getEnvAsStrOrDefault("DRAIN_ADDR", defaultDrainAddress), "Drain address")
	drainSeconds         = flag.Int("drainSeconds", getEnvAsIntOrDefault("DRAIN_SECONDS", defaultDrainSeconds), "seconds to wait for graceful shutdown")
	fetchIntervalSeconds = flag.Int("fetchIntervalSeconds", getEnvAsIntOrDefault("FETCH_INFO_INTERVAL_SECONDS", defaultFetchInfoIntervalSeconds), "seconds between builder info fetches")
	ttlCacheSeconds      = flag.Int("ttlCacheSeconds", getEnvAsIntOrDefault("TTL_CACHE_SECONDS", defaultRpcTTLCacheSeconds), "seconds to cache static requests")
	builderInfoSource    = flag.String("builderInfoSource", getEnvAsStrOrDefault("BUILDER_INFO_SOURCE", ""), "URL for json source of actual builder info")
	proxyUrl             = flag.String("proxy", getEnvAsStrOrDefault("PROXY_URL", defaultProxyUrl), "URL for default JSON-RPC proxy target (eth node, Infura, etc.)")
	proxyTimeoutSeconds  = flag.Int("proxyTimeoutSeconds", getEnvAsIntOrDefault("PROXY_TIMEOUT_SECONDS", defaultProxyTimeoutSeconds), "proxy client timeout in seconds")
	redisUrl             = flag.String("redis", getEnvAsStrOrDefault("REDIS_URL", defaultRedisUrl), "URL for Redis (use 'dev' to use integrated in-memory redis)")
	relayUrl             = flag.String("relayUrl", getEnvAsStrOrDefault("RELAY_URL", defaultRelayUrl), "URL for relay")
	relaySigningKey      = flag.String("signingKey", os.Getenv("RELAY_SIGNING_KEY"), "Signing key for relay requests")
	psqlDsn              = flag.String("psql", os.Getenv("POSTGRES_DSN"), "Postgres DSN")
	debugPtr             = flag.Bool("debug", defaultDebug, "print debug output")
	logJSONPtr           = flag.Bool("logJSON", defaultLogJSON, "log in JSON")
	serviceName          = flag.String("serviceName", defaultServiceName, "name of the service which will be used in the logs")
)

func main() {
	var key *ecdsa.PrivateKey
	var err error

	flag.Parse()

	logLevel := log.LvlInfo
	if *debugPtr {
		logLevel = log.LvlDebug
	}
	termHandler := log.NewTerminalHandlerWithLevel(os.Stderr, logLevel, false)
	rLogger := log.NewLogger(termHandler)
	if *logJSONPtr {
		// NOTE(tymurkh): unfortunately seems to be no way to pass LogLevel in jsonHandler, so level will always be info
		rLogger = log.NewLogger(log.JSONHandler(os.Stderr))
	}

	log.SetDefault(rLogger)

	logger := log.New()
	if *serviceName != "" {
		logger = logger.New("service", *serviceName)
	}
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

	logger.Info("Reading customer config from file", "file", defaultCustomerConfigFile)
	configurationWatcher, err := server.ReadCustomerConfigFromFile(defaultCustomerConfigFile)
	if err != nil {
		logger.Crit("Customer config file is set, but file is invalid", "error", err)
	}

	metrics.InitCustomersConfigMetric(configurationWatcher.Customers()...)

	// todo: setup configuration watcher

	// Start the endpoint
	s, err := server.NewRpcEndPointServer(server.Configuration{
		DB:                   db,
		DrainAddress:         *drainAddress,
		DrainSeconds:         *drainSeconds,
		ListenAddress:        *listenAddress,
		Logger:               logger,
		ProxyTimeoutSeconds:  *proxyTimeoutSeconds,
		ProxyUrl:             *proxyUrl,
		RedisUrl:             *redisUrl,
		RelaySigningKey:      key,
		RelayUrl:             *relayUrl,
		Version:              version,
		BuilderInfoSource:    *builderInfoSource,
		FetchInfoInterval:    *fetchIntervalSeconds,
		TTLCacheSeconds:      int64(*ttlCacheSeconds),
		DefaultMempoolRPC:    defaultMempoolRPC,
		ConfigurationWatcher: configurationWatcher,
	})
	if err != nil {
		logger.Crit("Server init error", "error", err)
	}
	logger.Info("Starting rpc-endpoint...", "relayUrl", *relayUrl, "proxyUrl", *proxyUrl)

	if defaultMetricsAddr != "" {
		go func() {
			if err := metrics.DefaultServer(defaultMetricsAddr).ListenAndServe(); err != nil {
				log.Crit("metrics http server crashed", "err", err)
			}
		}()
	}

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
