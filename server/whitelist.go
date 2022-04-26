// Whitelist for smart contract functions that never need protection.
package server

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
	"0x737901bea3eeb88459df9ef1be8ff3ae1b42a2ba": true, // Aztec rollup contract
}
