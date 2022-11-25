BEGIN;

CREATE TABLE rpc_endpoint_requests(
    id uuid not null unique primary key default gen_random_uuid(),
    received_at timestamp with time zone not null,
    inserted_at timestamp with time zone not null default now(),
    request_duration_ms bigint not null,
    is_batch_request boolean,
    num_request_in_batch integer,
    http_method varchar(10) not null,
    http_url varchar(100) not null,
    http_query_param text,
    http_response_status integer,
    origin text,
    host text,
    error text
);

CREATE TABLE rpc_endpoint_eth_send_raw_txs(
    id uuid not null unique primary key,
    request_id uuid not null,
    inserted_at timestamp with time zone not null default now(),
    is_on_oafc_list boolean,
    is_white_hat_bundle_collection boolean,
    white_hat_bundle_id varchar,
    is_cancel_tx boolean,
    needs_front_running_protection boolean,
    was_sent_to_relay boolean,
    was_sent_to_mempool boolean,
    is_blocked_bcz_already_sent boolean,
    error text,
    error_code integer,
    tx_raw text,
    tx_hash varchar(66),
    tx_from varchar(42),
    tx_to varchar(42),
    tx_nonce integer,
    tx_data text,
    tx_smart_contract_method varchar(10)
);

COMMIT;
