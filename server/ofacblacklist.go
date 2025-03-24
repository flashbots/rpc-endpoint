// OFAC banned addresses
package server

import "strings"

var ofacBlacklist = map[string]bool{}

func isOnOFACList(address string) bool {
	addrs := strings.ToLower(address)
	return ofacBlacklist[addrs]
}
