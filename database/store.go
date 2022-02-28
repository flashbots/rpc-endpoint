package database

type Store interface {
	SaveRequest(reqEntry *RequestEntry, rawTxEntries []*EthSendRawTxEntry)
	SaveRequestEntry(in *RequestEntry) error
	SaveRawTxEntries(in []*EthSendRawTxEntry) error
}
