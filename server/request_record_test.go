package server

import (
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func Test_requestRecord_getForwardedRawTxEntries(t *testing.T) {
	tests := map[string]struct {
		ethSendRawTxEntries []*database.EthSendRawTxEntry
		want                []*database.EthSendRawTxEntry
		len                 int
	}{
		"Should return ethSendRawTxEntries when request sent to mempool": {
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{WasSentToMempool: true}, {WasSentToMempool: true}},
			want:                []*database.EthSendRawTxEntry{{WasSentToMempool: true}, {WasSentToMempool: true}},
			len:                 2,
		},
		"Should return ethSendRawTxEntries when request sent to relay": {
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{WasSentToRelay: true}, {WasSentToRelay: true}},
			want:                []*database.EthSendRawTxEntry{{WasSentToRelay: true}, {WasSentToRelay: true}},
			len:                 2,
		},
		"Should return ethSendRawTxEntries when request entry has error": {
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{ErrorCode: -32600}, {ErrorCode: -32601}},
			want:                []*database.EthSendRawTxEntry{{ErrorCode: -32600}, {ErrorCode: -32601}},
			len:                 2,
		},
		"Should return empty ethSendRawTxEntries when request entry has no error and not sent to mempool as well as to relay": {
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{IsOnOafcList: true}, {IsCancelTx: true}},
			want:                []*database.EthSendRawTxEntry{},
			len:                 0,
		},
		"Should return ethSendRawTxEntries when request entry met entry condition": {
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{IsOnOafcList: true},
				{IsCancelTx: true}, {WasSentToRelay: true, ErrorCode: -32600}, {WasSentToMempool: true}},
			want: []*database.EthSendRawTxEntry{{WasSentToRelay: true, ErrorCode: -32600}, {WasSentToMempool: true}},
			len:  2,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			r := &requestRecord{
				ethSendRawTxEntries: testCase.ethSendRawTxEntries,
				mutex:               sync.Mutex{},
			}
			got := r.getValidRawTxEntriesToSave()
			require.Equal(t, testCase.want, got)
			require.Equal(t, testCase.len, len(got))
		})
	}
}

func Test_requestRecord_SaveRecord(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()
	tests := map[string]struct {
		id                  uuid.UUID
		requestEntry        database.RequestEntry
		ethSendRawTxEntries []*database.EthSendRawTxEntry
		rawTxEntryLen       int
	}{
		"Should successfully store batch request": {
			id:           id1,
			requestEntry: database.RequestEntry{Id: id1},
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{
				{RequestId: id1, WasSentToMempool: true}, {RequestId: id1, WasSentToMempool: true},
				{RequestId: id1, IsCancelTx: true}, {RequestId: id1, WasSentToRelay: true, ErrorCode: -32600}, {RequestId: id1, WasSentToMempool: true},
			},
			rawTxEntryLen: 4,
		},
		"Should successfully store single request": {
			id:                  id2,
			requestEntry:        database.RequestEntry{Id: id2},
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{RequestId: id2, WasSentToMempool: true}},
			rawTxEntryLen:       1,
		},
		"Should successfully store single request with rawTxEntry has error": {
			id:                  id3,
			requestEntry:        database.RequestEntry{Id: id3},
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{RequestId: id3, ErrorCode: -32600}},
			rawTxEntryLen:       1,
		},
		"Should not store if the request doesnt meet entry condition": {
			id:                  id4,
			requestEntry:        database.RequestEntry{Id: id4},
			ethSendRawTxEntries: []*database.EthSendRawTxEntry{{RequestId: id4, IsCancelTx: true}},
			rawTxEntryLen:       0,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			db := database.NewMemStore()
			pusher := NewRequestPusher(db, 1, time.Millisecond*2)
			pusher.Run()
			r := &requestRecord{
				requestEntry:        testCase.requestEntry,
				ethSendRawTxEntries: testCase.ethSendRawTxEntries,
				mutex:               sync.Mutex{},
				reqPusher:           pusher,
			}
			r.SaveRecord()
			//			require.Equal(t, testCase.rawTxEntryLen, len(db.EthSendRawTxs))
		})
	}
}
