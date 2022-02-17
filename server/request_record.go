package server

import (
	"context"
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

func (r *RequestRecord) SaveRequestEntryToDB(ctx context.Context) {
	r.requestEntry.InsertedAt = time.Now()
	r.logger.Info("[RequestRecord] requestEntry", "model", r.requestEntry)

}
func (r *RequestRecord) SaveEthSendRawTxEntryToDB(ctx context.Context) {
	r.logger.Info("[RequestRecord] eth_sendRawTxDBModel", "model", r.ethSendRawTxEntry)
}

func (r *RequestRecord) UpdateRequestEntry(req *http.Request, reqStatus int, error string) {
	// TODO:Error should be converted to enum
	r.requestEntry.ReceivedAt = time.Now()
	r.requestEntry.HttpMethod = req.Method
	r.requestEntry.Ip = GetIP(req)
	r.requestEntry.Error = error
	// TODO: handle url + query param
	r.requestEntry.HttpUrl = ""
	r.requestEntry.HttpResponseStatus = reqStatus
	r.requestEntry.Origin = req.Header.Get("Origin")
	r.requestEntry.Host = req.Header.Get("Host")
}
