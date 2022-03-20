package server

import (
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"github.com/flashbots/rpc-endpoint/database"
	"time"
)

type RequestPusher struct {
	db         database.Store
	EntryChan  chan database.Entry
	BatchChan  chan []database.Entry
	maxItem    int           // maximum item to be pushed as batch
	maxTimeOut time.Duration // timeout in which entry to be added to batch queue
}

func NewRequestPusher(db database.Store, maxItem int, maxTimeOut time.Duration) *RequestPusher {
	return &RequestPusher{
		db:         db,
		EntryChan:  make(chan database.Entry, 100),
		BatchChan:  make(chan []database.Entry),
		maxItem:    maxItem,
		maxTimeOut: maxTimeOut,
	}
}

func (r *RequestPusher) Run() {
	go r.saveEntries()
	go r.addEntryToEntryQueue()
}

func (r *RequestPusher) addEntryToEntryQueue() {
	for keepGoing := true; keepGoing; {
		var batch []database.Entry
		ticker := time.NewTicker(r.maxTimeOut)
		for {
			select {
			case entry, ok := <-r.EntryChan:
				if !ok {
					keepGoing = false
					goto push
				}
				batch = append(batch, entry)
				if len(batch) > r.maxItem {
					goto push
				}
			case <-ticker.C:
				goto push
			}
		}
	push:
		if len(batch) > 0 {
			r.BatchChan <- batch
		}
	}
}
func (r *RequestPusher) saveEntries() {
	for entry := range r.BatchChan {
		r.BatchInsert(entry)
	}
}

func (r *RequestPusher) BatchInsert(entries []database.Entry) error {
	var reqEntries []database.RequestEntry
	var rawTxEntries [][]*database.EthSendRawTxEntry
	for _, entry := range entries {
		reqEntries = append(reqEntries, entry.ReqEntry)
		rawTxEntries = append(rawTxEntries, entry.RawTxEntries)
	}
	if err := r.db.SaveRequestEntries(reqEntries); err != nil {
		log.Error("SaveRequestEntry failed %v", err)
		return err
	}
	fmt.Println("calling SaveRawTxEntries")
	if err := r.db.SaveRawTxEntries(rawTxEntries); err != nil {
		log.Error("SaveRawTxEntries failed %v", err)
		return err
	}
	return nil
}
