package server

import (
	"github.com/flashbots/rpc-endpoint/types"
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"
)

func TestExtractAuctionPreferenceFromUrl(t *testing.T) {
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
				originId: "" +
					"",
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
