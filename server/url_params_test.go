package server

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
)

func TestExtractAuctionPreferenceFromUrl(t *testing.T) {
	ptrInt := func(i int) *int {
		return &i
	}

	tests := map[string]struct {
		url  string
		want URLParameters
		err  error
	}{
		"no auction preference": {
			url: "https://rpc.flashbots.net",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"hash", "special_logs"}},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
		"correct hint preference": {
			url: "https://rpc.flashbots.net?hint=contract_address&hint=function_selector&hint=logs&hint=calldata&hint=hash",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"contract_address", "function_selector", "logs", "calldata", "hash", "special_logs"}},
				prefWasSet: true,
				originId:   "",
			},
			err: nil,
		},
		"incorrect hint preference": {
			url:  "https://rpc.flashbots.net?hint=contract_address&hint=function_selector&hint=logs&hint=incorrect",
			want: URLParameters{},
			err:  ErrIncorrectAuctionHints,
		},
		"fast url works": {
			url: "https://rpc.flashbots.net/fast",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"hash", "special_logs"}},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
		"rpc endpoint set": {
			url: "https://rpc.flashbots.net?rpc=https://mainnet.infura.io/v3/123",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"hash", "special_logs"}},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
		"origin id": {
			url: "https://rpc.flashbots.net?originId=123",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"hash", "special_logs"}},
				prefWasSet: false,
				originId:   "123",
			},
			err: nil,
		},
		"target builder": {
			url: "https://rpc.flashbots.net?builder=builder1&builder=builder2",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"hash", "special_logs"}, Builders: []string{"builder1", "builder2"}},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
		"set refund": {
			url: "https://rpc.flashbots.net?refund=17",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{Hints: []string{"hash", "special_logs"}, WantRefund: ptrInt(17)},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
		"incorrect refund, -1": {
			url: "https://rpc.flashbots.net?refund=-1",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{},
				prefWasSet: false,
				originId:   "",
			},
			err: ErrIncorrectRefundQuery,
		},
		"incorrect refund, 120": {
			url: "https://rpc.flashbots.net?refund=120",
			want: URLParameters{
				pref:       types.TxPrivacyPreferences{},
				prefWasSet: false,
				originId:   "",
			},
			err: ErrIncorrectRefundQuery,
		},
		"set refund address": {
			url: "https://rpc.flashbots.net?refund=17&refundAddress=0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			want: URLParameters{
				pref: types.TxPrivacyPreferences{
					Hints:        []string{"hash", "special_logs"},
					WantRefund:   ptrInt(17),
					RefundConfig: []types.RefundConfig{{Address: common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), Percent: 100}},
				},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
		"set refund addresses": {
			url: "https://rpc.flashbots.net?refund=17&refundAddress=0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:80&refundAddress=0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb:20",
			want: URLParameters{
				pref: types.TxPrivacyPreferences{
					Hints:      []string{"hash", "special_logs"},
					WantRefund: ptrInt(17),
					RefundConfig: []types.RefundConfig{
						{Address: common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), Percent: 80},
						{Address: common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), Percent: 20},
					},
				},
				prefWasSet: false,
				originId:   "",
			},
			err: nil,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			url, err := url.Parse(tt.url)
			if err != nil {
				t.Fatal("failed to parse url: ", err)
			}

			got, err := ExtractParametersFromUrl(url)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}

			if tt.err == nil {
				require.Equal(t, tt.want, got)
			}
		})
	}
}
