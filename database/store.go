package database

type Store interface {
	SaveRequest(reqEntry *RequestEntry, rawTxEntries []*EthSendRawTxEntry)
	SaveRequestEntry(entry *RequestEntry) error
	SaveRawTxEntries(entries []*EthSendRawTxEntry) error
}
