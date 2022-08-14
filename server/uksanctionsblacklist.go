// UK Sanctions List addresses
// https://docs.fcdo.gov.uk/docs/UK-Sanctions-List.html
package server

import "strings"

var ukSanctionsBlacklist = map[string]bool{
	"0x7ff9cfad3877f21d41da833e2f775db0569ee3d9": true,
}

func isOnUKSanctionsList(address string) bool {
	addrs := strings.ToLower(address)
	return ukSanctionsBlacklist[addrs]
}
