package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
)

var RedisPrefix = "rpc-endpoint:"

// Enable lookup of timeSentToRelay by txHash
var RedisPrefixTxSentToRelay = RedisPrefix + "tx-sent-to-relay:"
var RedisExpiryTxSentToRelay = time.Duration(24 * time.Hour) // 1 day

// Enable lookup of txHash by txFrom+nonce (only if sent to relay)
var RedisPrefixTxHashForSenderAndNonce = RedisPrefix + "txsender-and-nonce-to-txhash:"
var RedisExpiryTxHashForSenderAndNonce = time.Duration(24 * time.Hour) // 1 day

// nonce-fix of an account (with number of times sent)
var RedisPrefixNonceFixForAccount = RedisPrefix + "txsender-with-nonce-fix:"
var RedisExpiryNonceFixForAccount = time.Duration(24 * time.Hour)

// Enable lookup of txFrom by txHash
var RedisPrefixSenderOfTxHash = RedisPrefix + "txsender-of-txhash:"
var RedisExpirySenderOfTxHash = time.Duration(24 * time.Hour) // 1 day

// Remember nonce of pending user tx
var RedisPrefixSenderMaxNonce = RedisPrefix + "txsender-pending-max-nonce:"
var RedisExpirySenderMaxNonce = time.Duration(2 * time.Hour)

// Enable lookup of bundle txs by bundleId
var RedisPrefixWhitehatBundleTransactions = RedisPrefix + "tx-for-whitehat-bundle:"
var RedisExpiryWhitehatBundleTransactions = time.Duration(24 * time.Hour) // 1 day

// // Enable lookup of last privateTransaction-txHash sent by txFrom
// var RedisPrefixLastPrivTxHashOfAccount = RedisPrefix + "last-txhash-of-txsender:"
// var RedisExpiryLastPrivTxHashOfAccount = time.Duration(24 * time.Hour) // 1 day

func RedisKeyTxSentToRelay(txHash string) string {
	return RedisPrefixTxSentToRelay + strings.ToLower(txHash)
}

func RedisKeyTxHashForSenderAndNonce(txFrom string, nonce uint64) string {
	return fmt.Sprintf("%s%s_%d", RedisPrefixTxHashForSenderAndNonce, strings.ToLower(txFrom), nonce)
}

func RedisKeyNonceFixForAccount(txFrom string) string {
	return RedisPrefixNonceFixForAccount + strings.ToLower(txFrom)
}

func RedisKeySenderOfTxHash(txHash string) string {
	return RedisPrefixSenderOfTxHash + strings.ToLower(txHash)
}

func RedisKeySenderMaxNonce(txFrom string) string {
	return RedisPrefixSenderMaxNonce + strings.ToLower(txFrom)
}

func RedisKeyWhitehatBundleTransactions(bundleId string) string {
	return RedisPrefixWhitehatBundleTransactions + strings.ToLower(bundleId)
}

// func RedisKeyLastPrivTxHashOfAccount(txFrom string) string {
// 	return RedisPrefixLastPrivTxHashOfAccount + strings.ToLower(txFrom)
// }

type RedisState struct {
	RedisClient *redis.Client
}

func NewRedisState(redisUrl string) (*RedisState, error) {
	// Setup redis client and check connection
	redisClient := redis.NewClient(&redis.Options{Addr: redisUrl})

	// Try to get a key to see if there's an error with the connection
	if err := redisClient.Get(context.Background(), "somekey").Err(); err != nil && err != redis.Nil {
		return nil, errors.Wrap(err, "redis init error")
	}

	// Create and return the RedisState
	return &RedisState{
		RedisClient: redisClient,
	}, nil
}

//
// Enable lookup of timeSentToRelay by txHash
//
func (s *RedisState) SetTxSentToRelay(txHash string) error {
	key := RedisKeyTxSentToRelay(txHash)
	err := s.RedisClient.Set(context.Background(), key, Now().UTC().Unix(), RedisExpiryTxSentToRelay).Err()
	return err
}

func (s *RedisState) GetTxSentToRelay(txHash string) (timeSent time.Time, found bool, err error) {
	key := RedisKeyTxSentToRelay(txHash)
	val, err := s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return time.Time{}, false, nil // just not found
	} else if err != nil {
		return time.Time{}, false, err // found but error
	}

	timestampInt, err := strconv.Atoi(val)
	if err != nil {
		return time.Time{}, true, err // found but error
	}

	t := time.Unix(int64(timestampInt), 0)
	return t, true, nil
}

//
// Enable lookup of txHash by txFrom+nonce
//
func (s *RedisState) SetTxHashForSenderAndNonce(txFrom string, nonce uint64, txHash string) error {
	key := RedisKeyTxHashForSenderAndNonce(txFrom, nonce)
	err := s.RedisClient.Set(context.Background(), key, strings.ToLower(txHash), RedisExpiryTxHashForSenderAndNonce).Err()
	return err
}

