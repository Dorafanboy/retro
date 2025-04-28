package types

// TxStatus defines the possible statuses of a transaction record.
type TxStatus string

const (
	TxStatusSuccess         TxStatus = "Success"
	TxStatusFailed          TxStatus = "Failed"
	TxStatusErrorBeforeSend TxStatus = "ErrorBeforeSend"
)
