BEGIN;

CREATE TABLE rpc_endpoint_requests(
    id varchar(128) not null distkey,
    received_at timestamptz not null,
    inserted_at timestamptz not null default sysdate sortkey,
    request_duration_ms bigint not null,
    is_batch_request boolean,
    num_request_in_batch integer,
    http_method varchar(10) not null,
    http_url varchar(100) not null,
    http_query_param varchar(1024),
    http_response_status integer,
    ip_hash varchar(32) not null,
    origin varchar(100),
    host varchar(100),
    error varchar(1000)
);

CREATE TABLE rpc_endpoint_eth_send_raw_txs(
    id varchar(128) not null distkey,
    request_id varchar(128) not null,
    inserted_at timestamptz default sysdate sortkey,
    is_on_oafc_list boolean,
    is_white_hat_bundle_collection boolean,
    white_hat_bundle_id varchar(1024),
    is_cancel_tx boolean,
    needs_front_running_protection boolean,
    was_sent_to_relay boolean,
    was_sent_to_mempool boolean,
    is_blocked_bcz_already_sent boolean,
    error varchar(max),
    error_code integer,
    tx_raw varchar(max),
    tx_hash varchar(66),
    tx_from varchar(42),
    tx_to varchar(42),
    tx_nonce integer,
    tx_data varchar(max),
    tx_smart_contract_method varchar(10)
);

COMMIT;
