package database

type Store interface {
	SaveRequestEntry(in *RequestEntry) error
	SaveEthSendRawTxEntries(in []*EthSendRawTxEntry) error
}
