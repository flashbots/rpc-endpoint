package database

type Store interface {
	SaveRequestEntry(in *RequestEntry) error
	SaveEthSendRawTxEntry(in *EthSendRawTxEntry) error
}
