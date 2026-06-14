package protocol

type ErrorCode string

const (
	ErrorUnknown ErrorCode = "unknown"

	ErrorServerBusy ErrorCode = "server_busy"

	ErrorInvalidCommand  ErrorCode = "invalid_command"
	ErrorInvalidPayload  ErrorCode = "invalid_payload"
	ErrorInvalidDuration ErrorCode = "invalid_duration"
	ErrorInvalidResult   ErrorCode = "invalid_result"

	ErrorUnsupportedVersion ErrorCode = "unsupported_version"
	ErrorInvalidMagic       ErrorCode = "invalid_magic"

	ErrorDataPortUnavailable  ErrorCode = "data_port_unavailable"
	ErrorDataConnectionFailed ErrorCode = "data_connection_failed"
	ErrorDataTransferFailed   ErrorCode = "data_transfer_failed"

	ErrorInternal ErrorCode = "internal_error"
)

var (
	ErrInvalidMagic       = errors.New("invalid protocol magic")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrPayloadTooLarge    = errors.New("payload too large")
)
