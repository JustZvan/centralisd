package protocol

import (
    "bufio"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "errors"
    "strings"
)

type Packet struct {
    Type    string          `json:"type"`
    ID      string          `json:"id,omitempty"`
    ReplyTo string          `json:"reply_to,omitempty"`
    Error   string          `json:"error,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
}

func NewPacket(packetType string, payload any) (Packet, error) {
	if strings.TrimSpace(packetType) == "" {
		return Packet{}, errors.New("packet type is empty")
	}
    var raw json.RawMessage
    if payload != nil {
        b, err := json.Marshal(payload)
        if err != nil {
            return Packet{}, err
        }
        raw = b
    }
	return Packet{Type: packetType, Payload: raw}, nil
}

func NewError(message string) Packet {
	return Packet{Type: string(PacketError), Error: strings.TrimSpace(message)}
}

func NewReply(packetType, replyTo string, payload any) (Packet, error) {
    p, err := NewPacket(packetType, payload)
    if err != nil {
        return Packet{}, err
    }
    p.ReplyTo = strings.TrimSpace(replyTo)
    return p, nil
}

func DecodePayload(p Packet, out any) error {
    if len(p.Payload) == 0 {
        return errors.New("payload is empty")
    }
    return json.Unmarshal(p.Payload, out)
}

func WritePacket(writer *bufio.Writer, p Packet) error {
    if writer == nil {
        return errors.New("nil writer")
    }
    if strings.TrimSpace(p.Type) == "" {
        return errors.New("packet type is empty")
    }
    b, err := json.Marshal(p)
    if err != nil {
        return err
    }
    if _, err := writer.WriteString(string(b) + "\n"); err != nil {
        return err
    }
    return writer.Flush()
}

func ReadPacket(reader *bufio.Reader) (Packet, error) {
    if reader == nil {
        return Packet{}, errors.New("nil reader")
    }
    line, err := reader.ReadString('\n')
    if err != nil {
        return Packet{}, err
    }
    line = strings.TrimSpace(line)
    if line == "" {
        return Packet{}, errors.New("empty packet line")
    }
    p := Packet{}
    if err := json.Unmarshal([]byte(line), &p); err != nil {
        return Packet{}, err
    }
    if strings.TrimSpace(p.Type) == "" {
        return Packet{}, errors.New("packet type missing")
    }
    return p, nil
}

func NewID() string {
    b := make([]byte, 16)
    _, _ = rand.Read(b)
    return base64.RawURLEncoding.EncodeToString(b)
}
