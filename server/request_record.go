package server

import (
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/google/uuid"
	"net/http"
	"time"
)

type RequestRecord struct {
	logger            log.Logger
	requestEntry      *types.RequestEntry
	ethSendRawTxEntry *types.EthSendRawTxEntry
}

func NewRequestRecord() *RequestRecord {
	// If the incoming request exited in the server then below id will be used for RequestEntry
	// If not, below id will be replaced by uid if the request forwarded to request handler
	id := uuid.New()
	return &RequestRecord{
		logger:            log.New(),
		requestEntry:      &types.RequestEntry{Id: id},
		ethSendRawTxEntry: &types.EthSendRawTxEntry{},
	}
}

func (r *RequestRecord) SaveRequestEntryToDB() {
	r.requestEntry.InsertedAt = time.Now() // this will be updated while adding db integration
	r.logger.Info("[RequestRecord] SaveRequestEntryToDB called", "requestEntry", r.requestEntry)
}
func (r *RequestRecord) SaveEthSendRawTxEntryToDB() {
	r.logger.Info("[RequestRecord] SaveEthSendRawTxEntryToDB called", "EthSendRawTxEntry", r.ethSendRawTxEntry)
}

func (r *RequestRecord) UpdateRequestEntry(req *http.Request, reqStatus int, error string) {
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
