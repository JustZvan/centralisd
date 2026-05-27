package handlers

import (
	"centralisd/src/core/protocol"
	"encoding/json"
)

func handleNoop(cmd protocol.NodeCommand) (json.RawMessage, error) {
	return json.RawMessage{}, nil
}
