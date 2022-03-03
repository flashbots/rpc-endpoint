package server

import (
	"github.com/flashbots/rpc-endpoint/database"
	"github.com/stretchr/testify/require"
	"testing"
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
			}
			got := r.getForwardedRawTxEntries()
			require.Equal(t, testCase.want, got)
			require.Equal(t, testCase.len, len(got))
		})
	}
}
