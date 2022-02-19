package database

import (
	"context"
	"github.com/ethereum/go-ethereum/log"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"time"
)

const (
	connTimeOut = 10 * time.Second
)

type DatabaseService struct {
	DB *sqlx.DB
}

func NewDatabaseService(dsn string) Store {
	db := sqlx.MustConnect("postgres", dsn)
	return &DatabaseService{
		DB: db,
	}
}

func (d *DatabaseService) Close() {
	d.DB.Close()
}

func (d *DatabaseService) SaveRequestEntry(in *RequestEntry) error {
	query := `INSERT INTO requests.main 
	(id,received_at,inserted_at,request_duration,is_batch_request,num_request_in_batch,http_method,http_url,http_query_param,http_response_status,ip_hash,origin,host,error) VALUES (:id,:received_at,:inserted_at,:request_duration,:is_batch_request,:num_request_in_batch,:http_method,:http_url,:http_query_param,:http_response_status,:ip_hash,:origin,:host,:error)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	if _, err := d.DB.NamedExecContext(ctx, query, in); err != nil {
		log.Error("[DatabaseService] SaveRequestEntry failed", "error", err)
		return err
	}
	log.Info("[DatabaseService] SaveRequestEntry succeeded", "RequestEntry", in) // TODO:Remove logging
	return nil
}

func (d *DatabaseService) SaveEthSendRawTxEntry(in *EthSendRawTxEntry) error {
	query := `INSERT INTO requests.eth_send_raw_txs (id,request_id,is_on_oafc_list,is_white_hat_bundle_collection,white_hat_bundle_id,is_cancel_tx,needs_front_running_protection,was_sent_to_relay,is_tx_sent_to_relay,is_blocked_bcz_already_sent,error,error_code,tx_raw,tx_hash,tx_from,tx_to,tx_nonce,tx_data,tx_smart_contract_method) VALUES (:id,:request_id,:is_on_oafc_list,:is_white_hat_bundle_collection,:white_hat_bundle_id,:is_cancel_tx,:needs_front_running_protection,:was_sent_to_relay,:is_tx_sent_to_relay,:is_blocked_bcz_already_sent,:error,:error_code,:tx_raw,:tx_hash,:tx_from,:tx_to,:tx_nonce,:tx_data,:tx_smart_contract_method)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	if _, err := d.DB.NamedExecContext(ctx, query, in); err != nil {
		log.Error("[RequestRecord] SaveEthSendRawTxEntryToDB failed", "error", err)
		return err
	}
	log.Info("[RequestRecord] SaveEthSendRawTxEntryToDB succeeded", "EthSendRawTxEntry", in) // TODO:Remove logging
	return nil
}
