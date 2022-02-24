BEGIN;
CREATE TABLE rpc_endpoint_requests(
    id varchar(128) not null distkey,
    received_at timestamptz default sysdate,
    inserted_at timestamptz default sysdate sortkey,
    request_duration bigint not null,
    is_batch_request boolean,
    num_request_in_batch integer,
    http_method varchar(10),
    http_url varchar,
    http_query_param varchar,
    http_response_status integer,
    ip_hash varchar,
    origin varchar,
    host varchar,
    error varchar
);
CREATE TABLE rpc_endpoint_eth_send_raw_txs(
    id varchar(128) not null distkey,
    request_id varchar(128) not null,
    inserted_at timestamptz default sysdate sortkey,
    is_on_oafc_list boolean,
    is_white_hat_bundle_collection boolean,
    white_hat_bundle_id varchar,
    is_cancel_tx boolean,
    needs_front_running_protection boolean,
    was_sent_to_relay boolean,
    is_tx_sent_to_relay boolean,
    is_blocked_bcz_already_sent boolean,
    error varchar(max),
    error_code integer,
    tx_raw varchar,
    tx_hash varchar,
    tx_from varchar,
    tx_to varchar,
    tx_nonce integer,
    tx_data varchar(256),
    tx_smart_contract_method varchar
);
COMMIT;
