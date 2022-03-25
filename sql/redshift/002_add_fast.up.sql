ALTER TABLE
    rpc_endpoint_eth_send_raw_txs
ADD COLUMN
    fast boolean
DEFAULT FALSE;
