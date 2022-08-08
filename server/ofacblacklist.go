// OFAC banned addresses
package server

var ofacBlacklist = map[string]bool{
	"0x8576acc5c05d6ce88f4e49bf65bdf0c62f91353c": true,
	"0xd882cFc20F52f2599D84b8e8D58C7FB62cfE344b": true,
	"0x901bb9583b24D97e995513C6778dc6888AB6870e": true,
	"0xa7e5d5a720f06526557c513402f2e6b5fa20b00":  true, // this is an invalid address, but is what"s listed in the ofac ban list
	"0xA7e5d5A720f06526557c513402f2e6B5fA20b008": true, // the actual valid address
	"0x7F367cC41522cE07553e823bf3be79A889DEbe1B": true,
	"0x1da5821544e25c636c1417Ba96Ade4Cf6D2f9B5A": true,
	"0x7Db418b5D567A4e0E8c59Ad71BE1FcE48f3E6107": true,
	"0x72a5843cc08275C8171E582972Aa4fDa8C397B2A": true,
	"0x7F19720A857F834887FC9A7bC0a0fBe7Fc7f8102": true,
	"0x9F4cda013E354b8fC285BF4b9A60460cEe7f7Ea9": true,
	"0x722122dF12D4e14e13Ac3b6895a86e84145b6967": true,
	"0xDD4c48C0B24039969fC16D1cdF626eaB821d3384": true,
	"0xd90e2f925DA726b50C4Ed8D0Fb90Ad053324F31b": true,
	"0xd96f2B1c14Db8458374d9Aca76E26c3D18364307": true,
	"0x4736dCf1b7A3d580672CcE6E7c65cd5cc9cFBa9D": true,
	"0xD4B88Df4D29F5CedD6857912842cff3b20C8Cfa3": true,
	"0x910Cbd523D972eb0a6f4cAe4618aD62622b39DbF": true,
	"0xA160cdAB225685dA1d56aa342Ad8841c3b53f291": true,
	"0xFD8610d20aA15b7B2E3Be39B396a1bC3516c7144": true,
	"0x22aaA7720ddd5388A3c0A3333430953C68f1849b": true,
	"0xBA214C1c1928a32Bffe790263E38B4Af9bFCD659": true,
	"0xb1C8094B234DcE6e03f10a5b673c1d8C69739A00": true,
	"0x527653eA119F3E6a1F5BD18fbF4714081D7B31ce": true,
	"0x58E8dCC13BE9780fC42E8723D8EaD4CF46943dF2": true,
	"0xD691F27f38B395864Ea86CfC7253969B409c362d": true,
	"0xaEaaC358560e11f52454D997AAFF2c5731B6f8a6": true,
	"0x1356c899D8C9467C7f71C195612F8A395aBf2f0a": true,
	"0xA60C772958a3eD56c1F15dD055bA37AC8e523a0D": true,
	"0x169AD27A470D064DEDE56a2D3ff727986b15D52B": true,
	"0x0836222F2B2B24A3F36f98668Ed8F0B38D1a872f": true,
	"0xF67721A2D8F736E75a49FdD7FAd2e31D8676542a": true,
	"0x9AD122c22B14202B4490eDAf288FDb3C7cb3ff5E": true,
	"0x905b63Fff465B9fFBF41DeA908CEb12478ec7601": true,
	"0x07687e702b410Fa43f4cB4Af7FA097918ffD2730": true,
	"0x94A1B5CdB22c43faab4AbEb5c74999895464Ddaf": true,
	"0xb541fc07bC7619fD4062A54d96268525cBC6FfEF": true,
	"0x12D66f87A04A9E220743712cE6d9bB1B5616B8Fc": true,
	"0x47CE0C6eD5B0Ce3d3A51fdb1C52DC66a7c3c2936": true,
	"0x23773E65ed146A459791799d01336DB287f25334": true,
	"0xD21be7248e0197Ee08E0c20D4a96DEBdaC3D20Af": true,
	"0x610B717796ad172B316836AC95a2ffad065CeaB4": true,
	"0x178169B423a011fff22B9e3F3abeA13414dDD0F1": true,
	"0xbB93e510BbCD0B7beb5A853875f9eC60275CF498": true,
	"0x2717c5e28cf931547B621a5dddb772Ab6A35B701": true,
	"0x03893a7c7463AE47D46bc7f091665f1893656003": true,
	"0xCa0840578f57fE71599D29375e16783424023357": true,
	"0x8589427373D6D84E98730D7795D8f6f8731FDA16": true,
	"0x098B716B8Aaf21512996dC57EB0615e2383E2f96": true,
	"0xa0e1c89Ef1a489c9C7dE96311eD5Ce5D32c20E4B": true,
	"0x3Cffd56B47B7b41c56258D9C7731ABaDc360E073": true,
	"0x53b6936513e738f44FB50d2b9476730C0Ab3Bfc1": true,
	"0x35fB6f6DB4fb05e6A4cE86f2C93691425626d4b1": true,
	"0xF7B31119c2682c88d88D455dBb9d5932c65Cf1bE": true,
	"0x3e37627dEAA754090fBFbb8bd226c1CE66D255e9": true,
	"0x08723392Ed15743cc38513C4925f5e6be5c17243": true,
}

func isOnOFACList(address string) bool {
	return ofacBlacklist[address]
}
