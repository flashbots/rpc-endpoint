package database

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const (
	connTimeOut = 10 * time.Second
)

type postgresStore struct {
	DB *sqlx.DB
}

func NewPostgresStore(dsn string) *postgresStore {
	db := sqlx.MustConnect("postgres", dsn)
	db.DB.SetMaxOpenConns(50)
	db.DB.SetMaxIdleConns(10)
	db.DB.SetConnMaxIdleTime(0)
	return &postgresStore{
		DB: db,
	}
}

func (d *postgresStore) Close() {
	d.DB.Close()
}

func (d *postgresStore) SaveRequestEntry(entry RequestEntry) error {
	query := `INSERT INTO rpc_endpoint_requests
	(id, received_at, request_duration_ms, is_batch_request, num_request_in_batch, http_method, http_url, http_query_param, http_response_status, origin, host, error) VALUES (:id, :received_at, :request_duration_ms, :is_batch_request, :num_request_in_batch, :http_method, :http_url, :http_query_param, :http_response_status, :origin, :host, :error)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	_, err := d.DB.NamedExecContext(ctx, query, entry)
	return err
}

func (d *postgresStore) SaveRawTxEntries(entries []*EthSendRawTxEntry) error {
	query := `INSERT INTO rpc_endpoint_eth_send_raw_txs (id, request_id, is_on_oafc_list, is_white_hat_bundle_collection, white_hat_bundle_id, is_cancel_tx, needs_front_running_protection, was_sent_to_relay, was_sent_to_mempool, is_blocked, error, error_code, tx_raw, tx_hash, tx_from, tx_to, tx_nonce, tx_data, tx_smart_contract_method,fast) VALUES (:id, :request_id, :is_on_oafc_list, :is_white_hat_bundle_collection, :white_hat_bundle_id, :is_cancel_tx, :needs_front_running_protection, :was_sent_to_relay, :was_sent_to_mempool, :is_blocked, :error, :error_code, :tx_raw, :tx_hash, :tx_from, :tx_to, :tx_nonce, :tx_data, :tx_smart_contract_method, :fast)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	_, err := d.DB.NamedExecContext(ctx, query, entries)
	return err
}
