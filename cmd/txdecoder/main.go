package main

import (
	"fmt"
	"log"

	"github.com/flashbots/rpc-endpoint/server"
)

var rawTx = "0x02f86d010c851caf4ad000851caf4ad0008302134094c0e1142e97a9679fa0f9c13067af656cc24753738080c080a04db949f68b275646517c8225f2b783476579bfc58b6e6af9c0ead0b41f3f5fb1a0540bbff674288973d5c82c25bc7b8fe42343c4d64d85f79a9e21ab6698738103"

func main() {
	tx, err := server.GetTx(rawTx)
	if err != nil {
		log.Fatal(err)
	}

	txFrom, err := server.GetSenderFromRawTx(tx)
	if err != nil {
		log.Fatal(err)
	}

	txHash := tx.Hash().Hex()

	fmt.Println("rawTx:", rawTx)
	fmt.Println("Hash: ", txHash)
	fmt.Println("From: ", txFrom)
	fmt.Println("To:   ", tx.To())
	fmt.Println("Nonce:", tx.Nonce())
}
