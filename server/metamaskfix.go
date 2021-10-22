package server

import (
	"time"

	"github.com/ethereum/go-ethereum/core/types"
)

// MetaMask keeps re-sending tx, bombarding the system with eth_sendRawTransaction calls. If this happens, we prevent
// the tx from being forwarded to the TxManager, and force MetaMask to return an error (using eth_getTransactionCount).
type metaMaskFixer struct {
	blacklistedRawTx         map[string]time.Time       // prevents rawTx from being resubmitted to BE. key: lower(tx.Hash().Hex())
	accountAndNonce          map[string]*mmNonceHelper  // tracks the last good nonce, used to send wrong nonce to user. key: lower(tx.From)
	rawTransactionSubmission map[string]*mmRawTxTracker // tracks time when a rawTx was submitted. key: lower(tx.Hash().Hex())
}

func NewMetaMaskFixer() metaMaskFixer {
	return metaMaskFixer{
		blacklistedRawTx:         make(map[string]time.Time),
		accountAndNonce:          make(map[string]*mmNonceHelper),
		rawTransactionSubmission: make(map[string]*mmRawTxTracker),
	}
}

func (mmf *metaMaskFixer) CleanupStaleEntries() {
	for key, entry := range mmf.rawTransactionSubmission {
		if time.Since(entry.submittedAt) > 4*time.Hour {
			delete(mmf.rawTransactionSubmission, key)
			delete(mmf.accountAndNonce, entry.txFrom)
		}
	}

	for key, entry := range mmf.blacklistedRawTx {
		if time.Since(entry) > 4*time.Hour {
			delete(mmf.blacklistedRawTx, key)
		}
	}
}

// Store the correct nonce and number of calls to eth_getTransactionCount, in order to
// deliver a wrong nonce up to 4 times, for MetaMask to show a failed status and get unstuck.
type mmNonceHelper struct {
	Nonce    uint64
	NumTries uint64
}

type mmRawTxTracker struct {
	submittedAt time.Time // tracks time when a rawTx was submitted
	tx          *types.Transaction
	txFrom      string
}
