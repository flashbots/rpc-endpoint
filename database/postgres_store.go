package database

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"reflect"
	"strings"
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

func (d *postgresStore) SaveRequestEntries(entries []RequestEntry) error {
	tx, err := d.DB.Beginx()
	var (
		valStrings []string
		valArgs    []interface{}
	)
	count := reflect.TypeOf(RequestEntry{}).NumField()
	for i, entry := range entries {
		valStrings = append(valStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
			i*count+1, i*count+2, i*count+3, i*count+4, i*count+5, i*count+6, i*count+7, i*count+8, i*count+9, i*count+10, i*count+11, i*count+12, i*count+13, i*count+14))
		valArgs = append(valArgs, entry.Id, entry.ReceivedAt, time.Now(), entry.RequestDurationMs, entry.IsBatchRequest, entry.NumRequestInBatch, entry.HttpMethod, entry.HttpUrl, entry.HttpQueryParam, entry.HttpResponseStatus, entry.IpHash, entry.Origin, entry.Host, entry.Error)
	}

	if err != nil {
		log.Error("[saveRequestEntries] failed to begin tx")
		return err
	}

	query := fmt.Sprintf("INSERT INTO rpc_endpoint_requests (id, received_at, inserted_at, request_duration_ms, is_batch_request, num_request_in_batch, http_method, http_url, http_query_param, http_response_status, ip_hash, origin, host, error) VALUES %s ON CONFLICT(ID) DO NOTHING", strings.Join(valStrings, ","))
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	_, err = tx.ExecContext(ctx, query, valArgs...)
	cancel()
	//_, err = tx.Exec(query, valArgs...)
	if err != nil {
		if err = tx.Rollback(); err != nil {
			log.Error("[saveRequestEntries] failed to execute adn rollback tx")
			return err
		}
		log.Error("[saveRequestEntries] failed to execute tx")
		return err
	}
	if err = tx.Commit(); err != nil {
		log.Error("[saveRequestEntries] failed to commit tx")
		return err
	}
	return err
}

func (d *postgresStore) SaveRawTxEntries(entries [][]*EthSendRawTxEntry) error {
	tx, err := d.DB.Beginx()
	for _, outer := range entries {
		var (
			valStrings []string
			valArgs    []interface{}
		)
		count := reflect.TypeOf(EthSendRawTxEntry{}).NumField()
		for i, inner := range outer {
			valStrings = append(valStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				i*count+1, i*count+2, i*count+3, i*count+4, i*count+5, i*count+6, i*count+7, i*count+8, i*count+9, i*count+10, i*count+11, i*count+12, i*count+13, i*count+14, i*count+15, i*count+16, i*count+17, i*count+18, i*count+19, i*count+20))
			valArgs = append(valArgs, inner.Id, inner.RequestId, time.Now(), inner.IsOnOafcList, inner.IsWhiteHatBundleCollection, inner.WhiteHatBundleId, inner.IsCancelTx, inner.NeedsFrontRunningProtection, inner.WasSentToRelay, inner.WasSentToMempool, inner.IsBlockedBczAlreadySent, inner.Error, inner.ErrorCode, inner.TxRaw, inner.TxHash, inner.TxFrom, inner.TxTo, inner.TxNonce, inner.TxData, inner.TxSmartContractMethod)
		}

		if err != nil {
			log.Error("[saveRawTxEntries] failed to begin tx")
			return err
		}

		query := fmt.Sprintf("INSERT INTO rpc_endpoint_eth_send_raw_txs (id, request_id, inserted_at, is_on_oafc_list, is_white_hat_bundle_collection, white_hat_bundle_id, is_cancel_tx, needs_front_running_protection, was_sent_to_relay, was_sent_to_mempool, is_blocked_bcz_already_sent, error, error_code, tx_raw, tx_hash, tx_from, tx_to, tx_nonce, tx_data, tx_smart_contract_method) VALUES %s ON CONFLICT (id) DO NOTHING", strings.Join(valStrings, ","))
		ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
		_, err = tx.ExecContext(ctx, query, valArgs...)
		cancel()
		//_, err = tx.Exec(query, valArgs...)
		if err != nil {
			if err = tx.Rollback(); err != nil {
				log.Error("[saveRawTxEntries] failed to execute adn rollback tx")
				return err
			}
			log.Error("[saveRawTxEntries] failed to execute tx")
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		log.Error("[saveRawTxEntries] failed to commit tx")
		return err
	}
	return err
}
