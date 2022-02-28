package database

type mockStore struct{}

func NewMockStore() Store {
	return &mockStore{}
}

func (m *mockStore) SaveRequest(reqEntry *RequestEntry, rawTxEntries []*EthSendRawTxEntry) {}

func (m *mockStore) SaveRequestEntry(entry *RequestEntry) error {
	return nil
}
func (m *mockStore) SaveRawTxEntries(entries []*EthSendRawTxEntry) error {
	return nil
}
