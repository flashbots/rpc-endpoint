// OFAC banned addresses
package server

import "strings"

var ofacBlacklist = map[string]bool{
	"0x8576acc5c05d6ce88f4e49bf65bdf0c62f91353c": true,
	"0xd882cfc20f52f2599d84b8e8d58c7fb62cfe344b": true,
	"0x901bb9583b24d97e995513c6778dc6888ab6870e": true,
	"0xa7e5d5a720f06526557c513402f2e6b5fa20b00":  true, // this is an invalid address, but is what"s listed in the ofac ban list
	"0xa7e5d5a720f06526557c513402f2e6b5fa20b008": true, // the actual valid address
	"0x7f367cc41522ce07553e823bf3be79a889debe1b": true,
	"0x1da5821544e25c636c1417ba96ade4cf6d2f9b5a": true,
	"0x7db418b5d567a4e0e8c59ad71be1fce48f3e6107": true,
	"0x72a5843cc08275c8171e582972aa4fda8c397b2a": true,
	"0x7f19720a857f834887fc9a7bc0a0fbe7fc7f8102": true,
	"0x9f4cda013e354b8fc285bf4b9a60460cee7f7ea9": true,
	"0x722122df12d4e14e13ac3b6895a86e84145b6967": true,
	"0xdd4c48c0b24039969fc16d1cdf626eab821d3384": true,
	"0xd90e2f925da726b50c4ed8d0fb90ad053324f31b": true,
	"0xd96f2b1c14db8458374d9aca76e26c3d18364307": true,
	"0x4736dcf1b7a3d580672cce6e7c65cd5cc9cfba9d": true,
	"0xd4b88df4d29f5cedd6857912842cff3b20c8cfa3": true,
	"0x910cbd523d972eb0a6f4cae4618ad62622b39dbf": true,
	"0xa160cdab225685da1d56aa342ad8841c3b53f291": true,
	"0xfd8610d20aa15b7b2e3be39b396a1bc3516c7144": true,
	"0x22aaa7720ddd5388a3c0a3333430953c68f1849b": true,
	"0xba214c1c1928a32bffe790263e38b4af9bfcd659": true,
	"0xb1c8094b234dce6e03f10a5b673c1d8c69739a00": true,
	"0x527653ea119f3e6a1f5bd18fbf4714081d7b31ce": true,
	"0x58e8dcc13be9780fc42e8723d8ead4cf46943df2": true,
	"0xd691f27f38b395864ea86cfc7253969b409c362d": true,
	"0xaeaac358560e11f52454d997aaff2c5731b6f8a6": true,
	"0x1356c899d8c9467c7f71c195612f8a395abf2f0a": true,
	"0xa60c772958a3ed56c1f15dd055ba37ac8e523a0d": true,
	"0x169ad27a470d064dede56a2d3ff727986b15d52b": true,
	"0x0836222f2b2b24a3f36f98668ed8f0b38d1a872f": true,
	"0xf67721a2d8f736e75a49fdd7fad2e31d8676542a": true,
	"0x9ad122c22b14202b4490edaf288fdb3c7cb3ff5e": true,
	"0x905b63fff465b9ffbf41dea908ceb12478ec7601": true,
	"0x07687e702b410fa43f4cb4af7fa097918ffd2730": true,
	"0x94a1b5cdb22c43faab4abeb5c74999895464ddaf": true,
	"0xb541fc07bc7619fd4062a54d96268525cbc6ffef": true,
	"0x12d66f87a04a9e220743712ce6d9bb1b5616b8fc": true,
	"0x47ce0c6ed5b0ce3d3a51fdb1c52dc66a7c3c2936": true,
	"0x23773e65ed146a459791799d01336db287f25334": true,
	"0xd21be7248e0197ee08e0c20d4a96debdac3d20af": true,
	"0x610b717796ad172b316836ac95a2ffad065ceab4": true,
	"0x178169b423a011fff22b9e3f3abea13414ddd0f1": true,
	"0xbb93e510bbcd0b7beb5a853875f9ec60275cf498": true,
	"0x2717c5e28cf931547b621a5dddb772ab6a35b701": true,
	"0x03893a7c7463ae47d46bc7f091665f1893656003": true,
	"0xca0840578f57fe71599d29375e16783424023357": true,
	"0x8589427373d6d84e98730d7795d8f6f8731fda16": true,
	"0x098b716b8aaf21512996dc57eb0615e2383e2f96": true,
	"0xa0e1c89ef1a489c9c7de96311ed5ce5d32c20e4b": true,
	"0x3cffd56b47b7b41c56258d9c7731abadc360e073": true,
	"0x53b6936513e738f44fb50d2b9476730c0ab3bfc1": true,
	"0x35fb6f6db4fb05e6a4ce86f2c93691425626d4b1": true,
	"0xf7b31119c2682c88d88d455dbb9d5932c65cf1be": true,
	"0x3e37627deaa754090fbfbb8bd226c1ce66d255e9": true,
	"0x08723392ed15743cc38513c4925f5e6be5c17243": true,
}

func isOnOFACList(address string) bool {
	addrs := strings.ToLower(address)
	return ofacBlacklist[addrs]
}
