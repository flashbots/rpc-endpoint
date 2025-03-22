// OFAC banned addresses
package server

import "strings"

var ofacBlacklist = map[string]bool{
	"0x04dba1194ee10112fe6c3207c0687def0e78bacf": true,
	"0x08723392ed15743cc38513c4925f5e6be5c17243": true,
	"0x08b2efdcdb8822efe5ad0eae55517cf5dc544251": true,
	"0x098b716b8aaf21512996dc57eb0615e2383e2f96": true,
	"0x0ee5067b06776a89ccc7dc8ee369984ad7db5e06": true,
	"0x1da5821544e25c636c1417ba96ade4cf6d2f9b5a": true,
	"0x1967d8af5bd86a497fb3dd7899a020e47560daaf": true,
	"0x19aa5fe80d33a56d56c78e82ea5e50e5d80b4dff": true,
	"0x35fb6f6db4fb05e6a4ce86f2c93691425626d4b1": true,
	"0x3ad9db589d201a710ed237c829c7860ba86510fc": true,
	"0x3cbded43efdaf0fc77b9c55f6fc9988fcc9b757d": true,
	"0x3cffd56b47b7b41c56258d9c7731abadc360e073": true,
	"0x3e37627deaa754090fbfbb8bd226c1ce66d255e9": true,
	"0x48549a34ae37b12f6a30566245176994e17c6b4a": true,
	"0x502371699497d08d5339c870851898d6d72521dd": true,
	"0x53b6936513e738f44fb50d2b9476730c0ab3bfc1": true,
	"0x5512d943ed1f7c8a43f3435c85f7ab68b30121b0": true,
	"0x5a14e72060c11313e38738009254a90968f58f51": true,
	"0x6acdfba02d390b97ac2b2d42a63e85293bcc160e": true,
	"0x6f1ca141a28907f78ebaa64fb83a9088b02a8352": true,
	"0x72a5843cc08275c8171e582972aa4fda8c397b2a": true,
	"0x7db418b5d567a4e0e8c59ad71be1fce48f3e6107": true,
	"0x7f19720a857f834887fc9a7bc0a0fbe7fc7f8102": true,
	"0x7f367cc41522ce07553e823bf3be79a889debe1b": true,
	"0x7ff9cfad3877f21d41da833e2f775db0569ee3d9": true,
	"0x901bb9583b24d97e995513c6778dc6888ab6870e": true,
	"0x9f4cda013e354b8fc285bf4b9a60460cee7f7ea9": true,
	"0xa0e1c89ef1a489c9c7de96311ed5ce5d32c20e4b": true,
	"0xa7e5d5a720f06526557c513402f2e6b5fa20b008": true,
	"0xc2a3829f459b3edd87791c74cd45402ba0a20be3": true,
	"0xc455f7fd3e0e12afd51fba5c106909934d8a0e4a": true,
	"0xcc84179ffd19a1627e79f8648d09e095252bc418": true,
	"0xd0975b32cea532eadddfc9c60481976e39db3472": true,
	"0xd882cfc20f52f2599d84b8e8d58c7fb62cfe344b": true,
	"0xe7aa314c77f4233c18c6cc84384a9247c0cf367b": true,
	"0xefe301d259f525ca1ba74a7977b80d5b060b3cca": true,
}

func isOnOFACList(address string) bool {
	addrs := strings.ToLower(address)
	return ofacBlacklist[addrs]
}
