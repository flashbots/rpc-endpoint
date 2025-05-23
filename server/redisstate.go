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
var RedisExpiryTxSentToRelay = 24 * time.Hour // 1 day

// Enable lookup of txHash by txFrom+nonce (only if sent to relay)
var RedisPrefixTxHashForSenderAndNonce = RedisPrefix + "txsender-and-nonce-to-txhash:"
var RedisExpiryTxHashForSenderAndNonce = 24 * time.Hour // 1 day

// nonce-fix of an account (with number of times sent)
var RedisPrefixNonceFixForAccount = RedisPrefix + "txsender-with-nonce-fix:"
var RedisExpiryNonceFixForAccount = 24 * time.Hour // 1 day

// Enable lookup of txFrom by txHash
var RedisPrefixSenderOfTxHash = RedisPrefix + "txsender-of-txhash:"
var RedisExpirySenderOfTxHash = 24 * time.Hour // 1 day

// Enable lookup of txNonce by txHash
var RedisPrefixNonceOfTxHash = RedisPrefix + "txnonce-of-txhash:"
var RedisExpiryNonceOfTxHash = 24 * time.Hour // 1 day

// Remember nonce of pending user tx
var RedisPrefixSenderMaxNonce = RedisPrefix + "txsender-pending-max-nonce:"
var RedisExpirySenderMaxNonce = 280 * time.Second //weird time to be a little less than 5 minute default blockrange

// Enable lookup of bundle txs by bundleId
var RedisPrefixWhitehatBundleTransactions = RedisPrefix + "tx-for-whitehat-bundle:"
var RedisExpiryWhitehatBundleTransactions = 24 * time.Hour // 1 day

// Enable lookup of bundle txs by bundleId
var RedisPrefixBlockedTxHash = RedisPrefix + "blocked-tx-hash:"
var RedisExpiryBlockedTxHash = 24 * time.Hour // 1 day

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

func RedisKeyNonceOfTxHash(txHash string) string {
	return RedisPrefixNonceOfTxHash + strings.ToLower(txHash)
}

func RedisKeySenderMaxNonce(txFrom string) string {
	return RedisPrefixSenderMaxNonce + strings.ToLower(txFrom)
}

func RedisKeyWhitehatBundleTransactions(bundleId string) string {
	return RedisPrefixWhitehatBundleTransactions + strings.ToLower(bundleId)
}

func RedisKeyBlockedTxHash(txHash string) string {
	return RedisPrefixBlockedTxHash + strings.ToLower(txHash)
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

// Enable lookup of timeSentToRelay by txHash
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

// Enable lookup of txHash by txFrom+nonce
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

// nonce-fix per account
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

// Enable lookup of txFrom and nonce by txHash
func (s *RedisState) SetSenderAndNonceOfTxHash(txHash string, txFrom string, txNonce uint64) error {
	key := RedisKeySenderOfTxHash(txHash)
	err := s.RedisClient.Set(context.Background(), key, strings.ToLower(txFrom), RedisExpirySenderOfTxHash).Err()
	if err != nil {
		return err
	}
	key = RedisKeyNonceOfTxHash(txHash)
	err = s.RedisClient.Set(context.Background(), key, txNonce, RedisExpiryNonceOfTxHash).Err()
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

func (s *RedisState) GetNonceOfTxHash(txHash string) (txNonce uint64, found bool, err error) {
	key := RedisKeyNonceOfTxHash(txHash)
	val, err := s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return 0, false, nil
	}

	txNonce, err = strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, true, err
	}
	return txNonce, true, nil
}

// Enable lookup of tx bundles by bundle ID
func (s *RedisState) AddTxToWhitehatBundle(bundleId string, signedTx string) error {
	key := RedisKeyWhitehatBundleTransactions(bundleId)

	// Check if item already exists
	txs, err := s.GetWhitehatBundleTx(bundleId)
	if err == nil {
		for _, tx := range txs {
			if signedTx == tx {
				return nil
			}
		}
	}

	// Add item
	err = s.RedisClient.LPush(context.Background(), key, signedTx).Err()
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

func (s *RedisState) SetSenderMaxNonce(txFrom string, nonce uint64, blockRange int) error {
	prevMaxNonce, found, err := s.GetSenderMaxNonce(txFrom)
	if err != nil {
		return err
	}

	// Do nothing if current nonce is not higher than already existing
	if found && prevMaxNonce >= nonce {
		return nil
	}

	expiry := RedisExpirySenderMaxNonce
	if blockRange > 0 {
		expiry = 12 * time.Duration(blockRange) * time.Second
	}

	key := RedisKeySenderMaxNonce(txFrom)
	err = s.RedisClient.Set(context.Background(), key, nonce, expiry).Err()
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

func (s *RedisState) DelSenderMaxNonce(txFrom string) error {
	key := RedisKeySenderMaxNonce(txFrom)
	return s.RedisClient.Del(context.Background(), key).Err()
}

// Block transactions, with a specific return value (eg. "nonce too low")
func (s *RedisState) SetBlockedTxHash(txHash string, returnValue string) error {
	key := RedisKeyBlockedTxHash(txHash)
	err := s.RedisClient.Set(context.Background(), key, returnValue, RedisExpiryBlockedTxHash).Err()
	return err
}

func (s *RedisState) GetBlockedTxHash(txHash string) (returnValue string, found bool, err error) {
	key := RedisKeyBlockedTxHash(txHash)
	returnValue, err = s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil { // not found
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	return returnValue, true, nil
}
