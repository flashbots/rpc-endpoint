package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis"
	"github.com/stretchr/testify/require"
)

var redisServer *miniredis.Miniredis
var redisState *RedisState

func resetRedis() {
	var err error
	if redisServer != nil {
		redisServer.Close()
	}

	redisServer, err = miniredis.Run()
	if err != nil {
		panic(err)
	}

	redisState, err = NewRedisState(redisServer.Addr())
	// redisState, err = server.NewRedisState("localhost:6379")
	if err != nil {
		panic(err)
	}
}

func TestRedisStateSetup(t *testing.T) {
	var err error
	redisState, err = NewRedisState("localhost:18279")
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
	key := RedisKeyTxHashForSenderAndNonce(txFrom, nonce)
	expectedKey := fmt.Sprintf("%s%s_%d", RedisPrefixTxHashForSenderAndNonce, strings.ToLower(txFrom), nonce)
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

func TestNonceFixForAccount(t *testing.T) {
	var err error
	resetRedis()

	txFrom := "0x0Sender"

	numTimesSent, found, err := redisState.GetNonceFixForAccount(txFrom)
	require.Nil(t, err, err)
	require.False(t, found)
	require.Equal(t, uint64(0), numTimesSent)

	err = redisState.SetNonceFixForAccount(txFrom, 0)
	require.Nil(t, err, err)

	numTimesSent, found, err = redisState.GetNonceFixForAccount(txFrom)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(0), numTimesSent)

	err = redisState.DelNonceFixForAccount(txFrom)
	require.Nil(t, err, err)

	numTimesSent, found, err = redisState.GetNonceFixForAccount(txFrom)
	require.Nil(t, err, err)
	require.False(t, found)
	require.Equal(t, uint64(0), numTimesSent)

	err = redisState.SetNonceFixForAccount(txFrom, 17)
	require.Nil(t, err, err)

	numTimesSent, found, err = redisState.GetNonceFixForAccount(txFrom)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(17), numTimesSent)

	// Ensure it matches txFrom case-insensitive
	numTimesSent, found, err = redisState.GetNonceFixForAccount(strings.ToUpper(txFrom))
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(17), numTimesSent)
}

func TestSenderOfTxHash(t *testing.T) {
	var err error
	resetRedis()

	txFrom := "0x0Sender"
	txHash := "0xDeadBeef"

	val, found, err := redisState.GetSenderOfTxHash(txHash)
	require.Nil(t, err, err)
	require.False(t, found)
	require.Equal(t, "", val)

	err = redisState.SetSenderOfTxHash(txHash, txFrom)
	require.Nil(t, err, err)

	val, found, err = redisState.GetSenderOfTxHash(txHash)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, strings.ToLower(txFrom), val)
}

func TestSenderMaxNonce(t *testing.T) {
	var err error
	resetRedis()

	txFrom := "0x0Sender"

	val, found, err := redisState.GetSenderMaxNonce(txFrom)
	require.Nil(t, err, err)
	require.False(t, found)
	require.Equal(t, uint64(0), val)

	err = redisState.SetSenderMaxNonce(txFrom, 17)
	require.Nil(t, err, err)

	val, found, err = redisState.GetSenderMaxNonce(txFrom)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(17), val)

	err = redisState.SetSenderMaxNonce(txFrom, 16)
	require.Nil(t, err, err)

	val, found, err = redisState.GetSenderMaxNonce(txFrom)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(17), val)

	err = redisState.SetSenderMaxNonce(txFrom, 18)
	require.Nil(t, err, err)

	val, found, err = redisState.GetSenderMaxNonce(txFrom)
	require.Nil(t, err, err)
	require.True(t, found)
	require.Equal(t, uint64(18), val)
}

func TestWhitehatTx(t *testing.T) {
	resetRedis()
	bundleId := "123"

	// get (empty)
	txs, err := redisState.GetWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)
	require.Equal(t, 0, len(txs))

	// add #1
	tx1 := "0xa12345"
	tx2 := "0xb12345"
	err = redisState.AddTxToWhitehatBundle(bundleId, tx1)
	require.Nil(t, err, err)

	txs, err = redisState.GetWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)
	require.Equal(t, 1, len(txs))

	err = redisState.AddTxToWhitehatBundle(bundleId, tx2)
	require.Nil(t, err, err)

	txs, err = redisState.GetWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)
	require.Equal(t, 2, len(txs))
	require.Equal(t, tx2, txs[0])
	require.Equal(t, tx1, txs[1])

	err = redisState.DelWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)

	txs, err = redisState.GetWhitehatBundleTx(bundleId)
	require.Nil(t, err, err)
	require.Equal(t, 0, len(txs))
}
