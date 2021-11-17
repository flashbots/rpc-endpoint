package server

import "time"

type StringWithTime struct {
	s string
	t time.Time
}

func NewStringWithTime(s string) StringWithTime {
	return StringWithTime{
		s: s,
		t: Now(),
	}
}

// todo: put into redis
type GlobalState struct {
	txForwardedToRelay  map[string]time.Time
	accountWithNonceFix map[string]time.Time
	userLatestTx        map[string]StringWithTime // key: txFrom, value: txHash
	txToUser            map[string]StringWithTime // key: txHash, value: txFrom
}

func NewGlobalState() *GlobalState {
	return &GlobalState{
		txForwardedToRelay:  make(map[string]time.Time),
		accountWithNonceFix: make(map[string]time.Time),
		userLatestTx:        make(map[string]StringWithTime),
		txToUser:            make(map[string]StringWithTime),
	}
}

func (s *GlobalState) cleanup() {
	// txForwardedToRelay should be kept around for 20 minutes, after which a user can resubmit
	for txHash, t := range s.txForwardedToRelay {
		if time.Since(t).Minutes() > 20 {
			delete(s.txForwardedToRelay, txHash)
		}
	}

	// invalid nonce should be sent for at most 1h
	for txFrom, t := range s.accountWithNonceFix {
		if time.Since(t).Hours() >= 1 {
			delete(s.accountWithNonceFix, txFrom)
		}
	}

	// tx history should expire after 4h
	for txHash, entry := range s.txToUser {
		if time.Since(entry.t).Hours() >= 4 {
			delete(s.accountWithNonceFix, txHash)
		}
	}
}
