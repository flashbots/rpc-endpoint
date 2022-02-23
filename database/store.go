package database

type Store interface {
	SaveRequestEntry(in *RequestEntry) error
	SaveRawTxEntries(in []*EthSendRawTxEntry) error
}
