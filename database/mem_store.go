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

func (m *memStore) SaveRequestEntry(in *RequestEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.Requests[in.Id] = in
	return nil
}

func (m *memStore) SaveRawTxEntries(in []*EthSendRawTxEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, tx := range in {
		m.EthSendRawTxs[tx.Id] = tx
	}
	return nil
}
