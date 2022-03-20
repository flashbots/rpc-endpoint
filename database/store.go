package database

type Store interface {
	SaveRequestEntries(entries []RequestEntry) error
	SaveRawTxEntries(entries [][]*EthSendRawTxEntry) error
}
