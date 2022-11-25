package server

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/flashbots/rpc-endpoint/database"
	"github.com/google/uuid"
)

type requestRecord struct {
	requestEntry        database.RequestEntry
	ethSendRawTxEntries []*database.EthSendRawTxEntry
	mutex               sync.Mutex
	db                  database.Store
}

func NewRequestRecord(db database.Store) *requestRecord {
	return &requestRecord{
		db: db,
	}
}

func (r *requestRecord) AddEthSendRawTxEntry(id uuid.UUID) *database.EthSendRawTxEntry {
	entry := &database.EthSendRawTxEntry{
		Id:        id,
		RequestId: r.requestEntry.Id,
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.ethSendRawTxEntries = append(r.ethSendRawTxEntries, entry)
	return entry
}

func (r *requestRecord) UpdateRequestEntry(req *http.Request, reqStatus int, error string) {
	r.requestEntry.HttpMethod = req.Method
	r.requestEntry.Error = error
	r.requestEntry.HttpUrl = req.URL.Path
	r.requestEntry.HttpQueryParam = req.URL.RawQuery
	r.requestEntry.HttpResponseStatus = reqStatus
	r.requestEntry.Origin = req.Header.Get("Origin")
	r.requestEntry.Host = req.Header.Get("Host")
}

// SaveRecord will insert both requestRecord and rawTxEntries to db
func (r *requestRecord) SaveRecord() error {
	entries := r.getValidRawTxEntriesToSave()
	if len(entries) > 0 { // Save entries if the request contains rawTxEntries
		if err := r.db.SaveRequestEntry(r.requestEntry); err != nil {
			return fmt.Errorf("SaveRequestEntry failed %v", err)
		}
		if err := r.db.SaveRawTxEntries(entries); err != nil {
			return fmt.Errorf("SaveRawTxEntries failed %v", err)
		}
	}
	return nil
}

// getValidRawTxEntriesToSave returns list of rawTxEntry which are either sent to relay or mempool or entry with error
func (r *requestRecord) getValidRawTxEntriesToSave() []*database.EthSendRawTxEntry {
	entries := make([]*database.EthSendRawTxEntry, 0, len(r.ethSendRawTxEntries))
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, entry := range r.ethSendRawTxEntries {
		if entry.ErrorCode != 0 || entry.WasSentToRelay || entry.WasSentToMempool {
			entries = append(entries, entry)
		}
	}
	return entries
}
