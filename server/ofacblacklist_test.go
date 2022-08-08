package server

import "testing"

func Test_isOnOFACList(t *testing.T) {
	tests := map[string]struct {
		address string
		want    bool
	}{
		"Check different cases": {
			address: "0x53b6936513e738f44FB50d2b9476730c0ab3bfc1",
			want:    true,
		},
		"Check upper cases": {
			address: "0X53B6936513E738F44FB50D2B9476730C0AB3BFC1",
			want:    true,
		},
		"Check unknow": {
			address: "0X5",
			want:    false,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			if got := isOnOFACList(testCase.address); got != testCase.want {
				t.Errorf("isOnOFACList() = %v, want %v", got, testCase.want)
			}
		})
	}
}
