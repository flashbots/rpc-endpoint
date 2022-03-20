package server

import (
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/google/uuid"
	"net/http"
	"sync"
)

type requestRecord struct {
	requestEntry        database.RequestEntry
	ethSendRawTxEntries []*database.EthSendRawTxEntry
	mutex               sync.Mutex
	reqPusher           *RequestPusher
}

func NewRequestRecord(reqPusher *RequestPusher) *requestRecord {
	return &requestRecord{
		reqPusher: reqPusher,
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
	r.requestEntry.IpHash = GetIPHash(req)
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
		r.reqPusher.EntryChan <- database.Entry{
			ReqEntry:     r.requestEntry,
			RawTxEntries: entries,
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
