package test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/flashbots/rpc-endpoint/server"
	"github.com/stretchr/testify/require"
)

var redisServer *miniredis.Miniredis
var redisState *server.RedisState

func resetRedis() {
	var err error
	if redisServer != nil {
		redisServer.Close()
	}

	redisServer, err = miniredis.Run()
	if err != nil {
		panic(err)
	}

	redisState, err = server.NewRedisState(redisServer.Addr())
	// redisState, err = server.NewRedisState("localhost:6379")
	if err != nil {
		panic(err)
	}
}

func TestRedisStateSetup(t *testing.T) {
	var err error
	redisState, err = server.NewRedisState("localhost:18279")
	require.NotNil(t, err, err)
}

func TestTxSentToRelay(t *testing.T) {
	var err error
	resetRedis()

	timeBeforeSet := time.Now()
	err = redisState.SetTxSentToRelay("foo")
	require.Nil(t, err, err)

	timeSent, found, err := redisState.GetTxSentToRelay("foo")
	require.Nil(t, err, err)
	require.True(t, found)

	// Returned time should be after time set and within 1 second of current time
	require.True(t, time.Since(timeSent) >= time.Since(timeBeforeSet))
	require.True(t, time.Since(timeSent) < time.Second)

	// Invalid key should return found: false but no error
	timeSent, found, err = redisState.GetTxSentToRelay("XXX")
	require.Nil(t, err, err)
	require.False(t, found)

	// After resetting redis, we shouldn't be able to find the key
	resetRedis()
	timeSent, found, err = redisState.GetTxSentToRelay("foo")
	require.Nil(t, err, err)
	require.False(t, found)
}

func TestTxHashForSenderAndNonce(t *testing.T) {
	var err error
	resetRedis()

	txFrom := "0x0Sender"
	nonce := uint64(1337)
	txHash := "0x0TxHash"

	// Ensure key is correct
	key := server.RedisKeyTxHashForSenderAndNonce(txFrom, nonce)
	expectedKey := fmt.Sprintf("%s%s_%d", server.RedisPrefixTxHashForSenderAndNonce, strings.ToLower(txFrom), nonce)
	require.Equal(t, expectedKey, key)

	// Get before set: should return not found
	txHashFromRedis, found, err := redisState.GetTxHashForSenderAndNonce(txFrom, nonce)
	require.Nil(t, err, err)
	require.False(t, found)
	require.Equal(t, "", txHashFromRedis)

	// Set
	err = redisState.SetTxHashForSenderAndNonce(txFrom, nonce, txHash)
	require.Nil(t, err, err)

	// Get
	txHashFromRedis, found, err = redisState.GetTxHashForSenderAndNonce(txFrom, nonce)
	require.Nil(t, err, err)
	require.True(t, found)

	// The txHash is stored lowercased, so it doesn't match directly
	require.NotEqual(t, txHash, txHashFromRedis)
	require.Equal(t, strings.ToLower(txHash), txHashFromRedis)
}
