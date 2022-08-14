package server

import "testing"

func Test_isOnUKSanctionsList(t *testing.T) {
	tests := map[string]struct {
		address string
		want    bool
	}{
		"Check different cases": {
			address: "0X7ff9cfAD3877F21D41DA833E2F775DB0569EE3D9",
			want:    true,
		},
		"Check upper cases": {
			address: "0X7FF9CFAD3877F21D41DA833E2F775DB0569EE3D9",
			want:    true,
		},
		"Check unknow": {
			address: "0X5",
			want:    false,
		},
	}
	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			if got := isOnUKSanctionsList(testCase.address); got != testCase.want {
				t.Errorf("isOnUKSanctionsList() = %v, want %v", got, testCase.want)
			}
		})
	}
}