func (s *RedisState) GetTxHashForSenderAndNonce(txFrom string, nonce uint64) (txHash string, found bool, err error) {
	key := RedisKeyTxHashForSenderAndNonce(txFrom, nonce)
	txHash, err = s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return "", false, nil // not found
	} else if err != nil {
		return "", false, err
	}

	return txHash, true, nil
}

//
// nonce-fix per account
//
func (s *RedisState) SetNonceFixForAccount(txFrom string, numTimesSent uint64) error {
	key := RedisKeyNonceFixForAccount(txFrom)
	err := s.RedisClient.Set(context.Background(), key, numTimesSent, RedisExpiryNonceFixForAccount).Err()
	return err
}

func (s *RedisState) DelNonceFixForAccount(txFrom string) error {
	key := RedisKeyNonceFixForAccount(txFrom)
	err := s.RedisClient.Del(context.Background(), key).Err()
	return err
}

func (s *RedisState) GetNonceFixForAccount(txFrom string) (numTimesSent uint64, found bool, err error) {
	key := RedisKeyNonceFixForAccount(txFrom)
	val, err := s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return 0, false, nil // not found
	} else if err != nil {
		return 0, false, err
	}

	numTimesSent, err = strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, true, err
	}
	return numTimesSent, true, nil
}

//
// Enable lookup of txFrom by txHash
//
func (s *RedisState) SetSenderOfTxHash(txHash string, txFrom string) error {
	key := RedisKeySenderOfTxHash(txHash)
	err := s.RedisClient.Set(context.Background(), key, strings.ToLower(txFrom), RedisExpirySenderOfTxHash).Err()
	return err
}

func (s *RedisState) GetSenderOfTxHash(txHash string) (txSender string, found bool, err error) {
	key := RedisKeySenderOfTxHash(txHash)
	txSender, err = s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil { // not found
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	return strings.ToLower(txSender), true, nil
}

//
// Enable lookup of tx bundles by bundle ID
//
func (s *RedisState) AddTxToWhitehatBundle(bundleId string, signedTx string) error {
	key := RedisKeyWhitehatBundleTransactions(bundleId)

	// Add item
	err := s.RedisClient.LPush(context.Background(), key, signedTx).Err()
	if err != nil {
		return err
	}

	// Set expiry
	err = s.RedisClient.Expire(context.Background(), key, RedisExpiryWhitehatBundleTransactions).Err()
	if err != nil {
		return err
	}

	// Limit to 15 entries
	err = s.RedisClient.LTrim(context.Background(), key, 0, 15).Err()
	return err
}

func (s *RedisState) GetWhitehatBundleTx(bundleId string) ([]string, error) {
	key := RedisKeyWhitehatBundleTransactions(bundleId)
	return s.RedisClient.LRange(context.Background(), key, 0, -1).Result()
}

func (s *RedisState) DelWhitehatBundleTx(bundleId string) error {
	key := RedisKeyWhitehatBundleTransactions(bundleId)
	return s.RedisClient.Del(context.Background(), key).Err()
}

//
// Enable lookup of last txHash sent by txFrom
//
// func (s *RedisState) SetLastPrivTxHashOfAccount(txFrom string, txHash string) error {
// 	key := RedisKeyLastPrivTxHashOfAccount(txFrom)
// 	err := s.RedisClient.Set(context.Background(), key, strings.ToLower(txHash), RedisExpiryLastPrivTxHashOfAccount).Err()
// 	return err
// }

// func (s *RedisState) GetLastPrivTxHashOfAccount(txFrom string) (txHash string, found bool, err error) {
// 	key := RedisKeyLastPrivTxHashOfAccount(txFrom)
// 	txHash, err = s.RedisClient.Get(context.Background(), key).Result()
// 	if err == redis.Nil { // not found
// 		return "", false, nil
// 	} else if err != nil {
// 		return "", false, err
// 	}

// 	return strings.ToLower(txHash), true, nil
// }

func (s *RedisState) SetSenderMaxNonce(txFrom string, nonce uint64) error {
	prevMaxNonce, found, err := s.GetSenderMaxNonce(txFrom)
	if err != nil {
		return err
	}

	// Do nothing if current nonce is not higher than already existing
	if found && prevMaxNonce >= nonce {
		return nil
	}

	key := RedisKeySenderMaxNonce(txFrom)
	err = s.RedisClient.Set(context.Background(), key, nonce, RedisExpirySenderMaxNonce).Err()
	return err
}

func (s *RedisState) GetSenderMaxNonce(txFrom string) (senderMaxNonce uint64, found bool, err error) {
	key := RedisKeySenderMaxNonce(txFrom)
	val, err := s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return 0, false, nil // not found
	} else if err != nil {
		return 0, false, err
	}

	senderMaxNonce, err = strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, true, err
	}
	return senderMaxNonce, true, nil
}
