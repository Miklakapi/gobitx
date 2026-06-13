package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
)

type Codec struct {
	r io.Reader
	w io.Writer
}

func NewCodec(rw io.ReadWriter) *Codec {
	return &Codec{
		r: rw,
		w: rw,
	}
}

func (c *Codec) WriteFrame(frame Frame) error {
	payloadLength := len(frame.Payload)
	if payloadLength > MaxPayloadSize {
		return fmt.Errorf("payload too large: %d bytes", payloadLength)
	}

	header := make([]byte, HeaderSize)

	header[0] = ProtocolMagic[0]
	header[1] = ProtocolMagic[1]
	header[2] = ProtocolVersion
	header[3] = byte(frame.Command)

	binary.BigEndian.PutUint32(header[4:8], uint32(payloadLength))

	err := writeFull(c.w, header)
	if err != nil {
		return err
	}

	err = writeFull(c.w, frame.Payload)
	if err != nil {
		return err
	}

	return nil
}

func (c *Codec) ReadFrame() (Frame, error) {
	header := make([]byte, HeaderSize)

	_, err := io.ReadFull(c.r, header)
	if err != nil {
		return Frame{}, err
	}

	if !slices.Equal(header[:2], ProtocolMagic[:]) {
		return Frame{}, fmt.Errorf("invalid protocol magic")
	}

	if header[2] != ProtocolVersion {
		return Frame{}, fmt.Errorf("unsupported protocol version: %d", header[2])
	}

	command := Command(header[3])

	payloadLength := binary.BigEndian.Uint32(header[4:8])
	if payloadLength > MaxPayloadSize {
		return Frame{}, fmt.Errorf("payload too large: %d bytes", payloadLength)
	}

	payload := make([]byte, payloadLength)

	_, err = io.ReadFull(c.r, payload)
	if err != nil {
		return Frame{}, err
	}

	return Frame{
		Command: command,
		Payload: payload,
	}, nil
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}

		data = data[n:]
	}

	return nil
}
