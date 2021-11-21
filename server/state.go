package server

import (
	"strings"
	"time"
)

type nonceFix struct {
	createdAt time.Time
	txHash    string
	numTries  uint64
}

func NewNonceFix(txHash string) *nonceFix {
	return &nonceFix{
		createdAt: Now(),
		txHash:    strings.ToLower(txHash),
	}
}

// todo: put into redis
type GlobalState struct {
	accountWithNonceFix map[string]*nonceFix      // key: txFrom
	userLatestTxHash    map[string]StringWithTime // key: txFrom, value: txHash
	txHashToUser        map[string]StringWithTime // key: txHash, value: txFrom
	txStatus            map[string]StringWithTime // key: txHash, value: txStatus

	userTxWithNonceSentToRelay map[string]BoolWithTime // key: txFrom_nonce
}

func NewGlobalState() *GlobalState {
	return &GlobalState{
		accountWithNonceFix: make(map[string]*nonceFix),
		userLatestTxHash:    make(map[string]StringWithTime),
		txHashToUser:        make(map[string]StringWithTime),
		txStatus:            make(map[string]StringWithTime),

		userTxWithNonceSentToRelay: make(map[string]BoolWithTime),
	}
}

func (s *GlobalState) cleanup() {
	// invalid nonce should be sent for 1h max
	for txFrom, nonceFix := range s.accountWithNonceFix {
		if time.Since(nonceFix.createdAt).Hours() >= 1 {
			delete(s.accountWithNonceFix, txFrom)
		}
	}

	// txHistory should expire after 4h
	for txHash, entry := range s.txHashToUser {
		if time.Since(entry.t).Hours() >= 4 {
			delete(s.txHashToUser, txHash)
		}
	}

	// userLatestTx should expire after 4h
	for txFrom, entry := range s.userLatestTxHash {
		if time.Since(entry.t).Hours() >= 4 {
			delete(s.userLatestTxHash, txFrom)
		}
	}

	// txStatus should expire after 1h
	for txHash, entry := range s.txStatus {
		if time.Since(entry.t).Hours() >= 1 {
			delete(s.txStatus, txHash)
		}
	}

	// userTxWithNonceSentToRelay should expire after 2h
	for key, entry := range s.userTxWithNonceSentToRelay {
		if time.Since(entry.t).Hours() >= 2 {
			delete(s.userTxWithNonceSentToRelay, key)
		}
	}
}
