package server

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
)

type Configuration struct {
	DB                  database.Store
	DrainAddress        string
	DrainSeconds        int
	ListenAddress       string
	Logger              log.Logger
	ProxyTimeoutSeconds int
	ProxyUrl            string
	RedisUrl            string
	RelaySigningKey     *ecdsa.PrivateKey
	RelayUrl            string
	Version             string
	BuilderInfoSource   string
	FetchInfoInterval   int
	TTLCacheSeconds     int64
}
