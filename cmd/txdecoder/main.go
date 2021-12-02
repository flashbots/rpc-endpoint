package main

import (
	"fmt"
	"log"

	"github.com/flashbots/rpc-endpoint/server"
)

var rawTx = ""

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
