package protocol

const (
	ProtocolVersion uint8 = 1
	HeaderSize            = 8
	MaxPayloadSize        = 1024 * 1024
)

var ProtocolMagic = [2]byte{'G', 'B'}

type Frame struct {
	Command Command
	Payload []byte
}
