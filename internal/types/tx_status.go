package types

// TxStatus defines the possible statuses of a transaction record.
type TxStatus string

const (
	// TxStatusSuccess indicates the transaction was successfully sent and confirmed (if applicable).
	TxStatusSuccess TxStatus = "Success"
	// TxStatusFailed indicates the transaction was sent but failed on-chain.
	TxStatusFailed TxStatus = "Failed"
	// TxStatusErrorBeforeSend indicates an error occurred before the transaction could be sent (e.g., client creation, nonce fetch, gas estimation).
	TxStatusErrorBeforeSend TxStatus = "ErrorBeforeSend"
)
