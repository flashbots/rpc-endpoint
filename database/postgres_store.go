package database

import (
	"context"
	"github.com/ethereum/go-ethereum/log"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/lib/pq"
	"time"
)

const (
	connTimeOut = 10 * time.Second
)

type postgresStore struct {
	DB *pgxpool.Pool
}

func NewPostgresStore(dsn string) (*postgresStore, error) {
	pool, err := pgxpool.Connect(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &postgresStore{
		DB: pool,
	}, nil
}

func (d *postgresStore) Close() {
	d.DB.Close()
}

func (d *postgresStore) SaveRequestEntry(in *RequestEntry) error {
	query := `INSERT INTO rpc_endpoint_requests 
	(id, received_at, inserted_at, request_duration, is_batch_request, num_request_in_batch, http_method, http_url, http_query_param, http_response_status, ip_hash, origin, host, error) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`
	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()
	if _, err := d.DB.Exec(ctx, query, in.Id, in.ReceivedAt, time.Now(), in.RequestDuration, in.IsBatchRequest, in.NumRequestInBatch, in.HttpMethod, in.HttpUrl, in.HttpQueryParam, in.HttpResponseStatus, in.IpHash, in.Origin, in.Host, in.Error); err != nil {
		log.Error("[postgresStore] SaveRequestEntry failed", "error", err)
		return err
	}
	log.Info("[postgresStore] SaveRequestEntry succeeded", "RequestEntry", in) // TODO:Remove logging
	return nil
}

func (d *postgresStore) SaveEthSendRawTxEntries(in []*EthSendRawTxEntry) error {
	query := `INSERT INTO rpc_endpoint_eth_send_raw_txs (id, request_id, is_on_oafc_list, is_white_hat_bundle_collection, white_hat_bundle_id, is_cancel_tx, needs_front_running_protection, was_sent_to_relay, is_tx_sent_to_relay, is_blocked_bcz_already_sent, error, error_code, tx_raw, tx_hash, tx_from, tx_to, tx_nonce, tx_data, tx_smart_contract_method) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`

	ctx, cancel := context.WithTimeout(context.Background(), connTimeOut)
	defer cancel()

	// Begin transaction
	tx, err := d.DB.Begin(ctx)
	if err != nil {
		log.Error("[SaveEthSendRawTxEntryToDB] failed to begin transaction", "error", err)
	}

	// Prepare batch
	batch := &pgx.Batch{}
	for _, entry := range in {
		batch.Queue(query, entry.Id, entry.RequestId, entry.IsOnOafcList, entry.IsWhiteHatBundleCollection, entry.WhiteHatBundleId, entry.IsCancelTx, entry.NeedsFrontRunningProtection, entry.WasSentToRelay, entry.IsTxSentToRelay, entry.IsBlockedBczAlreadySent, entry.Error, entry.ErrorCode, entry.TxRaw, entry.TxHash, entry.TxFrom, entry.TxTo, entry.TxNonce, entry.TxData, entry.TxSmartContractMethod)
	}
	br := d.DB.SendBatch(ctx, batch)

	_, err = br.Exec()
	if err != nil {
		if e := tx.Rollback(ctx); e != nil {
			log.Error("[SaveEthSendRawTxEntryToDB] failed to rollback transaction", "error", err)
		}

		// It's very important to close the batch operation on error
		if e := br.Close(); e != nil {
			log.Error("[SaveEthSendRawTxEntryToDB] failed to close batch", "error", err)
		}
		return err
	}

	if err = br.Close(); err != nil {
		if e := tx.Rollback(ctx); e != nil {
			log.Error("[SaveEthSendRawTxEntryToDB] failed to close and rollback batch", "error", err)
		}
		return err
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		log.Error("[SaveEthSendRawTxEntryToDB] failed to commit batch tx", "error", err)
		return err
	}
	log.Info("[RequestRecord] SaveEthSendRawTxEntryToDB succeeded", "EthSendRawTxEntry", in) // TODO:Remove logging
	return nil
}
