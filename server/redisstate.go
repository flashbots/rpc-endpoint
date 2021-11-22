package server

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

var RedisPrefix = "rpc-endpoint:request:"
var RedisPrefixTxSentToRelay = RedisPrefix + "tx-sent-to-relay:"
var RedisExpiryTxSentToRelay = time.Duration(24 * time.Hour) // 1 day

func RedisKeyTxSentToRelay(txHash string) string {
	return RedisPrefixTxSentToRelay + txHash
}

type RedisState struct {
	RedisClient *redis.Client
}

func NewRedisState(redisUrl string) (*RedisState, error) {
	// Setup redis client and check connection
	redisClient := redis.NewClient(&redis.Options{Addr: redisUrl})
	if err := redisClient.Get(context.Background(), "foo").Err(); err != nil && err != redis.Nil {
		return nil, err
	}
	return &RedisState{
		RedisClient: redisClient,
	}, nil
}

func (s *RedisState) RedisGetStr(key string) (string, error) {
	val, err := s.RedisClient.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return val, nil
}

func (s *RedisState) GetStrOrLogError(key string) string {
	val, err := s.RedisGetStr(key)
	if err != nil {
		log.Printf("Redis error getting key %s: %s", key, err)
	}
	return val
}

func (s *RedisState) SetTxSentToRelay(txHash string) error {
	key := RedisKeyTxSentToRelay(strings.ToLower(txHash))
	err := s.RedisClient.Set(context.Background(), key, time.Now().UTC().Unix(), RedisExpiryTxSentToRelay).Err()
	return err
}

func (s *RedisState) GetTxSentToRelay(txHash string) (timeSent time.Time, found bool, err error) {
	key := RedisKeyTxSentToRelay(strings.ToLower(txHash))
	timestamp, err := s.RedisGetStr(key)
	if err == redis.Nil {
		return time.Time{}, false, nil // just not found
	} else if err != nil {
		return time.Time{}, true, err // found but error
	}

	timestampInt, err := strconv.Atoi(timestamp)
	if err != nil {
		return time.Time{}, true, err // found but error
	}

	t := time.Unix(int64(timestampInt), 0)
	return t, true, nil
}
