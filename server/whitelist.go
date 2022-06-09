// Whitelist for smart contract functions that never need protection.
package server

import "strings"

// Functions that never need protection
var allowedFunctions = map[string]bool{
	"a9059cbb": true, // transfer
	"23b872dd": true, // transferFrom
	"095ea7b3": true, // approve
	"2e1a7d4d": true, // weth withdraw
	"d0e30db0": true, // weth deposit
	"f242432a": true, // safe transfer NFT
}

func isOnFunctionWhitelist(data string) bool {
	if len(data) < 8 {
		return false
	}
	return allowedFunctions[data[0:8]]
}

var allowedLargeTxTargets = map[string]bool{
	"0xff1f2b4adb9df6fc8eafecdcbf96a2b351680455": true, // Aztec rollup contract
}

func init() {
	// Ensure that addresses are also indexed lowercase
	for target, val := range allowedLargeTxTargets {
		allowedLargeTxTargets[strings.ToLower(target)] = val
	}
}
