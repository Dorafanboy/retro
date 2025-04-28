package storage

const CreateTxTableSQL = `
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    wallet_address VARCHAR(42) NOT NULL,
    task_name VARCHAR(255) NOT NULL,
    network VARCHAR(255) NOT NULL,
    tx_hash VARCHAR(66),
    status VARCHAR(50) NOT NULL,
    error_message TEXT
);`

const CreateStateTableSQL = `
CREATE TABLE IF NOT EXISTS application_state (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`
