package database

type Store interface {
	SaveRequestEntry(entry RequestEntry) error
	SaveRawTxEntries(entries []*EthSendRawTxEntry) error
}
