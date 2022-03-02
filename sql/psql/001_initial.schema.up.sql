BEGIN;
CREATE TABLE rpc_endpoint_requests(
    id uuid not null unique primary key default gen_random_uuid(),
    received_at timestamp with time zone not null default now(),
    inserted_at timestamp with time zone not null default now(),
    request_duration_ms bigint not null,
    is_batch_request boolean,
    num_request_in_batch integer,
    http_method varchar(10) not null,
    http_url varchar(100) not null,
    http_query_param varchar,
    http_response_status integer,
    ip_hash varchar(32) not null,
    origin varchar(100),
    host varchar(100),
    error varchar(1000)
);
CREATE TABLE rpc_endpoint_eth_send_raw_txs(
    id uuid not null unique primary key,
    request_id uuid not null,
    is_on_oafc_list boolean,
    is_white_hat_bundle_collection boolean,
    white_hat_bundle_id varchar,
    is_cancel_tx boolean,
    needs_front_running_protection boolean,
    was_sent_to_relay boolean,
    is_blocked_bcz_already_sent boolean,
    error varchar(1000),
    error_code integer,
    tx_raw varchar,
    tx_hash varchar(66),
    tx_from varchar(42),
    tx_to varchar(42),
    tx_nonce integer,
    tx_data varchar(10000),
    tx_smart_contract_method varchar(10)
);
COMMIT;