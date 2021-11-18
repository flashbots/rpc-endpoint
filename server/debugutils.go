package server

import (
	"log"
	"os"
)

var DebugDontSendTx = os.Getenv("DEBUG_DONT_SEND_RAWTX") != ""

func init() {
	if DebugDontSendTx {
		log.Println("DEBUG: NOT SENDING RAWTX!")
	}
}
