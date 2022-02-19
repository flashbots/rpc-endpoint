package testutils

import "github.com/flashbots/rpc-endpoint/database"

type mockStore struct{}

func NewMockStore() database.Store {
	return &mockStore{}
}
func (m *mockStore) SaveRequestEntry(in *database.RequestEntry) error {
	return nil
}
func (m *mockStore) SaveEthSendRawTxEntry(in *database.EthSendRawTxEntry) error {
	return nil
}
