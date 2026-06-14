package protocol

import "encoding/json"

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

func NewFrame(command Command, payload any) (Frame, error) {
	var jsonPayload []byte = nil

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return Frame{}, err
		}

		jsonPayload = data
	}

	return Frame{
		Command: command,
		Payload: jsonPayload,
	}, nil
}
