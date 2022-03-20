package database

import (
	"sync"
)

type memStore struct {
	Requests      []RequestEntry
	EthSendRawTxs []*EthSendRawTxEntry
	mutex         sync.Mutex
}

func NewMemStore() *memStore {
	return &memStore{
		mutex: sync.Mutex{},
	}
}

func (m *memStore) SaveRequestEntries(entries []RequestEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.Requests = append(m.Requests, entries...)
	return nil

}

func (m *memStore) SaveRawTxEntries(entries [][]*EthSendRawTxEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, entryOuter := range entries {
		m.EthSendRawTxs = append(m.EthSendRawTxs, entryOuter...)
	}
	return nil
}
