package server

import "time"

// todo: put into redis
type GlobalState struct {
	txForwardedToRelay map[string]time.Time
}

func NewGlobalState() *GlobalState {
	return &GlobalState{
		txForwardedToRelay: make(map[string]time.Time),
	}
}

func (s *GlobalState) cleanup() {
	// txForwardedToRelay should be kept around for 20 minutes, after which a user can resubmit
	for txHash, t := range s.txForwardedToRelay {
		if time.Since(t).Minutes() > 20 {
			delete(s.txForwardedToRelay, txHash)
		}
	}
}
