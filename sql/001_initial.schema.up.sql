BEGIN;
CREATE SCHEMA IF NOT EXISTS requests;
CREATE TABLE requests.main(
    id uuid not null unique primary key,
    received_at timestamp with time zone not null default now(),
    inserted_at timestamp with time zone not null default now(),
    request_duration interval,
    is_batch_request boolean,
    num_request_in_batch integer,
    http_method varchar,
    http_url varchar,
    http_query_param varchar,
    http_response_status integer,
    ip_hash varchar,
    origin varchar,
    host varchar,
    error varchar
);
CREATE TABLE requests.eth_send_raw_txs (
    id uuid not null unique primary key,
    request_id uuid not null,
    is_on_oafc_list boolean,
    is_white_hat_bundle_collection boolean,
    white_hat_bundle_id varchar,
    is_cancel_tx varchar,
    needs_front_running_protection boolean,
    was_sent_to_relay boolean,
    is_tx_sent_to_relay boolean,
    is_blocked_bcz_already_sent boolean,
    error varchar,
    error_code integer,
    tx_raw varchar,
    tx_hash varchar,
    tx_from varchar,
    tx_to varchar,
    tx_nonce integer,
    tx_data bytea,
    tx_smart_contract_method bytea
);
COMMIT;