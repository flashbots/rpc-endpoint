package server

import (
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/google/uuid"
	"net/http"
	"sync"
)

type requestRecord struct {
	requestEntry        *database.RequestEntry
	ethSendRawTxEntries []*database.EthSendRawTxEntry
	mutex               *sync.Mutex
}

func NewRequestRecord() *requestRecord {
	return &requestRecord{
		requestEntry:        &database.RequestEntry{},
		ethSendRawTxEntries: make([]*database.EthSendRawTxEntry, 0),
		mutex:               &sync.Mutex{},
	}
}

func (r *requestRecord) AddEthSendRawTxEntry(jsonReq *types.JsonRpcRequest, id, requestId uuid.UUID) *database.EthSendRawTxEntry {
	var entry *database.EthSendRawTxEntry
	if jsonReq.Method == "eth_sendRawTransaction" {
		entry = &database.EthSendRawTxEntry{
			Id:        id,
			RequestId: requestId,
		}
		r.mutex.Lock()
		defer r.mutex.Unlock()
		r.ethSendRawTxEntries = append(r.ethSendRawTxEntries, entry)
	}
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
