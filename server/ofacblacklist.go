package server

var ofacBlacklist = map[string]bool{
	// OFAC banned addresses
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
}

func isOnOFACList(address string) bool {
	_, isBlacklisted := ofacBlacklist[address]
	return isBlacklisted
}
