package server

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/google/uuid"
	"net/http"
	"sync"
)

type requestRecord struct {
	requestEntry        *database.RequestEntry
	ethSendRawTxEntries []*database.EthSendRawTxEntry
	mutex               *sync.Mutex
	db                  database.Store
}

func NewRequestRecord(db database.Store) *requestRecord {
	return &requestRecord{
		requestEntry:        &database.RequestEntry{},
		ethSendRawTxEntries: make([]*database.EthSendRawTxEntry, 0),
		mutex:               &sync.Mutex{},
		db:                  db,
	}
}

func (r *requestRecord) AddEthSendRawTxEntry(id, requestId uuid.UUID) *database.EthSendRawTxEntry {
	entry := &database.EthSendRawTxEntry{
		Id:        id,
		RequestId: requestId,
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.ethSendRawTxEntries = append(r.ethSendRawTxEntries, entry)
	return entry
}

func (r *requestRecord) UpdateRequestEntry(req *http.Request, reqStatus int, error string) {
	// TODO:Error should be converted to enum
	r.requestEntry.HttpMethod = req.Method
	r.requestEntry.IpHash = GetIPHash(req)
	r.requestEntry.Error = error
	r.requestEntry.HttpUrl = req.URL.Path
	r.requestEntry.HttpQueryParam = req.URL.RawQuery
	r.requestEntry.HttpResponseStatus = reqStatus
	r.requestEntry.Origin = req.Header.Get("Origin")
	r.requestEntry.Host = req.Header.Get("Host")
}

func (r *requestRecord) SaveRecord() {
	if len(r.ethSendRawTxEntries) > 0 { // Save entries if the request contains rawTxEntries
		if err := r.db.SaveRequestEntry(r.requestEntry); err != nil {
			log.Error("[saveRecord] SaveRequestEntry failed", "id", r.requestEntry.Id, "error", err)
			return
		}
		if err := r.db.SaveRawTxEntries(r.ethSendRawTxEntries); err != nil {
			log.Error("[saveRecord] SaveRawTxEntries failed", "requestId", r.requestEntry.Id, "error", err)
		}
	}
}
