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
	txForwardedToRelay  map[string]time.Time      // key: txHash
	accountWithNonceFix map[string]*nonceFix      // key: txFrom
	userLatestTxHash    map[string]StringWithTime // key: txFrom, value: txHash
	txHashToUser        map[string]StringWithTime // key: txHash, value: txFrom
	txStatus            map[string]StringWithTime // key: txHash, value: txStatus

	userTxWithNonceSentToRelay map[string]BoolWithTime // key: txFrom_nonce
}

func NewGlobalState() *GlobalState {
	return &GlobalState{
		txForwardedToRelay:  make(map[string]time.Time),
		accountWithNonceFix: make(map[string]*nonceFix),
		userLatestTxHash:    make(map[string]StringWithTime),
		txHashToUser:        make(map[string]StringWithTime),
		txStatus:            make(map[string]StringWithTime),

		userTxWithNonceSentToRelay: make(map[string]BoolWithTime),
	}
}

func (s *GlobalState) cleanup() {
	// txForwardedToRelay should be kept around for 20 minutes, after which a user can resubmit
	for txHash, t := range s.txForwardedToRelay {
		if time.Since(t).Minutes() > 20 {
			delete(s.txForwardedToRelay, txHash)
		}
	}

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
