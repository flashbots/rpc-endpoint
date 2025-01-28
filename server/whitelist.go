// Whitelist for smart contract functions that never need protection.
package server

import "strings"

var allowedLargeTxTargets = map[string]bool{
	"0xff1f2b4adb9df6fc8eafecdcbf96a2b351680455": true, // Aztec rollup contract
	"0x47312450B3Ac8b5b8e247a6bB6d523e7605bDb60": true, // StarkWare SHARP Verifier (Mainnet)
	"0x8f97970aC5a9aa8D130d35146F5b59c4aef57963": true, // StarkWare SHARP Verifier (Goerli)
	"0x07ec0D28e50322Eb0C159B9090ecF3aeA8346DFe": true, // StarkWare SHARP Verifier (Sepolia)
	"0x000000000000feA5F4B241F9E77B4D43B76798a9": true, // oSnipe AutoSniper contract
}

func init() {
	// Ensure that addresses are also indexed lowercase
	for target, val := range allowedLargeTxTargets {
		allowedLargeTxTargets[strings.ToLower(target)] = val
	}
}
