package server

import (
	"time"

	"github.com/flashbots/rpc-endpoint/rpctypes"
)

// todo: put into redis
type GlobalState struct {
	// userLatestTxHash map[string]StringWithTime // key: txFrom, value: txHash
	txHashToUser map[string]rpctypes.StringWithTime // key: txHash, value: txFrom
	// txStatus     map[string]StringWithTime // key: txHash, value: txStatus
}

func NewGlobalState() *GlobalState {
	return &GlobalState{
		// userLatestTxHash: make(map[string]StringWithTime),
		txHashToUser: make(map[string]rpctypes.StringWithTime),
		// txStatus:     make(map[string]StringWithTime),
	}
}

func (s *GlobalState) cleanup() {
	// txHistory should expire after 4h
	for txHash, entry := range s.txHashToUser {
		if time.Since(entry.T).Hours() >= 4 {
			delete(s.txHashToUser, txHash)
		}
	}

	// // userLatestTx should expire after 4h
	// for txFrom, entry := range s.userLatestTxHash {
	// 	if time.Since(entry.t).Hours() >= 4 {
	// 		delete(s.userLatestTxHash, txFrom)
	// 	}
	// }

	// txStatus should expire after 1h
	// for txHash, entry := range s.txStatus {
	// 	if time.Since(entry.t).Hours() >= 1 {
	// 		delete(s.txStatus, txHash)
	// 	}
	// }
}
