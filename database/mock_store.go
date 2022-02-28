package database

type mockStore struct{}

func NewMockStore() Store {
	return &mockStore{}
}

func (m *mockStore) SaveRequest(reqEntry *RequestEntry, rawTxEntries []*EthSendRawTxEntry) {}

func (m *mockStore) SaveRequestEntry(in *RequestEntry) error {
	return nil
}
func (m *mockStore) SaveRawTxEntries(in []*EthSendRawTxEntry) error {
	return nil
}
