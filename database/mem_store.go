package database

import (
	"github.com/google/uuid"
	"sync"
)

type memStore struct {
	requests      map[uuid.UUID]*RequestEntry
	ethSendRawTxs map[uuid.UUID]*EthSendRawTxEntry
	mutex         *sync.Mutex
}

func NewMemStore(requests map[uuid.UUID]*RequestEntry, ethSendRawTxs map[uuid.UUID]*EthSendRawTxEntry) Store {
	return &memStore{
		requests:      requests,
		ethSendRawTxs: ethSendRawTxs,
		mutex:         &sync.Mutex{},
	}
}

func (m *memStore) SaveRequestEntry(in *RequestEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.requests[in.Id] = in
	return nil
}

func (m *memStore) SaveEthSendRawTxEntry(in *EthSendRawTxEntry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.ethSendRawTxs[in.Id] = in
	return nil
}
