package server

import (
	"crypto/ecdsa"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
)

type Configuration struct {
	DB                  database.Store
	ListenAddress       string
	Logger              log.Logger
	ProxyTimeoutSeconds int
	ProxyUrl            string
	RedisUrl            string
	RelaySigningKey     *ecdsa.PrivateKey
	RelayUrl            string
	Version             string
	ShutdownDrainTime   time.Duration
}
