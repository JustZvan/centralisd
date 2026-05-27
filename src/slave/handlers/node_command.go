package handlers

import (
	"bufio"
	"centralisd/src/core/protocol"
	"log"
	"strings"
)

func handleNodeCommand(writer *bufio.Writer, packet protocol.Packet) error {
	cmd := protocol.NodeCommand{}
	if err := protocol.DecodePayload(packet, &cmd); err != nil {
		log.Printf("slave: invalid command payload")
		return writeCommandReply(writer, packet.ID, protocol.CommandReply{Status: "error", Message: "invalid command"})
	}

	action := strings.TrimSpace(cmd.Action)
	log.Printf("slave: command action=%s", action)

	handler, ok := commandHandlers[action]
	if !ok {
		log.Printf("slave: unknown command action=%s", action)
		return writeCommandReply(writer, packet.ID, protocol.CommandReply{Status: "error", Message: "unknown action"})
	}

	output, err := handler(cmd)
	if err != nil {
		return writeCommandReply(writer, packet.ID, protocol.CommandReply{Status: "error", Message: err.Error()})
	}

	return writeCommandReply(writer, packet.ID, protocol.CommandReply{Status: "ok", Output: output})
}

func writeCommandReply(writer *bufio.Writer, replyTo string, payload protocol.CommandReply) error {
	reply, err := protocol.NewReply(string(protocol.PacketNodeCommandReply), replyTo, payload)
	if err != nil {
		return err
	}

	return protocol.WritePacket(writer, reply)
}
