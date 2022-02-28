package database

import (
	"github.com/google/uuid"
	"sync"
)

type memStore struct {
	Requests      map[uuid.UUID]*RequestEntry
	EthSendRawTxs map[uuid.UUID]*EthSendRawTxEntry
	mutex         *sync.Mutex
}

func NewMemStore() *memStore {
	return &memStore{
		Requests:      make(map[uuid.UUID]*RequestEntry),
		EthSendRawTxs: make(map[uuid.UUID]*EthSendRawTxEntry),
		mutex:         &sync.Mutex{},
	}
}

func (m *memStore) SaveRequestEntry(entry *RequestEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.Requests[entry.Id] = entry
	return nil
}

func (m *memStore) SaveRawTxEntries(entries []*EthSendRawTxEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, tx := range entries {
		m.EthSendRawTxs[tx.Id] = tx
	}
	return nil
}
