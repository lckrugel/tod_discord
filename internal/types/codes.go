package types

type ErrorCode string

const (
	BAD_CONFIG        ErrorCode = "E_BAD_CONFIG"
	CONNECTION_FAILED ErrorCode = "E_CONNECTION_FAILED"
)
