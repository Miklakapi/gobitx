package protocol

type Command uint8

const (
	CommandUnknown Command = iota

	CommandOK
	CommandError

	CommandPing
	CommandPong

	CommandDownload
	CommandUpload
	CommandQuality

	CommandReady
	CommandResult
)

func (c Command) String() string {
	switch c {
	case CommandOK:
		return "OK"
	case CommandError:
		return "ERROR"
	case CommandPing:
		return "PING"
	case CommandPong:
		return "PONG"
	case CommandDownload:
		return "DOWNLOAD"
	case CommandUpload:
		return "UPLOAD"
	case CommandQuality:
		return "QUALITY"
	case CommandReady:
		return "READY"
	case CommandResult:
		return "RESULT"
	default:
		return "UNKNOWN"
	}
}

func (c Command) IsResponse() bool {
	switch c {
	case CommandOK, CommandError, CommandPong, CommandReady:
		return true
	default:
		return false
	}
}

func (c Command) IsRequest() bool {
	switch c {
	case CommandPing,
		CommandDownload,
		CommandUpload,
		CommandQuality,
		CommandResult:
		return true
	default:
		return false
	}
}
