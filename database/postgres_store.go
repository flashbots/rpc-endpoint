package database

import (
	"context"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"time"
)

const (
	connTimeOut = 10 * time.Second
)

type postgresStore struct {
	DB *sqlx.DB
}

func NewPostgresStore(dsn string) *postgresStore {
	db := sqlx.MustConnect("postgres", dsn)
	return &postgresStore{
		DB: db,
	}
}

func (d *postgresStore) Close() {
	d.DB.Close()
}

func (d *postgresStore) SaveRequestEntry(entry RequestEntry) error {
	query := `INSERT INTO rpc_endpoint_requests 
	(id, received_at, inserted_at, request_duration_ms, is_batch_request, num_request_in_batch, http_method, http_url, http_query_param, http_response_status, ip_hash, origin, host, error) VALUES (:id, :received_at, :inserted_at, :request_duration_ms, :is_batch_request, :num_request_in_batch, :http_method, :http_url, :http_query_param, :http_response_status, :ip_hash, :origin, :host, :error)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	_, err := d.DB.NamedExecContext(ctx, query, entry)
	return err
}

func (d *postgresStore) SaveRawTxEntries(entries []*EthSendRawTxEntry) error {
	query := `INSERT INTO rpc_endpoint_eth_send_raw_txs (id, request_id, is_on_oafc_list, is_white_hat_bundle_collection, white_hat_bundle_id, is_cancel_tx, needs_front_running_protection, was_sent_to_relay, was_sent_to_mempool, is_blocked_bcz_already_sent, error, error_code, tx_raw, tx_hash, tx_from, tx_to, tx_nonce, tx_data, tx_smart_contract_method) VALUES (:id, :request_id, :is_on_oafc_list, :is_white_hat_bundle_collection, :white_hat_bundle_id, :is_cancel_tx, :needs_front_running_protection, :was_sent_to_relay, :was_sent_to_mempool, :is_blocked_bcz_already_sent, :error, :error_code, :tx_raw, :tx_hash, :tx_from, :tx_to, :tx_nonce, :tx_data, :tx_smart_contract_method)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	_, err := d.DB.NamedExecContext(ctx, query, entries)
	return err
}
