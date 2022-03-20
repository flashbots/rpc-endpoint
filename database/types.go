package database

import (
	"github.com/google/uuid"
	"time"
)

// RequestEntry to store each request
type RequestEntry struct {
	Id                 uuid.UUID `db:"id"`
	ReceivedAt         time.Time `db:"received_at"`
	InsertedAt         time.Time `db:"inserted_at"`
	RequestDurationMs  int64     `db:"request_duration_ms"`
	IsBatchRequest     bool      `db:"is_batch_request"`
	NumRequestInBatch  int       `db:"num_request_in_batch"`
	HttpMethod         string    `db:"http_method"`
	HttpUrl            string    `db:"http_url"`
	HttpQueryParam     string    `db:"http_query_param"`
	HttpResponseStatus int       `db:"http_response_status"`
	IpHash             string    `db:"ip_hash"`
	Origin             string    `db:"origin"`
	Host               string    `db:"host"`
	Error              string    `db:"error"`
}

// EthSendRawTxEntry to store each eth_sendRawTransaction calls
type EthSendRawTxEntry struct {
	Id                          uuid.UUID `db:"id"`
	RequestId                   uuid.UUID `db:"request_id"` // id from RequestEntry table
	InsertedAt                  time.Time `db:"inserted_at"`
	IsOnOafcList                bool      `db:"is_on_oafc_list"`
	IsWhiteHatBundleCollection  bool      `db:"is_white_hat_bundle_collection"`
	WhiteHatBundleId            string    `db:"white_hat_bundle_id"`
	IsCancelTx                  bool      `db:"is_cancel_tx"`
	NeedsFrontRunningProtection bool      `db:"needs_front_running_protection"`
	WasSentToRelay              bool      `db:"was_sent_to_relay"`
	WasSentToMempool            bool      `db:"was_sent_to_mempool"`
	IsBlockedBczAlreadySent     bool      `db:"is_blocked_bcz_already_sent"`
	Error                       string    `db:"error"`
	ErrorCode                   int       `db:"error_code"`
	TxRaw                       string    `db:"tx_raw"`
	TxHash                      string    `db:"tx_hash"`
	TxFrom                      string    `db:"tx_from"`
	TxTo                        string    `db:"tx_to"`
	TxNonce                     int       `db:"tx_nonce"`
	TxData                      string    `db:"tx_data"`
	TxSmartContractMethod       string    `db:"tx_smart_contract_method"`
}

type Entry struct {
	ReqEntry     RequestEntry
	RawTxEntries []*EthSendRawTxEntry
}
